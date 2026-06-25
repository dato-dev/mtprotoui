package deploy

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/timurabdullin/mtprotoui/internal/sshclient"
)

//go:embed scripts/deploy-mtproto.sh
var deployMTGScript string

//go:embed scripts/deploy-tgws.sh
var deployTGWSScript string

type Result struct {
	Port      int    `json:"port"`
	Secret    string `json:"secret"`
	SNI       string `json:"sni"`
	ProxyLink string `json:"proxy_link"`
}

// Proxy types.
const (
	ProxyTypeMTG  = "mtg"
	ProxyTypeTGWS = "tg-ws-proxy"
)

const (
	mtgContainerName  = "mtproto-mtg"
	mtgImage          = "nineseconds/mtg:2"
	mtgDefaultPort    = 443
	tgwsContainerName = "tg-ws-proxy"
	tgwsImage         = "dato1/tg-ws-proxy:latest"
	tgwsDefaultPort   = 1443
)

// Options describes a single deployment.
type Options struct {
	ProxyType     string
	SNIDomain     string
	Port          int
	FakeTLS       bool
	ContainerName string
}

// ValidProxyType reports whether t is a supported proxy type.
func ValidProxyType(t string) bool {
	return t == ProxyTypeMTG || t == ProxyTypeTGWS
}

// NormalizeProxyType returns a supported proxy type, defaulting to mtg.
func NormalizeProxyType(t string) string {
	if ValidProxyType(t) {
		return t
	}
	return ProxyTypeMTG
}

// DefaultContainerName returns the conventional container name for a proxy type.
func DefaultContainerName(proxyType string) string {
	if NormalizeProxyType(proxyType) == ProxyTypeTGWS {
		return tgwsContainerName
	}
	return mtgContainerName
}

// DefaultPort returns the recommended host port for a proxy type.
func DefaultPort(proxyType string) int {
	if NormalizeProxyType(proxyType) == ProxyTypeTGWS {
		return tgwsDefaultPort
	}
	return mtgDefaultPort
}

// ImageFor returns the Docker image used by a proxy type.
func ImageFor(proxyType string) string {
	if NormalizeProxyType(proxyType) == ProxyTypeTGWS {
		return tgwsImage
	}
	return mtgImage
}

// UsesSNI reports whether the deployment relies on an SNI domain from the
// whitelist. mtg always uses fake-TLS with SNI; tg-ws-proxy only when its
// FakeTLS option is enabled.
func UsesSNI(proxyType string, fakeTLS bool) bool {
	if NormalizeProxyType(proxyType) == ProxyTypeMTG {
		return true
	}
	return fakeTLS
}

func Deploy(cfg sshclient.Config, opts Options) (Result, error) {
	proxyType := NormalizeProxyType(opts.ProxyType)
	port := opts.Port
	if port == 0 {
		port = DefaultPort(proxyType)
	}
	container := opts.ContainerName
	if container == "" {
		container = DefaultContainerName(proxyType)
	}

	var stdout, stderr string
	var err error
	switch proxyType {
	case ProxyTypeTGWS:
		fakeTLSDomain := ""
		if opts.FakeTLS {
			fakeTLSDomain = opts.SNIDomain
		}
		stdout, stderr, err = sshclient.RunScript(
			cfg, deployTGWSScript, container, strconv.Itoa(port), fakeTLSDomain,
		)
	default:
		stdout, stderr, err = sshclient.RunScript(
			cfg, deployMTGScript, opts.SNIDomain, container, strconv.Itoa(port),
		)
	}
	if err != nil {
		return Result{}, fmt.Errorf("deploy failed: %w (stderr: %s)", err, stderr)
	}

	result, parseErr := parseResult(stdout)
	if parseErr != nil {
		return Result{}, fmt.Errorf("parse deploy output: %w (stdout: %s, stderr: %s)", parseErr, stdout, stderr)
	}
	return result, nil
}

// DeployWithRetry runs Deploy up to `attempts` times, bounding each attempt with
// `perAttempt` so a hung SSH session can't leave a server stuck in "deploying".
// Transient failures (flaky SSH, slow Docker install) get a short backoff.
func DeployWithRetry(cfg sshclient.Config, opts Options, attempts int, perAttempt time.Duration) (Result, error) {
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		res, err := deployWithTimeout(cfg, opts, perAttempt)
		if err == nil {
			return res, nil
		}
		lastErr = err
		if i < attempts-1 {
			time.Sleep(time.Duration(i+1) * 5 * time.Second)
		}
	}
	return Result{}, fmt.Errorf("deploy failed after %d attempt(s): %w", attempts, lastErr)
}

func deployWithTimeout(cfg sshclient.Config, opts Options, timeout time.Duration) (Result, error) {
	type outcome struct {
		res Result
		err error
	}
	ch := make(chan outcome, 1)
	go func() {
		res, err := Deploy(cfg, opts)
		ch <- outcome{res, err}
	}()

	select {
	case o := <-ch:
		return o.res, o.err
	case <-time.After(timeout):
		return Result{}, fmt.Errorf("deploy timed out after %s", timeout)
	}
}

// Stats holds container resource usage as reported by `docker stats`.
type Stats struct {
	CPUPerc  string `json:"cpu_perc"`
	MemUsage string `json:"mem_usage"`
	MemPerc  string `json:"mem_perc"`
	NetIO    string `json:"net_io"`
	BlockIO  string `json:"block_io"`
	PIDs     string `json:"pids"`
}

// ContainerStats returns a one-shot resource snapshot for the container.
func ContainerStats(cfg sshclient.Config, name string) (Stats, error) {
	if name == "" {
		name = mtgContainerName
	}
	// Use a printable separator: docker stats fields never contain "|", and a
	// control byte would be escaped to literal text by %q in the shell command.
	const sep = "|"
	format := "{{.CPUPerc}}" + sep + "{{.MemUsage}}" + sep + "{{.MemPerc}}" +
		sep + "{{.NetIO}}" + sep + "{{.BlockIO}}" + sep + "{{.PIDs}}"
	cmd := fmt.Sprintf(`docker stats --no-stream --format %q %q`, format, name)

	stdout, stderr, err := sshclient.RunCommand(cfg, cmd)
	if err != nil {
		return Stats{}, fmt.Errorf("container stats: %w (stderr: %s)", err, stderr)
	}

	line := strings.TrimSpace(stdout)
	if line == "" {
		return Stats{}, fmt.Errorf("container %q not running", name)
	}
	fields := strings.Split(line, sep)
	if len(fields) < 6 {
		return Stats{}, fmt.Errorf("unexpected docker stats output: %q", line)
	}
	return Stats{
		CPUPerc:  strings.TrimSpace(fields[0]),
		MemUsage: strings.TrimSpace(fields[1]),
		MemPerc:  strings.TrimSpace(fields[2]),
		NetIO:    strings.TrimSpace(fields[3]),
		BlockIO:  strings.TrimSpace(fields[4]),
		PIDs:     strings.TrimSpace(fields[5]),
	}, nil
}

func CleanupContainer(cfg sshclient.Config, name string) error {
	if name == "" {
		name = mtgContainerName
	}
	cmd := fmt.Sprintf(`docker rm -f %q 2>/dev/null || true`, name)
	_, stderr, err := sshclient.RunCommand(cfg, cmd)
	if err != nil {
		return fmt.Errorf("stop container: %w (stderr: %s)", err, stderr)
	}
	return nil
}

// ContainerLogs returns the last `tail` log lines of the given container.
func ContainerLogs(cfg sshclient.Config, name string, tail int) (string, error) {
	if name == "" {
		name = mtgContainerName
	}
	if tail <= 0 || tail > 2000 {
		tail = 200
	}
	cmd := fmt.Sprintf(`docker logs --tail %d %q 2>&1`, tail, name)
	stdout, stderr, err := sshclient.RunCommand(cfg, cmd)
	if err != nil {
		return stdout, fmt.Errorf("container logs: %w (stderr: %s)", err, stderr)
	}
	return stdout, nil
}

func CleanupImage(cfg sshclient.Config, image string) error {
	if image == "" {
		image = mtgImage
	}
	cmd := fmt.Sprintf(`docker rmi -f %q 2>/dev/null || true`, image)
	_, stderr, err := sshclient.RunCommand(cfg, cmd)
	if err != nil {
		return fmt.Errorf("remove image: %w (stderr: %s)", err, stderr)
	}
	return nil
}

func parseResult(stdout string) (Result, error) {
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var result Result
		if err := json.Unmarshal([]byte(line), &result); err != nil {
			continue
		}
		if result.Secret == "" || result.ProxyLink == "" {
			return Result{}, fmt.Errorf("incomplete result json")
		}
		return result, nil
	}
	return Result{}, fmt.Errorf("no json result in output")
}

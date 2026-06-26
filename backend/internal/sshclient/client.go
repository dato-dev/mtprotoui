package sshclient

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/timurabdullin/mtprotoui/internal/crypto"
)

// HostKeyStore persists SSH host keys for trust-on-first-use (TOFU) verification.
type HostKeyStore interface {
	// GetHostKey returns the pinned key line for host:port, or "" if unknown.
	GetHostKey(hostport string) (string, error)
	// PutHostKey pins the key line for host:port on first contact.
	PutHostKey(hostport, keyLine string) error
}

type Config struct {
	Host        string
	Port        int
	User        string
	AuthType    string
	Credentials crypto.SSHCredentials
	// HostKeys enables TOFU host-key verification. If nil, host keys are not
	// verified (insecure) — always set it for real connections.
	HostKeys HostKeyStore
}

func (c Config) addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

func (c Config) clientConfig() (*ssh.ClientConfig, error) {
	var auths []ssh.AuthMethod

	switch c.AuthType {
	case "password":
		auths = append(auths, ssh.Password(c.Credentials.Password))
	case "key":
		var signer ssh.Signer
		var err error
		if c.Credentials.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(
				[]byte(c.Credentials.PrivateKey),
				[]byte(c.Credentials.Passphrase),
			)
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(c.Credentials.PrivateKey))
		}
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		auths = append(auths, ssh.PublicKeys(signer))
	default:
		return nil, fmt.Errorf("unsupported auth type: %s", c.AuthType)
	}

	return &ssh.ClientConfig{
		User:            c.User,
		Auth:            auths,
		HostKeyCallback: c.hostKeyCallback(),
		Timeout:         15 * time.Second,
	}, nil
}

// hostKeyCallback implements TOFU: the first key seen for a host:port is pinned
// in the store; later connections must present the same key or are rejected.
func (c Config) hostKeyCallback() ssh.HostKeyCallback {
	if c.HostKeys == nil {
		return ssh.InsecureIgnoreHostKey() //nolint:gosec // no store provided
	}
	addr := c.addr()
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		presented := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
		stored, err := c.HostKeys.GetHostKey(addr)
		if err != nil {
			return fmt.Errorf("read pinned host key: %w", err)
		}
		if stored == "" {
			if err := c.HostKeys.PutHostKey(addr, presented); err != nil {
				return fmt.Errorf("pin host key: %w", err)
			}
			return nil
		}
		if stored != presented {
			return fmt.Errorf(
				"host key mismatch for %s (possible MITM): pinned key differs from presented %s — remove the pinned key to re-trust",
				addr, ssh.FingerprintSHA256(key),
			)
		}
		return nil
	}
}

func TestConnection(cfg Config) error {
	client, err := connect(cfg)
	if err != nil {
		return err
	}
	return client.Close()
}

func connect(cfg Config) (*ssh.Client, error) {
	clientCfg, err := cfg.clientConfig()
	if err != nil {
		return nil, err
	}
	return ssh.Dial("tcp", cfg.addr(), clientCfg)
}

func RunScript(cfg Config, script string, args ...string) (stdout, stderr string, err error) {
	client, err := connect(cfg)
	if err != nil {
		return "", "", err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", "", err
	}
	defer session.Close()

	var outBuf, errBuf bytes.Buffer
	session.Stdout = &outBuf
	session.Stderr = &errBuf

	remoteCmd := fmt.Sprintf("bash -s -- %s", shellQuoteArgs(args))
	session.Stdin = bytes.NewBufferString(script)

	if err := session.Run(remoteCmd); err != nil {
		return outBuf.String(), errBuf.String(), fmt.Errorf("%w: %s", err, errBuf.String())
	}
	return outBuf.String(), errBuf.String(), nil
}

func RunCommand(cfg Config, command string) (stdout, stderr string, err error) {
	client, err := connect(cfg)
	if err != nil {
		return "", "", err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", "", err
	}
	defer session.Close()

	var outBuf, errBuf bytes.Buffer
	session.Stdout = &outBuf
	session.Stderr = &errBuf

	if err := session.Run(command); err != nil {
		return outBuf.String(), errBuf.String(), fmt.Errorf("%w: %s", err, errBuf.String())
	}
	return outBuf.String(), errBuf.String(), nil
}

func TCPReachable(host string, port int, timeout time.Duration) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), timeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func shellQuoteArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = fmt.Sprintf("%q", a)
	}
	result := quoted[0]
	for _, q := range quoted[1:] {
		result += " " + q
	}
	return result
}

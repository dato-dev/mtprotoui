package health

import (
	"context"
	"fmt"
	"time"

	probing "github.com/prometheus-community/pro-bing"

	"github.com/timurabdullin/mtprotoui/internal/sshclient"
)

const (
	StatusOnline   = "online"
	StatusDegraded = "degraded"
	StatusOffline  = "offline"
	StatusUnknown  = "unknown"

	CheckOK      = "ok"
	CheckFail    = "fail"
	CheckUnknown = "unknown"
)

type Result struct {
	Status     string   `json:"status"`
	PingStatus string   `json:"ping_status"`
	TCPStatus  string   `json:"tcp_status"`
	PingRTTMs  *float64 `json:"ping_rtt_ms,omitempty"`
}

func Check(host string, port int) Result {
	if port == 0 {
		port = 443
	}

	tcpOK := sshclient.TCPReachable(host, port, cfg.TCPTimeout)

	var pingOK bool
	var rtt *float64
	var pingKnown bool
	if cfg.PingEnabled {
		pingOK, rtt, pingKnown = pingHost(host)
	}

	tcpStatus := CheckFail
	if tcpOK {
		tcpStatus = CheckOK
	}

	pingStatus := CheckUnknown
	if pingKnown {
		if pingOK {
			pingStatus = CheckOK
		} else {
			pingStatus = CheckFail
		}
	}

	return Result{
		Status:     aggregateStatus(tcpOK, pingStatus),
		PingStatus: pingStatus,
		TCPStatus:  tcpStatus,
		PingRTTMs:  rtt,
	}
}

func aggregateStatus(tcpOK bool, pingStatus string) string {
	if tcpOK {
		return StatusOnline
	}
	if pingStatus == CheckOK {
		return StatusDegraded
	}
	if pingStatus == CheckFail {
		return StatusOffline
	}
	return StatusOffline
}

func pingHost(host string) (ok bool, rttMs *float64, known bool) {
	pinger, err := probing.NewPinger(host)
	if err != nil {
		return false, nil, false
	}

	pinger.Count = cfg.PingCount
	pinger.Timeout = cfg.PingTimeout
	pinger.SetPrivileged(true)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.PingTimeout+time.Second)
	defer cancel()

	if err := pinger.RunWithContext(ctx); err != nil {
		return false, nil, false
	}

	stats := pinger.Statistics()
	if stats.PacketsRecv == 0 {
		return false, nil, true
	}

	ms := float64(stats.AvgRtt.Milliseconds())
	if ms < 0 {
		ms = 0
	}
	return true, &ms, true
}

func FormatSummary(r Result) string {
	switch r.Status {
	case StatusOnline:
		if r.PingStatus == CheckOK && r.PingRTTMs != nil {
			return fmt.Sprintf("online (ping %.0fms, tcp ok)", *r.PingRTTMs)
		}
		return "online (tcp ok)"
	case StatusDegraded:
		return "degraded (host reachable, proxy port closed)"
	case StatusOffline:
		return "offline"
	default:
		return r.Status
	}
}

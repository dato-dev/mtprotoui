package health

import "time"

type Config struct {
	Interval    time.Duration
	PingEnabled bool
	PingCount   int
	PingTimeout time.Duration
	TCPTimeout  time.Duration
}

var cfg = Config{
	Interval:    5 * time.Minute,
	PingEnabled: true,
	PingCount:   3,
	PingTimeout: 4 * time.Second,
	TCPTimeout:  4 * time.Second,
}

func Configure(c Config) {
	if c.Interval > 0 {
		cfg.Interval = c.Interval
	}
	cfg.PingEnabled = c.PingEnabled
	if c.PingCount > 0 {
		cfg.PingCount = c.PingCount
	}
	if c.PingTimeout > 0 {
		cfg.PingTimeout = c.PingTimeout
	}
	if c.TCPTimeout > 0 {
		cfg.TCPTimeout = c.TCPTimeout
	}
}

func Interval() time.Duration {
	return cfg.Interval
}

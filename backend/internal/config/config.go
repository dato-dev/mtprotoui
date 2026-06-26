package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ListenAddr          string
	DatabasePath        string
	EncryptionKey       []byte
	JWTSecret           []byte
	AdminUser           string
	AdminPassword       string
	AdminPasswordReset  bool
	WhitelistURL        string
	HealthInterval      time.Duration
	SNIRotationInterval time.Duration
	PingEnabled         bool
	PingCount           int
	PingTimeout         time.Duration
	TCPTimeout          time.Duration
}

func Load() (Config, error) {
	encKeyB64 := os.Getenv("ENCRYPTION_KEY")
	if encKeyB64 == "" {
		return Config{}, fmt.Errorf("ENCRYPTION_KEY is required")
	}
	encKey, err := base64.StdEncoding.DecodeString(encKeyB64)
	if err != nil {
		return Config{}, fmt.Errorf("ENCRYPTION_KEY must be base64: %w", err)
	}
	if len(encKey) != 32 {
		return Config{}, fmt.Errorf("ENCRYPTION_KEY must decode to 32 bytes")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}

	adminUser := os.Getenv("ADMIN_USER")
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminUser == "" || adminPassword == "" {
		return Config{}, fmt.Errorf("ADMIN_USER and ADMIN_PASSWORD are required")
	}
	adminPasswordReset := boolFromEnv("ADMIN_PASSWORD_RESET", false)

	healthInterval := 5 * time.Minute
	if v := os.Getenv("HEALTH_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid HEALTH_INTERVAL: %w", err)
		}
		healthInterval = d
	}
	// 0 disables scheduled SNI rotation.
	sniRotationInterval := durationFromEnv("SNI_ROTATION_INTERVAL", 0)
	pingTimeout := durationFromEnv("PING_TIMEOUT", 4*time.Second)
	tcpTimeout := durationFromEnv("TCP_TIMEOUT", 4*time.Second)
	pingEnabled := boolFromEnv("PING_ENABLED", true)
	pingCount := intFromEnv("PING_COUNT", 3)
	if pingCount < 1 {
		return Config{}, fmt.Errorf("PING_COUNT must be at least 1")
	}

	listenAddr := ":8080"
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		listenAddr = v
	}

	dbPath := "/data/mtprotoui.db"
	if v := os.Getenv("DATABASE_PATH"); v != "" {
		dbPath = v
	}

	whitelistURL := "https://raw.githubusercontent.com/hxehex/russia-mobile-internet-whitelist/main/whitelist.txt"
	if v := os.Getenv("WHITELIST_URL"); v != "" {
		whitelistURL = v
	}

	return Config{
		ListenAddr:          listenAddr,
		DatabasePath:        dbPath,
		EncryptionKey:       encKey,
		JWTSecret:           []byte(jwtSecret),
		AdminUser:           adminUser,
		AdminPassword:       adminPassword,
		AdminPasswordReset:  adminPasswordReset,
		WhitelistURL:        whitelistURL,
		HealthInterval:      healthInterval,
		SNIRotationInterval: sniRotationInterval,
		PingEnabled:         pingEnabled,
		PingCount:           pingCount,
		PingTimeout:         pingTimeout,
		TCPTimeout:          tcpTimeout,
	}, nil
}

func durationFromEnv(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func boolFromEnv(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func intFromEnv(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func PortFromEnv(key string, defaultPort int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultPort
	}
	p, err := strconv.Atoi(v)
	if err != nil {
		return defaultPort
	}
	return p
}

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/timurabdullin/mtprotoui/internal/api"
	"github.com/timurabdullin/mtprotoui/internal/auth"
	"github.com/timurabdullin/mtprotoui/internal/config"
	"github.com/timurabdullin/mtprotoui/internal/health"
	"github.com/timurabdullin/mtprotoui/internal/store"
	"github.com/timurabdullin/mtprotoui/internal/whitelist"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	st, err := store.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()

	authSvc := auth.New(st, cfg.JWTSecret)
	if err := authSvc.EnsureAdmin(context.Background(), cfg.AdminUser, cfg.AdminPassword); err != nil {
		log.Fatalf("admin: %v", err)
	}
	if cfg.AdminPasswordReset {
		if err := authSvc.ResetPassword(context.Background(), cfg.AdminUser, cfg.AdminPassword); err != nil {
			log.Fatalf("admin password reset: %v", err)
		}
		log.Printf("admin password reset for %q from ADMIN_PASSWORD (set ADMIN_PASSWORD_RESET=false to disable)", cfg.AdminUser)
	}

	wl := whitelist.New(cfg.WhitelistURL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wl.Start(ctx)
	log.Printf("whitelist: %d domains loaded", wl.Count())

	health.Configure(health.Config{
		Interval:    cfg.HealthInterval,
		PingEnabled: cfg.PingEnabled,
		PingCount:   cfg.PingCount,
		PingTimeout: cfg.PingTimeout,
		TCPTimeout:  cfg.TCPTimeout,
	})
	log.Printf("health: interval=%s ping=%v count=%d timeout=%s",
		cfg.HealthInterval, cfg.PingEnabled, cfg.PingCount, cfg.PingTimeout)

	healthWorker := health.New(st, health.Interval())
	go healthWorker.Start(ctx)

	handler := api.NewHandler(st, authSvc, cfg.EncryptionKey, wl)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	handler.Routes(mux)

	var root http.Handler = authSvc.Middleware(mux)
	root = corsMiddleware(root)

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           root,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", cfg.ListenAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

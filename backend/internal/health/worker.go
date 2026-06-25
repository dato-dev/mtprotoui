package health

import (
	"context"
	"log"
	"time"

	"github.com/timurabdullin/mtprotoui/internal/store"
)

type Worker struct {
	store    *store.Store
	interval time.Duration
}

func New(s *store.Store, interval time.Duration) *Worker {
	return &Worker{store: s, interval: interval}
}

func (w *Worker) Start(ctx context.Context) {
	w.runOnce(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *Worker) runOnce(ctx context.Context) {
	servers, err := w.store.ListServers(ctx)
	if err != nil {
		log.Printf("health: list servers: %v", err)
		return
	}

	now := time.Now().UTC()
	for _, srv := range servers {
		if srv.DeployStatus != "ready" {
			continue
		}
		result := Check(srv.Host, srv.MTProtoPort)
		if err := w.store.UpdateServerHealth(ctx, srv.ID, result.Status, result.PingStatus, result.TCPStatus, result.PingRTTMs, now); err != nil {
			log.Printf("health: update %s: %v", srv.ID, err)
		}
	}
}

package api

import (
	"context"
	"log"
	"time"

	"github.com/timurabdullin/mtprotoui/internal/deploy"
	"github.com/timurabdullin/mtprotoui/internal/operation"
)

// StartSNIRotation periodically re-deploys servers that opted into SNI rotation,
// picking a fresh whitelist domain each time. A zero interval disables it.
func (h *Handler) StartSNIRotation(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.rotateDueServers(ctx)
			}
		}
	}()
}

func (h *Handler) rotateDueServers(ctx context.Context) {
	servers, err := h.store.ListServers(ctx)
	if err != nil {
		log.Printf("sni rotation: list servers: %v", err)
		return
	}
	for _, srv := range servers {
		// Only ready servers that use SNI and opted in are eligible.
		if !srv.RotateSNI || srv.DeployStatus != "ready" {
			continue
		}
		if !deploy.UsesSNI(srv.ProxyType, srv.FakeTLS) {
			continue
		}

		log.Printf("sni rotation: rotating server %s (%s)", srv.Name, srv.ID)
		_ = h.store.UpdateServerDeploy(ctx, srv.ID, srv.SNIDomain, srv.Secret, srv.ProxyLink, srv.MTProtoPort, "deploying", "")
		h.setDeployStep(ctx, srv.ID, deployStepByKey(operation.DeployPreparing))
		// Synchronous so rotations run one at a time; runDeploy holds its own lock.
		h.runDeploy(srv.ID, true)
		h.addAudit("system", "rotate-sni", srv.ID, srv.Name, "scheduled SNI rotation")
	}
}

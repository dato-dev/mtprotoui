package api

import (
	"context"

	"github.com/timurabdullin/mtprotoui/internal/operation"
)

func (h *Handler) setOperationProgress(ctx context.Context, id, deployStatus, opType string, step operation.Step) {
	_ = h.store.UpdateServerOperation(ctx, id, deployStatus, opType, step.Step, step.Message, step.Progress)
}

func (h *Handler) clearOperation(ctx context.Context, id, deployStatus string) {
	_ = h.store.ClearServerOperation(ctx, id, deployStatus)
}

func (h *Handler) setDeployStep(ctx context.Context, id string, step operation.Step) {
	h.setOperationProgress(ctx, id, "deploying", operation.TypeDeploy, step)
}

func (h *Handler) setDeleteStep(ctx context.Context, id string, step operation.Step) {
	h.setOperationProgress(ctx, id, "deleting", operation.TypeDelete, step)
}

func deployStepByKey(key string) operation.Step {
	for _, s := range operation.DeploySteps() {
		if s.Step == key {
			return s
		}
	}
	return operation.Step{Step: key, Message: key, Progress: 0}
}

func deleteStepByKey(key string) operation.Step {
	for _, s := range operation.DeleteSteps() {
		if s.Step == key {
			return s
		}
	}
	return operation.Step{Step: key, Message: key, Progress: 0}
}

func (h *Handler) markDeployFailed(ctx context.Context, id, sni, errMsg string) {
	_ = h.store.SetServerDeployFailed(ctx, id, sni, errMsg)
}

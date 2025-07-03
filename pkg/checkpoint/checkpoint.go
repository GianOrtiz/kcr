package checkpoint

import (
	"context"
)

type CheckpointService interface {
	Checkpoint(podNode, podID, podNamespace, containerName string, ctx context.Context) (string, error)
}

package imagebuilder

import (
	"context"
)

type ImageBuilder interface {
	BuildFromCheckpoint(checkpointLocation string, imageName string, ctx context.Context) error
}

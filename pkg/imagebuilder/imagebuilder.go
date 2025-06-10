package imagebuilder

import (
	"context"
)

type ImageBuilder interface {
	BuildFromCheckpoint(checkpointLocation, containerName, imageName string, ctx context.Context) error
	PushToNodeRuntime(ctx context.Context, localImageName string, runtimeImageName string) error
}

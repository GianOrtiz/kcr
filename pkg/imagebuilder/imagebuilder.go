package imagebuilder

import (
	"context"
)

type RegistryBasicAuth struct {
	Username string
	Password string
}

type RegistryAuth struct {
	Basic    *RegistryBasicAuth
	AuthFile *string
	URL      string
}

type ImageBuilder interface {
	BuildFromCheckpoint(checkpointLocation, containerName, imageName string, ctx context.Context) error
	PushToNodeRuntime(ctx context.Context, localImageName string, runtimeImageName string) error
}

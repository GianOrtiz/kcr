package imagebuilder

import (
	"context"
	"fmt"
	"os"

	"github.com/containers/buildah"
	is "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type BuildahImageBuilder struct {
	buildStore   storage.Store
	registryAuth RegistryAuth
}

func NewBuildahImageBuilder(registryAuth RegistryAuth) (ImageBuilder, error) {
	buildStorageOptions, err := storage.DefaultStoreOptions()
	if err != nil {
		return nil, err
	}
	buildStore, err := storage.GetStore(buildStorageOptions)
	if err != nil {
		return nil, err
	}
	return BuildahImageBuilder{
		buildStore:   buildStore,
		registryAuth: registryAuth,
	}, nil
}

func (b BuildahImageBuilder) BuildFromCheckpoint(
	checkpointLocation, containerName, imageName string, ctx context.Context,
) error {
	log := log.FromContext(ctx)

	builderOptions := buildah.BuilderOptions{
		FromImage: "scratch",
	}

	buildahImageName := "localhost/" + imageName

	builder, err := buildah.NewBuilder(ctx, b.buildStore, builderOptions)
	if err != nil {
		return err
	}

	err = builder.Add("/", true, buildah.AddAndCopyOptions{}, checkpointLocation)
	if err != nil {
		return err
	}
	builder.SetAnnotation("io.kubernetes.cri-o.annotations.checkpoint.name", containerName)
	log.Info("Successfully added the checkpoint file to the builder")

	imageRef, err := is.Transport.ParseStoreReference(b.buildStore, buildahImageName)
	if err != nil {
		return err
	}

	_, _, _, err = builder.Commit(ctx, imageRef, buildah.CommitOptions{})
	return err
}

func (b BuildahImageBuilder) PushToNodeRuntime(
	ctx context.Context, localImageName string, runtimeImageName string,
) error {
	logger := log.FromContext(ctx)
	destinationSpec := "docker://" + b.registryAuth.URL + "/" + runtimeImageName
	imageReference, err := alltransports.ParseImageName(destinationSpec)
	if err != nil {
		logger.Error(err, "Failed to parse destination spec", "destination", destinationSpec)
		return fmt.Errorf("failed to parse destination spec %s: %w", destinationSpec, err)
	}

	_, err = is.Transport.ParseStoreReference(b.buildStore, localImageName)
	if err != nil {
		logger.Error(err, "Local image not found in store, cannot push", "imageName", localImageName)
		return fmt.Errorf("local image %s not found for push: %w", localImageName, err)
	}

	systemContext := types.SystemContext{
		DockerInsecureSkipTLSVerify: types.OptionalBoolTrue,
	}
	if b.registryAuth.Basic != nil {
		systemContext.DockerAuthConfig = &types.DockerAuthConfig{
			Username: b.registryAuth.Basic.Username,
			Password: b.registryAuth.Basic.Password,
		}
	} else if b.registryAuth.AuthFile != nil {
		systemContext.AuthFilePath = *b.registryAuth.AuthFile
	}

	options := buildah.PushOptions{
		Store:         b.buildStore,
		ReportWriter:  os.Stderr,
		SystemContext: &systemContext,
	}

	_, _, err = buildah.Push(ctx, localImageName, imageReference, options)
	if err != nil {
		logger.Error(err, "Failed to push image to node runtime", "imageName", localImageName, "destination", destinationSpec)
		return fmt.Errorf("failed to push image %s to %s: %w", localImageName, destinationSpec, err)
	}

	logger.Info("Successfully pushed image to local runtime", "imageName", localImageName, "destination", destinationSpec)

	return nil
}

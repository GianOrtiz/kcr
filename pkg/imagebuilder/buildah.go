package imagebuilder

import (
	"context"

	"github.com/containers/buildah"
	is "github.com/containers/image/v5/storage"
	"github.com/containers/storage"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type BuildahImageBuilder struct {
	buildStore storage.Store
}

func NewBuildahImageBuilder() (ImageBuilder, error) {
	buildStorageOptions, err := storage.DefaultStoreOptions()
	if err != nil {
		return nil, err
	}
	buildStore, err := storage.GetStore(buildStorageOptions)
	if err != nil {
		return nil, err
	}
	return BuildahImageBuilder{
		buildStore: buildStore,
	}, nil
}

func (b BuildahImageBuilder) BuildFromCheckpoint(checkpointLocation string, imageName string, ctx context.Context) error {
	log := log.FromContext(ctx)

	builderOptions := buildah.BuilderOptions{
		FromImage: "scratch",
	}

	builder, err := buildah.NewBuilder(ctx, b.buildStore, builderOptions)
	if err != nil {
		return err
	}
	log.Info("Successfully retrieved a builder")

	err = builder.Add(checkpointLocation, true, buildah.AddAndCopyOptions{}, ".")
	if err != nil {
		return err
	}
	log.Info("Successfully added the checkpoint file to the builder")

	imageRef, err := is.Transport.ParseStoreReference(b.buildStore, imageName)
	if err != nil {
		return err
	}
	log.Info("Successfully created the image reference")

	_, _, _, err = builder.Commit(ctx, imageRef, buildah.CommitOptions{})
	log.Info("Successfully created the image")
	return err
}

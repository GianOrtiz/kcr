package imagebuilder

import (
	"context"

	"github.com/containers/buildah"
	is "github.com/containers/image/v5/storage"
	"github.com/containers/storage"
)

type ImageBuilder interface {
	BuildFromCheckpoint(checkpointLocation string, imageName string, ctx context.Context) error
}

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
	builderOptions := buildah.BuilderOptions{
		FromImage: "scratch",
	}

	builder, err := buildah.NewBuilder(ctx, b.buildStore, builderOptions)
	if err != nil {
		return err
	}

	err = builder.Add(checkpointLocation, true, buildah.AddAndCopyOptions{}, ".")
	if err != nil {
		return err
	}

	imageRef, err := is.Transport.ParseStoreReference(b.buildStore, imageName)
	if err != nil {
		return err
	}

	_, _, _, err = builder.Commit(ctx, imageRef, buildah.CommitOptions{})
	return nil
}

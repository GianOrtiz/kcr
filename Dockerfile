# Build the manager binary
FROM docker.io/golang:1.23 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY internal/ internal/
COPY pkg/ pkg/

# Install packages required for buildah.
RUN apt update -y && apt upgrade -y
RUN apt install btrfs-progs golang-github-containerd-btrfs-dev libgpgme-dev passt -y

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=1 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager cmd/main.go

# We are not using the distroless image as we still need to have C binaries in order for buildah to work.
# Later we are going to split the buildah image into a separate image.
FROM debian:bookworm

# Install packages required for buildah.
# These were specified as needed for buildah and were present in the builder stage.
# If CGO_ENABLED=1 linked against .so files from these -dev packages, they are needed.
RUN apt-get update -y && \
    apt-get install -y --no-install-recommends \
    btrfs-progs \
    libc6 \
    passt \
    golang-github-containerd-btrfs-dev \
    libgpgme-dev && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /
COPY --from=builder /workspace/manager .

ENTRYPOINT ["/manager"]

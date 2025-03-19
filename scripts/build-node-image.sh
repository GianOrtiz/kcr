#!/bin/bash

usage() {
    echo "Fail"
    echo "Usage: K8S_VERSION=v1.30.0 CRIO_VERSION=v1.30 ./scripts/build-node-image.sh"
    exit 1
}

if [ -z "$K8S_VERSION" ]; then
    usage
fi

if [ -z "$CRIO_VERSION" ]; then
    usage
fi

# Build the base image.
KIND_DIR=$(mktemp -d)
git clone git@github.com:kubernetes-sigs/kind.git "$KIND_DIR"
cd "$KIND_DIR"/images/base
# Extract the base image tag from make quick output
BASE_IMAGE=$(make quick 2>&1 | grep "docker buildx build" | grep -o "gcr.io/k8s-staging-kind/base:[^ ]*")
if [ -z "$BASE_IMAGE" ]; then
    echo "Failed to extract base image tag from make quick output"
    exit 1
fi
cd -

rm -rf "$KIND_DIR"

# Clone kubernetes repository in order for kind to work while building node image.
KUBERNETES_PATH="$GOPATH/src/k8s.io/kubernetes"
if [ -d "$KUBERNETES_PATH" ]; then
    rm -rf "$KUBERNETES_PATH"
fi
mkdir -p "$KUBERNETES_PATH"
git clone --depth 1 --branch ${K8S_VERSION} https://github.com/kubernetes/kubernetes.git "$KUBERNETES_PATH"

# Build the node image.
kind build node-image --base-image ${BASE_IMAGE}

# Build the final kind image with CRIU.
docker build --build-arg CRIO_VERSION=$CRIO_VERSION -t kindnode/criu:$CRIO_VERSION -f kind-criu.Dockerfile .

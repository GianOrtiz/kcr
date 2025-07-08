#!/bin/bash

if [ -z $TAG ]; then
    export TAG=latest
fi

rm image.tar
docker save controller:latest -o image.tar
docker cp image.tar kind-control-plane:/image.tar
docker cp image.tar kind-worker:/image.tar
docker exec kind-worker skopeo copy docker-archive:/image.tar containers-storage:controller:$TAG
docker exec kind-control-plane skopeo copy docker-archive:/image.tar containers-storage:controller:$TAG
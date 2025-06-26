# Development guide

To develop this application you must install a few packages:

1. `buildah` 1.40.x
2. `docker` 28.2.x
3. `kind` v0.27.x

Then, you must build a custom image of the cluster in order to allow the checkpoint work with `cri-o` as the container runtime using the script at `scripts/build-node-image.sh`. This will create a custom image for kind to run.

After creating the custom image and installing the packages we can run the project with the following steps:

1. Start the cluster and a local docker registry with `scripts/cluster-up.sh`.
2. Install custom resource definitions and service accounts with `make install`.
3. Deploy a deployment application that works with our operator like the one at `examples/deployment.yaml`.
4. Expose the Kind Kubernetes API with `kubectl proxy`.
5. Configure run arguments trough the `RUN_ARGS`, I use this `RUN_ARGS="--kubernetes-api-address=http://127.0.0.1:8001 --checkpoints-directory=/home/gian/prog/kcr/checkpoints"`.
6. Start the operator with `sudo -E make run`. We did not worked out a way to run this application without root as we need to access a protected directory. The `-E` flag uses the current environment to sudo.

You should have a cluster up and running that will checkpoint the example application every 1min.

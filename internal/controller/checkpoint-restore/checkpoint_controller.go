/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package checkpointrestore

import (
	"context"
	"path/filepath"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	checkpointrestorev1 "github.com/GianOrtiz/kcr/api/checkpoint-restore/v1"
	"github.com/GianOrtiz/kcr/pkg/imagebuilder"
)

// CheckpointReconciler reconciles a Checkpoint object
type CheckpointReconciler struct {
	client.Client
	Scheme               *runtime.Scheme
	ImageBuilder         imagebuilder.ImageBuilder
	CheckpointsDirectory string
}

// +kubebuilder:rbac:groups=checkpoint-restore.kcr.io,resources=checkpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=checkpoint-restore.kcr.io,resources=checkpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=checkpoint-restore.kcr.io,resources=checkpoints/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *CheckpointReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	const (
		imageBuiltPhase = "ImageBuilt"
		failedPhase     = "Failed"
		processingPhase = "Processing"
	)
	log := log.FromContext(ctx)

	var checkpoint checkpointrestorev1.Checkpoint
	if err := r.Get(ctx, req.NamespacedName, &checkpoint); err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch Checkpoint")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Image is already processed, it should not be processed again.
	if checkpoint.Status.Phase == imageBuiltPhase || checkpoint.Status.Phase == failedPhase {
		return ctrl.Result{}, nil
	}

	// Image build process is in processing phase, we can reeschedule this reconcile loop to check the status.
	if checkpoint.Status.Phase == processingPhase {
		return ctrl.Result{Requeue: true}, nil
	}

	checkpointFile := checkpoint.Spec.CheckpointData
	checkpointFilePath := filepath.Join(r.CheckpointsDirectory, checkpointFile)
	checkpointImage := "checkpoint-" + checkpoint.Name
	if err := r.ImageBuilder.BuildFromCheckpoint(
		checkpointFilePath, checkpoint.Spec.ContainerName, checkpointImage, ctx,
	); err != nil {
		log.Error(err, "unable to build image from checkpoint")
		checkpoint.Status.Phase = failedPhase
		checkpoint.Status.FailedReason = err.Error()
		if err := r.Status().Update(ctx, &checkpoint); err != nil {
			log.Error(err, "unable to update checkpoint status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	runtimeImageName := "checkpoint-" + checkpoint.Name + ":latest"
	if err := r.ImageBuilder.PushToNodeRuntime(ctx, checkpointImage, runtimeImageName); err != nil {
		log.Error(err, "unable to push image from checkpoint")
		checkpoint.Status.Phase = failedPhase
		checkpoint.Status.FailedReason = err.Error()
		if err := r.Status().Update(ctx, &checkpoint); err != nil {
			log.Error(err, "unable to update checkpoint status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	checkpoint.Status.Phase = imageBuiltPhase
	checkpoint.Status.CheckpointImage = checkpointImage
	checkpoint.Status.RuntimeImage = runtimeImageName
	if err := r.Status().Update(ctx, &checkpoint); err != nil {
		log.Error(err, "unable to update checkpoint status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CheckpointReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&checkpointrestorev1.Checkpoint{}).
		Named("checkpoint-restore-checkpoint").
		Complete(r)
}

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
	"fmt"
	"os"
	"regexp"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	checkpointrestorev1 "github.com/GianOrtiz/kcr/api/checkpoint-restore/v1"
	"github.com/GianOrtiz/kcr/pkg/imagebuilder"
)

var checkpointEstimatePathRegex = regexp.MustCompile(`^/var/lib/kubelet/checkpoints/checkpoint-.+-(\d+)\.tar$`)
var checkpointPathRegex = regexp.MustCompile(`^/var/lib/kubelet/checkpoints/checkpoint-.+-((-?(?:[1-9][0-9]*)?[0-9]{4})-(1[0-2]|0[1-9])-(3[01]|0[1-9]|[12][0-9])T(2[0-3]|[01][0-9]):([0-5][0-9]):([0-5][0-9])(\.[0-9]+)?(Z|[+-](?:2[0-3]|[01][0-9]):[0-5][0-9])?)\.tar$`)

// CheckpointReconciler reconciles a Checkpoint object
type CheckpointReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	ImageBuilder imagebuilder.ImageBuilder
}

// +kubebuilder:rbac:groups=checkpoint-restore.kcr.io,resources=checkpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=checkpoint-restore.kcr.io,resources=checkpoints/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=checkpoint-restore.kcr.io,resources=checkpoints/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *CheckpointReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
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
	if checkpoint.Status.Phase == "ImageBuilt" || checkpoint.Status.Phase == "Failed" {
		return ctrl.Result{}, nil
	}

	// Image build process is in processing phase, we can reeschedule this reconcile loop to check the status.
	if checkpoint.Status.Phase == "Processing" {
		return ctrl.Result{Requeue: true}, nil
	}

	checkpointPath := checkpoint.Spec.CheckpointData
	matches := checkpointEstimatePathRegex.FindStringSubmatch(checkpointPath)

	if len(matches) < 2 {
		err := fmt.Errorf("checkpointData path %q does not match expected pattern %q", checkpointPath, checkpointEstimatePathRegex.String())
		log.Error(err, "Invalid checkpoint data path format")

		checkpoint.Status.Phase = "Failed"
		checkpoint.Status.FailedReason = err.Error()
		now := metav1.Now()
		checkpoint.Status.LastTransitionTime = &now
		if updateErr := r.Status().Update(ctx, &checkpoint); updateErr != nil {
			log.Error(updateErr, "Failed to update Checkpoint status to Failed for invalid path")
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, nil
	}

	timestamp, err := strconv.Atoi(matches[1])
	if err != nil {
		log.Error(err, "unable to convert timestamp to int")
		checkpoint.Status.Phase = "Failed"
		checkpoint.Status.FailedReason = err.Error()
		now := metav1.Now()
		checkpoint.Status.LastTransitionTime = &now
		if updateErr := r.Status().Update(ctx, &checkpoint); updateErr != nil {
			log.Error(updateErr, "failed to update Checkpoint status to Failed for invalid path timestamp")
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, nil
	}
	log.Info("Extracted timestamp from checkpoint path", "timestamp", timestamp)

	// Get all checkpoints and matches the required closer timestamp of the files so we can get the closest checkpoint.
	checkpointFiles, err := os.ReadDir("/var/lib/kubelet/checkpoints")
	if err != nil {
		log.Error(err, "unable to read checkpoints directory")
		checkpoint.Status.Phase = "Failed"
		checkpoint.Status.FailedReason = "unable to read checkpoints directory" + err.Error()
		if err := r.Status().Update(ctx, &checkpoint); err != nil {
			log.Error(err, "unable to update checkpoint status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	var checkpointClosestFile string
	for _, checkpointFile := range checkpointFiles {
		if isDir := checkpointFile.IsDir(); !isDir {
			filename := "/var/lib/kubelet/checkpoints/" + checkpointFile.Name()
			checkpointTimestampMatches := checkpointPathRegex.FindStringSubmatch(filename)
			if len(matches) < 2 {
				err := fmt.Errorf("checkpointData path %q does not match expected pattern %q", filename, checkpointPathRegex.String())
				log.Error(err, "Invalid checkpoint data path format")

				checkpoint.Status.Phase = "Failed"
				checkpoint.Status.FailedReason = err.Error()
				now := metav1.Now()
				checkpoint.Status.LastTransitionTime = &now
				if updateErr := r.Status().Update(ctx, &checkpoint); updateErr != nil {
					log.Error(updateErr, "Failed to update Checkpoint status to Failed for invalid path")
					return ctrl.Result{}, updateErr
				}
				return ctrl.Result{}, nil
			}

			checkpointTimestamp, err := time.Parse(time.RFC3339, checkpointTimestampMatches[1])
			if err != nil {
				log.Error(err, "unable to parse checkpoint timestamp")
				checkpoint.Status.Phase = "Failed"
				checkpoint.Status.FailedReason = err.Error()
				now := metav1.Now()
				checkpoint.Status.LastTransitionTime = &now
				if updateErr := r.Status().Update(ctx, &checkpoint); updateErr != nil {
					log.Error(updateErr, "Failed to update Checkpoint status to Failed for invalid path")
					return ctrl.Result{}, updateErr
				}
				return ctrl.Result{}, nil
			}

			differenceOfTime := (int)(checkpointTimestamp.Unix()) - timestamp
			differenceIsCloser := differenceOfTime <= 60 || differenceOfTime >= -60
			if differenceIsCloser {
				checkpointClosestFile = "/var/lib/kubelet/checkpoints/" + checkpointFile.Name()
				break
			}
		}
	}

	if err := r.ImageBuilder.BuildFromCheckpoint(checkpointClosestFile, "docker.io/gianortiz/checkpoint", ctx); err != nil {
		log.Error(err, "unable to build image from checkpoint")
		checkpoint.Status.Phase = "Failed"
		checkpoint.Status.FailedReason = err.Error()
		if err := r.Status().Update(ctx, &checkpoint); err != nil {
			log.Error(err, "unable to update checkpoint status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	checkpoint.Status.Phase = "ImageBuilt"
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

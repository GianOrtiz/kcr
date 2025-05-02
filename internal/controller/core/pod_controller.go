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

package core

import (
	"context"

	checkpointrestorev1 "github.com/GianOrtiz/kcr/api/checkpoint-restore/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PodReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		log.Error(err, "unable to fetch Pod")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if pod.Status.Phase != corev1.PodFailed {
		log.Info("Pod has not failed, ignoring")
		return ctrl.Result{}, nil
	}

	log.Info("Pod has failed")
	var checkpoints checkpointrestorev1.CheckpointList
	if err := r.List(ctx, &checkpoints, client.InNamespace(pod.Namespace), client.MatchingLabels(map[string]string{"pod": pod.Name})); err != nil {
		log.Error(err, "unable to list Checkpoints")
		return ctrl.Result{}, err
	}

	var oldestCheckpoint *checkpointrestorev1.Checkpoint
	for _, checkpoint := range checkpoints.Items {
		if oldestCheckpoint == nil {
			oldestCheckpoint = &checkpoint
		} else {
			if oldestCheckpoint.ObjectMeta.CreationTimestamp.Before(&checkpoint.ObjectMeta.CreationTimestamp) {
				oldestCheckpoint = &checkpoint
			}
		}
	}

	pod.Spec.Containers[0].Image = oldestCheckpoint.Status.CheckpointImage
	if err := r.Update(ctx, &pod); err != nil {
		log.Error(err, "unable to update Pod")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Named("core-pod").
		Complete(r)
}

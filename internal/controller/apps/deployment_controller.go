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

package controller

import (
	"context"
	"errors"
	"regexp"

	checkpointrestorev1 "github.com/GianOrtiz/kcr/api/checkpoint-restore/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const CHECKPOINT_RESTORE_SCHEDULE_ANNOTATION = "kcr.io/checkpoint-restore-schedule"

// DeploymentReconciler reconciles a Deployment object
type DeploymentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Deployment object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.2/pkg/reconcile
func (r *DeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var deployment appsv1.Deployment
	if err := r.Get(ctx, req.NamespacedName, &deployment); err != nil {
		log.Error(err, "unable to get Deployment")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	checkpointRestoreScheduleAnnotation, ok := deployment.Annotations[CHECKPOINT_RESTORE_SCHEDULE_ANNOTATION]
	if !ok {
		log.Info("not monitoring deployment as it is not annotated")
		return ctrl.Result{}, nil
	}

	cronRegex := "(@(annually|yearly|monthly|weekly|daily|hourly|reboot))|(@every (\\d+(ns|us|µs|ms|s|m|h))+)|((((\\d+,)+\\d+|(\\d+(\\/|-)\\d+)|\\d+|\\*) ?){5,7})"
	matched, err := regexp.MatchString(cronRegex, checkpointRestoreScheduleAnnotation)
	if err != nil {
		log.Error(err, "unable to parse checkpoint schedule")
		return ctrl.Result{}, err
	}

	if !matched {
		err = errors.New("checkpoint restore schedule annotation does not match a proper cron schedule")
		log.Error(err, "unable to parse the schedule")
		return ctrl.Result{}, err
	}

	var checkpointSchedule checkpointrestorev1.CheckpointSchedule
	err = r.Get(ctx, client.ObjectKey{Namespace: deployment.Namespace, Name: deployment.Name}, &checkpointSchedule)
	if err == nil {
		// Update the CheckpointSchedule
		checkpointSchedule.Spec.Schedule = checkpointRestoreScheduleAnnotation
		checkpointSchedule.Spec.Selector = *deployment.Spec.Selector
		if err := r.Update(ctx, &checkpointSchedule); err != nil {
			log.Error(err, "failed to update CheckpointSchedule")
			return ctrl.Result{Requeue: true}, err
		}
	}

	// Create the CheckpointSchedule as it does not exist yet.
	checkpointSchedule = checkpointrestorev1.CheckpointSchedule{
		ObjectMeta: v1.ObjectMeta{
			Name:      deployment.Name,
			Namespace: deployment.Namespace,
		},
		Spec: checkpointrestorev1.CheckpointScheduleSpec{
			Selector: *deployment.Spec.Selector,
			Schedule: checkpointRestoreScheduleAnnotation,
		},
		Status: checkpointrestorev1.CheckpointScheduleStatus{Running: false},
	}
	if err := r.Create(ctx, &checkpointSchedule); err != nil {
		log.Error(err, "failed to create CheckpointSchedule")
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appsv1.Deployment{}).
		Named("deployment").
		Complete(r)
}

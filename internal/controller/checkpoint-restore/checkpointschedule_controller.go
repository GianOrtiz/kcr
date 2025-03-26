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
	"time"

	"github.com/robfig/cron"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	checkpointrestorev1 "github.com/GianOrtiz/kcr/api/checkpoint-restore/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CheckpointScheduleReconciler reconciles a CheckpointSchedule object
type CheckpointScheduleReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	CronJobs []*cron.Cron
}

// +kubebuilder:rbac:groups=checkpoint-restore.kcr.io,resources=checkpointschedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=checkpoint-restore.kcr.io,resources=checkpointschedules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=checkpoint-restore.kcr.io,resources=checkpointschedules/finalizers,verbs=update
// +kubebuilder:rbac:groups=checkpoint-restore.kcr.io,resources=checkpointrequests,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *CheckpointScheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	var checkpointSchedule checkpointrestorev1.CheckpointSchedule
	if err := r.Get(ctx, req.NamespacedName, &checkpointSchedule); err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch CheckpointSchedule")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Parse the schedule into a cron expression
	schedule := checkpointSchedule.Spec.Schedule
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := parser.Parse(schedule); err != nil {
		log.Error(err, "failed to parse schedule", "schedule", schedule)
		return ctrl.Result{}, err
	}

	cronJob := cron.New()
	r.CronJobs = append(r.CronJobs, cronJob)
	cronJob.AddFunc(schedule, func() {
		checkpointCtx := context.Background()

		// Get updated schedule to ensure it still exists and hasn't changed
		var currentSchedule checkpointrestorev1.CheckpointSchedule
		if err := r.Get(checkpointCtx, req.NamespacedName, &currentSchedule); err != nil {
			log.Error(err, "failed to get current schedule")
			return
		}

		// Verify schedule hasn't changed
		if currentSchedule.Spec.Schedule != schedule {
			log.Info("schedule has changed, not executing checkpoint")
			return
		}

		// Get pods matching selector
		var podList corev1.PodList
		if err := r.List(checkpointCtx, &podList, &client.ListOptions{
			LabelSelector: labels.SelectorFromSet(currentSchedule.Spec.Selector.MatchLabels),
			Namespace:     req.Namespace,
		}); err != nil {
			log.Error(err, "failed to list pods")
			return
		}

		if len(podList.Items) == 0 {
			log.Info("no pods found matching selector", "selector", currentSchedule.Spec.Selector)
			return
		}

		pod := podList.Items[0]
		log.Info("creating checkpoint request for pod", "pod", pod.Name)

		// Create a CheckpointRequest resource
		checkpointRequest := &checkpointrestorev1.CheckpointRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s-%d", currentSchedule.Name, pod.Name, time.Now().Unix()),
				Namespace: req.Namespace,
				Labels: map[string]string{
					"app":           "checkpoint-restore",
					"pod":           pod.Name,
					"pod-ns":        pod.Namespace,
					"schedule-name": currentSchedule.Name,
				},
			},
			Spec: checkpointrestorev1.CheckpointRequestSpec{
				PodReference: checkpointrestorev1.PodReference{
					Name:      pod.Name,
					Namespace: pod.Namespace,
				},
				ContainerName: pod.Spec.Containers[0].Name,
				CheckpointScheduleRef: &corev1.ObjectReference{
					Kind:       "CheckpointSchedule",
					Name:       currentSchedule.Name,
					Namespace:  currentSchedule.Namespace,
					UID:        currentSchedule.UID,
					APIVersion: currentSchedule.APIVersion,
				},
			},
			Status: checkpointrestorev1.CheckpointRequestStatus{
				Phase: "Pending",
			},
		}

		// Set the controller reference to the CheckpointSchedule
		if err := ctrl.SetControllerReference(&currentSchedule, checkpointRequest, r.Scheme); err != nil {
			log.Error(err, "failed to set controller reference for CheckpointRequest")
			return
		}

		// Create the CheckpointRequest resource
		if err := r.Create(checkpointCtx, checkpointRequest); err != nil {
			log.Error(err, "failed to create CheckpointRequest resource")
			return
		}
		log.Info("created checkpoint request", "checkpointRequest", checkpointRequest.Name)

		// Update the CheckpointSchedule status with the last run time
		currentSchedule.Status.LastRunTime = &metav1.Time{Time: time.Now()}
		if err := r.Status().Update(checkpointCtx, &currentSchedule); err != nil {
			log.Error(err, "failed to update CheckpointSchedule status")
			return
		}
	})
	cronJob.Start()

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CheckpointScheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&checkpointrestorev1.CheckpointSchedule{}).
		Named("checkpoint-restore-checkpointschedule").
		Complete(r)
}

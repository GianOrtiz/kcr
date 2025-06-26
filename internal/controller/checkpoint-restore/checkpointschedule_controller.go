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

	"github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	checkpointrestorev1 "github.com/GianOrtiz/kcr/api/checkpoint-restore/v1"
)

// CheckpointScheduleReconciler reconciles a CheckpointSchedule object
type CheckpointScheduleReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	CronJobs map[string]*cron.Cron
}

func NewCheckpointScheduleReconciler(client client.Client, scheme *runtime.Scheme) *CheckpointScheduleReconciler {
	return &CheckpointScheduleReconciler{
		Client:   client,
		Scheme:   scheme,
		CronJobs: make(map[string]*cron.Cron),
	}
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
	if _, err := cron.ParseStandard(schedule); err != nil {
		log.Error(err, "failed to parse schedule", "schedule", schedule)
		return ctrl.Result{}, err
	}

	if r.CronJobs[checkpointSchedule.Name] != nil {
		delete(r.CronJobs, checkpointSchedule.Name)
	}
	cronJob := cron.New()
	r.CronJobs[checkpointSchedule.Name] = cronJob

	if _, err := cronJob.AddFunc(schedule, func() {
		if err := r.CronJob(ctx, req); err != nil {
			log.Error(err, "failed to execute cronjob")
			return
		}
	}); err != nil {
		log.Error(err, "failed to add cron job")
		return ctrl.Result{}, err
	}

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

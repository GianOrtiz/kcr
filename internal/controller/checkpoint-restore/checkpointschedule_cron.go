package checkpointrestore

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	checkpointrestorev1 "github.com/GianOrtiz/kcr/api/checkpoint-restore/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *CheckpointScheduleReconciler) CronJob(ctx context.Context, req ctrl.Request) error {
	log := log.FromContext(ctx)

	var currentSchedule checkpointrestorev1.CheckpointSchedule
	if err := r.Get(ctx, req.NamespacedName, &currentSchedule); err != nil {
		log.Error(err, "failed to get current schedule")
		err = fmt.Errorf("failed to get current schedule: %v", err)
		return err
	}

	// Get pods matching selector
	var podList corev1.PodList
	if err := r.List(ctx, &podList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(currentSchedule.Spec.Selector.MatchLabels),
		Namespace:     req.Namespace,
	}); err != nil {
		log.Error(err, "failed to list pods")
		err = fmt.Errorf("failed to list pods: %v", err)
		return err
	}

	if len(podList.Items) == 0 {
		log.Info("no pods found matching selector", "selector", currentSchedule.Spec.Selector)
		err := fmt.Errorf("no pods found matching selector")
		return err
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
		err = fmt.Errorf("failed to set controler reference for CheckpointRequest: %v", err)
		return err
	}

	// Create the CheckpointRequest resource
	if err := r.Create(ctx, checkpointRequest); err != nil {
		log.Error(err, "failed to create CheckpointRequest resource")
		err = fmt.Errorf("failed to create CheckpointRequest resource: %v", err)
		return err
	}
	log.Info("created checkpoint request", "checkpointRequest", checkpointRequest.Name)

	// Update the CheckpointSchedule status with the last run time
	currentSchedule.Status.LastRunTime = &metav1.Time{Time: time.Now()}
	if err := r.Status().Update(ctx, &currentSchedule); err != nil {
		log.Error(err, "failed to update CheckpointSchedule status")
		err = fmt.Errorf("failed to update CheckpointSchedule status: %v", err)
		return err
	}

	return nil
}

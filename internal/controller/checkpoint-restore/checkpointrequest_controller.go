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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	checkpointrestorev1 "github.com/GianOrtiz/kcr/api/checkpoint-restore/v1"
	"github.com/GianOrtiz/kcr/pkg/checkpoint"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CheckpointRequestReconciler reconciles a CheckpointRequest object
type CheckpointRequestReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	CheckpointService checkpoint.CheckpointService
}

// +kubebuilder:rbac:groups=checkpoint-restore.kcr.io,resources=checkpointrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=checkpoint-restore.kcr.io,resources=checkpointrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=checkpoint-restore.kcr.io,resources=checkpointrequests/finalizers,verbs=update
// +kubebuilder:rbac:groups=checkpoint-restore.kcr.io,resources=checkpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=nodes/proxy,verbs=get;create;post
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *CheckpointRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the CheckpointRequest
	var checkpointRequest checkpointrestorev1.CheckpointRequest
	if err := r.Get(ctx, req.NamespacedName, &checkpointRequest); err != nil {
		if client.IgnoreNotFound(err) != nil {
			log.Error(err, "unable to fetch CheckpointRequest")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// If it's already completed or failed, no need to process it again
	if checkpointRequest.Status.Phase == "Completed" || checkpointRequest.Status.Phase == "Failed" {
		return ctrl.Result{}, nil
	}

	// If it's not in Pending phase, and not Completed or Failed, it must be InProgress
	// We'll just requeue it for later processing to avoid race conditions
	if checkpointRequest.Status.Phase == "InProgress" {
		// Requeue after a short period
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Update the request to InProgress and set the start time
	checkpointRequest.Status.Phase = "InProgress"
	checkpointRequest.Status.StartTime = &metav1.Time{Time: time.Now()}
	if err := r.Status().Update(ctx, &checkpointRequest); err != nil {
		log.Error(err, "failed to update CheckpointRequest status to InProgress")
		return ctrl.Result{}, err
	}

	// Get the pod information
	podName := checkpointRequest.Spec.PodReference.Name
	podNamespace := checkpointRequest.Spec.PodReference.Namespace
	containerName := checkpointRequest.Spec.ContainerName

	// Get the pod to obtain node information
	var pod corev1.Pod
	if err := r.Get(ctx, client.ObjectKey{Name: podName, Namespace: podNamespace}, &pod); err != nil {
		log.Error(err, "failed to get pod", "pod", podName, "namespace", podNamespace)

		// Update the request to Failed
		checkpointRequest.Status.Phase = "Failed"
		checkpointRequest.Status.CompletionTime = &metav1.Time{Time: time.Now()}
		checkpointRequest.Status.Message = fmt.Sprintf("Failed to get pod: %v", err)
		if updateErr := r.Status().Update(ctx, &checkpointRequest); updateErr != nil {
			log.Error(updateErr, "failed to update CheckpointRequest status to Failed")
		}

		return ctrl.Result{}, err
	}

	nodeName := pod.Spec.NodeName
	log.Info("checkpointing pod", "nodeName", nodeName, "pod", podName, "namespace", podNamespace, "container", containerName)
	err := r.CheckpointService.Checkpoint(nodeName, podName, podNamespace, containerName)
	if err != nil {
		log.Error(err, "failed to checkpoint pod")

		// Update the request to Failed
		checkpointRequest.Status.Phase = "Failed"
		checkpointRequest.Status.CompletionTime = &metav1.Time{Time: time.Now()}
		checkpointRequest.Status.Message = fmt.Sprintf("Failed to checkpoint pod: %v", err)
		if updateErr := r.Status().Update(ctx, &checkpointRequest); updateErr != nil {
			log.Error(updateErr, "failed to update CheckpointRequest status to Failed")
		}

		return ctrl.Result{}, err
	}
	log.Info("checkpoint completed", "pod", podName)

	// Create a Checkpoint resource
	podFullName := podName + "_" + podNamespace
	timestamp := time.Now().Unix()
	checkpointID := fmt.Sprintf("%s-%d", podName+"-"+podNamespace, timestamp)

	// Construct path to checkpoint tar file
	checkpointFileName := fmt.Sprintf("checkpoint-%s-%s-%d.tar", podFullName, containerName, timestamp)
	checkpointFilePath := fmt.Sprintf("/var/lib/kubelet/checkpoints/%s", checkpointFileName)

	checkpoint := &checkpointrestorev1.Checkpoint{
		ObjectMeta: metav1.ObjectMeta{
			Name:      checkpointID,
			Namespace: req.Namespace,
			Labels: map[string]string{
				"pod":                     podName,
				"pod-ns":                  podNamespace,
				"container":               containerName,
				"checkpoint-request-name": checkpointRequest.Name,
			},
		},
		Spec: checkpointrestorev1.CheckpointSpec{
			CheckpointData:      checkpointFilePath,
			CheckpointTimestamp: &metav1.Time{Time: time.Now()},
			CheckpointID:        checkpointID,
			NodeName:            pod.Spec.NodeName,
		},
		Status: checkpointrestorev1.CheckpointStatus{
			Phase: "Created",
		},
	}

	// If the request has a parent CheckpointSchedule, add its reference and details
	if checkpointRequest.Spec.CheckpointScheduleRef != nil {
		checkpoint.Spec.CheckpointScheduleRef = checkpointRequest.Spec.CheckpointScheduleRef

		// Get the parent CheckpointSchedule to copy its schedule and selector
		var checkpointSchedule checkpointrestorev1.CheckpointSchedule
		if err := r.Get(ctx, client.ObjectKey{
			Name:      checkpointRequest.Spec.CheckpointScheduleRef.Name,
			Namespace: checkpointRequest.Spec.CheckpointScheduleRef.Namespace,
		}, &checkpointSchedule); err == nil {
			checkpoint.Spec.Schedule = checkpointSchedule.Spec.Schedule
			checkpoint.Spec.Selector = &metav1.LabelSelector{
				MatchLabels:      checkpointSchedule.Spec.Selector.MatchLabels,
				MatchExpressions: checkpointSchedule.Spec.Selector.MatchExpressions,
			}
		} else {
			log.Error(err, "failed to get parent CheckpointSchedule")
		}
	}

	// Set the controller reference to the CheckpointRequest
	if err := ctrl.SetControllerReference(&checkpointRequest, checkpoint, r.Scheme); err != nil {
		log.Error(err, "failed to set controller reference for Checkpoint")

		// Update the request to Failed
		checkpointRequest.Status.Phase = "Failed"
		checkpointRequest.Status.CompletionTime = &metav1.Time{Time: time.Now()}
		checkpointRequest.Status.Message = fmt.Sprintf("Failed to set controller reference: %v", err)
		if updateErr := r.Status().Update(ctx, &checkpointRequest); updateErr != nil {
			log.Error(updateErr, "failed to update CheckpointRequest status to Failed")
		}

		return ctrl.Result{}, err
	}

	// Create the Checkpoint resource
	if err := r.Create(ctx, checkpoint); err != nil {
		log.Error(err, "failed to create Checkpoint resource")

		// Update the request to Failed
		checkpointRequest.Status.Phase = "Failed"
		checkpointRequest.Status.CompletionTime = &metav1.Time{Time: time.Now()}
		checkpointRequest.Status.Message = fmt.Sprintf("Failed to create Checkpoint resource: %v", err)
		if updateErr := r.Status().Update(ctx, &checkpointRequest); updateErr != nil {
			log.Error(updateErr, "failed to update CheckpointRequest status to Failed")
		}

		return ctrl.Result{}, err
	}
	log.Info("created checkpoint resource", "checkpoint", checkpoint.Name)

	// Update the request to Completed
	checkpointRequest.Status.Phase = "Completed"
	checkpointRequest.Status.CompletionTime = &metav1.Time{Time: time.Now()}
	checkpointRequest.Status.Message = "Checkpoint created successfully"
	checkpointRequest.Status.Checkpoint = &corev1.ObjectReference{
		Kind:       "Checkpoint",
		Name:       checkpoint.Name,
		Namespace:  checkpoint.Namespace,
		UID:        checkpoint.UID,
		APIVersion: checkpoint.APIVersion,
	}

	if err := r.Status().Update(ctx, &checkpointRequest); err != nil {
		log.Error(err, "failed to update CheckpointRequest status to Completed")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CheckpointRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&checkpointrestorev1.CheckpointRequest{}).
		Named("checkpoint-restore-checkpointrequest").
		Complete(r)
}

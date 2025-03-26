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

package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodReference contains the information to identify a pod
type PodReference struct {
	// Name is the name of the pod
	Name string `json:"name"`

	// Namespace is the namespace of the pod
	Namespace string `json:"namespace"`
}

// CheckpointRequestSpec defines the desired state of CheckpointRequest
type CheckpointRequestSpec struct {
	// PodReference is a reference to the pod to be checkpointed
	PodReference PodReference `json:"podReference"`

	// ContainerName is the name of the container within the pod to checkpoint
	ContainerName string `json:"containerName"`

	// CheckpointScheduleRef is an optional reference to the parent CheckpointSchedule
	// if triggered by a schedule
	// +optional
	CheckpointScheduleRef *corev1.ObjectReference `json:"checkpointScheduleRef,omitempty"`

	// TimeoutSeconds is an optional timeout for the checkpoint operation
	// +optional
	// +kubebuilder:default=300
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
}

// CheckpointRequestStatus defines the observed state of CheckpointRequest
type CheckpointRequestStatus struct {
	// Phase represents the current state of the checkpoint request
	// +kubebuilder:validation:Enum=Pending;InProgress;Completed;Failed
	Phase string `json:"phase,omitempty"`

	// StartTime is when the checkpoint operation started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the checkpoint operation completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Checkpoint is a reference to the created Checkpoint resource if successful
	// +optional
	Checkpoint *corev1.ObjectReference `json:"checkpoint,omitempty"`

	// Message is a human-readable status or error message
	// +optional
	Message string `json:"message,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// CheckpointRequest is the Schema for the checkpointrequests API
type CheckpointRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CheckpointRequestSpec   `json:"spec,omitempty"`
	Status CheckpointRequestStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CheckpointRequestList contains a list of CheckpointRequest
type CheckpointRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CheckpointRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CheckpointRequest{}, &CheckpointRequestList{})
}

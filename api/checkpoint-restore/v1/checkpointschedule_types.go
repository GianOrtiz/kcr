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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CheckpointScheduleSpec defines the desired state of CheckpointSchedule.
type CheckpointScheduleSpec struct {
	// Selector enables the selection of correct pods for checkpoint.
	Selector metav1.LabelSelector `json:"selector,omitempty"`
	// The schedule to create checkpoints.
	Schedule string `json:"schedule,omitempty"`
}

// CheckpointScheduleStatus defines the observed state of CheckpointSchedule.
type CheckpointScheduleStatus struct {
	// LastRunTime is the time of the last checkpoint creation.
	LastRunTime *metav1.Time `json:"lastRunTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// CheckpointSchedule is the Schema for the checkpointschedules API.
type CheckpointSchedule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CheckpointScheduleSpec   `json:"spec,omitempty"`
	Status CheckpointScheduleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CheckpointScheduleList contains a list of CheckpointSchedule.
type CheckpointScheduleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []CheckpointSchedule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CheckpointSchedule{}, &CheckpointScheduleList{})
}

/*
Copyright 2023.

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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// VirtualMachineNodeWatcherSpec defines the desired state of VirtualMachineNodeWatcher
type VirtualMachineNodeWatcherSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Enable "make" to regenerate code after modifying this file

	// Interval  Migration Time Threshold
	Interval string `json:"interval,omitempty"`
	// Clean Timed cleaning of VirtualMachineInstanceRescue time
	Clean  string `json:"clean,omitempty"`
	Enable bool   `json:"enable,omitempty"`
}

// VirtualMachineNodeWatcherStatus defines the observed state of VirtualMachineNodeWatcher
type VirtualMachineNodeWatcherStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Enable "make" to regenerate code after modifying this file
	Phase string `json:"phase"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:resource:shortName=vmnw
//+kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
//+kubebuilder:printcolumn:name="Interval",type="string",JSONPath=".spec.interval"

// VirtualMachineNodeWatcher is the Schema for the virtualmachinenodewatchers API
type VirtualMachineNodeWatcher struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineNodeWatcherSpec   `json:"spec" valid:"required"`
	Status VirtualMachineNodeWatcherStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VirtualMachineNodeWatcherList contains a list of VirtualMachineNodeWatcher
type VirtualMachineNodeWatcherList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachineNodeWatcher `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VirtualMachineNodeWatcher{}, &VirtualMachineNodeWatcherList{})
}

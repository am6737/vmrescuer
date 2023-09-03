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

// VirtualMachineInstanceRescueSpec defines the desired state of VirtualMachineInstanceRescue
type VirtualMachineInstanceRescueSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	VMI  string `json:"vmi,omitempty"`
	Node string `json:"node,omitempty"`
}

type VirtualMachineInstanceMigrationPhase string

const (
	MigrationPending   VirtualMachineInstanceMigrationPhase = "Pending"
	MigrationQueuing   VirtualMachineInstanceMigrationPhase = "Queuing"
	MigrationRunning   VirtualMachineInstanceMigrationPhase = "Running"
	MigrationSucceeded VirtualMachineInstanceMigrationPhase = "Succeeded"
	MigrationFailed    VirtualMachineInstanceMigrationPhase = "Failed"
	MigrationCancel    VirtualMachineInstanceMigrationPhase = "Cancel"
)

// VirtualMachineInstanceRescueStatus defines the observed state of VirtualMachineInstanceRescue
type VirtualMachineInstanceRescueStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Phase         VirtualMachineInstanceMigrationPhase `json:"phase,omitempty"`
	MigrationTime metav1.Time                          `json:"migration_time,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=vmir
//+kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
//+kubebuilder:printcolumn:name="VMI",type="string",JSONPath=".spec.vmi"
//+kubebuilder:printcolumn:name="Node",type="string",JSONPath=".spec.node"
//+kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// VirtualMachineInstanceRescue is the Schema for the virtualmachineinstancerescues API
type VirtualMachineInstanceRescue struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineInstanceRescueSpec   `json:"spec,omitempty"`
	Status VirtualMachineInstanceRescueStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VirtualMachineInstanceRescueList contains a list of VirtualMachineInstanceRescue
type VirtualMachineInstanceRescueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachineInstanceRescue `json:"items"`
}

func init() {
	SchemeBuilder.Register(&VirtualMachineInstanceRescue{}, &VirtualMachineInstanceRescueList{})
}

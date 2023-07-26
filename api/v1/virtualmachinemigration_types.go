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

// VirtualMachineMigrationSpec defines the desired state of VirtualMachineMigration
type VirtualMachineMigrationSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Enable "make" to regenerate code after modifying this file

	// Foo is an example field of VirtualMachineMigration. Edit virtualmachinemigration_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// VirtualMachineMigrationStatus defines the observed state of VirtualMachineMigration
type VirtualMachineMigrationStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Enable "make" to regenerate code after modifying this file
	Name          string      `json:"name,omitempty"`
	Status        string      `json:"status,omitempty"`
	Node          string      `json:"node,omitempty"`
	MigrationTime metav1.Time `json:"migration_time,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Name",type="string",JSONPath=".status.name",description="The schedule in Cron format"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.status"
// +kubebuilder:printcolumn:name="Node",type="string",JSONPath=".status.node"
// +kubebuilder:printcolumn:name="MigrationTime",type="date",JSONPath=".status.migrationTime"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// VirtualMachineMigration is the Schema for the virtualmachinemigrations API
type VirtualMachineMigration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   VirtualMachineMigrationSpec   `json:"spec,omitempty"`
	Status VirtualMachineMigrationStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// VirtualMachineMigrationList contains a list of VirtualMachineMigration
type VirtualMachineMigrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []VirtualMachineMigration `json:"items"`
}

//+kubebuilder:rbac:groups=monitor.hitosea.com,resources=virtualmachinemigrations,verbs=create;delete;get;list;patch;update;watch


func init() {
	SchemeBuilder.Register(&VirtualMachineMigration{}, &VirtualMachineMigrationList{})
}

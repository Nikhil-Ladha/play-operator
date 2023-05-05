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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PlayOpSpec defines the desired state of PlayOp
type PlayOpSpec struct {

	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=20
	// +kubebuilder:validation:ExclusiveMaximum=false

	// Size defines the number of pods deployed by the operator
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Size int32 `json:"size"`
}

// PlayOpStatus defines the observed state of PlayOp
type PlayOpStatus struct {

	// Pods  are the names of the pods deployed by playop
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Pods []string `json:"pods,omitempty"`

	// Represents the observations of a Memcached's current state.
	// Memcached.status.conditions.type are: "Available", "Progressing", and "Degraded"
	// Memcached.status.conditions.status are one of True, False, Unknown.
	// Memcached.status.conditions.reason the value should be a CamelCase string and producers of specific
	// condition types may define expected values and meanings for this field, and whether the values
	// are considered a guaranteed API.
	// Memcached.status.conditions.Message is a human readable message indicating details about the transition.
	// For further information see: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// Conditions store the status conditions of the Memcached instances
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PlayOp is the Schema for the playops API
type PlayOp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PlayOpSpec   `json:"spec,omitempty"`
	Status PlayOpStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PlayOpList contains a list of PlayOp
type PlayOpList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PlayOp `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PlayOp{}, &PlayOpList{})
}

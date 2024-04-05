/*
Copyright 2024 The Kubernetes Authors.

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

// AzureASOManagedControlPlaneKind is the kind for AzureASOManagedControlPlane.
const AzureASOManagedControlPlaneKind = "AzureASOManagedControlPlane"

// AzureASOManagedControlPlaneSpec defines the desired state of AzureASOManagedControlPlane.
type AzureASOManagedControlPlaneSpec struct {
	AzureASOManagedControlPlaneTemplateResourceSpec `json:",inline"`
}

// AzureASOManagedControlPlaneStatus defines the observed state of AzureASOManagedControlPlane.
type AzureASOManagedControlPlaneStatus struct {
	//+optional
	Resources []ResourceStatus `json:"resources,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// AzureASOManagedControlPlane is the Schema for the azureasomanagedcontrolplanes API.
type AzureASOManagedControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AzureASOManagedControlPlaneSpec   `json:"spec,omitempty"`
	Status AzureASOManagedControlPlaneStatus `json:"status,omitempty"`
}

// SetResourceStatuses returns the status of resources.
func (a *AzureASOManagedControlPlane) SetResourceStatuses(r []ResourceStatus) {
	a.Status.Resources = r
}

//+kubebuilder:object:root=true

// AzureASOManagedControlPlaneList contains a list of AzureASOManagedControlPlane.
type AzureASOManagedControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AzureASOManagedControlPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AzureASOManagedControlPlane{}, &AzureASOManagedControlPlaneList{})
}

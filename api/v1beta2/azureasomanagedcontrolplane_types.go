/*
Copyright 2025 The Kubernetes Authors.

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

package v1beta2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// AzureASOManagedControlPlaneKind is the kind for AzureASOManagedControlPlane.
const AzureASOManagedControlPlaneKind = "AzureASOManagedControlPlane"

// AzureASOManagedControlPlaneSpec defines the desired state of AzureASOManagedControlPlane.
type AzureASOManagedControlPlaneSpec struct {
	AzureASOManagedControlPlaneTemplateResourceSpec `json:",inline"`
}

// AzureASOManagedControlPlaneStatus defines the observed state of AzureASOManagedControlPlane.
type AzureASOManagedControlPlaneStatus struct {
	// initialization provides observations of the AzureASOManagedControlPlane initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Cluster provisioning.
	// +optional
	Initialization AzureASOManagedControlPlaneInitializationStatus `json:"initialization,omitempty"`

	// Version is the observed Kubernetes version of the control plane. It fulfills Cluster API's control
	// plane provider contract.
	//+optional
	Version string `json:"version,omitempty"`

	//+optional
	Resources []ResourceStatus `json:"resources,omitempty"`

	// ControlPlaneEndpoint represents the endpoint for the cluster's API server.
	//+optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`

	// deprecated groups all the status fields that are deprecated and will be removed in a future version.
	// +optional
	Deprecated *AzureASOManagedControlPlaneDeprecatedStatus `json:"deprecated,omitempty"`
}

// AzureASOManagedControlPlaneInitializationStatus provides observations of the AzureASOManagedControlPlane initialization process.
type AzureASOManagedControlPlaneInitializationStatus struct {
	// provisioned is true when the infrastructure provider reports that the AzureASOManagedControlPlane's infrastructure is fully provisioned.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate provisioning.
	// The value of this field is never updated after provisioning is completed.
	// +optional
	Provisioned *bool `json:"provisioned,omitempty"`

	// controlPlaneInitialized denotes whether the control plane is already initialized.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate provisioning.
	// The value of this field is never updated after initialization is completed.
	// +optional
	ControlPlaneInitialized *bool `json:"controlPlaneInitialized,omitempty"`
}

// AzureASOManagedControlPlaneDeprecatedStatus groups all the status fields that are deprecated and will be removed in a future version.
type AzureASOManagedControlPlaneDeprecatedStatus struct {
	// v1beta1 groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
	// +optional
	V1Beta1 *AzureASOManagedControlPlaneV1Beta1DeprecatedStatus `json:"v1beta1,omitempty"`
}

// AzureASOManagedControlPlaneV1Beta1DeprecatedStatus groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
type AzureASOManagedControlPlaneV1Beta1DeprecatedStatus struct {
	// ready is true when the provider resource is ready.
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped.
	//
	// +optional
	Ready bool `json:"ready,omitempty"`

	// initialized represents whether or not the API server has been provisioned.
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped.
	//
	// +optional
	Initialized bool `json:"initialized,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// AzureASOManagedControlPlane is the Schema for the azureasomanagedcontrolplanes API.
type AzureASOManagedControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AzureASOManagedControlPlaneSpec   `json:"spec,omitempty"`
	Status AzureASOManagedControlPlaneStatus `json:"status,omitempty"`
}

// GetResourceStatuses returns the status of resources.
func (a *AzureASOManagedControlPlane) GetResourceStatuses() []ResourceStatus {
	return a.Status.Resources
}

// SetResourceStatuses sets the status of resources.
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
	objectTypes = append(objectTypes, &AzureASOManagedControlPlane{}, &AzureASOManagedControlPlaneList{})
}

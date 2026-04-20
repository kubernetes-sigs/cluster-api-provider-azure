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

const (
	// AzureASOManagedClusterKind is the kind for AzureASOManagedCluster.
	AzureASOManagedClusterKind = "AzureASOManagedCluster"

	// AzureASOManagedControlPlaneFinalizer is the finalizer added to AzureASOManagedControlPlanes.
	AzureASOManagedControlPlaneFinalizer = "azureasomanagedcontrolplane.infrastructure.cluster.x-k8s.io"
)

// AzureASOManagedClusterSpec defines the desired state of AzureASOManagedCluster.
type AzureASOManagedClusterSpec struct {
	AzureASOManagedClusterTemplateResourceSpec `json:",inline"`

	// ControlPlaneEndpoint is the location of the API server within the control plane. CAPZ manages this field
	// and it should not be set by the user. It fulfills Cluster API's cluster infrastructure provider contract.
	// Because this field is programmatically set by CAPZ after resource creation, we define it as +optional
	// in the API schema to permit resource admission.
	//+optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty,omitzero"`
}

// AzureASOManagedClusterStatus defines the observed state of AzureASOManagedCluster.
type AzureASOManagedClusterStatus struct {
	// initialization provides observations of the AzureASOManagedCluster initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Cluster provisioning.
	// +optional
	Initialization AzureASOManagedClusterInitializationStatus `json:"initialization,omitempty,omitzero"`

	//+optional
	Resources []ResourceStatus `json:"resources,omitempty"`

	// deprecated groups all the status fields that are deprecated and will be removed in a future version.
	// +optional
	Deprecated *AzureASOManagedClusterDeprecatedStatus `json:"deprecated,omitempty"`
}

// AzureASOManagedClusterInitializationStatus provides observations of the AzureASOManagedCluster initialization process.
// +kubebuilder:validation:MinProperties=1
type AzureASOManagedClusterInitializationStatus struct {
	// provisioned is true when the infrastructure provider reports that the AzureASOManagedCluster's infrastructure is fully provisioned.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate provisioning.
	// The value of this field is never updated after provisioning is completed.
	// +optional
	Provisioned *bool `json:"provisioned,omitempty"`
}

// AzureASOManagedClusterDeprecatedStatus groups all the status fields that are deprecated and will be removed in a future version.
type AzureASOManagedClusterDeprecatedStatus struct {
	// v1beta1 groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
	// +optional
	V1Beta1 *AzureASOManagedClusterV1Beta1DeprecatedStatus `json:"v1beta1,omitempty"`
}

// AzureASOManagedClusterV1Beta1DeprecatedStatus groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
type AzureASOManagedClusterV1Beta1DeprecatedStatus struct {
	// ready is true when the provider resource is ready.
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped.
	//
	// +optional
	Ready bool `json:"ready,omitempty"`
}

// ResourceStatus represents the status of a resource.
type ResourceStatus struct {
	Resource StatusResource `json:"resource"`
	Ready    bool           `json:"ready"`
}

// StatusResource is a handle to a resource.
type StatusResource struct {
	Group   string `json:"group"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
	Name    string `json:"name"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// AzureASOManagedCluster is the Schema for the azureasomanagedclusters API.
type AzureASOManagedCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AzureASOManagedClusterSpec   `json:"spec,omitempty"`
	Status AzureASOManagedClusterStatus `json:"status,omitempty"`
}

// GetResourceStatuses returns the status of resources.
func (a *AzureASOManagedCluster) GetResourceStatuses() []ResourceStatus {
	return a.Status.Resources
}

// SetResourceStatuses sets the status of resources.
func (a *AzureASOManagedCluster) SetResourceStatuses(r []ResourceStatus) {
	a.Status.Resources = r
}

//+kubebuilder:object:root=true

// AzureASOManagedClusterList contains a list of AzureASOManagedCluster.
type AzureASOManagedClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AzureASOManagedCluster `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &AzureASOManagedCluster{}, &AzureASOManagedClusterList{})
}

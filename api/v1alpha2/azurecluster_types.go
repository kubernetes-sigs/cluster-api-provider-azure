/*
Copyright 2019 The Kubernetes Authors.

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

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ClusterFinalizer allows ReconcileAzureCluster to clean up Azure resources associated with AzureCluster before
	// removing it from the apiserver.
	ClusterFinalizer = "azurecluster.infrastructure.cluster.x-k8s.io"
)

// AzureClusterSpec defines the desired state of AzureCluster
type AzureClusterSpec struct {
	// NetworkSpec encapsulates all things related to Azure network.
	NetworkSpec NetworkSpec `json:"networkSpec,omitempty"`

	ResourceGroup string `json:"resourceGroup"`

	Location string `json:"location"`

	ControlPlaneEndpoint string `json:"controlplaneendpoint"`

	// AdditionalTags is an optional set of tags to add to Azure resources managed by the Azure provider, in addition to the
	// ones added by default.
	// +optional
	AdditionalTags Tags `json:"additionalTags,omitempty"`
}

// AzureClusterStatus defines the observed state of AzureCluster
type AzureClusterStatus struct {
	Network Network `json:"network,omitempty"`

	//Bastion VM `json:"bastion,omitempty"`

	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

	// APIEndpoints represents the endpoints to communicate with the control plane.
	// +optional
	APIEndpoints []APIEndpoint `json:"apiEndpoints,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=azureclusters,scope=Namespaced,categories=cluster-api
// +kubebuilder:subresource:status

// AzureCluster is the Schema for the azureclusters API
type AzureCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AzureClusterSpec   `json:"spec,omitempty"`
	Status AzureClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AzureClusterList contains a list of AzureCluster
type AzureClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AzureCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AzureCluster{}, &AzureClusterList{})
}

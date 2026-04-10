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

// AzureManagedClusterSpec defines the desired state of AzureManagedCluster.
type AzureManagedClusterSpec struct {
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// Immutable, populated by the AKS API at create.
	// Because this field is programmatically set by CAPZ after resource creation, we define it as +optional
	// in the API schema to permit resource admission.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty,omitzero"`
}

// AzureManagedClusterStatus defines the observed state of AzureManagedCluster.
type AzureManagedClusterStatus struct {
	// initialization provides observations of the AzureManagedCluster initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Cluster provisioning.
	// +optional
	Initialization AzureManagedClusterInitializationStatus `json:"initialization,omitempty,omitzero"`

	// deprecated groups all the status fields that are deprecated and will be removed in a future version.
	// +optional
	Deprecated *AzureManagedClusterDeprecatedStatus `json:"deprecated,omitempty"`
}

// AzureManagedClusterInitializationStatus provides observations of the AzureManagedCluster initialization process.
// +kubebuilder:validation:MinProperties=1
type AzureManagedClusterInitializationStatus struct {
	// provisioned is true when the infrastructure provider reports that the AzureManagedCluster's infrastructure is fully provisioned.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate provisioning.
	// The value of this field is never updated after provisioning is completed.
	// +optional
	Provisioned *bool `json:"provisioned,omitempty"`
}

// AzureManagedClusterDeprecatedStatus groups all the status fields that are deprecated and will be removed in a future version.
type AzureManagedClusterDeprecatedStatus struct {
	// v1beta1 groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
	// +optional
	V1Beta1 *AzureManagedClusterV1Beta1DeprecatedStatus `json:"v1beta1,omitempty"`
}

// AzureManagedClusterV1Beta1DeprecatedStatus groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
type AzureManagedClusterV1Beta1DeprecatedStatus struct {
	// ready is true when the provider resource is ready.
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped.
	//
	// +optional
	Ready bool `json:"ready,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this AzureManagedCluster belongs"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of this AzureManagedCluster"
// +kubebuilder:resource:path=azuremanagedclusters,scope=Namespaced,categories=cluster-api,shortName=amc
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:deprecatedversion:warning="AzureManagedCluster and the AzureManaged API are deprecated. Please migrate to infrastructure.cluster.x-k8s.io/v1beta2 AzureASOManagedCluster and related AzureASOManaged resources instead."

// AzureManagedCluster is the Schema for the azuremanagedclusters API.
type AzureManagedCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AzureManagedClusterSpec   `json:"spec,omitempty"`
	Status AzureManagedClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AzureManagedClusterList contains a list of AzureManagedClusters.
type AzureManagedClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AzureManagedCluster `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &AzureManagedCluster{}, &AzureManagedClusterList{})
}

/*
Copyright 2023 The Kubernetes Authors.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

// AzureManagedClusterSpec defines the desired state of AzureManagedCluster.
type AzureManagedClusterSpec struct {
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// Immutable, populated by the AKS API at create.
	// Because this field is programmatically set by CAPZ after resource creation, we define it as +optional
	// in the API schema to permit resource admission.
	// +optional
	ControlPlaneEndpoint clusterv1beta1.APIEndpoint `json:"controlPlaneEndpoint"`
}

// AzureManagedClusterStatus defines the observed state of AzureManagedCluster.
type AzureManagedClusterStatus struct {
	// Ready is true when the provider resource is ready.
	// Deprecated: Use status.initialization.provisioned instead.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// initialization provides observations of the AzureManagedCluster initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Cluster provisioning.
	// +optional
	Initialization *AzureManagedClusterInitializationStatus `json:"initialization,omitempty"`
}

// AzureManagedClusterInitializationStatus provides observations of the AzureManagedCluster initialization process.
type AzureManagedClusterInitializationStatus struct {
	// provisioned is true when the infrastructure provider reports that the Cluster's infrastructure is fully provisioned.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate initial Cluster provisioning.
	// +optional
	Provisioned *bool `json:"provisioned,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this AzureManagedCluster belongs"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of this AzureManagedCluster"
// +kubebuilder:resource:path=azuremanagedclusters,scope=Namespaced,categories=cluster-api,shortName=amc
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:deprecatedversion:warning="AzureManagedCluster and the AzureManaged API are deprecated. Please migrate to infrastructure.cluster.x-k8s.io/v1beta1 AzureASOManagedCluster and related AzureASOManaged resources instead."

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

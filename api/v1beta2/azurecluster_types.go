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
	// ClusterFinalizer allows ReconcileAzureCluster to clean up Azure resources associated with AzureCluster before
	// removing it from the apiserver.
	ClusterFinalizer = "azurecluster.infrastructure.cluster.x-k8s.io"

	// ClusterLabelNamespace indicates the namespace of the cluster.
	ClusterLabelNamespace = "azurecluster.infrastructure.cluster.x-k8s.io/cluster-namespace"
)

// AzureClusterSpec defines the desired state of AzureCluster.
type AzureClusterSpec struct {
	AzureClusterClassSpec `json:",inline"`

	// NetworkSpec encapsulates all things related to Azure network.
	// +optional
	NetworkSpec NetworkSpec `json:"networkSpec,omitempty"`

	// +optional
	ResourceGroup string `json:"resourceGroup,omitempty"`

	// BastionSpec encapsulates all things related to the Bastions in the cluster.
	// +optional
	BastionSpec BastionSpec `json:"bastionSpec,omitempty"`

	// ControlPlaneEnabled enables control plane components in the cluster.
	// +kubebuilder:default=true
	// +optional
	ControlPlaneEnabled bool `json:"controlPlaneEnabled,omitempty"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane. It is not recommended to set
	// this when creating an AzureCluster as CAPZ will set this for you. However, if it is set, CAPZ will not change it.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint,omitempty,omitzero"`
}

// AzureClusterStatus defines the observed state of AzureCluster.
type AzureClusterStatus struct {
	// conditions represents the observations of an AzureCluster's current state.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// initialization provides observations of the AzureCluster initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Cluster provisioning.
	// +optional
	Initialization AzureClusterInitializationStatus `json:"initialization,omitempty,omitzero"`

	// failureDomains specifies the list of unique failure domains for the location/region of the cluster.
	// A FailureDomain maps to Availability Zone with an Azure Region (if the region support them). An
	// Availability Zone is a separate data center within a region and they can be used to ensure
	// the cluster is more resilient to failure.
	// See: https://learn.microsoft.com/azure/reliability/availability-zones-overview
	// This list will be used by Cluster API to try and spread the machines across the failure domains.
	// +optional
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=100
	FailureDomains []clusterv1.FailureDomain `json:"failureDomains,omitempty"`

	// LongRunningOperationStates saves the states for Azure long-running operations so they can be continued on the
	// next reconciliation loop.
	// +optional
	LongRunningOperationStates Futures `json:"longRunningOperationStates,omitempty"`

	// deprecated groups all the status fields that are deprecated and will be removed in a future version.
	// +optional
	Deprecated *AzureClusterDeprecatedStatus `json:"deprecated,omitempty"`
}

// AzureClusterInitializationStatus provides observations of the AzureCluster initialization process.
// +kubebuilder:validation:MinProperties=1
type AzureClusterInitializationStatus struct {
	// provisioned is true when the infrastructure provider reports that the AzureCluster's infrastructure is fully provisioned.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate provisioning.
	// The value of this field is never updated after provisioning is completed.
	// +optional
	Provisioned *bool `json:"provisioned,omitempty"`
}

// AzureClusterDeprecatedStatus groups all the status fields that are deprecated and will be removed in a future version.
type AzureClusterDeprecatedStatus struct {
	// v1beta1 groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
	// +optional
	V1Beta1 *AzureClusterV1Beta1DeprecatedStatus `json:"v1beta1,omitempty"`
}

// AzureClusterV1Beta1DeprecatedStatus groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.
type AzureClusterV1Beta1DeprecatedStatus struct {
	// ready is true when the provider resource is ready.
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped.
	//
	// +optional
	Ready bool `json:"ready,omitempty"`

	// conditions defines current service state of the AzureCluster.
	//
	// Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped.
	//
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"` //nolint:staticcheck // Intentionally using deprecated field for v1beta1 backward compat
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this AzureCluster belongs"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="Reason",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].reason"
// +kubebuilder:printcolumn:name="Message",type="string",priority=1,JSONPath=".status.conditions[?(@.type=='Ready')].message"
// +kubebuilder:printcolumn:name="Resource Group",type="string",priority=1,JSONPath=".spec.resourceGroup"
// +kubebuilder:printcolumn:name="SubscriptionID",type="string",priority=1,JSONPath=".spec.subscriptionID"
// +kubebuilder:printcolumn:name="Location",type="string",priority=1,JSONPath=".spec.location"
// +kubebuilder:printcolumn:name="Endpoint",type="string",priority=1,JSONPath=".spec.controlPlaneEndpoint.host",description="Control Plane Endpoint"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of this AzureCluster"
// +kubebuilder:resource:path=azureclusters,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

// AzureCluster is the Schema for the azureclusters API.
type AzureCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AzureClusterSpec   `json:"spec,omitempty"`
	Status AzureClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AzureClusterList contains a list of AzureClusters.
type AzureClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AzureCluster `json:"items"`
}

// GetConditions returns the list of conditions for an AzureCluster API object.
func (c *AzureCluster) GetConditions() []metav1.Condition {
	return c.Status.Conditions
}

// SetConditions will set the given conditions on an AzureCluster object.
func (c *AzureCluster) SetConditions(conditions []metav1.Condition) {
	c.Status.Conditions = conditions
}

// GetV1Beta1Conditions returns the v1beta1 conditions for an AzureCluster API object.
//
//nolint:staticcheck // Intentionally using deprecated field for v1beta1 backward compat
func (c *AzureCluster) GetV1Beta1Conditions() clusterv1.Conditions {
	if c.Status.Deprecated == nil || c.Status.Deprecated.V1Beta1 == nil {
		return nil
	}
	return c.Status.Deprecated.V1Beta1.Conditions
}

// SetV1Beta1Conditions sets the v1beta1 conditions on an AzureCluster object.
//
//nolint:staticcheck // Intentionally using deprecated field for v1beta1 backward compat
func (c *AzureCluster) SetV1Beta1Conditions(conditions clusterv1.Conditions) {
	if c.Status.Deprecated == nil {
		c.Status.Deprecated = &AzureClusterDeprecatedStatus{}
	}
	if c.Status.Deprecated.V1Beta1 == nil {
		c.Status.Deprecated.V1Beta1 = &AzureClusterV1Beta1DeprecatedStatus{}
	}
	c.Status.Deprecated.V1Beta1.Conditions = conditions
}

// GetFutures returns the list of long running operation states for an AzureCluster API object.
func (c *AzureCluster) GetFutures() Futures {
	return c.Status.LongRunningOperationStates
}

// SetFutures will set the given long running operation states on an AzureCluster object.
func (c *AzureCluster) SetFutures(futures Futures) {
	c.Status.LongRunningOperationStates = futures
}

func init() {
	objectTypes = append(objectTypes, &AzureCluster{}, &AzureClusterList{})
}

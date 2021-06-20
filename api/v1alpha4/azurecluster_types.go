/*
Copyright 2021 The Kubernetes Authors.

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

package v1alpha4

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
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
	// NetworkSpec encapsulates all things related to Azure network.
	NetworkSpec NetworkSpec `json:"networkSpec,omitempty"`

	// +optional
	ResourceGroup string `json:"resourceGroup,omitempty"`

	// +optional
	SubscriptionID string `json:"subscriptionID,omitempty"`

	Location string `json:"location"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`

	// AdditionalTags is an optional set of tags to add to Azure resources managed by the Azure provider, in addition to the
	// ones added by default.
	// +optional
	AdditionalTags Tags `json:"additionalTags,omitempty"`

	// IdentityRef is a reference to an AzureIdentity to be used when reconciling this cluster
	// +optional
	IdentityRef *corev1.ObjectReference `json:"identityRef,omitempty"`

	// AzureEnvironment is the name of the AzureCloud to be used.
	// The default value that would be used by most users is "AzurePublicCloud", other values are:
	// - ChinaCloud: "AzureChinaCloud"
	// - GermanCloud: "AzureGermanCloud"
	// - PublicCloud: "AzurePublicCloud"
	// - USGovernmentCloud: "AzureUSGovernmentCloud"
	// +optional
	AzureEnvironment string `json:"azureEnvironment,omitempty"`

	// BastionSpec encapsulates all things related to the Bastions in the cluster.
	// +optional
	BastionSpec BastionSpec `json:"bastionSpec,omitempty"`

	// CloudProviderConfigOverrides is an optional set of configuration values that can be overridden in azure cloud provider config.
	// This is only a subset of options that are available in azure cloud provider config.
	// Some values for the cloud provider config are inferred from other parts of cluster api provider azure spec, and may not be available for overrides.
	// See: https://kubernetes-sigs.github.io/cloud-provider-azure/install/configs
	// Note: All cloud provider config values can be customized by creating the secret beforehand. CloudProviderConfigOverrides is only used when the secret is managed by the Azure Provider.
	// +optional
	CloudProviderConfigOverrides *CloudProviderConfigOverrides `json:"cloudProviderConfigOverrides,omitempty"`
}

// AzureClusterStatus defines the observed state of AzureCluster.
type AzureClusterStatus struct {
	// FailureDomains specifies the list of unique failure domains for the location/region of the cluster.
	// A FailureDomain maps to Availability Zone with an Azure Region (if the region support them). An
	// Availability Zone is a separate data center within a region and they can be used to ensure
	// the cluster is more resilient to failure.
	// See: https://docs.microsoft.com/en-us/azure/availability-zones/az-overview
	// This list will be used by Cluster API to try and spread the machines across the failure domains.
	FailureDomains clusterv1.FailureDomains `json:"failureDomains,omitempty"`

	// Ready is true when the provider resource is ready.
	// +optional
	Ready bool `json:"ready"`

	// Conditions defines current service state of the AzureCluster.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this AzureCluster belongs"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready"
// +kubebuilder:printcolumn:name="Resource Group",type="string",priority=1,JSONPath=".spec.resourceGroup"
// +kubebuilder:printcolumn:name="SubscriptionID",type="string",priority=1,JSONPath=".spec.subscriptionID"
// +kubebuilder:printcolumn:name="Location",type="string",priority=1,JSONPath=".spec.location"
// +kubebuilder:printcolumn:name="Endpoint",type="string",priority=1,JSONPath=".spec.controlPlaneEndpoint.host",description="Control Plane Endpoint"
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
func (c *AzureCluster) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions will set the given conditions on an AzureCluster object.
func (c *AzureCluster) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&AzureCluster{}, &AzureClusterList{})
}

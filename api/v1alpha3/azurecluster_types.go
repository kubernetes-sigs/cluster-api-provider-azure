/*
Copyright 2020 The Kubernetes Authors.

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

package v1alpha3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
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
}

// AzureClusterStatus defines the observed state of AzureCluster
type AzureClusterStatus struct {
	Network Network `json:"network,omitempty"`

	// FailureDomains specifies the list of unique failure domains for the location/region of the cluster.
	// A FailureDomain maps to Availability Zone with an Azure Region (if the region support them). An
	// Availability Zone is a separate data center within a region and they can be used to ensure
	// the cluster is more resilient to failure.
	// See: https://docs.microsoft.com/en-us/azure/availability-zones/az-overview
	// This list will be used by Cluster API to try and spread the machines across the failure domains.
	FailureDomains clusterv1.FailureDomains `json:"failureDomains,omitempty"`

	Bastion VM `json:"bastion,omitempty"`

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

// GetConditions returns the list of conditions for an AzureCluster API object.
func (c *AzureCluster) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions will set the given conditions on an AzureCluster object
func (c *AzureCluster) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

// ClusterDescriber implementation

// SubscriptionID returns the cluster resource group.
func (c *AzureCluster) SubscriptionID() string {
	return c.Spec.SubscriptionID
}

// ResourceGroup returns the cluster resource group.
func (c *AzureCluster) ResourceGroup() string {
	return c.Spec.ResourceGroup
}

// ClusterName returns the cluster name.
func (c *AzureCluster) ClusterName() string {
	for _, ref := range c.OwnerReferences {
		if ref.Kind != "Cluster" {
			continue
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return ""
		}
		if gv.Group == clusterv1.GroupVersion.Group {
			return ref.Name
		}
	}
	return ""
}

// Location returns the cluster location.
func (c *AzureCluster) Location() string {
	return c.Spec.Location
}

// SetFailureDomain will set the spec for a for a given key
func (c *AzureCluster) SetFailureDomain(id string, spec clusterv1.FailureDomainSpec) {
	if c.Status.FailureDomains == nil {
		c.Status.FailureDomains = make(clusterv1.FailureDomains, 0)
	}
	c.Status.FailureDomains[id] = spec
}

// AdditionalTags returns AdditionalTags from the scope's AzureCluster.
func (c *AzureCluster) AdditionalTags() Tags {
	tags := make(Tags)
	if c.Spec.AdditionalTags != nil {
		tags = c.Spec.AdditionalTags.DeepCopy()
	}
	return tags
}

// END

// NetworkDescriber implementation

// LoadBalancerName returns the node load balancer name.
func (c *AzureCluster) LoadBalancerName() string {
	return c.ClusterName()
}

// Network returns the cluster network object.
func (c *AzureCluster) Network() *Network {
	return &c.Status.Network
}

// Vnet returns the cluster Vnet.
func (c *AzureCluster) Vnet() *VnetSpec {
	return &c.Spec.NetworkSpec.Vnet
}

// IsVnetManaged returns true if the vnet is managed.
func (c *AzureCluster) IsVnetManaged() bool {
	return c.Spec.NetworkSpec.Vnet.ID == "" || c.Spec.NetworkSpec.Vnet.Tags.HasOwned(c.ClusterName())
}

// Subnets returns the cluster subnets.
func (c *AzureCluster) Subnets() Subnets {
	return c.Spec.NetworkSpec.Subnets
}

// ControlPlaneSubnet returns the cluster control plane subnet.
func (c *AzureCluster) ControlPlaneSubnet() *SubnetSpec {
	return c.Spec.NetworkSpec.GetControlPlaneSubnet()
}

// NodeSubnet returns the cluster node subnet.
func (c *AzureCluster) NodeSubnet() *SubnetSpec {
	return c.Spec.NetworkSpec.GetNodeSubnet()
}

// RouteTable returns the cluster node routetable.
func (c *AzureCluster) RouteTable() *RouteTable {
	return &c.Spec.NetworkSpec.GetNodeSubnet().RouteTable
}

func init() {
	SchemeBuilder.Register(&AzureCluster{}, &AzureClusterList{})
}

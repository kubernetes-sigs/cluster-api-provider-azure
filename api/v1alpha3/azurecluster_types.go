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
	"fmt"
	"hash/fnv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/net"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/defaults"
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

// ControlPlaneRouteTable returns the cluster controlplane routetable.
func (c *AzureCluster) ControlPlaneRouteTable() *RouteTable {
	return &c.Spec.NetworkSpec.GetControlPlaneSubnet().RouteTable
}

// NodeRouteTable returns the cluster controlplane routetable.
func (c *AzureCluster) NodeRouteTable() *RouteTable {
	return &c.Spec.NetworkSpec.GetNodeSubnet().RouteTable
}

// IsIPv6Enabled returns true if IPv6 is enabled.
func (c *AzureCluster) IsIPv6Enabled() bool {
	for _, cidr := range c.Spec.NetworkSpec.Vnet.CIDRBlocks {
		if net.IsIPv6CIDRString(cidr) {
			return true
		}
	}
	return false
}

// APIServerLB returns the cluster API Server load balancer.
func (c *AzureCluster) APIServerLB() *LoadBalancerSpec {
	return &c.Spec.NetworkSpec.APIServerLB
}

// APIServerLBName returns the API Server LB name.
func (c *AzureCluster) APIServerLBName() string {
	return c.APIServerLB().Name
}

// IsAPIServerPrivate returns true if the API Server LB is of type Internal.
func (c *AzureCluster) IsAPIServerPrivate() bool {
	return c.APIServerLB().Type == Internal
}

// APIServerPublicIP returns the API Server public IP.
func (c *AzureCluster) APIServerPublicIP() *PublicIPSpec {
	return c.APIServerLB().FrontendIPs[0].PublicIP
}

// APIServerPrivateIP returns the API Server private IP.
func (c *AzureCluster) APIServerPrivateIP() string {
	return c.APIServerLB().FrontendIPs[0].PrivateIPAddress
}

// APIServerLBPoolName returns the API Server LB backend pool name.
func (c *AzureCluster) APIServerLBPoolName(loadBalancerName string) string {
	return defaults.GenerateBackendAddressPoolName(loadBalancerName)
}

// NodeOutboundLBName returns the name of the node outbound LB.
func (c *AzureCluster) NodeOutboundLBName() string {
	return c.ClusterName()
}

// OutboundLBName returns the name of the outbound LB.
func (c *AzureCluster) OutboundLBName(role string) string {
	if role == Node {
		return c.ClusterName()
	}
	if c.IsAPIServerPrivate() {
		return defaults.GenerateControlPlaneOutboundLBName(c.ClusterName())
	}
	return c.APIServerLBName()
}

// OutboundPoolName returns the outbound LB backend pool name.
func (c *AzureCluster) OutboundPoolName(loadBalancerName string) string {
	return defaults.GenerateOutboundBackendAddressPoolName(loadBalancerName)
}

// SetDNSName sets the API Server public IP DNS name.
func (c *AzureCluster) SetDNSName(dnsSuffix string) {
	// for back compat, set the old API Server defaults if no API Server Spec has been set by new webhooks.
	lb := c.APIServerLB()
	if lb == nil || lb.Name == "" {
		lbName := fmt.Sprintf("%s-%s", c.ClusterName(), "public-lb")
		ip, dns := c.GenerateLegacyFQDN(dnsSuffix)
		lb = &LoadBalancerSpec{
			Name: lbName,
			SKU:  SKUStandard,
			FrontendIPs: []FrontendIP{
				{
					Name: defaults.GenerateFrontendIPConfigName(lbName),
					PublicIP: &PublicIPSpec{
						Name:    ip,
						DNSName: dns,
					},
				},
			},
			Type: Public,
		}
		lb.DeepCopyInto(c.APIServerLB())
	}
	// Generate valid FQDN if not set.
	if !c.IsAPIServerPrivate() && c.APIServerPublicIP().DNSName == "" {
		c.APIServerPublicIP().DNSName = c.GenerateFQDN(dnsSuffix, c.APIServerPublicIP().Name)
	}
}

// GenerateLegacyFQDN generates an IP name and a fully qualified domain name, based on a hash, cluster name and cluster location.
// DEPRECATED: use GenerateFQDN instead.
func (c *AzureCluster) GenerateLegacyFQDN(dnsSuffix string) (string, string) {
	h := fnv.New32a()
	if _, err := h.Write([]byte(fmt.Sprintf("%s/%s/%s", c.SubscriptionID(), c.ResourceGroup(), c.ClusterName()))); err != nil {
		return "", ""
	}
	ipName := fmt.Sprintf("%s-%x", c.ClusterName(), h.Sum32())
	fqdn := fmt.Sprintf("%s.%s.%s", ipName, c.Location(), dnsSuffix)
	return ipName, fqdn
}

// GenerateFQDN generates a fully qualified domain name, based on a hash, cluster name and cluster location.
func (c *AzureCluster) GenerateFQDN(dnsSuffix, ipName string) string {
	h := fnv.New32a()
	if _, err := h.Write([]byte(fmt.Sprintf("%s/%s/%s", c.SubscriptionID(), c.ResourceGroup(), ipName))); err != nil {
		return ""
	}
	hash := fmt.Sprintf("%x", h.Sum32())
	return strings.ToLower(fmt.Sprintf("%s-%s.%s.%s", c.ClusterName(), hash, c.Location(), dnsSuffix))
}

// GetConditions returns the list of conditions for an AzureCluster API object.
func (c *AzureCluster) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions will set the given conditions on an AzureCluster object
func (c *AzureCluster) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&AzureCluster{}, &AzureClusterList{})
}

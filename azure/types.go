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

package azure

import (
	"reflect"

	"github.com/google/go-cmp/cmp"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

// PublicIPSpec defines the specification for a Public IP.
type PublicIPSpec struct {
	Name    string
	DNSName string
	IsIPv6  bool
}

// NICSpec defines the specification for a Network Interface.
type NICSpec struct {
	Name                      string
	MachineName               string
	SubnetName                string
	VNetName                  string
	VNetResourceGroup         string
	StaticIPAddress           string
	PublicLBName              string
	PublicLBAddressPoolName   string
	PublicLBNATRuleName       string
	InternalLBName            string
	InternalLBAddressPoolName string
	PublicIPName              string
	VMSize                    string
	AcceleratedNetworking     *bool
	IPv6Enabled               bool
	EnableIPForwarding        bool
}

// LBSpec defines the specification for a Load Balancer.
type LBSpec struct {
	Name                 string
	Role                 string
	Type                 infrav1.LBType
	SKU                  infrav1.SKU
	SubnetName           string
	BackendPoolName      string
	FrontendIPConfigs    []infrav1.FrontendIP
	APIServerPort        int32
	IdleTimeoutInMinutes *int32
}

// SubnetSpec defines the specification for a Subnet.
type SubnetSpec struct {
	Name              string
	CIDRs             []string
	VNetName          string
	RouteTableName    string
	SecurityGroupName string
	Role              infrav1.SubnetRole
	NatGatewayName    string
}

// VNetSpec defines the specification for a Virtual Network.
type VNetSpec struct {
	ResourceGroup string
	Name          string
	CIDRs         []string
	Peerings      []infrav1.VnetPeeringSpec
}

// RoleAssignmentSpec defines the specification for a Role Assignment.
type RoleAssignmentSpec struct {
	MachineName  string
	Name         string
	ResourceType string
}

// ResourceType defines the type azure resource being reconciled.
// Eg. Virtual Machine, Virtual Machine Scale Sets.
type ResourceType string

const (

	// VirtualMachine ...
	VirtualMachine = "VirtualMachine"

	// VirtualMachineScaleSet ...
	VirtualMachineScaleSet = "VirtualMachineScaleSet"
)

// NSGSpec defines the specification for a Security Group.
type NSGSpec struct {
	Name          string
	SecurityRules infrav1.SecurityRules
}

// BastionSpec defines the specification for the generic bastion feature.
type BastionSpec struct {
	AzureBastion *AzureBastionSpec
}

// AzureBastionSpec defines the specification for azure bastion feature.
type AzureBastionSpec struct { // nolint
	Name         string
	SubnetSpec   infrav1.SubnetSpec
	PublicIPName string
	VNetName     string
}

// ScaleSetSpec defines the specification for a Scale Set.
type ScaleSetSpec struct {
	Name                         string
	Size                         string
	Capacity                     int64
	SSHKeyData                   string
	OSDisk                       infrav1.OSDisk
	DataDisks                    []infrav1.DataDisk
	SubnetName                   string
	VNetName                     string
	VNetResourceGroup            string
	PublicLBName                 string
	PublicLBAddressPoolName      string
	AcceleratedNetworking        *bool
	TerminateNotificationTimeout *int
	Identity                     infrav1.VMIdentity
	UserAssignedIdentities       []infrav1.UserAssignedIdentity
	SecurityProfile              *infrav1.SecurityProfile
	SpotVMOptions                *infrav1.SpotVMOptions
	FailureDomains               []string
}

// TagsSpec defines the specification for a set of tags.
type TagsSpec struct {
	Scope string
	Tags  infrav1.Tags
	// Annotation is the key which stores the last applied tags as value in JSON format.
	// The last applied tags are used to find out which tags are being managed by CAPZ
	// and if any has to be deleted by comparing it with the new desired tags
	Annotation string
}

// PrivateDNSSpec defines the specification for a private DNS zone.
type PrivateDNSSpec struct {
	ZoneName string
	Links    []PrivateDNSLinkSpec
	Records  []infrav1.AddressRecord
}

// PrivateDNSLinkSpec defines the specification for a virtual network link in a private DNS zone.
type PrivateDNSLinkSpec struct {
	VNetName          string
	VNetResourceGroup string
	LinkName          string
}

// ExtensionSpec defines the specification for a VM or VMScaleSet extension.
type ExtensionSpec struct {
	Name              string
	VMName            string
	Publisher         string
	Version           string
	ProtectedSettings map[string]string
}

type (
	// VMSSVM defines a VM in a virtual machine scale set.
	VMSSVM struct {
		ID               string                    `json:"id,omitempty"`
		InstanceID       string                    `json:"instanceID,omitempty"`
		Image            infrav1.Image             `json:"image,omitempty"`
		Name             string                    `json:"name,omitempty"`
		AvailabilityZone string                    `json:"availabilityZone,omitempty"`
		State            infrav1.ProvisioningState `json:"vmState,omitempty"`
	}

	// VMSS defines a virtual machine scale set.
	VMSS struct {
		ID        string                    `json:"id,omitempty"`
		Name      string                    `json:"name,omitempty"`
		Sku       string                    `json:"sku,omitempty"`
		Capacity  int64                     `json:"capacity,omitempty"`
		Zones     []string                  `json:"zones,omitempty"`
		Image     infrav1.Image             `json:"image,omitempty"`
		State     infrav1.ProvisioningState `json:"vmState,omitempty"`
		Identity  infrav1.VMIdentity        `json:"identity,omitempty"`
		Tags      infrav1.Tags              `json:"tags,omitempty"`
		Instances []VMSSVM                  `json:"instances,omitempty"`
	}
)

// HasModelChanges returns true if the spec fields which will mutate the Azure VMSS model are different.
func (vmss VMSS) HasModelChanges(other VMSS) bool {
	equal := cmp.Equal(vmss.Image, other.Image) &&
		cmp.Equal(vmss.Identity, other.Identity) &&
		cmp.Equal(vmss.Zones, other.Zones) &&
		cmp.Equal(vmss.Tags, other.Tags) &&
		cmp.Equal(vmss.Sku, other.Sku)
	return !equal
}

// InstancesByProviderID returns VMSSVMs by ID.
func (vmss VMSS) InstancesByProviderID() map[string]VMSSVM {
	instancesByProviderID := make(map[string]VMSSVM, len(vmss.Instances))
	for _, instance := range vmss.Instances {
		instancesByProviderID[instance.ProviderID()] = instance
	}

	return instancesByProviderID
}

// ProviderID returns the K8s provider ID for the VMSS instance.
func (vm VMSSVM) ProviderID() string {
	return ProviderIDPrefix + vm.ID
}

// HasLatestModelAppliedToAll returns true if all VMSS instance have the latest model applied.
func (vmss VMSS) HasLatestModelAppliedToAll() bool {
	for _, instance := range vmss.Instances {
		if !vmss.HasLatestModelApplied(instance) {
			return false
		}
	}

	return true
}

// HasEnoughLatestModelOrNotMixedModel returns true if VMSS instance have the latest model applied to all or equal to the capacity.
func (vmss VMSS) HasEnoughLatestModelOrNotMixedModel() bool {
	if vmss.HasLatestModelAppliedToAll() {
		return true
	}

	counter := int64(0)
	for _, instance := range vmss.Instances {
		if vmss.HasLatestModelApplied(instance) {
			counter++
		}
	}

	return counter == vmss.Capacity
}

// HasLatestModelApplied returns true if the VMSS instance matches the VMSS image reference.
func (vmss VMSS) HasLatestModelApplied(vm VMSSVM) bool {
	// if the images match, then the VM is of the same model
	return reflect.DeepEqual(vm.Image, vmss.Image)
}

// ManagedClusterSpec contains properties to create a managed cluster.
type ManagedClusterSpec struct {
	// Name is the name of this AKS Cluster.
	Name string

	// ResourceGroupName is the name of the Azure resource group for this AKS Cluster.
	ResourceGroupName string

	// NodeResourceGroupName is the name of the Azure resource group containing IaaS VMs.
	NodeResourceGroupName string

	// VnetSubnetID is the Azure Resource ID for the subnet which should contain nodes.
	VnetSubnetID string

	// Location is a string matching one of the canonical Azure region names. Examples: "westus2", "eastus".
	Location string

	// Tags is a set of tags to add to this cluster.
	Tags map[string]string

	// Version defines the desired Kubernetes version.
	Version string

	// LoadBalancerSKU for the managed cluster. Possible values include: 'Standard', 'Basic'. Defaults to Standard.
	LoadBalancerSKU string

	// NetworkPlugin used for building Kubernetes network. Possible values include: 'azure', 'kubenet'. Defaults to azure.
	NetworkPlugin string

	// NetworkPolicy used for building Kubernetes network. Possible values include: 'calico', 'azure'. Defaults to azure.
	NetworkPolicy string

	// SSHPublicKey is a string literal containing an ssh public key. Will autogenerate and discard if not provided.
	SSHPublicKey string

	// AgentPools is the list of agent pool specifications in this cluster.
	AgentPools []AgentPoolSpec

	// PodCIDR is the CIDR block for IP addresses distributed to pods
	PodCIDR string

	// ServiceCIDR is the CIDR block for IP addresses distributed to services
	ServiceCIDR string

	// DNSServiceIP is an IP address assigned to the Kubernetes DNS service
	DNSServiceIP *string

	// AADProfile is Azure Active Directory configuration to integrate with AKS, for aad authentication.
	AADProfile *AADProfile

	// SKU is the SKU of the AKS to be provisioned.
	SKU *SKU

	// LoadBalancerProfile is the profile of the cluster load balancer.
	LoadBalancerProfile *LoadBalancerProfile

	// APIServerAccessProfile is the access profile for AKS API server.
	APIServerAccessProfile *APIServerAccessProfile
}

// AADProfile is Azure Active Directory configuration to integrate with AKS, for aad authentication.
type AADProfile struct {
	// Managed - Whether to enable managed AAD.
	Managed bool

	// EnableAzureRBAC - Whether to enable Azure RBAC for Kubernetes authorization.
	EnableAzureRBAC bool

	// AdminGroupObjectIDs - AAD group object IDs that will have admin role of the cluster.
	AdminGroupObjectIDs []string
}

// SKU - AKS SKU.
type SKU struct {
	// Tier - Tier of a managed cluster SKU.
	Tier string
}

// LoadBalancerProfile - Profile of the cluster load balancer.
type LoadBalancerProfile struct {
	// Load balancer profile must specify at most one of ManagedOutboundIPs, OutboundIPPrefixes and OutboundIPs.
	// By default the AKS cluster automatically creates a public IP in the AKS-managed infrastructure resource group and assigns it to the load balancer outbound pool.
	// Alternatively, you can assign your own custom public IP or public IP prefix at cluster creation time.
	// See https://docs.microsoft.com/en-us/azure/aks/load-balancer-standard#provide-your-own-outbound-public-ips-or-prefixes

	// ManagedOutboundIPs - Desired managed outbound IPs for the cluster load balancer.
	ManagedOutboundIPs *int32

	// OutboundIPPrefixes - Desired outbound IP Prefix resources for the cluster load balancer.
	OutboundIPPrefixes []string

	// OutboundIPs - Desired outbound IP resources for the cluster load balancer.
	OutboundIPs []string

	// AllocatedOutboundPorts - Desired number of allocated SNAT ports per VM. Allowed values must be in the range of 0 to 64000 (inclusive). The default value is 0 which results in Azure dynamically allocating ports.
	AllocatedOutboundPorts *int32

	// IdleTimeoutInMinutes - Desired outbound flow idle timeout in minutes. Allowed values must be in the range of 4 to 120 (inclusive). The default value is 30 minutes.
	IdleTimeoutInMinutes *int32
}

// APIServerAccessProfile is the access profile for AKS API server.
type APIServerAccessProfile struct {
	// AuthorizedIPRanges - Authorized IP Ranges to kubernetes API server.
	AuthorizedIPRanges []string
	// EnablePrivateCluster - Whether to create the cluster as a private cluster or not.
	EnablePrivateCluster *bool
	// PrivateDNSZone - Private dns zone mode for private cluster.
	PrivateDNSZone *string
	// EnablePrivateClusterPublicFQDN - Whether to create additional public FQDN for private cluster or not.
	EnablePrivateClusterPublicFQDN *bool
}

// AgentPoolSpec contains agent pool specification details.
type AgentPoolSpec struct {
	// Name is the name of agent pool.
	Name string

	// ResourceGroup is the name of the Azure resource group for the AKS Cluster.
	ResourceGroup string

	// Cluster is the name of the AKS cluster.
	Cluster string

	// Version defines the desired Kubernetes version.
	Version *string

	// SKU defines the Azure VM size for the agent pool VMs.
	SKU string

	// Replicas is the number of desired machines.
	Replicas int32

	// OSDiskSizeGB is the OS disk size in GB for every machine in this agent pool.
	OSDiskSizeGB int32

	// VnetSubnetID is the Azure Resource ID for the subnet which should contain nodes.
	VnetSubnetID string

	// Mode represents mode of an agent pool. Possible values include: 'System', 'User'.
	Mode string

	//  Maximum number of nodes for auto-scaling
	MaxCount *int32 `json:"maxCount,omitempty"`

	// Minimum number of nodes for auto-scaling
	MinCount *int32 `json:"minCount,omitempty"`

	// EnableAutoScaling - Whether to enable auto-scaler
	EnableAutoScaling *bool `json:"enableAutoScaling,omitempty"`

	// AvailabilityZones represents the Availability zones for nodes in the AgentPool.
	AvailabilityZones []string

	// MaxPods specifies the kubelet --max-pods configuration for the agent pool.
	MaxPods *int32 `json:"maxPods,omitempty"`

	// OsDiskType specifies the OS disk type for each node in the pool. Allowed values are 'Ephemeral' and 'Managed'.
	OsDiskType *string `json:"osDiskType,omitempty"`
}

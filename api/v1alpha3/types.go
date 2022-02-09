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
	corev1 "k8s.io/api/core/v1"
)

const (
	// ControlPlane machine label.
	ControlPlane string = "control-plane"
	// Node machine label.
	Node string = "node"
)

// Future contains the data needed for an Azure long-running operation to continue across reconcile loops.
type Future struct {
	// Type describes the type of future, update, create, delete, etc.
	Type string `json:"type"`

	// ResourceGroup is the Azure resource group for the resource.
	// +optional
	ResourceGroup string `json:"resourceGroup,omitempty"`

	// Name is the name of the Azure resource.
	// +optional
	Name string `json:"name,omitempty"`

	// FutureData is the base64 url encoded json Azure AutoRest Future.
	FutureData string `json:"futureData,omitempty"`
}

// NetworkSpec specifies what the Azure networking resources should look like.
type NetworkSpec struct {
	// Vnet is the configuration for the Azure virtual network.
	// +optional
	Vnet VnetSpec `json:"vnet,omitempty"`

	// Subnets is the configuration for the control-plane subnet and the node subnet.
	// +optional
	Subnets Subnets `json:"subnets,omitempty"`

	// APIServerLB is the configuration for the control-plane load balancer.
	// +optional
	APIServerLB LoadBalancerSpec `json:"apiServerLB,omitempty"`
}

// VnetSpec configures an Azure virtual network.
type VnetSpec struct {
	// ResourceGroup is the name of the resource group of the existing virtual network
	// or the resource group where a managed virtual network should be created.
	ResourceGroup string `json:"resourceGroup,omitempty"`

	// ID is the identifier of the virtual network this provider should use to create resources.
	ID string `json:"id,omitempty"`

	// Name defines a name for the virtual network resource.
	Name string `json:"name"`

	// CidrBlock is the CIDR block to be used when the provider creates a managed virtual network.
	// Deprecated: Use CIDRBlocks instead
	// +optional
	CidrBlock string `json:"cidrBlock,omitempty"`

	// CIDRBlocks defines the virtual network's address space, specified as one or more address prefixes in CIDR notation.
	// +optional
	CIDRBlocks []string `json:"cidrBlocks,omitempty"`

	// Tags is a collection of tags describing the resource.
	// +optional
	Tags Tags `json:"tags,omitempty"`
}

// IsManaged returns true if the vnet is managed.
func (v *VnetSpec) IsManaged(clusterName string) bool {
	return v.ID == "" || v.Tags.HasOwned(clusterName)
}

// Subnets is a slice of Subnet.
type Subnets []SubnetSpec

// SecurityGroupRole defines the unique role of a security group.
type SecurityGroupRole string

const (
	// SecurityGroupNode defines a Kubernetes workload node role.
	SecurityGroupNode = SecurityGroupRole(Node)

	// SecurityGroupControlPlane defines a Kubernetes control plane node role.
	SecurityGroupControlPlane = SecurityGroupRole(ControlPlane)
)

// SecurityGroup defines an Azure security group.
type SecurityGroup struct {
	ID           string       `json:"id,omitempty"`
	Name         string       `json:"name,omitempty"`
	IngressRules IngressRules `json:"ingressRule,omitempty"`
	Tags         Tags         `json:"tags,omitempty"`
}

// RouteTable defines an Azure route table.
type RouteTable struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// SecurityGroupProtocol defines the protocol type for a security group rule.
type SecurityGroupProtocol string

const (
	// SecurityGroupProtocolAll is a wildcard for all IP protocols.
	SecurityGroupProtocolAll = SecurityGroupProtocol("*")

	// SecurityGroupProtocolTCP represents the TCP protocol in ingress rules.
	SecurityGroupProtocolTCP = SecurityGroupProtocol("Tcp")

	// SecurityGroupProtocolUDP represents the UDP protocol in ingress rules.
	SecurityGroupProtocolUDP = SecurityGroupProtocol("Udp")
)

// IngressRule defines an Azure ingress rule for security groups.
type IngressRule struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Protocol    SecurityGroupProtocol `json:"protocol"`

	// Priority - A number between 100 and 4096. Each rule should have a unique value for priority. Rules are processed in priority order, with lower numbers processed before higher numbers. Once traffic matches a rule, processing stops.
	Priority int32 `json:"priority,omitempty"`

	// SourcePorts - The source port or range. Integer or range between 0 and 65535. Asterix '*' can also be used to match all ports.
	SourcePorts *string `json:"sourcePorts,omitempty"`

	// DestinationPorts - The destination port or range. Integer or range between 0 and 65535. Asterix '*' can also be used to match all ports.
	DestinationPorts *string `json:"destinationPorts,omitempty"`

	// Source - The CIDR or source IP range. Asterix '*' can also be used to match all source IPs. Default tags such as 'VirtualNetwork', 'AzureLoadBalancer' and 'Internet' can also be used. If this is an ingress rule, specifies where network traffic originates from.
	Source *string `json:"source,omitempty"`

	// Destination - The destination address prefix. CIDR or destination IP range. Asterix '*' can also be used to match all source IPs. Default tags such as 'VirtualNetwork', 'AzureLoadBalancer' and 'Internet' can also be used.
	Destination *string `json:"destination,omitempty"`
}

// IngressRules is a slice of Azure ingress rules for security groups.
type IngressRules []IngressRule

// LoadBalancerSpec defines an Azure load balancer.
type LoadBalancerSpec struct {
	ID          string       `json:"id,omitempty"`
	Name        string       `json:"name,omitempty"`
	SKU         SKU          `json:"sku,omitempty"`
	FrontendIPs []FrontendIP `json:"frontendIPs,omitempty"`
	Type        LBType       `json:"type,omitempty"`
}

// SKU defines an Azure load balancer SKU.
type SKU string

const (
	// SKUStandard is the value for the Azure load balancer Standard SKU.
	SKUStandard = SKU("Standard")
)

// LBType defines an Azure load balancer Type.
type LBType string

const (
	// Internal is the value for the Azure load balancer internal type.
	Internal = LBType("Internal")
	// Public is the value for the Azure load balancer public type.
	Public = LBType("Public")
)

// FrontendIP defines a load balancer frontend IP configuration.
type FrontendIP struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +optional
	PrivateIPAddress string `json:"privateIP,omitempty"`
	// +optional
	PublicIP *PublicIPSpec `json:"publicIP,omitempty"`
}

// PublicIPSpec defines the inputs to create an Azure public IP address.
type PublicIPSpec struct {
	Name string `json:"name"`
	// +optional
	DNSName string `json:"dnsName,omitempty"`
}

// VMState describes the state of an Azure virtual machine.
type VMState string

const (
	// VMStateCreating ...
	VMStateCreating VMState = "Creating"
	// VMStateDeleting ...
	VMStateDeleting VMState = "Deleting"
	// VMStateFailed ...
	VMStateFailed VMState = "Failed"
	// VMStateMigrating ...
	VMStateMigrating VMState = "Migrating"
	// VMStateSucceeded ...
	VMStateSucceeded VMState = "Succeeded"
	// VMStateUpdating ...
	VMStateUpdating VMState = "Updating"
	// VMStateDeleted represents a deleted VM
	// NOTE: This state is specific to capz, and does not have corresponding mapping in Azure API (https://docs.microsoft.com/en-us/azure/virtual-machines/states-lifecycle#provisioning-states)
	VMStateDeleted VMState = "Deleted"
)

// VM describes an Azure virtual machine.
type VM struct {
	ID               string `json:"id,omitempty"`
	Name             string `json:"name,omitempty"`
	AvailabilityZone string `json:"availabilityZone,omitempty"`
	// Hardware profile
	VMSize string `json:"vmSize,omitempty"`
	// Storage profile
	Image         Image  `json:"image,omitempty"`
	OSDisk        OSDisk `json:"osDisk,omitempty"`
	StartupScript string `json:"startupScript,omitempty"`
	// State - The provisioning state, which only appears in the response.
	State    VMState    `json:"vmState,omitempty"`
	Identity VMIdentity `json:"identity,omitempty"`
	Tags     Tags       `json:"tags,omitempty"`

	// Addresses contains the addresses associated with the Azure VM.
	Addresses []corev1.NodeAddress `json:"addresses,omitempty"`
}

// Image defines information about the image to use for VM creation.
// There are three ways to specify an image: by ID, Marketplace Image or SharedImageGallery
// One of ID, SharedImage or Marketplace should be set.
type Image struct {
	// ID specifies an image to use by ID
	// +optional
	ID *string `json:"id,omitempty"`

	// SharedGallery specifies an image to use from an Azure Shared Image Gallery
	// +optional
	SharedGallery *AzureSharedGalleryImage `json:"sharedGallery,omitempty"`

	// Marketplace specifies an image to use from the Azure Marketplace
	// +optional
	Marketplace *AzureMarketplaceImage `json:"marketplace,omitempty"`
}

// AzureMarketplaceImage defines an image in the Azure Marketplace to use for VM creation.
type AzureMarketplaceImage struct {
	// Publisher is the name of the organization that created the image
	// +kubebuilder:validation:MinLength=1
	Publisher string `json:"publisher"`
	// Offer specifies the name of a group of related images created by the publisher.
	// For example, UbuntuServer, WindowsServer
	// +kubebuilder:validation:MinLength=1
	Offer string `json:"offer"`
	// SKU specifies an instance of an offer, such as a major release of a distribution.
	// For example, 18.04-LTS, 2019-Datacenter
	// +kubebuilder:validation:MinLength=1
	SKU string `json:"sku"`
	// Version specifies the version of an image sku. The allowed formats
	// are Major.Minor.Build or 'latest'. Major, Minor, and Build are decimal numbers.
	// Specify 'latest' to use the latest version of an image available at deploy time.
	// Even if you use 'latest', the VM image will not automatically update after deploy
	// time even if a new version becomes available.
	// +kubebuilder:validation:MinLength=1
	Version string `json:"version"`
	// ThirdPartyImage indicates the image is published by a third party publisher and a Plan
	// will be generated for it.
	// +kubebuilder:default=false
	// +optional
	ThirdPartyImage bool `json:"thirdPartyImage"`
}

// AzureSharedGalleryImage defines an image in a Shared Image Gallery to use for VM creation.
type AzureSharedGalleryImage struct {
	// SubscriptionID is the identifier of the subscription that contains the shared image gallery
	// +kubebuilder:validation:MinLength=1
	SubscriptionID string `json:"subscriptionID"`
	// ResourceGroup specifies the resource group containing the shared image gallery
	// +kubebuilder:validation:MinLength=1
	ResourceGroup string `json:"resourceGroup"`
	// Gallery specifies the name of the shared image gallery that contains the image
	// +kubebuilder:validation:MinLength=1
	Gallery string `json:"gallery"`
	// Name is the name of the image
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Version specifies the version of the marketplace image. The allowed formats
	// are Major.Minor.Build or 'latest'. Major, Minor, and Build are decimal numbers.
	// Specify 'latest' to use the latest version of an image available at deploy time.
	// Even if you use 'latest', the VM image will not automatically update after deploy
	// time even if a new version becomes available.
	// +kubebuilder:validation:MinLength=1
	Version string `json:"version"`
}

// AvailabilityZone specifies an Azure Availability Zone.
//
// Deprecated: Use FailureDomain instead.
type AvailabilityZone struct {
	ID      *string `json:"id,omitempty"`
	Enabled *bool   `json:"enabled,omitempty"`
}

// VMIdentity defines the identity of the virtual machine, if configured.
// +kubebuilder:validation:Enum=None;SystemAssigned;UserAssigned
type VMIdentity string

const (
	// VMIdentityNone ...
	VMIdentityNone VMIdentity = "None"
	// VMIdentitySystemAssigned ...
	VMIdentitySystemAssigned VMIdentity = "SystemAssigned"
	// VMIdentityUserAssigned ...
	VMIdentityUserAssigned VMIdentity = "UserAssigned"
)

// UserAssignedIdentity defines the user-assigned identities provided
// by the user to be assigned to Azure resources.
type UserAssignedIdentity struct {
	// ProviderID is the identification ID of the user-assigned Identity, the format of an identity is:
	// 'azure:///subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.ManagedIdentity/userAssignedIdentities/{identityName}'
	ProviderID string `json:"providerID"`
}

const (
	// AzureIdentityBindingSelector is the label used to match with the AzureIdentityBinding
	// For the controller to match an identity binding, it needs a [label] with the key `aadpodidbinding`
	// whose value is that of the `selector:` field in the `AzureIdentityBinding`.
	AzureIdentityBindingSelector = "capz-controller-aadpodidentity-selector"
)

// IdentityType represents different types of identities.
// +kubebuilder:validation:Enum=ServicePrincipal;UserAssignedMSI
type IdentityType string

const (
	// UserAssignedMSI represents a user-assigned identity.
	UserAssignedMSI IdentityType = "UserAssignedMSI"

	// ServicePrincipal represents a service principal.
	ServicePrincipal IdentityType = "ServicePrincipal"
)

// OSDisk defines the operating system disk for a VM.
//
// WARNING: this requires any updates to ManagedDisk to be manually converted. This is due to the odd issue with
// conversion-gen where the warning message generated uses a relative directory import rather than the fully
// qualified import when generating outside of the GOPATH.
// +k8s:conversion-gen=false
type OSDisk struct {
	OSType           string            `json:"osType"`
	DiskSizeGB       int32             `json:"diskSizeGB"`
	ManagedDisk      ManagedDisk       `json:"managedDisk"`
	DiffDiskSettings *DiffDiskSettings `json:"diffDiskSettings,omitempty"`
	// +optional
	CachingType string `json:"cachingType,omitempty"`
}

// DataDisk specifies the parameters that are used to add one or more data disks to the machine.
type DataDisk struct {
	// NameSuffix is the suffix to be appended to the machine name to generate the disk name.
	// Each disk name will be in format <machineName>_<nameSuffix>.
	NameSuffix string `json:"nameSuffix"`
	// DiskSizeGB is the size in GB to assign to the data disk.
	DiskSizeGB int32 `json:"diskSizeGB"`
	// +optional
	ManagedDisk *ManagedDisk `json:"managedDisk,omitempty"`
	// Lun Specifies the logical unit number of the data disk. This value is used to identify data disks within the VM and therefore must be unique for each data disk attached to a VM.
	// The value must be between 0 and 63.
	Lun *int32 `json:"lun,omitempty"`
	// +optional
	CachingType string `json:"cachingType,omitempty"`
}

// ManagedDisk defines the managed disk options for a VM.
type ManagedDisk struct {
	StorageAccountType string                       `json:"storageAccountType"`
	DiskEncryptionSet  *DiskEncryptionSetParameters `json:"diskEncryptionSet,omitempty"`
}

// DiskEncryptionSetParameters defines disk encryption options.
type DiskEncryptionSetParameters struct {
	// ID defines resourceID for diskEncryptionSet resource. It must be in the same subscription
	ID string `json:"id,omitempty"`
}

// DiffDiskSettings describe ephemeral disk settings for the os disk.
type DiffDiskSettings struct {
	// Option enables ephemeral OS when set to "Local"
	// See https://docs.microsoft.com/en-us/azure/virtual-machines/ephemeral-os-disks for full details
	// +kubebuilder:validation:Enum=Local
	Option string `json:"option"`
}

// SubnetRole defines the unique role of a subnet.
type SubnetRole string

const (
	// SubnetNode defines a Kubernetes workload node role.
	SubnetNode = SubnetRole(Node)

	// SubnetControlPlane defines a Kubernetes control plane node role.
	SubnetControlPlane = SubnetRole(ControlPlane)
)

// SubnetSpec configures an Azure subnet.
type SubnetSpec struct {
	// Role defines the subnet role (eg. Node, ControlPlane)
	Role SubnetRole `json:"role,omitempty"`

	// ID defines a unique identifier to reference this resource.
	// +optional
	ID string `json:"id,omitempty"`

	// Name defines a name for the subnet resource.
	Name string `json:"name"`

	// CidrBlock is the CIDR block to be used when the provider creates a managed Vnet.
	// Deprecated: Use CIDRBlocks instead
	// +optional
	CidrBlock string `json:"cidrBlock,omitempty"`

	// CIDRBlocks defines the subnet's address space, specified as one or more address prefixes in CIDR notation.
	// +optional
	CIDRBlocks []string `json:"cidrBlocks,omitempty"`

	// InternalLBIPAddress is the IP address that will be used as the internal LB private IP.
	// For the control plane subnet only.
	// +optional
	// Deprecated: Use LoadBalancer private IP instead
	InternalLBIPAddress string `json:"internalLBIPAddress,omitempty"`

	// SecurityGroup defines the NSG (network security group) that should be attached to this subnet.
	// +optional
	SecurityGroup SecurityGroup `json:"securityGroup,omitempty"`

	// RouteTable defines the route table that should be attached to this subnet.
	// +optional
	RouteTable RouteTable `json:"routeTable,omitempty"`
}

// GetControlPlaneSubnet returns the cluster control plane subnet.
func (n *NetworkSpec) GetControlPlaneSubnet() *SubnetSpec {
	for _, sn := range n.Subnets {
		if sn.Role == SubnetControlPlane {
			return &sn
		}
	}
	return nil
}

// GetNodeSubnet returns the cluster node subnet.
func (n *NetworkSpec) GetNodeSubnet() *SubnetSpec {
	for _, sn := range n.Subnets {
		if sn.Role == SubnetNode {
			return &sn
		}
	}
	return nil
}

// SecurityProfile specifies the Security profile settings for a
// virtual machine or virtual machine scale set.
type SecurityProfile struct {
	// This field indicates whether Host Encryption should be enabled
	// or disabled for a virtual machine or virtual machine scale
	// set. Default is disabled.
	EncryptionAtHost *bool `json:"encryptionAtHost,omitempty"`
}

// AddressRecord specifies a DNS record mapping a hostname to an IPV4 or IPv6 address.
type AddressRecord struct {
	Hostname string
	IP       string
}

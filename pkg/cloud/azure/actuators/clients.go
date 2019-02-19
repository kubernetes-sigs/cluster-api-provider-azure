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

package actuators

import (
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2018-10-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"github.com/Azure/go-autorest/autorest"
	providerv1 "sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

// AzureClients contains all the Azure clients used by the scopes.
type AzureClients struct {
	// TODO: Remove legacy clients once interfaces are reimplemented
	Compute   AzureComputeClient
	Network   AzureNetworkClient
	Resources AzureResourcesClient

	// Compute
	VM    compute.VirtualMachinesClient
	Disks compute.DisksClient

	// Network
	VirtualNetworks   network.VirtualNetworksClient
	SecurityGroups    network.SecurityGroupsClient
	Interfaces        network.InterfacesClient
	LB                network.LoadBalancersClient
	PublicIPAddresses network.PublicIPAddressesClient

	// Resources
	Groups      resources.GroupsClient
	Deployments resources.DeploymentsClient
	Tags        resources.TagsClient
}

// AzureComputeClient defines the operations that will interact with the Azure Compute API
type AzureComputeClient interface {
	// Virtual Machines Operations
	RunCommand(resoureGroup string, name string, cmd string) (compute.VirtualMachinesRunCommandFuture, error)
	VMIfExists(resourceGroup string, name string) (*compute.VirtualMachine, error)
	DeleteVM(resourceGroup string, name string) (compute.VirtualMachinesDeleteFuture, error)
	WaitForVMRunCommandFuture(future compute.VirtualMachinesRunCommandFuture) error
	WaitForVMDeletionFuture(future compute.VirtualMachinesDeleteFuture) error

	// Disk Operations
	DeleteManagedDisk(resourceGroup string, name string) (compute.DisksDeleteFuture, error)
	WaitForDisksDeleteFuture(future compute.DisksDeleteFuture) error
}

// AzureNetworkClient defines the operations that will interact with the Azure Network API
type AzureNetworkClient interface {
	// Network Interfaces Operations
	DeleteNetworkInterface(resourceGroupName string, networkInterfaceName string) (network.InterfacesDeleteFuture, error)
	WaitForNetworkInterfacesDeleteFuture(future network.InterfacesDeleteFuture) error

	// Network Security Groups Operations
	CreateOrUpdateNetworkSecurityGroup(resourceGroupName string, networkSecurityGroupName string, location string) (*network.SecurityGroupsCreateOrUpdateFuture, error)
	NetworkSGIfExists(resourceGroupName string, networkSecurityGroupName string) (*network.SecurityGroup, error)
	WaitForNetworkSGsCreateOrUpdateFuture(future network.SecurityGroupsCreateOrUpdateFuture) error

	// Public Ip Address Operations
	CreateOrUpdatePublicIPAddress(resourceGroupName string, IPName string) (network.PublicIPAddress, error)
	DeletePublicIPAddress(resourceGroup string, IPName string) (network.PublicIPAddressesDeleteFuture, error)
	WaitForPublicIPAddressDeleteFuture(future network.PublicIPAddressesDeleteFuture) error

	// Virtual Networks Operations
	CreateOrUpdateVnet(resourceGroupName string, virtualNetworkName string, location string) (*network.VirtualNetworksCreateOrUpdateFuture, error)
	WaitForVnetCreateOrUpdateFuture(future network.VirtualNetworksCreateOrUpdateFuture) error
}

// AzureResourcesClient defines the operations that will interact with the Azure Resources API
type AzureResourcesClient interface {
	// Resource Groups Operations
	CreateOrUpdateGroup(resourceGroupName string, location string) (resources.Group, error)
	DeleteGroup(resourceGroupName string) (resources.GroupsDeleteFuture, error)
	CheckGroupExistence(rgName string) (autorest.Response, error)
	WaitForGroupsDeleteFuture(future resources.GroupsDeleteFuture) error

	// Deployment Operations
	CreateOrUpdateDeployment(machine *clusterv1.Machine, clusterConfig *providerv1.AzureClusterProviderSpec, machineConfig *providerv1.AzureMachineProviderSpec, startupScript string) (*resources.DeploymentsCreateOrUpdateFuture, error)
	GetDeploymentResult(future resources.DeploymentsCreateOrUpdateFuture) (de resources.DeploymentExtended, err error)
	ValidateDeployment(machine *clusterv1.Machine, clusterConfig *providerv1.AzureClusterProviderSpec, machineConfig *providerv1.AzureMachineProviderSpec, startupScript string) error
	WaitForDeploymentsCreateOrUpdateFuture(future resources.DeploymentsCreateOrUpdateFuture) error
}

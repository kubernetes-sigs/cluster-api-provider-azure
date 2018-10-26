/*
Copyright 2018 The Kubernetes Authors.
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
package services

import (
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2018-04-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-01-01/network"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/Azure/go-autorest/autorest"
)

// interface for all azure services clients
type AzureClients struct {
	Compute            AzureComputeClient
	Network            AzureNetworkClient
	Resourcemanagement AzureResourceManagementClient
}

type AzureComputeClient interface {
	// Virtual Machines Operations
	VmIfExists(resourceGroup string, name string) (*compute.VirtualMachine, error)
	DeleteVM(resourceGroup string, name string) (compute.VirtualMachinesDeleteFuture, error)
	WaitForVMDeletionFuture(future compute.VirtualMachinesDeleteFuture) error

	// Disk Operations
	DeleteManagedDisk(resourceGroup string, name string) (compute.DisksDeleteFuture, error)
	WaitForDisksDeleteFuture(future compute.DisksDeleteFuture) error
}

type AzureNetworkClient interface {
	// Network Interfaces Operations
	DeleteNetworkInterface(resourceGroupName string, networkInterfaceName string) (network.InterfacesDeleteFuture, error)
	WaitForNetworkInterfacesDeleteFuture(future network.InterfacesDeleteFuture) error

	// Network Security Groups Operations
	CreateOrUpdateNetworkSecurityGroup(resourceGroupName string, networkSecurityGroupName string, location string) (*network.SecurityGroupsCreateOrUpdateFuture, error)
	NetworkSGIfExists(resourceGroupName string, networkSecurityGroupName string) (*network.SecurityGroup, error)
	WaitForNetworkSGsCreateOrUpdateFuture(future network.SecurityGroupsCreateOrUpdateFuture) error

	// Public Ip Address Operations
	DeletePublicIpAddress(resourceGroup string, IPName string) (network.PublicIPAddressesDeleteFuture, error)
	WaitForPublicIpAddressDeleteFuture(future network.PublicIPAddressesDeleteFuture) error

	// Virtual Networks Operations
	CreateOrUpdateVnet(resourceGroupName string, virtualNetworkName string, location string) (*network.VirtualNetworksCreateOrUpdateFuture, error)
	WaitForVnetCreateOrUpdateFuture(future network.VirtualNetworksCreateOrUpdateFuture) error
}

type AzureResourceManagementClient interface {
	// Resource Groups Operations
	CreateOrUpdateGroup(resourceGroupName string, location string) (resources.Group, error)
	DeleteGroup(resourceGroupName string) (resources.GroupsDeleteFuture, error)
	CheckGroupExistence(rgName string) (autorest.Response, error)
	WaitForGroupsDeleteFuture(future resources.GroupsDeleteFuture) error
}

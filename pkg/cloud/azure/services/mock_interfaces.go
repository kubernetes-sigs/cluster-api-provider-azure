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
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2018-10-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-10-01/network"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"github.com/Azure/go-autorest/autorest"
	azureconfigv1 "sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

// MockAzureComputeClient is a mock implementation of AzureComputeClient.
type MockAzureComputeClient struct {
	MockRunCommand                func(resourceGroup string, name string, cmd string) (compute.VirtualMachinesRunCommandFuture, error)
	MockVMIfExists                func(resourceGroup string, name string) (*compute.VirtualMachine, error)
	MockDeleteVM                  func(resourceGroup string, name string) (compute.VirtualMachinesDeleteFuture, error)
	MockWaitForVMRunCommandFuture func(future compute.VirtualMachinesRunCommandFuture) error
	MockWaitForVMDeletionFuture   func(future compute.VirtualMachinesDeleteFuture) error
	MockDeleteManagedDisk         func(resourceGroup string, name string) (compute.DisksDeleteFuture, error)
	MockWaitForDisksDeleteFuture  func(future compute.DisksDeleteFuture) error
}

// MockAzureNetworkClient is a mock implementation of MockAzureNetworkClient.
type MockAzureNetworkClient struct {
	MockDeleteNetworkInterface               func(resourceGroup string, networkInterfaceName string) (network.InterfacesDeleteFuture, error)
	MockWaitForNetworkInterfacesDeleteFuture func(future network.InterfacesDeleteFuture) error

	MockCreateOrUpdateNetworkSecurityGroup    func(resourceGroupName string, networkSecurityGroupName string, location string) (*network.SecurityGroupsCreateOrUpdateFuture, error)
	MockNetworkSGIfExists                     func(resourceGroupName string, networkSecurityGroupName string) (*network.SecurityGroup, error)
	MockWaitForNetworkSGsCreateOrUpdateFuture func(future network.SecurityGroupsCreateOrUpdateFuture) error

	MockGetPublicIPAddress                 func(resourceGroup string, IPName string) (network.PublicIPAddress, error)
	MockDeletePublicIPAddress              func(resourceGroup string, IPName string) (network.PublicIPAddressesDeleteFuture, error)
	MockWaitForPublicIPAddressDeleteFuture func(future network.PublicIPAddressesDeleteFuture) error

	MockCreateOrUpdateVnet              func(resourceGroupName string, virtualNetworkName string, location string) (*network.VirtualNetworksCreateOrUpdateFuture, error)
	MockWaitForVnetCreateOrUpdateFuture func(future network.VirtualNetworksCreateOrUpdateFuture) error
}

// MockAzureResourceManagementClient is a mock implementation of MockAzureResourceManagementClient.
type MockAzureResourceManagementClient struct {
	MockCreateOrUpdateGroup       func(resourceGroupName string, location string) (resources.Group, error)
	MockDeleteGroup               func(resourceGroupName string) (resources.GroupsDeleteFuture, error)
	MockCheckGroupExistence       func(rgName string) (autorest.Response, error)
	MockWaitForGroupsDeleteFuture func(future resources.GroupsDeleteFuture) error

	MockCreateOrUpdateDeployment               func(machine *clusterv1.Machine, clusterConfig *providerv1.AzureClusterProviderSpec, machineConfig *providerv1.AzureMachineProviderSpec) (*resources.DeploymentsCreateOrUpdateFuture, error)
	MockGetDeploymentResult                    func(future resources.DeploymentsCreateOrUpdateFuture) (de resources.DeploymentExtended, err error)
	MockValidateDeployment                     func(machine *clusterv1.Machine, clusterConfig *providerv1.AzureClusterProviderSpec, machineConfig *providerv1.AzureMachineProviderSpec) error
	MockWaitForDeploymentsCreateOrUpdateFuture func(future resources.DeploymentsCreateOrUpdateFuture) error
}

// RunCommand executes a command on the VM.
func (m *MockAzureComputeClient) RunCommand(resourceGroup string, name string, cmd string) (compute.VirtualMachinesRunCommandFuture, error) {
	if m.MockRunCommand == nil {
		return compute.VirtualMachinesRunCommandFuture{}, nil
	}
	return m.MockRunCommand(resourceGroup, name, cmd)
}

// VMIfExists returns the reference to the VM object if it exists.
func (m *MockAzureComputeClient) VMIfExists(resourceGroup string, name string) (*compute.VirtualMachine, error) {
	if m.MockVMIfExists == nil {
		return nil, nil
	}
	return m.MockVMIfExists(resourceGroup, name)
}

// DeleteVM deletes the virtual machine.
func (m *MockAzureComputeClient) DeleteVM(resourceGroup string, name string) (compute.VirtualMachinesDeleteFuture, error) {
	if m.MockDeleteVM == nil {
		return compute.VirtualMachinesDeleteFuture{}, nil
	}
	return m.MockDeleteVM(resourceGroup, name)
}

// DeleteManagedDisk deletes a managed disk resource.
func (m *MockAzureComputeClient) DeleteManagedDisk(resourceGroup string, name string) (compute.DisksDeleteFuture, error) {
	if m.MockDeleteManagedDisk == nil {
		return compute.DisksDeleteFuture{}, nil
	}
	return m.MockDeleteManagedDisk(resourceGroup, name)
}

// WaitForVMRunCommandFuture returns when the RunCommand operation completes.
func (m *MockAzureComputeClient) WaitForVMRunCommandFuture(future compute.VirtualMachinesRunCommandFuture) error {
	if m.MockWaitForVMRunCommandFuture == nil {
		return nil
	}
	return m.MockWaitForVMRunCommandFuture(future)
}

// WaitForVMDeletionFuture returns when the DeleteVM operation completes.
func (m *MockAzureComputeClient) WaitForVMDeletionFuture(future compute.VirtualMachinesDeleteFuture) error {
	if m.MockWaitForVMDeletionFuture == nil {
		return nil
	}
	return m.MockWaitForVMDeletionFuture(future)
}

// WaitForDisksDeleteFuture waits for the DeleteManagedDisk operation to complete.
func (m *MockAzureComputeClient) WaitForDisksDeleteFuture(future compute.DisksDeleteFuture) error {
	if m.MockWaitForDisksDeleteFuture == nil {
		return nil
	}
	return m.MockWaitForDisksDeleteFuture(future)
}

// DeleteNetworkInterface deletes the NIC resource.
func (m *MockAzureNetworkClient) DeleteNetworkInterface(resourceGroup string, networkInterfaceName string) (network.InterfacesDeleteFuture, error) {
	if m.MockDeleteNetworkInterface == nil {
		return network.InterfacesDeleteFuture{}, nil
	}
	return m.MockDeleteNetworkInterface(resourceGroup, networkInterfaceName)
}

// WaitForNetworkInterfacesDeleteFuture returns when the DeleteNetworkInterface operation completes.
func (m *MockAzureNetworkClient) WaitForNetworkInterfacesDeleteFuture(future network.InterfacesDeleteFuture) error {
	if m.MockWaitForNetworkInterfacesDeleteFuture == nil {
		return nil
	}
	return m.MockWaitForNetworkInterfacesDeleteFuture(future)
}

// GetPublicIPAddress retrieves the reference of the PublicIPAddress resource.
func (m *MockAzureNetworkClient) GetPublicIPAddress(resourceGroup string, IPName string) (network.PublicIPAddress, error) {
	if m.MockGetPublicIPAddress == nil {
		return network.PublicIPAddress{}, nil
	}
	return m.MockGetPublicIPAddress(resourceGroup, IPName)
}

// DeletePublicIPAddress deletes the PublicIPAddress resource.
func (m *MockAzureNetworkClient) DeletePublicIPAddress(resourceGroup string, IPName string) (network.PublicIPAddressesDeleteFuture, error) {
	if m.MockDeletePublicIPAddress == nil {
		return network.PublicIPAddressesDeleteFuture{}, nil
	}
	return m.MockDeletePublicIPAddress(resourceGroup, IPName)
}

// WaitForPublicIPAddressDeleteFuture returns when the DeletePublicIPAddress completes.
func (m *MockAzureNetworkClient) WaitForPublicIPAddressDeleteFuture(future network.PublicIPAddressesDeleteFuture) error {
	if m.MockWaitForPublicIPAddressDeleteFuture == nil {
		return nil
	}
	return m.MockWaitForPublicIPAddressDeleteFuture(future)
}

// CreateOrUpdateNetworkSecurityGroup creates or updates the NSG resource.
func (m *MockAzureNetworkClient) CreateOrUpdateNetworkSecurityGroup(resourceGroupName string, networkSecurityGroupName string, location string) (*network.SecurityGroupsCreateOrUpdateFuture, error) {
	if m.MockCreateOrUpdateNetworkSecurityGroup == nil {
		return nil, nil
	}
	return m.MockCreateOrUpdateNetworkSecurityGroup(resourceGroupName, networkSecurityGroupName, location)
}

// NetworkSGIfExists returns the nsg resource reference if it exists.
func (m *MockAzureNetworkClient) NetworkSGIfExists(resourceGroupName string, networkSecurityGroupName string) (*network.SecurityGroup, error) {
	if m.MockNetworkSGIfExists == nil {
		return nil, nil
	}
	return m.MockNetworkSGIfExists(resourceGroupName, networkSecurityGroupName)
}

// WaitForNetworkSGsCreateOrUpdateFuture returns when the CreateOrUpdateNetworkSecurityGroup operation completes.
func (m *MockAzureNetworkClient) WaitForNetworkSGsCreateOrUpdateFuture(future network.SecurityGroupsCreateOrUpdateFuture) error {
	if m.MockWaitForNetworkSGsCreateOrUpdateFuture == nil {
		return nil
	}
	return m.MockWaitForNetworkSGsCreateOrUpdateFuture(future)
}

// CreateOrUpdateVnet creates or updates the vnet resource.
func (m *MockAzureNetworkClient) CreateOrUpdateVnet(resourceGroupName string, virtualNetworkName string, location string) (*network.VirtualNetworksCreateOrUpdateFuture, error) {
	if m.MockCreateOrUpdateVnet == nil {
		return nil, nil
	}
	return m.MockCreateOrUpdateVnet(resourceGroupName, virtualNetworkName, location)
}

// WaitForVnetCreateOrUpdateFuture returns when the CreateOrUpdateVnet operation completes.
func (m *MockAzureNetworkClient) WaitForVnetCreateOrUpdateFuture(future network.VirtualNetworksCreateOrUpdateFuture) error {
	if m.MockWaitForVnetCreateOrUpdateFuture == nil {
		return nil
	}
	return m.MockWaitForVnetCreateOrUpdateFuture(future)
}

// CreateOrUpdateGroup creates or updates an azure resource group.
func (m *MockAzureResourceManagementClient) CreateOrUpdateGroup(resourceGroupName string, location string) (resources.Group, error) {
	if m.MockCreateOrUpdateGroup == nil {
		return resources.Group{}, nil
	}
	return m.MockCreateOrUpdateGroup(resourceGroupName, location)
}

// DeleteGroup deletes an azure resource group.
func (m *MockAzureResourceManagementClient) DeleteGroup(resourceGroupName string) (resources.GroupsDeleteFuture, error) {
	if m.MockDeleteGroup == nil {
		return resources.GroupsDeleteFuture{}, nil
	}
	return m.MockDeleteGroup(resourceGroupName)
}

// CheckGroupExistence checks if a resource group with name 'rgName' exists.
func (m *MockAzureResourceManagementClient) CheckGroupExistence(rgName string) (autorest.Response, error) {
	if m.MockCheckGroupExistence == nil {
		return autorest.Response{}, nil
	}
	return m.MockCheckGroupExistence(rgName)
}

// WaitForGroupsDeleteFuture returns when the DeleteGroup operation completes.
func (m *MockAzureResourceManagementClient) WaitForGroupsDeleteFuture(future resources.GroupsDeleteFuture) error {
	if m.MockWaitForGroupsDeleteFuture == nil {
		return nil
	}
	return m.MockWaitForGroupsDeleteFuture(future)
}

func (m *MockAzureResourceManagementClient) CreateOrUpdateDeployment(machine *clusterv1.Machine, clusterConfig *providerv1.AzureClusterProviderSpec, machineConfig *providerv1.AzureMachineProviderSpec) (*resources.DeploymentsCreateOrUpdateFuture, error) {
	if m.MockCreateOrUpdateDeployment == nil {
		return nil, nil
	}
	return m.MockCreateOrUpdateDeployment(machine, clusterConfig, machineConfig)
}

func (m *MockAzureResourceManagementClient) ValidateDeployment(machine *clusterv1.Machine, clusterConfig *providerv1.AzureClusterProviderSpec, machineConfig *providerv1.AzureMachineProviderSpec) error {
	if m.MockValidateDeployment == nil {
		return nil
	}
	return m.MockValidateDeployment(machine, clusterConfig, machineConfig)
}

// GetDeploymentResult retrives an existing ARM deployment reference.
func (m *MockAzureResourceManagementClient) GetDeploymentResult(future resources.DeploymentsCreateOrUpdateFuture) (de resources.DeploymentExtended, err error) {
	if m.MockGetDeploymentResult == nil {
		return resources.DeploymentExtended{}, nil
	}
	return m.MockGetDeploymentResult(future)
}

// WaitForDeploymentsCreateOrUpdateFuture returns when the CreateOrUpdateDeployment operation completes.
func (m *MockAzureResourceManagementClient) WaitForDeploymentsCreateOrUpdateFuture(future resources.DeploymentsCreateOrUpdateFuture) error {
	if m.MockWaitForDeploymentsCreateOrUpdateFuture == nil {
		return nil
	}
	return m.MockWaitForDeploymentsCreateOrUpdateFuture(future)
}
*/

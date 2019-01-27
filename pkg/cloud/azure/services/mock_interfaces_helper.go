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
	"errors"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2018-10-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	azureconfigv1 "sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

// MockVMExists mocks the VMIfExists success response.
func MockVMExists() MockAzureComputeClient {
	return MockAzureComputeClient{
		MockVMIfExists: func(resourceGroup string, name string) (*compute.VirtualMachine, error) {
			networkProfile := compute.NetworkProfile{NetworkInterfaces: &[]compute.NetworkInterfaceReference{{ID: to.StringPtr("001")}}}
			OsDiskName := fmt.Sprintf("OS_Disk_%v", name)
			storageProfile := compute.StorageProfile{OsDisk: &compute.OSDisk{Name: &OsDiskName}}
			vmProperties := compute.VirtualMachineProperties{StorageProfile: &storageProfile, NetworkProfile: &networkProfile}
			return &compute.VirtualMachine{Name: &name, VirtualMachineProperties: &vmProperties}, nil
		},
	}
}

// MockVMExistsNICInvalid mocks the VMIfExists Invalid NIC response.
func MockVMExistsNICInvalid() MockAzureComputeClient {
	return MockAzureComputeClient{
		MockVMIfExists: func(resourceGroup string, name string) (*compute.VirtualMachine, error) {
			networkProfile := compute.NetworkProfile{NetworkInterfaces: &[]compute.NetworkInterfaceReference{{ID: to.StringPtr("")}}}
			OsDiskName := fmt.Sprintf("OS_Disk_%v", name)
			storageProfile := compute.StorageProfile{OsDisk: &compute.OSDisk{Name: &OsDiskName}}
			vmProperties := compute.VirtualMachineProperties{StorageProfile: &storageProfile, NetworkProfile: &networkProfile}
			return &compute.VirtualMachine{Name: &name, VirtualMachineProperties: &vmProperties}, nil
		},
	}
}

// MockVMNotExists mocks the VMExists not found response.
func MockVMNotExists() MockAzureComputeClient {
	return MockAzureComputeClient{
		MockVMIfExists: func(resourceGroup string, name string) (*compute.VirtualMachine, error) {
			return nil, nil
		},
	}
}

// MockVMCheckFailure mocks the VMIfExists failure response
func MockVMCheckFailure() MockAzureComputeClient {
	return MockAzureComputeClient{
		MockVMIfExists: func(resourceGroup string, name string) (*compute.VirtualMachine, error) {
			return &compute.VirtualMachine{}, errors.New("error while checking if vm exists")
		},
	}
}

// MockVMDeleteFailure mocks the VMDelete failure response.
func MockVMDeleteFailure() MockAzureComputeClient {
	return MockAzureComputeClient{
		MockDeleteVM: func(resourceGroup string, name string) (compute.VirtualMachinesDeleteFuture, error) {
			return compute.VirtualMachinesDeleteFuture{}, errors.New("error while deleting vm")
		},
	}
}

// MockVMDeleteFutureFailure mocks the VMDeleteFutureFailure response.
func MockVMDeleteFutureFailure() MockAzureComputeClient {
	return MockAzureComputeClient{
		MockWaitForVMDeletionFuture: func(future compute.VirtualMachinesDeleteFuture) error {
			return errors.New("failed on waiting for VirtualMachinesDeleteFuture")
		},
	}
}

// MockDisksDeleteFailure mocks the Disks Delete failure response.
func MockDisksDeleteFailure() MockAzureComputeClient {
	return MockAzureComputeClient{
		MockDeleteManagedDisk: func(resourceGroup string, name string) (compute.DisksDeleteFuture, error) {
			return compute.DisksDeleteFuture{}, errors.New("error while deleting managed disk")
		},
	}
}

// MockDisksDeleteFutureFailure mocks the Disks Delete Future failure response.
func MockDisksDeleteFutureFailure() MockAzureComputeClient {
	return MockAzureComputeClient{
		MockWaitForDisksDeleteFuture: func(future compute.DisksDeleteFuture) error {
			return errors.New("failed on waiting for VirtualMachinesDeleteFuture")
		},
	}
}

// MockRunCommandFailure mocks the RunCommand failure response.
func MockRunCommandFailure() MockAzureComputeClient {
	return MockAzureComputeClient{
		MockRunCommand: func(resourceGroup string, name string, cmd string) (compute.VirtualMachinesRunCommandFuture, error) {
			return compute.VirtualMachinesRunCommandFuture{}, errors.New("error while running command on vm")
		},
	}
}

// MockRunCommandFutureFailure mocks the RunCommand's future failure response.
func MockRunCommandFutureFailure() MockAzureComputeClient {
	return MockAzureComputeClient{
		MockWaitForVMRunCommandFuture: func(future compute.VirtualMachinesRunCommandFuture) error {
			return errors.New("failed on waiting for VirtualMachinesRunCommandFuture")
		},
	}
}

// MockNsgCreateOrUpdateSuccess mocks the SecurityGroupsCreateOrUpdateFuture response.
func MockNsgCreateOrUpdateSuccess() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockCreateOrUpdateNetworkSecurityGroup: func(resourceGroupName string, networkSecurityGroupName string, location string) (*network.SecurityGroupsCreateOrUpdateFuture, error) {
			return &network.SecurityGroupsCreateOrUpdateFuture{}, nil
		},
	}
}

// MockNsgCreateOrUpdateFailure SecurityGroupsCreateOrUpdateFuture failure response.
func MockNsgCreateOrUpdateFailure() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockCreateOrUpdateNetworkSecurityGroup: func(resourceGroupName string, networkSecurityGroupName string, location string) (*network.SecurityGroupsCreateOrUpdateFuture, error) {
			return nil, errors.New("failed to create or update network security group")
		},
	}
}

// MockVnetCreateOrUpdateSuccess mocks the VnetCreateOrUpdateSuccess response.
func MockVnetCreateOrUpdateSuccess() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockCreateOrUpdateVnet: func(resourceGroupName string, virtualNetworkName string, location string) (*network.VirtualNetworksCreateOrUpdateFuture, error) {
			return &network.VirtualNetworksCreateOrUpdateFuture{}, nil
		},
	}
}

// MockVnetCreateOrUpdateFailure mocks the VnetCreateOrUpdateSuccess failure response.
func MockVnetCreateOrUpdateFailure() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockCreateOrUpdateVnet: func(resourceGroupName string, virtualNetworkName string, location string) (*network.VirtualNetworksCreateOrUpdateFuture, error) {
			return nil, errors.New("failed to create or update vnet")
		},
	}
}

// MockNsgCreateOrUpdateFutureFailure mocks the SecurityGroupsCreateOrUpdateSuccess future failure response.
func MockNsgCreateOrUpdateFutureFailure() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockWaitForNetworkSGsCreateOrUpdateFuture: func(future network.SecurityGroupsCreateOrUpdateFuture) error {
			return errors.New("failed on waiting for SecurityGroupsCreateOrUpdateFuture")
		},
	}
}

// MockVnetCreateOrUpdateFutureFailure mocks the VnetCreateOrUpdate future failure response.
func MockVnetCreateOrUpdateFutureFailure() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockWaitForVnetCreateOrUpdateFuture: func(future network.VirtualNetworksCreateOrUpdateFuture) error {
			return errors.New("failed on waiting for VirtualNetworksCreateOrUpdateFuture")
		},
	}
}

// MockNicDeleteFailure mocks the InterfacesDelete failure response.
func MockNicDeleteFailure() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockDeleteNetworkInterface: func(resourceGroup string, networkInterfaceName string) (network.InterfacesDeleteFuture, error) {
			return network.InterfacesDeleteFuture{}, errors.New("failed to delete network interface")
		},
	}
}

// MockNicDeleteFutureFailure mocks the InterfacesDelete future failure response.
func MockNicDeleteFutureFailure() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockWaitForNetworkInterfacesDeleteFuture: func(future network.InterfacesDeleteFuture) error {
			return errors.New("failed on waiting for InterfacesDeleteFuture")
		},
	}
}

// MockPublicIPDeleteFailure mocks the PublicIPDeleteFailure response.
func MockPublicIPDeleteFailure() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockDeletePublicIPAddress: func(resourceGroup string, IPName string) (network.PublicIPAddressesDeleteFuture, error) {
			return network.PublicIPAddressesDeleteFuture{}, errors.New("failed to delete public ip address")
		},
	}
}

// MockPublicIPDeleteFutureFailure mocks the PublicIPDeleteFailure future response.
func MockPublicIPDeleteFutureFailure() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockWaitForPublicIPAddressDeleteFuture: func(future network.PublicIPAddressesDeleteFuture) error {
			return errors.New("failed on waiting for PublicIPAddressesDeleteFuture")
		},
	}
}

// MockGetPublicIPAddress mocks the GetPublicIPAddress success response.
func MockGetPublicIPAddress(ip string) MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockGetPublicIPAddress: func(resourceGroup string, IPName string) (network.PublicIPAddress, error) {
			publicIPAddress := network.PublicIPAddress{PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{}}
			publicIPAddress.IPAddress = to.StringPtr(ip)
			return publicIPAddress, nil
		},
	}
}

// MockGetPublicIPAddressFailure mocks the GetPublicIPAddress failure response.
func MockGetPublicIPAddressFailure() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockGetPublicIPAddress: func(resourceGroup string, IPName string) (network.PublicIPAddress, error) {
			return network.PublicIPAddress{}, errors.New("failed to get public ip address")
		},
	}
}

// ResourceManagement Mocks

// MockRgExists mocks the CheckGroupExistence response.
func MockRgExists() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockCheckGroupExistence: func(rgName string) (autorest.Response, error) {
			return autorest.Response{Response: &http.Response{StatusCode: 200}}, nil
		},
	}
}

// MockRgNotExists mocks the CheckGroupExistence not found response.
func MockRgNotExists() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockCheckGroupExistence: func(rgName string) (autorest.Response, error) {
			return autorest.Response{Response: &http.Response{StatusCode: 404}}, nil
		},
	}
}

// MockRgCheckFailure mocks the CheckGroupExistence failure response.
func MockRgCheckFailure() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockCheckGroupExistence: func(rgName string) (autorest.Response, error) {
			return autorest.Response{Response: &http.Response{StatusCode: 200}}, errors.New("failed to check resource group existence")
		},
	}
}

// MockRgCreateOrUpdateFailure mocks the CheckGroupExistence future failure response.
func MockRgCreateOrUpdateFailure() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockCreateOrUpdateGroup: func(resourceGroupName string, location string) (resources.Group, error) {
			return resources.Group{}, errors.New("failed to create resource group")
		},
	}
}

// MockRgDeleteSuccess mocks the WaitForGroupsDeleteFuture response
func MockRgDeleteSuccess() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockDeleteGroup: func(resourceGroupName string) (resources.GroupsDeleteFuture, error) {
			return resources.GroupsDeleteFuture{}, nil
		},
	}
}

// MockRgDeleteFailure mocks the groups delete response.
func MockRgDeleteFailure() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockDeleteGroup: func(resourceGroupName string) (resources.GroupsDeleteFuture, error) {
			return resources.GroupsDeleteFuture{}, errors.New("failed to delete resource group")
		},
	}
}

// MockRgDeleteFutureFailure mocks the WaitForGroupsDeleteFuture failure response.
func MockRgDeleteFutureFailure() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockWaitForGroupsDeleteFuture: func(future resources.GroupsDeleteFuture) error {
			return errors.New("error waiting for GroupsDeleteFuture")
		},
	}
}

// MockDeploymentCreateOrUpdateSuccess mocks the DeploymentCreateOrUpdate success response.
func MockDeploymentCreateOrUpdateSuccess() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockCreateOrUpdateDeployment: func(machine *clusterv1.Machine, clusterConfig *azureconfigv1.AzureClusterProviderSpec, machineConfig *azureconfigv1.AzureMachineProviderSpec) (*resources.DeploymentsCreateOrUpdateFuture, error) {
			return &resources.DeploymentsCreateOrUpdateFuture{}, nil
		},
	}
}

// MockDeploymentCreateOrUpdateFailure mocks the DeploymentCreateOrUpdate failure response.
func MockDeploymentCreateOrUpdateFailure() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockCreateOrUpdateDeployment: func(machine *clusterv1.Machine, clusterConfig *azureconfigv1.AzureClusterProviderSpec, machineConfig *azureconfigv1.AzureMachineProviderSpec) (*resources.DeploymentsCreateOrUpdateFuture, error) {
			return nil, errors.New("failed to create resource")
		},
	}
}

// MockDeploymentCreateOrUpdateFutureFailure mocks the DeploymentCreateOrUpdate future failure response.
func MockDeploymentCreateOrUpdateFutureFailure() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockWaitForDeploymentsCreateOrUpdateFuture: func(future resources.DeploymentsCreateOrUpdateFuture) error {
			return errors.New("failed on waiting for DeploymentsCreateOrUpdateFuture")
		},
	}
}

// MockDeloymentGetResultSuccess mocks the DeploymentGetResult success response.
func MockDeloymentGetResultSuccess() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockGetDeploymentResult: func(future resources.DeploymentsCreateOrUpdateFuture) (resources.DeploymentExtended, error) {
			return resources.DeploymentExtended{Name: to.StringPtr("deployment-test")}, nil
		},
	}
}

// MockDeloymentGetResultFailure mocks the DeploymentGetResult failure response.
func MockDeloymentGetResultFailure() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockGetDeploymentResult: func(future resources.DeploymentsCreateOrUpdateFuture) (resources.DeploymentExtended, error) {
			return resources.DeploymentExtended{}, errors.New("error getting deployment result")
		},
	}
}

// MockDeploymentValidate mocks the DeploymentValidate error response.
func MockDeploymentValidate() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockValidateDeployment: func(machine *clusterv1.Machine, clusterConfig *azureconfigv1.AzureClusterProviderSpec, machineConfig *azureconfigv1.AzureMachineProviderSpec) error {
			return errors.New("error validating deployment")
		},
	}
}

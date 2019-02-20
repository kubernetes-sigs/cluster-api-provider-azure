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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2018-04-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-01-01/network"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	azureconfigv1 "sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

// Compute Mocks
func MockVmExists() MockAzureComputeClient {
	return MockAzureComputeClient{
		MockVmIfExists: func(resourceGroup string, name string) (*compute.VirtualMachine, error) {
			networkProfile := compute.NetworkProfile{NetworkInterfaces: &[]compute.NetworkInterfaceReference{compute.NetworkInterfaceReference{ID: to.StringPtr("001")}}}
			OsDiskName := fmt.Sprintf("OS_Disk_%v", name)
			storageProfile := compute.StorageProfile{OsDisk: &compute.OSDisk{Name: &OsDiskName}}
			vmProperties := compute.VirtualMachineProperties{StorageProfile: &storageProfile, NetworkProfile: &networkProfile}
			return &compute.VirtualMachine{Name: &name, VirtualMachineProperties: &vmProperties}, nil
		},
	}
}

func MockVmExistsNICInvalid() MockAzureComputeClient {
	return MockAzureComputeClient{
		MockVmIfExists: func(resourceGroup string, name string) (*compute.VirtualMachine, error) {
			networkProfile := compute.NetworkProfile{NetworkInterfaces: &[]compute.NetworkInterfaceReference{compute.NetworkInterfaceReference{ID: to.StringPtr("")}}}
			OsDiskName := fmt.Sprintf("OS_Disk_%v", name)
			storageProfile := compute.StorageProfile{OsDisk: &compute.OSDisk{Name: &OsDiskName}}
			vmProperties := compute.VirtualMachineProperties{StorageProfile: &storageProfile, NetworkProfile: &networkProfile}
			return &compute.VirtualMachine{Name: &name, VirtualMachineProperties: &vmProperties}, nil
		},
	}
}
func MockVmNotExists() MockAzureComputeClient {
	return MockAzureComputeClient{
		MockVmIfExists: func(resourceGroup string, name string) (*compute.VirtualMachine, error) {
			return nil, nil
		},
	}
}

func MockVmCheckFailure() MockAzureComputeClient {
	return MockAzureComputeClient{
		MockVmIfExists: func(resourceGroup string, name string) (*compute.VirtualMachine, error) {
			return &compute.VirtualMachine{}, errors.New("error while checking if vm exists")
		},
	}
}

func MockVmDeleteFailure() MockAzureComputeClient {
	return MockAzureComputeClient{
		MockDeleteVM: func(resourceGroup string, name string) (compute.VirtualMachinesDeleteFuture, error) {
			return compute.VirtualMachinesDeleteFuture{}, errors.New("error while deleting vm")
		},
	}
}

func MockVmDeleteFutureFailure() MockAzureComputeClient {
	return MockAzureComputeClient{
		MockWaitForVMDeletionFuture: func(future compute.VirtualMachinesDeleteFuture) error {
			return errors.New("failed on waiting for VirtualMachinesDeleteFuture")
		},
	}
}

func MockDisksDeleteFailure() MockAzureComputeClient {
	return MockAzureComputeClient{
		MockDeleteManagedDisk: func(resourceGroup string, name string) (compute.DisksDeleteFuture, error) {
			return compute.DisksDeleteFuture{}, errors.New("error while deleting managed disk")
		},
	}
}

func MockDisksDeleteFutureFailure() MockAzureComputeClient {
	return MockAzureComputeClient{
		MockWaitForDisksDeleteFuture: func(future compute.DisksDeleteFuture) error {
			return errors.New("failed on waiting for VirtualMachinesDeleteFuture")
		},
	}
}

func MockRunCommandFailure() MockAzureComputeClient {
	return MockAzureComputeClient{
		MockRunCommand: func(resourceGroup string, name string, cmd string) (compute.VirtualMachinesRunCommandFuture, error) {
			return compute.VirtualMachinesRunCommandFuture{}, errors.New("error while running command on vm")
		},
	}
}

func MockRunCommandFutureFailure() MockAzureComputeClient {
	return MockAzureComputeClient{
		MockWaitForVMRunCommandFuture: func(future compute.VirtualMachinesRunCommandFuture) error {
			return errors.New("failed on waiting for VirtualMachinesRunCommandFuture")
		},
	}
}

// Network Mocks
func MockNsgCreateOrUpdateSuccess() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockCreateOrUpdateNetworkSecurityGroup: func(resourceGroupName string, networkSecurityGroupName string, location string) (*network.SecurityGroupsCreateOrUpdateFuture, error) {
			return &network.SecurityGroupsCreateOrUpdateFuture{}, nil
		},
	}
}

func MockNsgCreateOrUpdateFailure() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockCreateOrUpdateNetworkSecurityGroup: func(resourceGroupName string, networkSecurityGroupName string, location string) (*network.SecurityGroupsCreateOrUpdateFuture, error) {
			return nil, errors.New("failed to create or update network security group")
		},
	}
}

func MockVnetCreateOrUpdateSuccess() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockCreateOrUpdateVnet: func(resourceGroupName string, virtualNetworkName string, location string) (*network.VirtualNetworksCreateOrUpdateFuture, error) {
			return &network.VirtualNetworksCreateOrUpdateFuture{}, nil
		},
	}
}

func MockVnetCreateOrUpdateFailure() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockCreateOrUpdateVnet: func(resourceGroupName string, virtualNetworkName string, location string) (*network.VirtualNetworksCreateOrUpdateFuture, error) {
			return nil, errors.New("failed to create or update vnet")
		},
	}
}
func MockNsgCreateOrUpdateFutureFailure() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockWaitForNetworkSGsCreateOrUpdateFuture: func(future network.SecurityGroupsCreateOrUpdateFuture) error {
			return errors.New("failed on waiting for SecurityGroupsCreateOrUpdateFuture")
		},
	}
}

func MockVnetCreateOrUpdateFutureFailure() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockWaitForVnetCreateOrUpdateFuture: func(future network.VirtualNetworksCreateOrUpdateFuture) error {
			return errors.New("failed on waiting for VirtualNetworksCreateOrUpdateFuture")
		},
	}
}

func MockNicDeleteFailure() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockDeleteNetworkInterface: func(resourceGroup string, networkInterfaceName string) (network.InterfacesDeleteFuture, error) {
			return network.InterfacesDeleteFuture{}, errors.New("failed to delete network interface")
		},
	}
}

func MockNicDeleteFutureFailure() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockWaitForNetworkInterfacesDeleteFuture: func(future network.InterfacesDeleteFuture) error {
			return errors.New("failed on waiting for InterfacesDeleteFuture")
		},
	}
}

func MockPublicIpDeleteFailure() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockDeletePublicIpAddress: func(resourceGroup string, IPName string) (network.PublicIPAddressesDeleteFuture, error) {
			return network.PublicIPAddressesDeleteFuture{}, errors.New("failed to delete public ip address")
		},
	}
}

func MockPublicIpDeleteFutureFailure() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockWaitForPublicIpAddressDeleteFuture: func(future network.PublicIPAddressesDeleteFuture) error {
			return errors.New("failed on waiting for PublicIPAddressesDeleteFuture")
		},
	}
}

func MockGetPublicIPAddress(ip string) MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockGetPublicIpAddress: func(resourceGroup string, IPName string) (network.PublicIPAddress, error) {
			publicIPAddress := network.PublicIPAddress{PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{}}
			publicIPAddress.IPAddress = to.StringPtr(ip)
			return publicIPAddress, nil
		},
	}
}

func MockGetPublicIPAddressFailure() MockAzureNetworkClient {
	return MockAzureNetworkClient{
		MockGetPublicIpAddress: func(resourceGroup string, IPName string) (network.PublicIPAddress, error) {
			return network.PublicIPAddress{}, errors.New("failed to get public ip address")
		},
	}
}

// ResourceManagement Mocks

func MockRgExists() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockCheckGroupExistence: func(rgName string) (autorest.Response, error) {
			return autorest.Response{Response: &http.Response{StatusCode: 200}}, nil
		},
	}
}

func MockRgNotExists() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockCheckGroupExistence: func(rgName string) (autorest.Response, error) {
			return autorest.Response{Response: &http.Response{StatusCode: 404}}, nil
		},
	}
}

func MockRgCheckFailure() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockCheckGroupExistence: func(rgName string) (autorest.Response, error) {
			return autorest.Response{Response: &http.Response{StatusCode: 200}}, errors.New("failed to check resource group existence")
		},
	}
}

func MockRgCreateOrUpdateFailure() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockCreateOrUpdateGroup: func(resourceGroupName string, location string) (resources.Group, error) {
			return resources.Group{}, errors.New("failed to create resource group")
		},
	}
}

func MockRgDeleteSuccess() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockDeleteGroup: func(resourceGroupName string) (resources.GroupsDeleteFuture, error) {
			return resources.GroupsDeleteFuture{}, nil
		},
	}
}

func MockRgDeleteFailure() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockDeleteGroup: func(resourceGroupName string) (resources.GroupsDeleteFuture, error) {
			return resources.GroupsDeleteFuture{}, errors.New("failed to delete resource group")
		},
	}
}

func MockRgDeleteFutureFailure() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockWaitForGroupsDeleteFuture: func(future resources.GroupsDeleteFuture) error {
			return errors.New("error waiting for GroupsDeleteFuture")
		},
	}
}

func MockDeploymentCreateOrUpdateSuccess() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockCreateOrUpdateDeployment: func(machine *clusterv1.Machine, clusterConfig *azureconfigv1.AzureClusterProviderSpec, machineConfig *azureconfigv1.AzureMachineProviderSpec) (*resources.DeploymentsCreateOrUpdateFuture, error) {
			return &resources.DeploymentsCreateOrUpdateFuture{}, nil
		},
	}
}

func MockDeploymentCreateOrUpdateFailure() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockCreateOrUpdateDeployment: func(machine *clusterv1.Machine, clusterConfig *azureconfigv1.AzureClusterProviderSpec, machineConfig *azureconfigv1.AzureMachineProviderSpec) (*resources.DeploymentsCreateOrUpdateFuture, error) {
			return nil, errors.New("failed to create resource")
		},
	}
}

func MockDeploymentCreateOrUpdateFutureFailure() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockWaitForDeploymentsCreateOrUpdateFuture: func(future resources.DeploymentsCreateOrUpdateFuture) error {
			return errors.New("failed on waiting for DeploymentsCreateOrUpdateFuture")
		},
	}
}
func MockDeloymentGetResultSuccess() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockGetDeploymentResult: func(future resources.DeploymentsCreateOrUpdateFuture) (resources.DeploymentExtended, error) {
			return resources.DeploymentExtended{Name: to.StringPtr("deployment-test")}, nil
		},
	}
}

func MockDeloymentGetResultFailure() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockGetDeploymentResult: func(future resources.DeploymentsCreateOrUpdateFuture) (resources.DeploymentExtended, error) {
			return resources.DeploymentExtended{}, errors.New("error getting deployment result")
		},
	}
}

func MockDeploymentValidate() MockAzureResourceManagementClient {
	return MockAzureResourceManagementClient{
		MockValidateDeployment: func(machine *clusterv1.Machine, clusterConfig *azureconfigv1.AzureClusterProviderSpec, machineConfig *azureconfigv1.AzureMachineProviderSpec) error {
			return errors.New("error validating deployment")
		},
	}
}

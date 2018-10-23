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

type MockAzureComputeClient struct {
	MockVmIfExists               func(resourceGroup string, name string) (*compute.VirtualMachine, error)
	MockDeleteVM                 func(resourceGroup string, name string) (compute.VirtualMachinesDeleteFuture, error)
	MockWaitForVMDeletionFuture  func(future compute.VirtualMachinesDeleteFuture) error
	MockDeleteManagedDisk        func(resourceGroup string, name string) (compute.DisksDeleteFuture, error)
	MockWaitForDisksDeleteFuture func(future compute.DisksDeleteFuture) error
}

type MockAzureNetworkClient struct {
	MockCreateOrUpdateNetworkSecurityGroup    func(resourceGroupName string, networkSecurityGroupName string, location string) (*network.SecurityGroupsCreateOrUpdateFuture, error)
	MockNetworkSGIfExists                     func(resourceGroupName string, networkSecurityGroupName string) (*network.SecurityGroup, error)
	MockWaitForNetworkSGsCreateOrUpdateFuture func(future network.SecurityGroupsCreateOrUpdateFuture) error
}

type MockAzureResourceManagementClient struct {
	MockCreateOrUpdateGroup       func(resourceGroupName string, location string) (resources.Group, error)
	MockDeleteGroup               func(resourceGroupName string) (resources.GroupsDeleteFuture, error)
	MockCheckGroupExistence       func(rgName string) (autorest.Response, error)
	MockWaitForGroupsDeleteFuture func(future resources.GroupsDeleteFuture) error
}

func (m *MockAzureComputeClient) VmIfExists(resourceGroup string, name string) (*compute.VirtualMachine, error) {
	if m.MockVmIfExists == nil {
		return nil, nil
	}
	return m.MockVmIfExists(resourceGroup, name)
}

func (m *MockAzureComputeClient) DeleteVM(resourceGroup string, name string) (compute.VirtualMachinesDeleteFuture, error) {
	if m.MockDeleteVM == nil {
		return compute.VirtualMachinesDeleteFuture{}, nil
	}
	return m.MockDeleteVM(resourceGroup, name)
}

func (m *MockAzureComputeClient) DeleteManagedDisk(resourceGroup string, name string) (compute.DisksDeleteFuture, error) {
	if m.MockDeleteVM == nil {
		return compute.DisksDeleteFuture{}, nil
	}
	return m.MockDeleteManagedDisk(resourceGroup, name)
}

func (m *MockAzureComputeClient) WaitForVMDeletionFuture(future compute.VirtualMachinesDeleteFuture) error {
	if m.MockDeleteVM == nil {
		return nil
	}
	return m.MockWaitForVMDeletionFuture(future)
}

func (m *MockAzureComputeClient) WaitForDisksDeleteFuture(future compute.DisksDeleteFuture) error {
	if m.MockDeleteVM == nil {
		return nil
	}
	return m.MockWaitForDisksDeleteFuture(future)
}

func (m *MockAzureNetworkClient) CreateOrUpdateNetworkSecurityGroup(resourceGroupName string, networkSecurityGroupName string, location string) (*network.SecurityGroupsCreateOrUpdateFuture, error) {
	if m.MockCreateOrUpdateNetworkSecurityGroup == nil {
		return nil, nil
	}
	return m.MockCreateOrUpdateNetworkSecurityGroup(resourceGroupName, networkSecurityGroupName, location)
}

func (m *MockAzureNetworkClient) NetworkSGIfExists(resourceGroupName string, networkSecurityGroupName string) (*network.SecurityGroup, error) {
	if m.MockNetworkSGIfExists == nil {
		return nil, nil
	}
	return m.MockNetworkSGIfExists(resourceGroupName, networkSecurityGroupName)
}

func (m *MockAzureNetworkClient) WaitForNetworkSGsCreateOrUpdateFuture(future network.SecurityGroupsCreateOrUpdateFuture) error {
	if m.MockWaitForNetworkSGsCreateOrUpdateFuture == nil {
		return nil
	}
	return m.MockWaitForNetworkSGsCreateOrUpdateFuture(future)
}

func (m *MockAzureResourceManagementClient) CreateOrUpdateGroup(resourceGroupName string, location string) (resources.Group, error) {
	if m.MockCreateOrUpdateGroup == nil {
		return resources.Group{}, nil
	}
	return m.MockCreateOrUpdateGroup(resourceGroupName, location)
}

func (m *MockAzureResourceManagementClient) DeleteGroup(resourceGroupName string) (resources.GroupsDeleteFuture, error) {
	if m.MockDeleteGroup == nil {
		return resources.GroupsDeleteFuture{}, nil
	}
	return m.MockDeleteGroup(resourceGroupName)
}

func (m *MockAzureResourceManagementClient) CheckGroupExistence(rgName string) (autorest.Response, error) {
	if m.MockCheckGroupExistence == nil {
		return autorest.Response{}, nil
	}
	return m.MockCheckGroupExistence(rgName)
}

func (m *MockAzureResourceManagementClient) WaitForGroupsDeleteFuture(future resources.GroupsDeleteFuture) error {
	if m.MockWaitForGroupsDeleteFuture == nil {
		return nil
	}
	return m.MockWaitForGroupsDeleteFuture(future)
}

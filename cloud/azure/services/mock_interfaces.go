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

import "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-01-01/network"

type MockAzureNetworkClient struct {
	MockCreateOrUpdateNetworkSecurityGroup    func(resourceGroupName string, networkSecurityGroupName string, location string) (*network.SecurityGroupsCreateOrUpdateFuture, error)
	MockNetworkSGIfExists                     func(resourceGroupName string, networkSecurityGroupName string) (*network.SecurityGroup, error)
	MockWaitForNetworkSGsCreateOrUpdateFuture func(future network.SecurityGroupsCreateOrUpdateFuture) error
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

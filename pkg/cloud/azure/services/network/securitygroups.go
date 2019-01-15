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

package network

import (
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-10-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
)

const (
	// SecurityGroupDefaultName is the default name for the network security group of the cluster.
	SecurityGroupDefaultName = "ClusterAPINSG"
)

// NetworkSGIfExists returns the nsg reference if the nsg resource exists.
func (s *Service) NetworkSGIfExists(resourceGroupName string, networkSecurityGroupName string) (*network.SecurityGroup, error) {
	networkSG, err := s.scope.AzureClients.SecurityGroups.Get(s.scope.Context, resourceGroupName, networkSecurityGroupName, "")
	if err != nil {
		if aerr, ok := err.(autorest.DetailedError); ok {
			if aerr.StatusCode.(int) == 404 {
				return nil, nil
			}
		}
		return nil, err
	}
	return &networkSG, nil
}

// CreateOrUpdateNetworkSecurityGroup creates or updates the nsg resource.
func (s *Service) CreateOrUpdateNetworkSecurityGroup(resourceGroupName string, networkSecurityGroupName string, location string) (*network.SecurityGroupsCreateOrUpdateFuture, error) {
	if networkSecurityGroupName == "" {
		networkSecurityGroupName = SecurityGroupDefaultName
	}
	sshInbound := network.SecurityRule{
		Name: to.StringPtr("ClusterAPISSH"),
		SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
			Protocol:                 network.SecurityRuleProtocolTCP,
			SourcePortRange:          to.StringPtr("*"),
			DestinationPortRange:     to.StringPtr("22"),
			SourceAddressPrefix:      to.StringPtr("*"),
			DestinationAddressPrefix: to.StringPtr("*"),
			Priority:                 to.Int32Ptr(1000),
			Direction:                network.SecurityRuleDirectionInbound,
			Access:                   network.SecurityRuleAccessAllow,
		},
	}

	kubernetesInbound := network.SecurityRule{
		Name: to.StringPtr("KubernetesAPI"),
		SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
			Protocol:                 network.SecurityRuleProtocolTCP,
			SourcePortRange:          to.StringPtr("*"),
			DestinationPortRange:     to.StringPtr("6443"),
			SourceAddressPrefix:      to.StringPtr("*"),
			DestinationAddressPrefix: to.StringPtr("*"),
			Priority:                 to.Int32Ptr(1001),
			Direction:                network.SecurityRuleDirectionInbound,
			Access:                   network.SecurityRuleAccessAllow,
		},
	}

	securityGroupProperties := network.SecurityGroupPropertiesFormat{
		SecurityRules: &[]network.SecurityRule{sshInbound, kubernetesInbound},
	}
	securityGroup := network.SecurityGroup{
		Location:                      to.StringPtr(location),
		SecurityGroupPropertiesFormat: &securityGroupProperties,
	}
	sgFuture, err := s.scope.AzureClients.SecurityGroups.CreateOrUpdate(s.scope.Context, resourceGroupName, networkSecurityGroupName, securityGroup)
	if err != nil {
		return nil, err
	}
	return &sgFuture, nil
}

// DeleteNetworkSecurityGroup deletes the nsg resource.
func (s *Service) DeleteNetworkSecurityGroup(resourceGroupName string, networkSecurityGroupName string) (network.SecurityGroupsDeleteFuture, error) {
	return s.scope.AzureClients.SecurityGroups.Delete(s.scope.Context, resourceGroupName, networkSecurityGroupName)
}

// TODO: Dead code
/*
func (s *Service) WaitForNetworkSGsCreateOrUpdateFuture(future network.SecurityGroupsCreateOrUpdateFuture) error {
	return future.Future.WaitForCompletionRef(s.scope.Context, s.scope.AzureClients.SecurityGroups.Client)
}
*/

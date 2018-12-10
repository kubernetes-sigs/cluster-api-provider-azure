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
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-01-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
)

const (
	SecurityGroupDefaultName = "ClusterAPINSG"
)

func (s *Service) NetworkSGIfExists(resourceGroupName string, networkSecurityGroupName string) (*network.SecurityGroup, error) {
	networkSG, err := s.SecurityGroupsClient.Get(s.ctx, resourceGroupName, networkSecurityGroupName, "")
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
	sgFuture, err := s.SecurityGroupsClient.CreateOrUpdate(s.ctx, resourceGroupName, networkSecurityGroupName, securityGroup)
	if err != nil {
		return nil, err
	}
	return &sgFuture, nil
}

func (s *Service) DeleteNetworkSecurityGroup(resourceGroupName string, networkSecurityGroupName string) (network.SecurityGroupsDeleteFuture, error) {
	return s.SecurityGroupsClient.Delete(s.ctx, resourceGroupName, networkSecurityGroupName)
}

func (s *Service) WaitForNetworkSGsCreateOrUpdateFuture(future network.SecurityGroupsCreateOrUpdateFuture) error {
	return future.Future.WaitForCompletionRef(s.ctx, s.SecurityGroupsClient.Client)
}

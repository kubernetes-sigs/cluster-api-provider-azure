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
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"k8s.io/klog"
)

const (
	// SecurityGroupDefaultName is the default name for the network security group of the cluster.
	SecurityGroupDefaultName = "ClusterAPINSG"
)

// NetworkSGIfExists returns the nsg reference if the nsg resource exists.
func (s *Service) NetworkSGIfExists(resourceGroupName string, networkSecurityGroupName string) (nsg network.SecurityGroup, err error) {
	networkSG, err := s.scope.AzureClients.SecurityGroups.Get(s.scope.Context, resourceGroupName, networkSecurityGroupName, "")
	if err != nil {
		if aerr, ok := err.(autorest.DetailedError); ok {
			if aerr.StatusCode.(int) == 404 {
				return nsg, nil
			}
		}
		return nsg, err
	}

	return networkSG, nil
}

// CreateOrUpdateNetworkSecurityGroup creates or updates the nsg resource.
func (s *Service) CreateOrUpdateNetworkSecurityGroup(resourceGroupName, networkSecurityGroupName string) (nsg network.SecurityGroup, err error) {
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
		Location:                      to.StringPtr(s.scope.Location()),
		SecurityGroupPropertiesFormat: &securityGroupProperties,
	}

	future, err := s.scope.AzureClients.SecurityGroups.CreateOrUpdate(s.scope.Context, resourceGroupName, networkSecurityGroupName, securityGroup)

	if err != nil {
		return nsg, err
	}

	err = future.WaitForCompletionRef(s.scope.Context, s.scope.AzureClients.SecurityGroups.Client)
	if err != nil {
		return nsg, fmt.Errorf("cannot get NSG create or update future response: %v", err)
	}

	klog.V(2).Info("Successfully updated NSG")
	return future.Result(s.scope.AzureClients.SecurityGroups)
}

// DeleteNetworkSecurityGroup deletes the nsg resource.
func (s *Service) DeleteNetworkSecurityGroup(resourceGroupName string, networkSecurityGroupName string) (network.SecurityGroupsDeleteFuture, error) {
	return s.scope.AzureClients.SecurityGroups.Delete(s.scope.Context, resourceGroupName, networkSecurityGroupName)
}

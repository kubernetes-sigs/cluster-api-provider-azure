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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-12-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"k8s.io/klog"
)

const (
	// VnetDefaultName is the default name for the cluster's virtual network.
	VnetDefaultName = "ClusterAPIVnet"
	// SubnetDefaultName is the default name for the cluster's subnet.
	SubnetDefaultName        = "ClusterAPISubnet"
	defaultPrivateSubnetCIDR = "10.0.0.0/24"
)

// CreateOrUpdateVnet creates or updates a virtual network resource.
func (s *Service) CreateOrUpdateVnet(resourceGroupName, virtualNetworkName string) (vnet network.VirtualNetwork, err error) {
	if virtualNetworkName == "" {
		virtualNetworkName = VnetDefaultName
	}

	// TODO: Rewrite to allow for non-default NSG name.
	nsg, err := s.CreateOrUpdateNetworkSecurityGroup(s.scope.ClusterConfig.ResourceGroup, SecurityGroupDefaultName)
	if err != nil {
		klog.V(2).Info("CreateOrUpdateVnet: could not get NSG")
		return vnet, err
	}

	subnets := []network.Subnet{
		{
			Name: to.StringPtr(SubnetDefaultName),
			SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
				AddressPrefix:        to.StringPtr(defaultPrivateSubnetCIDR),
				NetworkSecurityGroup: &nsg,
			},
		},
	}

	virtualNetworkProperties := network.VirtualNetworkPropertiesFormat{
		AddressSpace: &network.AddressSpace{
			AddressPrefixes: &[]string{defaultPrivateSubnetCIDR},
		},
		Subnets: &subnets,
	}

	virtualNetwork := network.VirtualNetwork{
		Location:                       to.StringPtr(s.scope.Location()),
		VirtualNetworkPropertiesFormat: &virtualNetworkProperties,
	}

	future, err := s.scope.VirtualNetworks.CreateOrUpdate(s.scope.Context, resourceGroupName, virtualNetworkName, virtualNetwork)

	if err != nil {
		return vnet, err
	}

	err = future.WaitForCompletionRef(s.scope.Context, s.scope.AzureClients.VirtualNetworks.Client)
	if err != nil {
		return vnet, fmt.Errorf("cannot get vnet create or update future response: %v", err)
	}

	klog.V(2).Info("Successfully updated virtual network")
	return future.Result(s.scope.AzureClients.VirtualNetworks)
}

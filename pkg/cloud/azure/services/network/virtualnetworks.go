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
	"github.com/Azure/go-autorest/autorest/to"
)

const (
	// VnetDefaultName is the default name for the cluster's virtual network.
	VnetDefaultName = "ClusterAPIVnet"
	// SubnetDefaultName is the default name for the cluster's subnet.
	SubnetDefaultName        = "ClusterAPISubnet"
	defaultPrivateSubnetCIDR = "10.0.0.0/24"
)

// CreateOrUpdateVnet creates or updates a virtual network resource.
func (s *Service) CreateOrUpdateVnet(resourceGroupName string, virtualNetworkName string, location string) (*network.VirtualNetworksCreateOrUpdateFuture, error) {
	if virtualNetworkName == "" {
		virtualNetworkName = VnetDefaultName
	}

	subnets := []network.Subnet{
		{
			Name: to.StringPtr(SubnetDefaultName),
			SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
				AddressPrefix: to.StringPtr(defaultPrivateSubnetCIDR),
			},
		},
	}
	virtualNetworkProperties := network.VirtualNetworkPropertiesFormat{
		AddressSpace: &network.AddressSpace{&[]string{defaultPrivateSubnetCIDR}},
		Subnets:      &subnets,
	}
	virtualNetwork := network.VirtualNetwork{
		Location:                       to.StringPtr(location),
		VirtualNetworkPropertiesFormat: &virtualNetworkProperties,
	}
	sgFuture, err := s.scope.AzureClients.VirtualNetworks.CreateOrUpdate(s.scope.Context, resourceGroupName, virtualNetworkName, virtualNetwork)
	if err != nil {
		return nil, err
	}
	return &sgFuture, nil
}

// TODO: Dead code
/*
func (s *Service) WaitForVnetCreateOrUpdateFuture(future network.VirtualNetworksCreateOrUpdateFuture) error {
	return future.Future.WaitForCompletionRef(s.scope.Context, s.scope.AzureClients.VirtualNetworks.Client)
}
*/

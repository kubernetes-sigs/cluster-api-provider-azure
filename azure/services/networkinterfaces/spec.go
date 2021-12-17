/*
Copyright 2021 The Kubernetes Authors.

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

package networkinterfaces

import (
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
)

// NICSpec defines the specification for a Network Interface.
type NICSpec struct {
	Name                      string
	ResourceGroup             string
	Location                  string
	SubscriptionID            string
	MachineName               string
	SubnetName                string
	VNetName                  string
	VNetResourceGroup         string
	StaticIPAddress           string
	PublicLBName              string
	PublicLBAddressPoolName   string
	PublicLBNATRuleName       string
	InternalLBName            string
	InternalLBAddressPoolName string
	PublicIPName              string
	AcceleratedNetworking     *bool
	IPv6Enabled               bool
	EnableIPForwarding        bool
	SKU                       *resourceskus.SKU
}

// ResourceName returns the name of the network interface.
func (s *NICSpec) ResourceName() string {
	return s.Name
}

// ResourceGroupName returns the name of the resource group.
func (s *NICSpec) ResourceGroupName() string {
	return s.ResourceGroup
}

// OwnerResourceName is a no-op for network interfaces.
func (s *NICSpec) OwnerResourceName() string {
	return ""
}

// Parameters returns the parameters for the network interface.
func (s *NICSpec) Parameters(existing interface{}) (parameters interface{}, err error) {
	if existing != nil {
		if _, ok := existing.(network.Interface); !ok {
			return nil, errors.Errorf("%T is not a network.Interface", existing)
		}
		// network interface already exists
		return nil, nil
	}

	nicConfig := &network.InterfaceIPConfigurationPropertiesFormat{}

	subnet := &network.Subnet{
		ID: to.StringPtr(azure.SubnetID(s.SubscriptionID, s.VNetResourceGroup, s.VNetName, s.SubnetName)),
	}
	nicConfig.Subnet = subnet

	nicConfig.PrivateIPAllocationMethod = network.IPAllocationMethodDynamic
	if s.StaticIPAddress != "" {
		nicConfig.PrivateIPAllocationMethod = network.IPAllocationMethodStatic
		nicConfig.PrivateIPAddress = to.StringPtr(s.StaticIPAddress)
	}

	backendAddressPools := []network.BackendAddressPool{}
	if s.PublicLBName != "" {
		if s.PublicLBAddressPoolName != "" {
			backendAddressPools = append(backendAddressPools,
				network.BackendAddressPool{
					ID: to.StringPtr(azure.AddressPoolID(s.SubscriptionID, s.ResourceGroup, s.PublicLBName, s.PublicLBAddressPoolName)),
				})
		}
		if s.PublicLBNATRuleName != "" {
			nicConfig.LoadBalancerInboundNatRules = &[]network.InboundNatRule{
				{
					ID: to.StringPtr(azure.NATRuleID(s.SubscriptionID, s.ResourceGroup, s.PublicLBName, s.PublicLBNATRuleName)),
				},
			}
		}
	}
	if s.InternalLBName != "" && s.InternalLBAddressPoolName != "" {
		backendAddressPools = append(backendAddressPools,
			network.BackendAddressPool{
				ID: to.StringPtr(azure.AddressPoolID(s.SubscriptionID, s.ResourceGroup, s.InternalLBName, s.InternalLBAddressPoolName)),
			})
	}
	nicConfig.LoadBalancerBackendAddressPools = &backendAddressPools

	if s.PublicIPName != "" {
		nicConfig.PublicIPAddress = &network.PublicIPAddress{
			ID: to.StringPtr(azure.PublicIPID(s.SubscriptionID, s.ResourceGroup, s.PublicIPName)),
		}
	}

	if s.AcceleratedNetworking == nil {
		// set accelerated networking to the capability of the VMSize
		if s.SKU == nil {
			return nil, errors.New("unable to get required network interface SKU from machine cache")
		}

		accelNet := s.SKU.HasCapability(resourceskus.AcceleratedNetworking)
		s.AcceleratedNetworking = &accelNet
	}

	ipConfigurations := []network.InterfaceIPConfiguration{
		{
			Name:                                     to.StringPtr("pipConfig"),
			InterfaceIPConfigurationPropertiesFormat: nicConfig,
		},
	}

	if s.IPv6Enabled {
		ipv6Config := network.InterfaceIPConfiguration{
			Name: to.StringPtr("ipConfigv6"),
			InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
				PrivateIPAddressVersion: "IPv6",
				Primary:                 to.BoolPtr(false),
				Subnet:                  &network.Subnet{ID: subnet.ID},
			},
		}

		ipConfigurations = append(ipConfigurations, ipv6Config)
	}

	return network.Interface{
		Location: to.StringPtr(s.Location),
		InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
			EnableAcceleratedNetworking: s.AcceleratedNetworking,
			IPConfigurations:            &ipConfigurations,
			EnableIPForwarding:          to.BoolPtr(s.EnableIPForwarding),
		},
	}, nil
}

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
	"context"
	"strconv"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"github.com/pkg/errors"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
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
	DNSServers                []string
	AdditionalTags            infrav1.Tags
	ClusterName               string
	IPConfigs                 []IPConfig
}

// IPConfig defines the specification for an IP address configuration.
type IPConfig struct {
	PrivateIP       *string
	PublicIPAddress *string
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
func (s *NICSpec) Parameters(ctx context.Context, existing interface{}) (parameters interface{}, err error) {
	if existing != nil {
		if _, ok := existing.(network.Interface); !ok {
			return nil, errors.Errorf("%T is not a network.Interface", existing)
		}
		// network interface already exists
		return nil, nil
	}

	primaryIPConfig := &network.InterfaceIPConfigurationPropertiesFormat{
		Primary: pointer.Bool(true),
	}

	subnet := &network.Subnet{
		ID: pointer.String(azure.SubnetID(s.SubscriptionID, s.VNetResourceGroup, s.VNetName, s.SubnetName)),
	}
	primaryIPConfig.Subnet = subnet

	primaryIPConfig.PrivateIPAllocationMethod = network.IPAllocationMethodDynamic
	if s.StaticIPAddress != "" {
		primaryIPConfig.PrivateIPAllocationMethod = network.IPAllocationMethodStatic
		primaryIPConfig.PrivateIPAddress = pointer.String(s.StaticIPAddress)
	}

	backendAddressPools := []network.BackendAddressPool{}
	if s.PublicLBName != "" {
		if s.PublicLBAddressPoolName != "" {
			backendAddressPools = append(backendAddressPools,
				network.BackendAddressPool{
					ID: pointer.String(azure.AddressPoolID(s.SubscriptionID, s.ResourceGroup, s.PublicLBName, s.PublicLBAddressPoolName)),
				})
		}
		if s.PublicLBNATRuleName != "" {
			primaryIPConfig.LoadBalancerInboundNatRules = &[]network.InboundNatRule{
				{
					ID: pointer.String(azure.NATRuleID(s.SubscriptionID, s.ResourceGroup, s.PublicLBName, s.PublicLBNATRuleName)),
				},
			}
		}
	}
	if s.InternalLBName != "" && s.InternalLBAddressPoolName != "" {
		backendAddressPools = append(backendAddressPools,
			network.BackendAddressPool{
				ID: pointer.String(azure.AddressPoolID(s.SubscriptionID, s.ResourceGroup, s.InternalLBName, s.InternalLBAddressPoolName)),
			})
	}
	primaryIPConfig.LoadBalancerBackendAddressPools = &backendAddressPools

	if s.PublicIPName != "" {
		primaryIPConfig.PublicIPAddress = &network.PublicIPAddress{
			ID: pointer.String(azure.PublicIPID(s.SubscriptionID, s.ResourceGroup, s.PublicIPName)),
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

	dnsSettings := network.InterfaceDNSSettings{}
	if len(s.DNSServers) > 0 {
		dnsSettings.DNSServers = &s.DNSServers
	}

	ipConfigurations := []network.InterfaceIPConfiguration{
		{
			Name:                                     pointer.String("pipConfig"),
			InterfaceIPConfigurationPropertiesFormat: primaryIPConfig,
		},
	}

	// Build additional IPConfigs if more than 1 is specified
	for i := 1; i < len(s.IPConfigs); i++ {
		c := s.IPConfigs[i]
		newIPConfigPropertiesFormat := &network.InterfaceIPConfigurationPropertiesFormat{}
		newIPConfigPropertiesFormat.Subnet = subnet
		config := network.InterfaceIPConfiguration{
			Name:                                     pointer.String(s.Name + "-" + strconv.Itoa(i)),
			InterfaceIPConfigurationPropertiesFormat: newIPConfigPropertiesFormat,
		}
		if c.PrivateIP != nil && *c.PrivateIP != "" {
			config.InterfaceIPConfigurationPropertiesFormat.PrivateIPAllocationMethod = network.IPAllocationMethodStatic
			config.InterfaceIPConfigurationPropertiesFormat.PrivateIPAddress = c.PrivateIP
		} else {
			config.InterfaceIPConfigurationPropertiesFormat.PrivateIPAllocationMethod = network.IPAllocationMethodDynamic
		}

		if c.PublicIPAddress != nil && *c.PublicIPAddress != "" {
			config.InterfaceIPConfigurationPropertiesFormat.PublicIPAddress = &network.PublicIPAddress{
				PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
					PublicIPAllocationMethod: network.IPAllocationMethodStatic,
					IPAddress:                c.PublicIPAddress,
				},
			}
		} else if c.PublicIPAddress != nil {
			config.InterfaceIPConfigurationPropertiesFormat.PublicIPAddress = &network.PublicIPAddress{
				PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
					PublicIPAllocationMethod: network.IPAllocationMethodDynamic,
				},
			}
		}
		config.InterfaceIPConfigurationPropertiesFormat.Primary = pointer.Bool(false)
		ipConfigurations = append(ipConfigurations, config)
	}
	if s.IPv6Enabled {
		ipv6Config := network.InterfaceIPConfiguration{
			Name: pointer.String("ipConfigv6"),
			InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
				PrivateIPAddressVersion: "IPv6",
				Primary:                 pointer.Bool(false),
				Subnet:                  &network.Subnet{ID: subnet.ID},
			},
		}

		ipConfigurations = append(ipConfigurations, ipv6Config)
	}

	return network.Interface{
		Location: pointer.String(s.Location),
		InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
			EnableAcceleratedNetworking: s.AcceleratedNetworking,
			IPConfigurations:            &ipConfigurations,
			DNSSettings:                 &dnsSettings,
			EnableIPForwarding:          pointer.Bool(s.EnableIPForwarding),
		},
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: s.ClusterName,
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Name:        pointer.String(s.Name),
			Additional:  s.AdditionalTags,
		})),
	}, nil
}

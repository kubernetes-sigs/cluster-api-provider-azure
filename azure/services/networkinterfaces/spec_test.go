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
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
)

var (
	fakeMissingSKUNICSpec = NICSpec{
		Name:                  "my-net-interface",
		ResourceGroup:         "my-rg",
		Location:              "fake-location",
		SubscriptionID:        "123",
		MachineName:           "azure-test1",
		SubnetName:            "my-subnet",
		VNetName:              "my-vnet",
		VNetResourceGroup:     "my-rg",
		PublicLBName:          "my-public-lb",
		AcceleratedNetworking: nil,
	}
	fakeSku = resourceskus.SKU{
		Name: to.StringPtr("Standard_D2v2"),
		Kind: to.StringPtr(string(resourceskus.VirtualMachines)),
		Locations: &[]string{
			"fake-location",
		},
		LocationInfo: &[]compute.ResourceSkuLocationInfo{
			{
				Location: to.StringPtr("fake-location"),
				Zones:    &[]string{"1"},
			},
		},
		Capabilities: &[]compute.ResourceSkuCapabilities{
			{
				Name:  to.StringPtr(resourceskus.AcceleratedNetworking),
				Value: to.StringPtr(string(resourceskus.CapabilitySupported)),
			},
		},
	}
	fakeStaticPrivateIPNICSpec = NICSpec{
		Name:                    "my-net-interface",
		ResourceGroup:           "my-rg",
		Location:                "fake-location",
		SubscriptionID:          "123",
		MachineName:             "azure-test1",
		SubnetName:              "my-subnet",
		VNetName:                "my-vnet",
		VNetResourceGroup:       "my-rg",
		PublicLBName:            "my-public-lb",
		PublicLBAddressPoolName: "cluster-name-outboundBackendPool",
		StaticIPAddress:         "fake.static.ip",
		AcceleratedNetworking:   nil,
		SKU:                     &fakeSku,
	}

	fakeDynamicPrivateIPNICSpec = NICSpec{
		Name:                    "my-net-interface",
		ResourceGroup:           "my-rg",
		Location:                "fake-location",
		SubscriptionID:          "123",
		MachineName:             "azure-test1",
		SubnetName:              "my-subnet",
		VNetName:                "my-vnet",
		VNetResourceGroup:       "my-rg",
		PublicLBName:            "my-public-lb",
		PublicLBAddressPoolName: "cluster-name-outboundBackendPool",
		AcceleratedNetworking:   nil,
		SKU:                     &fakeSku,
	}

	fakeControlPlaneNICSpec = NICSpec{
		Name:                      "my-net-interface",
		ResourceGroup:             "my-rg",
		Location:                  "fake-location",
		SubscriptionID:            "123",
		MachineName:               "azure-test1",
		SubnetName:                "my-subnet",
		VNetName:                  "my-vnet",
		VNetResourceGroup:         "my-rg",
		PublicLBName:              "my-public-lb",
		PublicLBAddressPoolName:   "my-public-lb-backendPool",
		PublicLBNATRuleName:       "azure-test1",
		InternalLBName:            "my-internal-lb",
		InternalLBAddressPoolName: "my-internal-lb-backendPool",
		AcceleratedNetworking:     nil,
		SKU:                       &fakeSku,
	}

	fakeAcceleratedNetworkingNICSpec = NICSpec{
		Name:                  "my-net-interface",
		ResourceGroup:         "my-rg",
		Location:              "fake-location",
		SubscriptionID:        "123",
		MachineName:           "azure-test1",
		SubnetName:            "my-subnet",
		VNetName:              "my-vnet",
		VNetResourceGroup:     "my-rg",
		PublicLBName:          "my-public-lb",
		AcceleratedNetworking: nil,
		SKU:                   &fakeSku,
	}

	fakeNonAcceleratedNetworkingNICSpec = NICSpec{
		Name:                  "my-net-interface",
		ResourceGroup:         "my-rg",
		Location:              "fake-location",
		SubscriptionID:        "123",
		MachineName:           "azure-test1",
		SubnetName:            "my-subnet",
		VNetName:              "my-vnet",
		VNetResourceGroup:     "my-rg",
		PublicLBName:          "my-public-lb",
		AcceleratedNetworking: to.BoolPtr(false),
	}

	fakeIpv6NICSpec = NICSpec{
		Name:                  "my-net-interface",
		ResourceGroup:         "my-rg",
		Location:              "fake-location",
		SubscriptionID:        "123",
		MachineName:           "azure-test1",
		SubnetName:            "my-subnet",
		VNetName:              "my-vnet",
		IPv6Enabled:           true,
		VNetResourceGroup:     "my-rg",
		PublicLBName:          "my-public-lb",
		AcceleratedNetworking: nil,
		SKU:                   &fakeSku,
		EnableIPForwarding:    true,
	}
)

func TestParameters(t *testing.T) {
	testcases := []struct {
		name          string
		spec          *NICSpec
		existing      interface{}
		expect        func(g *WithT, result interface{})
		expectedError string
	}{
		{
			name:     "error when accelerted networking is nil and no SKU is present",
			spec:     &fakeMissingSKUNICSpec,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "unable to get required network interface SKU from machine cache",
		},
		{
			name:     "get parameters for network interface with static private IP",
			spec:     &fakeStaticPrivateIPNICSpec,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(network.Interface{}))
				g.Expect(result.(network.Interface)).To(Equal(network.Interface{
					Location: to.StringPtr("fake-location"),
					InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
						EnableAcceleratedNetworking: to.BoolPtr(true),
						EnableIPForwarding:          to.BoolPtr(false),
						IPConfigurations: &[]network.InterfaceIPConfiguration{
							{
								Name: to.StringPtr("pipConfig"),
								InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
									LoadBalancerBackendAddressPools: &[]network.BackendAddressPool{{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-public-lb/backendAddressPools/cluster-name-outboundBackendPool")}},
									PrivateIPAllocationMethod:       network.IPAllocationMethodStatic,
									PrivateIPAddress:                to.StringPtr("fake.static.ip"),
									Subnet:                          &network.Subnet{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
								},
							},
						},
					},
				}))
			},
			expectedError: "",
		},
		{
			name:     "get parameters for network interface with dynamic private IP",
			spec:     &fakeDynamicPrivateIPNICSpec,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(network.Interface{}))
				g.Expect(result.(network.Interface)).To(Equal(network.Interface{
					Location: to.StringPtr("fake-location"),
					InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
						EnableAcceleratedNetworking: to.BoolPtr(true),
						EnableIPForwarding:          to.BoolPtr(false),
						IPConfigurations: &[]network.InterfaceIPConfiguration{
							{
								Name: to.StringPtr("pipConfig"),
								InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
									LoadBalancerBackendAddressPools: &[]network.BackendAddressPool{{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-public-lb/backendAddressPools/cluster-name-outboundBackendPool")}},
									PrivateIPAllocationMethod:       network.IPAllocationMethodDynamic,
									Subnet:                          &network.Subnet{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
								},
							},
						},
					},
				}))
			},
			expectedError: "",
		},
		{
			name:     "get parameters for control plane network interface",
			spec:     &fakeControlPlaneNICSpec,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(network.Interface{}))
				g.Expect(result.(network.Interface)).To(Equal(network.Interface{
					Location: to.StringPtr("fake-location"),
					InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
						EnableAcceleratedNetworking: to.BoolPtr(true),
						EnableIPForwarding:          to.BoolPtr(false),
						IPConfigurations: &[]network.InterfaceIPConfiguration{
							{
								Name: to.StringPtr("pipConfig"),
								InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
									Subnet:                      &network.Subnet{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									PrivateIPAllocationMethod:   network.IPAllocationMethodDynamic,
									LoadBalancerInboundNatRules: &[]network.InboundNatRule{{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-public-lb/inboundNatRules/azure-test1")}},
									LoadBalancerBackendAddressPools: &[]network.BackendAddressPool{
										{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-public-lb/backendAddressPools/my-public-lb-backendPool")},
										{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-internal-lb/backendAddressPools/my-internal-lb-backendPool")}},
								},
							},
						},
					},
				}))
			},
			expectedError: "",
		},
		{
			name:     "get parameters for network interface with accelerated networking",
			spec:     &fakeAcceleratedNetworkingNICSpec,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(network.Interface{}))
				g.Expect(result.(network.Interface)).To(Equal(network.Interface{
					Location: to.StringPtr("fake-location"),
					InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
						EnableAcceleratedNetworking: to.BoolPtr(true),
						EnableIPForwarding:          to.BoolPtr(false),
						IPConfigurations: &[]network.InterfaceIPConfiguration{
							{
								Name: to.StringPtr("pipConfig"),
								InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
									Subnet:                          &network.Subnet{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									PrivateIPAllocationMethod:       network.IPAllocationMethodDynamic,
									LoadBalancerBackendAddressPools: &[]network.BackendAddressPool{},
								},
							},
						},
					},
				}))
			},
			expectedError: "",
		},
		{
			name:     "get parameters for network interface without accelerated networking",
			spec:     &fakeNonAcceleratedNetworkingNICSpec,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(network.Interface{}))
				g.Expect(result.(network.Interface)).To(Equal(network.Interface{
					Location: to.StringPtr("fake-location"),
					InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
						EnableAcceleratedNetworking: to.BoolPtr(false),
						EnableIPForwarding:          to.BoolPtr(false),
						IPConfigurations: &[]network.InterfaceIPConfiguration{
							{
								Name: to.StringPtr("pipConfig"),
								InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
									Subnet:                          &network.Subnet{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									PrivateIPAllocationMethod:       network.IPAllocationMethodDynamic,
									LoadBalancerBackendAddressPools: &[]network.BackendAddressPool{},
								},
							},
						},
					},
				}))
			},
			expectedError: "",
		},
		{
			name:     "get parameters for network interface ipv6",
			spec:     &fakeIpv6NICSpec,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(network.Interface{}))
				g.Expect(result.(network.Interface)).To(Equal(network.Interface{
					Location: to.StringPtr("fake-location"),
					InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
						EnableAcceleratedNetworking: to.BoolPtr(true),
						EnableIPForwarding:          to.BoolPtr(true),
						IPConfigurations: &[]network.InterfaceIPConfiguration{
							{
								Name: to.StringPtr("pipConfig"),
								InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
									Subnet:                          &network.Subnet{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									PrivateIPAllocationMethod:       network.IPAllocationMethodDynamic,
									LoadBalancerBackendAddressPools: &[]network.BackendAddressPool{},
								},
							},
							{
								Name: to.StringPtr("ipConfigv6"),
								InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
									Subnet:                  &network.Subnet{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									Primary:                 to.BoolPtr(false),
									PrivateIPAddressVersion: "IPv6",
								},
							},
						},
					},
				}))
			},
			expectedError: "",
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(tc.existing)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			tc.expect(g, result)
		})
	}
}

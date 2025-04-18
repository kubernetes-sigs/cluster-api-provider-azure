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
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"k8s.io/utils/ptr"

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
		ClusterName:           "my-cluster",
	}
	fakeSku = resourceskus.SKU{
		Name: ptr.To("Standard_D2v2"),
		Kind: ptr.To(string(resourceskus.VirtualMachines)),
		Locations: []*string{
			ptr.To("fake-location"),
		},
		LocationInfo: []*armcompute.ResourceSKULocationInfo{
			{
				Location: ptr.To("fake-location"),
				Zones:    []*string{ptr.To("1")},
			},
		},
		Capabilities: []*armcompute.ResourceSKUCapabilities{
			{
				Name:  ptr.To(resourceskus.AcceleratedNetworking),
				Value: ptr.To(string(resourceskus.CapabilitySupported)),
			},
		},
	}

	fakeCustomDNSServers = []string{"123.123.123.123", "124.124.124.124"}

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
		ClusterName:             "my-cluster",
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
		ClusterName:             "my-cluster",
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
		ClusterName:               "my-cluster",
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
		ClusterName:           "my-cluster",
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
		AcceleratedNetworking: ptr.To(false),
		ClusterName:           "my-cluster",
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
		ClusterName:           "my-cluster",
	}

	fakeControlPlaneCustomDNSSettingsNICSpec = NICSpec{
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
		DNSServers:                fakeCustomDNSServers,
		ClusterName:               "my-cluster",
	}
	fakeDefaultIPconfigNICSpec = NICSpec{
		Name:                  "my-net-interface",
		ResourceGroup:         "my-rg",
		Location:              "fake-location",
		SubscriptionID:        "123",
		MachineName:           "azure-test1",
		SubnetName:            "my-subnet",
		VNetName:              "my-vnet",
		IPv6Enabled:           false,
		VNetResourceGroup:     "my-rg",
		PublicLBName:          "my-public-lb",
		AcceleratedNetworking: nil,
		SKU:                   &fakeSku,
		EnableIPForwarding:    true,
		IPConfigs:             []IPConfig{},
		ClusterName:           "my-cluster",
	}
	fakeOneIPconfigNICSpec = NICSpec{
		Name:                  "my-net-interface",
		ResourceGroup:         "my-rg",
		Location:              "fake-location",
		SubscriptionID:        "123",
		MachineName:           "azure-test1",
		SubnetName:            "my-subnet",
		VNetName:              "my-vnet",
		IPv6Enabled:           false,
		VNetResourceGroup:     "my-rg",
		PublicLBName:          "my-public-lb",
		AcceleratedNetworking: nil,
		SKU:                   &fakeSku,
		EnableIPForwarding:    true,
		IPConfigs:             []IPConfig{{}},
		ClusterName:           "my-cluster",
	}
	fakeTwoIPconfigNICSpec = NICSpec{
		Name:                  "my-net-interface",
		ResourceGroup:         "my-rg",
		Location:              "fake-location",
		SubscriptionID:        "123",
		MachineName:           "azure-test1",
		SubnetName:            "my-subnet",
		VNetName:              "my-vnet",
		IPv6Enabled:           false,
		VNetResourceGroup:     "my-rg",
		PublicLBName:          "my-public-lb",
		AcceleratedNetworking: nil,
		SKU:                   &fakeSku,
		EnableIPForwarding:    true,
		IPConfigs:             []IPConfig{{}, {}},
		ClusterName:           "my-cluster",
	}
	fakeTwoIPconfigWithPublicNICSpec = NICSpec{
		Name:                  "my-net-interface",
		ResourceGroup:         "my-rg",
		Location:              "fake-location",
		SubscriptionID:        "123",
		MachineName:           "azure-test1",
		SubnetName:            "my-subnet",
		VNetName:              "my-vnet",
		IPv6Enabled:           false,
		VNetResourceGroup:     "my-rg",
		PublicIPName:          "pip-azure-test1",
		AcceleratedNetworking: nil,
		SKU:                   &fakeSku,
		EnableIPForwarding:    true,
		IPConfigs:             []IPConfig{{}, {}},
		ClusterName:           "my-cluster",
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
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.Interface{}))
				g.Expect(result.(armnetwork.Interface)).To(Equal(armnetwork.Interface{
					Tags: map[string]*string{
						"Name": ptr.To("my-net-interface"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
					},
					Location: ptr.To("fake-location"),
					Properties: &armnetwork.InterfacePropertiesFormat{
						Primary:                     nil,
						EnableAcceleratedNetworking: ptr.To(true),
						EnableIPForwarding:          ptr.To(false),
						DNSSettings:                 &armnetwork.InterfaceDNSSettings{},
						IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
							{
								Name: ptr.To("pipConfig"),
								Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
									Primary:                         ptr.To(true),
									LoadBalancerBackendAddressPools: []*armnetwork.BackendAddressPool{{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-public-lb/backendAddressPools/cluster-name-outboundBackendPool")}},
									PrivateIPAllocationMethod:       ptr.To(armnetwork.IPAllocationMethodStatic),
									PrivateIPAddress:                ptr.To("fake.static.ip"),
									Subnet:                          &armnetwork.Subnet{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
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
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.Interface{}))
				g.Expect(result.(armnetwork.Interface)).To(Equal(armnetwork.Interface{
					Tags: map[string]*string{
						"Name": ptr.To("my-net-interface"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
					},
					Location: ptr.To("fake-location"),
					Properties: &armnetwork.InterfacePropertiesFormat{
						EnableAcceleratedNetworking: ptr.To(true),
						EnableIPForwarding:          ptr.To(false),
						DNSSettings:                 &armnetwork.InterfaceDNSSettings{},
						Primary:                     nil,
						IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
							{
								Name: ptr.To("pipConfig"),
								Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
									Primary:                         ptr.To(true),
									LoadBalancerBackendAddressPools: []*armnetwork.BackendAddressPool{{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-public-lb/backendAddressPools/cluster-name-outboundBackendPool")}},
									PrivateIPAllocationMethod:       ptr.To(armnetwork.IPAllocationMethodDynamic),
									Subnet:                          &armnetwork.Subnet{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
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
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.Interface{}))
				g.Expect(result.(armnetwork.Interface)).To(Equal(armnetwork.Interface{
					Tags: map[string]*string{
						"Name": ptr.To("my-net-interface"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
					},
					Location: ptr.To("fake-location"),
					Properties: &armnetwork.InterfacePropertiesFormat{
						EnableAcceleratedNetworking: ptr.To(true),
						EnableIPForwarding:          ptr.To(false),
						DNSSettings:                 &armnetwork.InterfaceDNSSettings{},
						Primary:                     nil,
						IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
							{
								Name: ptr.To("pipConfig"),
								Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
									Primary:                     ptr.To(true),
									Subnet:                      &armnetwork.Subnet{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									PrivateIPAllocationMethod:   ptr.To(armnetwork.IPAllocationMethodDynamic),
									LoadBalancerInboundNatRules: []*armnetwork.InboundNatRule{{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-public-lb/inboundNatRules/azure-test1")}},
									LoadBalancerBackendAddressPools: []*armnetwork.BackendAddressPool{
										{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-public-lb/backendAddressPools/my-public-lb-backendPool")},
										{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-internal-lb/backendAddressPools/my-internal-lb-backendPool")}},
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
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.Interface{}))
				g.Expect(result.(armnetwork.Interface)).To(Equal(armnetwork.Interface{
					Tags: map[string]*string{
						"Name": ptr.To("my-net-interface"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
					},
					Location: ptr.To("fake-location"),
					Properties: &armnetwork.InterfacePropertiesFormat{
						Primary:                     nil,
						EnableAcceleratedNetworking: ptr.To(true),
						EnableIPForwarding:          ptr.To(false),
						DNSSettings:                 &armnetwork.InterfaceDNSSettings{},
						IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
							{
								Name: ptr.To("pipConfig"),
								Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
									Primary:                         ptr.To(true),
									Subnet:                          &armnetwork.Subnet{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									PrivateIPAllocationMethod:       ptr.To(armnetwork.IPAllocationMethodDynamic),
									LoadBalancerBackendAddressPools: []*armnetwork.BackendAddressPool{},
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
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.Interface{}))
				g.Expect(result.(armnetwork.Interface)).To(Equal(armnetwork.Interface{
					Tags: map[string]*string{
						"Name": ptr.To("my-net-interface"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
					},
					Location: ptr.To("fake-location"),
					Properties: &armnetwork.InterfacePropertiesFormat{
						Primary:                     nil,
						EnableAcceleratedNetworking: ptr.To(false),
						EnableIPForwarding:          ptr.To(false),
						DNSSettings:                 &armnetwork.InterfaceDNSSettings{},
						IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
							{
								Name: ptr.To("pipConfig"),
								Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
									Primary:                         ptr.To(true),
									Subnet:                          &armnetwork.Subnet{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									PrivateIPAllocationMethod:       ptr.To(armnetwork.IPAllocationMethodDynamic),
									LoadBalancerBackendAddressPools: []*armnetwork.BackendAddressPool{},
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
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.Interface{}))
				g.Expect(result.(armnetwork.Interface)).To(Equal(armnetwork.Interface{
					Tags: map[string]*string{
						"Name": ptr.To("my-net-interface"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
					},
					Location: ptr.To("fake-location"),
					Properties: &armnetwork.InterfacePropertiesFormat{
						Primary:                     nil,
						EnableAcceleratedNetworking: ptr.To(true),
						EnableIPForwarding:          ptr.To(true),
						DNSSettings:                 &armnetwork.InterfaceDNSSettings{},
						IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
							{
								Name: ptr.To("pipConfig"),
								Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
									Primary:                         ptr.To(true),
									Subnet:                          &armnetwork.Subnet{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									PrivateIPAllocationMethod:       ptr.To(armnetwork.IPAllocationMethodDynamic),
									LoadBalancerBackendAddressPools: []*armnetwork.BackendAddressPool{},
								},
							},
							{
								Name: ptr.To("ipConfigv6"),
								Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
									Subnet:                  &armnetwork.Subnet{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									Primary:                 ptr.To(false),
									PrivateIPAddressVersion: ptr.To(armnetwork.IPVersionIPv6),
								},
							},
						},
					},
				}))
			},
			expectedError: "",
		},
		{
			name:     "get parameters for network interface default ipconfig",
			spec:     &fakeDefaultIPconfigNICSpec,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.Interface{}))
				g.Expect(result.(armnetwork.Interface)).To(Equal(armnetwork.Interface{
					Tags: map[string]*string{
						"Name": ptr.To("my-net-interface"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
					},
					Location: ptr.To("fake-location"),
					Properties: &armnetwork.InterfacePropertiesFormat{
						Primary:                     nil,
						EnableAcceleratedNetworking: ptr.To(true),
						EnableIPForwarding:          ptr.To(true),
						DNSSettings:                 &armnetwork.InterfaceDNSSettings{},
						IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
							{
								Name: ptr.To("pipConfig"),
								Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
									Primary:                         ptr.To(true),
									Subnet:                          &armnetwork.Subnet{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									PrivateIPAllocationMethod:       ptr.To(armnetwork.IPAllocationMethodDynamic),
									LoadBalancerBackendAddressPools: []*armnetwork.BackendAddressPool{},
								},
							},
						},
					},
				}))
			},
			expectedError: "",
		},
		{
			name:     "get parameters for network interface with one ipconfig",
			spec:     &fakeOneIPconfigNICSpec,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.Interface{}))
				g.Expect(result.(armnetwork.Interface)).To(Equal(armnetwork.Interface{
					Tags: map[string]*string{
						"Name": ptr.To("my-net-interface"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
					},
					Location: ptr.To("fake-location"),
					Properties: &armnetwork.InterfacePropertiesFormat{
						Primary:                     nil,
						EnableAcceleratedNetworking: ptr.To(true),
						EnableIPForwarding:          ptr.To(true),
						DNSSettings:                 &armnetwork.InterfaceDNSSettings{},
						IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
							{
								Name: ptr.To("pipConfig"),
								Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
									Primary:                         ptr.To(true),
									Subnet:                          &armnetwork.Subnet{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									PrivateIPAllocationMethod:       ptr.To(armnetwork.IPAllocationMethodDynamic),
									LoadBalancerBackendAddressPools: []*armnetwork.BackendAddressPool{},
								},
							},
						},
					},
				}))
			},
			expectedError: "",
		},
		{
			name:     "get parameters for network interface with two ipconfigs",
			spec:     &fakeTwoIPconfigNICSpec,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.Interface{}))
				g.Expect(result.(armnetwork.Interface)).To(Equal(armnetwork.Interface{
					Tags: map[string]*string{
						"Name": ptr.To("my-net-interface"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
					},
					Location: ptr.To("fake-location"),
					Properties: &armnetwork.InterfacePropertiesFormat{
						Primary:                     nil,
						EnableAcceleratedNetworking: ptr.To(true),
						EnableIPForwarding:          ptr.To(true),
						DNSSettings:                 &armnetwork.InterfaceDNSSettings{},
						IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
							{
								Name: ptr.To("pipConfig"),
								Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
									Primary:                         ptr.To(true),
									Subnet:                          &armnetwork.Subnet{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									PrivateIPAllocationMethod:       ptr.To(armnetwork.IPAllocationMethodDynamic),
									LoadBalancerBackendAddressPools: []*armnetwork.BackendAddressPool{},
								},
							},
							{
								Name: ptr.To("my-net-interface-1"),
								Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
									Primary:                         ptr.To(false),
									Subnet:                          &armnetwork.Subnet{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									PrivateIPAllocationMethod:       ptr.To(armnetwork.IPAllocationMethodDynamic),
									LoadBalancerBackendAddressPools: nil,
								},
							},
						},
					},
				}))
			},
			expectedError: "",
		},
		{
			name:     "get parameters for network interface with two ipconfigs and a public ip",
			spec:     &fakeTwoIPconfigWithPublicNICSpec,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.Interface{}))
				g.Expect(result.(armnetwork.Interface)).To(Equal(armnetwork.Interface{
					Tags: map[string]*string{
						"Name": ptr.To("my-net-interface"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
					},
					Location: ptr.To("fake-location"),
					Properties: &armnetwork.InterfacePropertiesFormat{
						Primary:                     nil,
						EnableAcceleratedNetworking: ptr.To(true),
						EnableIPForwarding:          ptr.To(true),
						DNSSettings:                 &armnetwork.InterfaceDNSSettings{},
						IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
							{
								Name: ptr.To("pipConfig"),
								Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
									Primary:                   ptr.To(true),
									Subnet:                    &armnetwork.Subnet{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									PrivateIPAllocationMethod: ptr.To(armnetwork.IPAllocationMethodDynamic),
									PublicIPAddress: &armnetwork.PublicIPAddress{
										ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/publicIPAddresses/pip-azure-test1"),
									},
									LoadBalancerBackendAddressPools: []*armnetwork.BackendAddressPool{},
								},
							},
							{
								Name: ptr.To("my-net-interface-1"),
								Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
									Primary:                         ptr.To(false),
									Subnet:                          &armnetwork.Subnet{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									PrivateIPAllocationMethod:       ptr.To(armnetwork.IPAllocationMethodDynamic),
									LoadBalancerBackendAddressPools: nil,
								},
							},
						},
					},
				}))
			},
			expectedError: "",
		},
		{
			name:     "get parameters for control plane network interface with DNS servers",
			spec:     &fakeControlPlaneCustomDNSSettingsNICSpec,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.Interface{}))
				g.Expect(result.(armnetwork.Interface)).To(Equal(armnetwork.Interface{
					Tags: map[string]*string{
						"Name": ptr.To("my-net-interface"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
					},
					Location: ptr.To("fake-location"),
					Properties: &armnetwork.InterfacePropertiesFormat{
						EnableAcceleratedNetworking: ptr.To(true),
						EnableIPForwarding:          ptr.To(false),
						DNSSettings: &armnetwork.InterfaceDNSSettings{
							DNSServers: []*string{ptr.To("123.123.123.123"), ptr.To("124.124.124.124")},
						},
						IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
							{
								Name: ptr.To("pipConfig"),
								Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
									Subnet:                      &armnetwork.Subnet{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									Primary:                     ptr.To(true),
									PrivateIPAllocationMethod:   ptr.To(armnetwork.IPAllocationMethodDynamic),
									LoadBalancerInboundNatRules: []*armnetwork.InboundNatRule{{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-public-lb/inboundNatRules/azure-test1")}},
									LoadBalancerBackendAddressPools: []*armnetwork.BackendAddressPool{
										{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-public-lb/backendAddressPools/my-public-lb-backendPool")},
										{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-internal-lb/backendAddressPools/my-internal-lb-backendPool")}},
								},
							},
						},
					},
				}))
			},
			expectedError: "",
		},
		{
			name: "recreate parameters for network interface when Azure provisioning state is Failed",
			spec: func() *NICSpec {
				s := fakeStaticPrivateIPNICSpec // value‑copy
				return &s                       // pointer to the copy, not the global
			}(),
			existing: armnetwork.Interface{
				ID:       ptr.To(""),
				Name:     ptr.To("my-net-interface"),
				Location: ptr.To("fake-location"),
				Type:     ptr.To("Microsoft.Network/networkInterfaces"),
				Properties: &armnetwork.InterfacePropertiesFormat{
					ProvisioningState: ptr.To(armnetwork.ProvisioningStateFailed),
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.Interface{}))
				g.Expect(result.(armnetwork.Interface)).To(Equal(armnetwork.Interface{
					Tags: map[string]*string{
						"Name": ptr.To("my-net-interface"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
					},
					Location: ptr.To("fake-location"),
					Properties: &armnetwork.InterfacePropertiesFormat{
						Primary:                     nil,
						EnableAcceleratedNetworking: ptr.To(true),
						EnableIPForwarding:          ptr.To(false),
						DNSSettings:                 &armnetwork.InterfaceDNSSettings{},
						IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
							{
								Name: ptr.To("pipConfig"),
								Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
									Primary:                         ptr.To(true),
									LoadBalancerBackendAddressPools: []*armnetwork.BackendAddressPool{{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-public-lb/backendAddressPools/cluster-name-outboundBackendPool")}},
									PrivateIPAllocationMethod:       ptr.To(armnetwork.IPAllocationMethodStatic),
									PrivateIPAddress:                ptr.To("fake.static.ip"),
									Subnet:                          &armnetwork.Subnet{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
								},
							},
						},
					},
				}))
			},
			expectedError: "",
		},
		{
			name: "do not recreate parameters for network interface when Azure provisioning state is Deleting",
			spec: func() *NICSpec {
				s := fakeStaticPrivateIPNICSpec // value‑copy
				return &s                       // pointer to the copy, not the global
			}(),
			existing: armnetwork.Interface{
				ID:       ptr.To(""),
				Name:     ptr.To("my-net-interface"),
				Location: ptr.To("fake-location"),
				Type:     ptr.To("Microsoft.Network/networkInterfaces"),
				Properties: &armnetwork.InterfacePropertiesFormat{
					ProvisioningState: ptr.To(armnetwork.ProvisioningStateDeleting),
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
		{
			name: "do not recreate parameters for network interface when Azure provisioning state is Succeeded",
			spec: func() *NICSpec {
				s := fakeStaticPrivateIPNICSpec // value‑copy
				return &s                       // pointer to the copy, not the global
			}(),
			existing: armnetwork.Interface{
				ID:       ptr.To(""),
				Name:     ptr.To("my-net-interface"),
				Location: ptr.To("fake-location"),
				Type:     ptr.To("Microsoft.Network/networkInterfaces"),
				Properties: &armnetwork.InterfacePropertiesFormat{
					ProvisioningState: ptr.To(armnetwork.ProvisioningStateSucceeded),
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
		{
			name: "do not recreate parameters for network interface when Azure provisioning state is Updating",
			spec: func() *NICSpec {
				s := fakeStaticPrivateIPNICSpec // value‑copy
				return &s                       // pointer to the copy, not the global
			}(),
			existing: armnetwork.Interface{
				ID:       ptr.To(""),
				Name:     ptr.To("my-net-interface"),
				Location: ptr.To("fake-location"),
				Type:     ptr.To("Microsoft.Network/networkInterfaces"),
				Properties: &armnetwork.InterfacePropertiesFormat{
					ProvisioningState: ptr.To(armnetwork.ProvisioningStateUpdating),
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
		{
			name: "recreate parameters for network interface when Azure provisioning state nil",
			spec: func() *NICSpec {
				s := fakeStaticPrivateIPNICSpec // value‑copy
				return &s                       // pointer to the copy, not the global
			}(),
			existing: armnetwork.Interface{
				ID:       ptr.To(""),
				Name:     ptr.To("my-net-interface"),
				Location: ptr.To("fake-location"),
				Type:     ptr.To("Microsoft.Network/networkInterfaces"),
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.Interface{}))
				g.Expect(result.(armnetwork.Interface)).To(Equal(armnetwork.Interface{
					Tags: map[string]*string{
						"Name": ptr.To("my-net-interface"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
					},
					Location: ptr.To("fake-location"),
					Properties: &armnetwork.InterfacePropertiesFormat{
						Primary:                     nil,
						EnableAcceleratedNetworking: ptr.To(true),
						EnableIPForwarding:          ptr.To(false),
						DNSSettings:                 &armnetwork.InterfaceDNSSettings{},
						IPConfigurations: []*armnetwork.InterfaceIPConfiguration{
							{
								Name: ptr.To("pipConfig"),
								Properties: &armnetwork.InterfaceIPConfigurationPropertiesFormat{
									Primary:                         ptr.To(true),
									LoadBalancerBackendAddressPools: []*armnetwork.BackendAddressPool{{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-public-lb/backendAddressPools/cluster-name-outboundBackendPool")}},
									PrivateIPAllocationMethod:       ptr.To(armnetwork.IPAllocationMethodStatic),
									PrivateIPAddress:                ptr.To("fake.static.ip"),
									Subnet:                          &armnetwork.Subnet{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
								},
							},
						},
					},
				}))
			},
			expectedError: "",
		},
	}
	format.MaxLength = 10000
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(context.TODO(), tc.existing)
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

/*
Copyright 2020 The Kubernetes Authors.

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
	"fmt"
	"net/http"
	"testing"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"k8s.io/klog/klogr"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/networkinterfaces/mock_networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
)

func TestReconcileNetworkInterface(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_networkinterfaces.MockNICScopeMockRecorder, m *mock_networkinterfaces.MockClientMockRecorder)
	}{
		{
			name:          "network interface already exists",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, m *mock_networkinterfaces.MockClientMockRecorder) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:              "nic-1",
						MachineName:       "azure-test1",
						SubnetName:        "my-subnet",
						VNetName:          "my-vnet",
						VNetResourceGroup: "my-rg",
						VMSize:            "Standard_D2v2",
					},
					{
						Name:              "nic-2",
						MachineName:       "azure-test1",
						SubnetName:        "my-subnet",
						VNetName:          "my-vnet",
						VNetResourceGroup: "my-rg",
						VMSize:            "Standard_D2v2",
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				gomock.InOrder(
					m.Get(gomockinternal.AContext(), "my-rg", "nic-1"),
					m.Get(gomockinternal.AContext(), "my-rg", "nic-2"))
			},
		},
		{
			name:          "node network interface create fails",
			expectedError: "failed to create network interface my-net-interface in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, m *mock_networkinterfaces.MockClientMockRecorder) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                  "my-net-interface",
						MachineName:           "azure-test1",
						SubnetName:            "my-subnet",
						VNetName:              "my-vnet",
						VNetResourceGroup:     "my-rg",
						PublicLBName:          "my-public-lb",
						VMSize:                "Standard_D2v2",
						AcceleratedNetworking: nil,
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				gomock.InOrder(
					m.Get(gomockinternal.AContext(), "my-rg", "my-net-interface").
						Return(network.Interface{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found")),
					m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-net-interface", gomock.AssignableToTypeOf(network.Interface{})).
						Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")))
			},
		},
		{
			name:          "node network interface with Static private IP successfully created",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, m *mock_networkinterfaces.MockClientMockRecorder) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                    "my-net-interface",
						MachineName:             "azure-test1",
						SubnetName:              "my-subnet",
						VNetName:                "my-vnet",
						VNetResourceGroup:       "my-rg",
						PublicLBName:            "my-public-lb",
						PublicLBAddressPoolName: "cluster-name-outboundBackendPool",
						StaticIPAddress:         "fake.static.ip",
						VMSize:                  "Standard_D2v2",
						AcceleratedNetworking:   nil,
					},
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Get(gomockinternal.AContext(), "my-rg", "my-net-interface").
					Return(network.Interface{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-net-interface", gomockinternal.DiffEq(network.Interface{
					Location: to.StringPtr("fake-location"),
					InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
						EnableAcceleratedNetworking: to.BoolPtr(true),
						EnableIPForwarding:          to.BoolPtr(false),
						IPConfigurations: &[]network.InterfaceIPConfiguration{
							{
								Name: to.StringPtr("pipConfig"),
								InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
									LoadBalancerBackendAddressPools: &[]network.BackendAddressPool{{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-public-lb/backendAddressPools/cluster-name-outboundBackendPool")}},
									PrivateIPAllocationMethod:       network.Static,
									PrivateIPAddress:                to.StringPtr("fake.static.ip"),
									Subnet:                          &network.Subnet{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
								},
							},
						},
					},
				}))
			},
		},
		{
			name:          "node network interface with Dynamic private IP successfully created",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, m *mock_networkinterfaces.MockClientMockRecorder) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                    "my-net-interface",
						MachineName:             "azure-test1",
						SubnetName:              "my-subnet",
						VNetName:                "my-vnet",
						VNetResourceGroup:       "my-rg",
						PublicLBName:            "my-public-lb",
						PublicLBAddressPoolName: "cluster-name-outboundBackendPool",
						VMSize:                  "Standard_D2v2",
						AcceleratedNetworking:   nil,
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.V(gomock.AssignableToTypeOf(3)).AnyTimes().Return(klogr.New())
				gomock.InOrder(
					m.Get(gomockinternal.AContext(), "my-rg", "my-net-interface").
						Return(network.Interface{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found")),
					m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-net-interface", gomockinternal.DiffEq(network.Interface{
						Location: to.StringPtr("fake-location"),
						InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
							EnableAcceleratedNetworking: to.BoolPtr(true),
							EnableIPForwarding:          to.BoolPtr(false),
							IPConfigurations: &[]network.InterfaceIPConfiguration{
								{
									Name: to.StringPtr("pipConfig"),
									InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
										LoadBalancerBackendAddressPools: &[]network.BackendAddressPool{{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-public-lb/backendAddressPools/cluster-name-outboundBackendPool")}},
										PrivateIPAllocationMethod:       network.Dynamic,
										Subnet:                          &network.Subnet{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									},
								},
							},
						},
					})))
			},
		},
		{
			name:          "control plane network interface successfully created",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, m *mock_networkinterfaces.MockClientMockRecorder) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                      "my-net-interface",
						MachineName:               "azure-test1",
						SubnetName:                "my-subnet",
						VNetName:                  "my-vnet",
						VNetResourceGroup:         "my-rg",
						PublicLBName:              "my-public-lb",
						PublicLBAddressPoolName:   "my-public-lb-backendPool",
						PublicLBNATRuleName:       "azure-test1",
						InternalLBName:            "my-internal-lb",
						InternalLBAddressPoolName: "my-internal-lb-backendPool",
						VMSize:                    "Standard_D2v2",
						AcceleratedNetworking:     nil,
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.V(gomock.AssignableToTypeOf(3)).AnyTimes().Return(klogr.New())
				m.Get(gomockinternal.AContext(), "my-rg", "my-net-interface").
					Return(network.Interface{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-net-interface", gomockinternal.DiffEq(network.Interface{
					Location: to.StringPtr("fake-location"),
					InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
						EnableAcceleratedNetworking: to.BoolPtr(true),
						EnableIPForwarding:          to.BoolPtr(false),
						IPConfigurations: &[]network.InterfaceIPConfiguration{
							{
								Name: to.StringPtr("pipConfig"),
								InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
									Subnet:                      &network.Subnet{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									PrivateIPAllocationMethod:   network.Dynamic,
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
		},
		{
			name:          "network interface with Public IP successfully created",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, m *mock_networkinterfaces.MockClientMockRecorder) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                  "my-public-net-interface",
						MachineName:           "azure-test1",
						SubnetName:            "my-subnet",
						VNetName:              "my-vnet",
						VNetResourceGroup:     "my-rg",
						PublicIPName:          "my-public-ip",
						VMSize:                "Standard_D2v2",
						AcceleratedNetworking: nil,
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.V(gomock.AssignableToTypeOf(3)).AnyTimes().Return(klogr.New())
				m.Get(gomockinternal.AContext(), "my-rg", "my-public-net-interface").
					Return(network.Interface{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-public-net-interface", gomock.AssignableToTypeOf(network.Interface{}))
			},
		},
		{
			name:          "network interface with accelerated networking successfully created",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, m *mock_networkinterfaces.MockClientMockRecorder) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                  "my-net-interface",
						MachineName:           "azure-test1",
						SubnetName:            "my-subnet",
						VNetName:              "my-vnet",
						VNetResourceGroup:     "my-rg",
						PublicLBName:          "my-public-lb",
						VMSize:                "Standard_D2v2",
						AcceleratedNetworking: nil,
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Get(gomockinternal.AContext(), "my-rg", "my-net-interface").
					Return(network.Interface{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-net-interface", gomockinternal.DiffEq(network.Interface{
					Location: to.StringPtr("fake-location"),
					InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
						EnableAcceleratedNetworking: to.BoolPtr(true),
						EnableIPForwarding:          to.BoolPtr(false),
						IPConfigurations: &[]network.InterfaceIPConfiguration{
							{
								Name: to.StringPtr("pipConfig"),
								InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
									Subnet:                          &network.Subnet{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									PrivateIPAllocationMethod:       network.Dynamic,
									LoadBalancerBackendAddressPools: &[]network.BackendAddressPool{},
								},
							},
						},
					},
				}),
				)
			},
		},
		{
			name:          "network interface without accelerated networking successfully created",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, m *mock_networkinterfaces.MockClientMockRecorder) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                  "my-net-interface",
						MachineName:           "azure-test1",
						SubnetName:            "my-subnet",
						VNetName:              "my-vnet",
						VNetResourceGroup:     "my-rg",
						PublicLBName:          "my-public-lb",
						VMSize:                "Standard_D2v2",
						AcceleratedNetworking: to.BoolPtr(false),
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.Location().AnyTimes().Return("fake-location")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Get(gomockinternal.AContext(), "my-rg", "my-net-interface").
					Return(network.Interface{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-net-interface", gomockinternal.DiffEq(network.Interface{
					Location: to.StringPtr("fake-location"),
					InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
						EnableAcceleratedNetworking: to.BoolPtr(false),
						EnableIPForwarding:          to.BoolPtr(false),
						IPConfigurations: &[]network.InterfaceIPConfiguration{
							{
								Name: to.StringPtr("pipConfig"),
								InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
									Subnet:                          &network.Subnet{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
									PrivateIPAllocationMethod:       network.Dynamic,
									LoadBalancerBackendAddressPools: &[]network.BackendAddressPool{},
								},
							},
						},
					},
				}))
			},
		},
		{
			name:          "network interface with ipv6 created successfully",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, m *mock_networkinterfaces.MockClientMockRecorder) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                  "my-net-interface",
						MachineName:           "azure-test1",
						SubnetName:            "my-subnet",
						VNetName:              "my-vnet",
						IPv6Enabled:           true,
						VNetResourceGroup:     "my-rg",
						PublicLBName:          "my-public-lb",
						VMSize:                "Standard_D2v2",
						AcceleratedNetworking: nil,
						EnableIPForwarding:    true,
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				gomock.InOrder(
					m.Get(gomockinternal.AContext(), "my-rg", "my-net-interface").
						Return(network.Interface{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found")),
					m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-net-interface", gomockinternal.DiffEq(network.Interface{
						Location: to.StringPtr("fake-location"),
						InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
							EnableAcceleratedNetworking: to.BoolPtr(true),
							EnableIPForwarding:          to.BoolPtr(true),
							IPConfigurations: &[]network.InterfaceIPConfiguration{
								{
									Name: to.StringPtr("pipConfig"),
									InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
										Subnet:                          &network.Subnet{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet")},
										PrivateIPAllocationMethod:       network.Dynamic,
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
					})),
				)
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_networkinterfaces.NewMockNICScope(mockCtrl)
			clientMock := mock_networkinterfaces.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				Client: clientMock,
				resourceSKUCache: resourceskus.NewStaticCache([]compute.ResourceSku{
					{
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
					},
				}),
			}

			err := s.Reconcile(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				fmt.Print(cmp.Diff(err.Error(), tc.expectedError))

				g.Expect(err.Error()).To(Equal(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteNetworkInterface(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_networkinterfaces.MockNICScopeMockRecorder, m *mock_networkinterfaces.MockClientMockRecorder)
	}{
		{
			name:          "successfully delete an existing network interface",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, m *mock_networkinterfaces.MockClientMockRecorder) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:         "my-net-interface",
						PublicLBName: "my-public-lb",
						MachineName:  "azure-test1",
					},
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-net-interface")
			},
		},
		{
			name:          "network interface already deleted",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, m *mock_networkinterfaces.MockClientMockRecorder) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:         "my-net-interface",
						PublicLBName: "my-public-lb",
						MachineName:  "azure-test1",
					},
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-net-interface").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name:          "network interface deletion fails",
			expectedError: "failed to delete network interface my-net-interface in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder, m *mock_networkinterfaces.MockClientMockRecorder) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:         "my-net-interface",
						PublicLBName: "my-public-lb",
						MachineName:  "azure-test1",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Delete(gomockinternal.AContext(), "my-rg", "my-net-interface").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_networkinterfaces.NewMockNICScope(mockCtrl)
			clientMock := mock_networkinterfaces.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				Client: clientMock,
			}

			err := s.Delete(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

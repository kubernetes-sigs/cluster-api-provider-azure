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

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/loadbalancers/mock_loadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/networkinterfaces/mock_networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips/mock_publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/subnets/mock_subnets"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"k8s.io/klog/klogr"
	"k8s.io/utils/pointer"
)

func TestReconcileNetworkInterface(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_networkinterfaces.MockNICScopeMockRecorder,
			m *mock_networkinterfaces.MockClientMockRecorder,
			mSubnet *mock_subnets.MockClientMockRecorder,
			mLoadBalancer *mock_loadbalancers.MockClientMockRecorder,
			mPublicIP *mock_publicips.MockClientMockRecorder,
		)
	}{
		{
			name:          "get subnets fails",
			expectedError: "failed to get subnets: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder,
				m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
			) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:              "my-net-interface",
						MachineName:       "azure-test1",
						SubnetName:        "my-subnet",
						VNetName:          "my-vnet",
						VNetResourceGroup: "vnet-rg",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				mSubnet.Get(context.TODO(), "vnet-rg", "my-vnet", "my-subnet").
					Return(network.Subnet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "node network interface create fails",
			expectedError: "failed to create network interface my-net-interface in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder,
				m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
			) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                   "my-net-interface",
						MachineName:            "azure-test1",
						MachineRole:            infrav1.Node,
						SubnetName:             "my-subnet",
						VNetName:               "my-vnet",
						VNetResourceGroup:      "my-rg",
						PublicLoadBalancerName: "my-public-lb",
						VMSize:                 "Standard_D2v2",
						AcceleratedNetworking:  nil,
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").
						Return(network.Subnet{}, nil),
					mLoadBalancer.Get(context.TODO(), "my-rg", "my-public-lb").Return(getFakeNodeOutboundLoadBalancer(), nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", gomock.AssignableToTypeOf(network.Interface{})).
						Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")))
			},
		},
		{
			name:          "node network interface with Static private IP successfully created",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder,
				m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
			) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                   "my-net-interface",
						MachineName:            "azure-test1",
						MachineRole:            infrav1.Node,
						SubnetName:             "my-subnet",
						VNetName:               "my-vnet",
						VNetResourceGroup:      "my-rg",
						PublicLoadBalancerName: "my-public-lb",
						StaticIPAddress:        "fake.static.ip",
						VMSize:                 "Standard_D2v2",
						AcceleratedNetworking:  nil,
					},
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mLoadBalancer.Get(context.TODO(), "my-rg", "my-public-lb").Return(getFakeNodeOutboundLoadBalancer(), nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", matchers.DiffEq(network.Interface{
						Location: to.StringPtr("fake-location"),
						InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
							EnableAcceleratedNetworking: to.BoolPtr(true),
							IPConfigurations: &[]network.InterfaceIPConfiguration{
								{
									Name: to.StringPtr("pipConfig"),
									InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
										LoadBalancerBackendAddressPools: &[]network.BackendAddressPool{{ID: to.StringPtr("cluster-name-outboundBackendPool")}},
										PrivateIPAllocationMethod:       network.Static,
										PrivateIPAddress:                to.StringPtr("fake.static.ip"),
										Subnet:                          &network.Subnet{},
									},
								},
							},
						},
					})))
			},
		},
		{
			name:          "node network interface with Dynamic private IP successfully created",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder,
				m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
			) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                   "my-net-interface",
						MachineName:            "azure-test1",
						MachineRole:            infrav1.Node,
						SubnetName:             "my-subnet",
						VNetName:               "my-vnet",
						VNetResourceGroup:      "my-rg",
						PublicLoadBalancerName: "my-public-lb",
						VMSize:                 "Standard_D2v2",
						AcceleratedNetworking:  nil,
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.V(gomock.AssignableToTypeOf(3)).AnyTimes().Return(klogr.New())
				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mLoadBalancer.Get(context.TODO(), "my-rg", "my-public-lb").Return(getFakeNodeOutboundLoadBalancer(), nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", matchers.DiffEq(network.Interface{
						Location: to.StringPtr("fake-location"),
						InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
							EnableAcceleratedNetworking: to.BoolPtr(true),
							IPConfigurations: &[]network.InterfaceIPConfiguration{
								{
									Name: to.StringPtr("pipConfig"),
									InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
										LoadBalancerBackendAddressPools: &[]network.BackendAddressPool{{ID: to.StringPtr("cluster-name-outboundBackendPool")}},
										PrivateIPAllocationMethod:       network.Dynamic,
										Subnet:                          &network.Subnet{},
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
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder,
				m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
			) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                     "my-net-interface",
						MachineName:              "azure-test1",
						MachineRole:              infrav1.ControlPlane,
						SubnetName:               "my-subnet",
						VNetName:                 "my-vnet",
						VNetResourceGroup:        "my-rg",
						PublicLoadBalancerName:   "my-public-lb",
						InternalLoadBalancerName: "my-internal-lb",
						VMSize:                   "Standard_D2v2",
						AcceleratedNetworking:    nil,
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.V(gomock.AssignableToTypeOf(3)).AnyTimes().Return(klogr.New())

				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").
						Return(network.Subnet{ID: to.StringPtr("my-subnet-id")}, nil),
					mLoadBalancer.Get(context.TODO(), "my-rg", "my-public-lb").Return(network.LoadBalancer{
						Name: to.StringPtr("my-public-lb"),
						ID:   pointer.StringPtr("my-public-lb-id"),
						LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
							FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
								{
									ID: to.StringPtr("frontend-ip-config-id"),
								},
							},
							BackendAddressPools: &[]network.BackendAddressPool{
								{
									ID: pointer.StringPtr("my-backend-pool-id"),
								},
							},
							InboundNatRules: &[]network.InboundNatRule{},
						}}, nil),
					mLoadBalancer.Get(context.TODO(), "my-rg", "my-internal-lb").
						Return(network.LoadBalancer{
							ID: pointer.StringPtr("my-internal-lb-id"),
							LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
								BackendAddressPools: &[]network.BackendAddressPool{
									{
										ID: pointer.StringPtr("my-internal-backend-pool-id"),
									},
								},
							}}, nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", matchers.DiffEq(network.Interface{
						Location: to.StringPtr("fake-location"),
						InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
							EnableAcceleratedNetworking: to.BoolPtr(true),
							IPConfigurations: &[]network.InterfaceIPConfiguration{
								{
									Name: to.StringPtr("pipConfig"),
									InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
										Subnet:                          &network.Subnet{ID: to.StringPtr("my-subnet-id")},
										PrivateIPAllocationMethod:       network.Dynamic,
										LoadBalancerInboundNatRules:     &[]network.InboundNatRule{{ID: to.StringPtr("my-public-lb-id/inboundNatRules/azure-test1")}},
										LoadBalancerBackendAddressPools: &[]network.BackendAddressPool{{ID: to.StringPtr("my-backend-pool-id")}, {ID: to.StringPtr("my-internal-backend-pool-id")}},
									},
								},
							},
						},
					})))
			},
		},
		{
			name:          "control plane network interface fail to get public LB",
			expectedError: "failed to get public LB: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder,
				m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
			) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                     "my-net-interface",
						MachineName:              "azure-test1",
						MachineRole:              infrav1.ControlPlane,
						SubnetName:               "my-subnet",
						VNetName:                 "my-vnet",
						VNetResourceGroup:        "my-rg",
						PublicLoadBalancerName:   "my-public-lb",
						InternalLoadBalancerName: "my-internal-lb",
						VMSize:                   "Standard_D2v2",
						AcceleratedNetworking:    nil,
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mLoadBalancer.Get(context.TODO(), "my-rg", "my-public-lb").
						Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")))
			},
		},
		{
			name:          "control plane network interface fail to get internal LB",
			expectedError: "failed to get internalLB: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder,
				m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
			) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                     "my-net-interface",
						MachineName:              "azure-test1",
						MachineRole:              infrav1.ControlPlane,
						SubnetName:               "my-subnet",
						VNetName:                 "my-vnet",
						VNetResourceGroup:        "my-rg",
						InternalLoadBalancerName: "my-internal-lb",
						VMSize:                   "Standard_D2v2",
						AcceleratedNetworking:    nil,
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.V(gomock.AssignableToTypeOf(3)).AnyTimes().Return(klogr.New())

				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mLoadBalancer.Get(context.TODO(), "my-rg", "my-internal-lb").
						Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")))
			},
		},
		{
			name:          "network interface with Public IP successfully created",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder,
				m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
			) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                  "my-public-net-interface",
						MachineName:           "azure-test1",
						MachineRole:           infrav1.Node,
						SubnetName:            "my-subnet",
						VNetName:              "my-vnet",
						VNetResourceGroup:     "my-rg",
						PublicIPName:          "my-public-ip",
						VMSize:                "Standard_D2v2",
						AcceleratedNetworking: nil,
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.V(gomock.AssignableToTypeOf(3)).AnyTimes().Return(klogr.New())

				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mPublicIP.Get(context.TODO(), "my-rg", "my-public-ip").Return(network.PublicIPAddress{}, nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-public-net-interface", gomock.AssignableToTypeOf(network.Interface{})),
				)
			},
		},
		{
			name:          "network interface with Public IP fail to get Public IP",
			expectedError: "failed to get publicIP: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder,
				m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
			) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                   "my-net-interface",
						MachineName:            "azure-test1",
						MachineRole:            infrav1.ControlPlane,
						SubnetName:             "my-subnet",
						VNetName:               "my-vnet",
						VNetResourceGroup:      "my-rg",
						PublicLoadBalancerName: "my-public-lb",
						PublicIPName:           "my-public-ip",
						VMSize:                 "Standard_D2v2",
						AcceleratedNetworking:  nil,
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.V(gomock.Any()).AnyTimes().Return(klogr.New())
				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mLoadBalancer.Get(context.TODO(), "my-rg", "my-public-lb").Return(network.LoadBalancer{
						Name: to.StringPtr("my-public-lb"),
						ID:   pointer.StringPtr("my-public-lb-id"),
						LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
							FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
								{
									ID: to.StringPtr("frontend-ip-config-id"),
								},
							},
							BackendAddressPools: &[]network.BackendAddressPool{
								{
									ID: pointer.StringPtr("my-backend-pool-id"),
								},
							},
							InboundNatRules: &[]network.InboundNatRule{},
						},
					}, nil),
					mPublicIP.Get(context.TODO(), "my-rg", "my-public-ip").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")))
			},
		},
		{
			name:          "network interface with accelerated networking successfully created",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder,
				m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
			) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                   "my-net-interface",
						MachineName:            "azure-test1",
						MachineRole:            infrav1.Node,
						SubnetName:             "my-subnet",
						VNetName:               "my-vnet",
						VNetResourceGroup:      "my-rg",
						PublicLoadBalancerName: "my-public-lb",
						VMSize:                 "Standard_D2v2",
						AcceleratedNetworking:  nil,
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mLoadBalancer.Get(context.TODO(), "my-rg", "my-public-lb").Return(getFakeNodeOutboundLoadBalancer(), nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", matchers.DiffEq(network.Interface{
						Location: to.StringPtr("fake-location"),
						InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
							EnableAcceleratedNetworking: to.BoolPtr(true),
							IPConfigurations: &[]network.InterfaceIPConfiguration{
								{
									Name: to.StringPtr("pipConfig"),
									InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
										Subnet:                          &network.Subnet{},
										PrivateIPAllocationMethod:       network.Dynamic,
										LoadBalancerBackendAddressPools: &[]network.BackendAddressPool{{ID: to.StringPtr("cluster-name-outboundBackendPool")}},
									},
								},
							},
						},
					})),
				)
			},
		},
		{
			name:          "network interface without accelerated networking successfully created",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder,
				m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_loadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder,
			) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                   "my-net-interface",
						MachineName:            "azure-test1",
						MachineRole:            infrav1.Node,
						SubnetName:             "my-subnet",
						VNetName:               "my-vnet",
						VNetResourceGroup:      "my-rg",
						PublicLoadBalancerName: "my-public-lb",
						VMSize:                 "Standard_D2v2",
						AcceleratedNetworking:  to.BoolPtr(false),
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.Location().AnyTimes().Return("fake-location")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())

				gomock.InOrder(
					mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mLoadBalancer.Get(context.TODO(), "my-rg", "my-public-lb").Return(getFakeNodeOutboundLoadBalancer(), nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", matchers.DiffEq(network.Interface{
						Location: to.StringPtr("fake-location"),
						InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
							EnableAcceleratedNetworking: to.BoolPtr(false),
							IPConfigurations: &[]network.InterfaceIPConfiguration{
								{
									Name: to.StringPtr("pipConfig"),
									InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
										Subnet:                          &network.Subnet{},
										PrivateIPAllocationMethod:       network.Dynamic,
										LoadBalancerBackendAddressPools: &[]network.BackendAddressPool{{ID: to.StringPtr("cluster-name-outboundBackendPool")}},
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
			// t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_networkinterfaces.NewMockNICScope(mockCtrl)
			clientMock := mock_networkinterfaces.NewMockClient(mockCtrl)
			subnetMock := mock_subnets.NewMockClient(mockCtrl)
			loadBalancerMock := mock_loadbalancers.NewMockClient(mockCtrl)
			publicIPsMock := mock_publicips.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), subnetMock.EXPECT(),
				loadBalancerMock.EXPECT(), publicIPsMock.EXPECT(),
			)

			s := &Service{
				Scope:               scopeMock,
				Client:              clientMock,
				SubnetsClient:       subnetMock,
				LoadBalancersClient: loadBalancerMock,
				PublicIPsClient:     publicIPsMock,
				ResourceSKUCache: resourceskus.NewStaticCache([]compute.ResourceSku{
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
				fmt.Printf(cmp.Diff(err.Error(), tc.expectedError))

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
		expect        func(s *mock_networkinterfaces.MockNICScopeMockRecorder,
			m *mock_networkinterfaces.MockClientMockRecorder, mPublicIP *mock_publicips.MockClientMockRecorder)
	}{
		{
			name:          "successfully delete an existing network interface",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder,
				m *mock_networkinterfaces.MockClientMockRecorder, mPublicIP *mock_publicips.MockClientMockRecorder) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                   "my-net-interface",
						PublicLoadBalancerName: "my-public-lb",
						MachineName:            "azure-test1",
					},
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(context.TODO(), "my-rg", "my-net-interface")
			},
		},
		{
			name:          "network interface already deleted",
			expectedError: "",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder,
				m *mock_networkinterfaces.MockClientMockRecorder, mPublicIP *mock_publicips.MockClientMockRecorder) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                   "my-net-interface",
						PublicLoadBalancerName: "my-public-lb",
						MachineName:            "azure-test1",
					},
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(context.TODO(), "my-rg", "my-net-interface").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name:          "network interface deletion fails",
			expectedError: "failed to delete network interface my-net-interface in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_networkinterfaces.MockNICScopeMockRecorder,
				m *mock_networkinterfaces.MockClientMockRecorder, mPublicIP *mock_publicips.MockClientMockRecorder) {
				s.NICSpecs().Return([]azure.NICSpec{
					{
						Name:                   "my-net-interface",
						PublicLoadBalancerName: "my-public-lb",
						MachineName:            "azure-test1",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Delete(context.TODO(), "my-rg", "my-net-interface").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			// t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_networkinterfaces.NewMockNICScope(mockCtrl)
			clientMock := mock_networkinterfaces.NewMockClient(mockCtrl)
			publicIPMock := mock_publicips.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), publicIPMock.EXPECT())

			s := &Service{
				Scope:           scopeMock,
				Client:          clientMock,
				PublicIPsClient: publicIPMock,
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

func getFakeNodeOutboundLoadBalancer() network.LoadBalancer {
	return network.LoadBalancer{
		LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
			InboundNatRules: &[]network.InboundNatRule{{
				Name: pointer.StringPtr("azure-test1"),
				InboundNatRulePropertiesFormat: &network.InboundNatRulePropertiesFormat{
					FrontendPort:         to.Int32Ptr(22),
					BackendPort:          to.Int32Ptr(22),
					EnableFloatingIP:     to.BoolPtr(false),
					IdleTimeoutInMinutes: to.Int32Ptr(4),
					FrontendIPConfiguration: &network.SubResource{
						ID: to.StringPtr("frontend-ip-config-id"),
					},
					Protocol: network.TransportProtocolTCP,
				},
			}},
			FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
				{
					ID: to.StringPtr("frontend-ip-config-id"),
				},
			},
			BackendAddressPools: &[]network.BackendAddressPool{
				{
					ID: pointer.StringPtr("cluster-name-outboundBackendPool"),
				},
			},
		}}
}

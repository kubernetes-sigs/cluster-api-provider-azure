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

package loadbalancers

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/klog/klogr"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/loadbalancers/mock_loadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualnetworks/mock_virtualnetworks"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

func TestReconcileLoadBalancer(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder)
	}{
		{
			name:          "fail to create a public LB",
			expectedError: "failed to create load balancer \"my-publiclb\": #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder) {
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name: "my-publiclb",
						Role: infrav1.APIServerRole,
					},
				})
				setupDefaultLBExpectations(s)
				m.Get(gomockinternal.AContext(), "my-rg", "my-publiclb").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-publiclb", gomock.AssignableToTypeOf(network.LoadBalancer{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "create public apiserver LB",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder) {
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name:            "my-publiclb",
						Role:            infrav1.APIServerRole,
						Type:            infrav1.Public,
						SKU:             infrav1.SKUStandard,
						SubnetName:      "my-cp-subnet",
						BackendPoolName: "my-publiclb-backendPool",
						FrontendIPConfigs: []infrav1.FrontendIP{
							{
								Name: "my-publiclb-frontEnd",
								PublicIP: &infrav1.PublicIPSpec{
									Name:    "my-publicip",
									DNSName: "my-cluster.12345.mydomain.com",
								},
							},
						},
						APIServerPort: 6443,
					},
				})
				setupDefaultLBExpectations(s)
				gomock.InOrder(
					m.Get(gomockinternal.AContext(), "my-rg", "my-publiclb").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found")),
					m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-publiclb", gomockinternal.DiffEq(newDefaultPublicAPIServerLB())).Return(nil))
			},
		},
		{
			name:          "create internal apiserver LB",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder) {
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name:            "my-private-lb",
						Role:            infrav1.APIServerRole,
						Type:            infrav1.Internal,
						SKU:             infrav1.SKUStandard,
						SubnetName:      "my-cp-subnet",
						BackendPoolName: "my-private-lb-backendPool",
						FrontendIPConfigs: []infrav1.FrontendIP{
							{
								Name:             "my-private-lb-frontEnd",
								PrivateIPAddress: "10.0.0.10",
							},
						},
						APIServerPort: 6443,
					},
				})
				setupDefaultLBExpectations(s)
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{
					ResourceGroup: "my-rg",
					Name:          "my-vnet",
				})
				gomock.InOrder(
					m.Get(gomockinternal.AContext(), "my-rg", "my-private-lb").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found")),
					m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-private-lb", gomockinternal.DiffEq(newDefaultInternalAPIServerLB())).Return(nil))
			},
		},
		{
			name:          "create node outbound LB",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder) {
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name:            "my-cluster",
						Role:            infrav1.NodeOutboundRole,
						Type:            infrav1.Public,
						SKU:             infrav1.SKUStandard,
						BackendPoolName: "my-cluster-outboundBackendPool",
						FrontendIPConfigs: []infrav1.FrontendIP{
							{
								Name: "my-cluster-frontEnd",
								PublicIP: &infrav1.PublicIPSpec{
									Name: "outbound-publicip",
								},
							},
						},
					},
				})
				setupDefaultLBExpectations(s)
				gomock.InOrder(
					m.Get(gomockinternal.AContext(), "my-rg", "my-cluster").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found")),
					m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-cluster", gomockinternal.DiffEq(newDefaultNodeOutboundLB())).Return(nil))
			},
		},
		{
			name:          "create multiple LBs",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder) {
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name:          "my-lb",
						SubnetName:    "my-subnet",
						APIServerPort: 6443,
						Role:          infrav1.APIServerRole,
						Type:          infrav1.Internal,
					},
					{
						Name:          "my-lb-2",
						APIServerPort: 6443,
						Role:          infrav1.APIServerRole,
						Type:          infrav1.Public,
					},
					{
						Name: "my-lb-3",
						Role: infrav1.NodeOutboundRole,
						Type: infrav1.Public,
					},
				})
				setupDefaultLBExpectations(s)
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{
					ResourceGroup: "my-rg",
					Name:          "my-vnet",
				})
				m.Get(gomockinternal.AContext(), "my-rg", "my-lb").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-lb", gomock.AssignableToTypeOf(network.LoadBalancer{}))
				m.Get(gomockinternal.AContext(), "my-rg", "my-lb-2").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-lb-2", gomock.AssignableToTypeOf(network.LoadBalancer{}))
				m.Get(gomockinternal.AContext(), "my-rg", "my-lb-3").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-lb-3", gomock.AssignableToTypeOf(network.LoadBalancer{}))
			},
		},
		{
			name:          "LB already exists and needs no updates",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder) {
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name:            "my-publiclb",
						Role:            infrav1.APIServerRole,
						Type:            infrav1.Public,
						SKU:             infrav1.SKUStandard,
						SubnetName:      "my-cp-subnet",
						BackendPoolName: "my-publiclb-backendPool",
						FrontendIPConfigs: []infrav1.FrontendIP{
							{
								Name: "my-publiclb-frontEnd",
								PublicIP: &infrav1.PublicIPSpec{
									Name:    "my-publicip",
									DNSName: "my-cluster.12345.mydomain.com",
								},
							},
						},
						APIServerPort: 6443,
					},
				})
				setupDefaultLBExpectations(s)
				existingLB := newDefaultPublicAPIServerLB()
				existingLB.ID = to.StringPtr("azure/my-publiclb")
				m.Get(gomockinternal.AContext(), "my-rg", "my-publiclb").Return(existingLB, nil)
			},
		},
		{
			name:          "LB already exists and is missing properties",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder, mVnet *mock_virtualnetworks.MockClientMockRecorder) {
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name:            "my-publiclb",
						Role:            infrav1.APIServerRole,
						Type:            infrav1.Public,
						SKU:             infrav1.SKUStandard,
						SubnetName:      "my-cp-subnet",
						BackendPoolName: "my-publiclb-backendPool",
						FrontendIPConfigs: []infrav1.FrontendIP{
							{
								Name: "my-publiclb-frontEnd",
								PublicIP: &infrav1.PublicIPSpec{
									Name:    "my-publicip",
									DNSName: "my-cluster.12345.mydomain.com",
								},
							},
						},
						APIServerPort: 6443,
					},
				})
				setupDefaultLBExpectations(s)
				existingLB := newDefaultPublicAPIServerLB()
				existingLB.ID = to.StringPtr("azure/my-publiclb")
				existingLB.BackendAddressPools = &[]network.BackendAddressPool{}
				m.Get(gomockinternal.AContext(), "my-rg", "my-publiclb").Return(existingLB, nil)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-publiclb", gomockinternal.DiffEq(newDefaultPublicAPIServerLB())).Return(nil)
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

			scopeMock := mock_loadbalancers.NewMockLBScope(mockCtrl)
			clientMock := mock_loadbalancers.NewMockClient(mockCtrl)
			vnetMock := mock_virtualnetworks.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), vnetMock.EXPECT())

			s := &Service{
				Scope:                 scopeMock,
				Client:                clientMock,
				virtualNetworksClient: vnetMock,
			}
			err := s.Reconcile(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteLoadBalancer(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder)
	}{
		{
			name:          "successfully delete an existing load balancer",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name: "my-internallb",
					},
					{
						Name: "my-publiclb",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-internallb")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-publiclb")
			},
		},
		{
			name:          "load balancer already deleted",
			expectedError: "",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name: "my-publiclb",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-publiclb").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name:          "load balancer deletion fails",
			expectedError: "failed to delete load balancer my-publiclb in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_loadbalancers.MockLBScopeMockRecorder, m *mock_loadbalancers.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.LBSpecs().Return([]azure.LBSpec{
					{
						Name: "my-publiclb",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-publiclb").
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

			scopeMock := mock_loadbalancers.NewMockLBScope(mockCtrl)
			publicLBMock := mock_loadbalancers.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), publicLBMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				Client: publicLBMock,
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

func newDefaultNodeOutboundLB() network.LoadBalancer {
	return network.LoadBalancer{
		Tags: map[string]*string{
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
			"sigs.k8s.io_cluster-api-provider-azure_role":               to.StringPtr(infrav1.NodeOutboundRole),
		},
		Sku:      &network.LoadBalancerSku{Name: network.LoadBalancerSkuNameStandard},
		Location: to.StringPtr("testlocation"),
		LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
			FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
				{
					Name: to.StringPtr("my-cluster-frontEnd"),
					FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
						PublicIPAddress: &network.PublicIPAddress{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/publicIPAddresses/outbound-publicip")},
					},
				},
			},
			BackendAddressPools: &[]network.BackendAddressPool{
				{
					Name: to.StringPtr("my-cluster-outboundBackendPool"),
				},
			},
			LoadBalancingRules: &[]network.LoadBalancingRule{},
			Probes:             &[]network.Probe{},
			OutboundRules: &[]network.OutboundRule{
				{
					Name: to.StringPtr("OutboundNATAllProtocols"),
					OutboundRulePropertiesFormat: &network.OutboundRulePropertiesFormat{
						FrontendIPConfigurations: &[]network.SubResource{
							{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-cluster/frontendIPConfigurations/my-cluster-frontEnd")},
						},
						BackendAddressPool: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-cluster/backendAddressPools/my-cluster-outboundBackendPool"),
						},
						Protocol:             network.LoadBalancerOutboundRuleProtocolAll,
						IdleTimeoutInMinutes: to.Int32Ptr(4),
					},
				},
			},
		},
	}
}

func newDefaultPublicAPIServerLB() network.LoadBalancer {
	return network.LoadBalancer{
		Tags: map[string]*string{
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
			"sigs.k8s.io_cluster-api-provider-azure_role":               to.StringPtr(infrav1.APIServerRole),
		},
		Sku:      &network.LoadBalancerSku{Name: network.LoadBalancerSkuNameStandard},
		Location: to.StringPtr("testlocation"),
		LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
			FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
				{
					Name: to.StringPtr("my-publiclb-frontEnd"),
					FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
						PublicIPAddress: &network.PublicIPAddress{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/publicIPAddresses/my-publicip")},
					},
				},
			},
			BackendAddressPools: &[]network.BackendAddressPool{
				{
					Name: to.StringPtr("my-publiclb-backendPool"),
				},
			},
			LoadBalancingRules: &[]network.LoadBalancingRule{
				{
					Name: to.StringPtr(lbRuleHTTPS),
					LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
						DisableOutboundSnat:  to.BoolPtr(true),
						Protocol:             network.TransportProtocolTCP,
						FrontendPort:         to.Int32Ptr(6443),
						BackendPort:          to.Int32Ptr(6443),
						IdleTimeoutInMinutes: to.Int32Ptr(4),
						EnableFloatingIP:     to.BoolPtr(false),
						LoadDistribution:     network.LoadDistributionDefault,
						FrontendIPConfiguration: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/frontendIPConfigurations/my-publiclb-frontEnd"),
						},
						BackendAddressPool: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/backendAddressPools/my-publiclb-backendPool"),
						},
						Probe: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/probes/HTTPSProbe"),
						},
					},
				},
			},
			Probes: &[]network.Probe{
				{
					Name: to.StringPtr(httpsProbe),
					ProbePropertiesFormat: &network.ProbePropertiesFormat{
						Protocol:          network.ProbeProtocolHTTPS,
						Port:              to.Int32Ptr(6443),
						RequestPath:       to.StringPtr("/healthz"),
						IntervalInSeconds: to.Int32Ptr(15),
						NumberOfProbes:    to.Int32Ptr(4),
					},
				},
			},
			OutboundRules: &[]network.OutboundRule{
				{
					Name: to.StringPtr("OutboundNATAllProtocols"),
					OutboundRulePropertiesFormat: &network.OutboundRulePropertiesFormat{
						FrontendIPConfigurations: &[]network.SubResource{
							{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/frontendIPConfigurations/my-publiclb-frontEnd")},
						},
						BackendAddressPool: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/backendAddressPools/my-publiclb-backendPool"),
						},
						Protocol:             network.LoadBalancerOutboundRuleProtocolAll,
						IdleTimeoutInMinutes: to.Int32Ptr(4),
					},
				},
			},
		},
	}
}

func newDefaultInternalAPIServerLB() network.LoadBalancer {
	return network.LoadBalancer{
		Tags: map[string]*string{
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
			"sigs.k8s.io_cluster-api-provider-azure_role":               to.StringPtr(infrav1.APIServerRole),
		},
		Sku:      &network.LoadBalancerSku{Name: network.LoadBalancerSkuNameStandard},
		Location: to.StringPtr("testlocation"),
		LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
			FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
				{
					Name: to.StringPtr("my-private-lb-frontEnd"),
					FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
						PrivateIPAllocationMethod: network.Static,
						Subnet: &network.Subnet{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-cp-subnet"),
						},
						PrivateIPAddress: to.StringPtr("10.0.0.10"),
					},
				},
			},
			BackendAddressPools: &[]network.BackendAddressPool{
				{
					Name: to.StringPtr("my-private-lb-backendPool"),
				},
			},
			LoadBalancingRules: &[]network.LoadBalancingRule{
				{
					Name: to.StringPtr(lbRuleHTTPS),
					LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
						DisableOutboundSnat:  to.BoolPtr(true),
						Protocol:             network.TransportProtocolTCP,
						FrontendPort:         to.Int32Ptr(6443),
						BackendPort:          to.Int32Ptr(6443),
						IdleTimeoutInMinutes: to.Int32Ptr(4),
						EnableFloatingIP:     to.BoolPtr(false),
						LoadDistribution:     network.LoadDistributionDefault,
						FrontendIPConfiguration: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-private-lb/frontendIPConfigurations/my-private-lb-frontEnd"),
						},
						BackendAddressPool: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-private-lb/backendAddressPools/my-private-lb-backendPool"),
						},
						Probe: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-private-lb/probes/HTTPSProbe"),
						},
					},
				},
			},
			OutboundRules: &[]network.OutboundRule{},
			Probes: &[]network.Probe{
				{
					Name: to.StringPtr(httpsProbe),
					ProbePropertiesFormat: &network.ProbePropertiesFormat{
						Protocol:          network.ProbeProtocolHTTPS,
						Port:              to.Int32Ptr(6443),
						RequestPath:       to.StringPtr("/healthz"),
						IntervalInSeconds: to.Int32Ptr(15),
						NumberOfProbes:    to.Int32Ptr(4),
					},
				},
			},
		},
	}
}

func setupDefaultLBExpectations(s *mock_loadbalancers.MockLBScopeMockRecorder) {
	s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
	s.SubscriptionID().AnyTimes().Return("123")
	s.ResourceGroup().AnyTimes().Return("my-rg")
	s.Location().AnyTimes().Return("testlocation")
	s.ClusterName().AnyTimes().Return("my-cluster")
	s.AdditionalTags().AnyTimes().Return(infrav1.Tags{})
}

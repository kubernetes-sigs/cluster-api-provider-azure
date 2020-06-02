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

package publicloadbalancers

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips/mock_publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicloadbalancers/mock_publicloadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"

	network "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	expectedInvalidSpec = "invalid public loadbalancer specification"
	subscriptionID      = "123"
)

func init() {
	clusterv1.AddToScheme(scheme.Scheme)
}

func TestInvalidPublicLBSpec(t *testing.T) {
	g := NewWithT(t)

	mockCtrl := gomock.NewController(t)
	publicLBMock := mock_publicloadbalancers.NewMockClient(mockCtrl)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
	}

	client := fake.NewFakeClient(cluster)

	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		AzureClients: scope.AzureClients{
			Authorizer: autorest.NullAuthorizer{},
		},
		Client:  client,
		Cluster: cluster,
		AzureCluster: &infrav1.AzureCluster{
			Spec: infrav1.AzureClusterSpec{
				Location: "test-location",
				ResourceGroup:  "my-rg",
				SubscriptionID: subscriptionID,
				NetworkSpec: infrav1.NetworkSpec{
					Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
				},
			},
		},
	})
	g.Expect(err).NotTo(HaveOccurred())

	s := &Service{
		Scope:  clusterScope,
		Client: publicLBMock,
	}

	// Wrong Spec
	wrongSpec := &network.PublicIPAddress{}

	err = s.Reconcile(context.TODO(), &wrongSpec)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(expectedInvalidSpec))

	_, err = s.Get(context.TODO(), &wrongSpec)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(expectedInvalidSpec))

	err = s.Delete(context.TODO(), &wrongSpec)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(expectedInvalidSpec))
}

func TestGetPublicLB(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name          string
		publicLBSpec  Spec
		expectedError string
		expect        func(m *mock_publicloadbalancers.MockClientMockRecorder)
	}{
		{
			name: "get existing public load balancer",
			publicLBSpec: Spec{
				Name: "my-publiclb",
			},
			expectedError: "",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-publiclb").Return(network.LoadBalancer{}, nil)
			},
		},
		{
			name: "public load balancer not found",
			publicLBSpec: Spec{
				Name: "my-publiclb",
			},
			expectedError: "load balancer my-publiclb not found: #: Not found: StatusCode=404",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-publiclb").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "public load balancer retrieval fails",
			publicLBSpec: Spec{
				Name: "my-publiclb",
			},
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-publiclb").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			publicLBMock := mock_publicloadbalancers.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(publicLBMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						ResourceGroup:  "my-rg",
						SubscriptionID: subscriptionID,
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
						},
					},
				},
			})
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:  clusterScope,
				Client: publicLBMock,
			}

			_, err = s.Get(context.TODO(), &tc.publicLBSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestReconcilePublicLoadBalancer(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name          string
		publicLBSpec  Spec
		expectedError string
		expect        func(m *mock_publicloadbalancers.MockClientMockRecorder,
			publicIP *mock_publicips.MockClientMockRecorder)
	}{
		{
			name: "public IP does not exist",
			publicLBSpec: Spec{
				Name:         "my-publiclb",
				PublicIPName: "my-publicip",
			},
			expectedError: "public ip my-publicip not found in RG my-rg: #: Not found: StatusCode=404",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder,
				publicIP *mock_publicips.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-publiclb", gomock.AssignableToTypeOf(network.LoadBalancer{}))
				publicIP.Get(context.TODO(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "public IP retrieval fails",
			publicLBSpec: Spec{
				Name:         "my-publiclb",
				PublicIPName: "my-publicip",
			},
			expectedError: "failed to look for existing public IP: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder,
				publicIP *mock_publicips.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-publiclb", gomock.AssignableToTypeOf(network.LoadBalancer{}))
				publicIP.Get(context.TODO(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name: "successfully create a public LB",
			publicLBSpec: Spec{
				Name:         "my-publiclb",
				PublicIPName: "my-publicip",
			},
			expectedError: "",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder,
				publicIP *mock_publicips.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-publiclb", gomock.AssignableToTypeOf(network.LoadBalancer{})).Return(nil)
				publicIP.Get(context.TODO(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, nil)
			},
		},
		{
			name: "fail to create a public LB",
			publicLBSpec: Spec{
				Name:         "my-publiclb",
				PublicIPName: "my-publicip",
			},
			expectedError: "cannot create public load balancer: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder,
				publicIP *mock_publicips.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-publiclb", gomock.AssignableToTypeOf(network.LoadBalancer{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
				publicIP.Get(context.TODO(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, nil)
			},
		},
		{
			name: "create apiserver LB",
			publicLBSpec: Spec{
				Name:         "my-publiclb",
				PublicIPName: "my-publicip",
				Role:         infrav1.APIServerRole,
			},
			expectedError: "",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder,
				publicIP *mock_publicips.MockClientMockRecorder) {
				gomock.InOrder(
					publicIP.Get(context.TODO(), "my-rg", "my-publicip").Return(network.PublicIPAddress{Name: to.StringPtr("my-publicip")}, nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-publiclb", matchers.DiffEq(network.LoadBalancer{
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": to.StringPtr("owned"),
							"sigs.k8s.io_cluster-api-provider-azure_role":                 to.StringPtr(infrav1.APIServerRole),
						},
						Sku: &network.LoadBalancerSku{Name: network.LoadBalancerSkuNameStandard},
						Location: to.StringPtr("test-location"),
						LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
							FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
								{
									Name: to.StringPtr("my-publiclb-frontEnd"),
									FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
										PrivateIPAllocationMethod: network.Dynamic,
										PublicIPAddress:           &network.PublicIPAddress{Name: to.StringPtr("my-publicip")},
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
									Name: to.StringPtr("LBRuleHTTPS"),
									LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
										DisableOutboundSnat:  to.BoolPtr(true),
										Protocol:             network.TransportProtocolTCP,
										FrontendPort:         to.Int32Ptr(6443),
										BackendPort:          to.Int32Ptr(6443),
										IdleTimeoutInMinutes: to.Int32Ptr(4),
										EnableFloatingIP:     to.BoolPtr(false),
										LoadDistribution:     network.LoadDistributionDefault,
										FrontendIPConfiguration: &network.SubResource{
											ID: to.StringPtr("//subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/frontendIPConfigurations/my-publiclb-frontEnd"),
										},
										BackendAddressPool: &network.SubResource{
											ID: to.StringPtr("//subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/backendAddressPools/my-publiclb-backendPool"),
										},
										Probe: &network.SubResource{
											ID: to.StringPtr("//subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/probes/tcpHTTPSProbe"),
										},
									},
								},
							},
							Probes: &[]network.Probe{
								{
									Name: to.StringPtr("tcpHTTPSProbe"),
									ProbePropertiesFormat: &network.ProbePropertiesFormat{
										Protocol:          network.ProbeProtocolTCP,
										Port:              to.Int32Ptr(6443),
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
											{ID: to.StringPtr("//subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/frontendIPConfigurations/my-publiclb-frontEnd")},
										},
										BackendAddressPool: &network.SubResource{
											ID: to.StringPtr("//subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/backendAddressPools/my-publiclb-backendPool"),
										},
										Protocol:             network.LoadBalancerOutboundRuleProtocolAll,
										IdleTimeoutInMinutes: to.Int32Ptr(4),
									},
								},
							},
						},
					})).Return(nil))
			},
		},
		{
			name: "create node outbound LB",
			publicLBSpec: Spec{
				Name:         "cluster-name",
				PublicIPName: "outbound-publicip",
				Role:         infrav1.NodeOutboundRole,
			},
			expectedError: "",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder,
				publicIP *mock_publicips.MockClientMockRecorder) {
				gomock.InOrder(
					publicIP.Get(context.TODO(), "my-rg", "outbound-publicip").Return(network.PublicIPAddress{Name: to.StringPtr("outbound-publicip")}, nil),
					m.CreateOrUpdate(context.TODO(), "my-rg", "cluster-name", matchers.DiffEq(network.LoadBalancer{
						Tags: map[string]*string{
							"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": to.StringPtr("owned"),
							"sigs.k8s.io_cluster-api-provider-azure_role":                 to.StringPtr(infrav1.NodeOutboundRole),
						},
						Sku: &network.LoadBalancerSku{Name: network.LoadBalancerSkuNameStandard},
						Location: to.StringPtr("test-location"),
						LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
							FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
								{
									Name: to.StringPtr("cluster-name-frontEnd"),
									FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
										PrivateIPAllocationMethod: network.Dynamic,
										PublicIPAddress:           &network.PublicIPAddress{Name: to.StringPtr("outbound-publicip")},
									},
								},
							},
							BackendAddressPools: &[]network.BackendAddressPool{
								{
									Name: to.StringPtr("cluster-name-outboundBackendPool"),
								},
							},
							OutboundRules: &[]network.OutboundRule{
								{
									Name: to.StringPtr("OutboundNATAllProtocols"),
									OutboundRulePropertiesFormat: &network.OutboundRulePropertiesFormat{
										FrontendIPConfigurations: &[]network.SubResource{
											{ID: to.StringPtr("//subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/cluster-name/frontendIPConfigurations/cluster-name-frontEnd")},
										},
										BackendAddressPool: &network.SubResource{
											ID: to.StringPtr("//subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/cluster-name/backendAddressPools/cluster-name-outboundBackendPool"),
										},
										Protocol:             network.LoadBalancerOutboundRuleProtocolAll,
										IdleTimeoutInMinutes: to.Int32Ptr(4),
									},
								},
							},
						},
					})).Return(nil))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			publicLBMock := mock_publicloadbalancers.NewMockClient(mockCtrl)
			publicIPsMock := mock_publicips.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(publicLBMock.EXPECT(), publicIPsMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						ResourceGroup:  "my-rg",
						SubscriptionID: subscriptionID,
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
						},
					},
				},
			})
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:           clusterScope,
				Client:          publicLBMock,
				PublicIPsClient: publicIPsMock,
			}

			err = s.Reconcile(context.TODO(), &tc.publicLBSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeletePublicLB(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name          string
		publicLBSpec  Spec
		expectedError string
		expect        func(m *mock_publicloadbalancers.MockClientMockRecorder)
	}{
		{
			name: "successfully delete an existing public load balancer",
			publicLBSpec: Spec{
				Name:         "my-publiclb",
				PublicIPName: "my-publicip",
			},
			expectedError: "",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-publiclb")
			},
		},
		{
			name: "public load balancer already deleted",
			publicLBSpec: Spec{
				Name:         "my-publiclb",
				PublicIPName: "my-publicip",
			},
			expectedError: "",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-publiclb").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "public load balancer deletion fails",
			publicLBSpec: Spec{
				Name:         "my-publiclb",
				PublicIPName: "my-publicip",
			},
			expectedError: "failed to delete public load balancer my-publiclb in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_publicloadbalancers.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-publiclb").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			publicLBMock := mock_publicloadbalancers.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(publicLBMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						ResourceGroup:  "my-rg",
						SubscriptionID: subscriptionID,
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
						},
					},
				},
			})
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:  clusterScope,
				Client: publicLBMock,
			}

			err = s.Delete(context.TODO(), &tc.publicLBSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

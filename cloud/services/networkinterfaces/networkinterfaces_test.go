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
	"net/http"
	"testing"

	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/networkinterfaces/mock_networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips/mock_publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicloadbalancers/mock_publicloadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/subnets/mock_subnets"

	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"

	network "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const expectedInvalidSpec = "invalid network interface specification"

func init() {
	clusterv1.AddToScheme(scheme.Scheme)
}

func TestInvalidNetworkInterface(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	netInterfaceMock := mock_networkinterfaces.NewMockClient(mockCtrl)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
	}

	client := fake.NewFakeClient(cluster)

	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		AzureClients: scope.AzureClients{
			SubscriptionID: "123",
			Authorizer:     autorest.NullAuthorizer{},
		},
		Client:  client,
		Cluster: cluster,
		AzureCluster: &infrav1.AzureCluster{
			Spec: infrav1.AzureClusterSpec{
				Location: "test-location",
				ResourceGroup: "my-rg",
				NetworkSpec: infrav1.NetworkSpec{
					Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Failed to create test context: %v", err)
	}

	s := &Service{
		Scope:  clusterScope,
		Client: netInterfaceMock,
	}

	// Wrong Spec
	wrongSpec := &network.PublicIPAddress{}

	err = s.Reconcile(context.TODO(), &wrongSpec)
	if err == nil {
		t.Fatalf("it should fail")
	}
	if err.Error() != expectedInvalidSpec {
		t.Fatalf("got an unexpected error: %v", err)
	}

	_, err = s.Get(context.TODO(), &wrongSpec)
	if err == nil {
		t.Fatalf("it should fail")
	}
	if err.Error() != expectedInvalidSpec {
		t.Fatalf("got an unexpected error: %v", err)
	}

	err = s.Delete(context.TODO(), &wrongSpec)
	if err == nil {
		t.Fatalf("it should fail")
	}
	if err.Error() != expectedInvalidSpec {
		t.Fatalf("got an unexpected error: %v", err)
	}
}

func TestGetNetworkInterface(t *testing.T) {
	testcases := []struct {
		name             string
		netInterfaceSpec Spec
		expectedError    string
		expect           func(m *mock_networkinterfaces.MockClientMockRecorder)
	}{
		{
			name: "get existing network interface",
			netInterfaceSpec: Spec{
				Name: "my-net-interface",
			},
			expectedError: "",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-net-interface").Return(network.Interface{}, nil)
			},
		},
		{
			name: "network interface not found",
			netInterfaceSpec: Spec{
				Name: "my-net-interface",
			},
			expectedError: "network interface my-net-interface not found: #: Not found: StatusCode=404",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-net-interface").Return(network.Interface{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "network interface retrieval fails",
			netInterfaceSpec: Spec{
				Name: "my-net-interface",
			},
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-net-interface").Return(network.Interface{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			netInterfaceMock := mock_networkinterfaces.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(netInterfaceMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					SubscriptionID: "123",
					Authorizer:     autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						ResourceGroup: "my-rg",
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
						},
					},
				},
			})
			if err != nil {
				t.Fatalf("Failed to create test context: %v", err)
			}

			s := &Service{
				Scope:  clusterScope,
				Client: netInterfaceMock,
			}

			_, err = s.Get(context.TODO(), &tc.netInterfaceSpec)
			if err != nil {
				if tc.expectedError == "" || err.Error() != tc.expectedError {
					t.Fatalf("got an unexpected error: %v", err)
				}
			} else {
				if tc.expectedError != "" {
					t.Fatalf("expected an error: %v", tc.expectedError)
				}
			}
		})
	}
}

func TestReconcileNetworkInterface(t *testing.T) {
	testcases := []struct {
		name             string
		netInterfaceSpec Spec
		expectedError    string
		expect           func(m *mock_networkinterfaces.MockClientMockRecorder,
			mSubnet *mock_subnets.MockClientMockRecorder,
			mLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
			mPublicIP *mock_publicips.MockClientMockRecorder)
	}{
		{
			name: "subnet fails to return",
			netInterfaceSpec: Spec{
				Name:       "my-net-interface",
				VnetName:   "my-vnet",
				SubnetName: "my-subnet",
			},
			expectedError: "failed to get subNets: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", gomock.AssignableToTypeOf(network.Interface{}))
				mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name: "network interface retrieval fails",
			netInterfaceSpec: Spec{
				Name:       "my-net-interface",
				VnetName:   "my-vnet",
				SubnetName: "my-subnet",
			},
			expectedError: "failed to create network interface my-net-interface in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", gomock.AssignableToTypeOf(network.Interface{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
				mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil)
			},
		},
		{
			name: "network interface with Static IP successfully created",
			netInterfaceSpec: Spec{
				Name:            "my-net-interface",
				VnetName:        "my-vnet",
				SubnetName:      "my-subnet",
				StaticIPAddress: "1.2.3.4",
			},
			expectedError: "",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", gomock.AssignableToTypeOf(network.Interface{}))
				mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil)
			},
		},
		{
			name: "network interface with Dynamic IP successfully created",
			netInterfaceSpec: Spec{
				Name:       "my-net-interface",
				VnetName:   "my-vnet",
				SubnetName: "my-subnet",
			},
			expectedError: "",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", gomock.AssignableToTypeOf(network.Interface{}))
				mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil)
			},
		},
		{
			name: "network interface with Public LB successfully created",
			netInterfaceSpec: Spec{
				Name:                   "my-net-interface",
				VnetName:               "my-vnet",
				SubnetName:             "my-subnet",
				PublicLoadBalancerName: "my-publiclb",
				NatRule:                0,
			},
			expectedError: "",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", gomock.AssignableToTypeOf(network.Interface{}))
				mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil)
				mLoadBalancer.Get(context.TODO(), "my-rg", "my-publiclb").Return(network.LoadBalancer{
					ID: pointer.StringPtr("my-publiclb-id"),
					LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
						BackendAddressPools: &[]network.BackendAddressPool{
							{
								ID: pointer.StringPtr("my-backend-pool"),
							},
						},
						InboundNatRules: &[]network.InboundNatRule{
							{
								Name: pointer.StringPtr("my-nat-rule"),
								ID:   pointer.StringPtr("some-natrules-id"),
							},
						},
					}}, nil)
			},
		},
		{
			name: "network interface with Public LB fail to get LB",
			netInterfaceSpec: Spec{
				Name:                   "my-net-interface",
				VnetName:               "my-vnet",
				SubnetName:             "my-subnet",
				PublicLoadBalancerName: "my-publiclb",
			},
			expectedError: "failed to get publicLB: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", gomock.AssignableToTypeOf(network.Interface{}))
				mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil)
				mLoadBalancer.Get(context.TODO(), "my-rg", "my-publiclb").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name: "network interface with Internal LB successfully created",
			netInterfaceSpec: Spec{
				Name:                     "my-net-interface",
				VnetName:                 "my-vnet",
				SubnetName:               "my-subnet",
				InternalLoadBalancerName: "my-internal-lb",
				NatRule:                  0,
			},
			expectedError: "",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", gomock.AssignableToTypeOf(network.Interface{}))
				mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil)
				mLoadBalancer.Get(context.TODO(), "my-rg", "my-internal-lb").Return(network.LoadBalancer{
					ID: pointer.StringPtr("my-internal-lb-id"),
					LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
						BackendAddressPools: &[]network.BackendAddressPool{
							{
								ID: pointer.StringPtr("my-backend-pool"),
							},
						},
						InboundNatRules: &[]network.InboundNatRule{
							{
								Name: pointer.StringPtr("my-nat-rule"),
								ID:   pointer.StringPtr("some-natrules-id"),
							},
						},
					}}, nil)
			},
		},
		{
			name: "network interface with Internal LB fail to get Internal LB",
			netInterfaceSpec: Spec{
				Name:                     "my-net-interface",
				VnetName:                 "my-vnet",
				SubnetName:               "my-subnet",
				InternalLoadBalancerName: "my-internal-lb",
			},
			expectedError: "failed to get internalLB: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", gomock.AssignableToTypeOf(network.Interface{}))
				mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil)
				mLoadBalancer.Get(context.TODO(), "my-rg", "my-internal-lb").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name: "network interface with Public IP successfully created",
			netInterfaceSpec: Spec{
				Name:         "my-net-interface",
				VnetName:     "my-vnet",
				SubnetName:   "my-subnet",
				PublicIPName: "my-public-ip",
			},
			expectedError: "",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", gomock.AssignableToTypeOf(network.Interface{}))
				mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil)
				mPublicIP.Get(context.TODO(), "my-rg", "my-public-ip").Return(network.PublicIPAddress{}, nil)
			},
		},
		{
			name: "network interface with Public IP fail to get Public IP",
			netInterfaceSpec: Spec{
				Name:         "my-net-interface",
				VnetName:     "my-vnet",
				SubnetName:   "my-subnet",
				PublicIPName: "my-public-ip",
			},
			expectedError: "failed to get publicIP: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mLoadBalancer *mock_publicloadbalancers.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-net-interface", gomock.AssignableToTypeOf(network.Interface{}))
				mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil)
				mPublicIP.Get(context.TODO(), "my-rg", "my-public-ip").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			netInterfaceMock := mock_networkinterfaces.NewMockClient(mockCtrl)
			subnetMock := mock_subnets.NewMockClient(mockCtrl)
			loadBalancerMock := mock_publicloadbalancers.NewMockClient(mockCtrl)
			publicIPsMock := mock_publicips.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(netInterfaceMock.EXPECT(), subnetMock.EXPECT(),
				loadBalancerMock.EXPECT(), publicIPsMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					SubscriptionID: "123",
					Authorizer:     autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						ResourceGroup: "my-rg",
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
							Subnets: []*infrav1.SubnetSpec{{
								Name: "my-subnet",
								Role: infrav1.SubnetNode,
							}},
						},
					},
				},
			})
			if err != nil {
				t.Fatalf("Failed to create test context: %v", err)
			}

			s := &Service{
				Scope:               clusterScope,
				Client:              netInterfaceMock,
				SubnetsClient:       subnetMock,
				LoadBalancersClient: loadBalancerMock,
				PublicIPsClient:     publicIPsMock,
			}

			if err := s.Reconcile(context.TODO(), &tc.netInterfaceSpec); err != nil {
				if tc.expectedError == "" || err.Error() != tc.expectedError {
					t.Fatalf("got an unexpected error: %v", err)
				}
			} else {
				if tc.expectedError != "" {
					t.Fatalf("expected an error: %v", tc.expectedError)

				}
			}
		})
	}
}

func TestDeleteNetworkInterface(t *testing.T) {
	testcases := []struct {
		name             string
		netInterfaceSpec Spec
		expectedError    string
		expect           func(m *mock_networkinterfaces.MockClientMockRecorder)
	}{
		{
			name: "successfully delete an existing network interface",
			netInterfaceSpec: Spec{
				Name: "my-net-interface",
			},
			expectedError: "",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-net-interface")
			},
		},
		{
			name: "network interface already deleted",
			netInterfaceSpec: Spec{
				Name: "my-net-interface",
			},
			expectedError: "",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-net-interface").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "network interface deletion fails",
			netInterfaceSpec: Spec{
				Name: "my-net-interface",
			},
			expectedError: "failed to delete network interface my-net-interface in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_networkinterfaces.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-net-interface").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			netInterfaceMock := mock_networkinterfaces.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(netInterfaceMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					SubscriptionID: "123",
					Authorizer:     autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						ResourceGroup: "my-rg",
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
						},
					},
				},
			})
			if err != nil {
				t.Fatalf("Failed to create test context: %v", err)
			}

			s := &Service{
				Scope:  clusterScope,
				Client: netInterfaceMock,
			}

			if err := s.Delete(context.TODO(), &tc.netInterfaceSpec); err != nil {
				if tc.expectedError == "" || err.Error() != tc.expectedError {
					t.Fatalf("got an unexpected error: %v", err)
				}
			} else {
				if tc.expectedError != "" {
					t.Fatalf("expected an error: %v", tc.expectedError)
				}
			}
		})
	}
}

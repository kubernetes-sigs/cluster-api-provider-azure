/*
Copyright 2019 The Kubernetes Authors.

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

package internalloadbalancers

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/go-autorest/autorest/to"

	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/internalloadbalancers/mock_internalloadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/subnets/mock_subnets"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualnetworks/mock_virtualnetworks"

	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"

	network "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	clusterv1.AddToScheme(scheme.Scheme)
}

func TestReconcileInternalLoadBalancer(t *testing.T) {
	testcases := []struct {
		name           string
		internalLBSpec Spec
		expectedError  string
		expect         func(m *mock_internalloadbalancers.MockClientMockRecorder,
			mVnet *mock_virtualnetworks.MockClientMockRecorder,
			mSubnet *mock_subnets.MockClientMockRecorder)
	}{
		{
			name: "internal load balancer does not exist",
			internalLBSpec: Spec{
				Name:       "my-lb",
				SubnetCidr: "10.0.0.0/16",
				SubnetName: "my-subnet",
				VnetName:   "my-vnet",
				IPAddress:  "10.0.0.10",
			},
			expectedError: "",
			expect: func(m *mock_internalloadbalancers.MockClientMockRecorder,
				mVnet *mock_virtualnetworks.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-lb").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				mVnet.CheckIPAddressAvailability(context.TODO(), "my-rg", "my-vnet", "10.0.0.10").Return(network.IPAddressAvailabilityResult{Available: to.BoolPtr(true)}, nil)
				mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil)
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-lb", gomock.AssignableToTypeOf(network.LoadBalancer{}))
			},
		},
		{
			name: "internal load balancer retrieval fails",
			internalLBSpec: Spec{
				Name:       "my-lb",
				SubnetCidr: "10.0.0.0/16",
				SubnetName: "my-subnet",
				VnetName:   "my-vnet",
				IPAddress:  "10.0.0.10",
			},
			expectedError: "failed to look for existing internal LB: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_internalloadbalancers.MockClientMockRecorder,
				mVnet *mock_virtualnetworks.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-lb").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name: "internal load balancer exists",
			internalLBSpec: Spec{
				Name:       "my-lb",
				SubnetCidr: "10.0.0.0/16",
				SubnetName: "my-subnet",
				VnetName:   "my-vnet",
				IPAddress:  "10.0.0.10",
			},
			expectedError: "",
			expect: func(m *mock_internalloadbalancers.MockClientMockRecorder,
				mVnet *mock_virtualnetworks.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-lb").Return(network.LoadBalancer{
					LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
						FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
							{
								FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{},
							},
						}}}, nil)
				mVnet.CheckIPAddressAvailability(context.TODO(), "my-rg", "my-vnet", "10.0.0.10").Return(network.IPAddressAvailabilityResult{Available: to.BoolPtr(true)}, nil)
				mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil)
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-lb", gomock.AssignableToTypeOf(network.LoadBalancer{}))
			},
		},
		{
			name: "internal load balancer does not exist and IP is not available",
			internalLBSpec: Spec{
				Name:       "my-lb",
				SubnetCidr: "10.0.0.0/16",
				SubnetName: "my-subnet",
				VnetName:   "my-vnet",
				IPAddress:  "10.0.0.10",
			},
			expectedError: "IP 10.0.0.10 is not available in vnet my-vnet and there were no other available IPs found",
			expect: func(m *mock_internalloadbalancers.MockClientMockRecorder,
				mVnet *mock_virtualnetworks.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-lb").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				mVnet.CheckIPAddressAvailability(context.TODO(), "my-rg", "my-vnet", "10.0.0.10").Return(network.IPAddressAvailabilityResult{Available: to.BoolPtr(false)}, nil)
			},
		},
		{
			name: "internal load balancer does not exist and subnet does not exist",
			internalLBSpec: Spec{
				Name:       "my-lb",
				SubnetCidr: "10.0.0.0/16",
				SubnetName: "my-subnet",
				VnetName:   "my-vnet",
				IPAddress:  "10.0.0.10",
			},
			expectedError: "failed to get subnet: #: Not found: StatusCode=404",
			expect: func(m *mock_internalloadbalancers.MockClientMockRecorder,
				mVnet *mock_virtualnetworks.MockClientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder) {
				m.Get(context.TODO(), "my-rg", "my-lb").Return(network.LoadBalancer{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				mVnet.CheckIPAddressAvailability(context.TODO(), "my-rg", "my-vnet", "10.0.0.10").Return(network.IPAddressAvailabilityResult{Available: to.BoolPtr(true)}, nil)
				mSubnet.Get(context.TODO(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			internalLBMock := mock_internalloadbalancers.NewMockClient(mockCtrl)
			subnetMock := mock_subnets.NewMockClient(mockCtrl)
			vnetMock := mock_virtualnetworks.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(internalLBMock.EXPECT(), vnetMock.EXPECT(), subnetMock.EXPECT())

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
				Scope:                 clusterScope,
				Client:                internalLBMock,
				SubnetsClient:         subnetMock,
				VirtualNetworksClient: vnetMock,
			}

			if err := s.Reconcile(context.TODO(), &tc.internalLBSpec); err != nil {
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

func TestDeleteInternalLB(t *testing.T) {
	testcases := []struct {
		name           string
		internalLBSpec Spec
		expectedError  string
		expect         func(m *mock_internalloadbalancers.MockClientMockRecorder)
	}{
		{
			name: "internal load balancer exists",
			internalLBSpec: Spec{
				Name:       "my-lb",
				SubnetCidr: "10.0.0.0/16",
				SubnetName: "my-subnet",
				VnetName:   "my-vnet",
				IPAddress:  "10.0.0.10",
			},
			expectedError: "",
			expect: func(m *mock_internalloadbalancers.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-lb")
			},
		},
		{
			name: "internal load balancer already deleted",
			internalLBSpec: Spec{
				Name:       "my-lb",
				SubnetCidr: "10.0.0.0/16",
				SubnetName: "my-subnet",
				VnetName:   "my-vnet",
				IPAddress:  "10.0.0.10",
			},
			expectedError: "",
			expect: func(m *mock_internalloadbalancers.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-lb").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "internal load balancer deletion fails",
			internalLBSpec: Spec{
				Name:       "my-lb",
				SubnetCidr: "10.0.0.0/16",
				SubnetName: "my-subnet",
				VnetName:   "my-vnet",
				IPAddress:  "10.0.0.10",
			},
			expectedError: "failed to delete internal load balancer my-lb in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_internalloadbalancers.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-lb").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			internalLBMock := mock_internalloadbalancers.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(internalLBMock.EXPECT())

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
				Scope:  clusterScope,
				Client: internalLBMock,
			}

			if err := s.Delete(context.TODO(), &tc.internalLBSpec); err != nil {
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

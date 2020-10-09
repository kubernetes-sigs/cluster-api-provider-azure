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

package subnets

import (
	"context"
	"net/http"
	"testing"

	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"

	. "github.com/onsi/gomega"
	"k8s.io/klog/klogr"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/subnets/mock_subnets"

	"github.com/golang/mock/gomock"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
)

func TestReconcileSubnets(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_subnets.MockSubnetScopeMockRecorder, m *mock_subnets.MockClientMockRecorder)
	}{
		{
			name:          "subnet does not exist",
			expectedError: "",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, m *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.SubnetSpecs().Return([]azure.SubnetSpec{
					{
						Name:              "my-subnet",
						CIDRs:             []string{"10.0.0.0/16"},
						VNetName:          "my-vnet",
						RouteTableName:    "my-subnet_route_table",
						SecurityGroupName: "my-sg",
						Role:              infrav1.SubnetNode,
					},
				})
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{Name: "my-vnet"})
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.IsIPv6Enabled().AnyTimes().Return(false)
				s.IsVnetManaged().Return(true)
				m.Get(gomockinternal.AContext(), "", "my-vnet", "my-subnet").
					Return(network.Subnet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdate(gomockinternal.AContext(), "", "my-vnet", "my-subnet", gomockinternal.DiffEq(network.Subnet{
					SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
						AddressPrefix:        to.StringPtr("10.0.0.0/16"),
						NetworkSecurityGroup: &network.SecurityGroup{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/networkSecurityGroups/my-sg")},
						RouteTable:           &network.RouteTable{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/routeTables/my-subnet_route_table")},
					},
				}))
			},
		},
		{
			name:          "subnet ipv6 does not exist",
			expectedError: "",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, m *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.SubnetSpecs().Return([]azure.SubnetSpec{
					{
						Name:              "my-ipv6-subnet",
						CIDRs:             []string{"10.0.0.0/16", "2001:1234:5678:9abd::/64"},
						VNetName:          "my-vnet",
						RouteTableName:    "my-subnet_route_table",
						SecurityGroupName: "my-sg",
						Role:              infrav1.SubnetNode,
					},
				})
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"})
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.SubscriptionID().AnyTimes().Return("123")
				s.IsVnetManaged().Return(true)
				m.Get(gomockinternal.AContext(), "my-rg", "my-vnet", "my-ipv6-subnet").
					Return(network.Subnet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vnet", "my-ipv6-subnet", gomockinternal.DiffEq(network.Subnet{
					SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
						AddressPrefixes: &[]string{
							"10.0.0.0/16",
							"2001:1234:5678:9abd::/64",
						},
						RouteTable:           &network.RouteTable{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/routeTables/my-subnet_route_table")},
						NetworkSecurityGroup: &network.SecurityGroup{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/networkSecurityGroups/my-sg")},
					},
				}))
			},
		},
		{
			name:          "fail to create subnet",
			expectedError: "failed to create subnet my-subnet in resource group : #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, m *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.SubnetSpecs().Return([]azure.SubnetSpec{
					{
						Name:              "my-subnet",
						CIDRs:             []string{"10.0.0.0/16"},
						VNetName:          "my-vnet",
						RouteTableName:    "my-subnet_route_table",
						SecurityGroupName: "my-sg",
						Role:              infrav1.SubnetNode,
					},
				})
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{Name: "my-vnet"})
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.IsIPv6Enabled().AnyTimes().Return(false)
				s.IsVnetManaged().Return(true)
				m.Get(gomockinternal.AContext(), "", "my-vnet", "my-subnet").
					Return(network.Subnet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.CreateOrUpdate(gomockinternal.AContext(), "", "my-vnet", "my-subnet", gomock.AssignableToTypeOf(network.Subnet{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "fail to get existing subnet",
			expectedError: "failed to get subnet my-subnet: failed to fetch subnet named my-vnet in vnet my-subnet: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, m *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.SubnetSpecs().Return([]azure.SubnetSpec{
					{
						Name:              "my-subnet",
						CIDRs:             []string{"10.0.0.0/16"},
						VNetName:          "my-vnet",
						RouteTableName:    "my-subnet_route_table",
						SecurityGroupName: "my-sg",
						Role:              infrav1.SubnetNode,
					},
				})
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{Name: "my-vnet"})
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(gomockinternal.AContext(), "", "my-vnet", "my-subnet").
					Return(network.Subnet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "vnet was provided but subnet is missing",
			expectedError: "vnet was provided but subnet my-subnet is missing",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, m *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.SubnetSpecs().Return([]azure.SubnetSpec{
					{
						Name:              "my-subnet",
						CIDRs:             []string{"10.0.0.0/16"},
						VNetName:          "custom-vnet",
						RouteTableName:    "my-subnet_route_table",
						SecurityGroupName: "my-sg",
						Role:              infrav1.SubnetNode,
					},
				})
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{ResourceGroup: "custom-vnet-rg", Name: "custom-vnet", ID: "id1", Tags: infrav1.Tags{
					"Name": "vnet-exists",
					"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "not",
					"sigs.k8s.io_cluster-api-provider-azure_role":                 "dd",
				}})
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("custom-vnet-rg")
				s.IsVnetManaged().Return(false)
				m.Get(gomockinternal.AContext(), "custom-vnet-rg", "custom-vnet", "my-subnet").
					Return(network.Subnet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name:          "vnet was provided and subnet exists",
			expectedError: "",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, m *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.SubnetSpecs().AnyTimes().Return([]azure.SubnetSpec{
					{
						Name:              "my-subnet",
						CIDRs:             []string{"10.0.0.0/16"},
						VNetName:          "my-vnet",
						RouteTableName:    "my-subnet_route_table",
						SecurityGroupName: "my-sg",
						Role:              infrav1.SubnetNode,
					},
					{
						Name:              "my-subnet-1",
						CIDRs:             []string{"10.2.0.0/16"},
						VNetName:          "my-vnet",
						RouteTableName:    "my-subnet_route_table",
						SecurityGroupName: "my-sg-1",
						Role:              infrav1.SubnetControlPlane,
					},
				})
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{Name: "my-vnet"})
				s.NodeSubnet().AnyTimes().Return(&infrav1.SubnetSpec{
					Name: "my-subnet",
					Role: infrav1.SubnetNode,
				})
				s.ControlPlaneSubnet().AnyTimes().Return(&infrav1.SubnetSpec{
					Name: "my-subnet-1",
					Role: infrav1.SubnetControlPlane,
				})
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(gomockinternal.AContext(), "", "my-vnet", "my-subnet").
					Return(network.Subnet{
						ID:   to.StringPtr("subnet-id"),
						Name: to.StringPtr("my-subnet"),
						SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
							AddressPrefix: to.StringPtr("10.0.0.0/16"),
							RouteTable: &network.RouteTable{
								ID:   to.StringPtr("rt-id"),
								Name: to.StringPtr("my-subnet_route_table"),
							},
							NetworkSecurityGroup: &network.SecurityGroup{
								ID:   to.StringPtr("sg-id"),
								Name: to.StringPtr("my-sg"),
							},
						},
					}, nil)
				m.Get(gomockinternal.AContext(), "", "my-vnet", "my-subnet-1").
					Return(network.Subnet{
						ID:   to.StringPtr("subnet-id-1"),
						Name: to.StringPtr("my-subnet-1"),
						SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
							AddressPrefix: to.StringPtr("10.2.0.0/16"),
							RouteTable: &network.RouteTable{
								ID:   to.StringPtr("rt-id"),
								Name: to.StringPtr("my-subnet_route_table"),
							},
							NetworkSecurityGroup: &network.SecurityGroup{
								ID:   to.StringPtr("sg-id"),
								Name: to.StringPtr("my-sg-1"),
							},
						},
					}, nil)
			},
		},
		{
			name:          "vnet for ipv6 is provided",
			expectedError: "",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, m *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.SubnetSpecs().AnyTimes().Return([]azure.SubnetSpec{
					{
						Name:              "my-ipv6-subnet",
						CIDRs:             []string{"10.0.0.0/16", "2001:1234:5678:9abd::/64"},
						VNetName:          "my-vnet",
						RouteTableName:    "my-subnet_route_table",
						SecurityGroupName: "my-sg",
						Role:              infrav1.SubnetNode,
					},
					{
						Name:              "my-ipv6-subnet-cp",
						CIDRs:             []string{"10.2.0.0/16", "2001:1234:5678:9abc::/64"},
						VNetName:          "my-vnet",
						RouteTableName:    "my-subnet_route_table",
						SecurityGroupName: "my-sg-1",
						Role:              infrav1.SubnetControlPlane,
					},
				})
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{Name: "my-vnet"})
				s.NodeSubnet().AnyTimes().Return(&infrav1.SubnetSpec{
					Name: "my-subnet",
					Role: infrav1.SubnetNode,
				})
				s.ControlPlaneSubnet().AnyTimes().Return(&infrav1.SubnetSpec{
					Name: "my-subnet-1",
					Role: infrav1.SubnetControlPlane,
				})
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.IsIPv6Enabled().AnyTimes().Return(true)
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(gomockinternal.AContext(), "", "my-vnet", "my-ipv6-subnet").
					Return(network.Subnet{
						ID:   to.StringPtr("subnet-id"),
						Name: to.StringPtr("my-ipv6-subnet"),
						SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
							AddressPrefixes: &[]string{
								"10.0.0.0/16",
								"2001:1234:5678:9abd::/64",
							},
							RouteTable: &network.RouteTable{
								ID:   to.StringPtr("rt-id"),
								Name: to.StringPtr("my-subnet_route_table"),
							},
							NetworkSecurityGroup: &network.SecurityGroup{
								ID:   to.StringPtr("sg-id"),
								Name: to.StringPtr("my-sg"),
							},
						},
					}, nil)
				m.Get(gomockinternal.AContext(), "", "my-vnet", "my-ipv6-subnet-cp").
					Return(network.Subnet{
						ID:   to.StringPtr("subnet-id-1"),
						Name: to.StringPtr("my-ipv6-subnet-cp"),
						SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
							AddressPrefixes: &[]string{
								"10.2.0.0/16",
								"2001:1234:5678:9abc::/64",
							},
							RouteTable: &network.RouteTable{
								ID:   to.StringPtr("rt-id"),
								Name: to.StringPtr("my-subnet_route_table"),
							},
							NetworkSecurityGroup: &network.SecurityGroup{
								ID:   to.StringPtr("sg-id"),
								Name: to.StringPtr("my-sg-1"),
							},
						},
					}, nil)
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
			scopeMock := mock_subnets.NewMockSubnetScope(mockCtrl)
			clientMock := mock_subnets.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				Client: clientMock,
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

func TestDeleteSubnets(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_subnets.MockSubnetScopeMockRecorder, m *mock_subnets.MockClientMockRecorder)
	}{
		{
			name:          "subnet deleted successfully",
			expectedError: "",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, m *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.SubnetSpecs().Return([]azure.SubnetSpec{
					{
						Name:              "my-subnet",
						CIDRs:             []string{"10.0.0.0/16"},
						VNetName:          "my-vnet",
						RouteTableName:    "my-subnet_route_table",
						SecurityGroupName: "my-sg",
						Role:              infrav1.SubnetNode,
					},
					{
						Name:              "my-subnet-1",
						CIDRs:             []string{"10.1.0.0/16"},
						VNetName:          "my-vnet",
						RouteTableName:    "my-subnet_route_table",
						SecurityGroupName: "my-sg",
						Role:              infrav1.SubnetControlPlane,
					},
				})
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{Name: "my-vnet"})
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "", "my-vnet", "my-subnet")
				m.Delete(gomockinternal.AContext(), "", "my-vnet", "my-subnet-1")
			},
		},
		{
			name:          "subnet already deleted",
			expectedError: "",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, m *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.SubnetSpecs().Return([]azure.SubnetSpec{
					{
						Name:              "my-subnet",
						CIDRs:             []string{"10.0.0.0/16"},
						VNetName:          "my-vnet",
						RouteTableName:    "my-subnet_route_table",
						SecurityGroupName: "my-sg",
						Role:              infrav1.SubnetNode,
					},
				})
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{Name: "my-vnet"})
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "", "my-vnet", "my-subnet").
					Return(autorest.NewErrorWithResponse("", "my-vnet", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name:          "node subnet already deleted and controlplane subnet deleted successfully",
			expectedError: "",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, m *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.SubnetSpecs().Return([]azure.SubnetSpec{
					{
						Name:              "my-subnet",
						CIDRs:             []string{"10.0.0.0/16"},
						VNetName:          "my-vnet",
						RouteTableName:    "my-subnet_route_table",
						SecurityGroupName: "my-sg",
						Role:              infrav1.SubnetNode,
					},
					{
						Name:              "my-subnet-1",
						CIDRs:             []string{"10.1.0.0/16"},
						VNetName:          "my-vnet",
						RouteTableName:    "my-subnet_route_table",
						SecurityGroupName: "my-sg",
						Role:              infrav1.SubnetControlPlane,
					},
				})
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{Name: "my-vnet"})
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "", "my-vnet", "my-subnet").
					Return(autorest.NewErrorWithResponse("", "my-vnet", &http.Response{StatusCode: 404}, "Not found"))
				m.Delete(gomockinternal.AContext(), "", "my-vnet", "my-subnet-1")
			},
		},
		{
			name:          "skip delete if vnet is managed",
			expectedError: "",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, m *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.SubnetSpecs().Return([]azure.SubnetSpec{
					{
						Name:              "my-subnet",
						CIDRs:             []string{"10.0.0.0/16"},
						VNetName:          "custom-vnet",
						RouteTableName:    "my-subnet_route_table",
						SecurityGroupName: "my-sg",
						Role:              infrav1.SubnetNode,
					},
				})
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{ResourceGroup: "custom-vnet-rg", Name: "custom-vnet", ID: "id1"})
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.ResourceGroup().AnyTimes().Return("my-rg")
			},
		},
		{
			name:          "fail delete subnet",
			expectedError: "failed to delete subnet my-subnet in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_subnets.MockSubnetScopeMockRecorder, m *mock_subnets.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.SubnetSpecs().Return([]azure.SubnetSpec{
					{
						Name:              "my-subnet",
						CIDRs:             []string{"10.0.0.0/16"},
						VNetName:          "my-vnet",
						RouteTableName:    "my-subnet_route_table",
						SecurityGroupName: "my-sg",
						Role:              infrav1.SubnetNode,
					},
				})
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"})
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-vnet", "my-subnet").Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
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
			scopeMock := mock_subnets.NewMockSubnetScope(mockCtrl)
			clientMock := mock_subnets.NewMockClient(mockCtrl)

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

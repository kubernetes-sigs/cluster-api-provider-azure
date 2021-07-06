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

package natgateways

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/natgateways/mock_natgateways"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

func init() {
	_ = clusterv1.AddToScheme(scheme.Scheme)
}

func TestReconcileNatGateways(t *testing.T) {
	testcases := []struct {
		name          string
		tags          infrav1.Tags
		expectedError string
		expect        func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockclientMockRecorder)
	}{
		{
			name: "nat gateways in custom vnet mode",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "shared",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockclientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					ID:   "1234",
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
			},
		},
		{
			name: "nat gateway create successfully",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockclientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.NatGatewaySpecs().Return([]azure.NatGatewaySpec{
					{
						Name: "my-node-natgateway",
						Subnet: infrav1.SubnetSpec{
							Name: "node-subnet",
							Role: infrav1.SubnetNode,
						},
					},
				})

				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(gomockinternal.AContext(), "my-rg", "my-node-natgateway").Return(network.NatGateway{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found")).Times(1)
				s.SetNodeNatGateway(gomock.Any()).Times(1)
				s.Location().Return("westus")
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-node-natgateway", gomock.AssignableToTypeOf(network.NatGateway{})).Times(1)
			},
		},
		{
			name: "update nat gateway if already exists",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockclientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.NatGatewaySpecs().Return([]azure.NatGatewaySpec{
					{
						Name: "my-node-natgateway",
						Subnet: infrav1.SubnetSpec{
							Name: "node-subnet",
							Role: infrav1.SubnetNode,
						},
					},
				})

				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().Return("my-rg").AnyTimes()
				m.Get(gomockinternal.AContext(), "my-rg", "my-node-natgateway").Times(1).Return(network.NatGateway{
					Name: to.StringPtr("my-node-natgateway"),
					ID:   to.StringPtr("1"),
					NatGatewayPropertiesFormat: &network.NatGatewayPropertiesFormat{PublicIPAddresses: &[]network.SubResource{
						{ID: to.StringPtr("1")},
					}},
				}, nil)
				s.SetNodeNatGateway(gomock.Any()).Times(2)
				s.Location().Return("westus")
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-node-natgateway", gomock.AssignableToTypeOf(network.NatGateway{})).Times(1)
			},
		},
		{
			name: "fail when getting existing nat gateway",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "failed to get nat gateway my-node-natgateway in my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockclientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.NatGatewaySpecs().Return([]azure.NatGatewaySpec{
					{
						Name: "my-node-natgateway",
						Subnet: infrav1.SubnetSpec{
							Name: "node-subnet",
							Role: infrav1.SubnetNode,
						},
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(gomockinternal.AContext(), "my-rg", "my-node-natgateway").Return(network.NatGateway{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
				m.CreateOrUpdate(gomockinternal.AContext(), gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(network.NatGateway{})).Times(0)
				s.NodeNatGateway().Times(0)
			},
		},
		{
			name: "fail to create a nat gateway",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "failed to create nat gateway my-node-natgateway in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockclientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.NatGatewaySpecs().Return([]azure.NatGatewaySpec{
					{
						Name: "my-node-natgateway",
						Subnet: infrav1.SubnetSpec{
							Name: "node-subnet",
							Role: infrav1.SubnetNode,
						},
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(gomockinternal.AContext(), "my-rg", "my-node-natgateway").Return(network.NatGateway{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.Location().Return("westus")
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-node-natgateway", gomock.AssignableToTypeOf(network.NatGateway{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
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
			scopeMock := mock_natgateways.NewMockNatGatewayScope(mockCtrl)
			clientMock := mock_natgateways.NewMockclient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				client: clientMock,
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

func TestDeleteNatGateway(t *testing.T) {
	testcases := []struct {
		name          string
		tags          infrav1.Tags
		expectedError string
		expect        func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockclientMockRecorder)
	}{
		{
			name: "nat gateways in custom vnet mode",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "shared",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockclientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					ID:   "1234",
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
			},
		},
		{
			name: "nat gateway deleted successfully",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockclientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.NatGatewaySpecs().Return([]azure.NatGatewaySpec{
					{
						Name: "my-node-natgateway",
						Subnet: infrav1.SubnetSpec{
							Name: "node-subnet",
							Role: infrav1.SubnetNode,
						},
					},
				})
				s.ResourceGroup().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-node-natgateway")
			},
		},
		{
			name: "nat gateway already deleted",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockclientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.NatGatewaySpecs().Return([]azure.NatGatewaySpec{
					{
						Name: "my-node-natgateway",
						Subnet: infrav1.SubnetSpec{
							Name: "node-subnet",
							Role: infrav1.SubnetNode,
						},
					},
				})
				s.ResourceGroup().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-node-natgateway").Return(autorest.NewErrorWithResponse("", "", &http.Response{
					StatusCode: 404,
				}, "Not Found"))
			},
		},
		{
			name: "nat gateway deletion fails",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "failed to delete nat gateway my-node-natgateway in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockclientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.NatGatewaySpecs().Return([]azure.NatGatewaySpec{
					{
						Name: "my-node-natgateway",
						Subnet: infrav1.SubnetSpec{
							Name: "node-subnet",
							Role: infrav1.SubnetNode,
						},
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-node-natgateway").Return(autorest.NewErrorWithResponse("", "", &http.Response{
					StatusCode: 500,
				}, "Internal Server Error"))
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
			scopeMock := mock_natgateways.NewMockNatGatewayScope(mockCtrl)
			clientMock := mock_natgateways.NewMockclient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				client: clientMock,
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

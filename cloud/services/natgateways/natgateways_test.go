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

	"github.com/Azure/go-autorest/autorest/to"

	. "github.com/onsi/gomega"

	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"

	network "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/natgateways/mock_natgateways"
)

func init() {
	clusterv1.AddToScheme(scheme.Scheme)
}

func TestReconcileNatGateways(t *testing.T) {
	testcases := []struct {
		name          string
		tags          infrav1.Tags
		expectedError string
		expect        func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockClientMockRecorder)
	}{
		{
			name: "nat gateways in custom vnet mode",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "shared",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockClientMockRecorder) {
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
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockClientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.NatGatewaySpecs().Return([]azure.NatGatewaySpec{{
					Name: "my-natgateway",
				}})
				s.NatGateway().AnyTimes().Return(&infrav1.NatGateway{Name: "my-natgateway"})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(context.TODO(), "my-rg", "my-natgateway").Return(network.NatGateway{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.Location().Return("westus")
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-natgateway", gomock.AssignableToTypeOf(network.NatGateway{}))
			},
		},
		{
			name: "do not create nat gateway if already exists",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockClientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.NatGatewaySpecs().Return([]azure.NatGatewaySpec{{
					Name: "my-natgateway",
				}})
				s.NatGateway().AnyTimes().Return(&infrav1.NatGateway{Name: "my-natgateway"})
				s.ResourceGroup().Return("my-rg")
				m.Get(context.TODO(), "my-rg", "my-natgateway").Return(network.NatGateway{
					Name: to.StringPtr("my-natgateway"),
					ID:   to.StringPtr("1"),
				}, nil)
				s.NodeSubnet().AnyTimes().Return(&infrav1.SubnetSpec{})
				s.ControlPlaneSubnet().AnyTimes().Return(&infrav1.SubnetSpec{})
				m.CreateOrUpdate(context.TODO(), gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(network.NatGateway{})).Times(0)
			},
		},
		{
			name: "fail when getting existing nat gateways",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "failed to get nat gateway my-natgateway in my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockClientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.NatGatewaySpecs().Return([]azure.NatGatewaySpec{{
					Name: "my-natgateway",
				}})
				s.NatGateway().AnyTimes().Return(&infrav1.NatGateway{Name: "my-natgateway"})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(context.TODO(), "my-rg", "my-natgateway").Return(network.NatGateway{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
				m.CreateOrUpdate(context.TODO(), gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(network.NatGateway{})).Times(0)
			},
		},
		{
			name: "fail to create a nat gateway",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "failed to create nat gateway my-natgateway in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockClientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.NatGatewaySpecs().Return([]azure.NatGatewaySpec{{
					Name: "my-natgateway",
				}})
				s.NatGateway().AnyTimes().Return(&infrav1.NatGateway{Name: "my-natgateway"})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(context.TODO(), "my-rg", "my-natgateway").Return(network.NatGateway{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.Location().Return("westus")
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-natgateway", gomock.AssignableToTypeOf(network.NatGateway{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
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
			clientMock := mock_natgateways.NewMockClient(mockCtrl)

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

func TestDeleteNatGateway(t *testing.T) {
	testcases := []struct {
		name          string
		tags          infrav1.Tags
		expectedError string
		expect        func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockClientMockRecorder)
	}{
		{
			name: "nat gateways in custom vnet mode",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "shared",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockClientMockRecorder) {
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
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockClientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.NatGatewaySpecs().Return([]azure.NatGatewaySpec{{
					Name: "my-natgateway",
				}})
				s.NatGateway().AnyTimes().Return(&infrav1.NatGateway{Name: "my-natgateway"})
				s.ResourceGroup().Return("my-rg")
				m.Delete(context.TODO(), "my-rg", "my-natgateway")
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
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockClientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.NatGatewaySpecs().Return([]azure.NatGatewaySpec{{
					Name: "my-natgateway",
				}})
				s.NatGateway().AnyTimes().Return(&infrav1.NatGateway{Name: "my-natgateway"})
				s.ResourceGroup().Return("my-rg")
				m.Delete(context.TODO(), "my-rg", "my-natgateway").Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not Found"))
			},
		},
		{
			name: "nat gateway deletion fails",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "failed to delete nat gateway my-natgateway in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_natgateways.MockNatGatewayScopeMockRecorder, m *mock_natgateways.MockClientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.NatGatewaySpecs().Return([]azure.NatGatewaySpec{{
					Name: "my-natgateway",
				}})
				s.NatGateway().AnyTimes().Return(&infrav1.NatGateway{Name: "my-natgateway"})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(context.TODO(), "my-rg", "my-natgateway").Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
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
			clientMock := mock_natgateways.NewMockClient(mockCtrl)

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

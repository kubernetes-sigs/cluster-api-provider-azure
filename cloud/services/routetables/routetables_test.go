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

package routetables

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/go-autorest/autorest/to"

	. "github.com/onsi/gomega"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/routetables/mock_routetables"

	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"

	network "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

func init() {
	clusterv1.AddToScheme(scheme.Scheme)
}

func TestReconcileRouteTables(t *testing.T) {
	testcases := []struct {
		name          string
		tags          infrav1.Tags
		expectedError string
		expect        func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockClientMockRecorder)
	}{
		{
			name: "route tables in custom vnet mode",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "shared",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockClientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					ID:   "1234",
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
			},
		},
		{
			name: "route table create successfully",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockClientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.RouteTableSpecs().Return([]azure.RouteTableSpec{{
					Name: "my-routetable",
				}})
				s.RouteTable().AnyTimes().Return(&infrav1.RouteTable{Name: "my-routetable"})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(context.TODO(), "my-rg", "my-routetable").Return(network.RouteTable{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.Location().Return("westus")
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-routetable", gomock.AssignableToTypeOf(network.RouteTable{}))
			},
		},
		{
			name: "do not create route table if already exists",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockClientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.RouteTableSpecs().Return([]azure.RouteTableSpec{{
					Name: "my-routetable",
				}})
				s.RouteTable().AnyTimes().Return(&infrav1.RouteTable{Name: "my-routetable"})
				s.ResourceGroup().Return("my-rg")
				m.Get(context.TODO(), "my-rg", "my-routetable").Return(network.RouteTable{
					Name: to.StringPtr("my-routetable"),
					ID:   to.StringPtr("1"),
				}, nil)
				s.NodeSubnet().AnyTimes().Return(&infrav1.SubnetSpec{})
				s.ControlPlaneSubnet().AnyTimes().Return(&infrav1.SubnetSpec{})
				m.CreateOrUpdate(context.TODO(), gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(network.RouteTable{})).Times(0)
			},
		},
		{
			name: "fail when getting existing route table",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "failed to get route table my-routetable in my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockClientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.RouteTableSpecs().Return([]azure.RouteTableSpec{{
					Name: "my-routetable",
				}})
				s.RouteTable().AnyTimes().Return(&infrav1.RouteTable{Name: "my-routetable"})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(context.TODO(), "my-rg", "my-routetable").Return(network.RouteTable{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
				m.CreateOrUpdate(context.TODO(), gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(network.RouteTable{})).Times(0)
			},
		},
		{
			name: "fail to create a route table",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "failed to create route table my-routetable in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockClientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.RouteTableSpecs().Return([]azure.RouteTableSpec{{
					Name: "my-routetable",
				}})
				s.RouteTable().AnyTimes().Return(&infrav1.RouteTable{Name: "my-routetable"})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(context.TODO(), "my-rg", "my-routetable").Return(network.RouteTable{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.Location().Return("westus")
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-routetable", gomock.AssignableToTypeOf(network.RouteTable{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_routetables.NewMockRouteTableScope(mockCtrl)
			clientMock := mock_routetables.NewMockClient(mockCtrl)

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

func TestDeleteRouteTable(t *testing.T) {
	testcases := []struct {
		name          string
		tags          infrav1.Tags
		expectedError string
		expect        func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockClientMockRecorder)
	}{
		{
			name: "route tables in custom vnet mode",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "shared",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockClientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					ID:   "1234",
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
			},
		},
		{
			name: "route table deleted successfully",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockClientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.RouteTableSpecs().Return([]azure.RouteTableSpec{{
					Name: "my-routetable",
				}})
				s.RouteTable().AnyTimes().Return(&infrav1.RouteTable{Name: "my-routetable"})
				s.ResourceGroup().Return("my-rg")
				m.Delete(context.TODO(), "my-rg", "my-routetable")
			},
		},
		{
			name: "route table already deleted",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockClientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.RouteTableSpecs().Return([]azure.RouteTableSpec{{
					Name: "my-routetable",
				}})
				s.RouteTable().AnyTimes().Return(&infrav1.RouteTable{Name: "my-routetable"})
				s.ResourceGroup().Return("my-rg")
				m.Delete(context.TODO(), "my-rg", "my-routetable").Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not Found"))
			},
		},
		{
			name: "route table deletion fails",
			tags: infrav1.Tags{
				"Name": "my-vnet",
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned",
				"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
			},
			expectedError: "failed to delete route table my-routetable in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockClientMockRecorder) {
				s.Vnet().Return(&infrav1.VnetSpec{
					Name: "my-vnet",
				})
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName()
				s.RouteTableSpecs().Return([]azure.RouteTableSpec{{
					Name: "my-routetable",
				}})
				s.RouteTable().AnyTimes().Return(&infrav1.RouteTable{Name: "my-routetable"})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(context.TODO(), "my-rg", "my-routetable").Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_routetables.NewMockRouteTableScope(mockCtrl)
			clientMock := mock_routetables.NewMockClient(mockCtrl)

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

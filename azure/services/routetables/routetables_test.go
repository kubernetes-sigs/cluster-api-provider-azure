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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/routetables/mock_routetables"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

func init() {
	_ = clusterv1.AddToScheme(scheme.Scheme)
}

func TestReconcileRouteTables(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockclientMockRecorder)
	}{
		{
			name:          "route tables in custom vnet mode",
			expectedError: "",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockclientMockRecorder) {
				s.IsVnetManaged(gomockinternal.AContext()).Return(false, nil)
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
			},
		},
		{
			name:          "route table create successfully",
			expectedError: "",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockclientMockRecorder) {
				s.IsVnetManaged(gomockinternal.AContext()).Return(true, nil)
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.RouteTableSpecs().Return([]azure.RouteTableSpec{
					{
						Name: "my-cp-routetable",
						Subnet: infrav1.SubnetSpec{
							Name: "control-plane-subnet",
							Role: infrav1.SubnetControlPlane,
						},
					},
					{
						Name: "my-node-routetable",
						Subnet: infrav1.SubnetSpec{
							Name: "node-subnet",
							Role: infrav1.SubnetNode,
						},
					},
				})
				s.ControlPlaneRouteTable().AnyTimes().Return(infrav1.RouteTable{Name: "my-cp-routetable"})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(gomockinternal.AContext(), "my-rg", "my-cp-routetable").Return(network.RouteTable{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.Location().Return("westus")
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-cp-routetable", gomock.AssignableToTypeOf(network.RouteTable{}))
				m.Get(gomockinternal.AContext(), "my-rg", "my-node-routetable").Return(network.RouteTable{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.Location().Return("westus")
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-node-routetable", gomock.AssignableToTypeOf(network.RouteTable{}))
			},
		},
		{
			name:          "do not create route table if already exists",
			expectedError: "",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockclientMockRecorder) {
				s.IsVnetManaged(gomockinternal.AContext()).Return(true, nil)
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.RouteTableSpecs().AnyTimes().Return([]azure.RouteTableSpec{
					{
						Name: "my-cp-routetable",
						Subnet: infrav1.SubnetSpec{
							Name: "control-plane-subnet",
							Role: infrav1.SubnetControlPlane,
						},
					},
					{
						Name: "my-node-routetable",
						Subnet: infrav1.SubnetSpec{
							Name: "node-subnet",
							Role: infrav1.SubnetNode,
						},
					},
				})
				s.ControlPlaneSubnet().AnyTimes().Return(infrav1.SubnetSpec{Name: "control-plane-subnet", Role: infrav1.SubnetControlPlane})
				s.ControlPlaneRouteTable().AnyTimes().Return(infrav1.RouteTable{Name: "my-cp-routetable"})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(gomockinternal.AContext(), "my-rg", "my-cp-routetable").Return(network.RouteTable{
					Name: to.StringPtr("my-cp-routetable"),
					ID:   to.StringPtr("1"),
				}, nil)
				s.SetSubnet(infrav1.SubnetSpec{
					Name: "control-plane-subnet",
					Role: infrav1.SubnetControlPlane,
					RouteTable: infrav1.RouteTable{
						ID:   "1",
						Name: "my-cp-routetable",
					},
				}).Times(1)
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(gomockinternal.AContext(), "my-rg", "my-node-routetable").Return(network.RouteTable{
					Name: to.StringPtr("my-node-routetable"),
					ID:   to.StringPtr("2"),
				}, nil)
				s.SetSubnet(infrav1.SubnetSpec{
					Name: "node-subnet",
					Role: infrav1.SubnetNode,
					RouteTable: infrav1.RouteTable{
						ID:   "2",
						Name: "my-node-routetable",
					},
				}).Times(1)
			},
		},
		{
			name:          "fail when getting existing route table",
			expectedError: "failed to get route table my-cp-routetable in my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockclientMockRecorder) {
				s.IsVnetManaged(gomockinternal.AContext()).Return(true, nil)
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.RouteTableSpecs().Return([]azure.RouteTableSpec{{
					Name: "my-cp-routetable",
					Subnet: infrav1.SubnetSpec{
						Name: "control-plane-subnet",
						Role: infrav1.SubnetControlPlane,
					},
				}})
				s.ControlPlaneSubnet().AnyTimes().Return(infrav1.SubnetSpec{})
				s.ControlPlaneRouteTable().AnyTimes().Return(infrav1.RouteTable{Name: "my-routetable"})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(gomockinternal.AContext(), "my-rg", "my-cp-routetable").Return(network.RouteTable{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
				m.CreateOrUpdate(gomockinternal.AContext(), gomock.Any(), gomock.Any(), gomock.AssignableToTypeOf(network.RouteTable{})).Times(0)
			},
		},
		{
			name:          "fail to create a route table",
			expectedError: "failed to create route table my-cp-routetable in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockclientMockRecorder) {
				s.IsVnetManaged(gomockinternal.AContext()).Return(true, nil)
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.RouteTableSpecs().Return([]azure.RouteTableSpec{{
					Name: "my-cp-routetable",
					Subnet: infrav1.SubnetSpec{
						Name: "control-plane-subnet",
						Role: infrav1.SubnetControlPlane,
					},
				}})
				s.ControlPlaneSubnet().AnyTimes().Return(infrav1.SubnetSpec{})
				s.ControlPlaneRouteTable().AnyTimes().Return(infrav1.RouteTable{Name: "my-cp-routetable"})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(gomockinternal.AContext(), "my-rg", "my-cp-routetable").Return(network.RouteTable{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.Location().Return("westus")
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-cp-routetable", gomock.AssignableToTypeOf(network.RouteTable{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
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
			scopeMock := mock_routetables.NewMockRouteTableScope(mockCtrl)
			clientMock := mock_routetables.NewMockclient(mockCtrl)

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

func TestDeleteRouteTable(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockclientMockRecorder)
	}{
		{
			name:          "route tables in custom vnet mode",
			expectedError: "",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockclientMockRecorder) {
				s.IsVnetManaged(gomockinternal.AContext()).Return(false, nil)
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
			},
		},
		{
			name:          "route table deleted successfully",
			expectedError: "",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockclientMockRecorder) {
				s.IsVnetManaged(gomockinternal.AContext()).Return(true, nil)
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.RouteTableSpecs().Return([]azure.RouteTableSpec{
					{
						Name: "my-cp-routetable",
						Subnet: infrav1.SubnetSpec{
							Name: "control-plane-subnet",
							Role: infrav1.SubnetControlPlane,
						},
					},
					{
						Name: "my-node-routetable",
						Subnet: infrav1.SubnetSpec{
							Name: "node-subnet",
							Role: infrav1.SubnetNode,
						},
					},
				})
				s.ControlPlaneRouteTable().AnyTimes().Return(infrav1.RouteTable{Name: "my-cp-routetable"})
				s.ResourceGroup().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-cp-routetable")
				s.ResourceGroup().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-node-routetable")
			},
		},
		{
			name:          "route table already deleted",
			expectedError: "",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockclientMockRecorder) {
				s.IsVnetManaged(gomockinternal.AContext()).Return(true, nil)
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.RouteTableSpecs().Return([]azure.RouteTableSpec{
					{
						Name: "my-cp-routetable",
						Subnet: infrav1.SubnetSpec{
							Name: "control-plane-subnet",
							Role: infrav1.SubnetControlPlane,
						},
					},
					{
						Name: "my-node-routetable",
						Subnet: infrav1.SubnetSpec{
							Name: "node-subnet",
							Role: infrav1.SubnetNode,
						},
					},
				})
				s.ControlPlaneRouteTable().AnyTimes().Return(infrav1.RouteTable{Name: "my-cp-routetable"})
				s.ResourceGroup().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-cp-routetable").Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not Found"))
				s.ResourceGroup().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-node-routetable").Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not Found"))
			},
		},
		{
			name:          "route table deletion fails",
			expectedError: "failed to delete route table my-cp-routetable in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, m *mock_routetables.MockclientMockRecorder) {
				s.IsVnetManaged(gomockinternal.AContext()).Return(true, nil)
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.RouteTableSpecs().Return([]azure.RouteTableSpec{{
					Name: "my-cp-routetable",
					Subnet: infrav1.SubnetSpec{
						Name: "control-plane-subnet",
						Role: infrav1.SubnetControlPlane,
					},
				}})
				s.ControlPlaneRouteTable().AnyTimes().Return(infrav1.RouteTable{Name: "my-cp-routetable"})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-cp-routetable").Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
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
			scopeMock := mock_routetables.NewMockRouteTableScope(mockCtrl)
			clientMock := mock_routetables.NewMockclient(mockCtrl)

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

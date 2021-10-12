/*
Copyright 2021 The Kubernetes Authors.

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

package vnetpeerings

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/vnetpeerings/mock_vnetpeerings"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

func TestReconcileVnetPeerings(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder)
	}{
		{
			name:          "create one peering",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return([]azure.VnetPeeringSpec{
					{
						PeeringName:         "vnet1-to-vnet2",
						SourceVnetName:      "vnet1",
						SourceResourceGroup: "group1",
						RemoteVnetName:      "vnet2",
						RemoteResourceGroup: "group2",
					},
				})
				p.ClusterName().AnyTimes().Return("fake-cluster")
				p.SubscriptionID().AnyTimes().Return("123")
				m.CreateOrUpdate(gomockinternal.AContext(), "group1", "vnet1", "vnet1-to-vnet2", gomockinternal.DiffEq(network.VirtualNetworkPeering{
					VirtualNetworkPeeringPropertiesFormat: &network.VirtualNetworkPeeringPropertiesFormat{
						RemoteVirtualNetwork: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/group2/providers/Microsoft.Network/virtualNetworks/vnet2"),
						},
					},
				}))
			},
		},
		{
			name:          "create no peerings",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return([]azure.VnetPeeringSpec{})
				p.ClusterName().AnyTimes().Return("fake-cluster")
				p.SubscriptionID().AnyTimes().Return("123")
			},
		},
		{
			name:          "create even number of peerings",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return([]azure.VnetPeeringSpec{
					{
						PeeringName:         "vnet1-to-vnet2",
						SourceVnetName:      "vnet1",
						SourceResourceGroup: "group1",
						RemoteVnetName:      "vnet2",
						RemoteResourceGroup: "group2",
					},
					{
						PeeringName:         "vnet2-to-vnet1",
						SourceVnetName:      "vnet2",
						SourceResourceGroup: "group2",
						RemoteVnetName:      "vnet1",
						RemoteResourceGroup: "group1",
					},
				})
				p.ClusterName().AnyTimes().Return("fake-cluster")
				p.SubscriptionID().AnyTimes().Return("123")
				m.CreateOrUpdate(gomockinternal.AContext(), "group1", "vnet1", "vnet1-to-vnet2", gomockinternal.DiffEq(network.VirtualNetworkPeering{
					VirtualNetworkPeeringPropertiesFormat: &network.VirtualNetworkPeeringPropertiesFormat{
						RemoteVirtualNetwork: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/group2/providers/Microsoft.Network/virtualNetworks/vnet2"),
						},
					},
				}))
				m.CreateOrUpdate(gomockinternal.AContext(), "group2", "vnet2", "vnet2-to-vnet1", gomockinternal.DiffEq(network.VirtualNetworkPeering{
					VirtualNetworkPeeringPropertiesFormat: &network.VirtualNetworkPeeringPropertiesFormat{
						RemoteVirtualNetwork: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/group1/providers/Microsoft.Network/virtualNetworks/vnet1"),
						},
					},
				}))
			},
		},
		{
			name:          "create odd number of peerings",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return([]azure.VnetPeeringSpec{
					{
						PeeringName:         "vnet1-to-vnet2",
						SourceVnetName:      "vnet1",
						SourceResourceGroup: "group1",
						RemoteVnetName:      "vnet2",
						RemoteResourceGroup: "group2",
					},
					{
						PeeringName:         "vnet2-to-vnet1",
						SourceVnetName:      "vnet2",
						SourceResourceGroup: "group2",
						RemoteVnetName:      "vnet1",
						RemoteResourceGroup: "group1",
					},
					{
						PeeringName:         "extra-peering",
						SourceVnetName:      "vnet3",
						SourceResourceGroup: "group3",
						RemoteVnetName:      "vnet4",
						RemoteResourceGroup: "group4",
					},
				})
				p.Vnet().AnyTimes().Return(&infrav1.VnetSpec{Name: "vnet2"})
				p.ClusterName().AnyTimes().Return("fake-cluster")
				p.SubscriptionID().AnyTimes().Return("123")
				m.CreateOrUpdate(gomockinternal.AContext(), "group1", "vnet1", "vnet1-to-vnet2", gomockinternal.DiffEq(network.VirtualNetworkPeering{
					VirtualNetworkPeeringPropertiesFormat: &network.VirtualNetworkPeeringPropertiesFormat{
						RemoteVirtualNetwork: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/group2/providers/Microsoft.Network/virtualNetworks/vnet2"),
						},
					},
				}))
				m.CreateOrUpdate(gomockinternal.AContext(), "group2", "vnet2", "vnet2-to-vnet1", gomockinternal.DiffEq(network.VirtualNetworkPeering{
					VirtualNetworkPeeringPropertiesFormat: &network.VirtualNetworkPeeringPropertiesFormat{
						RemoteVirtualNetwork: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/group1/providers/Microsoft.Network/virtualNetworks/vnet1"),
						},
					},
				}))
				m.CreateOrUpdate(gomockinternal.AContext(), "group3", "vnet3", "extra-peering", gomockinternal.DiffEq(network.VirtualNetworkPeering{
					VirtualNetworkPeeringPropertiesFormat: &network.VirtualNetworkPeeringPropertiesFormat{
						RemoteVirtualNetwork: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/group4/providers/Microsoft.Network/virtualNetworks/vnet4"),
						},
					},
				}))
			},
		},
		{
			name:          "create multiple peerings on one vnet",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return([]azure.VnetPeeringSpec{
					{
						PeeringName:         "vnet1-to-vnet2",
						SourceVnetName:      "vnet1",
						SourceResourceGroup: "group1",
						RemoteVnetName:      "vnet2",
						RemoteResourceGroup: "group2",
					},
					{
						PeeringName:         "vnet2-to-vnet1",
						SourceVnetName:      "vnet2",
						SourceResourceGroup: "group2",
						RemoteVnetName:      "vnet1",
						RemoteResourceGroup: "group1",
					},
					{
						PeeringName:         "vnet1-to-vnet3",
						SourceVnetName:      "vnet1",
						SourceResourceGroup: "group1",
						RemoteVnetName:      "vnet3",
						RemoteResourceGroup: "group3",
					},
					{
						PeeringName:         "vnet3-to-vnet1",
						SourceVnetName:      "vnet3",
						SourceResourceGroup: "group3",
						RemoteVnetName:      "vnet1",
						RemoteResourceGroup: "group1",
					},
				})
				p.ClusterName().AnyTimes().Return("fake-cluster")
				p.SubscriptionID().AnyTimes().Return("123")
				m.CreateOrUpdate(gomockinternal.AContext(), "group1", "vnet1", "vnet1-to-vnet2", gomockinternal.DiffEq(network.VirtualNetworkPeering{
					VirtualNetworkPeeringPropertiesFormat: &network.VirtualNetworkPeeringPropertiesFormat{
						RemoteVirtualNetwork: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/group2/providers/Microsoft.Network/virtualNetworks/vnet2"),
						},
					},
				}))
				m.CreateOrUpdate(gomockinternal.AContext(), "group2", "vnet2", "vnet2-to-vnet1", gomockinternal.DiffEq(network.VirtualNetworkPeering{
					VirtualNetworkPeeringPropertiesFormat: &network.VirtualNetworkPeeringPropertiesFormat{
						RemoteVirtualNetwork: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/group1/providers/Microsoft.Network/virtualNetworks/vnet1"),
						},
					},
				}))
				m.CreateOrUpdate(gomockinternal.AContext(), "group1", "vnet1", "vnet1-to-vnet3", gomockinternal.DiffEq(network.VirtualNetworkPeering{
					VirtualNetworkPeeringPropertiesFormat: &network.VirtualNetworkPeeringPropertiesFormat{
						RemoteVirtualNetwork: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/group3/providers/Microsoft.Network/virtualNetworks/vnet3"),
						},
					},
				}))
				m.CreateOrUpdate(gomockinternal.AContext(), "group3", "vnet3", "vnet3-to-vnet1", gomockinternal.DiffEq(network.VirtualNetworkPeering{
					VirtualNetworkPeeringPropertiesFormat: &network.VirtualNetworkPeeringPropertiesFormat{
						RemoteVirtualNetwork: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/group1/providers/Microsoft.Network/virtualNetworks/vnet1"),
						},
					},
				}))
			},
		},
		{
			name:          "error in creating peering where loop terminates prematurely",
			expectedError: "failed to create peering vnet1-to-vnet3 in resource group group1: #: Internal Server Error: StatusCode=500",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return([]azure.VnetPeeringSpec{
					{
						PeeringName:         "vnet1-to-vnet2",
						SourceVnetName:      "vnet1",
						SourceResourceGroup: "group1",
						RemoteVnetName:      "vnet2",
						RemoteResourceGroup: "group2",
					},
					{
						PeeringName:         "vnet2-to-vnet1",
						SourceVnetName:      "vnet2",
						SourceResourceGroup: "group2",
						RemoteVnetName:      "vnet1",
						RemoteResourceGroup: "group1",
					},
					{
						PeeringName:         "vnet1-to-vnet3",
						SourceVnetName:      "vnet1",
						SourceResourceGroup: "group1",
						RemoteVnetName:      "vnet3",
						RemoteResourceGroup: "group3",
					},
					{
						PeeringName:         "vnet3-to-vnet1",
						SourceVnetName:      "vnet3",
						SourceResourceGroup: "group3",
						RemoteVnetName:      "vnet1",
						RemoteResourceGroup: "group1",
					},
				})
				p.Vnet().AnyTimes().Return(&infrav1.VnetSpec{
					Name:          "vnet1",
					ResourceGroup: "group1",
				})
				p.ClusterName().AnyTimes().Return("fake-cluster")
				p.SubscriptionID().AnyTimes().Return("123")
				m.CreateOrUpdate(gomockinternal.AContext(), "group1", "vnet1", "vnet1-to-vnet2", gomockinternal.DiffEq(network.VirtualNetworkPeering{
					VirtualNetworkPeeringPropertiesFormat: &network.VirtualNetworkPeeringPropertiesFormat{
						RemoteVirtualNetwork: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/group2/providers/Microsoft.Network/virtualNetworks/vnet2"),
						},
					},
				}))
				m.CreateOrUpdate(gomockinternal.AContext(), "group2", "vnet2", "vnet2-to-vnet1", gomockinternal.DiffEq(network.VirtualNetworkPeering{
					VirtualNetworkPeeringPropertiesFormat: &network.VirtualNetworkPeeringPropertiesFormat{
						RemoteVirtualNetwork: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/group1/providers/Microsoft.Network/virtualNetworks/vnet1"),
						},
					},
				}))
				m.CreateOrUpdate(gomockinternal.AContext(), "group1", "vnet1", "vnet1-to-vnet3", gomockinternal.DiffEq(network.VirtualNetworkPeering{
					VirtualNetworkPeeringPropertiesFormat: &network.VirtualNetworkPeeringPropertiesFormat{
						RemoteVirtualNetwork: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/group3/providers/Microsoft.Network/virtualNetworks/vnet3"),
						},
					},
				})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
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
			scopeMock := mock_vnetpeerings.NewMockVnetPeeringScope(mockCtrl)
			clientMock := mock_vnetpeerings.NewMockClient(mockCtrl)

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

func TestDeleteVnetPeerings(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder)
	}{
		{
			name:          "delete one peering",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return([]azure.VnetPeeringSpec{
					{
						PeeringName:         "vnet1-to-vnet2",
						SourceVnetName:      "vnet1",
						SourceResourceGroup: "group1",
						RemoteVnetName:      "vnet2",
						RemoteResourceGroup: "group2",
					},
				})
				p.ClusterName().AnyTimes().Return("fake-cluster")
				p.SubscriptionID().AnyTimes().Return("123")
				m.Delete(gomockinternal.AContext(), "group1", "vnet1", "vnet1-to-vnet2")
			},
		},
		{
			name:          "delete no peerings",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return([]azure.VnetPeeringSpec{})
				p.ClusterName().AnyTimes().Return("fake-cluster")
				p.SubscriptionID().AnyTimes().Return("123")
			},
		},
		{
			name:          "delete even number of peerings",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return([]azure.VnetPeeringSpec{
					{
						PeeringName:         "vnet1-to-vnet2",
						SourceVnetName:      "vnet1",
						SourceResourceGroup: "group1",
						RemoteVnetName:      "vnet2",
						RemoteResourceGroup: "group2",
					},
					{
						PeeringName:         "vnet2-to-vnet1",
						SourceVnetName:      "vnet2",
						SourceResourceGroup: "group2",
						RemoteVnetName:      "vnet1",
						RemoteResourceGroup: "group1",
					},
				})
				p.ClusterName().AnyTimes().Return("fake-cluster")
				p.SubscriptionID().AnyTimes().Return("123")
				m.Delete(gomockinternal.AContext(), "group1", "vnet1", "vnet1-to-vnet2")
				m.Delete(gomockinternal.AContext(), "group2", "vnet2", "vnet2-to-vnet1")
			},
		},
		{
			name:          "delete odd number of peerings",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return([]azure.VnetPeeringSpec{
					{
						PeeringName:         "vnet1-to-vnet2",
						SourceVnetName:      "vnet1",
						SourceResourceGroup: "group1",
						RemoteVnetName:      "vnet2",
						RemoteResourceGroup: "group2",
					},
					{
						PeeringName:         "vnet2-to-vnet1",
						SourceVnetName:      "vnet2",
						SourceResourceGroup: "group2",
						RemoteVnetName:      "vnet1",
						RemoteResourceGroup: "group1",
					},
					{
						PeeringName:         "extra-peering",
						SourceVnetName:      "vnet3",
						SourceResourceGroup: "group3",
						RemoteVnetName:      "vnet4",
						RemoteResourceGroup: "group4",
					},
				})
				p.Vnet().AnyTimes().Return(&infrav1.VnetSpec{Name: "vnet2"})
				p.ClusterName().AnyTimes().Return("fake-cluster")
				p.SubscriptionID().AnyTimes().Return("123")
				m.Delete(gomockinternal.AContext(), "group1", "vnet1", "vnet1-to-vnet2")
				m.Delete(gomockinternal.AContext(), "group2", "vnet2", "vnet2-to-vnet1")
				m.Delete(gomockinternal.AContext(), "group3", "vnet3", "extra-peering")
			},
		},
		{
			name:          "delete multiple peerings on one vnet",
			expectedError: "",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return([]azure.VnetPeeringSpec{
					{
						PeeringName:         "vnet1-to-vnet2",
						SourceVnetName:      "vnet1",
						SourceResourceGroup: "group1",
						RemoteVnetName:      "vnet2",
						RemoteResourceGroup: "group2",
					},
					{
						PeeringName:         "vnet2-to-vnet1",
						SourceVnetName:      "vnet2",
						SourceResourceGroup: "group2",
						RemoteVnetName:      "vnet1",
						RemoteResourceGroup: "group1",
					},
					{
						PeeringName:         "vnet1-to-vnet3",
						SourceVnetName:      "vnet1",
						SourceResourceGroup: "group1",
						RemoteVnetName:      "vnet3",
						RemoteResourceGroup: "group3",
					},
					{
						PeeringName:         "vnet3-to-vnet1",
						SourceVnetName:      "vnet3",
						SourceResourceGroup: "group3",
						RemoteVnetName:      "vnet1",
						RemoteResourceGroup: "group1",
					},
				})
				p.ClusterName().AnyTimes().Return("fake-cluster")
				p.SubscriptionID().AnyTimes().Return("123")
				m.Delete(gomockinternal.AContext(), "group1", "vnet1", "vnet1-to-vnet2")
				m.Delete(gomockinternal.AContext(), "group2", "vnet2", "vnet2-to-vnet1")
				m.Delete(gomockinternal.AContext(), "group1", "vnet1", "vnet1-to-vnet3")
				m.Delete(gomockinternal.AContext(), "group3", "vnet3", "vnet3-to-vnet1")
			},
		},
		{
			name:          "error in deleting peering where loop terminates prematurely",
			expectedError: "failed to delete peering vnet1-to-vnet3 in vnet vnet1 and resource group group1: #: Internal Server Error: StatusCode=500",
			expect: func(p *mock_vnetpeerings.MockVnetPeeringScopeMockRecorder, m *mock_vnetpeerings.MockClientMockRecorder) {
				p.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				p.VnetPeeringSpecs().Return([]azure.VnetPeeringSpec{
					{
						PeeringName:         "vnet1-to-vnet2",
						SourceVnetName:      "vnet1",
						SourceResourceGroup: "group1",
						RemoteVnetName:      "vnet2",
						RemoteResourceGroup: "group2",
					},
					{
						PeeringName:         "vnet2-to-vnet1",
						SourceVnetName:      "vnet2",
						SourceResourceGroup: "group2",
						RemoteVnetName:      "vnet1",
						RemoteResourceGroup: "group1",
					},
					{
						PeeringName:         "vnet1-to-vnet3",
						SourceVnetName:      "vnet1",
						SourceResourceGroup: "group1",
						RemoteVnetName:      "vnet3",
						RemoteResourceGroup: "group3",
					},
					{
						PeeringName:         "vnet3-to-vnet1",
						SourceVnetName:      "vnet3",
						SourceResourceGroup: "group3",
						RemoteVnetName:      "vnet1",
						RemoteResourceGroup: "group1",
					},
				})
				p.Vnet().AnyTimes().Return(&infrav1.VnetSpec{
					Name:          "vnet1",
					ResourceGroup: "group1",
				})
				p.ClusterName().AnyTimes().Return("fake-cluster")
				p.SubscriptionID().AnyTimes().Return("123")
				m.Delete(gomockinternal.AContext(), "group1", "vnet1", "vnet1-to-vnet2")
				m.Delete(gomockinternal.AContext(), "group2", "vnet2", "vnet2-to-vnet1")
				m.Delete(gomockinternal.AContext(), "group1", "vnet1", "vnet1-to-vnet3").Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
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
			scopeMock := mock_vnetpeerings.NewMockVnetPeeringScope(mockCtrl)
			clientMock := mock_vnetpeerings.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				Client: clientMock,
			}

			err := s.Delete(context.TODO())
			if tc.expectedError != "" {
				fmt.Printf("\nExpected error:\t%s\n", tc.expectedError)
				fmt.Printf("\nActual error:\t%s\n", err.Error())
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

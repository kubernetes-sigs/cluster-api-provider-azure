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

package privatedns

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/privatedns/mgmt/2018-09-01/privatedns"
	"github.com/Azure/go-autorest/autorest/to"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/privatedns/mock_privatedns"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"

	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"

	"k8s.io/klog/klogr"
)

func TestReconcilePrivateDNS(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_privatedns.MockScopeMockRecorder, m *mock_privatedns.MockclientMockRecorder)
	}{
		{
			name:          "no private dns",
			expectedError: "",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, m *mock_privatedns.MockclientMockRecorder) {
				s.PrivateDNSSpec().Return(nil)
			},
		},
		{
			name:          "create ipv4 private dns successfully",
			expectedError: "",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, m *mock_privatedns.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PrivateDNSSpec().Return(&azure.PrivateDNSSpec{
					ZoneName:          "my-dns-zone",
					VNetName:          "my-vnet",
					VNetResourceGroup: "vnet-rg",
					LinkName:          "my-link",
					Records: []infrav1.AddressRecord{
						{
							Hostname: "hostname-1",
							IP:       "10.0.0.8",
						},
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.SubscriptionID().Return("123")
				m.CreateOrUpdateZone(gomockinternal.AContext(), "my-rg", "my-dns-zone", privatedns.PrivateZone{Location: to.StringPtr(azure.Global)})
				m.CreateOrUpdateLink(gomockinternal.AContext(), "my-rg", "my-dns-zone", "my-link", privatedns.VirtualNetworkLink{
					VirtualNetworkLinkProperties: &privatedns.VirtualNetworkLinkProperties{
						VirtualNetwork: &privatedns.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/vnet-rg/providers/Microsoft.Network/virtualNetworks/my-vnet"),
						},
						RegistrationEnabled: to.BoolPtr(false),
					},
					Location: to.StringPtr(azure.Global),
				})
				m.CreateOrUpdateRecordSet(gomockinternal.AContext(), "my-rg", "my-dns-zone", privatedns.A, "hostname-1", privatedns.RecordSet{
					RecordSetProperties: &privatedns.RecordSetProperties{
						TTL: to.Int64Ptr(300),
						ARecords: &[]privatedns.ARecord{
							{
								Ipv4Address: to.StringPtr("10.0.0.8"),
							},
						},
					},
				})
			},
		},
		{
			name:          "create ipv6 private dns successfully",
			expectedError: "",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, m *mock_privatedns.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PrivateDNSSpec().Return(&azure.PrivateDNSSpec{
					ZoneName:          "my-dns-zone",
					VNetName:          "my-vnet",
					VNetResourceGroup: "vnet-rg",
					LinkName:          "my-link",
					Records: []infrav1.AddressRecord{
						{
							Hostname: "hostname-2",
							IP:       "2603:1030:805:2::b",
						},
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.SubscriptionID().Return("123")
				m.CreateOrUpdateZone(gomockinternal.AContext(), "my-rg", "my-dns-zone", privatedns.PrivateZone{Location: to.StringPtr(azure.Global)})
				m.CreateOrUpdateLink(gomockinternal.AContext(), "my-rg", "my-dns-zone", "my-link", privatedns.VirtualNetworkLink{
					VirtualNetworkLinkProperties: &privatedns.VirtualNetworkLinkProperties{
						VirtualNetwork: &privatedns.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/vnet-rg/providers/Microsoft.Network/virtualNetworks/my-vnet"),
						},
						RegistrationEnabled: to.BoolPtr(false),
					},
					Location: to.StringPtr(azure.Global),
				})
				m.CreateOrUpdateRecordSet(gomockinternal.AContext(), "my-rg", "my-dns-zone", privatedns.AAAA, "hostname-2", privatedns.RecordSet{
					RecordSetProperties: &privatedns.RecordSetProperties{
						TTL: to.Int64Ptr(300),
						AaaaRecords: &[]privatedns.AaaaRecord{
							{
								Ipv6Address: to.StringPtr("2603:1030:805:2::b"),
							},
						},
					},
				})
			},
		},
		{
			name:          "link creation fails",
			expectedError: "failed to create virtual network link my-link: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, m *mock_privatedns.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PrivateDNSSpec().Return(&azure.PrivateDNSSpec{
					ZoneName:          "my-dns-zone",
					VNetName:          "my-vnet",
					VNetResourceGroup: "vnet-rg",
					LinkName:          "my-link",
					Records: []infrav1.AddressRecord{
						{
							Hostname: "hostname-1",
							IP:       "10.0.0.8",
						},
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.SubscriptionID().Return("123")
				m.CreateOrUpdateZone(gomockinternal.AContext(), "my-rg", "my-dns-zone", privatedns.PrivateZone{Location: to.StringPtr(azure.Global)})
				m.CreateOrUpdateLink(gomockinternal.AContext(), "my-rg", "my-dns-zone", "my-link", privatedns.VirtualNetworkLink{
					VirtualNetworkLinkProperties: &privatedns.VirtualNetworkLinkProperties{
						VirtualNetwork: &privatedns.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/vnet-rg/providers/Microsoft.Network/virtualNetworks/my-vnet"),
						},
						RegistrationEnabled: to.BoolPtr(false),
					},
					Location: to.StringPtr(azure.Global),
				}).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
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
			scopeMock := mock_privatedns.NewMockScope(mockCtrl)
			clientMock := mock_privatedns.NewMockclient(mockCtrl)

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

func TestDeletePrivateDNS(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_privatedns.MockScopeMockRecorder, m *mock_privatedns.MockclientMockRecorder)
	}{
		{
			name:          "no private dns",
			expectedError: "",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, m *mock_privatedns.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PrivateDNSSpec().Return(nil)
			},
		},
		{
			name:          "delete the dns zone",
			expectedError: "",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, m *mock_privatedns.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PrivateDNSSpec().Return(&azure.PrivateDNSSpec{
					ZoneName:          "my-dns-zone",
					VNetName:          "my-vnet",
					VNetResourceGroup: "vnet-rg",
					LinkName:          "my-link",
					Records: []infrav1.AddressRecord{
						{
							Hostname: "hostname-1",
							IP:       "10.0.0.8",
						},
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.DeleteLink(gomockinternal.AContext(), "my-rg", "my-dns-zone", "my-link")
				m.DeleteZone(gomockinternal.AContext(), "my-rg", "my-dns-zone")
			},
		},
		{
			name:          "link already deleted",
			expectedError: "",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, m *mock_privatedns.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PrivateDNSSpec().Return(&azure.PrivateDNSSpec{
					ZoneName:          "my-dns-zone",
					VNetName:          "my-vnet",
					VNetResourceGroup: "vnet-rg",
					LinkName:          "my-link",
					Records: []infrav1.AddressRecord{
						{
							Hostname: "hostname-1",
							IP:       "10.0.0.8",
						},
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.DeleteLink(gomockinternal.AContext(), "my-rg", "my-dns-zone", "my-link").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.DeleteZone(gomockinternal.AContext(), "my-rg", "my-dns-zone")
			},
		},
		{
			name:          "zone already deleted",
			expectedError: "",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, m *mock_privatedns.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PrivateDNSSpec().Return(&azure.PrivateDNSSpec{
					ZoneName:          "my-dns-zone",
					VNetName:          "my-vnet",
					VNetResourceGroup: "vnet-rg",
					LinkName:          "my-link",
					Records: []infrav1.AddressRecord{
						{
							Hostname: "hostname-1",
							IP:       "10.0.0.8",
						},
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.DeleteLink(gomockinternal.AContext(), "my-rg", "my-dns-zone", "my-link").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.DeleteZone(gomockinternal.AContext(), "my-rg", "my-dns-zone").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name:          "error while trying to delete the link",
			expectedError: "failed to delete virtual network link my-vnet with zone my-dns-zone in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, m *mock_privatedns.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PrivateDNSSpec().Return(&azure.PrivateDNSSpec{
					ZoneName:          "my-dns-zone",
					VNetName:          "my-vnet",
					VNetResourceGroup: "vnet-rg",
					LinkName:          "my-link",
					Records: []infrav1.AddressRecord{
						{
							Hostname: "hostname-1",
							IP:       "10.0.0.8",
						},
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.DeleteLink(gomockinternal.AContext(), "my-rg", "my-dns-zone", "my-link").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "error while trying to delete the zone",
			expectedError: "failed to delete private dns zone my-dns-zone in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, m *mock_privatedns.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PrivateDNSSpec().Return(&azure.PrivateDNSSpec{
					ZoneName:          "my-dns-zone",
					VNetName:          "my-vnet",
					VNetResourceGroup: "vnet-rg",
					LinkName:          "my-link",
					Records: []infrav1.AddressRecord{
						{
							Hostname: "hostname-1",
							IP:       "10.0.0.8",
						},
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.DeleteLink(gomockinternal.AContext(), "my-rg", "my-dns-zone", "my-link")
				m.DeleteZone(gomockinternal.AContext(), "my-rg", "my-dns-zone").
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
			scopeMock := mock_privatedns.NewMockScope(mockCtrl)
			clientMock := mock_privatedns.NewMockclient(mockCtrl)

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

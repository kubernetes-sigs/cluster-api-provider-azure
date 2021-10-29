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

package publicips

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
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/publicips/mock_publicips"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

func init() {
	_ = clusterv1.AddToScheme(scheme.Scheme)
}

func TestReconcilePublicIP(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder)
	}{
		{
			name:          "can create public IPs",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.PublicIPSpecs().Return([]azure.PublicIPSpec{
					{
						Name:    "my-publicip",
						DNSName: "fakedns.mydomain.io",
					},
					{
						Name:    "my-publicip-2",
						DNSName: "fakedns2-52959.uksouth.cloudapp.azure.com",
					},
					{
						Name: "my-publicip-3",
					},
					{
						Name:    "my-publicip-ipv6",
						IsIPv6:  true,
						DNSName: "fakename.mydomain.io",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.ClusterName().AnyTimes().Return("my-cluster")
				s.AdditionalTags().AnyTimes().Return(infrav1.Tags{})
				s.Location().AnyTimes().Return("testlocation")
				s.FailureDomains().AnyTimes().Return([]string{"1,2,3"})
				gomock.InOrder(
					m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-publicip", gomockinternal.DiffEq(network.PublicIPAddress{
						Name:     to.StringPtr("my-publicip"),
						Sku:      &network.PublicIPAddressSku{Name: network.PublicIPAddressSkuNameStandard},
						Location: to.StringPtr("testlocation"),
						Tags: map[string]*string{
							"Name": to.StringPtr("my-publicip"),
							"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						},
						PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
							PublicIPAddressVersion:   network.IPVersionIPv4,
							PublicIPAllocationMethod: network.IPAllocationMethodStatic,
							DNSSettings: &network.PublicIPAddressDNSSettings{
								DomainNameLabel: to.StringPtr("fakedns"),
								Fqdn:            to.StringPtr("fakedns.mydomain.io"),
							},
						},
						Zones: to.StringSlicePtr([]string{"1,2,3"}),
					})).Times(1),
					m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-publicip-2", gomockinternal.DiffEq(network.PublicIPAddress{
						Name:     to.StringPtr("my-publicip-2"),
						Sku:      &network.PublicIPAddressSku{Name: network.PublicIPAddressSkuNameStandard},
						Location: to.StringPtr("testlocation"),
						Tags: map[string]*string{
							"Name": to.StringPtr("my-publicip-2"),
							"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						},
						PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
							PublicIPAddressVersion:   network.IPVersionIPv4,
							PublicIPAllocationMethod: network.IPAllocationMethodStatic,
							DNSSettings: &network.PublicIPAddressDNSSettings{
								DomainNameLabel: to.StringPtr("fakedns2-52959"),
								Fqdn:            to.StringPtr("fakedns2-52959.uksouth.cloudapp.azure.com"),
							},
						},
						Zones: to.StringSlicePtr([]string{"1,2,3"}),
					})).Times(1),
					m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-publicip-3", gomockinternal.DiffEq(network.PublicIPAddress{
						Name:     to.StringPtr("my-publicip-3"),
						Sku:      &network.PublicIPAddressSku{Name: network.PublicIPAddressSkuNameStandard},
						Location: to.StringPtr("testlocation"),
						Tags: map[string]*string{
							"Name": to.StringPtr("my-publicip-3"),
							"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						},
						PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
							PublicIPAddressVersion:   network.IPVersionIPv4,
							PublicIPAllocationMethod: network.IPAllocationMethodStatic,
						},
						Zones: to.StringSlicePtr([]string{"1,2,3"}),
					})).Times(1),
					m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-publicip-ipv6", gomockinternal.DiffEq(network.PublicIPAddress{
						Name:     to.StringPtr("my-publicip-ipv6"),
						Sku:      &network.PublicIPAddressSku{Name: network.PublicIPAddressSkuNameStandard},
						Location: to.StringPtr("testlocation"),
						Tags: map[string]*string{
							"Name": to.StringPtr("my-publicip-ipv6"),
							"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						},
						PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
							PublicIPAddressVersion:   network.IPVersionIPv6,
							PublicIPAllocationMethod: network.IPAllocationMethodStatic,
							DNSSettings: &network.PublicIPAddressDNSSettings{
								DomainNameLabel: to.StringPtr("fakename"),
								Fqdn:            to.StringPtr("fakename.mydomain.io"),
							},
						},
						Zones: to.StringSlicePtr([]string{"1,2,3"}),
					})).Times(1),
				)
			},
		},
		{
			name:          "fail to create a public IP",
			expectedError: "cannot create public IP: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.PublicIPSpecs().Return([]azure.PublicIPSpec{
					{
						Name:    "my-publicip",
						DNSName: "fakedns.mydomain.io",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.ClusterName().AnyTimes().Return("my-cluster")
				s.AdditionalTags().AnyTimes().Return(infrav1.Tags{})
				s.Location().AnyTimes().Return("testlocation")
				s.FailureDomains().Times(1)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-publicip", gomock.AssignableToTypeOf(network.PublicIPAddress{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
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
			scopeMock := mock_publicips.NewMockPublicIPScope(mockCtrl)
			clientMock := mock_publicips.NewMockClient(mockCtrl)

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

func TestDeletePublicIP(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder)
	}{
		{
			name:          "successfully delete two existing public IP",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.PublicIPSpecs().Return([]azure.PublicIPSpec{
					{
						Name: "my-publicip",
					},
					{
						Name: "my-publicip-2",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.ClusterName().AnyTimes().Return("my-cluster")
				m.Get(gomockinternal.AContext(), "my-rg", "my-publicip").Return(network.PublicIPAddress{
					Name: to.StringPtr("my-publicip"),
					Tags: map[string]*string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"foo": to.StringPtr("bar"),
					},
				}, nil)
				m.Delete(gomockinternal.AContext(), "my-rg", "my-publicip")
				m.Get(gomockinternal.AContext(), "my-rg", "my-publicip-2").Return(network.PublicIPAddress{
					Name: to.StringPtr("my-publicip-2"),
					Tags: map[string]*string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"foo": to.StringPtr("buzz"),
					},
				}, nil)
				m.Delete(gomockinternal.AContext(), "my-rg", "my-publicip-2")
			},
		},
		{
			name:          "public ip already deleted",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.PublicIPSpecs().Return([]azure.PublicIPSpec{
					{
						Name: "my-publicip",
					},
					{
						Name: "my-publicip-2",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.ClusterName().AnyTimes().Return("my-cluster")
				m.Get(gomockinternal.AContext(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.Get(gomockinternal.AContext(), "my-rg", "my-publicip-2").Return(network.PublicIPAddress{
					Name: to.StringPtr("my-public-ip-2"),
					Tags: map[string]*string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"foo": to.StringPtr("buzz"),
					},
				}, nil)
				m.Delete(gomockinternal.AContext(), "my-rg", "my-publicip-2")
			},
		},
		{
			name:          "public ip deletion fails",
			expectedError: "failed to delete public IP my-publicip in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.PublicIPSpecs().Return([]azure.PublicIPSpec{
					{
						Name: "my-publicip",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.ClusterName().AnyTimes().Return("my-cluster")
				m.Get(gomockinternal.AContext(), "my-rg", "my-publicip").Return(network.PublicIPAddress{
					Name: to.StringPtr("my-publicip"),
					Tags: map[string]*string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"foo": to.StringPtr("bar"),
					},
				}, nil)
				m.Delete(gomockinternal.AContext(), "my-rg", "my-publicip").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "skip unmanaged public ip deletion",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.PublicIPSpecs().Return([]azure.PublicIPSpec{
					{
						Name: "my-publicip",
					},
					{
						Name: "my-publicip-2",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.ClusterName().AnyTimes().Return("my-cluster")
				m.Get(gomockinternal.AContext(), "my-rg", "my-publicip").Return(network.PublicIPAddress{
					Name: to.StringPtr("my-public-ip"),
					Tags: map[string]*string{
						"foo": to.StringPtr("bar"),
					},
				}, nil)
				m.Get(gomockinternal.AContext(), "my-rg", "my-publicip-2").Return(network.PublicIPAddress{
					Name: to.StringPtr("my-publicip-2"),
					Tags: map[string]*string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"foo": to.StringPtr("buzz"),
					},
				}, nil)
				m.Delete(gomockinternal.AContext(), "my-rg", "my-publicip-2")
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
			scopeMock := mock_publicips.NewMockPublicIPScope(mockCtrl)
			clientMock := mock_publicips.NewMockClient(mockCtrl)

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

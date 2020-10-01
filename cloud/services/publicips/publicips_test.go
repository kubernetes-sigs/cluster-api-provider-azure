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

	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips/mock_publicips"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"

	network "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
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
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return([]azure.PublicIPSpec{
					{
						Name:    "my-publicip",
						DNSName: "fakedns",
					},
					{
						Name:    "my-publicip-2",
						DNSName: "fakedns2",
					},
					{
						Name: "my-publicip-3",
					},
					{
						Name:    "my-publicip-ipv6",
						IsIPv6:  true,
						DNSName: "fakename",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("testlocation")
				gomock.InOrder(
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-publicip", gomockinternal.DiffEq(network.PublicIPAddress{
						Name:     to.StringPtr("my-publicip"),
						Sku:      &network.PublicIPAddressSku{Name: network.PublicIPAddressSkuNameStandard},
						Location: to.StringPtr("testlocation"),
						PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
							PublicIPAddressVersion:   network.IPv4,
							PublicIPAllocationMethod: network.Static,
							DNSSettings: &network.PublicIPAddressDNSSettings{
								DomainNameLabel: to.StringPtr("my-publicip"),
								Fqdn:            to.StringPtr("fakedns"),
							},
						},
					})).Times(1),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-publicip-2", gomockinternal.DiffEq(network.PublicIPAddress{
						Name:     to.StringPtr("my-publicip-2"),
						Sku:      &network.PublicIPAddressSku{Name: network.PublicIPAddressSkuNameStandard},
						Location: to.StringPtr("testlocation"),
						PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
							PublicIPAddressVersion:   network.IPv4,
							PublicIPAllocationMethod: network.Static,
							DNSSettings: &network.PublicIPAddressDNSSettings{
								DomainNameLabel: to.StringPtr("my-publicip-2"),
								Fqdn:            to.StringPtr("fakedns2"),
							},
						},
					})).Times(1),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-publicip-3", gomockinternal.DiffEq(network.PublicIPAddress{
						Name:     to.StringPtr("my-publicip-3"),
						Sku:      &network.PublicIPAddressSku{Name: network.PublicIPAddressSkuNameStandard},
						Location: to.StringPtr("testlocation"),
						PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
							PublicIPAddressVersion:   network.IPv4,
							PublicIPAllocationMethod: network.Static,
						},
					})).Times(1),
					m.CreateOrUpdate(context.TODO(), "my-rg", "my-publicip-ipv6", gomockinternal.DiffEq(network.PublicIPAddress{
						Name:     to.StringPtr("my-publicip-ipv6"),
						Sku:      &network.PublicIPAddressSku{Name: network.PublicIPAddressSkuNameStandard},
						Location: to.StringPtr("testlocation"),
						PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
							PublicIPAddressVersion:   network.IPv6,
							PublicIPAllocationMethod: network.Static,
							DNSSettings: &network.PublicIPAddressDNSSettings{
								DomainNameLabel: to.StringPtr("my-publicip-ipv6"),
								Fqdn:            to.StringPtr("fakename"),
							},
						},
					})).Times(1),
				)
			},
		},
		{
			name:          "fail to create a public IP",
			expectedError: "cannot create public IP: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return([]azure.PublicIPSpec{
					{
						Name:    "my-publicip",
						DNSName: "fakedns",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("testlocation")
				m.CreateOrUpdate(context.TODO(), "my-rg", "my-publicip", gomock.AssignableToTypeOf(network.PublicIPAddress{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
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
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return([]azure.PublicIPSpec{
					{
						Name: "my-publicip",
					},
					{
						Name: "my-publicip-2",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(context.TODO(), "my-rg", "my-publicip")
				m.Delete(context.TODO(), "my-rg", "my-publicip-2")
			},
		},
		{
			name:          "public ip already deleted",
			expectedError: "",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return([]azure.PublicIPSpec{
					{
						Name: "my-publicip",
					},
					{
						Name: "my-publicip-2",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(context.TODO(), "my-rg", "my-publicip").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.Delete(context.TODO(), "my-rg", "my-publicip-2")
			},
		},
		{
			name:          "public ip deletion fails",
			expectedError: "failed to delete public IP my-publicip in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_publicips.MockPublicIPScopeMockRecorder, m *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.PublicIPSpecs().Return([]azure.PublicIPSpec{
					{
						Name: "my-publicip",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(context.TODO(), "my-rg", "my-publicip").
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

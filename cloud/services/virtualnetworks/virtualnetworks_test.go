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

package virtualnetworks

import (
	"context"
	"net/http"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"k8s.io/klog/klogr"
	infrav1 "github.com/niachary/cluster-api-provider-azure/api/v1alpha3"
	azure "github.com/niachary/cluster-api-provider-azure/cloud"
	"github.com/niachary/cluster-api-provider-azure/cloud/services/virtualnetworks/mock_virtualnetworks"
)

func TestReconcileVnet(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder)
	}{
		{
			name:          "managed vnet exists",
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{Name: "vnet-exists"})
				s.VNetSpecs().Return([]azure.VNetSpec{
					{
						ResourceGroup: "my-rg",
						Name:          "vnet-exists",
						CIDR:          "10.0.0.0/8",
					},
				})
				m.Get(context.TODO(), "my-rg", "vnet-exists").
					Return(network.VirtualNetwork{
						ID:   to.StringPtr("azure/fake/id"),
						Name: to.StringPtr("vnet-exists"),
						VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
							AddressSpace: &network.AddressSpace{
								AddressPrefixes: to.StringSlicePtr([]string{"10.0.0.0/8"}),
							},
						},
						Tags: map[string]*string{
							"Name": to.StringPtr("vnet-exists"),
							"sigs.k8s.io_cluster-api-provider-azure_cluster_fake-cluster": to.StringPtr("owned"),
							"sigs.k8s.io_cluster-api-provider-azure_role":                 to.StringPtr("common"),
						},
					}, nil)
			},
		},
		{
			name:          "vnet created successufuly",
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.Location().AnyTimes().Return("fake-location")
				s.AdditionalTags().AnyTimes().Return(infrav1.Tags{})
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{Name: "vnet-new"})
				s.VNetSpecs().Return([]azure.VNetSpec{
					{
						ResourceGroup: "my-rg",
						Name:          "vnet-new",
						CIDR:          "10.0.0.0/8",
					},
				})
				m.Get(context.TODO(), "my-rg", "vnet-new").
					Return(network.VirtualNetwork{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))

				m.CreateOrUpdate(context.TODO(), "my-rg", "vnet-new", gomock.AssignableToTypeOf(network.VirtualNetwork{}))
			},
		},
		{
			name:          "unmanaged vnet exists",
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.Location().AnyTimes().Return("fake-location")
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{Name: "custom-vnet"})
				s.VNetSpecs().Return([]azure.VNetSpec{
					{
						ResourceGroup: "custom-vnet-rg",
						Name:          "custom-vnet",
						CIDR:          "10.0.0.0/16",
					},
				})
				m.Get(context.TODO(), "custom-vnet-rg", "custom-vnet").
					Return(network.VirtualNetwork{
						ID:   to.StringPtr("azure/custom-vnet/id"),
						Name: to.StringPtr("custom-vnet"),
						VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
							AddressSpace: &network.AddressSpace{
								AddressPrefixes: to.StringSlicePtr([]string{"10.0.0.0/16"}),
							},
						},
						Tags: map[string]*string{
							"Name": to.StringPtr("my-custom-vnet"),
						},
					}, nil)
			},
		},
		{
			name:          "custom vnet not found",
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.Location().AnyTimes().Return("fake-location")
				s.AdditionalTags().AnyTimes().Return(infrav1.Tags{})
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{Name: "custom-vnet"})
				s.VNetSpecs().Return([]azure.VNetSpec{
					{
						ResourceGroup: "custom-vnet-rg",
						Name:          "custom-vnet",
						CIDR:          "10.0.0.0/16",
					},
				})
				m.Get(context.TODO(), "custom-vnet-rg", "custom-vnet").
					Return(network.VirtualNetwork{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))

				m.CreateOrUpdate(context.TODO(), "custom-vnet-rg", "custom-vnet", gomock.AssignableToTypeOf(network.VirtualNetwork{}))
			},
		},
		{
			name:          "failed to fetch vnet",
			expectedError: "failed to get VNet custom-vnet: failed to get VNet custom-vnet: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.VNetSpecs().Return([]azure.VNetSpec{
					{
						ResourceGroup: "custom-vnet-rg",
						Name:          "custom-vnet",
						CIDR:          "10.0.0.0/16",
					},
				})
				m.Get(context.TODO(), "custom-vnet-rg", "custom-vnet").
					Return(network.VirtualNetwork{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "fail to create vnet",
			expectedError: "failed to create virtual network custom-vnet: #: Internal Server Honk: StatusCode=500",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.Location().AnyTimes().Return("fake-location")
				s.AdditionalTags().AnyTimes().Return(infrav1.Tags{})
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{Name: "custom-vnet"})
				s.VNetSpecs().Return([]azure.VNetSpec{
					{
						ResourceGroup: "custom-vnet-rg",
						Name:          "custom-vnet",
						CIDR:          "10.0.0.0/16",
					},
				})
				m.Get(context.TODO(), "custom-vnet-rg", "custom-vnet").
					Return(network.VirtualNetwork{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))

				m.CreateOrUpdate(context.TODO(), "custom-vnet-rg", "custom-vnet", gomock.AssignableToTypeOf(network.VirtualNetwork{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Honk"))
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
			scopeMock := mock_virtualnetworks.NewMockVNetScope(mockCtrl)
			clientMock := mock_virtualnetworks.NewMockClient(mockCtrl)

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

func TestDeleteVnet(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder)
	}{
		{
			name:          "managed vnet exists",
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.Location().AnyTimes().Return("fake-location")
				s.AdditionalTags().AnyTimes().Return(infrav1.Tags{
					"Name": "vnet-exists",
					"sigs.k8s.io_cluster-api-provider-azure_cluster_fake-cluster": "owned",
					"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
				})
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{Name: "vnet-exists"})
				s.VNetSpecs().Return([]azure.VNetSpec{
					{
						ResourceGroup: "my-rg",
						Name:          "vnet-exists",
						CIDR:          "10.0.0.0/16",
					},
				})
				m.Delete(context.TODO(), "my-rg", "vnet-exists")
			},
		},
		{
			name:          "managed vnet already deleted",
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.Location().AnyTimes().Return("fake-location")
				s.AdditionalTags().AnyTimes().Return(infrav1.Tags{
					"Name": "vnet-exists",
					"sigs.k8s.io_cluster-api-provider-azure_cluster_fake-cluster": "owned",
					"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
				})
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{Name: "vnet-exists"})
				s.VNetSpecs().Return([]azure.VNetSpec{
					{
						ResourceGroup: "my-rg",
						Name:          "vnet-exists",
						CIDR:          "10.0.0.0/16",
					},
				})
				m.Delete(context.TODO(), "my-rg", "vnet-exists").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name:          "unmanaged vnet",
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.Location().AnyTimes().Return("fake-location")
				s.AdditionalTags().AnyTimes().Return(infrav1.Tags{})
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{ResourceGroup: "my-rg", Name: "my-vnet", ID: "azure/custom-vnet/id"})
				s.VNetSpecs().Return([]azure.VNetSpec{
					{
						ResourceGroup: "my-rg",
						Name:          "my-vnet",
						CIDR:          "10.0.0.0/16",
					},
				})
			},
		},
		{
			name:          "fail to delete vnet",
			expectedError: "failed to delete VNet vnet-exists in resource group my-rg: #: Internal Honk Server: StatusCode=500",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ClusterName().AnyTimes().Return("fake-cluster")
				s.Location().AnyTimes().Return("fake-location")
				s.AdditionalTags().AnyTimes().Return(infrav1.Tags{
					"Name": "vnet-exists",
					"sigs.k8s.io_cluster-api-provider-azure_cluster_fake-cluster": "owned",
					"sigs.k8s.io_cluster-api-provider-azure_role":                 "common",
				})
				s.Vnet().AnyTimes().Return(&infrav1.VnetSpec{Name: "vnet-exists"})
				s.VNetSpecs().Return([]azure.VNetSpec{
					{
						ResourceGroup: "my-rg",
						Name:          "vnet-exists",
						CIDR:          "10.0.0.0/16",
					},
				})
				m.Delete(context.TODO(), "my-rg", "vnet-exists").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Honk Server"))

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
			scopeMock := mock_virtualnetworks.NewMockVNetScope(mockCtrl)
			clientMock := mock_virtualnetworks.NewMockClient(mockCtrl)

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

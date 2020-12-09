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

package bastionhosts

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	mock_bastionhosts "sigs.k8s.io/cluster-api-provider-azure/cloud/services/bastionhosts/mocks_bastionhosts"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips/mock_publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/subnets/mock_subnets"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"

	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"

	network "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

func init() {
	_ = clusterv1.AddToScheme(scheme.Scheme)
}

func TestReconcileBastionHosts(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_bastionhosts.MockBastionScopeMockRecorder,
			m *mock_bastionhosts.MockclientMockRecorder,
			mSubnet *mock_subnets.MockClientMockRecorder,
			mPublicIP *mock_publicips.MockClientMockRecorder)
	}{
		{
			name:          "fail to get subnets",
			expectedError: "failed to get subnet: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder,
				m *mock_bastionhosts.MockclientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.BastionSpecs().Return([]azure.BastionSpec{
					{
						Name:         "my-bastion",
						VNetName:     "my-vnet",
						SubnetName:   "my-subnet",
						PublicIPName: "my-publicip",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				mSubnet.Get(gomockinternal.AContext(), "my-rg", "my-vnet", "my-subnet").
					Return(network.Subnet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "fail to get publicip",
			expectedError: "failed to get existing publicIP: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder,
				m *mock_bastionhosts.MockclientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.BastionSpecs().Return([]azure.BastionSpec{
					{
						Name:         "my-bastion",
						VNetName:     "my-vnet",
						SubnetName:   "my-subnet",
						PublicIPName: "my-publicip",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				gomock.InOrder(
					mSubnet.Get(gomockinternal.AContext(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mPublicIP.Get(gomockinternal.AContext(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")),
				)
			},
		},
		{
			name:          "create publicip fails",
			expectedError: "failed to create bastion publicIP: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder,
				m *mock_bastionhosts.MockclientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.BastionSpecs().Return([]azure.BastionSpec{
					{
						Name:         "my-bastion",
						VNetName:     "my-vnet",
						SubnetName:   "my-subnet",
						PublicIPName: "my-publicip",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				gomock.InOrder(
					mSubnet.Get(gomockinternal.AContext(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mPublicIP.Get(gomockinternal.AContext(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found")),
					mPublicIP.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-publicip", gomock.AssignableToTypeOf(network.PublicIPAddress{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")),
				)
			},
		},
		{
			name:          "fails to get a created publicip",
			expectedError: "failed to get created publicIP: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder,
				m *mock_bastionhosts.MockclientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.BastionSpecs().Return([]azure.BastionSpec{
					{
						Name:         "my-bastion",
						VNetName:     "my-vnet",
						SubnetName:   "my-subnet",
						PublicIPName: "my-publicip",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				mSubnet.Get(gomockinternal.AContext(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil)
				mPublicIP.Get(gomockinternal.AContext(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				mPublicIP.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-publicip", gomock.AssignableToTypeOf(network.PublicIPAddress{})).Return(nil)
				mPublicIP.Get(gomockinternal.AContext(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "bastion successfully created with created public ip",
			expectedError: "",
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder,
				m *mock_bastionhosts.MockclientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.BastionSpecs().Return([]azure.BastionSpec{
					{
						Name:         "my-bastion",
						VNetName:     "my-vnet",
						SubnetName:   "my-subnet",
						PublicIPName: "my-publicip",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.ClusterName().AnyTimes().Return("fake-cluster")
				gomock.InOrder(
					mSubnet.Get(gomockinternal.AContext(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mPublicIP.Get(gomockinternal.AContext(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found")),
					mPublicIP.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-publicip", gomock.AssignableToTypeOf(network.PublicIPAddress{})).Return(nil),
					mPublicIP.Get(gomockinternal.AContext(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, nil),
					m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-bastion", gomock.AssignableToTypeOf(network.BastionHost{})),
				)
			},
		},
		{
			name:          "bastion successfully created",
			expectedError: "",
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder,
				m *mock_bastionhosts.MockclientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.BastionSpecs().Return([]azure.BastionSpec{
					{
						Name:         "my-bastion",
						VNetName:     "my-vnet",
						SubnetName:   "my-subnet",
						PublicIPName: "my-publicip",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.ClusterName().AnyTimes().Return("fake-cluster")
				gomock.InOrder(
					mSubnet.Get(gomockinternal.AContext(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mPublicIP.Get(gomockinternal.AContext(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, nil),
					m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-bastion", gomock.AssignableToTypeOf(network.BastionHost{})),
				)
			},
		},
		{
			name:          "fail to create a bastion",
			expectedError: "cannot create bastion host: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder,
				m *mock_bastionhosts.MockclientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.BastionSpecs().Return([]azure.BastionSpec{
					{
						Name:         "my-bastion",
						VNetName:     "my-vnet",
						SubnetName:   "my-subnet",
						PublicIPName: "my-publicip",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.ClusterName().AnyTimes().Return("fake-cluster")
				gomock.InOrder(
					mSubnet.Get(gomockinternal.AContext(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					mPublicIP.Get(gomockinternal.AContext(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, nil),
					m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-bastion", gomock.AssignableToTypeOf(network.BastionHost{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")),
				)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_bastionhosts.NewMockBastionScope(mockCtrl)
			clientMock := mock_bastionhosts.NewMockclient(mockCtrl)
			subnetMock := mock_subnets.NewMockClient(mockCtrl)
			publicIPsMock := mock_publicips.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(),
				subnetMock.EXPECT(), publicIPsMock.EXPECT())

			s := &Service{
				Scope:           scopeMock,
				client:          clientMock,
				subnetsClient:   subnetMock,
				publicIPsClient: publicIPsMock,
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

func TestDeleteBastionHost(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_bastionhosts.MockBastionScopeMockRecorder,
			m *mock_bastionhosts.MockclientMockRecorder,
			mSubnet *mock_subnets.MockClientMockRecorder,
			mPublicIP *mock_publicips.MockClientMockRecorder)
	}{
		{
			name: "successfully delete an existing bastion host",

			expectedError: "",
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder,
				m *mock_bastionhosts.MockclientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.BastionSpecs().Return([]azure.BastionSpec{
					{
						Name:         "my-bastionhost",
						VNetName:     "my-vnet",
						SubnetName:   "my-subnet",
						PublicIPName: "my-publicip",
					},
					{
						Name:         "my-bastionhost1",
						VNetName:     "my-vnet",
						SubnetName:   "my-subnet",
						PublicIPName: "my-publicip",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-bastionhost")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-bastionhost1")
			},
		},
		{
			name:          "bastion host already deleted",
			expectedError: "",
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder,
				m *mock_bastionhosts.MockclientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.BastionSpecs().Return([]azure.BastionSpec{
					{
						Name:         "my-bastionhost",
						VNetName:     "my-vnet",
						SubnetName:   "my-subnet",
						PublicIPName: "my-publicip",
					},
					{
						Name:         "my-bastionhost1",
						VNetName:     "my-vnet",
						SubnetName:   "my-subnet",
						PublicIPName: "my-publicip",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-bastionhost").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.Delete(gomockinternal.AContext(), "my-rg", "my-bastionhost1")
			},
		},
		{
			name:          "bastion host deletion fails",
			expectedError: "failed to delete Bastion Host my-bastionhost in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder,
				m *mock_bastionhosts.MockclientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.BastionSpecs().Return([]azure.BastionSpec{
					{
						Name:         "my-bastionhost",
						VNetName:     "my-vnet",
						SubnetName:   "my-subnet",
						PublicIPName: "my-publicip",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-bastionhost").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_bastionhosts.NewMockBastionScope(mockCtrl)
			clientMock := mock_bastionhosts.NewMockclient(mockCtrl)
			subnetMock := mock_subnets.NewMockClient(mockCtrl)
			publicIPsMock := mock_publicips.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(),
				subnetMock.EXPECT(), publicIPsMock.EXPECT())

			s := &Service{
				Scope:           scopeMock,
				client:          clientMock,
				subnetsClient:   subnetMock,
				publicIPsClient: publicIPsMock,
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

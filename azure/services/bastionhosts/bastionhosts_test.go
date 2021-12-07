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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	mock_bastionhosts "sigs.k8s.io/cluster-api-provider-azure/azure/services/bastionhosts/mocks_bastionhosts"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/publicips/mock_publicips"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/subnets/mock_subnets"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
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
			name:          "fail to get publicip",
			expectedError: "error creating Azure Bastion: failed to get public IP for azure bastion: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder,
				m *mock_bastionhosts.MockclientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				s.BastionSpec().Return(azure.BastionSpec{
					AzureBastion: &azure.AzureBastionSpec{
						Name:     "my-bastion",
						VNetName: "my-vnet",
						SubnetSpec: v1beta1.SubnetSpec{
							Name: "my-subnet",
						},
						PublicIPName: "my-publicip",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				mPublicIP.Get(gomockinternal.AContext(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "fail to get subnets",
			expectedError: "error creating Azure Bastion: failed to get subnet for azure bastion: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder,
				m *mock_bastionhosts.MockclientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				s.BastionSpec().Return(azure.BastionSpec{
					AzureBastion: &azure.AzureBastionSpec{
						Name:     "my-bastion",
						VNetName: "my-vnet",
						SubnetSpec: v1beta1.SubnetSpec{
							Name: "my-subnet",
						},
						PublicIPName: "my-publicip",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				gomock.InOrder(
					mPublicIP.Get(gomockinternal.AContext(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, nil),
					mSubnet.Get(gomockinternal.AContext(), "my-rg", "my-vnet", "my-subnet").
						Return(network.Subnet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")),
				)
			},
		},
		{
			name:          "fail to create a bastion",
			expectedError: "error creating Azure Bastion: cannot create Azure Bastion: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder,
				m *mock_bastionhosts.MockclientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				s.BastionSpec().Return(azure.BastionSpec{
					AzureBastion: &azure.AzureBastionSpec{
						Name:     "my-bastion",
						VNetName: "my-vnet",
						SubnetSpec: v1beta1.SubnetSpec{
							Name: "my-subnet",
						},
						PublicIPName: "my-publicip",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.ClusterName().AnyTimes().Return("fake-cluster")
				gomock.InOrder(
					mPublicIP.Get(gomockinternal.AContext(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, nil),
					mSubnet.Get(gomockinternal.AContext(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-bastion", gomock.AssignableToTypeOf(network.BastionHost{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")),
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
				s.BastionSpec().Return(azure.BastionSpec{
					AzureBastion: &azure.AzureBastionSpec{
						Name:     "my-bastion",
						VNetName: "my-vnet",
						SubnetSpec: v1beta1.SubnetSpec{
							Name: "my-subnet",
						},
						PublicIPName: "my-publicip",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.Location().AnyTimes().Return("fake-location")
				s.ClusterName().AnyTimes().Return("fake-cluster")
				gomock.InOrder(
					mPublicIP.Get(gomockinternal.AContext(), "my-rg", "my-publicip").Return(network.PublicIPAddress{}, nil),
					mSubnet.Get(gomockinternal.AContext(), "my-rg", "my-vnet", "my-subnet").Return(network.Subnet{}, nil),
					m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-bastion", gomock.AssignableToTypeOf(network.BastionHost{})),
				)
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
			name:          "bastion host deletion fails",
			expectedError: "error deleting Azure Bastion: failed to delete Azure Bastion my-bastionhost in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder,
				m *mock_bastionhosts.MockclientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				s.BastionSpec().Return(azure.BastionSpec{
					AzureBastion: &azure.AzureBastionSpec{
						Name:     "my-bastionhost",
						VNetName: "my-vnet",
						SubnetSpec: v1beta1.SubnetSpec{
							Name: "my-subnet",
						},
						PublicIPName: "my-publicip",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-bastionhost").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "bastion host already deleted",
			expectedError: "",
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder,
				m *mock_bastionhosts.MockclientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				s.BastionSpec().Return(azure.BastionSpec{
					AzureBastion: &azure.AzureBastionSpec{
						Name:     "my-bastionhost",
						VNetName: "my-vnet",
						SubnetSpec: v1beta1.SubnetSpec{
							Name: "my-subnet",
						},
						PublicIPName: "my-publicip",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-bastionhost").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "successfully delete an existing bastion host",

			expectedError: "",
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder,
				m *mock_bastionhosts.MockclientMockRecorder,
				mSubnet *mock_subnets.MockClientMockRecorder,
				mPublicIP *mock_publicips.MockClientMockRecorder) {
				s.BastionSpec().Return(azure.BastionSpec{
					AzureBastion: &azure.AzureBastionSpec{
						Name:     "my-bastionhost",
						VNetName: "my-vnet",
						SubnetSpec: v1beta1.SubnetSpec{
							Name: "my-subnet",
						},
						PublicIPName: "my-publicip",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(gomockinternal.AContext(), "my-rg", "my-bastionhost")
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

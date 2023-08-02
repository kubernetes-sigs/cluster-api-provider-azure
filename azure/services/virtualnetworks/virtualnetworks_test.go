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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-10-01/resources"
	"github.com/Azure/go-autorest/autorest"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/virtualnetworks/mock_virtualnetworks"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	fakeVNetSpec = VNetSpec{
		ResourceGroup:  "test-group",
		Name:           "test-vnet",
		CIDRs:          []string{"10.0.0.0/8"},
		Location:       "test-location",
		ClusterName:    "test-cluster",
		AdditionalTags: map[string]string{"foo": "bar"},
	}

	managedTags = resources.TagsResource{
		Properties: &resources.Tags{
			Tags: map[string]*string{
				"foo": pointer.String("bar"),
				"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": pointer.String("owned"),
			},
		},
	}

	unmanagedTags = resources.TagsResource{
		Properties: &resources.Tags{
			Tags: map[string]*string{
				"foo":       pointer.String("bar"),
				"something": pointer.String("else"),
			},
		},
	}

	customVnet = network.VirtualNetwork{
		ID:   pointer.String("/subscriptions/subscription/resourceGroups/test-group/providers/Microsoft.Network/virtualNetworks/test-vnet"),
		Name: pointer.String("test-vnet"),
		Tags: map[string]*string{
			"foo":       pointer.String("bar"),
			"something": pointer.String("else"),
		},
		VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
			AddressSpace: &network.AddressSpace{
				AddressPrefixes: &[]string{"fake-cidr"},
			},
			Subnets: &[]network.Subnet{
				{
					Name: pointer.String("test-subnet"),
					SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
						AddressPrefix: pointer.String("subnet-cidr"),
					},
				},
				{
					Name: pointer.String("test-subnet-2"),
					SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
						AddressPrefixes: &[]string{
							"subnet-cidr-1",
							"subnet-cidr-2",
						},
					},
				},
			},
		},
	}
	internalError = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error")
)

func TestReconcileVnet(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no vnet spec is found",
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VNetSpec().Return(nil)
			},
		},
		{
			name:          "reconcile when vnet is not managed",
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VNetSpec().Return(&fakeVNetSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeVNetSpec, serviceName).Return(nil, nil)
				s.IsVnetManaged().Return(false)
			},
		},
		{
			name:          "create vnet succeeds, should not return an error",
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VNetSpec().Return(&fakeVNetSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeVNetSpec, serviceName).Return(nil, nil)
				s.IsVnetManaged().Return(true)
				s.UpdatePutStatus(infrav1.VNetReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "create vnet fails, should return an error",
			expectedError: internalError.Error(),
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VNetSpec().Return(&fakeVNetSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeVNetSpec, serviceName).Return(nil, internalError)
				s.IsVnetManaged().Return(true)
				s.UpdatePutStatus(infrav1.VNetReadyCondition, serviceName, internalError)
			},
		},
		{
			name:          "existing vnet should update subnet CIDR blocks",
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VNetSpec().Return(&fakeVNetSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeVNetSpec, serviceName).Return(customVnet, nil)
				s.Vnet().Return(&infrav1.VnetSpec{})
				s.UpdateSubnetCIDRs("test-subnet", []string{"subnet-cidr"})
				s.UpdateSubnetCIDRs("test-subnet-2", []string{"subnet-cidr-1", "subnet-cidr-2"})
				s.IsVnetManaged().Return(false)
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
			tagsGetterMock := mock_async.NewMockTagsGetter(mockCtrl)
			reconcilerMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), tagsGetterMock.EXPECT(), reconcilerMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				TagsGetter: tagsGetterMock,
				Reconciler: reconcilerMock,
			}

			err := s.Reconcile(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
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
		expect        func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no vnet spec is found",
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VNetSpec().Return(nil)
			},
		},
		{
			name:          "delete vnet succeeds, should not return an error",
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VNetSpec().Times(2).Return(&fakeVNetSpec)
				s.SubscriptionID().Return("123")
				m.GetAtScope(gomockinternal.AContext(), azure.VNetID("123", fakeVNetSpec.ResourceGroupName(), fakeVNetSpec.Name)).Return(managedTags, nil)
				s.ClusterName().Return("test-cluster")
				r.DeleteResource(gomockinternal.AContext(), &fakeVNetSpec, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.VNetReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "delete vnet fails, should return an error",
			expectedError: internalError.Error(),
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VNetSpec().Times(2).Return(&fakeVNetSpec)
				s.SubscriptionID().Return("123")
				m.GetAtScope(gomockinternal.AContext(), azure.VNetID("123", fakeVNetSpec.ResourceGroupName(), fakeVNetSpec.Name)).Return(managedTags, nil)
				s.ClusterName().Return("test-cluster")
				r.DeleteResource(gomockinternal.AContext(), &fakeVNetSpec, serviceName).Return(internalError)
				s.UpdateDeleteStatus(infrav1.VNetReadyCondition, serviceName, internalError)
			},
		},
		{
			name:          "vnet is not managed, do nothing",
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VNetSpec().Times(2).Return(&fakeVNetSpec)
				s.SubscriptionID().Return("123")
				m.GetAtScope(gomockinternal.AContext(), azure.VNetID("123", fakeVNetSpec.ResourceGroupName(), fakeVNetSpec.Name)).Return(unmanagedTags, nil)
				s.ClusterName().Return("test-cluster")
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
			tagsGetterMock := mock_async.NewMockTagsGetter(mockCtrl)
			reconcilerMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), tagsGetterMock.EXPECT(), reconcilerMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				TagsGetter: tagsGetterMock,
				Reconciler: reconcilerMock,
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

func TestIsVnetManaged(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		result        bool
		expect        func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder)
	}{
		{
			name:          "spec is nil",
			result:        false,
			expectedError: "cannot get vnet to check if it is managed: spec is nil",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder) {
				s.VNetSpec().Return(nil)
			},
		},
		{
			name:          "managed vnet returns true",
			result:        true,
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder) {
				s.VNetSpec().Return(&fakeVNetSpec)
				s.SubscriptionID().Return("123")
				m.GetAtScope(gomockinternal.AContext(), azure.VNetID("123", fakeVNetSpec.ResourceGroupName(), fakeVNetSpec.Name)).Return(managedTags, nil)
				s.ClusterName().Return("test-cluster")
			},
		},
		{
			name:          "custom vnet returns false",
			result:        false,
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder) {
				s.VNetSpec().Return(&fakeVNetSpec)
				s.SubscriptionID().Return("123")
				m.GetAtScope(gomockinternal.AContext(), azure.VNetID("123", fakeVNetSpec.ResourceGroupName(), fakeVNetSpec.Name)).Return(unmanagedTags, nil)
				s.ClusterName().Return("test-cluster")
			},
		},
		{
			name:          "GET fails returns an error",
			expectedError: internalError.Error(),
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockTagsGetterMockRecorder) {
				s.VNetSpec().Return(&fakeVNetSpec)
				s.SubscriptionID().Return("123")
				m.GetAtScope(gomockinternal.AContext(), azure.VNetID("123", fakeVNetSpec.ResourceGroupName(), fakeVNetSpec.Name)).Return(resources.TagsResource{}, internalError)
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
			tagsGetterMock := mock_async.NewMockTagsGetter(mockCtrl)

			tc.expect(scopeMock.EXPECT(), tagsGetterMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				TagsGetter: tagsGetterMock,
			}

			result, err := s.IsManaged(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(tc.result))
			}
		})
	}
}

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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
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
	managedVnet = network.VirtualNetwork{
		ID:   to.StringPtr("/subscriptions/subscription/resourceGroups/test-group/providers/Microsoft.Network/virtualNetworks/test-vnet"),
		Name: to.StringPtr("test-vnet"),
		Tags: map[string]*string{
			"foo": to.StringPtr("bar"),
			"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": to.StringPtr("owned"),
		},
	}
	customVnet = network.VirtualNetwork{
		ID:   to.StringPtr("/subscriptions/subscription/resourceGroups/test-group/providers/Microsoft.Network/virtualNetworks/test-vnet"),
		Name: to.StringPtr("test-vnet"),
		Tags: map[string]*string{
			"foo":       to.StringPtr("bar"),
			"something": to.StringPtr("else"),
		},
	}
	internalError = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")
)

func TestReconcileVnet(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "create vnet succeeds, should not return an error",
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VNetSpec().Return(&fakeVNetSpec)
				r.CreateResource(gomockinternal.AContext(), &fakeVNetSpec, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.VNetReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "create vnet fails, should return an error",
			expectedError: internalError.Error(),
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VNetSpec().Return(&fakeVNetSpec)
				r.CreateResource(gomockinternal.AContext(), &fakeVNetSpec, serviceName).Return(nil, internalError)
				s.UpdatePutStatus(infrav1.VNetReadyCondition, serviceName, internalError)
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
			getterMock := mock_async.NewMockGetter(mockCtrl)
			reconcilerMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), getterMock.EXPECT(), reconcilerMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Getter:     getterMock,
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
		expect        func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "delete vnet succeeds, should not return an error",
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VNetSpec().Return(&fakeVNetSpec)
				m.Get(gomockinternal.AContext(), &fakeVNetSpec).Return(managedVnet, nil)
				s.ClusterName().Return("test-cluster")
				r.DeleteResource(gomockinternal.AContext(), &fakeVNetSpec, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.VNetReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "delete vnet fails, should return an error",
			expectedError: internalError.Error(),
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VNetSpec().Return(&fakeVNetSpec)
				m.Get(gomockinternal.AContext(), &fakeVNetSpec).Return(managedVnet, nil)
				s.ClusterName().Return("test-cluster")
				r.DeleteResource(gomockinternal.AContext(), &fakeVNetSpec, serviceName).Return(internalError)
				s.UpdateDeleteStatus(infrav1.VNetReadyCondition, serviceName, internalError)
			},
		},
		{
			name:          "vnet is not managed, do nothing",
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockGetterMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VNetSpec().Return(&fakeVNetSpec)
				m.Get(gomockinternal.AContext(), &fakeVNetSpec).Return(customVnet, nil)
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
			getterMock := mock_async.NewMockGetter(mockCtrl)
			reconcilerMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), getterMock.EXPECT(), reconcilerMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Getter:     getterMock,
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
		vnetSpec      azure.ResourceSpecGetter
		expectedError string
		result        bool
		expect        func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockGetterMockRecorder)
	}{
		{
			name:          "spec is nil",
			vnetSpec:      nil,
			result:        false,
			expectedError: "cannot get vnet to check if it is managed: spec is nil",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockGetterMockRecorder) {
			},
		},
		{
			name:          "managed vnet returns true",
			vnetSpec:      &fakeVNetSpec,
			result:        true,
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockGetterMockRecorder) {
				m.Get(gomockinternal.AContext(), &fakeVNetSpec).Return(managedVnet, nil)
				s.ClusterName().Return("test-cluster")
			},
		},
		{
			name:          "custom vnet returns false",
			vnetSpec:      &fakeVNetSpec,
			result:        false,
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockGetterMockRecorder) {
				m.Get(gomockinternal.AContext(), &fakeVNetSpec).Return(customVnet, nil)
				s.ClusterName().Return("test-cluster")
			},
		},
		{
			name:          "GET fails returns an error",
			vnetSpec:      &fakeVNetSpec,
			expectedError: internalError.Error(),
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_async.MockGetterMockRecorder) {
				m.Get(gomockinternal.AContext(), &fakeVNetSpec).Return(network.VirtualNetwork{}, internalError)
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
			getterMock := mock_async.NewMockGetter(mockCtrl)

			tc.expect(scopeMock.EXPECT(), getterMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				Getter: getterMock,
			}

			result, err := s.IsManaged(context.TODO(), tc.vnetSpec)
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

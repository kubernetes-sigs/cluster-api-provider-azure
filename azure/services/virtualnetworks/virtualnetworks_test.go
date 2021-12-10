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
	"fmt"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
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
	fakeCreateFuture = infrav1.Future{
		Type:          infrav1.PutFuture,
		ServiceName:   serviceName,
		Name:          "test-vnet",
		ResourceGroup: "test-group",
		Data:          "eyJtZXRob2QiOiJQVVQiLCJwb2xsaW5nTWV0aG9kIjoiTG9jYXRpb24iLCJscm9TdGF0ZSI6IkluUHJvZ3Jlc3MifQ==",
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
		expect        func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "create vnet succeeds, should not return an error",
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VNetSpec().Return(&fakeVNetSpec)
				r.CreateResource(gomockinternal.AContext(), &fakeVNetSpec, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.VNetReadyCondition, serviceName, nil)
				s.Vnet().Return(&infrav1.VnetSpec{})
			},
		},
		{
			name:          "create vnet fails, should return an error",
			expectedError: "failed to create resource test-group/test-vnet (service: virtualnetwork): #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VNetSpec().Return(&fakeVNetSpec)
				r.CreateResource(gomockinternal.AContext(), &fakeVNetSpec, serviceName).Return(nil, nil, internalError)
				s.UpdatePutStatus(infrav1.VNetReadyCondition, serviceName, gomockinternal.ErrStrEq(fmt.Sprintf("failed to create resource test-group/test-vnet (service: virtualnetwork): %s", internalError.Error())))
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
			reconcilerMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), reconcilerMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Client:     clientMock,
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
		expect        func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "delete vnet succeeds, should not return an error",
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VNetSpec().Return(&fakeVNetSpec)
				s.IsVnetManaged(gomockinternal.AContext()).Return(true, nil)
				r.DeleteResource(gomockinternal.AContext(), &fakeVNetSpec, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.VNetReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "delete vnet fails, should return an error",
			expectedError: "failed to delete resource test-group/test-vnet (service: virtualnetwork): #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VNetSpec().Return(&fakeVNetSpec)
				s.IsVnetManaged(gomockinternal.AContext()).Return(true, nil)
				r.DeleteResource(gomockinternal.AContext(), &fakeVNetSpec, serviceName).Return(internalError)
				s.UpdateDeleteStatus(infrav1.VNetReadyCondition, serviceName, gomockinternal.ErrStrEq(fmt.Sprintf("failed to delete resource test-group/test-vnet (service: virtualnetwork): %s", internalError.Error())))
			},
		},
		{
			name:          "vnet is not managed, do nothing",
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VNetSpec().Return(&fakeVNetSpec)
				s.IsVnetManaged(gomockinternal.AContext()).Return(false, nil)
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
			reconcilerMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), reconcilerMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Client:     clientMock,
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
		expect        func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder)
	}{
		{
			name:          "managed vnet returns true",
			result:        true,
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder) {
				s.VNetSpec().Return(&fakeVNetSpec)
				m.Get(gomockinternal.AContext(), "test-group", "test-vnet").Return(managedVnet, nil)
				s.ClusterName().Return("test-cluster")
			},
		},
		{
			name:          "custom vnet returns false",
			result:        false,
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.VNetSpec().Return(&fakeVNetSpec)
				m.Get(gomockinternal.AContext(), "test-group", "test-vnet").Return(customVnet, nil)
				s.ClusterName().Return("test-cluster")
			},
		},
		{
			name:          "GET fails returns an error",
			expectedError: internalError.Error(),
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.VNetSpec().Return(&fakeVNetSpec)
				m.Get(gomockinternal.AContext(), "test-group", "test-vnet").Return(network.VirtualNetwork{}, internalError)
			},
		},
		{
			name:          "vnet not found, spec has owned tag",
			result:        true,
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.VNetSpec().Return(&fakeVNetSpec)
				m.Get(gomockinternal.AContext(), "test-group", "test-vnet").Return(network.VirtualNetwork{}, notFoundError)
				s.Vnet().Times(2).Return(&infrav1.VnetSpec{ID: "my/vnet/id", Name: "test-vnet", Tags: infrav1.Tags{"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": "owned"}})
				s.ClusterName().Return("test-cluster")
			},
		},
		{
			name:          "vnet not found, spec doesn't have owned tag",
			result:        false,
			expectedError: "",
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.VNetSpec().Return(&fakeVNetSpec)
				m.Get(gomockinternal.AContext(), "test-group", "test-vnet").Return(network.VirtualNetwork{}, notFoundError)
				s.Vnet().Times(2).Return(&infrav1.VnetSpec{ID: "my/vnet/id", Name: "test-vnet", Tags: infrav1.Tags{"foo": "bar"}})
				s.ClusterName().Return("test-cluster")
			},
		},
		{
			name:          "vnet not found, spec doesn't ID yet",
			result:        false,
			expectedError: notFoundError.Error(),
			expect: func(s *mock_virtualnetworks.MockVNetScopeMockRecorder, m *mock_virtualnetworks.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.VNetSpec().Return(&fakeVNetSpec)
				m.Get(gomockinternal.AContext(), "test-group", "test-vnet").Return(network.VirtualNetwork{}, notFoundError)
				s.Vnet().Return(&infrav1.VnetSpec{Name: "test-vnet"})
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

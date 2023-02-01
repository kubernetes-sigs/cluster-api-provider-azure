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

package groups

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/groups/mock_groups"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	fakeGroupSpec = GroupSpec{
		Name:           "test-group",
		Location:       "test-location",
		ClusterName:    "test-cluster",
		AdditionalTags: map[string]string{"foo": "bar"},
	}
	internalError      = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error")
	notFoundError      = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusNotFound}, "Not Found")
	sampleManagedGroup = resources.Group{
		Name:       pointer.String("test-group"),
		Location:   pointer.String("test-location"),
		Properties: &resources.GroupProperties{},
		Tags:       map[string]*string{"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": pointer.String("owned")},
	}
	sampleBYOGroup = resources.Group{
		Name:       pointer.String("test-group"),
		Location:   pointer.String("test-location"),
		Properties: &resources.GroupProperties{},
		Tags:       map[string]*string{"foo": pointer.String("bar")},
	}
)

func TestReconcileGroups(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no group spec is found",
			expectedError: "",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.GroupSpec().Return(nil)
			},
		},
		{
			name:          "create group succeeds",
			expectedError: "",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.GroupSpec().Return(&fakeGroupSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeGroupSpec, ServiceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.ResourceGroupReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "create resource group fails",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.GroupSpec().Return(&fakeGroupSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeGroupSpec, ServiceName).Return(nil, internalError)
				s.UpdatePutStatus(infrav1.ResourceGroupReadyCondition, ServiceName, internalError)
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
			scopeMock := mock_groups.NewMockGroupScope(mockCtrl)
			clientMock := mock_groups.NewMockclient(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				client:     clientMock,
				Reconciler: asyncMock,
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

func TestDeleteGroups(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no group spec is found",
			expectedError: "",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.GroupSpec().Return(nil)
			},
		},
		{
			name:          "delete operation is successful for managed resource group",
			expectedError: "",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.GroupSpec().AnyTimes().Return(&fakeGroupSpec)
				m.Get(gomockinternal.AContext(), &fakeGroupSpec).Return(sampleManagedGroup, nil)
				s.ClusterName().Return("test-cluster")
				r.DeleteResource(gomockinternal.AContext(), &fakeGroupSpec, ServiceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.ResourceGroupReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "resource group is not managed by capz",
			expectedError: "",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.GroupSpec().AnyTimes().Return(&fakeGroupSpec)
				m.Get(gomockinternal.AContext(), &fakeGroupSpec).Return(sampleBYOGroup, nil)
				s.ClusterName().Return("test-cluster")
			},
		},
		{
			name:          "fail to check if resource group is managed",
			expectedError: "could not get resource group management state",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.GroupSpec().AnyTimes().Return(&fakeGroupSpec)
				m.Get(gomockinternal.AContext(), &fakeGroupSpec).Return(resources.Group{}, internalError)
			},
		},
		{
			name:          "resource group doesn't exist",
			expectedError: "",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.GroupSpec().AnyTimes().Return(&fakeGroupSpec)
				m.Get(gomockinternal.AContext(), &fakeGroupSpec).Return(resources.Group{}, notFoundError)
				s.DeleteLongRunningOperationState("test-group", ServiceName, infrav1.DeleteFuture)
				s.UpdateDeleteStatus(infrav1.ResourceGroupReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "error occurs when deleting resource group",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.GroupSpec().AnyTimes().Return(&fakeGroupSpec)
				m.Get(gomockinternal.AContext(), &fakeGroupSpec).Return(sampleManagedGroup, nil)
				s.ClusterName().Return("test-cluster")
				r.DeleteResource(gomockinternal.AContext(), &fakeGroupSpec, ServiceName).Return(internalError)
				s.UpdateDeleteStatus(infrav1.ResourceGroupReadyCondition, ServiceName, gomockinternal.ErrStrEq("#: Internal Server Error: StatusCode=500"))
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
			scopeMock := mock_groups.NewMockGroupScope(mockCtrl)
			clientMock := mock_groups.NewMockclient(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				client:     clientMock,
				Reconciler: asyncMock,
			}

			err := s.Delete(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

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
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
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
	internalError  = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")
	notFoundError  = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not Found")
	errCtxExceeded = errors.New("ctx exceeded")
	fakeFuture     = infrav1.Future{
		Type:          infrav1.DeleteFuture,
		ServiceName:   serviceName,
		Name:          "test-group",
		ResourceGroup: "test-group",
		Data:          "eyJtZXRob2QiOiJERUxFVEUiLCJwb2xsaW5nTWV0aG9kIjoiTG9jYXRpb24iLCJscm9TdGF0ZSI6IkluUHJvZ3Jlc3MifQ==",
	}
	sampleManagedGroup = resources.Group{
		Name:       to.StringPtr("test-group"),
		Location:   to.StringPtr("test-location"),
		Properties: &resources.GroupProperties{},
		Tags:       map[string]*string{"sigs.k8s.io_cluster-api-provider-azure_cluster_test-cluster": to.StringPtr("owned")},
	}
	sampleBYOGroup = resources.Group{
		Name:       to.StringPtr("test-group"),
		Location:   to.StringPtr("test-location"),
		Properties: &resources.GroupProperties{},
		Tags:       map[string]*string{"foo": to.StringPtr("bar")},
	}
)

func TestReconcileGroups(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder)
	}{
		{
			name:          "create group succeeds",
			expectedError: "",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder) {
				s.GroupSpec().Return(&fakeGroupSpec)
				s.GetLongRunningOperationState("test-group", serviceName)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakeGroupSpec).Return(nil, nil, nil)
				s.UpdatePutStatus(infrav1.ResourceGroupReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "create resource group fails",
			expectedError: "failed to create resource test-group/test-group (service: group): #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder) {
				s.GroupSpec().Return(&fakeGroupSpec)
				s.GetLongRunningOperationState("test-group", serviceName)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), &fakeGroupSpec).Return(nil, nil, internalError)
				s.UpdatePutStatus(infrav1.ResourceGroupReadyCondition, serviceName, gomockinternal.ErrStrEq(fmt.Sprintf("failed to create resource test-group/test-group (service: group): %s", internalError.Error())))
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

func TestDeleteGroups(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder)
	}{
		{
			name:          "long running delete operation is done",
			expectedError: "",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder) {
				s.GroupSpec().AnyTimes().Return(&fakeGroupSpec)
				m.Get(gomockinternal.AContext(), "test-group").Return(sampleManagedGroup, nil)
				s.ClusterName().Return("test-cluster")
				s.GetLongRunningOperationState("test-group", serviceName).Times(2).Return(&fakeFuture)
				m.IsDone(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(true, nil)
				m.Result(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{}), infrav1.DeleteFuture).Return(nil, nil)
				s.DeleteLongRunningOperationState("test-group", serviceName)
				s.UpdateDeleteStatus(infrav1.ResourceGroupReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "long running delete operation is not done",
			expectedError: "operation type DELETE on Azure resource test-group/test-group is not done. Object will be requeued after 15s",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder) {
				s.GroupSpec().AnyTimes().Return(&fakeGroupSpec)
				m.Get(gomockinternal.AContext(), "test-group").Return(sampleManagedGroup, nil)
				s.ClusterName().Return("test-cluster")
				s.GetLongRunningOperationState("test-group", serviceName).Times(2).Return(&fakeFuture)
				m.IsDone(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(false, nil)
				s.UpdateDeleteStatus(infrav1.ResourceGroupReadyCondition, serviceName, gomockinternal.ErrStrEq("operation type DELETE on Azure resource test-group/test-group is not done. Object will be requeued after 15s"))
			},
		},
		{
			name:          "resource group is not managed by capz",
			expectedError: azure.ErrNotOwned.Error(),
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder) {
				s.GroupSpec().AnyTimes().Return(&fakeGroupSpec)
				m.Get(gomockinternal.AContext(), "test-group").Return(sampleBYOGroup, nil)
				s.ClusterName().Return("test-cluster")
			},
		},
		{
			name:          "fail to check if resource group is managed",
			expectedError: "could not get resource group management state",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder) {
				s.GroupSpec().AnyTimes().Return(&fakeGroupSpec)
				m.Get(gomockinternal.AContext(), "test-group").Return(resources.Group{}, internalError)
			},
		},
		{
			name:          "resource group doesn't exist",
			expectedError: "",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder) {
				s.GroupSpec().AnyTimes().Return(&fakeGroupSpec)
				m.Get(gomockinternal.AContext(), "test-group").Return(resources.Group{}, notFoundError)
				s.DeleteLongRunningOperationState("test-group", serviceName)
				s.UpdateDeleteStatus(infrav1.ResourceGroupReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "error occurs when deleting resource group",
			expectedError: "failed to delete resource test-group/test-group (service: group): #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder) {
				s.GroupSpec().AnyTimes().Return(&fakeGroupSpec)
				s.GetLongRunningOperationState("test-group", serviceName).Return(nil)
				m.Get(gomockinternal.AContext(), "test-group").Return(sampleManagedGroup, nil)
				s.ClusterName().Return("test-cluster")
				m.DeleteAsync(gomockinternal.AContext(), &fakeGroupSpec).Return(nil, internalError)
				s.UpdateDeleteStatus(infrav1.ResourceGroupReadyCondition, serviceName, gomockinternal.ErrStrEq("failed to delete resource test-group/test-group (service: group): #: Internal Server Error: StatusCode=500"))
			},
		},
		{
			name:          "context deadline exceeded while deleting resource group",
			expectedError: "operation type DELETE on Azure resource test-group/test-group is not done. Object will be requeued after 15s",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder) {
				s.GroupSpec().AnyTimes().Return(&fakeGroupSpec)
				s.GetLongRunningOperationState("test-group", serviceName).Return(nil)
				m.Get(gomockinternal.AContext(), "test-group").Return(sampleManagedGroup, nil)
				s.ClusterName().Return("test-cluster")
				m.DeleteAsync(gomockinternal.AContext(), &fakeGroupSpec).Return(&azureautorest.Future{}, errCtxExceeded)
				s.SetLongRunningOperationState(gomock.AssignableToTypeOf(&infrav1.Future{}))
				s.UpdateDeleteStatus(infrav1.ResourceGroupReadyCondition, serviceName, gomockinternal.ErrStrEq("operation type DELETE on Azure resource test-group/test-group is not done. Object will be requeued after 15s"))
			},
		},
		{
			name:          "delete the resource group successfully",
			expectedError: "",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, m *mock_groups.MockclientMockRecorder) {
				s.GroupSpec().AnyTimes().Return(&fakeGroupSpec)
				s.GetLongRunningOperationState("test-group", serviceName).Return(nil)
				m.Get(gomockinternal.AContext(), "test-group").Return(sampleManagedGroup, nil)
				s.ClusterName().Return("test-cluster")
				m.DeleteAsync(gomockinternal.AContext(), &fakeGroupSpec).Return(nil, nil)
				s.UpdateDeleteStatus(infrav1.ResourceGroupReadyCondition, serviceName, nil)
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

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				client: clientMock,
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

/*
Copyright 2021 The Kubernetes Authors.

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

package async

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2/klogr"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	validCreateFuture = infrav1.Future{
		Type:          infrav1.PutFuture,
		ServiceName:   "test-service",
		Name:          "test-resource",
		ResourceGroup: "test-group",
		Data:          "eyJtZXRob2QiOiJQVVQiLCJwb2xsaW5nTWV0aG9kIjoiTG9jYXRpb24iLCJscm9TdGF0ZSI6IkluUHJvZ3Jlc3MifQ==",
	}
	validDeleteFuture = infrav1.Future{
		Type:          infrav1.DeleteFuture,
		ServiceName:   "test-service",
		Name:          "test-resource",
		ResourceGroup: "test-group",
		Data:          "eyJtZXRob2QiOiJERUxFVEUiLCJwb2xsaW5nTWV0aG9kIjoiTG9jYXRpb24iLCJscm9TdGF0ZSI6IkluUHJvZ3Jlc3MifQ==",
	}
	invalidFuture = infrav1.Future{
		Type:          infrav1.DeleteFuture,
		ServiceName:   "test-service",
		Name:          "test-resource",
		ResourceGroup: "test-group",
		Data:          "ZmFrZSBiNjQgZnV0dXJlIGRhdGEK",
	}
	fakeError      = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")
	errCtxExceeded = errors.New("ctx exceeded")
)

// TestProcessOngoingOperation tests the processOngoingOperation function.
func TestProcessOngoingOperation(t *testing.T) {
	testcases := []struct {
		name          string
		resourceName  string
		serviceName   string
		expectedError string
		expect        func(s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockFutureHandlerMockRecorder)
	}{
		{
			name:          "no future data stored in status",
			expectedError: "",
			resourceName:  "test-resource",
			serviceName:   "test-service",
			expect: func(s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockFutureHandlerMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.GetLongRunningOperationState("test-resource", "test-service").Return(nil)
			},
		},
		{
			name:          "future data is not valid",
			expectedError: "could not decode future data, resetting long-running operation state",
			resourceName:  "test-resource",
			serviceName:   "test-service",
			expect: func(s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockFutureHandlerMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.GetLongRunningOperationState("test-resource", "test-service").Return(&invalidFuture)
				s.DeleteLongRunningOperationState("test-resource", "test-service")
			},
		},
		{
			name:          "fail to check if ongoing operation is done",
			expectedError: "failed checking if the operation was complete",
			resourceName:  "test-resource",
			serviceName:   "test-service",
			expect: func(s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockFutureHandlerMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.GetLongRunningOperationState("test-resource", "test-service").Return(&validDeleteFuture)
				c.IsDone(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(false, fakeError)
			},
		},
		{
			name:          "ongoing operation is not done",
			expectedError: "operation type DELETE on Azure resource test-group/test-resource is not done",
			resourceName:  "test-resource",
			serviceName:   "test-service",
			expect: func(s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockFutureHandlerMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.GetLongRunningOperationState("test-resource", "test-service").Return(&validDeleteFuture)
				c.IsDone(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(false, nil)
			},
		},
		{
			name:          "operation is done",
			expectedError: "",
			resourceName:  "test-resource",
			serviceName:   "test-service",
			expect: func(s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockFutureHandlerMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.GetLongRunningOperationState("test-resource", "test-service").Return(&validDeleteFuture)
				c.IsDone(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(true, nil)
				s.DeleteLongRunningOperationState("test-resource", "test-service")
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
			scopeMock := mock_async.NewMockFutureScope(mockCtrl)
			clientMock := mock_async.NewMockFutureHandler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			err := processOngoingOperation(context.TODO(), scopeMock, clientMock, tc.resourceName, tc.serviceName)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

// TestCreateResource tests the CreateResource function.
func TestCreateResource(t *testing.T) {
	testcases := []struct {
		name          string
		serviceName   string
		expectedError string
		expect        func(s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockCreatorMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder)
	}{
		{
			name:          "create operation is already in progress",
			expectedError: "transient reconcile error occurred: operation type PUT on Azure resource test-group/test-resource is not done. Object will be requeued after 15s",
			serviceName:   "test-service",
			expect: func(s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockCreatorMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service").Times(2).Return(&validCreateFuture)
				c.IsDone(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(false, nil)
			},
		},
		{
			name:          "create async returns success",
			expectedError: "",
			serviceName:   "test-service",
			expect: func(s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockCreatorMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service").Return(nil)
				c.CreateOrUpdateAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(&mock_azure.MockResourceSpecGetter{})).Return(nil, nil)
			},
		},
		{
			name:          "error occurs while running async create",
			expectedError: "failed to create resource test-group/test-resource (service: test-service)",
			serviceName:   "test-service",
			expect: func(s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockCreatorMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service").Return(nil)
				c.CreateOrUpdateAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(&mock_azure.MockResourceSpecGetter{})).Return(nil, fakeError)
			},
		},
		{
			name:          "create async exits before completing",
			expectedError: "transient reconcile error occurred: operation type PUT on Azure resource test-group/test-resource is not done. Object will be requeued after 15s",
			serviceName:   "test-service",
			expect: func(s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockCreatorMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service").Return(nil)
				c.CreateOrUpdateAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(&mock_azure.MockResourceSpecGetter{})).Return(&azureautorest.Future{}, errCtxExceeded)
				s.SetLongRunningOperationState(gomock.AssignableToTypeOf(&infrav1.Future{}))
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
			scopeMock := mock_async.NewMockFutureScope(mockCtrl)
			clientMock := mock_async.NewMockCreator(mockCtrl)
			specMock := mock_azure.NewMockResourceSpecGetter(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), specMock.EXPECT())

			err := CreateResource(context.TODO(), scopeMock, clientMock, specMock, tc.serviceName)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

// TestDeleteResource tests the DeleteResource function.
func TestDeleteResource(t *testing.T) {
	testcases := []struct {
		name          string
		serviceName   string
		expectedError string
		expect        func(s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockDeleterMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder)
	}{
		{
			name:          "delete operation is already in progress",
			expectedError: "transient reconcile error occurred: operation type DELETE on Azure resource test-group/test-resource is not done. Object will be requeued after 15s",
			serviceName:   "test-service",
			expect: func(s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockDeleterMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service").Times(2).Return(&validDeleteFuture)
				c.IsDone(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(false, nil)
			},
		},
		{
			name:          "delete async returns success",
			expectedError: "",
			serviceName:   "test-service",
			expect: func(s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockDeleterMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service").Return(nil)
				c.DeleteAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(&mock_azure.MockResourceSpecGetter{})).Return(nil, nil)
			},
		},
		{
			name:          "error occurs while running async delete",
			expectedError: "failed to delete resource test-group/test-resource (service: test-service)",
			serviceName:   "test-service",
			expect: func(s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockDeleterMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service").Return(nil)
				c.DeleteAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(&mock_azure.MockResourceSpecGetter{})).Return(nil, fakeError)
			},
		},
		{
			name:          "delete async exits before completing",
			expectedError: "transient reconcile error occurred: operation type DELETE on Azure resource test-group/test-resource is not done. Object will be requeued after 15s",
			serviceName:   "test-service",
			expect: func(s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockDeleterMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service").Return(nil)
				c.DeleteAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(&mock_azure.MockResourceSpecGetter{})).Return(&azureautorest.Future{}, errCtxExceeded)
				s.SetLongRunningOperationState(gomock.AssignableToTypeOf(&infrav1.Future{}))
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
			scopeMock := mock_async.NewMockFutureScope(mockCtrl)
			clientMock := mock_async.NewMockDeleter(mockCtrl)
			specMock := mock_azure.NewMockResourceSpecGetter(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), specMock.EXPECT())

			err := DeleteResource(context.TODO(), scopeMock, clientMock, specMock, tc.serviceName)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

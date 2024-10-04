/*
Copyright 2023 The Kubernetes Authors.

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
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
)

func TestServiceCreateOrUpdateResource(t *testing.T) {
	testcases := []struct {
		name           string
		serviceName    string
		expectedError  string
		expectedResult interface{}
		expect         func(g *WithT, s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockCreatorMockRecorder[MockCreator], r *mock_azure.MockResourceSpecGetterMockRecorder)
	}{
		{
			name:          "invalid future",
			serviceName:   serviceName,
			expectedError: "could not decode future data, resetting long-running operation state",
			expect: func(g *WithT, s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockCreatorMockRecorder[MockCreator], r *mock_azure.MockResourceSpecGetterMockRecorder) {
				gomock.InOrder(
					r.ResourceName().Return(resourceName),
					r.ResourceGroupName().Return(resourceGroupName),
					s.GetLongRunningOperationState(resourceName, serviceName, infrav1.PutFuture).Return(invalidPutFuture),
					s.DeleteLongRunningOperationState(resourceName, serviceName, infrav1.PutFuture),
				)
			},
		},
		{
			name:           "operation completed",
			serviceName:    serviceName,
			expectedError:  "",
			expectedResult: fakeResource,
			expect: func(g *WithT, s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockCreatorMockRecorder[MockCreator], r *mock_azure.MockResourceSpecGetterMockRecorder) {
				gomock.InOrder(
					r.ResourceName().Return(resourceName),
					r.ResourceGroupName().Return(resourceGroupName),
					s.GetLongRunningOperationState(resourceName, serviceName, infrav1.PutFuture).Return(validPutFuture),
					c.CreateOrUpdateAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(azureResourceGetterType), resumeToken, gomock.Any()).Return(fakeResource, nil, nil),
					s.DeleteLongRunningOperationState(resourceName, serviceName, infrav1.PutFuture),
				)
			},
		},
		{
			name:          "operation in progress",
			serviceName:   serviceName,
			expectedError: "operation type PUT on Azure resource mock-resourcegroup/mock-resource is not done. Object will be requeued after 15s",
			expect: func(g *WithT, s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockCreatorMockRecorder[MockCreator], r *mock_azure.MockResourceSpecGetterMockRecorder) {
				gomock.InOrder(
					r.ResourceName().Return(resourceName),
					r.ResourceGroupName().Return(resourceGroupName),
					s.GetLongRunningOperationState(resourceName, serviceName, infrav1.PutFuture).Return(validPutFuture),
					c.CreateOrUpdateAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(azureResourceGetterType), resumeToken, gomock.Any()).Return(nil, fakePoller[MockCreator](g, http.StatusAccepted), context.DeadlineExceeded),
					s.SetLongRunningOperationState(gomock.AssignableToTypeOf(&infrav1.Future{})),
					s.DefaultedReconcilerRequeue().Return(reconciler.DefaultReconcilerRequeue),
				)
			},
		},
		{
			name:          "operation failed",
			serviceName:   serviceName,
			expectedError: "failed to create or update resource mock-resourcegroup/mock-resource (service: mock-service): foo",
			expect: func(g *WithT, s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockCreatorMockRecorder[MockCreator], r *mock_azure.MockResourceSpecGetterMockRecorder) {
				gomock.InOrder(
					r.ResourceName().Return(resourceName),
					r.ResourceGroupName().Return(resourceGroupName),
					s.GetLongRunningOperationState(resourceName, serviceName, infrav1.PutFuture).Return(validPutFuture),
					c.CreateOrUpdateAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(azureResourceGetterType), resumeToken, gomock.Any()).Return(nil, fakePoller[MockCreator](g, http.StatusAccepted), errors.New("foo")),
					s.DeleteLongRunningOperationState(resourceName, serviceName, infrav1.PutFuture),
				)
			},
		},
		{
			name:          "get returns resource not found error",
			serviceName:   serviceName,
			expectedError: "operation type PUT on Azure resource mock-resourcegroup/mock-resource is not done. Object will be requeued after 15s",
			expect: func(g *WithT, s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockCreatorMockRecorder[MockCreator], r *mock_azure.MockResourceSpecGetterMockRecorder) {
				gomock.InOrder(
					r.ResourceName().Return(resourceName),
					r.ResourceGroupName().Return(resourceGroupName),
					s.GetLongRunningOperationState(resourceName, serviceName, infrav1.PutFuture).Return(nil),
					c.Get(gomockinternal.AContext(), gomock.AssignableToTypeOf(azureResourceGetterType)).Return(nil, &azcore.ResponseError{StatusCode: http.StatusNotFound}),
					r.Parameters(gomockinternal.AContext(), nil).Return(fakeParameters, nil),
					c.CreateOrUpdateAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(azureResourceGetterType), "", gomock.Any()).Return(nil, fakePoller[MockCreator](g, http.StatusAccepted), context.DeadlineExceeded),
					s.SetLongRunningOperationState(gomock.AssignableToTypeOf(&infrav1.Future{})),
					s.DefaultedReconcilerRequeue().Return(reconciler.DefaultReconcilerRequeue),
				)
			},
		},
		{
			name:          "get returns unexpected error",
			serviceName:   serviceName,
			expectedError: "failed to get existing resource mock-resourcegroup/mock-resource (service: mock-service): foo. Object will be requeued after 15s",
			expect: func(g *WithT, s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockCreatorMockRecorder[MockCreator], r *mock_azure.MockResourceSpecGetterMockRecorder) {
				gomock.InOrder(
					r.ResourceName().Return(resourceName),
					r.ResourceGroupName().Return(resourceGroupName),
					s.GetLongRunningOperationState(resourceName, serviceName, infrav1.PutFuture).Return(nil),
					c.Get(gomockinternal.AContext(), gomock.AssignableToTypeOf(azureResourceGetterType)).Return(nil, errors.New("foo")),
				)
			},
		},
		{
			name:           "parameters are nil: up to date",
			serviceName:    serviceName,
			expectedError:  "",
			expectedResult: fakeResource,
			expect: func(g *WithT, s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockCreatorMockRecorder[MockCreator], r *mock_azure.MockResourceSpecGetterMockRecorder) {
				gomock.InOrder(
					r.ResourceName().Return(resourceName),
					r.ResourceGroupName().Return(resourceGroupName),
					s.GetLongRunningOperationState(resourceName, serviceName, infrav1.PutFuture).Return(nil),
					c.Get(gomockinternal.AContext(), gomock.AssignableToTypeOf(azureResourceGetterType)).Return(fakeResource, nil),
					r.Parameters(gomockinternal.AContext(), fakeResource).Return(nil, nil),
				)
			},
		},
		{
			name:           "parameters returns error",
			serviceName:    serviceName,
			expectedError:  "failed to get desired parameters for resource mock-resourcegroup/mock-resource (service: mock-service): foo",
			expectedResult: nil,
			expect: func(g *WithT, s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockCreatorMockRecorder[MockCreator], r *mock_azure.MockResourceSpecGetterMockRecorder) {
				gomock.InOrder(
					r.ResourceName().Return(resourceName),
					r.ResourceGroupName().Return(resourceGroupName),
					s.GetLongRunningOperationState(resourceName, serviceName, infrav1.PutFuture).Return(nil),
					c.Get(gomockinternal.AContext(), gomock.AssignableToTypeOf(azureResourceGetterType)).Return(fakeResource, nil),
					r.Parameters(gomockinternal.AContext(), fakeResource).Return(nil, errors.New("foo")),
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
			scopeMock := mock_async.NewMockFutureScope(mockCtrl)
			creatorMock := mock_async.NewMockCreator[MockCreator](mockCtrl)
			svc := New[MockCreator, MockDeleter](scopeMock, creatorMock, nil)
			specMock := mock_azure.NewMockResourceSpecGetter(mockCtrl)

			tc.expect(g, scopeMock.EXPECT(), creatorMock.EXPECT(), specMock.EXPECT())

			result, err := svc.CreateOrUpdateResource(context.TODO(), specMock, serviceName)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				if tc.expectedResult != nil {
					g.Expect(result).To(Equal(tc.expectedResult))
				} else {
					g.Expect(result).To(BeNil())
				}
			}
		})
	}
}

func TestServiceDeleteResource(t *testing.T) {
	testcases := []struct {
		name           string
		serviceName    string
		expectedError  string
		expectedResult interface{}
		expect         func(g *GomegaWithT, s *mock_async.MockFutureScopeMockRecorder, d *mock_async.MockDeleterMockRecorder[MockDeleter], r *mock_azure.MockResourceSpecGetterMockRecorder)
	}{
		{
			name:          "invalid future",
			serviceName:   serviceName,
			expectedError: "could not decode future data",
			expect: func(_ *GomegaWithT, s *mock_async.MockFutureScopeMockRecorder, _ *mock_async.MockDeleterMockRecorder[MockDeleter], r *mock_azure.MockResourceSpecGetterMockRecorder) {
				gomock.InOrder(
					r.ResourceName().Return(resourceName),
					r.ResourceGroupName().Return(resourceGroupName),
					s.GetLongRunningOperationState(resourceName, serviceName, infrav1.DeleteFuture).Return(invalidDeleteFuture),
					s.DeleteLongRunningOperationState(resourceName, serviceName, infrav1.DeleteFuture),
				)
			},
		},
		{
			name:          "operation in progress",
			serviceName:   serviceName,
			expectedError: "operation type DELETE on Azure resource mock-resourcegroup/mock-resource is not done. Object will be requeued after 15s",
			expect: func(g *GomegaWithT, s *mock_async.MockFutureScopeMockRecorder, d *mock_async.MockDeleterMockRecorder[MockDeleter], r *mock_azure.MockResourceSpecGetterMockRecorder) {
				gomock.InOrder(
					r.ResourceName().Return(resourceName),
					r.ResourceGroupName().Return(resourceGroupName),
					s.GetLongRunningOperationState(resourceName, serviceName, infrav1.DeleteFuture).Return(validDeleteFuture),
					d.DeleteAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(azureResourceGetterType), gomock.Any()).Return(fakePoller[MockDeleter](g, http.StatusAccepted), context.DeadlineExceeded),
					s.SetLongRunningOperationState(gomock.AssignableToTypeOf(&infrav1.Future{})),
					s.DefaultedReconcilerRequeue().Return(reconciler.DefaultReconcilerRequeue),
				)
			},
		},
		{
			name:          "operation succeeds",
			serviceName:   serviceName,
			expectedError: "",
			expect: func(_ *GomegaWithT, s *mock_async.MockFutureScopeMockRecorder, d *mock_async.MockDeleterMockRecorder[MockDeleter], r *mock_azure.MockResourceSpecGetterMockRecorder) {
				gomock.InOrder(
					r.ResourceName().Return(resourceName),
					r.ResourceGroupName().Return(resourceGroupName),
					s.GetLongRunningOperationState(resourceName, serviceName, infrav1.DeleteFuture).Return(validDeleteFuture),
					d.DeleteAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(azureResourceGetterType), gomock.Any()).Return(nil, nil),
					s.DeleteLongRunningOperationState(resourceName, serviceName, infrav1.DeleteFuture),
				)
			},
		},
		{
			name:          "operation fails",
			serviceName:   serviceName,
			expectedError: "failed to delete resource mock-resourcegroup/mock-resource (service: mock-service): foo",
			expect: func(g *GomegaWithT, s *mock_async.MockFutureScopeMockRecorder, d *mock_async.MockDeleterMockRecorder[MockDeleter], r *mock_azure.MockResourceSpecGetterMockRecorder) {
				gomock.InOrder(
					r.ResourceName().Return(resourceName),
					r.ResourceGroupName().Return(resourceGroupName),
					s.GetLongRunningOperationState(resourceName, serviceName, infrav1.DeleteFuture).Return(validDeleteFuture),
					d.DeleteAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(azureResourceGetterType), gomock.Any()).Return(fakePoller[MockDeleter](g, http.StatusAccepted), errors.New("foo")),
					s.DeleteLongRunningOperationState(resourceName, serviceName, infrav1.DeleteFuture),
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
			scopeMock := mock_async.NewMockFutureScope(mockCtrl)
			deleterMock := mock_async.NewMockDeleter[MockDeleter](mockCtrl)
			svc := New[MockCreator, MockDeleter](scopeMock, nil, deleterMock)
			specMock := mock_azure.NewMockResourceSpecGetter(mockCtrl)

			tc.expect(g, scopeMock.EXPECT(), deleterMock.EXPECT(), specMock.EXPECT())

			err := svc.DeleteResource(context.TODO(), specMock, tc.serviceName)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

const (
	resourceGroupName  = "mock-resourcegroup"
	resourceName       = "mock-resource"
	serviceName        = "mock-service"
	resumeToken        = "mock-resume-token"
	invalidResumeToken = "!invalid-resume-token"
)

var (
	validPutFuture = &infrav1.Future{
		Type:          infrav1.PutFuture,
		ServiceName:   serviceName,
		Name:          resourceName,
		ResourceGroup: resourceGroupName,
		Data:          base64.URLEncoding.EncodeToString([]byte(resumeToken)),
	}
	invalidPutFuture = &infrav1.Future{
		Type:          infrav1.PutFuture,
		ServiceName:   serviceName,
		Name:          resourceName,
		ResourceGroup: resourceGroupName,
		Data:          invalidResumeToken,
	}
	validDeleteFuture = &infrav1.Future{
		Type:          infrav1.DeleteFuture,
		ServiceName:   serviceName,
		Name:          resourceName,
		ResourceGroup: resourceGroupName,
		Data:          base64.URLEncoding.EncodeToString([]byte(resumeToken)),
	}
	invalidDeleteFuture = &infrav1.Future{
		Type:          infrav1.DeleteFuture,
		ServiceName:   serviceName,
		Name:          resourceName,
		ResourceGroup: resourceGroupName,
		Data:          invalidResumeToken,
	}
	fakeResource            = armresources.GenericResource{}
	fakeParameters          = armresources.GenericResource{}
	azureResourceGetterType = reflect.TypeOf((*azure.ResourceSpecGetter)(nil)).Elem()
)

func fakePoller[T any](g *GomegaWithT, statusCode int) *runtime.Poller[T] {
	response := &http.Response{
		Body: io.NopCloser(strings.NewReader("")),
		Request: &http.Request{
			Method: http.MethodPut,
			URL:    &url.URL{Path: "/"},
		},
		StatusCode: statusCode,
	}
	pipeline := runtime.NewPipeline("testmodule", "v0.1.0", runtime.PipelineOptions{}, nil)
	poller, err := runtime.NewPoller[T](response, pipeline, nil)
	g.Expect(err).NotTo(HaveOccurred())
	return poller
}

type MockCreator struct{}
type MockDeleter struct{}

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
	"time"

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
		name            string
		serviceName     string
		expectedError   string
		expectedResult  any
		expectTransient bool
		expect          func(g *WithT, s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockCreatorMockRecorder[MockCreator], r *mock_azure.MockResourceSpecGetterMockRecorder)
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
			name:            "operation in progress",
			serviceName:     serviceName,
			expectedError:   "operation type PUT on Azure resource mock-resourcegroup/mock-resource is not done. Object will be requeued after 15s",
			expectTransient: true,
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
			name:          "operation failed with a non-retryable error",
			serviceName:   serviceName,
			expectedError: "failed to create or update resource mock-resourcegroup/mock-resource (service: mock-service): foo",
			// A plain, non-retryable error is returned unwrapped (not classified
			// transient); UpdatePutStatus reports it as a VM provisioning failure,
			// though controller-runtime may still retry the returned error with backoff.
			expectTransient: false,
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
			name:          "operation failed with a non-retryable status code",
			serviceName:   serviceName,
			expectedError: "failed to create or update resource mock-resourcegroup/mock-resource (service: mock-service)",
			// A 400 is not in the retryable set, so it is returned unwrapped (not
			// classified transient).
			expectTransient: false,
			expect: func(g *WithT, s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockCreatorMockRecorder[MockCreator], r *mock_azure.MockResourceSpecGetterMockRecorder) {
				gomock.InOrder(
					r.ResourceName().Return(resourceName),
					r.ResourceGroupName().Return(resourceGroupName),
					s.GetLongRunningOperationState(resourceName, serviceName, infrav1.PutFuture).Return(validPutFuture),
					c.CreateOrUpdateAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(azureResourceGetterType), resumeToken, gomock.Any()).Return(nil, fakePoller[MockCreator](g, http.StatusAccepted), &azcore.ResponseError{StatusCode: http.StatusBadRequest}),
					s.DeleteLongRunningOperationState(resourceName, serviceName, infrav1.PutFuture),
				)
			},
		},
		{
			name:          "operation failed with a throttling error is transient",
			serviceName:   serviceName,
			expectedError: "failed to create or update resource mock-resourcegroup/mock-resource (service: mock-service)",
			// A 429 that outlived the SDK's own retries is wrapped as transient so
			// it is requeued rather than reported as a terminal provisioning failure.
			expectTransient: true,
			expect: func(g *WithT, s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockCreatorMockRecorder[MockCreator], r *mock_azure.MockResourceSpecGetterMockRecorder) {
				gomock.InOrder(
					r.ResourceName().Return(resourceName),
					r.ResourceGroupName().Return(resourceGroupName),
					s.GetLongRunningOperationState(resourceName, serviceName, infrav1.PutFuture).Return(validPutFuture),
					c.CreateOrUpdateAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(azureResourceGetterType), resumeToken, gomock.Any()).Return(nil, fakePoller[MockCreator](g, http.StatusAccepted), &azcore.ResponseError{StatusCode: http.StatusTooManyRequests}),
					s.DeleteLongRunningOperationState(resourceName, serviceName, infrav1.PutFuture),
				)
			},
		},
		{
			name:          "operation timed out before a poller is available is transient",
			serviceName:   serviceName,
			expectedError: "failed to create or update resource mock-resourcegroup/mock-resource (service: mock-service): context deadline exceeded. Object will be requeued after 15s",
			// A timeout with no poller cannot be resumed, so it is retried as transient.
			expectTransient: true,
			expect: func(g *WithT, s *mock_async.MockFutureScopeMockRecorder, c *mock_async.MockCreatorMockRecorder[MockCreator], r *mock_azure.MockResourceSpecGetterMockRecorder) {
				gomock.InOrder(
					r.ResourceName().Return(resourceName),
					r.ResourceGroupName().Return(resourceGroupName),
					s.GetLongRunningOperationState(resourceName, serviceName, infrav1.PutFuture).Return(validPutFuture),
					c.CreateOrUpdateAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(azureResourceGetterType), resumeToken, gomock.Any()).Return(nil, nil, context.DeadlineExceeded),
					s.DeleteLongRunningOperationState(resourceName, serviceName, infrav1.PutFuture),
				)
			},
		},
		{
			name:            "get returns resource not found error",
			serviceName:     serviceName,
			expectedError:   "operation type PUT on Azure resource mock-resourcegroup/mock-resource is not done. Object will be requeued after 15s",
			expectTransient: true,
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
			name:            "get returns unexpected error",
			serviceName:     serviceName,
			expectedError:   "failed to get existing resource mock-resourcegroup/mock-resource (service: mock-service): foo. Object will be requeued after 15s",
			expectTransient: true,
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

			result, err := svc.CreateOrUpdateResource(t.Context(), specMock, serviceName)
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
			g.Expect(isTransientReconcileError(err)).To(Equal(tc.expectTransient))
		})
	}
}

// isTransientReconcileError reports whether err is (or wraps) a transient
// azure.ReconcileError. The controller requeues such errors with an Azure-aware
// RequeueAfter. An error that is not classified this way (e.g. a plain wrapped
// ResponseError) is still returned to controller-runtime and may be retried with
// backoff; this helper only detects the explicit transient marker.
func isTransientReconcileError(err error) bool {
	var reconcileErr azure.ReconcileError
	return errors.As(err, &reconcileErr) && reconcileErr.IsTransient()
}

// newRetryAfterError returns a ResponseError with the given status code and
// Retry-After header value. An empty retryAfter omits the header.
func newRetryAfterError(statusCode int, retryAfter string) *azcore.ResponseError {
	header := http.Header{}
	if retryAfter != "" {
		header.Set("Retry-After", retryAfter)
	}
	return &azcore.ResponseError{
		StatusCode: statusCode,
		RawResponse: &http.Response{
			StatusCode: statusCode,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     header,
		},
	}
}

// elapsedHTTPDate is a Retry-After HTTP-date that is always in the past.
const elapsedHTTPDate = "Mon, 02 Jan 2006 15:04:05 GMT"

func TestGetRetryAfterFromError(t *testing.T) {
	testcases := []struct {
		name     string
		err      error
		expected time.Duration
	}{
		{
			name:     "not a ResponseError",
			err:      errors.New("foo"),
			expected: reconciler.DefaultReconcilerRequeue,
		},
		{
			name:     "Retry-After in seconds",
			err:      newRetryAfterError(http.StatusTooManyRequests, "30"),
			expected: 30 * time.Second,
		},
		{
			name:     "Retry-After of zero falls back to the default",
			err:      newRetryAfterError(http.StatusTooManyRequests, "0"),
			expected: reconciler.DefaultReconcilerRequeue,
		},
		{
			name:     "negative Retry-After falls back to the default",
			err:      newRetryAfterError(http.StatusTooManyRequests, "-5"),
			expected: reconciler.DefaultReconcilerRequeue,
		},
		{
			name:     "elapsed Retry-After HTTP-date falls back to the default",
			err:      newRetryAfterError(http.StatusServiceUnavailable, elapsedHTTPDate),
			expected: reconciler.DefaultReconcilerRequeue,
		},
		{
			name:     "429 without Retry-After",
			err:      newRetryAfterError(http.StatusTooManyRequests, ""),
			expected: reconciler.DefaultHTTP429RetryAfter,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(GetRetryAfterFromError(tc.err)).To(Equal(tc.expected))
		})
	}
}

func TestServiceCreateOrUpdateResourceRetryAfter(t *testing.T) {
	testcases := []struct {
		name                 string
		retryAfter           string
		expectedRequeueAfter time.Duration
	}{
		{
			name:                 "positive Retry-After is preserved",
			retryAfter:           "30",
			expectedRequeueAfter: 30 * time.Second,
		},
		{
			name:                 "Retry-After of zero requeues after the default",
			retryAfter:           "0",
			expectedRequeueAfter: reconciler.DefaultReconcilerRequeue,
		},
		{
			name:                 "negative Retry-After requeues after the default",
			retryAfter:           "-5",
			expectedRequeueAfter: reconciler.DefaultReconcilerRequeue,
		},
		{
			name:                 "elapsed Retry-After HTTP-date requeues after the default",
			retryAfter:           elapsedHTTPDate,
			expectedRequeueAfter: reconciler.DefaultReconcilerRequeue,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			scopeMock := mock_async.NewMockFutureScope(mockCtrl)
			creatorMock := mock_async.NewMockCreator[MockCreator](mockCtrl)
			svc := New[MockCreator, MockDeleter](scopeMock, creatorMock, nil)
			specMock := mock_azure.NewMockResourceSpecGetter(mockCtrl)

			s, c, r := scopeMock.EXPECT(), creatorMock.EXPECT(), specMock.EXPECT()
			gomock.InOrder(
				r.ResourceName().Return(resourceName),
				r.ResourceGroupName().Return(resourceGroupName),
				s.GetLongRunningOperationState(resourceName, serviceName, infrav1.PutFuture).Return(validPutFuture),
				c.CreateOrUpdateAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(azureResourceGetterType), resumeToken, gomock.Any()).Return(nil, fakePoller[MockCreator](g, http.StatusAccepted), newRetryAfterError(http.StatusTooManyRequests, tc.retryAfter)),
				s.DeleteLongRunningOperationState(resourceName, serviceName, infrav1.PutFuture),
			)

			_, err := svc.CreateOrUpdateResource(t.Context(), specMock, serviceName)
			g.Expect(err).To(HaveOccurred())
			var reconcileErr azure.ReconcileError
			g.Expect(errors.As(err, &reconcileErr)).To(BeTrue())
			g.Expect(reconcileErr.IsTransient()).To(BeTrue())
			g.Expect(reconcileErr.RequeueAfter()).To(BeNumerically(">", 0))
			g.Expect(reconcileErr.RequeueAfter()).To(Equal(tc.expectedRequeueAfter))
		})
	}
}

func TestServiceDeleteResource(t *testing.T) {
	testcases := []struct {
		name           string
		serviceName    string
		expectedError  string
		expectedResult any
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

			err := svc.DeleteResource(t.Context(), specMock, tc.serviceName)
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
	azureResourceGetterType = reflect.TypeFor[azure.ResourceSpecGetter]()
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

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

package asyncpoller

/*
import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/asyncpoller/mock_asyncpoller"
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
	fakeExistingResource   = resources.GenericResource{}
	fakeResourceParameters = resources.GenericResource{}
	fakeInternalError      = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error")
	fakeNotFoundError      = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusNotFound}, "Not Found")
	errCtxExceeded         = errors.New("ctx exceeded")
)

// TestProcessOngoingOperation tests the processOngoingOperation function.
func TestProcessOngoingOperation(t *testing.T) {
	testcases := []struct {
		name           string
		resourceName   string
		serviceName    string
		futureType     string
		expectedError  string
		expectedResult interface{}
		expect         func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockFutureHandlerMockRecorder)
	}{
		{
			name:          "no future data stored in status",
			expectedError: "",
			resourceName:  "test-resource",
			serviceName:   "test-service",
			futureType:    infrav1.DeleteFuture,
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockFutureHandlerMockRecorder) {
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.DeleteFuture).Return(nil)
			},
		},
		{
			name:          "future data is not valid",
			expectedError: "could not decode future data, resetting long-running operation state",
			resourceName:  "test-resource",
			serviceName:   "test-service",
			futureType:    infrav1.DeleteFuture,
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockFutureHandlerMockRecorder) {
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.DeleteFuture).Return(&invalidFuture)
				s.DeleteLongRunningOperationState("test-resource", "test-service", infrav1.DeleteFuture)
			},
		},
		{
			name:          "fail to check if ongoing operation is done",
			expectedError: "failed checking if the operation was complete",
			resourceName:  "test-resource",
			serviceName:   "test-service",
			futureType:    infrav1.DeleteFuture,
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockFutureHandlerMockRecorder) {
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.DeleteFuture).Return(&validDeleteFuture)
				c.IsDone(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(false, fakeInternalError)
			},
		},
		{
			name:          "ongoing operation is not done",
			expectedError: "operation type DELETE on Azure resource test-group/test-resource is not done",
			resourceName:  "test-resource",
			serviceName:   "test-service",
			futureType:    infrav1.DeleteFuture,
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockFutureHandlerMockRecorder) {
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.DeleteFuture).Return(&validDeleteFuture)
				c.IsDone(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(false, nil)
			},
		},
		{
			name:           "operation is done",
			expectedError:  "",
			expectedResult: &fakeExistingResource,
			resourceName:   "test-resource",
			serviceName:    "test-service",
			futureType:     infrav1.DeleteFuture,
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockFutureHandlerMockRecorder) {
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.DeleteFuture).Return(&validDeleteFuture)
				c.IsDone(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(true, nil)
				c.Result(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(&fakeExistingResource, nil)
				s.DeleteLongRunningOperationState("test-resource", "test-service", infrav1.DeleteFuture)
			},
		},
		{
			name:           "resource was deleted by an external process",
			expectedError:  fakeNotFoundError.Error(),
			expectedResult: nil,
			resourceName:   "test-resource",
			serviceName:    "test-service",
			futureType:     infrav1.DeleteFuture,
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockFutureHandlerMockRecorder) {
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.DeleteFuture).Return(&validDeleteFuture)
				c.IsDone(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(true, nil)
				c.Result(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(nil, fakeNotFoundError)
				s.DeleteLongRunningOperationState("test-resource", "test-service", infrav1.DeleteFuture)
			},
		},
		{
			name:           "failed to get resulting resource",
			expectedError:  fakeInternalError.Error(),
			expectedResult: nil,
			resourceName:   "test-resource",
			serviceName:    "test-service",
			futureType:     infrav1.DeleteFuture,
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockFutureHandlerMockRecorder) {
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.DeleteFuture).Return(&validDeleteFuture)
				c.IsDone(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(true, nil)
				c.Result(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(nil, fakeInternalError)
				s.DeleteLongRunningOperationState("test-resource", "test-service", infrav1.DeleteFuture)
			},
		},
		{
			name:           "terminal failure with IsDone error",
			expectedError:  fakeInternalError.Error(),
			expectedResult: nil,
			resourceName:   "test-resource",
			serviceName:    "test-service",
			futureType:     infrav1.DeleteFuture,
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockFutureHandlerMockRecorder) {
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.DeleteFuture).Return(&validDeleteFuture)
				c.IsDone(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(true, errors.New("IsDone error"))
				c.Result(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(nil, fakeInternalError)
				s.DeleteLongRunningOperationState("test-resource", "test-service", infrav1.DeleteFuture)
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
			scopeMock := mock_asyncpoller.NewMockFutureScope(mockCtrl)
			clientMock := mock_asyncpoller.NewMockFutureHandler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			result, err := processOngoingOperation[mock_asyncpoller.MockFutureHandler](context.TODO(), scopeMock, clientMock, tc.resourceName, tc.serviceName, tc.futureType)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			if tc.expectedResult != nil {
				g.Expect(result).To(Equal(tc.expectedResult))
			} else {
				g.Expect(result).To(BeNil())
			}
		})
	}
}

// TestCreateOrUpdateResource tests the CreateOrUpdateResource function.
func TestCreateOrUpdateResource(t *testing.T) {
	testcases := []struct {
		name           string
		serviceName    string
		expectedError  string
		expectedResult interface{}
		expect         func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockCreatorMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder)
	}{
		{
			name:          "create operation is already in progress",
			expectedError: "operation type PUT on Azure resource test-group/test-resource is not done. Object will be requeued after 15s",
			serviceName:   "test-service",
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockCreatorMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.PutFuture).Times(2).Return(&validCreateFuture)
				c.IsDone(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(false, nil)
			},
		},
		{
			name:           "create async returns success",
			expectedError:  "",
			expectedResult: "test-resource",
			serviceName:    "test-service",
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockCreatorMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.PutFuture).Return(nil)
				c.Get(gomockinternal.AContext(), gomock.AssignableToTypeOf(&mock_azure.MockResourceSpecGetter{})).Return(&fakeExistingResource, nil)
				r.Parameters(gomockinternal.AContext(), &fakeExistingResource).Return(&fakeResourceParameters, nil)
				c.CreateOrUpdateAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(&mock_azure.MockResourceSpecGetter{}), &fakeResourceParameters).Return("test-resource", nil, nil)
			},
		},
		{
			name:          "error occurs while running async get",
			expectedError: "failed to get existing resource test-group/test-resource (service: test-service)",
			serviceName:   "test-service",
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockCreatorMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.PutFuture).Return(nil)
				c.Get(gomockinternal.AContext(), gomock.AssignableToTypeOf(&mock_azure.MockResourceSpecGetter{})).Return(nil, fakeInternalError)
			},
		},
		{
			name:           "async get returns not found",
			expectedError:  "",
			serviceName:    "test-service",
			expectedResult: &fakeExistingResource,
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockCreatorMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.PutFuture).Return(nil)
				c.Get(gomockinternal.AContext(), gomock.AssignableToTypeOf(&mock_azure.MockResourceSpecGetter{})).Return(nil, fakeNotFoundError)
				r.Parameters(gomockinternal.AContext(), nil).Return(&fakeResourceParameters, nil)
				c.CreateOrUpdateAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(&mock_azure.MockResourceSpecGetter{}), &fakeResourceParameters).Return(&fakeExistingResource, nil, nil)
			},
		},
		{
			name:          "error occurs while running async spec parameters",
			expectedError: "failed to get desired parameters for resource test-group/test-resource (service: test-service)",
			serviceName:   "test-service",
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockCreatorMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.PutFuture).Return(nil)
				c.Get(gomockinternal.AContext(), gomock.AssignableToTypeOf(&mock_azure.MockResourceSpecGetter{})).Return(&fakeExistingResource, nil)
				r.Parameters(gomockinternal.AContext(), &fakeExistingResource).Return(nil, fakeInternalError)
			},
		},
		{
			name:           "async spec parameters returns nil",
			expectedError:  "",
			serviceName:    "test-service",
			expectedResult: &fakeExistingResource,
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockCreatorMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.PutFuture).Return(nil)
				c.Get(gomockinternal.AContext(), gomock.AssignableToTypeOf(&mock_azure.MockResourceSpecGetter{})).Return(&fakeExistingResource, nil)
				r.Parameters(gomockinternal.AContext(), &fakeExistingResource).Return(nil, nil)
			},
		},
		{
			name:          "error occurs while running async create",
			expectedError: "failed to update resource test-group/test-resource (service: test-service)",
			serviceName:   "test-service",
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockCreatorMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.PutFuture).Return(nil)
				c.Get(gomockinternal.AContext(), gomock.AssignableToTypeOf(&mock_azure.MockResourceSpecGetter{})).Return(&fakeExistingResource, nil)
				r.Parameters(gomockinternal.AContext(), &fakeExistingResource).Return(&fakeResourceParameters, nil)
				c.CreateOrUpdateAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(&mock_azure.MockResourceSpecGetter{}), &fakeResourceParameters).Return(nil, nil, fakeInternalError)
			},
		},
		{
			name:          "create async exits before completing",
			expectedError: "operation type PUT on Azure resource test-group/test-resource is not done. Object will be requeued after 15s",
			serviceName:   "test-service",
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockCreatorMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.PutFuture).Return(nil)
				c.Get(gomockinternal.AContext(), gomock.AssignableToTypeOf(&mock_azure.MockResourceSpecGetter{})).Return(&fakeExistingResource, nil)
				r.Parameters(gomockinternal.AContext(), &fakeExistingResource).Return(&fakeResourceParameters, nil)
				c.CreateOrUpdateAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(&mock_azure.MockResourceSpecGetter{}), &fakeResourceParameters).Return(nil, &azureautorest.Future{}, errCtxExceeded)
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
			scopeMock := mock_asyncpoller.NewMockFutureScope(mockCtrl)
			creatorMock := mock_asyncpoller.NewMockCreator(mockCtrl)
			specMock := mock_azure.NewMockResourceSpecGetter(mockCtrl)

			tc.expect(scopeMock.EXPECT(), creatorMock.EXPECT(), specMock.EXPECT())

			s := New(scopeMock, creatorMock, nil)
			result, err := s.CreateOrUpdateResource(context.TODO(), specMock, tc.serviceName)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(tc.expectedResult))
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
		expect        func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockDeleterMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder)
	}{
		{
			name:          "delete operation is already in progress",
			expectedError: "operation type DELETE on Azure resource test-group/test-resource is not done. Object will be requeued after 15s",
			serviceName:   "test-service",
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockDeleterMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.DeleteFuture).Times(2).Return(&validDeleteFuture)
				c.IsDone(gomockinternal.AContext(), gomock.AssignableToTypeOf(&azureautorest.Future{})).Return(false, nil)
			},
		},
		{
			name:          "delete async returns success",
			expectedError: "",
			serviceName:   "test-service",
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockDeleterMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.DeleteFuture).Return(nil)
				c.DeleteAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(&mock_azure.MockResourceSpecGetter{})).Return(nil, nil)
			},
		},
		{
			name:          "delete async returns not found",
			expectedError: "",
			serviceName:   "test-service",
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockDeleterMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.DeleteFuture).Return(nil)
				c.DeleteAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(&mock_azure.MockResourceSpecGetter{})).Return(nil, fakeNotFoundError)
			},
		},
		{
			name:          "error occurs while running async delete",
			expectedError: "failed to delete resource test-group/test-resource (service: test-service)",
			serviceName:   "test-service",
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockDeleterMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.DeleteFuture).Return(nil)
				c.DeleteAsync(gomockinternal.AContext(), gomock.AssignableToTypeOf(&mock_azure.MockResourceSpecGetter{})).Return(nil, fakeInternalError)
			},
		},
		{
			name:          "delete async exits before completing",
			expectedError: "operation type DELETE on Azure resource test-group/test-resource is not done. Object will be requeued after 15s",
			serviceName:   "test-service",
			expect: func(s *mock_asyncpoller.MockFutureScopeMockRecorder, c *mock_asyncpoller.MockDeleterMockRecorder, r *mock_azure.MockResourceSpecGetterMockRecorder) {
				r.ResourceName().Return("test-resource")
				r.ResourceGroupName().Return("test-group")
				s.GetLongRunningOperationState("test-resource", "test-service", infrav1.DeleteFuture).Return(nil)
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
			scopeMock := mock_asyncpoller.NewMockFutureScope(mockCtrl)
			mockObj := mock_asyncpoller
			deleterMock := mock_asyncpoller.NewMockDeleter(mockCtrl)
			specMock := mock_azure.NewMockResourceSpecGetter(mockCtrl)

			tc.expect(scopeMock.EXPECT(), deleterMock.EXPECT(), specMock.EXPECT())

			s := New(scopeMock, nil, deleterMock)
			err := s.DeleteResource(context.TODO(), specMock, tc.serviceName)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestGetRetryAfterFromError(t *testing.T) {
	cases := []struct {
		name                   string
		input                  error
		expected               time.Duration
		expectedRangeTolerance time.Duration
	}{
		{
			name: "Retry-After header data present in the form of units of seconds",
			input: autorest.DetailedError{
				Response: &http.Response{
					Header: http.Header{
						"Retry-After": []string{"2"},
					},
				},
			},
			expected: 2 * time.Second,
		},
		{
			name: "Retry-After header data present in the form of units of absolute time",
			input: autorest.DetailedError{
				Response: &http.Response{
					Header: http.Header{
						"Retry-After": []string{time.Now().Add(1 * time.Hour).Format(time.RFC1123)},
					},
				},
			},
			expected:               1 * time.Hour,
			expectedRangeTolerance: 5 * time.Second,
		},
		{
			name: "Retry-After header data not present",
			input: autorest.DetailedError{
				Response: &http.Response{
					Header: http.Header{
						"foo": []string{"bar"},
					},
				},
			},
			expected: reconciler.DefaultReconcilerRequeue,
		},
		{
			name: "Retry-After header data not present in HTTP 429",
			input: autorest.DetailedError{
				Response: &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header: http.Header{
						"foo": []string{"bar"},
					},
				},
			},
			expected: reconciler.DefaultHTTP429RetryAfter,
		},
		{
			name: "nil http.Response",
			input: autorest.DetailedError{
				Response: nil,
			},
			expected: reconciler.DefaultReconcilerRequeue,
		},
		{
			name:     "error type is not autorest.DetailedError",
			input:    errors.New("error"),
			expected: reconciler.DefaultReconcilerRequeue,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			ret := getRetryAfterFromError(c.input)
			if c.expectedRangeTolerance > 0 {
				g.Expect(ret).To(BeNumerically("<", c.expected))
				g.Expect(ret + c.expectedRangeTolerance).To(BeNumerically(">", c.expected))
			} else {
				g.Expect(ret).To(Equal(c.expected))
			}
		})
	}
}
.
*/

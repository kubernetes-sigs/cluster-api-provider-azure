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

package privateendpoints

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/privateendpoints/mock_privateendpoints"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	fakePrivateEndpoint1 = PrivateEndpointSpec{
		Name:                          "fake-private-endpoint1",
		ApplicationSecurityGroups:     []string{"asg1"},
		PrivateLinkServiceConnections: []PrivateLinkServiceConnection{{PrivateLinkServiceID: "testPl", RequestMessage: "Please approve my connection."}},
		SubnetID:                      "mySubnet",
		ResourceGroup:                 "my-rg",
		ManualApproval:                false,
	}

	fakePrivateEndpoint2 = PrivateEndpointSpec{
		Name:                          "fake-private-endpoint2",
		PrivateLinkServiceConnections: []PrivateLinkServiceConnection{{PrivateLinkServiceID: "testPl", RequestMessage: "Please approve my connection."}},
		SubnetID:                      "mySubnet",
		ResourceGroup:                 "my-rg",
		ManualApproval:                true,
	}

	fakePrivateEndpoint3 = PrivateEndpointSpec{
		Name:                          "fake-private-endpoint3",
		ApplicationSecurityGroups:     []string{"sg1"},
		PrivateLinkServiceConnections: []PrivateLinkServiceConnection{{PrivateLinkServiceID: "testPl", RequestMessage: "Please approve my connection."}},
		SubnetID:                      "mySubnet",
		ResourceGroup:                 "my-rg",
		ManualApproval:                false,
		CustomNetworkInterfaceName:    "pestaticconfig",
		PrivateIPAddresses:            []string{"10.0.0.1"},
	}

	emptyPrivateEndpointSpec = PrivateEndpointSpec{}
	fakePrivateEndpointSpecs = []azure.ResourceSpecGetter{&fakePrivateEndpoint1, &fakePrivateEndpoint2, &fakePrivateEndpoint3, &emptyPrivateEndpointSpec}

	internalError = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error")
	notDoneError  = azure.NewOperationNotDoneError(&infrav1.Future{})
)

func TestReconcilePrivateEndpoint(t *testing.T) {
	testcases := []struct {
		name          string
		expect        func(s *mock_privateendpoints.MockPrivateEndpointScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
		expectedError string
	}{
		{
			name:          "create a private endpoint with automatic approval",
			expectedError: "",
			expect: func(p *mock_privateendpoints.MockPrivateEndpointScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.PrivateEndpointSpecs().Return([]azure.ResourceSpecGetter{&fakePrivateEndpoint1})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePrivateEndpoint1, ServiceName).Return(&fakePrivateEndpoint1, nil)
				p.UpdatePutStatus(infrav1.PrivateEndpointsReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "create a private endpoint with manual approval",
			expectedError: "",
			expect: func(p *mock_privateendpoints.MockPrivateEndpointScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.PrivateEndpointSpecs().Return(fakePrivateEndpointSpecs[1:2])
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePrivateEndpoint2, ServiceName).Return(&fakePrivateEndpoint2, nil)
				p.UpdatePutStatus(infrav1.PrivateEndpointsReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "create multiple private endpoints",
			expectedError: "",
			expect: func(p *mock_privateendpoints.MockPrivateEndpointScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.PrivateEndpointSpecs().Return(fakePrivateEndpointSpecs[:2])
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePrivateEndpoint1, ServiceName).Return(&fakePrivateEndpoint1, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePrivateEndpoint2, ServiceName).Return(&fakePrivateEndpoint2, nil)
				p.UpdatePutStatus(infrav1.PrivateEndpointsReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "return error when creating a private endpoint using an empty spec",
			expectedError: internalError.Error(),
			expect: func(p *mock_privateendpoints.MockPrivateEndpointScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.PrivateEndpointSpecs().Return(fakePrivateEndpointSpecs[3:])
				r.CreateOrUpdateResource(gomockinternal.AContext(), &emptyPrivateEndpointSpec, ServiceName).Return(nil, internalError)
				p.UpdatePutStatus(infrav1.PrivateEndpointsReadyCondition, ServiceName, internalError)
			},
		},
		{
			name:          "not done error in creating is ignored",
			expectedError: internalError.Error(),
			expect: func(p *mock_privateendpoints.MockPrivateEndpointScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.PrivateEndpointSpecs().Return(fakePrivateEndpointSpecs[:3])
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePrivateEndpoint1, ServiceName).Return(&fakePrivateEndpoint1, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePrivateEndpoint2, ServiceName).Return(&fakePrivateEndpoint2, internalError)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePrivateEndpoint3, ServiceName).Return(&fakePrivateEndpoint3, notDoneError)
				p.UpdatePutStatus(infrav1.PrivateEndpointsReadyCondition, ServiceName, internalError)
			},
		},
		{
			name:          "not done error in creating remains",
			expectedError: "operation type  on Azure resource / is not done",
			expect: func(p *mock_privateendpoints.MockPrivateEndpointScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.PrivateEndpointSpecs().Return(fakePrivateEndpointSpecs[:3])
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePrivateEndpoint1, ServiceName).Return(&fakePrivateEndpoint1, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePrivateEndpoint2, ServiceName).Return(nil, notDoneError)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakePrivateEndpoint3, ServiceName).Return(&fakePrivateEndpoint3, nil)
				p.UpdatePutStatus(infrav1.PrivateEndpointsReadyCondition, ServiceName, notDoneError)
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
			scopeMock := mock_privateendpoints.NewMockPrivateEndpointScope(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
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

func TestDeletePrivateEndpoints(t *testing.T) {
	testcases := []struct {
		name          string
		expect        func(s *mock_privateendpoints.MockPrivateEndpointScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
		expectedError string
	}{
		{
			name:          "delete a private endpoint",
			expectedError: "",
			expect: func(p *mock_privateendpoints.MockPrivateEndpointScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.PrivateEndpointSpecs().Return(fakePrivateEndpointSpecs[:1])
				r.DeleteResource(gomockinternal.AContext(), &fakePrivateEndpoint1, ServiceName).Return(nil)
				p.UpdateDeleteStatus(infrav1.PrivateEndpointsReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "noop if no private endpoints specs are found",
			expectedError: "",
			expect: func(p *mock_privateendpoints.MockPrivateEndpointScopeMockRecorder, _ *mock_async.MockReconcilerMockRecorder) {
				p.PrivateEndpointSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "delete multiple private endpoints",
			expectedError: "",
			expect: func(p *mock_privateendpoints.MockPrivateEndpointScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.PrivateEndpointSpecs().Return(fakePrivateEndpointSpecs[:2])
				r.DeleteResource(gomockinternal.AContext(), &fakePrivateEndpoint1, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePrivateEndpoint2, ServiceName).Return(nil)
				p.UpdateDeleteStatus(infrav1.PrivateEndpointsReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "error in deleting peering",
			expectedError: internalError.Error(),
			expect: func(p *mock_privateendpoints.MockPrivateEndpointScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.PrivateEndpointSpecs().Return(fakePrivateEndpointSpecs[:2])
				r.DeleteResource(gomockinternal.AContext(), &fakePrivateEndpoint1, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePrivateEndpoint2, ServiceName).Return(internalError)
				p.UpdateDeleteStatus(infrav1.PrivateEndpointsReadyCondition, ServiceName, internalError)
			},
		},
		{
			name:          "not done error in deleting is ignored",
			expectedError: internalError.Error(),
			expect: func(p *mock_privateendpoints.MockPrivateEndpointScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.PrivateEndpointSpecs().Return(fakePrivateEndpointSpecs[:3])
				r.DeleteResource(gomockinternal.AContext(), &fakePrivateEndpoint1, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePrivateEndpoint2, ServiceName).Return(internalError)
				r.DeleteResource(gomockinternal.AContext(), &fakePrivateEndpoint3, ServiceName).Return(notDoneError)
				p.UpdateDeleteStatus(infrav1.PrivateEndpointsReadyCondition, ServiceName, internalError)
			},
		},
		{
			name:          "not done error in deleting remains",
			expectedError: "operation type  on Azure resource / is not done",
			expect: func(p *mock_privateendpoints.MockPrivateEndpointScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				p.PrivateEndpointSpecs().Return(fakePrivateEndpointSpecs[:3])
				r.DeleteResource(gomockinternal.AContext(), &fakePrivateEndpoint1, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePrivateEndpoint2, ServiceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakePrivateEndpoint3, ServiceName).Return(notDoneError)
				p.UpdateDeleteStatus(infrav1.PrivateEndpointsReadyCondition, ServiceName, notDoneError)
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
			scopeMock := mock_privateendpoints.NewMockPrivateEndpointScope(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: asyncMock,
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

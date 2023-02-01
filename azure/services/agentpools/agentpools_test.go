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

package agentpools

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/agentpools/mock_agentpools"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var internalError = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error")

func TestReconcileAgentPools(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_agentpools.MockAgentPoolScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "agent pool successfully created with autoscaling enabled",
			expectedError: "",
			expect: func(s *mock_agentpools.MockAgentPoolScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				fakeAgentPoolSpec := fakeAgentPool()
				s.AgentPoolSpec().Return(&fakeAgentPoolSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeAgentPoolSpec, serviceName).Return(sdkFakeAgentPool(sdkWithAutoscaling(true), sdkWithCount(1)), nil)
				s.SetCAPIMachinePoolAnnotation(clusterv1.ReplicasManagedByAnnotation, "true")
				s.SetCAPIMachinePoolReplicas(pointer.Int32(1))
				s.UpdatePutStatus(infrav1.AgentPoolsReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "agent pool successfully created with autoscaling disabled",
			expectedError: "",
			expect: func(s *mock_agentpools.MockAgentPoolScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				fakeAgentPoolSpec := fakeAgentPool()
				s.AgentPoolSpec().Return(&fakeAgentPoolSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeAgentPoolSpec, serviceName).Return(sdkFakeAgentPool(sdkWithAutoscaling(false), sdkWithCount(1)), nil)
				s.RemoveCAPIMachinePoolAnnotation(clusterv1.ReplicasManagedByAnnotation)

				s.UpdatePutStatus(infrav1.AgentPoolsReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "no agent pool spec found",
			expectedError: "",
			expect: func(s *mock_agentpools.MockAgentPoolScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AgentPoolSpec().Return(nil)
			},
		},
		{
			name:          "fail to create a agent pool",
			expectedError: internalError.Error(),
			expect: func(s *mock_agentpools.MockAgentPoolScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				fakeAgentPoolSpec := fakeAgentPool()
				s.AgentPoolSpec().Return(&fakeAgentPoolSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeAgentPoolSpec, serviceName).Return(nil, internalError)
				s.UpdatePutStatus(infrav1.AgentPoolsReadyCondition, serviceName, internalError)
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
			scopeMock := mock_agentpools.NewMockAgentPoolScope(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				scope:      scopeMock,
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

func TestDeleteAgentPools(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_agentpools.MockAgentPoolScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "existing agent pool successfully deleted",
			expectedError: "",
			expect: func(s *mock_agentpools.MockAgentPoolScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				fakeAgentPoolSpec := fakeAgentPool()
				s.AgentPoolSpec().Return(&fakeAgentPoolSpec)
				r.DeleteResource(gomockinternal.AContext(), &fakeAgentPoolSpec, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.AgentPoolsReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "no agent pool spec found",
			expectedError: "",
			expect: func(s *mock_agentpools.MockAgentPoolScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AgentPoolSpec().Return(nil)
			},
		},
		{
			name:          "fail to delete a agent pool",
			expectedError: internalError.Error(),
			expect: func(s *mock_agentpools.MockAgentPoolScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				fakeAgentPoolSpec := fakeAgentPool()
				s.AgentPoolSpec().Return(&fakeAgentPoolSpec)
				r.DeleteResource(gomockinternal.AContext(), &fakeAgentPoolSpec, serviceName).Return(internalError)
				s.UpdateDeleteStatus(infrav1.AgentPoolsReadyCondition, serviceName, internalError)
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
			scopeMock := mock_agentpools.NewMockAgentPoolScope(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				scope:      scopeMock,
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

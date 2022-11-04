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
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/agentpools/mock_agentpools"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	fakeAgentPoolSpec = AgentPoolSpec{
		Name:              "fake-agent-pool-name",
		ResourceGroup:     "fake-rg",
		Cluster:           "fake-cluster",
		Version:           to.StringPtr("fake-version"),
		SKU:               "fake-sku",
		Replicas:          1,
		OSDiskSizeGB:      2,
		VnetSubnetID:      "fake-vnet-subnet-id",
		Mode:              "fake-mode",
		MaxCount:          to.Int32Ptr(5),
		MinCount:          to.Int32Ptr(1),
		NodeLabels:        map[string]*string{"fake-label": to.StringPtr("fake-value")},
		NodeTaints:        []string{"fake-taint"},
		EnableAutoScaling: to.BoolPtr(true),
		AvailabilityZones: []string{"fake-zone"},
		MaxPods:           to.Int32Ptr(10),
		OsDiskType:        to.StringPtr("fake-os-disk-type"),
		EnableUltraSSD:    to.BoolPtr(true),
		OSType:            to.StringPtr("fake-os-type"),
		Headers:           map[string]string{"fake-header": "fake-value"},
	}

	internalError = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error")
)

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
				s.AgentPoolSpec().Return(&fakeAgentPoolSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeAgentPoolSpec, serviceName).Return(fakeAgentPoolWithAutoscalingAndCount(true, 1), nil)
				s.SetCAPIMachinePoolAnnotation(azure.ReplicasManagedByAutoscalerAnnotation, "true")
				s.SetCAPIMachinePoolReplicas(fakeAgentPoolWithAutoscalingAndCount(true, 1).Count)
				s.UpdatePutStatus(infrav1.AgentPoolsReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "agent pool successfully created with autoscaling disabled",
			expectedError: "",
			expect: func(s *mock_agentpools.MockAgentPoolScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AgentPoolSpec().Return(&fakeAgentPoolSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeAgentPoolSpec, serviceName).Return(fakeAgentPoolWithAutoscalingAndCount(false, 1), nil)
				s.RemoveCAPIMachinePoolAnnotation(azure.ReplicasManagedByAutoscalerAnnotation)

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

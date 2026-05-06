/*
Copyright 2025 The Kubernetes Authors.

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

package scope

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	cplane "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
	v1beta2 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
)

// getScheme creates a runtime.Scheme with all necessary types registered for testing.
func getScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = cplane.AddToScheme(scheme)
	_ = v1beta2.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	return scheme
}

func TestNewAROMachinePoolScope(t *testing.T) {
	scheme := getScheme(t)
	g := NewWithT(t)

	testCases := []struct {
		name          string
		params        AROMachinePoolScopeParams
		expectError   bool
		errorContains string
	}{
		{
			name: "successfully creates new scope",
			params: AROMachinePoolScopeParams{
				Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
				Cluster: &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
				},
				MachinePool: &clusterv1.MachinePool{
					ObjectMeta: metav1.ObjectMeta{Name: "test-mp", Namespace: "default"},
				},
				ControlPlane: &cplane.AROControlPlane{
					ObjectMeta: metav1.ObjectMeta{Name: "test-cp", Namespace: "default"},
				},
				AROMachinePool: &v1beta2.AROMachinePool{
					ObjectMeta: metav1.ObjectMeta{Name: "test-aromp", Namespace: "default"},
				},
			},
			expectError: false,
		},
		{
			name: "fails when AROMachinePool is nil",
			params: AROMachinePoolScopeParams{
				Client:         fake.NewClientBuilder().WithScheme(scheme).Build(),
				Cluster:        &clusterv1.Cluster{},
				MachinePool:    &clusterv1.MachinePool{},
				ControlPlane:   &cplane.AROControlPlane{},
				AROMachinePool: nil,
			},
			expectError:   true,
			errorContains: "failed to generate new scope from nil AROMachinePool",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scope, err := NewAROMachinePoolScope(t.Context(), tc.params)
			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.errorContains))
				g.Expect(scope).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(scope).NotTo(BeNil())
				g.Expect(scope.Client).To(Equal(tc.params.Client))
				g.Expect(scope.Cluster).To(Equal(tc.params.Cluster))
				g.Expect(scope.MachinePool).To(Equal(tc.params.MachinePool))
				g.Expect(scope.ControlPlane).To(Equal(tc.params.ControlPlane))
				g.Expect(scope.InfraMachinePool).To(Equal(tc.params.AROMachinePool))
			}
		})
	}
}

func TestAROMachinePoolScope_LongRunningOperations(t *testing.T) {
	g := NewWithT(t)

	aroMP := &v1beta2.AROMachinePool{
		ObjectMeta: metav1.ObjectMeta{Name: "test-aromp", Namespace: "default"},
	}

	scope := &AROMachinePoolScope{
		InfraMachinePool: aroMP,
	}

	// Test Set/Get/Delete LongRunningOperationState
	future := &infrav1.Future{
		Type:          "PUT",
		Name:          "test-resource",
		ServiceName:   "test-service",
		ResourceGroup: "test-rg",
	}

	// Initially, no future should exist
	retrieved := scope.GetLongRunningOperationState("test-resource", "test-service", "PUT")
	g.Expect(retrieved).To(BeNil())

	// Set the future
	scope.SetLongRunningOperationState(future)

	// Get the future back
	retrieved = scope.GetLongRunningOperationState("test-resource", "test-service", "PUT")
	g.Expect(retrieved).NotTo(BeNil())
	g.Expect(retrieved.Name).To(Equal("test-resource"))
	g.Expect(retrieved.ServiceName).To(Equal("test-service"))
	g.Expect(retrieved.Type).To(Equal("PUT"))

	// Delete the future
	scope.DeleteLongRunningOperationState("test-resource", "test-service", "PUT")

	// Verify it's gone
	retrieved = scope.GetLongRunningOperationState("test-resource", "test-service", "PUT")
	g.Expect(retrieved).To(BeNil())
}

func TestAROMachinePoolScope_UpdateDeleteStatus(t *testing.T) {
	testCases := []struct {
		name            string
		err             error
		expectedStatus  metav1.ConditionStatus
		expectedReason  string
		expectedMessage string
	}{
		{
			name:            "successful deletion",
			err:             nil,
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  infrav1.DeletedReason,
			expectedMessage: "test-service successfully deleted",
		},
		{
			name:            "deletion in progress",
			err:             azure.NewOperationNotDoneError(&infrav1.Future{}),
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  infrav1.DeletingReason,
			expectedMessage: "test-service deleting",
		},
		{
			name:            "deletion failed",
			err:             &azcore.ResponseError{ErrorCode: "InternalError"},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  infrav1.DeletionFailedReason,
			expectedMessage: "test-service failed to delete",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			aroMP := &v1beta2.AROMachinePool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-aromp", Namespace: "default"},
			}

			scope := &AROMachinePoolScope{
				InfraMachinePool: aroMP,
			}

			scope.UpdateDeleteStatus(v1beta2.AROMachinePoolReadyCondition, "test-service", tc.err)

			g.Expect(aroMP.Status.Conditions).To(HaveLen(1))
			cond := aroMP.Status.Conditions[0]
			g.Expect(cond.Type).To(Equal(string(v1beta2.AROMachinePoolReadyCondition)))
			g.Expect(cond.Status).To(Equal(tc.expectedStatus))
			g.Expect(cond.Reason).To(Equal(tc.expectedReason))
			g.Expect(cond.Message).To(ContainSubstring(tc.expectedMessage))
		})
	}
}

func TestAROMachinePoolScope_UpdatePutStatus(t *testing.T) {
	testCases := []struct {
		name              string
		provisioningState string
		err               error
		expectedStatus    metav1.ConditionStatus
		expectedReason    string
		expectedMessage   string
	}{
		{
			name:            "successful creation",
			err:             nil,
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  "Succeeded",
			expectedMessage: "",
		},
		{
			name:            "creation in progress",
			err:             azure.NewOperationNotDoneError(&infrav1.Future{}),
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  infrav1.CreatingReason,
			expectedMessage: "test-service creating or updating",
		},
		{
			name:              "update in progress",
			provisioningState: ProvisioningStateUpdating,
			err:               azure.NewOperationNotDoneError(&infrav1.Future{}),
			expectedStatus:    metav1.ConditionFalse,
			expectedReason:    infrav1.UpdatingReason,
			expectedMessage:   "test-service creating or updating",
		},
		{
			name:            "creation failed",
			err:             &azcore.ResponseError{ErrorCode: "InternalError"},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  infrav1.FailedReason,
			expectedMessage: "test-service failed to create or update",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			aroMP := &v1beta2.AROMachinePool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-aromp", Namespace: "default"},
				Status: v1beta2.AROMachinePoolStatus{
					ProvisioningState: tc.provisioningState,
				},
			}

			scope := &AROMachinePoolScope{
				InfraMachinePool: aroMP,
			}

			scope.UpdatePutStatus(v1beta2.AROMachinePoolReadyCondition, "test-service", tc.err)

			g.Expect(aroMP.Status.Conditions).To(HaveLen(1))
			cond := aroMP.Status.Conditions[0]
			g.Expect(cond.Type).To(Equal(string(v1beta2.AROMachinePoolReadyCondition)))
			g.Expect(cond.Status).To(Equal(tc.expectedStatus))
			g.Expect(cond.Reason).To(Equal(tc.expectedReason))
			if tc.expectedMessage != "" {
				g.Expect(cond.Message).To(ContainSubstring(tc.expectedMessage))
			}
		})
	}
}

func TestAROMachinePoolScope_UpdatePatchStatus(t *testing.T) {
	testCases := []struct {
		name            string
		err             error
		expectedStatus  metav1.ConditionStatus
		expectedReason  string
		expectedMessage string
	}{
		{
			name:            "successful patch",
			err:             nil,
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  "Succeeded",
			expectedMessage: "",
		},
		{
			name:            "patch in progress",
			err:             azure.NewOperationNotDoneError(&infrav1.Future{}),
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  infrav1.UpdatingReason,
			expectedMessage: "test-service updating",
		},
		{
			name:            "patch failed",
			err:             &azcore.ResponseError{ErrorCode: "InternalError"},
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  infrav1.FailedReason,
			expectedMessage: "test-service failed to update",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			aroMP := &v1beta2.AROMachinePool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-aromp", Namespace: "default"},
			}

			scope := &AROMachinePoolScope{
				InfraMachinePool: aroMP,
			}

			scope.UpdatePatchStatus(v1beta2.AROMachinePoolReadyCondition, "test-service", tc.err)

			g.Expect(aroMP.Status.Conditions).To(HaveLen(1))
			cond := aroMP.Status.Conditions[0]
			g.Expect(cond.Type).To(Equal(string(v1beta2.AROMachinePoolReadyCondition)))
			g.Expect(cond.Status).To(Equal(tc.expectedStatus))
			g.Expect(cond.Reason).To(Equal(tc.expectedReason))
			if tc.expectedMessage != "" {
				g.Expect(cond.Message).To(ContainSubstring(tc.expectedMessage))
			}
		})
	}
}

func TestAROMachinePoolScope_GetterMethods(t *testing.T) {
	g := NewWithT(t)
	scheme := getScheme(t)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-cluster",
			Namespace:         "default",
			DeletionTimestamp: &metav1.Time{},
		},
	}
	aroMP := &v1beta2.AROMachinePool{
		ObjectMeta: metav1.ObjectMeta{Name: "test-aromp", Namespace: "default"},
	}

	scope := &AROMachinePoolScope{
		Client:           fakeClient,
		Cluster:          cluster,
		InfraMachinePool: aroMP,
	}

	g.Expect(scope.GetClient()).To(Equal(fakeClient))
	g.Expect(scope.GetDeletionTimestamp()).To(Equal(cluster.DeletionTimestamp))
	g.Expect(scope.Name()).To(Equal("test-aromp"))
	g.Expect(scope.ClusterName()).To(Equal("test-cluster"))
	g.Expect(scope.Namespace()).To(Equal("default"))
}

func TestAROMachinePoolScope_SetAgentPoolProvisioningState(t *testing.T) {
	g := NewWithT(t)

	aroMP := &v1beta2.AROMachinePool{
		ObjectMeta: metav1.ObjectMeta{Name: "test-aromp", Namespace: "default"},
	}

	scope := &AROMachinePoolScope{
		InfraMachinePool: aroMP,
	}

	scope.SetAgentPoolProvisioningState(ProvisioningStateSucceeded)
	g.Expect(aroMP.Status.ProvisioningState).To(Equal(ProvisioningStateSucceeded))

	scope.SetAgentPoolProvisioningState(ProvisioningStateUpdating)
	g.Expect(aroMP.Status.ProvisioningState).To(Equal(ProvisioningStateUpdating))
}

func TestAROMachinePoolScope_SetAgentPoolReady(t *testing.T) {
	testCases := []struct {
		name                string
		provisioningState   string
		readyInput          bool
		expectedReady       bool
		expectedProvisioned bool
	}{
		{
			name:                "ready when provisioning succeeded",
			provisioningState:   ProvisioningStateSucceeded,
			readyInput:          true,
			expectedReady:       true,
			expectedProvisioned: true,
		},
		{
			name:                "ready when provisioning updating",
			provisioningState:   ProvisioningStateUpdating,
			readyInput:          true,
			expectedReady:       true,
			expectedProvisioned: true,
		},
		{
			name:                "not ready when provisioning in progress",
			provisioningState:   "Creating",
			readyInput:          true,
			expectedReady:       false,
			expectedProvisioned: false,
		},
		{
			name:                "not ready when explicitly false",
			provisioningState:   ProvisioningStateSucceeded,
			readyInput:          false,
			expectedReady:       false,
			expectedProvisioned: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			aroMP := &v1beta2.AROMachinePool{
				ObjectMeta: metav1.ObjectMeta{Name: "test-aromp", Namespace: "default"},
				Status: v1beta2.AROMachinePoolStatus{
					ProvisioningState: tc.provisioningState,
				},
			}

			scope := &AROMachinePoolScope{
				InfraMachinePool: aroMP,
			}

			scope.SetAgentPoolReady(tc.readyInput)

			g.Expect(aroMP.Status.Ready).To(Equal(tc.expectedReady))
			g.Expect(aroMP.Status.Initialization).NotTo(BeNil())
			g.Expect(aroMP.Status.Initialization.Provisioned).To(Equal(tc.expectedProvisioned))
		})
	}
}

func TestAROMachinePoolScope_SetAgentPoolProviderIDList(t *testing.T) {
	g := NewWithT(t)

	aroMP := &v1beta2.AROMachinePool{
		ObjectMeta: metav1.ObjectMeta{Name: "test-aromp", Namespace: "default"},
	}

	scope := &AROMachinePoolScope{
		InfraMachinePool: aroMP,
	}

	providerIDs := []string{
		"azure:///subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm1",
		"azure:///subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm2",
	}

	scope.SetAgentPoolProviderIDList(providerIDs)
	g.Expect(aroMP.Spec.ProviderIDList).To(Equal(providerIDs))
}

func TestAROMachinePoolScope_PatchObject(t *testing.T) {
	g := NewWithT(t)
	scheme := getScheme(t)

	aroMP := &v1beta2.AROMachinePool{
		ObjectMeta: metav1.ObjectMeta{Name: "test-aromp", Namespace: "default"},
	}
	machinePool := &clusterv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{Name: "test-mp", Namespace: "default"},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(aroMP, machinePool).
		WithStatusSubresource(aroMP, machinePool).
		Build()

	params := AROMachinePoolScopeParams{
		Client: fakeClient,
		Cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
		},
		MachinePool: machinePool,
		ControlPlane: &cplane.AROControlPlane{
			ObjectMeta: metav1.ObjectMeta{Name: "test-cp", Namespace: "default"},
		},
		AROMachinePool: aroMP,
	}

	scope, err := NewAROMachinePoolScope(t.Context(), params)
	g.Expect(err).NotTo(HaveOccurred())

	// Modify the status
	scope.SetAgentPoolProvisioningState(ProvisioningStateSucceeded)

	// Patch should succeed
	err = scope.PatchObject(t.Context())
	g.Expect(err).NotTo(HaveOccurred())
}

func TestAROMachinePoolScope_Close(t *testing.T) {
	g := NewWithT(t)
	scheme := getScheme(t)

	aroMP := &v1beta2.AROMachinePool{
		ObjectMeta: metav1.ObjectMeta{Name: "test-aromp", Namespace: "default"},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(aroMP).Build()

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	timeouts := mock_azure.NewMockAsyncReconciler(mockCtrl)

	params := AROMachinePoolScopeParams{
		Client: fakeClient,
		Cluster: &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
		},
		MachinePool: &clusterv1.MachinePool{
			ObjectMeta: metav1.ObjectMeta{Name: "test-mp", Namespace: "default"},
		},
		ControlPlane: &cplane.AROControlPlane{
			ObjectMeta: metav1.ObjectMeta{Name: "test-cp", Namespace: "default"},
		},
		AROMachinePool: aroMP,
		Timeouts:       timeouts,
	}

	scope, err := NewAROMachinePoolScope(t.Context(), params)
	g.Expect(err).NotTo(HaveOccurred())

	// Close should patch and succeed
	err = scope.Close(t.Context())
	g.Expect(err).NotTo(HaveOccurred())
}

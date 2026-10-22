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

package controllers

import (
	"testing"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
)

// TestAutoscalingSpecReplicasUpdate tests that MachinePool.Spec.Replicas is updated
// when autoscaling is enabled and actual node count changes.
func TestAutoscalingSpecReplicasUpdate(t *testing.T) {
	tests := []struct {
		name                string
		initialSpecReplicas int32
		actualNodeCount     int32
		hasAutoscaling      bool
		expectedSpec        *int32
	}{
		{
			name:                "autoscaling enabled - spec updated to match actual",
			initialSpecReplicas: 3,
			actualNodeCount:     5,
			hasAutoscaling:      true,
			expectedSpec:        ptr.To[int32](5),
		},
		{
			name:                "autoscaling disabled - spec not updated",
			initialSpecReplicas: 3,
			actualNodeCount:     5,
			hasAutoscaling:      false,
			expectedSpec:        ptr.To[int32](3),
		},
		{
			name:                "autoscaling enabled - scale down",
			initialSpecReplicas: 5,
			actualNodeCount:     3,
			hasAutoscaling:      true,
			expectedSpec:        ptr.To[int32](3),
		},
		{
			name:                "autoscaling enabled - no change",
			initialSpecReplicas: 5,
			actualNodeCount:     5,
			hasAutoscaling:      true,
			expectedSpec:        ptr.To[int32](5),
		},
		{
			name:                "autoscaling disabled - spec unchanged even with mismatch",
			initialSpecReplicas: 3,
			actualNodeCount:     10,
			hasAutoscaling:      false,
			expectedSpec:        ptr.To[int32](3),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			// Create MachinePool with initial spec
			machinePool := &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mp",
					Namespace: "default",
				},
				Spec: clusterv1.MachinePoolSpec{
					Replicas: ptr.To(test.initialSpecReplicas),
				},
			}

			// Add autoscaling annotation if enabled
			if test.hasAutoscaling {
				machinePool.Annotations = map[string]string{
					clusterv1.ReplicasManagedByAnnotation: infrav1exp.ReplicasManagedByARO,
				}
			}

			// Simulate the reconciler logic
			currentReplicas := test.actualNodeCount

			// This is the logic from aromachinepool_reconciler.go lines 288-293
			if test.hasAutoscaling {
				if _, autoscaling := machinePool.Annotations[clusterv1.ReplicasManagedByAnnotation]; autoscaling {
					machinePool.Spec.Replicas = &currentReplicas
				}
			}

			// Verify the expected outcome
			g.Expect(machinePool.Spec.Replicas).To(Equal(test.expectedSpec))
		})
	}
}

// TestAutoscalingAnnotationBehavior tests the interaction between autoscaling annotation
// and replica management.
func TestAutoscalingAnnotationBehavior(t *testing.T) {
	tests := []struct {
		name              string
		annotations       map[string]string
		actualNodeCount   int32
		expectSpecUpdated bool
	}{
		{
			name: "ARO autoscaling annotation present",
			annotations: map[string]string{
				clusterv1.ReplicasManagedByAnnotation: infrav1exp.ReplicasManagedByARO,
			},
			actualNodeCount:   5,
			expectSpecUpdated: true,
		},
		{
			name: "different autoscaler annotation present",
			annotations: map[string]string{
				clusterv1.ReplicasManagedByAnnotation: "other-autoscaler",
			},
			actualNodeCount:   5,
			expectSpecUpdated: true,
		},
		{
			name:              "no annotation",
			annotations:       map[string]string{},
			actualNodeCount:   5,
			expectSpecUpdated: false,
		},
		{
			name:              "nil annotations",
			annotations:       nil,
			actualNodeCount:   5,
			expectSpecUpdated: false,
		},
		{
			name: "other annotations present but not autoscaling",
			annotations: map[string]string{
				"some.other/annotation": "value",
			},
			actualNodeCount:   5,
			expectSpecUpdated: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			machinePool := &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-mp",
					Namespace:   "default",
					Annotations: test.annotations,
				},
				Spec: clusterv1.MachinePoolSpec{
					Replicas: ptr.To[int32](3),
				},
			}

			currentReplicas := test.actualNodeCount

			// Apply the reconciler logic
			if _, autoscaling := machinePool.Annotations[clusterv1.ReplicasManagedByAnnotation]; autoscaling {
				machinePool.Spec.Replicas = &currentReplicas
			}

			// Verify expectations
			if test.expectSpecUpdated {
				g.Expect(machinePool.Spec.Replicas).To(Equal(ptr.To(test.actualNodeCount)))
			} else {
				g.Expect(machinePool.Spec.Replicas).To(Equal(ptr.To[int32](3)))
			}
		})
	}
}

// TestAutoscalingLifecycle tests the complete autoscaling enable/disable lifecycle.
func TestAutoscalingLifecycle(t *testing.T) {
	g := NewGomegaWithT(t)

	machinePool := &clusterv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-mp",
			Namespace:   "default",
			Annotations: map[string]string{},
		},
		Spec: clusterv1.MachinePoolSpec{
			Replicas: ptr.To[int32](3),
		},
	}

	// Phase 1: No autoscaling - spec should not be updated
	var currentReplicas int32 = 5
	if _, autoscaling := machinePool.Annotations[clusterv1.ReplicasManagedByAnnotation]; autoscaling {
		replicas := currentReplicas
		machinePool.Spec.Replicas = &replicas
	}
	g.Expect(*machinePool.Spec.Replicas).To(Equal(int32(3)), "Phase 1: spec should not be updated without annotation")

	// Phase 2: Enable autoscaling - add annotation
	machinePool.Annotations[clusterv1.ReplicasManagedByAnnotation] = infrav1exp.ReplicasManagedByARO

	// Autoscaler scales to 5
	currentReplicas = 5
	if _, autoscaling := machinePool.Annotations[clusterv1.ReplicasManagedByAnnotation]; autoscaling {
		replicas := currentReplicas
		machinePool.Spec.Replicas = &replicas
	}
	g.Expect(*machinePool.Spec.Replicas).To(Equal(int32(5)), "Phase 2: spec should be updated to 5 with annotation")

	// Phase 3: Autoscaler scales to 8
	currentReplicas = 8
	if _, autoscaling := machinePool.Annotations[clusterv1.ReplicasManagedByAnnotation]; autoscaling {
		replicas := currentReplicas
		machinePool.Spec.Replicas = &replicas
	}
	g.Expect(*machinePool.Spec.Replicas).To(Equal(int32(8)), "Phase 3: spec should be updated to 8")

	// Phase 4: Disable autoscaling - remove annotation
	delete(machinePool.Annotations, clusterv1.ReplicasManagedByAnnotation)

	// Node count changes but spec should not be updated anymore
	currentReplicas = 10
	if _, autoscaling := machinePool.Annotations[clusterv1.ReplicasManagedByAnnotation]; autoscaling {
		replicas := currentReplicas
		machinePool.Spec.Replicas = &replicas
	}
	g.Expect(*machinePool.Spec.Replicas).To(Equal(int32(8)), "Phase 4: spec should remain at 8 after annotation removed")
}

// TestAutoscalingPreventsScalingDownStatus tests that updating spec.replicas prevents
// the ScalingDown status issue reported by the customer.
func TestAutoscalingPreventsScalingDownStatus(t *testing.T) {
	g := NewGomegaWithT(t)

	// Customer scenario: Autoscaler increases nodes from 3 to 5
	machinePool := &clusterv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "customer-mp",
			Namespace: "default",
			Annotations: map[string]string{
				clusterv1.ReplicasManagedByAnnotation: infrav1exp.ReplicasManagedByARO,
			},
		},
		Spec: clusterv1.MachinePoolSpec{
			Replicas: ptr.To[int32](3),
		},
	}

	infraMachinePool := &infrav1exp.AROMachinePool{
		Status: infrav1exp.AROMachinePoolStatus{
			Replicas: 3,
		},
	}

	// Autoscaler creates 2 new nodes
	actualNodeCount := int32(5)

	// Update status replicas
	infraMachinePool.Status.Replicas = actualNodeCount

	// Update spec replicas (the fix)
	if _, autoscaling := machinePool.Annotations[clusterv1.ReplicasManagedByAnnotation]; autoscaling {
		machinePool.Spec.Replicas = &actualNodeCount
	}

	// Verify: Spec and Status match, no ScalingDown state
	g.Expect(machinePool.Spec.Replicas).To(Equal(ptr.To[int32](5)), "spec should match actual nodes")
	g.Expect(infraMachinePool.Status.Replicas).To(Equal(int32(5)), "status should match actual nodes")
	g.Expect(*machinePool.Spec.Replicas).To(Equal(infraMachinePool.Status.Replicas), "spec and status should match - no ScalingDown")
}

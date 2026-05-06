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

package mutators

import (
	"encoding/json"
	"errors"
	"testing"

	asoredhatopenshiftv1 "github.com/Azure/azure-service-operator/v2/api/redhatopenshift/v1api20240610preview"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
)

// mustMarshalJSON marshals an object to JSON or panics.
func mustMarshalJSON(obj interface{}) []byte {
	data, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	return data
}

func TestReconcileAROAutoscaling(t *testing.T) {
	tests := []struct {
		name        string
		autoScaling map[string]interface{}
		machinePool *clusterv1.MachinePool
		expected    *clusterv1.MachinePool
		expectedErr error
	}{
		{
			name:        "autoscaling disabled, no annotation",
			autoScaling: nil,
			machinePool: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			expected: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
		},
		{
			name:        "autoscaling disabled, removes ARO annotation",
			autoScaling: nil,
			machinePool: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						clusterv1.ReplicasManagedByAnnotation: infrav1exp.ReplicasManagedByARO,
					},
				},
			},
			expected: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
		},
		{
			name:        "autoscaling disabled, leaves other annotation",
			autoScaling: nil,
			machinePool: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						clusterv1.ReplicasManagedByAnnotation: "not-" + infrav1exp.ReplicasManagedByARO,
					},
				},
			},
			expected: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						clusterv1.ReplicasManagedByAnnotation: "not-" + infrav1exp.ReplicasManagedByARO,
					},
				},
			},
		},
		{
			name: "autoscaling enabled with min/max, adds annotation",
			autoScaling: map[string]interface{}{
				"min": int64(3),
				"max": int64(10),
			},
			machinePool: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			expected: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						clusterv1.ReplicasManagedByAnnotation: infrav1exp.ReplicasManagedByARO,
					},
				},
			},
		},
		{
			name: "autoscaling enabled, annotation already set",
			autoScaling: map[string]interface{}{
				"min": int64(3),
				"max": int64(10),
			},
			machinePool: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						clusterv1.ReplicasManagedByAnnotation: infrav1exp.ReplicasManagedByARO,
					},
				},
			},
			expected: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						clusterv1.ReplicasManagedByAnnotation: infrav1exp.ReplicasManagedByARO,
					},
				},
			},
		},
		{
			name: "autoscaling enabled, manager set to something else",
			autoScaling: map[string]interface{}{
				"min": int64(3),
				"max": int64(10),
			},
			machinePool: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mp",
					Annotations: map[string]string{
						clusterv1.ReplicasManagedByAnnotation: "not-" + infrav1exp.ReplicasManagedByARO,
					},
				},
			},
			expectedErr: errors.New("failed to enable autoscaling, replicas are already being managed by not-aro-hcp according to MachinePool mp's cluster.x-k8s.io/replicas-managed-by annotation"),
		},
		{
			name:        "autoscaling enabled with empty config, adds annotation",
			autoScaling: map[string]interface{}{},
			machinePool: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			expected: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
		},
		{
			name: "autoscaling enabled, nil annotations initializes map",
			autoScaling: map[string]interface{}{
				"min": int64(3),
				"max": int64(10),
			},
			machinePool: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: nil,
				},
			},
			expected: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						clusterv1.ReplicasManagedByAnnotation: infrav1exp.ReplicasManagedByARO,
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			// Create HcpOpenShiftClustersNodePool with autoscaling config
			nodePool := &unstructured.Unstructured{}
			nodePool.SetGroupVersionKind(asoredhatopenshiftv1.GroupVersion.WithKind("HcpOpenShiftClustersNodePool"))

			if test.autoScaling != nil {
				err := unstructured.SetNestedMap(nodePool.UnstructuredContent(), test.autoScaling, "spec", "properties", "autoScaling")
				g.Expect(err).NotTo(HaveOccurred())
			}

			err := reconcileAROAutoscaling(nodePool, test.machinePool)

			if test.expectedErr != nil {
				g.Expect(err).To(MatchError(test.expectedErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(cmp.Diff(test.expected, test.machinePool)).To(BeEmpty())
			}
		})
	}
}

func TestSetHcpOpenShiftNodePoolDefaults(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name               string
		aroMachinePool     *infrav1exp.AROMachinePool
		machinePool        *clusterv1.MachinePool
		hcpClusterName     string
		autoScalingConfig  map[string]interface{}
		expectedAnnotation string
		expectedErr        error
	}{
		{
			name: "no autoscaling, no annotation",
			aroMachinePool: &infrav1exp.AROMachinePool{
				Spec: infrav1exp.AROMachinePoolSpec{
					Resources: []runtime.RawExtension{
						{
							Raw: mustMarshalJSON(&unstructured.Unstructured{
								Object: map[string]interface{}{
									"apiVersion": "redhatopenshift.azure.com/v1api20240610preview",
									"kind":       "HcpOpenShiftClustersNodePool",
									"metadata": map[string]interface{}{
										"name": "test-nodepool",
									},
									"spec": map[string]interface{}{
										"properties": map[string]interface{}{},
									},
								},
							}),
						},
					},
				},
			},
			machinePool: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-mp",
					Annotations: map[string]string{},
				},
			},
			hcpClusterName:     "test-cluster",
			autoScalingConfig:  nil,
			expectedAnnotation: "",
		},
		{
			name: "autoscaling enabled, sets annotation",
			aroMachinePool: &infrav1exp.AROMachinePool{
				Spec: infrav1exp.AROMachinePoolSpec{
					Resources: []runtime.RawExtension{
						{
							Raw: mustMarshalJSON(&unstructured.Unstructured{
								Object: map[string]interface{}{
									"apiVersion": "redhatopenshift.azure.com/v1api20240610preview",
									"kind":       "HcpOpenShiftClustersNodePool",
									"metadata": map[string]interface{}{
										"name": "test-nodepool",
									},
									"spec": map[string]interface{}{
										"properties": map[string]interface{}{
											"autoScaling": map[string]interface{}{
												"min": int64(3),
												"max": int64(10),
											},
										},
									},
								},
							}),
						},
					},
				},
			},
			machinePool: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-mp",
					Annotations: map[string]string{},
				},
			},
			hcpClusterName: "test-cluster",
			autoScalingConfig: map[string]interface{}{
				"min": int64(3),
				"max": int64(10),
			},
			expectedAnnotation: infrav1exp.ReplicasManagedByARO,
		},
		{
			name: "no HcpOpenShiftClustersNodePool returns error",
			aroMachinePool: &infrav1exp.AROMachinePool{
				Spec: infrav1exp.AROMachinePoolSpec{
					Resources: []runtime.RawExtension{},
				},
			},
			machinePool: &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-mp",
				},
			},
			hcpClusterName: "test-cluster",
			expectedErr:    ErrNoHcpOpenShiftClustersNodePoolDefined,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			mutator := SetHcpOpenShiftNodePoolDefaults(nil, test.aroMachinePool, test.hcpClusterName, test.machinePool)
			_, err := ApplyMutators(ctx, test.aroMachinePool.Spec.Resources, mutator)

			if test.expectedErr != nil {
				g.Expect(err).To(MatchError(test.expectedErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())

				// Check if annotation was set correctly
				if test.expectedAnnotation != "" {
					g.Expect(test.machinePool.Annotations).To(HaveKey(clusterv1.ReplicasManagedByAnnotation))
					g.Expect(test.machinePool.Annotations[clusterv1.ReplicasManagedByAnnotation]).To(Equal(test.expectedAnnotation))
				} else {
					g.Expect(test.machinePool.Annotations).NotTo(HaveKey(clusterv1.ReplicasManagedByAnnotation))
				}
			}
		})
	}
}

func TestReconcileAROAutoscaling_WithDifferentAutoScalingConfigs(t *testing.T) {
	tests := []struct {
		name             string
		autoScalingField interface{}
		expectEnabled    bool
	}{
		{
			name:             "autoscaling with min and max",
			autoScalingField: map[string]interface{}{"min": int64(1), "max": int64(10)},
			expectEnabled:    true,
		},
		{
			name:             "autoscaling with only min",
			autoScalingField: map[string]interface{}{"min": int64(1)},
			expectEnabled:    true,
		},
		{
			name:             "autoscaling with only max",
			autoScalingField: map[string]interface{}{"max": int64(10)},
			expectEnabled:    true,
		},
		{
			name:             "autoscaling empty object",
			autoScalingField: map[string]interface{}{},
			expectEnabled:    false,
		},
		{
			name:             "autoscaling nil",
			autoScalingField: nil,
			expectEnabled:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			nodePool := &unstructured.Unstructured{}
			nodePool.SetGroupVersionKind(asoredhatopenshiftv1.GroupVersion.WithKind("HcpOpenShiftClustersNodePool"))

			if test.autoScalingField != nil {
				if asMap, ok := test.autoScalingField.(map[string]interface{}); ok {
					err := unstructured.SetNestedMap(nodePool.UnstructuredContent(), asMap, "spec", "properties", "autoScaling")
					g.Expect(err).NotTo(HaveOccurred())
				}
			}

			machinePool := &clusterv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			}

			err := reconcileAROAutoscaling(nodePool, machinePool)
			g.Expect(err).NotTo(HaveOccurred())

			if test.expectEnabled {
				g.Expect(machinePool.Annotations).To(HaveKey(clusterv1.ReplicasManagedByAnnotation))
				g.Expect(machinePool.Annotations[clusterv1.ReplicasManagedByAnnotation]).To(Equal(infrav1exp.ReplicasManagedByARO))
			} else {
				g.Expect(machinePool.Annotations).NotTo(HaveKey(clusterv1.ReplicasManagedByAnnotation))
			}
		})
	}
}

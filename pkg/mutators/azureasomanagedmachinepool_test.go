/*
Copyright 2024 The Kubernetes Authors.

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
	"context"
	"errors"
	"testing"

	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231001"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1alpha "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha1"
)

func TestSetAgentPoolDefaults(t *testing.T) {
	ctx := context.Background()
	g := NewGomegaWithT(t)

	tests := []struct {
		name                  string
		asoManagedMachinePool *infrav1alpha.AzureASOManagedMachinePool
		machinePool           *expv1.MachinePool
		expected              []*unstructured.Unstructured
		expectedErr           error
	}{
		{
			name: "no ManagedClustersAgentPool",
			asoManagedMachinePool: &infrav1alpha.AzureASOManagedMachinePool{
				Spec: infrav1alpha.AzureASOManagedMachinePoolSpec{
					AzureASOManagedMachinePoolTemplateResourceSpec: infrav1alpha.AzureASOManagedMachinePoolTemplateResourceSpec{
						Resources: []runtime.RawExtension{},
					},
				},
			},
			expectedErr: ErrNoManagedClustersAgentPoolDefined,
		},
		{
			name: "success",
			asoManagedMachinePool: &infrav1alpha.AzureASOManagedMachinePool{
				Spec: infrav1alpha.AzureASOManagedMachinePoolSpec{
					AzureASOManagedMachinePoolTemplateResourceSpec: infrav1alpha.AzureASOManagedMachinePoolTemplateResourceSpec{
						Resources: []runtime.RawExtension{
							{
								Raw: apJSON(g, &asocontainerservicev1.ManagedClustersAgentPool{}),
							},
						},
					},
				},
			},
			machinePool: &expv1.MachinePool{
				Spec: expv1.MachinePoolSpec{
					Replicas: ptr.To[int32](1),
					Template: clusterv1.MachineTemplateSpec{
						Spec: clusterv1.MachineSpec{
							Version: ptr.To("vcapi k8s version"),
						},
					},
				},
			},
			expected: []*unstructured.Unstructured{
				apUnstructured(g, &asocontainerservicev1.ManagedClustersAgentPool{
					Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
						OrchestratorVersion: ptr.To("capi k8s version"),
						Count:               ptr.To(1),
					},
				}),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			mutator := SetAgentPoolDefaults(nil, test.machinePool)
			actual, err := ApplyMutators(ctx, test.asoManagedMachinePool.Spec.Resources, mutator)
			if test.expectedErr != nil {
				g.Expect(err).To(MatchError(test.expectedErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			g.Expect(cmp.Diff(test.expected, actual)).To(BeEmpty())
		})
	}
}

func TestSetAgentPoolOrchestratorVersion(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		machinePool *expv1.MachinePool
		agentPool   *asocontainerservicev1.ManagedClustersAgentPool
		expected    *asocontainerservicev1.ManagedClustersAgentPool
		expectedErr error
	}{
		{
			name: "no CAPI opinion",
			machinePool: &expv1.MachinePool{
				Spec: expv1.MachinePoolSpec{
					Template: clusterv1.MachineTemplateSpec{
						Spec: clusterv1.MachineSpec{
							Version: nil,
						},
					},
				},
			},
			agentPool: &asocontainerservicev1.ManagedClustersAgentPool{
				Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
					OrchestratorVersion: ptr.To("user k8s version"),
				},
			},
			expected: &asocontainerservicev1.ManagedClustersAgentPool{
				Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
					OrchestratorVersion: ptr.To("user k8s version"),
				},
			},
		},
		{
			name: "set from CAPI opinion",
			machinePool: &expv1.MachinePool{
				Spec: expv1.MachinePoolSpec{
					Template: clusterv1.MachineTemplateSpec{
						Spec: clusterv1.MachineSpec{
							Version: ptr.To("vcapi k8s version"),
						},
					},
				},
			},
			agentPool: &asocontainerservicev1.ManagedClustersAgentPool{
				Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
					OrchestratorVersion: nil,
				},
			},
			expected: &asocontainerservicev1.ManagedClustersAgentPool{
				Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
					OrchestratorVersion: ptr.To("capi k8s version"),
				},
			},
		},
		{
			name: "user value matching CAPI ok",
			machinePool: &expv1.MachinePool{
				Spec: expv1.MachinePoolSpec{
					Template: clusterv1.MachineTemplateSpec{
						Spec: clusterv1.MachineSpec{
							Version: ptr.To("vcapi k8s version"),
						},
					},
				},
			},
			agentPool: &asocontainerservicev1.ManagedClustersAgentPool{
				Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
					OrchestratorVersion: ptr.To("capi k8s version"),
				},
			},
			expected: &asocontainerservicev1.ManagedClustersAgentPool{
				Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
					OrchestratorVersion: ptr.To("capi k8s version"),
				},
			},
		},
		{
			name: "incompatible",
			machinePool: &expv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mp",
				},
				Spec: expv1.MachinePoolSpec{
					Template: clusterv1.MachineTemplateSpec{
						Spec: clusterv1.MachineSpec{
							Version: ptr.To("vcapi k8s version"),
						},
					},
				},
			},
			agentPool: &asocontainerservicev1.ManagedClustersAgentPool{
				Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
					OrchestratorVersion: ptr.To("user k8s version"),
				},
			},
			expectedErr: Incompatible{
				mutation: mutation{
					location: ".spec.orchestratorVersion",
					val:      "capi k8s version",
					reason:   "because MachinePool mp's spec.template.spec.version is vcapi k8s version",
				},
				userVal: "user k8s version",
			},
		},
	}

	s := runtime.NewScheme()
	NewGomegaWithT(t).Expect(asocontainerservicev1.AddToScheme(s)).To(Succeed())

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			before := test.agentPool.DeepCopy()
			uap := apUnstructured(g, test.agentPool)

			err := setAgentPoolOrchestratorVersion(ctx, test.machinePool, "", uap)
			g.Expect(s.Convert(uap, test.agentPool, nil)).To(Succeed())
			if test.expectedErr != nil {
				g.Expect(err).To(MatchError(test.expectedErr))
				g.Expect(cmp.Diff(before, test.agentPool)).To(BeEmpty()) // errors should never modify the resource.
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(cmp.Diff(test.expected, test.agentPool)).To(BeEmpty())
			}
		})
	}
}

func TestReconcileAutoscaling(t *testing.T) {
	tests := []struct {
		name        string
		autoscaling bool
		machinePool *expv1.MachinePool
		expected    *expv1.MachinePool
		expectedErr error
	}{
		{
			name:        "autoscaling disabled removes aks annotation",
			autoscaling: false,
			machinePool: &expv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						clusterv1.ReplicasManagedByAnnotation: infrav1alpha.ReplicasManagedByAKS,
					},
				},
			},
			expected: &expv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
		},
		{
			name:        "autoscaling disabled leaves other annotation",
			autoscaling: false,
			machinePool: &expv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						clusterv1.ReplicasManagedByAnnotation: "not-" + infrav1alpha.ReplicasManagedByAKS,
					},
				},
			},
			expected: &expv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						clusterv1.ReplicasManagedByAnnotation: "not-" + infrav1alpha.ReplicasManagedByAKS,
					},
				},
			},
		},
		{
			name:        "autoscaling enabled, manager undefined adds annotation",
			autoscaling: true,
			machinePool: &expv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
			},
			expected: &expv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						clusterv1.ReplicasManagedByAnnotation: infrav1alpha.ReplicasManagedByAKS,
					},
				},
			},
		},
		{
			name:        "autoscaling enabled, manager already set",
			autoscaling: true,
			machinePool: &expv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						clusterv1.ReplicasManagedByAnnotation: infrav1alpha.ReplicasManagedByAKS,
					},
				},
			},
			expected: &expv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						clusterv1.ReplicasManagedByAnnotation: infrav1alpha.ReplicasManagedByAKS,
					},
				},
			},
		},
		{
			name:        "autoscaling enabled, manager set to something else",
			autoscaling: true,
			machinePool: &expv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mp",
					Annotations: map[string]string{
						clusterv1.ReplicasManagedByAnnotation: "not-" + infrav1alpha.ReplicasManagedByAKS,
					},
				},
			},
			expectedErr: errors.New("failed to enable autoscaling, replicas are already being managed by not-aks according to MachinePool mp's cluster.x-k8s.io/replicas-managed-by annotation"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			agentPool := &asocontainerservicev1.ManagedClustersAgentPool{
				Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
					EnableAutoScaling: ptr.To(test.autoscaling),
				},
			}

			err := reconcileAutoscaling(apUnstructured(g, agentPool), test.machinePool)

			if test.expectedErr != nil {
				g.Expect(err).To(MatchError(test.expectedErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(cmp.Diff(test.expected, test.machinePool)).To(BeEmpty())
			}
		})
	}
}

func TestSetAgentPoolCount(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name              string
		machinePool       *expv1.MachinePool
		agentPool         *asocontainerservicev1.ManagedClustersAgentPool
		existingAgentPool *asocontainerservicev1.ManagedClustersAgentPool
		expected          *asocontainerservicev1.ManagedClustersAgentPool
		expectedErr       error
	}{
		{
			name: "no CAPI opinion",
			machinePool: &expv1.MachinePool{
				Spec: expv1.MachinePoolSpec{
					Replicas: nil,
				},
			},
			agentPool: &asocontainerservicev1.ManagedClustersAgentPool{
				Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
					Count: ptr.To(2),
				},
			},
			expected: &asocontainerservicev1.ManagedClustersAgentPool{
				Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
					Count: ptr.To(2),
				},
			},
		},
		{
			name: "autoscaling enabled",
			machinePool: &expv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						clusterv1.ReplicasManagedByAnnotation: infrav1alpha.ReplicasManagedByAKS,
					},
				},
				Spec: expv1.MachinePoolSpec{
					Replicas: ptr.To[int32](3),
				},
			},
			agentPool: &asocontainerservicev1.ManagedClustersAgentPool{
				Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
					Count: nil,
				},
			},
			existingAgentPool: &asocontainerservicev1.ManagedClustersAgentPool{
				Status: asocontainerservicev1.ManagedClusters_AgentPool_STATUS{
					Count: ptr.To(2),
				},
			},
			expected: &asocontainerservicev1.ManagedClustersAgentPool{
				Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
					Count: nil,
				},
			},
		},
		{
			name: "set from CAPI opinion",
			machinePool: &expv1.MachinePool{
				Spec: expv1.MachinePoolSpec{
					Replicas: ptr.To[int32](1),
				},
			},
			agentPool: &asocontainerservicev1.ManagedClustersAgentPool{
				Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
					Count: nil,
				},
			},
			expected: &asocontainerservicev1.ManagedClustersAgentPool{
				Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
					Count: ptr.To(1),
				},
			},
		},
		{
			name: "user value matching CAPI ok",
			machinePool: &expv1.MachinePool{
				Spec: expv1.MachinePoolSpec{
					Replicas: ptr.To[int32](1),
				},
			},
			agentPool: &asocontainerservicev1.ManagedClustersAgentPool{
				Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
					Count: ptr.To(1),
				},
			},
			expected: &asocontainerservicev1.ManagedClustersAgentPool{
				Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
					Count: ptr.To(1),
				},
			},
		},
		{
			name: "incompatible",
			machinePool: &expv1.MachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mp",
				},
				Spec: expv1.MachinePoolSpec{
					Replicas: ptr.To[int32](1),
				},
			},
			agentPool: &asocontainerservicev1.ManagedClustersAgentPool{
				Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
					Count: ptr.To(2),
				},
			},
			expectedErr: Incompatible{
				mutation: mutation{
					location: ".spec.count",
					val:      int64(1),
					reason:   "because MachinePool mp's spec.replicas is 1",
				},
				userVal: int64(2),
			},
		},
	}

	s := runtime.NewScheme()
	NewGomegaWithT(t).Expect(asocontainerservicev1.AddToScheme(s)).To(Succeed())

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			var c client.Client
			if test.existingAgentPool != nil {
				c = fakeclient.NewClientBuilder().
					WithScheme(s).
					WithObjects(test.existingAgentPool).
					Build()
			}

			before := test.agentPool.DeepCopy()
			uap := apUnstructured(g, test.agentPool)

			err := setAgentPoolCount(ctx, c, test.machinePool, "", uap)
			g.Expect(s.Convert(uap, test.agentPool, nil)).To(Succeed())
			if test.expectedErr != nil {
				g.Expect(err).To(MatchError(test.expectedErr))
				g.Expect(cmp.Diff(before, test.agentPool)).To(BeEmpty()) // errors should never modify the resource.
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(cmp.Diff(test.expected, test.agentPool)).To(BeEmpty())
			}
		})
	}
}

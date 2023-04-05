/*
Copyright 2019 The Kubernetes Authors.

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
	"context"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("AzureMachineReconciler", func() {
	BeforeEach(func() {})
	AfterEach(func() {})

	Context("Reconcile an AzureMachine", func() {
		It("should not error with minimal set up", func() {
			reconciler := NewAzureMachineReconciler(testEnv, testEnv.GetEventRecorderFor("azuremachine-reconciler"), reconciler.DefaultLoopTimeout, "")

			By("Calling reconcile")
			name := test.RandomName("foo", 10)
			instance := &infrav1.AzureMachine{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
			result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: client.ObjectKey{
					Namespace: instance.Namespace,
					Name:      instance.Name,
				},
			})

			Expect(err).To(BeNil())
			Expect(result.RequeueAfter).To(BeZero())
		})
	})
})

func TestConditions(t *testing.T) {
	g := NewWithT(t)
	scheme := setupScheme(g)

	testcases := []struct {
		name               string
		clusterStatus      clusterv1.ClusterStatus
		machine            *clusterv1.Machine
		azureMachine       *infrav1.AzureMachine
		expectedConditions []clusterv1.Condition
	}{
		{
			name: "cluster infrastructure is not ready yet",
			clusterStatus: clusterv1.ClusterStatus{
				InfrastructureReady: false,
			},
			machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "my-cluster",
					},
					Name: "my-machine",
				},
			},
			azureMachine: &infrav1.AzureMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "azure-test1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "test1",
						},
					},
				},
			},
			expectedConditions: []clusterv1.Condition{{
				Type:     "VMRunning",
				Status:   corev1.ConditionFalse,
				Severity: clusterv1.ConditionSeverityInfo,
				Reason:   "WaitingForClusterInfrastructure",
			}},
		},
		{
			name: "bootstrap data secret reference is not yet available",
			clusterStatus: clusterv1.ClusterStatus{
				InfrastructureReady: true,
			},
			machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "my-cluster",
					},
					Name: "my-machine",
				},
			},
			azureMachine: &infrav1.AzureMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "azure-test1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "test1",
						},
					},
				},
			},
			expectedConditions: []clusterv1.Condition{{
				Type:     "VMRunning",
				Status:   corev1.ConditionFalse,
				Severity: clusterv1.ConditionSeverityInfo,
				Reason:   "WaitingForBootstrapData",
			}},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
				},
				Status: tc.clusterStatus,
			}
			azureCluster := &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
					},
				},
			}
			initObjects := []runtime.Object{
				cluster,
				tc.machine,
				azureCluster,
				tc.azureMachine,
			}
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()
			recorder := record.NewFakeRecorder(10)

			reconciler := NewAzureMachineReconciler(client, recorder, reconciler.DefaultLoopTimeout, "")

			clusterScope, err := scope.NewClusterScope(context.TODO(), scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				Client:       client,
				Cluster:      cluster,
				AzureCluster: azureCluster,
			})
			g.Expect(err).NotTo(HaveOccurred())

			machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
				Client:       client,
				ClusterScope: clusterScope,
				Machine:      tc.machine,
				AzureMachine: tc.azureMachine,
				Cache:        &scope.MachineCache{},
			})
			g.Expect(err).NotTo(HaveOccurred())

			_, err = reconciler.reconcileNormal(context.TODO(), machineScope, clusterScope)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(len(machineScope.AzureMachine.GetConditions())).To(Equal(len(tc.expectedConditions)))
			for i, c := range machineScope.AzureMachine.GetConditions() {
				g.Expect(conditionsMatch(c, tc.expectedConditions[i])).To(BeTrue())
			}
		})
	}
}

func conditionsMatch(i, j clusterv1.Condition) bool {
	return i.Type == j.Type &&
		i.Status == j.Status &&
		i.Reason == j.Reason &&
		i.Severity == j.Severity
}

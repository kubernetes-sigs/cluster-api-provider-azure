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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/record"
)

var _ = Describe("AzureMachineReconciler", func() {
	BeforeEach(func() {})
	AfterEach(func() {})

	Context("Reconcile an AzureMachine", func() {
		It("should not error with minimal set up", func() {
			reconciler := &AzureMachineReconciler{
				Client: testEnv,
				Log:    testEnv.Log,
			}

			By("Calling reconcile")
			name := test.RandomName("foo", 10)
			instance := &infrav1.AzureMachine{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
			result, err := reconciler.Reconcile(ctrl.Request{
				NamespacedName: client.ObjectKey{
					Namespace: instance.Namespace,
					Name:      instance.Name,
				},
			})

			Expect(err).To(BeNil())
			Expect(result.RequeueAfter).To(BeZero())
		})

		It("should exit early if the cluster is paused", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			logListener := record.NewListener(testEnv.LogRecorder)
			del := logListener.Listen()
			defer del()

			cluster, _, del := createPausedOwningClusterAndAzCluster(ctx)
			defer del()

			azureMachineName := test.RandomName("azmachine", 10)
			machine := &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Labels:    map[string]string{clusterv1.ClusterLabelName: cluster.Name},
					Name:      test.RandomName("machine", 10),
					Namespace: cluster.Namespace,
				},
				Spec: clusterv1.MachineSpec{
					ClusterName: cluster.Name,
					InfrastructureRef: corev1.ObjectReference{
						APIVersion: infrav1.GroupVersion.String(),
						Kind:       "AzureMachine",
						Name:       azureMachineName,
						Namespace:  cluster.Namespace,
					},
				},
			}
			Expect(testEnv.Create(ctx, machine)).To(Succeed())
			defer func() {
				err := testEnv.Delete(ctx, machine)
				Expect(err).NotTo(HaveOccurred())
			}()

			azureMachine := &infrav1.AzureMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      test.RandomName("azmachine", 10),
					Namespace: cluster.Namespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       machine.Name,
							UID:        machine.GetUID(),
						},
					},
				},
			}
			Expect(testEnv.Create(ctx, azureMachine)).To(Succeed())
			defer func() {
				err := testEnv.Delete(ctx, azureMachine)
				Expect(err).NotTo(HaveOccurred())
			}()

			Eventually(logListener.GetEntries).Should(ContainElement(
				record.LogEntry{
					LogFunc: "Info",
					Values: []interface{}{
						"namespace",
						cluster.Namespace,
						"azureMachine",
						azureMachine.Name,
						"machine",
						machine.Name,
						"cluster",
						cluster.Name,
						"msg",
						"AzureMachine or linked Cluster is marked as paused. Won't reconcile",
					},
				}))

		})
	})
})

func createPausedOwningClusterAndAzCluster(ctx context.Context) (*clusterv1.Cluster, *infrav1.AzureCluster, func()) {
	azClusterName := test.RandomName("azcluster", 10)
	clusterName := test.RandomName("cluster", 10)
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: "default",
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &corev1.ObjectReference{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       "AzureCluster",
				Name:       azClusterName,
				Namespace:  "default",
			},
			Paused: true,
		},
	}
	Expect(testEnv.Create(ctx, cluster)).To(Succeed())
	cleanupCluster := func() error {
		return testEnv.Delete(ctx, cluster)
	}

	azCluster := &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      azClusterName,
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
					Name:       cluster.Name,
					UID:        cluster.GetUID(),
				},
			},
		},
	}
	Expect(testEnv.Create(ctx, azCluster)).To(Succeed())
	cleanupAzCluster := func() error {
		return testEnv.Delete(ctx, azCluster)
	}

	return cluster, azCluster, func() {
		err1 := cleanupCluster()
		err2 := cleanupAzCluster()
		Expect(err1).ToNot(HaveOccurred())
		Expect(err2).ToNot(HaveOccurred())
	}
}

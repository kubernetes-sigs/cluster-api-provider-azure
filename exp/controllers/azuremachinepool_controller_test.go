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

package controllers

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("AzureMachinePoolReconciler", func() {
	var (
		name       = "foo"
		namespace  = "default"
		secretName = "foo-secret"
		cluster    *clusterv1.Cluster
		secret     *corev1.Secret

		azureMachinePool *infrav1exp.AzureMachinePool
	)

	BeforeEach(func() {
		cluster = getValidCluster(name, namespace)
		secret = getSecret(secretName, namespace)
		azureMachinePool = getAzureMachinePool(name, namespace, "")
	})

	AfterEach(func() {})

	Context("Reconcile an AzureMachinePool", func() {
		It("should not error with minimal set up", func() {
			reconciler := NewAzureMachinePoolReconciler(testEnv, testEnv.GetEventRecorderFor("azuremachinepool-reconciler"),
				reconciler.Timeouts{}, "")

			By("Calling reconcile")
			instance := &infrav1exp.AzureMachinePool{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}
			result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: client.ObjectKey{
					Namespace: instance.Namespace,
					Name:      instance.Name,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())
		})

		It("should set proper machinePool status when machinePool has invalid configuration", func() {
			cluster.Status.InfrastructureReady = true

			requiredObjects := [3]client.Object{secret, cluster, azureMachinePool}
			for _, obj := range requiredObjects {
				err := testEnv.Create(context.Background(), obj)
				Expect(err).To(BeNil())
			}

			machinePoolScope, _ := scope.NewMachinePoolScope(scope.MachinePoolScopeParams{
				Client:           testEnv.GetClient(),
				MachinePool:      getMachinePool(name, namespace, secretName),
				AzureMachinePool: azureMachinePool,
			})
			machinePoolScope.VMSkuResolver = func(scope *scope.MachinePoolScope, context context.Context) (resourceskus.SKU, error) {
				return resourceskus.SKU{}, azure.WithTerminalError(errors.New("resource sku with name 'Standard_D2s_v3' and category 'virtualMachines' not found in location 'eastus'"))
			}
			reconciler := NewAzureMachinePoolReconciler(testEnv, testEnv.GetEventRecorderFor("azuremachinepool-reconciler"), reconciler.Timeouts{}, "")

			By("reconciler.reconcileNormal ")
			cluster.Status.InfrastructureReady = true
			result, err := reconciler.reconcileNormal(context.Background(), machinePoolScope, cluster)

			Expect(err).To(BeNil())
			Expect(result.Requeue).Should(BeFalse())
			Expect(*machinePoolScope.AzureMachinePool.Status.FailureReason).Should(Equal(capierrors.InvalidConfigurationMachinePoolError))
		})

		It("should not reconcile if machinePool has status.FailureReason", func() {
			failureReason := capierrors.InvalidConfigurationMachinePoolError
			failureMessage := "invalid configuration"
			azureMachinePool.Status = infrav1exp.AzureMachinePoolStatus{FailureReason: &failureReason, FailureMessage: &failureMessage}

			machinePoolScope, _ := scope.NewMachinePoolScope(scope.MachinePoolScopeParams{
				Client:           testEnv.GetClient(),
				MachinePool:      getMachinePool(name, namespace, secretName),
				AzureMachinePool: azureMachinePool,
			})
			reconciler := NewAzureMachinePoolReconciler(testEnv, testEnv.GetEventRecorderFor("azuremachinepool-reconciler"),
				reconciler.Timeouts{}, "")

			By("reconciler.reconcileNormal ")
			result, err := reconciler.reconcileNormal(context.Background(), machinePoolScope, getValidCluster(azureMachinePool.Name, azureMachinePool.Namespace))
			Expect(err).To(BeNil())
			Expect(result.RequeueAfter).To(BeZero(), "should not be re-queued since terminal failure")
		})
	})
})

func getValidCluster(name, namespace string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: clusterv1.ClusterSpec{
			Paused: true,
			InfrastructureRef: &corev1.ObjectReference{
				Kind:      "AzureCluster",
				Name:      name,
				Namespace: namespace,
			},
		},
	}
}

func getMachinePool(name, namespace, secretName string) *expv1.MachinePool {
	return &expv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: name,
			},
		},
		Spec: expv1.MachinePoolSpec{
			ClusterName: name,
			Template: clusterv1.MachineTemplateSpec{
				Spec: clusterv1.MachineSpec{
					ClusterName: name,
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: &secretName,
					},
				},
			},
		},
	}
}

func getSecret(secretName, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"value":        []byte("fooSecret"),
			"clientSecret": []byte("fooSecret"),
		},
	}
}

func getIdentity() *infrav1.AzureClusterIdentity {
	return &infrav1.AzureClusterIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-identity",
			Namespace: "default",
		},
		Spec: infrav1.AzureClusterIdentitySpec{
			Type: infrav1.ServicePrincipal,
			ClientSecret: corev1.SecretReference{
				Name:      "fooSecret",
				Namespace: "default",
			},
			TenantID: "fake-tenantid",
		},
	}
}

func getAzureMachinePool(name, namespace, machinePoolName string) *infrav1exp.AzureMachinePool {
	azureMachinePoolInstance := &infrav1exp.AzureMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: infrav1exp.AzureMachinePoolSpec{
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				Image: &infrav1.Image{},
			},
		},
	}

	if machinePoolName != "" {
		azureMachinePoolInstance.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
			{
				Kind:       "MachinePool",
				APIVersion: expv1.GroupVersion.String(),
				Name:       machinePoolName,
			},
		}
	}
	return azureMachinePoolInstance
}

func TestAzureMachinePoolReconcilePaused(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()

	sb := runtime.NewSchemeBuilder(
		clusterv1.AddToScheme,
		infrav1.AddToScheme,
		expv1.AddToScheme,
		infrav1exp.AddToScheme,
		corev1.AddToScheme,
	)
	s := runtime.NewScheme()
	g.Expect(sb.AddToScheme(s)).To(Succeed())
	c := fake.NewClientBuilder().
		WithScheme(s).
		Build()

	recorder := record.NewFakeRecorder(1)
	reconciler := NewAzureMachinePoolReconciler(c, recorder, reconciler.Timeouts{}, "")
	name := test.RandomName("paused", 10)
	namespace := "default"

	cluster := getValidCluster(name, namespace)
	g.Expect(c.Create(ctx, cluster)).To(Succeed())

	azCluster := &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: infrav1.AzureClusterSpec{
			AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
				SubscriptionID: "something",
				IdentityRef: &corev1.ObjectReference{
					Name:      "fake-identity",
					Namespace: "default",
					Kind:      "AzureClusterIdentity",
				},
			},
		},
	}
	g.Expect(c.Create(ctx, azCluster)).To(Succeed())

	fakeIdentity := getIdentity()
	secretName := "fooSecret"
	fakeSecret := getSecret(secretName, namespace)
	g.Expect(c.Create(ctx, fakeIdentity)).To(Succeed())
	g.Expect(c.Create(ctx, fakeSecret)).To(Succeed())

	mp := getMachinePool(name, namespace, secretName)
	g.Expect(c.Create(ctx, mp)).To(Succeed())

	instance := getAzureMachinePool(name, namespace, mp.Name)
	g.Expect(c.Create(ctx, instance)).To(Succeed())

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{
			Namespace: instance.Namespace,
			Name:      instance.Name,
		},
	})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.RequeueAfter).To(BeZero())
}

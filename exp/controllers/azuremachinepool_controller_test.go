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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("AzureMachinePoolReconciler", func() {
	BeforeEach(func() {})
	AfterEach(func() {})

	Context("Reconcile an AzureMachinePool", func() {
		It("should not error with minimal set up", func() {
			reconciler := NewAzureMachinePoolReconciler(testEnv, testEnv.GetEventRecorderFor("azuremachinepool-reconciler"),
				reconciler.DefaultLoopTimeout, "")
			By("Calling reconcile")
			instance := &infrav1exp.AzureMachinePool{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}
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

func TestAzureMachinePoolReconcilePaused(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()

	sb := runtime.NewSchemeBuilder(
		clusterv1.AddToScheme,
		infrav1.AddToScheme,
		expv1.AddToScheme,
		infrav1exp.AddToScheme,
	)
	s := runtime.NewScheme()
	g.Expect(sb.AddToScheme(s)).To(Succeed())
	c := fake.NewClientBuilder().
		WithScheme(s).
		Build()

	recorder := record.NewFakeRecorder(1)

	reconciler := NewAzureMachinePoolReconciler(c, recorder, reconciler.DefaultLoopTimeout, "")
	name := test.RandomName("paused", 10)
	namespace := "default"

	cluster := &clusterv1.Cluster{
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
	g.Expect(c.Create(ctx, cluster)).To(Succeed())

	azCluster := &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: infrav1.AzureClusterSpec{
			AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
				SubscriptionID: "something",
			},
		},
	}
	g.Expect(c.Create(ctx, azCluster)).To(Succeed())

	mp := &expv1.MachinePool{
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
				},
			},
		},
	}
	g.Expect(c.Create(ctx, mp)).To(Succeed())

	instance := &infrav1exp.AzureMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind:       "MachinePool",
					APIVersion: expv1.GroupVersion.String(),
					Name:       mp.Name,
				},
			},
		},
	}
	g.Expect(c.Create(ctx, instance)).To(Succeed())

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{
			Namespace: instance.Namespace,
			Name:      instance.Name,
		},
	})

	g.Expect(err).To(BeNil())
	g.Expect(result.RequeueAfter).To(BeZero())
}

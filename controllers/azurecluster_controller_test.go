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

	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("AzureClusterReconciler", func() {
	BeforeEach(func() {})
	AfterEach(func() {})

	Context("Reconcile an AzureCluster", func() {
		It("should not error with minimal set up", func() {
			reconciler := NewAzureClusterReconciler(testEnv, testEnv.GetEventRecorderFor("azurecluster-reconciler"), reconciler.DefaultLoopTimeout, "")
			By("Calling reconcile")
			name := test.RandomName("foo", 10)
			instance := &infrav1.AzureCluster{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
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

func TestAzureClusterReconcilePaused(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()

	sb := runtime.NewSchemeBuilder(
		clusterv1.AddToScheme,
		infrav1.AddToScheme,
		asoresourcesv1.AddToScheme,
	)
	s := runtime.NewScheme()
	g.Expect(sb.AddToScheme(s)).To(Succeed())
	c := fake.NewClientBuilder().
		WithScheme(s).
		Build()

	recorder := record.NewFakeRecorder(1)

	reconciler := NewAzureClusterReconciler(c, recorder, reconciler.DefaultLoopTimeout, "")
	name := test.RandomName("paused", 10)
	namespace := "default"

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: clusterv1.ClusterSpec{
			Paused: true,
		},
	}
	g.Expect(c.Create(ctx, cluster)).To(Succeed())

	instance := &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind:       "Cluster",
					APIVersion: clusterv1.GroupVersion.String(),
					Name:       cluster.Name,
					UID:        cluster.UID,
				},
			},
		},
		Spec: infrav1.AzureClusterSpec{
			AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
				SubscriptionID: "something",
			},
			ResourceGroup: name,
		},
	}
	g.Expect(c.Create(ctx, instance)).To(Succeed())

	rg := &asoresourcesv1.ResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	g.Expect(c.Create(ctx, rg)).To(Succeed())

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{
			Namespace: instance.Namespace,
			Name:      instance.Name,
		},
	})

	g.Expect(err).To(BeNil())
	g.Expect(result.RequeueAfter).To(BeZero())

	g.Eventually(recorder.Events).Should(Receive(Equal("Normal ClusterPaused AzureCluster or linked Cluster is marked as paused. Won't reconcile normally")))
}

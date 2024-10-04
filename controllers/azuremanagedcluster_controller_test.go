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

package controllers

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
)

func TestAzureManagedClusterController(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	scheme := runtime.NewScheme()
	g.Expect(infrav1.AddToScheme(scheme)).To(Succeed())
	g.Expect(clusterv1.AddToScheme(scheme)).To(Succeed())

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-capi-cluster",
			Namespace: "fake-namespace",
		},
		Spec: clusterv1.ClusterSpec{
			ControlPlaneRef: &corev1.ObjectReference{
				Name: "fake-control-plane",
			},
		},
	}
	cp := &infrav1.AzureManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-control-plane",
			Namespace: "fake-namespace",
		},
		Spec: infrav1.AzureManagedControlPlaneSpec{
			ControlPlaneEndpoint: clusterv1.APIEndpoint{
				Host: "fake-host",
				Port: int32(8080),
			},
		},
	}
	aksCluster := &infrav1.AzureManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-cluster",
			Namespace: "fake-namespace",
		},
	}

	g.Expect(controllerutil.SetOwnerReference(cluster, aksCluster, scheme)).To(Succeed())
	fakeclient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, cp, aksCluster).WithStatusSubresource(cp, cluster, aksCluster).Build()
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "fake-cluster",
			Namespace: "fake-namespace"},
	}
	to := reconciler.Timeouts{}
	rec := &AzureManagedClusterReconciler{
		Timeouts: to,
		Client:   fakeclient,
	}

	_, err := rec.Reconcile(ctx, req)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(rec.Get(ctx, client.ObjectKeyFromObject(aksCluster), aksCluster)).To(Succeed())
	g.Expect(aksCluster.Spec.ControlPlaneEndpoint).To(Equal(cp.Spec.ControlPlaneEndpoint))
	g.Expect(aksCluster.Status.Ready).To(BeTrue())
}

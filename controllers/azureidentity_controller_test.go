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

	aadpodv1 "github.com/Azure/aad-pod-identity/pkg/apis/aadpodidentity/v1"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	featuregatetesting "k8s.io/component-base/featuregate/testing"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/system"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capifeature "sigs.k8s.io/cluster-api/feature"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAzureIdentityControllerReconcileAzureCluster(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "fake-cluster", Namespace: "fake-ns"}}
	scheme, err := newScheme()
	g.Expect(err).ToNot(HaveOccurred())

	azureCluster := &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-cluster",
			Namespace: "fake-ns",
		},
	}

	bindings := getFakeAzureIdentityBinding()
	azIdentity := getFakeAzureIdentity()
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(azureCluster, bindings, azIdentity).Build()
	aiRec := &AzureIdentityReconciler{
		Client:           client,
		Recorder:         record.NewFakeRecorder(42),
		ReconcileTimeout: reconciler.DefaultLoopTimeout,
		WatchFilterValue: "fake",
	}
	_, err = aiRec.Reconcile(ctx, req)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestAzureIdentityControllerReconcileAzureManagedControlPlane(t *testing.T) {
	g := NewWithT(t)
	defer featuregatetesting.SetFeatureGateDuringTest(t, capifeature.Gates, capifeature.MachinePool, true)()
	ctx := context.Background()
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "fake-controlplane", Namespace: "fake-ns"}}
	scheme, err := newScheme()
	g.Expect(err).ToNot(HaveOccurred())

	azureManagedControlPlane := &infrav1.AzureManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-controlplane",
			Namespace: "fake-ns",
		},
	}

	bindings := getFakeAzureIdentityBinding()
	azIdentity := getFakeAzureIdentity()

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(azureManagedControlPlane, bindings, azIdentity).Build()
	aiRec := &AzureIdentityReconciler{
		Client:           client,
		Recorder:         record.NewFakeRecorder(42),
		ReconcileTimeout: reconciler.DefaultLoopTimeout,
		WatchFilterValue: "fake",
	}
	_, err = aiRec.Reconcile(ctx, req)
	g.Expect(err).ToNot(HaveOccurred())
}

func getFakeAzureIdentityBinding() *aadpodv1.AzureIdentityBinding {
	return &aadpodv1.AzureIdentityBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-binding",
			Namespace: system.GetManagerNamespace(),
			Labels: map[string]string{
				clusterv1.ClusterNameLabel:    "another-fake-cluster",
				infrav1.ClusterLabelNamespace: "fake-ns",
			},
		},
		Spec: aadpodv1.AzureIdentityBindingSpec{
			AzureIdentity: "managedIdentity",
		},
	}
}

func getFakeAzureIdentity() *aadpodv1.AzureIdentity {
	return &aadpodv1.AzureIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "managedIdentity",
			Namespace: system.GetManagerNamespace(),
		},
	}
}

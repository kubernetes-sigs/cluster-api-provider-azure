//go:build e2e
// +build e2e

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

package e2e

import (
	"context"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AKSAdoptSpecInput struct {
	ApplyInput   clusterctl.ApplyClusterTemplateAndWaitInput
	ApplyResult  *clusterctl.ApplyClusterTemplateAndWaitResult
	Cluster      *clusterv1.Cluster
	MachinePools []*expv1.MachinePool
}

// AKSAdoptSpec tests adopting an existing AKS cluster into management by CAPZ. It first relies on a CAPZ AKS
// cluster having already been created. Then, it will orphan that cluster such that the CAPI and CAPZ
// resources are deleted but the Azure resources remain. Finally, it applies the cluster template again and
// waits for the cluster to become ready.
func AKSAdoptSpec(ctx context.Context, inputGetter func() AKSAdoptSpecInput) {
	input := inputGetter()

	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	updateResource := []any{"30s", "5s"}

	waitForNoBlockMove := func(obj client.Object) {
		waitForBlockMoveGone := []any{"30s", "5s"}
		Eventually(func(g Gomega) {
			g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
			g.Expect(obj.GetAnnotations()).NotTo(HaveKey(clusterctlv1.BlockMoveAnnotation))
		}, waitForBlockMoveGone...).Should(Succeed())
	}

	removeFinalizers := func(obj client.Object) {
		Eventually(func(g Gomega) {
			g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
			obj.SetFinalizers([]string{})
			g.Expect(mgmtClient.Update(ctx, obj)).To(Succeed())
		}, updateResource...).Should(Succeed())
	}

	waitForImmediateDelete := []any{"30s", "5s"}
	beginDelete := func(obj client.Object) {
		Eventually(func(g Gomega) {
			err := mgmtClient.Delete(ctx, obj)
			g.Expect(err).NotTo(HaveOccurred())
		}, updateResource...).Should(Succeed())
	}
	shouldNotExist := func(obj client.Object) {
		waitForGone := []any{"30s", "5s"}
		Eventually(func(g Gomega) {
			err := mgmtClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, waitForGone...).Should(Succeed())
	}
	deleteAndWait := func(obj client.Object) {
		Eventually(func(g Gomega) {
			err := mgmtClient.Delete(ctx, obj)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, waitForImmediateDelete...).Should(Succeed())
	}

	cluster := input.Cluster
	Eventually(func(g Gomega) {
		g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)).To(Succeed())
		cluster.Spec.Paused = true
		g.Expect(mgmtClient.Update(ctx, cluster)).To(Succeed())
	}, updateResource...).Should(Succeed())

	// wait for the pause to take effect before deleting anything
	amcp := &infrav1.AzureManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Spec.ControlPlaneRef.Namespace,
			Name:      cluster.Spec.ControlPlaneRef.Name,
		},
	}
	waitForNoBlockMove(amcp)
	for _, mp := range input.MachinePools {
		ammp := &infrav1.AzureManagedMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: mp.Spec.Template.Spec.InfrastructureRef.Namespace,
				Name:      mp.Spec.Template.Spec.InfrastructureRef.Name,
			},
		}
		waitForNoBlockMove(ammp)
	}

	beginDelete(cluster)

	for _, mp := range input.MachinePools {
		beginDelete(mp)

		ammp := &infrav1.AzureManagedMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: mp.Spec.Template.Spec.InfrastructureRef.Namespace,
				Name:      mp.Spec.Template.Spec.InfrastructureRef.Name,
			},
		}
		removeFinalizers(ammp)
		deleteAndWait(ammp)

		removeFinalizers(mp)
		shouldNotExist(mp)
	}

	removeFinalizers(amcp)
	deleteAndWait(amcp)
	// AzureManagedCluster never gets a finalizer
	deleteAndWait(&infrav1.AzureManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: cluster.Spec.InfrastructureRef.Namespace,
			Name:      cluster.Spec.InfrastructureRef.Name,
		},
	})

	removeFinalizers(cluster)
	shouldNotExist(cluster)

	clusterctl.ApplyClusterTemplateAndWait(ctx, input.ApplyInput, input.ApplyResult)
}

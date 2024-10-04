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
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231001"
	asocontainerservicev1preview "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231102preview"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

type AKSPatchSpecInput struct {
	Cluster       *clusterv1.Cluster
	MachinePools  []*expv1.MachinePool
	WaitForUpdate []interface{}
}

func AKSPatchSpec(ctx context.Context, inputGetter func() AKSPatchSpecInput) {
	input := inputGetter()

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	Expect(err).NotTo(HaveOccurred())

	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	infraControlPlane := &infrav1.AzureManagedControlPlane{}
	err = mgmtClient.Get(ctx, client.ObjectKey{
		Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace,
		Name:      input.Cluster.Spec.ControlPlaneRef.Name,
	}, infraControlPlane)
	Expect(err).NotTo(HaveOccurred())

	managedClustersClient, err := armcontainerservice.NewManagedClustersClient(getSubscriptionID(Default), cred, nil)
	Expect(err).NotTo(HaveOccurred())

	var wg sync.WaitGroup

	type CheckInput struct {
		exist      map[string]string
		doNotExist []string
	}

	checkAnnotations := func(obj client.Object, c CheckInput) func(Gomega) {
		return func(g Gomega) {
			err := mgmtClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)
			g.Expect(err).NotTo(HaveOccurred())
			for k, v := range c.exist {
				g.Expect(obj.GetAnnotations()).To(HaveKeyWithValue(k, v))
			}
			for _, k := range c.doNotExist {
				g.Expect(obj.GetAnnotations()).NotTo(HaveKey(k))
			}
		}
	}

	wg.Add(1)
	go func() {
		defer GinkgoRecover()
		defer wg.Done()

		managedCluster := &asocontainerservicev1.ManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: infraControlPlane.Namespace,
				Name:      infraControlPlane.Name,
			},
		}

		var initialPatches []string
		By("Deleting patches for control plane")
		Eventually(func(g Gomega) {
			g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(infraControlPlane), infraControlPlane)).To(Succeed())
			initialPatches = infraControlPlane.Spec.ASOManagedClusterPatches
			infraControlPlane.Spec.ASOManagedClusterPatches = nil
			g.Expect(mgmtClient.Update(ctx, infraControlPlane)).To(Succeed())
		}, inputGetter().WaitForUpdate...).Should(Succeed())

		By("Creating patches for control plane")
		Eventually(func(g Gomega) {
			g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(infraControlPlane), infraControlPlane)).To(Succeed())
			infraControlPlane.Spec.ASOManagedClusterPatches = []string{
				`{"metadata": {"annotations": {"capzpatchtest": "value"}}}`,
			}
			g.Expect(mgmtClient.Update(ctx, infraControlPlane)).To(Succeed())
		}, inputGetter().WaitForUpdate...).Should(Succeed())
		Eventually(checkAnnotations(managedCluster, CheckInput{exist: map[string]string{"capzpatchtest": "value"}}), input.WaitForUpdate...).Should(Succeed())

		By("Updating patches for control plane")
		Eventually(func(g Gomega) {
			g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(infraControlPlane), infraControlPlane)).To(Succeed())
			infraControlPlane.Spec.ASOManagedClusterPatches = append(infraControlPlane.Spec.ASOManagedClusterPatches, `{"metadata": {"annotations": {"capzpatchtest": null}}}`)
			g.Expect(mgmtClient.Update(ctx, infraControlPlane)).To(Succeed())
		}, inputGetter().WaitForUpdate...).Should(Succeed())
		Eventually(checkAnnotations(managedCluster, CheckInput{doNotExist: []string{"capzpatchtest"}}), input.WaitForUpdate...).Should(Succeed())

		By("Enabling preview features on the control plane")
		var infraControlPlane = &infrav1.AzureManagedControlPlane{}
		Eventually(func(g Gomega) {
			err = mgmtClient.Get(ctx, client.ObjectKey{Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace, Name: input.Cluster.Spec.ControlPlaneRef.Name}, infraControlPlane)
			g.Expect(err).NotTo(HaveOccurred())
			infraControlPlane.Spec.EnablePreviewFeatures = ptr.To(true)
			g.Expect(mgmtClient.Update(ctx, infraControlPlane)).To(Succeed())
		}, input.WaitForUpdate...).Should(Succeed())

		Eventually(func(g Gomega) {
			resp, err := managedClustersClient.Get(ctx, infraControlPlane.Spec.ResourceGroupName, infraControlPlane.Name, nil)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(resp.Properties.ProvisioningState).To(Equal(ptr.To("Succeeded")))
		}, input.WaitForUpdate...).Should(Succeed())

		By("Patching a preview feature on the control plane")
		Eventually(func(g Gomega) {
			err = mgmtClient.Get(ctx, client.ObjectKey{Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace, Name: input.Cluster.Spec.ControlPlaneRef.Name}, infraControlPlane)
			g.Expect(err).NotTo(HaveOccurred())
			infraControlPlane.Spec.ASOManagedClusterPatches = append(infraControlPlane.Spec.ASOManagedClusterPatches, `{"spec": {"enableNamespaceResources": true}}`)
			g.Expect(mgmtClient.Update(ctx, infraControlPlane)).To(Succeed())
		}, input.WaitForUpdate...).Should(Succeed())

		asoManagedCluster := &asocontainerservicev1preview.ManagedCluster{}
		Eventually(func(g Gomega) {
			err = mgmtClient.Get(ctx, client.ObjectKey{Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace, Name: infraControlPlane.Name}, asoManagedCluster)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(asoManagedCluster.Spec.EnableNamespaceResources).To(HaveValue(BeTrue()))
		}, input.WaitForUpdate...).Should(Succeed())

		By("Restoring initial patches for control plane")
		Eventually(func(g Gomega) {
			g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(infraControlPlane), infraControlPlane)).To(Succeed())
			infraControlPlane.Spec.ASOManagedClusterPatches = initialPatches
			g.Expect(mgmtClient.Update(ctx, infraControlPlane)).To(Succeed())
		}, inputGetter().WaitForUpdate...).Should(Succeed())

		By("Disabling preview features on the control plane")
		Eventually(func(g Gomega) {
			err = mgmtClient.Get(ctx, client.ObjectKey{Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace, Name: input.Cluster.Spec.ControlPlaneRef.Name}, infraControlPlane)
			g.Expect(err).NotTo(HaveOccurred())
			infraControlPlane.Spec.EnablePreviewFeatures = ptr.To(false)
			g.Expect(mgmtClient.Update(ctx, infraControlPlane)).To(Succeed())
		}, input.WaitForUpdate...).Should(Succeed())

		Eventually(func(g Gomega) {
			resp, err := managedClustersClient.Get(ctx, infraControlPlane.Spec.ResourceGroupName, infraControlPlane.Name, nil)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(resp.Properties.ProvisioningState).To(Equal(ptr.To("Succeeded")))
		}, input.WaitForUpdate...).Should(Succeed())
	}()

	for _, mp := range input.MachinePools {
		wg.Add(1)
		go func(mp *expv1.MachinePool) {
			defer GinkgoRecover()
			defer wg.Done()

			ammp := &infrav1.AzureManagedMachinePool{}
			Expect(mgmtClient.Get(ctx, types.NamespacedName{
				Namespace: mp.Spec.Template.Spec.InfrastructureRef.Namespace,
				Name:      mp.Spec.Template.Spec.InfrastructureRef.Name,
			}, ammp)).To(Succeed())

			agentPool := &asocontainerservicev1.ManagedClustersAgentPool{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: ammp.Namespace,
					Name:      ammp.Name,
				},
			}

			var initialPatches []string
			Byf("Deleting all patches for machine pool %s", mp.Name)
			Eventually(func(g Gomega) {
				g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)).To(Succeed())
				initialPatches = ammp.Spec.ASOManagedClustersAgentPoolPatches
				ammp.Spec.ASOManagedClustersAgentPoolPatches = nil
				g.Expect(mgmtClient.Update(ctx, ammp)).To(Succeed())
			}, inputGetter().WaitForUpdate...).Should(Succeed())

			Byf("Creating patches for machine pool %s", mp.Name)
			Eventually(func(g Gomega) {
				g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)).To(Succeed())
				ammp.Spec.ASOManagedClustersAgentPoolPatches = []string{
					`{"metadata": {"annotations": {"capzpatchtest": "value"}}}`,
				}
				g.Expect(mgmtClient.Update(ctx, ammp)).To(Succeed())
			}, inputGetter().WaitForUpdate...).Should(Succeed())
			Eventually(checkAnnotations(agentPool, CheckInput{exist: map[string]string{"capzpatchtest": "value"}}), input.WaitForUpdate...).Should(Succeed())

			Byf("Updating patches for machine pool %s", mp.Name)
			Eventually(func(g Gomega) {
				g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)).To(Succeed())
				ammp.Spec.ASOManagedClustersAgentPoolPatches = append(ammp.Spec.ASOManagedClustersAgentPoolPatches, `{"metadata": {"annotations": {"capzpatchtest": null}}}`)
				g.Expect(mgmtClient.Update(ctx, ammp)).To(Succeed())
			}, inputGetter().WaitForUpdate...).Should(Succeed())
			Eventually(checkAnnotations(agentPool, CheckInput{doNotExist: []string{"capzpatchtest"}}), input.WaitForUpdate...).Should(Succeed())

			Byf("Restoring initial patches for machine pool %s", mp.Name)
			Eventually(func(g Gomega) {
				g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)).To(Succeed())
				ammp.Spec.ASOManagedClustersAgentPoolPatches = initialPatches
				g.Expect(mgmtClient.Update(ctx, ammp)).To(Succeed())
			}, inputGetter().WaitForUpdate...).Should(Succeed())
		}(mp)
	}

	wg.Wait()
}

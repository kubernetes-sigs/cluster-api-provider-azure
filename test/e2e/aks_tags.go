//go:build e2e
// +build e2e

/*
Copyright 2022 The Kubernetes Authors.

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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AKSAdditionalTagsSpecInput struct {
	Cluster       *clusterv1.Cluster
	MachinePools  []*expv1.MachinePool
	WaitForUpdate []interface{}
}

func AKSAdditionalTagsSpec(ctx context.Context, inputGetter func() AKSAdditionalTagsSpecInput) {
	input := inputGetter()

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	Expect(err).NotTo(HaveOccurred())

	managedclustersClient, err := armcontainerservice.NewManagedClustersClient(getSubscriptionID(Default), cred, nil)
	Expect(err).NotTo(HaveOccurred())

	agentpoolsClient, err := armcontainerservice.NewAgentPoolsClient(getSubscriptionID(Default), cred, nil)
	Expect(err).NotTo(HaveOccurred())

	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	infraControlPlane := &infrav1.AzureManagedControlPlane{}
	err = mgmtClient.Get(ctx, client.ObjectKey{
		Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace,
		Name:      input.Cluster.Spec.ControlPlaneRef.Name,
	}, infraControlPlane)
	Expect(err).NotTo(HaveOccurred())

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer GinkgoRecover()
		defer wg.Done()

		nonAdditionalTagKeys := map[string]struct{}{}
		resp, err := managedclustersClient.Get(ctx, infraControlPlane.Spec.ResourceGroupName, infraControlPlane.Name, nil)
		Expect(err).NotTo(HaveOccurred())
		for k := range resp.ManagedCluster.Tags {
			if _, exists := infraControlPlane.Spec.AdditionalTags[k]; !exists {
				nonAdditionalTagKeys[k] = struct{}{}
			}
		}

		var expectedTags infrav1.Tags
		checkTags := func(g Gomega) {
			resp, err := managedclustersClient.Get(ctx, infraControlPlane.Spec.ResourceGroupName, infraControlPlane.Name, nil)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(resp.Properties.ProvisioningState).To(Equal(ptr.To("Succeeded")))
			actualTags := converters.MapToTags(resp.ManagedCluster.Tags)
			// Ignore tags not originally specified in spec.additionalTags
			for k := range nonAdditionalTagKeys {
				delete(actualTags, k)
			}
			if len(actualTags) == 0 {
				actualTags = nil
			}
			if expectedTags == nil {
				g.Expect(actualTags).To(BeNil())
			} else {
				g.Expect(actualTags).To(Equal(expectedTags))
			}
		}

		var initialTags infrav1.Tags
		Eventually(func(g Gomega) {
			g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(infraControlPlane), infraControlPlane)).To(Succeed())
			initialTags = infraControlPlane.Spec.AdditionalTags
		}, inputGetter().WaitForUpdate...).Should(Succeed())

		By("Creating tags for control plane")
		// Preserve "creationTimestamp" so the RG cleanup doesn't fire on this cluster during this test.
		expectedTags = maps.Clone(initialTags)
		expectedTags["test"] = "tag"
		expectedTags["another"] = "value"
		Eventually(func(g Gomega) {
			g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(infraControlPlane), infraControlPlane)).To(Succeed())
			infraControlPlane.Spec.AdditionalTags = expectedTags
			g.Expect(mgmtClient.Update(ctx, infraControlPlane)).To(Succeed())
		}, inputGetter().WaitForUpdate...).Should(Succeed())
		Eventually(checkTags, input.WaitForUpdate...).Should(Succeed())

		By("Updating tags for control plane")
		expectedTags["test"] = "updated"
		delete(expectedTags, "another")
		expectedTags["new"] = "value"
		Eventually(func(g Gomega) {
			g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(infraControlPlane), infraControlPlane)).To(Succeed())
			infraControlPlane.Spec.AdditionalTags = expectedTags
			g.Expect(mgmtClient.Update(ctx, infraControlPlane)).To(Succeed())
		}, inputGetter().WaitForUpdate...).Should(Succeed())
		Eventually(checkTags, input.WaitForUpdate...).Should(Succeed())

		By("Restoring initial tags for control plane")
		expectedTags = initialTags
		Eventually(func(g Gomega) {
			g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(infraControlPlane), infraControlPlane)).To(Succeed())
			infraControlPlane.Spec.AdditionalTags = expectedTags
			g.Expect(mgmtClient.Update(ctx, infraControlPlane)).To(Succeed())
		}, inputGetter().WaitForUpdate...).Should(Succeed())
		Eventually(checkTags, input.WaitForUpdate...).Should(Succeed())
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

			nonAdditionalTagKeys := map[string]struct{}{}
			resp, err := agentpoolsClient.Get(ctx, infraControlPlane.Spec.ResourceGroupName, infraControlPlane.Name, *ammp.Spec.Name, nil)
			Expect(err).NotTo(HaveOccurred())
			for k := range resp.AgentPool.Properties.Tags {
				if _, exists := infraControlPlane.Spec.AdditionalTags[k]; !exists {
					nonAdditionalTagKeys[k] = struct{}{}
				}
			}

			var expectedTags infrav1.Tags
			checkTags := func(g Gomega) {
				resp, err := agentpoolsClient.Get(ctx, infraControlPlane.Spec.ResourceGroupName, infraControlPlane.Name, *ammp.Spec.Name, nil)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp.Properties.ProvisioningState).To(Equal(ptr.To("Succeeded")))
				actualTags := converters.MapToTags(resp.AgentPool.Properties.Tags)
				// Ignore tags not originally specified in spec.additionalTags
				for k := range nonAdditionalTagKeys {
					delete(actualTags, k)
				}
				if len(actualTags) == 0 {
					actualTags = nil
				}
				if expectedTags == nil {
					g.Expect(actualTags).To(BeNil())
				} else {
					g.Expect(actualTags).To(Equal(expectedTags))
				}
			}

			Byf("Deleting all tags for machine pool %s", mp.Name)
			expectedTags = nil
			var initialTags infrav1.Tags
			Eventually(func(g Gomega) {
				g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)).To(Succeed())
				initialTags = ammp.Spec.AdditionalTags
				ammp.Spec.AdditionalTags = expectedTags
				g.Expect(mgmtClient.Update(ctx, ammp)).To(Succeed())
			}, inputGetter().WaitForUpdate...).Should(Succeed())
			Eventually(checkTags, input.WaitForUpdate...).Should(Succeed())

			Byf("Creating tags for machine pool %s", mp.Name)
			expectedTags = infrav1.Tags{
				"test":    "tag",
				"another": "value",
			}
			Eventually(func(g Gomega) {
				g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)).To(Succeed())
				ammp.Spec.AdditionalTags = expectedTags
				g.Expect(mgmtClient.Update(ctx, ammp)).To(Succeed())
			}, inputGetter().WaitForUpdate...).Should(Succeed())
			Eventually(checkTags, input.WaitForUpdate...).Should(Succeed())

			Byf("Updating tags for machine pool %s", mp.Name)
			expectedTags["test"] = "updated"
			delete(expectedTags, "another")
			expectedTags["new"] = "value"
			Eventually(func(g Gomega) {
				g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)).To(Succeed())
				ammp.Spec.AdditionalTags = expectedTags
				g.Expect(mgmtClient.Update(ctx, ammp)).To(Succeed())
			}, inputGetter().WaitForUpdate...).Should(Succeed())
			Eventually(checkTags, input.WaitForUpdate...).Should(Succeed())

			Byf("Restoring initial tags for machine pool %s", mp.Name)
			expectedTags = initialTags
			Eventually(func(g Gomega) {
				g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)).To(Succeed())
				ammp.Spec.AdditionalTags = expectedTags
				g.Expect(mgmtClient.Update(ctx, ammp)).To(Succeed())
			}, inputGetter().WaitForUpdate...).Should(Succeed())
			Eventually(checkTags, input.WaitForUpdate...).Should(Succeed())
		}(mp)
	}

	wg.Wait()
}

//go:build e2e
// +build e2e

/*
Copyright 2023 The Kubernetes Authors.

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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AKSNodeLabelsSpecInput struct {
	Cluster       *clusterv1.Cluster
	MachinePools  []*expv1.MachinePool
	WaitForUpdate []interface{}
}

func AKSNodeLabelsSpec(ctx context.Context, inputGetter func() AKSNodeLabelsSpecInput) {
	input := inputGetter()

	cred, err := azidentity.NewDefaultAzureCredential(nil)
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

			var expectedLabels map[string]string
			checkLabels := func(g Gomega) {
				resp, err := agentpoolsClient.Get(ctx, infraControlPlane.Spec.ResourceGroupName, infraControlPlane.Name, *ammp.Spec.Name, nil)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp.Properties.ProvisioningState).To(Equal(ptr.To("Succeeded")))

				agentpool := resp.AgentPool
				var actualLabels map[string]string
				if agentpool.Properties.NodeLabels != nil {
					actualLabels = make(map[string]string)
					for k, v := range agentpool.Properties.NodeLabels {
						actualLabels[k] = ptr.Deref(v, "")
					}
				}
				if expectedLabels == nil {
					g.Expect(actualLabels).To(BeNil())
				} else {
					g.Expect(actualLabels).To(Equal(expectedLabels))
				}
			}

			Byf("Deleting all node labels for machine pool %s", mp.Name)
			expectedLabels = nil
			var initialLabels map[string]string
			Eventually(func(g Gomega) {
				g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)).To(Succeed())
				initialLabels = ammp.Spec.NodeLabels
				ammp.Spec.NodeLabels = expectedLabels
				g.Expect(mgmtClient.Update(ctx, ammp)).To(Succeed())
			}, inputGetter().WaitForUpdate...).Should(Succeed())
			Eventually(checkLabels, input.WaitForUpdate...).Should(Succeed())

			Byf("Creating node labels for machine pool %s", mp.Name)
			expectedLabels = map[string]string{
				"test":    "label",
				"another": "value",
			}
			Eventually(func(g Gomega) {
				g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)).To(Succeed())
				ammp.Spec.NodeLabels = expectedLabels
				g.Expect(mgmtClient.Update(ctx, ammp)).To(Succeed())
			}, input.WaitForUpdate...).Should(Succeed())
			Eventually(checkLabels, input.WaitForUpdate...).Should(Succeed())

			Byf("Updating node labels for machine pool %s", mp.Name)
			expectedLabels["test"] = "updated"
			delete(expectedLabels, "another")
			expectedLabels["new"] = "value"
			Eventually(func(g Gomega) {
				g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)).To(Succeed())
				ammp.Spec.NodeLabels = expectedLabels
				g.Expect(mgmtClient.Update(ctx, ammp)).To(Succeed())
			}, input.WaitForUpdate...).Should(Succeed())
			Eventually(checkLabels, input.WaitForUpdate...).Should(Succeed())

			Byf("Restoring initial node labels for machine pool %s", mp.Name)
			expectedLabels = initialLabels
			Eventually(func(g Gomega) {
				g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)).To(Succeed())
				ammp.Spec.NodeLabels = expectedLabels
				g.Expect(mgmtClient.Update(ctx, ammp)).To(Succeed())
			}, input.WaitForUpdate...).Should(Succeed())
			Eventually(checkLabels, input.WaitForUpdate...).Should(Succeed())
		}(mp)
	}

	wg.Wait()
}

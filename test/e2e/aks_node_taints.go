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
	"fmt"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AKSNodeTaintsSpecInput struct {
	Cluster       *clusterv1.Cluster
	MachinePools  []*expv1.MachinePool
	WaitForUpdate []interface{}
}

func AKSNodeTaintsSpec(ctx context.Context, inputGetter func() AKSNodeTaintsSpecInput) {
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
			initialTaints := ammp.Spec.Taints

			var expectedTaints infrav1.Taints
			checkTaints := func(g Gomega) {
				var expectedTaintStrs []*string
				if expectedTaints != nil {
					expectedTaintStrs = make([]*string, 0, len(expectedTaints))
					for _, taint := range expectedTaints {
						expectedTaintStrs = append(expectedTaintStrs, ptr.To(fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect)))
					}
				}

				resp, err := agentpoolsClient.Get(ctx, infraControlPlane.Spec.ResourceGroupName, infraControlPlane.Name, *ammp.Spec.Name, nil)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(resp.Properties.ProvisioningState).To(Equal(ptr.To("Succeeded")))
				actualTaintStrs := resp.AgentPool.Properties.NodeTaints
				if expectedTaintStrs == nil {
					g.Expect(actualTaintStrs).To(BeNil())
				} else {
					g.Expect(actualTaintStrs).To(Equal(expectedTaintStrs))
				}
			}

			Byf("Deleting all node taints for machine pool %s", mp.Name)
			expectedTaints = nil
			Eventually(func(g Gomega) {
				g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)).To(Succeed())
				ammp.Spec.Taints = expectedTaints
				g.Expect(mgmtClient.Update(ctx, ammp)).To(Succeed())
			}, inputGetter().WaitForUpdate...).Should(Succeed())
			Eventually(checkTaints, input.WaitForUpdate...).Should(Succeed())

			Byf("Creating taints for machine pool %s", mp.Name)
			expectedTaints = infrav1.Taints{
				{
					Effect: infrav1.TaintEffect(corev1.TaintEffectPreferNoSchedule),
					Key:    "capz-e2e-1",
					Value:  "test1",
				},
				{
					Effect: infrav1.TaintEffect(corev1.TaintEffectPreferNoSchedule),
					Key:    "capz-e2e-2",
					Value:  "test2",
				},
			}
			Eventually(func(g Gomega) {
				g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)).To(Succeed())
				ammp.Spec.Taints = expectedTaints
				g.Expect(mgmtClient.Update(ctx, ammp)).To(Succeed())
				Eventually(checkTaints, input.WaitForUpdate...).Should(Succeed())
			}, input.WaitForUpdate...).Should(Succeed())

			Byf("Updating taints for machine pool %s", mp.Name)
			expectedTaints = expectedTaints[:1]
			Eventually(func(g Gomega) {
				g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)).To(Succeed())
				ammp.Spec.Taints = expectedTaints
				g.Expect(mgmtClient.Update(ctx, ammp)).To(Succeed())
			}, input.WaitForUpdate...).Should(Succeed())
			Eventually(checkTaints, input.WaitForUpdate...).Should(Succeed())

			Byf("Restoring initial taints for machine pool %s", mp.Name)
			expectedTaints = initialTaints
			Eventually(func(g Gomega) {
				g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)).To(Succeed())
				ammp.Spec.Taints = expectedTaints
				g.Expect(mgmtClient.Update(ctx, ammp)).To(Succeed())
			}, input.WaitForUpdate...).Should(Succeed())
			Eventually(checkTaints, input.WaitForUpdate...).Should(Succeed())
		}(mp)
	}

	wg.Wait()
}

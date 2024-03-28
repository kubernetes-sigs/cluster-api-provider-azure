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

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/kubernetesconfiguration/armkubernetesconfiguration"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AKSMarketplaceExtensionSpecInput struct {
	Cluster       *clusterv1.Cluster
	WaitIntervals []interface{}
}

const (
	extensionName         = "aks-marketplace-extension"
	officialExtensionName = "official-aks-extension"
)

func AKSMarketplaceExtensionSpec(ctx context.Context, inputGetter func() AKSMarketplaceExtensionSpecInput) {
	input := inputGetter()

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	Expect(err).NotTo(HaveOccurred())

	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	amcp := &infrav1.AzureManagedControlPlane{}
	err = mgmtClient.Get(ctx, types.NamespacedName{
		Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace,
		Name:      input.Cluster.Spec.ControlPlaneRef.Name,
	}, amcp)
	Expect(err).NotTo(HaveOccurred())

	agentpoolsClient, err := armcontainerservice.NewAgentPoolsClient(amcp.Spec.SubscriptionID, cred, nil)
	Expect(err).NotTo(HaveOccurred())

	extensionClient, err := armkubernetesconfiguration.NewExtensionsClient(amcp.Spec.SubscriptionID, cred, nil)
	Expect(err).NotTo(HaveOccurred())

	By("Deleting all node taints for windows machine pool")
	var ammp = &infrav1.AzureManagedMachinePool{}
	Expect(mgmtClient.Get(ctx, types.NamespacedName{
		Namespace: input.Cluster.Namespace,
		Name:      input.Cluster.Name + "-pool2",
	}, ammp)).To(Succeed())
	initialTaints := ammp.Spec.Taints
	var expectedTaints []infrav1.Taint
	expectedTaints = nil
	checkTaints := func(g Gomega) {
		var expectedTaintStrs []*string
		if expectedTaints != nil {
			expectedTaintStrs = make([]*string, 0, len(expectedTaints))
			for _, taint := range expectedTaints {
				expectedTaintStrs = append(expectedTaintStrs, ptr.To(fmt.Sprintf("%s=%s:%s", taint.Key, taint.Value, taint.Effect)))
			}
		}

		resp, err := agentpoolsClient.Get(ctx, amcp.Spec.ResourceGroupName, amcp.Name, *ammp.Spec.Name, nil)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp.Properties.ProvisioningState).To(Equal(ptr.To("Succeeded")))
		actualTaintStrs := resp.AgentPool.Properties.NodeTaints
		if expectedTaintStrs == nil {
			g.Expect(actualTaintStrs).To(BeNil())
		} else {
			g.Expect(actualTaintStrs).To(Equal(expectedTaintStrs))
		}
	}
	Eventually(func(g Gomega) {
		g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)).To(Succeed())
		ammp.Spec.Taints = expectedTaints
		g.Expect(mgmtClient.Update(ctx, ammp)).To(Succeed())
	}, inputGetter().WaitIntervals...).Should(Succeed())
	Eventually(checkTaints, input.WaitIntervals...).Should(Succeed())

	By("Adding a taint to the Windows node pool")
	expectedTaints = []infrav1.Taint{
		{
			Effect: infrav1.TaintEffect(corev1.TaintEffectNoSchedule),
			Key:    "capz-e2e-1",
			Value:  "test1",
		},
	}
	Eventually(func(g Gomega) {
		g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)).To(Succeed())
		ammp.Spec.Taints = expectedTaints
		g.Expect(mgmtClient.Update(ctx, ammp)).To(Succeed())
		Eventually(checkTaints, input.WaitIntervals...).Should(Succeed())
	}, input.WaitIntervals...).Should(Succeed())

	By("Updating taints for Windows machine pool")
	expectedTaints = expectedTaints[:1]
	Eventually(func(g Gomega) {
		g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)).To(Succeed())
		ammp.Spec.Taints = expectedTaints
		g.Expect(mgmtClient.Update(ctx, ammp)).To(Succeed())
	}, input.WaitIntervals...).Should(Succeed())
	Eventually(checkTaints, input.WaitIntervals...).Should(Succeed())

	By("Adding an official AKS Extension & AKS Marketplace Extension to the AzureManagedControlPlane")
	var infraControlPlane = &infrav1.AzureManagedControlPlane{}
	Eventually(func(g Gomega) {
		err = mgmtClient.Get(ctx, client.ObjectKey{
			Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace,
			Name:      input.Cluster.Spec.ControlPlaneRef.Name,
		}, infraControlPlane)
		g.Expect(err).NotTo(HaveOccurred())
		infraControlPlane.Spec.Extensions = []infrav1.AKSExtension{
			{
				Name:          extensionName,
				ExtensionType: ptr.To("TraefikLabs.TraefikProxy"),
				Plan: &infrav1.ExtensionPlan{
					Name:      "traefik-proxy",
					Product:   "traefik-proxy",
					Publisher: "containous",
				},
			},
			{
				Name:          officialExtensionName,
				ExtensionType: ptr.To("microsoft.flux"),
			},
		}
		g.Expect(mgmtClient.Update(ctx, infraControlPlane)).To(Succeed())
	}, input.WaitIntervals...).Should(Succeed())

	By("Ensuring the AKS Marketplace Extension status is ready on the AzureManagedControlPlane")
	Eventually(func(g Gomega) {
		err = mgmtClient.Get(ctx, client.ObjectKey{Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace, Name: input.Cluster.Spec.ControlPlaneRef.Name}, infraControlPlane)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(conditions.IsTrue(infraControlPlane, infrav1.AKSExtensionsReadyCondition)).To(BeTrue())
	}, input.WaitIntervals...).Should(Succeed())

	By("Ensuring the AKS Marketplace Extension is added to the AzureManagedControlPlane")
	ensureAKSExtensionAdded(ctx, input, extensionName, "TraefikLabs.TraefikProxy", extensionClient, amcp)
	ensureAKSExtensionAdded(ctx, input, officialExtensionName, "microsoft.flux", extensionClient, amcp)

	By("Restoring initial taints for Windows machine pool")
	expectedTaints = initialTaints
	Eventually(func(g Gomega) {
		g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)).To(Succeed())
		ammp.Spec.Taints = expectedTaints
		g.Expect(mgmtClient.Update(ctx, ammp)).To(Succeed())
	}, input.WaitIntervals...).Should(Succeed())
	Eventually(checkTaints, input.WaitIntervals...).Should(Succeed())
}

func ensureAKSExtensionAdded(ctx context.Context, input AKSMarketplaceExtensionSpecInput, extensionName, extensionType string, extensionClient *armkubernetesconfiguration.ExtensionsClient, amcp *infrav1.AzureManagedControlPlane) {
	Eventually(func(g Gomega) {
		resp, err := extensionClient.Get(ctx, amcp.Spec.ResourceGroupName, "Microsoft.ContainerService", "managedClusters", input.Cluster.Name, extensionName, nil)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp.Properties.ProvisioningState).To(Equal(ptr.To(armkubernetesconfiguration.ProvisioningStateSucceeded)))
		extension := resp.Extension
		g.Expect(extension.Properties).NotTo(BeNil())
		g.Expect(extension.Name).To(Equal(ptr.To(extensionName)))
		g.Expect(extension.Properties.AutoUpgradeMinorVersion).To(Equal(ptr.To(true)))
		g.Expect(extension.Properties.ExtensionType).To(Equal(ptr.To(extensionType)))
	}, input.WaitIntervals...).Should(Succeed())
}

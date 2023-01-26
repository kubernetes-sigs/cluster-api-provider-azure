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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AKSPublicIPPrefixSpecInput struct {
	Cluster           *clusterv1.Cluster
	KubernetesVersion string
	WaitIntervals     []interface{}
}

func AKSPublicIPPrefixSpec(ctx context.Context, inputGetter func() AKSPublicIPPrefixSpecInput) {
	input := inputGetter()

	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	auth, err := azureutil.GetAuthorizer(settings)
	Expect(err).NotTo(HaveOccurred())

	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	infraControlPlane := &infrav1.AzureManagedControlPlane{}
	err = mgmtClient.Get(ctx, client.ObjectKey{Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace, Name: input.Cluster.Spec.ControlPlaneRef.Name}, infraControlPlane)
	Expect(err).NotTo(HaveOccurred())

	resourceGroupName := infraControlPlane.Spec.ResourceGroupName

	publicIPPrefixClient := network.NewPublicIPPrefixesClient(subscriptionID)
	publicIPPrefixClient.Authorizer = auth

	By("Creating public IP prefix with 2 addresses")
	publicIPPrefixFuture, err := publicIPPrefixClient.CreateOrUpdate(ctx, resourceGroupName, input.Cluster.Name, network.PublicIPPrefix{
		Location: to.StringPtr(infraControlPlane.Spec.Location),
		Sku: &network.PublicIPPrefixSku{
			Name: network.PublicIPPrefixSkuNameStandard,
		},
		PublicIPPrefixPropertiesFormat: &network.PublicIPPrefixPropertiesFormat{
			PrefixLength: to.Int32Ptr(31), // In bits. This provides 2 addresses.
		},
	})
	Expect(err).NotTo(HaveOccurred())
	var publicIPPrefix network.PublicIPPrefix
	Eventually(func(g Gomega) {
		publicIPPrefix, err = publicIPPrefixFuture.Result(publicIPPrefixClient)
		g.Expect(err).NotTo(HaveOccurred())
	}, input.WaitIntervals...).Should(Succeed(), "failed to create public IP prefix")

	By("Creating node pool with 3 nodes")
	infraMachinePool := &infrav1.AzureManagedMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pool3",
			Namespace: input.Cluster.Namespace,
		},
		Spec: infrav1.AzureManagedMachinePoolSpec{
			Mode:                 "User",
			SKU:                  "Standard_D2s_v3",
			EnableNodePublicIP:   to.BoolPtr(true),
			NodePublicIPPrefixID: to.StringPtr("/subscriptions/" + subscriptionID + "/resourceGroups/" + resourceGroupName + "/providers/Microsoft.Network/publicipprefixes/" + *publicIPPrefix.Name),
		},
	}
	err = mgmtClient.Create(ctx, infraMachinePool)
	Expect(err).NotTo(HaveOccurred())

	machinePool := &expv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: infraMachinePool.Namespace,
			Name:      infraMachinePool.Name,
		},
		Spec: expv1.MachinePoolSpec{
			ClusterName: input.Cluster.Name,
			Replicas:    to.Int32Ptr(3),
			Template: clusterv1.MachineTemplateSpec{
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: to.StringPtr(""),
					},
					ClusterName: input.Cluster.Name,
					InfrastructureRef: corev1.ObjectReference{
						APIVersion: infrav1.GroupVersion.String(),
						Kind:       "AzureManagedMachinePool",
						Name:       infraMachinePool.Name,
					},
					Version: to.StringPtr(input.KubernetesVersion),
				},
			},
		},
	}
	err = mgmtClient.Create(ctx, machinePool)
	Expect(err).NotTo(HaveOccurred())

	defer func() {
		By("Deleting the node pool")
		err := mgmtClient.Delete(ctx, machinePool)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func(g Gomega) {
			err := mgmtClient.Get(ctx, client.ObjectKeyFromObject(machinePool), &expv1.MachinePool{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, input.WaitIntervals...).Should(Succeed(), "Deleted MachinePool %s/%s still exists", machinePool.Namespace, machinePool.Name)

		Eventually(func(g Gomega) {
			err := mgmtClient.Get(ctx, client.ObjectKeyFromObject(infraMachinePool), &infrav1.AzureManagedMachinePool{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, input.WaitIntervals...).Should(Succeed(), "Deleted AzureManagedMachinePool %s/%s still exists", infraMachinePool.Namespace, infraMachinePool.Name)
	}()

	By("Verifying the AzureManagedMachinePool converges to a failed ready status")
	Eventually(func(g Gomega) {
		infraMachinePool := &infrav1.AzureManagedMachinePool{}
		err := mgmtClient.Get(ctx, client.ObjectKeyFromObject(machinePool), infraMachinePool)
		g.Expect(err).NotTo(HaveOccurred())
		cond := conditions.Get(infraMachinePool, infrav1.AgentPoolsReadyCondition)
		g.Expect(cond).NotTo(BeNil())
		g.Expect(cond.Status).To(Equal(corev1.ConditionFalse))
		g.Expect(cond.Reason).To(Equal(infrav1.FailedReason))
		g.Expect(cond.Message).To(HavePrefix("failed to find vm scale set"))
	}, input.WaitIntervals...).Should(Succeed())

	By("Scaling the MachinePool to 2 nodes")
	Eventually(func(g Gomega) {
		err = mgmtClient.Get(ctx, client.ObjectKeyFromObject(machinePool), machinePool)
		g.Expect(err).NotTo(HaveOccurred())
		machinePool.Spec.Replicas = to.Int32Ptr(2)
		err = mgmtClient.Update(ctx, machinePool)
		g.Expect(err).NotTo(HaveOccurred())
	}, input.WaitIntervals...).Should(Succeed())

	By("Verifying the AzureManagedMachinePool becomes ready")
	Eventually(func(g Gomega) {
		infraMachinePool := &infrav1.AzureManagedMachinePool{}
		err := mgmtClient.Get(ctx, client.ObjectKeyFromObject(machinePool), infraMachinePool)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(conditions.IsTrue(infraMachinePool, infrav1.AgentPoolsReadyCondition)).To(BeTrue())
	}, input.WaitIntervals...).Should(Succeed())
}

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
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
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

	subscriptionID := getSubscriptionID(Default)

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	Expect(err).NotTo(HaveOccurred())

	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	infraControlPlane := &infrav1.AzureManagedControlPlane{}
	err = mgmtClient.Get(ctx, client.ObjectKey{Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace, Name: input.Cluster.Spec.ControlPlaneRef.Name}, infraControlPlane)
	Expect(err).NotTo(HaveOccurred())

	resourceGroupName := infraControlPlane.Spec.ResourceGroupName

	publicIPPrefixClient, err := armnetwork.NewPublicIPPrefixesClient(subscriptionID, cred, nil)
	Expect(err).NotTo(HaveOccurred())

	By("Creating public IP prefix with 2 addresses")
	poller, err := publicIPPrefixClient.BeginCreateOrUpdate(ctx, resourceGroupName, input.Cluster.Name, armnetwork.PublicIPPrefix{
		Location: ptr.To(infraControlPlane.Spec.Location),
		SKU: &armnetwork.PublicIPPrefixSKU{
			Name: ptr.To(armnetwork.PublicIPPrefixSKUNameStandard),
		},
		Properties: &armnetwork.PublicIPPrefixPropertiesFormat{
			PrefixLength: ptr.To[int32](31), // In bits. This provides 2 addresses.
		},
	}, nil)
	Expect(err).NotTo(HaveOccurred())
	var publicIPPrefix armnetwork.PublicIPPrefix
	Eventually(func(g Gomega) {
		resp, err := poller.PollUntilDone(ctx, nil)
		Expect(err).NotTo(HaveOccurred())
		publicIPPrefix = resp.PublicIPPrefix
	}, input.WaitIntervals...).Should(Succeed(), "failed to create public IP prefix")

	By("Creating node pool with 2 nodes")
	infraMachinePool := &infrav1.AzureManagedMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pool3",
			Namespace: input.Cluster.Namespace,
		},
		Spec: infrav1.AzureManagedMachinePoolSpec{
			Mode:                 "User",
			SKU:                  os.Getenv("AZURE_NODE_MACHINE_TYPE"),
			EnableNodePublicIP:   ptr.To(true),
			NodePublicIPPrefixID: ptr.To("/subscriptions/" + subscriptionID + "/resourceGroups/" + resourceGroupName + "/providers/Microsoft.Network/publicipprefixes/" + *publicIPPrefix.Name),
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
			Replicas:    ptr.To[int32](2),
			Template: clusterv1.MachineTemplateSpec{
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: ptr.To(""),
					},
					ClusterName: input.Cluster.Name,
					InfrastructureRef: corev1.ObjectReference{
						APIVersion: infrav1.GroupVersion.String(),
						Kind:       "AzureManagedMachinePool",
						Name:       infraMachinePool.Name,
					},
					Version: ptr.To(input.KubernetesVersion),
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

	By("Verifying the AzureManagedMachinePool becomes ready")
	Eventually(func(g Gomega) {
		infraMachinePool := &infrav1.AzureManagedMachinePool{}
		err := mgmtClient.Get(ctx, client.ObjectKeyFromObject(machinePool), infraMachinePool)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(conditions.IsTrue(infraMachinePool, infrav1.AgentPoolsReadyCondition)).To(BeTrue())
	}, input.WaitIntervals...).Should(Succeed())
}

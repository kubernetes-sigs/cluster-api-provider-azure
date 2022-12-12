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
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-05-01/containerservice"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
)

type AKSWebhooksSpecInput struct {
	Cluster      *clusterv1.Cluster
	MachinePools []*expv1.MachinePool
}

func AKSWebhooksSpec(ctx context.Context, inputGetter func() AKSWebhooksSpecInput) {
	input := inputGetter()

	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	auth, err := settings.GetAuthorizer()
	Expect(err).NotTo(HaveOccurred())
	agentpoolClient := containerservice.NewAgentPoolsClient(subscriptionID)
	agentpoolClient.Authorizer = auth
	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	By("Validating that AzureManagedCluster webhooks are working")
	amc, err := getAzureManagedCluster(ctx, mgmtClient, input.Cluster.Namespace, input.Cluster.Name)
	Expect(err).NotTo(HaveOccurred())

	disallowedPrefix := azure.CustomHeaderPrefix + "foo"
	By(fmt.Sprintf("Validating that we can't manually add an annotation to AzureManagedCluster with a prefix of '%s'", disallowedPrefix))
	amc.ObjectMeta.Annotations[disallowedPrefix] = "bar"
	err = mgmtClient.Update(ctx, amc)
	Expect(err).To(HaveOccurred())
	Expect(apierrors.IsInvalid(err)).To(BeTrue())

	By("Validating that AzureManagedControlPlane webhooks are working")

	By("Getting the existing AzureManagedControlPlane resource")
	amcp, err := getAzureManagedControlPlane(ctx, mgmtClient, input.Cluster.Namespace, input.Cluster.Name)
	Expect(err).NotTo(HaveOccurred())

	By("Validating that we can't update AzureManagedControlPlane.Spec.SubscriptionID")
	amcp, err = getAzureManagedControlPlane(ctx, mgmtClient, input.Cluster.Namespace, input.Cluster.Name)
	Expect(err).NotTo(HaveOccurred())
	amcp.Spec.SubscriptionID += "123"
	err = mgmtClient.Update(ctx, amcp)
	Expect(err).To(HaveOccurred())
	Expect(apierrors.IsInvalid(err)).To(BeTrue())

	By("Validating that we can't update AzureManagedControlPlane.Spec.ResourceGroupName")
	amcp, err = getAzureManagedControlPlane(ctx, mgmtClient, input.Cluster.Namespace, input.Cluster.Name)
	Expect(err).NotTo(HaveOccurred())
	amcp.Spec.ResourceGroupName = amcp.Spec.ResourceGroupName + "foo"
	err = mgmtClient.Update(ctx, amcp)
	Expect(err).To(HaveOccurred())
	Expect(apierrors.IsInvalid(err)).To(BeTrue())

	By("Validating that we can't update AzureManagedControlPlane.Spec.NodeResourceGroupName")
	amcp, err = getAzureManagedControlPlane(ctx, mgmtClient, input.Cluster.Namespace, input.Cluster.Name)
	Expect(err).NotTo(HaveOccurred())
	amcp.Spec.NodeResourceGroupName = amcp.Spec.NodeResourceGroupName + "foo"
	err = mgmtClient.Update(ctx, amcp)
	Expect(err).To(HaveOccurred())
	Expect(apierrors.IsInvalid(err)).To(BeTrue())

	By("Validating that we can't update AzureManagedControlPlane.Spec.Location")
	amcp, err = getAzureManagedControlPlane(ctx, mgmtClient, input.Cluster.Namespace, input.Cluster.Name)
	Expect(err).NotTo(HaveOccurred())
	amcp.Spec.Location = "notavalidregion"
	err = mgmtClient.Update(ctx, amcp)
	Expect(err).To(HaveOccurred())
	Expect(apierrors.IsInvalid(err)).To(BeTrue())

	By("Validating that we can't update AzureManagedControlPlane.Spec.SSHPublicKey")
	amcp, err = getAzureManagedControlPlane(ctx, mgmtClient, input.Cluster.Namespace, input.Cluster.Name)
	Expect(err).NotTo(HaveOccurred())
	amcp.Spec.SSHPublicKey = amcp.Spec.SSHPublicKey + "foo"
	err = mgmtClient.Update(ctx, amcp)
	Expect(err).To(HaveOccurred())
	Expect(apierrors.IsInvalid(err)).To(BeTrue())

	By("Validating that we can't update AzureManagedControlPlane.Spec.DNSServiceIP")
	amcp, err = getAzureManagedControlPlane(ctx, mgmtClient, input.Cluster.Namespace, input.Cluster.Name)
	Expect(err).NotTo(HaveOccurred())
	amcp.Spec.DNSServiceIP = to.StringPtr("256.256.256.256")
	err = mgmtClient.Update(ctx, amcp)
	Expect(err).To(HaveOccurred())
	Expect(apierrors.IsInvalid(err)).To(BeTrue())

	By("Validating that we can't update AzureManagedControlPlane.Spec.NetworkPlugin")
	amcp, err = getAzureManagedControlPlane(ctx, mgmtClient, input.Cluster.Namespace, input.Cluster.Name)
	Expect(err).NotTo(HaveOccurred())
	if to.String(amcp.Spec.NetworkPlugin) == "azure" {
		amcp.Spec.NetworkPlugin = to.StringPtr("kubenet")
	} else {
		amcp.Spec.NetworkPlugin = to.StringPtr("azure")
	}
	err = mgmtClient.Update(ctx, amcp)
	Expect(err).To(HaveOccurred())
	Expect(apierrors.IsInvalid(err)).To(BeTrue())

	By("Validating that we can't update AzureManagedControlPlane.Spec.NetworkPolicy")
	amcp, err = getAzureManagedControlPlane(ctx, mgmtClient, input.Cluster.Namespace, input.Cluster.Name)
	Expect(err).NotTo(HaveOccurred())
	if to.String(amcp.Spec.NetworkPolicy) == "azure" {
		amcp.Spec.NetworkPolicy = to.StringPtr("calico")
	} else {
		amcp.Spec.NetworkPolicy = to.StringPtr("azure")
	}
	err = mgmtClient.Update(ctx, amcp)
	Expect(err).To(HaveOccurred())
	Expect(apierrors.IsInvalid(err)).To(BeTrue())

	By("Validating that we can't update AzureManagedControlPlane.Spec.LoadBalancerSKU")
	amcp, err = getAzureManagedControlPlane(ctx, mgmtClient, input.Cluster.Namespace, input.Cluster.Name)
	Expect(err).NotTo(HaveOccurred())
	if to.String(amcp.Spec.LoadBalancerSKU) == "Standard" {
		amcp.Spec.LoadBalancerSKU = to.StringPtr("Basic")
	} else {
		amcp.Spec.LoadBalancerSKU = to.StringPtr("Standard")
	}
	err = mgmtClient.Update(ctx, amcp)
	Expect(err).To(HaveOccurred())
	Expect(apierrors.IsInvalid(err)).To(BeTrue())

	By("Validating that we can't update AzureManagedControlPlane.Spec.AADProfile")
	amcp, err = getAzureManagedControlPlane(ctx, mgmtClient, input.Cluster.Namespace, input.Cluster.Name)
	Expect(err).NotTo(HaveOccurred())
	if amcp.Spec.AADProfile != nil {
		aadProfile := &infrav1exp.AADProfile{}
		if amcp.Spec.AADProfile.Managed {
			aadProfile.Managed = false
		} else {
			aadProfile.Managed = true
		}
		amcp.Spec.AADProfile = aadProfile
		err = mgmtClient.Update(ctx, amcp)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
	}

	By("Validating that we can't update AzureManagedControlPlane.Spec.VirtualNetwork.Name")
	amcp, err = getAzureManagedControlPlane(ctx, mgmtClient, input.Cluster.Namespace, input.Cluster.Name)
	Expect(err).NotTo(HaveOccurred())
	amcp.Spec.VirtualNetwork.Name = amcp.Spec.VirtualNetwork.Name + "foo"
	err = mgmtClient.Update(ctx, amcp)
	Expect(err).To(HaveOccurred())
	Expect(apierrors.IsInvalid(err)).To(BeTrue())

	By("Validating that we can't update AzureManagedControlPlane.Spec.VirtualNetwork.CIDRBlock")
	amcp, err = getAzureManagedControlPlane(ctx, mgmtClient, input.Cluster.Namespace, input.Cluster.Name)
	Expect(err).NotTo(HaveOccurred())
	amcp.Spec.VirtualNetwork.CIDRBlock = amcp.Spec.VirtualNetwork.CIDRBlock + "123"
	err = mgmtClient.Update(ctx, amcp)
	Expect(err).To(HaveOccurred())
	Expect(apierrors.IsInvalid(err)).To(BeTrue())

	By("Validating that we can't update AzureManagedControlPlane.Spec.VirtualNetwork.Subnet.Name")
	amcp, err = getAzureManagedControlPlane(ctx, mgmtClient, input.Cluster.Namespace, input.Cluster.Name)
	Expect(err).NotTo(HaveOccurred())
	amcp.Spec.VirtualNetwork.Subnet.Name = amcp.Spec.VirtualNetwork.Subnet.Name + "foo"
	err = mgmtClient.Update(ctx, amcp)
	Expect(err).To(HaveOccurred())
	Expect(apierrors.IsInvalid(err)).To(BeTrue())

	By("Validating that we can't update AzureManagedControlPlane.Spec.VirtualNetwork.ResourceGroup")
	amcp, err = getAzureManagedControlPlane(ctx, mgmtClient, input.Cluster.Namespace, input.Cluster.Name)
	Expect(err).NotTo(HaveOccurred())
	amcp.Spec.VirtualNetwork.ResourceGroup = amcp.Spec.VirtualNetwork.ResourceGroup + "foo"
	err = mgmtClient.Update(ctx, amcp)
	Expect(err).To(HaveOccurred())
	Expect(apierrors.IsInvalid(err)).To(BeTrue())

	By("Validating that we can't update AzureManagedControlPlane.Spec.APIServerAccessProfile")
	amcp, err = getAzureManagedControlPlane(ctx, mgmtClient, input.Cluster.Namespace, input.Cluster.Name)
	Expect(err).NotTo(HaveOccurred())
	if amcp.Spec.APIServerAccessProfile != nil {
		apiServerAccessProfile := &infrav1exp.APIServerAccessProfile{}
		if to.Bool(amcp.Spec.APIServerAccessProfile.EnablePrivateCluster) {
			apiServerAccessProfile.EnablePrivateCluster = to.BoolPtr(false)
		} else {
			apiServerAccessProfile.EnablePrivateCluster = to.BoolPtr(true)
		}
		amcp.Spec.APIServerAccessProfile = apiServerAccessProfile
		err = mgmtClient.Update(ctx, amcp)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())
	}

	By("Validating that AzureManagedMachinePool webhooks are working")
	var nodePoolModeToUserError error
	for _, mp := range input.MachinePools {
		ammp, err := getAzureManagedMachinePool(ctx, mgmtClient, mp)
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("Validating webhooks against AzureManagedMachinePool %s", ammp.Name))

		By("Validating that we can't add a node label using the reserved 'kubernetes.azure.com/' prefix")
		if ammp.Spec.NodeLabels == nil {
			ammp.Spec.NodeLabels = map[string]string{}
		}
		ammp.Spec.NodeLabels[azureutil.AzureSystemNodeLabelPrefix+"/foo"] = "bar"
		err = mgmtClient.Update(ctx, ammp)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())

		By("Validating that we can't update AzureManagedMachinePool.Spec.OSType")
		ammp, err = getAzureManagedMachinePool(ctx, mgmtClient, mp)
		Expect(err).NotTo(HaveOccurred())
		if to.String(ammp.Spec.OSType) == "Linux" {
			ammp.Spec.OSType = to.StringPtr("Windows")
		} else {
			ammp.Spec.OSType = to.StringPtr("Linux")
		}
		err = mgmtClient.Update(ctx, ammp)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())

		By("Validating that we can't update AzureManagedMachinePool.Spec.SKU")
		ammp, err = getAzureManagedMachinePool(ctx, mgmtClient, mp)
		Expect(err).NotTo(HaveOccurred())
		ammp.Spec.SKU = ammp.Spec.SKU + "_vInvalid"
		err = mgmtClient.Update(ctx, ammp)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())

		By("Validating that we can't update AzureManagedMachinePool.Spec.OSDiskSizeGB")
		ammp, err = getAzureManagedMachinePool(ctx, mgmtClient, mp)
		Expect(err).NotTo(HaveOccurred())
		ammp.Spec.OSDiskSizeGB = to.Int32Ptr(to.Int32(ammp.Spec.OSDiskSizeGB) + 1)
		err = mgmtClient.Update(ctx, ammp)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())

		By("Validating that we can't add a new 'infrastructure.cluster.x-k8s.io/custom-header-' Annotation")
		ammp, err = getAzureManagedMachinePool(ctx, mgmtClient, mp)
		Expect(err).NotTo(HaveOccurred())
		if ammp.ObjectMeta.Annotations == nil {
			ammp.ObjectMeta.Annotations = map[string]string{}
		}
		ammp.ObjectMeta.Annotations[azure.CustomHeaderPrefix+"foo"] = "bar"
		err = mgmtClient.Update(ctx, ammp)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())

		By("Validating that we can't add or remove from AzureManagedMachinePool.Spec.AvailabilityZones")
		ammp, err = getAzureManagedMachinePool(ctx, mgmtClient, mp)
		Expect(err).NotTo(HaveOccurred())
		ammp.Spec.AvailabilityZones = append(ammp.Spec.AvailabilityZones, "99")
		err = mgmtClient.Update(ctx, ammp)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())

		ammp, err = getAzureManagedMachinePool(ctx, mgmtClient, mp)
		Expect(err).NotTo(HaveOccurred())
		if len(ammp.Spec.AvailabilityZones) > 0 {
			ammp.Spec.AvailabilityZones = ammp.Spec.AvailabilityZones[1:]
			err = mgmtClient.Update(ctx, ammp)
			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInvalid(err)).To(BeTrue())
		}

		By("Validating that we can't update AzureManagedMachinePool.Spec.MaxPods")
		ammp, err = getAzureManagedMachinePool(ctx, mgmtClient, mp)
		Expect(err).NotTo(HaveOccurred())
		ammp.Spec.MaxPods = to.Int32Ptr(to.Int32(ammp.Spec.MaxPods) + 1)
		err = mgmtClient.Update(ctx, ammp)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())

		By("Validating that we can't update AzureManagedMachinePool.Spec.OsDiskType")
		ammp, err = getAzureManagedMachinePool(ctx, mgmtClient, mp)
		Expect(err).NotTo(HaveOccurred())
		if to.String(ammp.Spec.OsDiskType) == "Ephemeral" {
			ammp.Spec.OsDiskType = to.StringPtr("Managed")
		} else {
			ammp.Spec.OsDiskType = to.StringPtr("Ephemeral")
		}
		err = mgmtClient.Update(ctx, ammp)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())

		By("Validating that we can't update AzureManagedMachinePool.Spec.ScaleSetPriority")
		ammp, err = getAzureManagedMachinePool(ctx, mgmtClient, mp)
		Expect(err).NotTo(HaveOccurred())
		if to.String(ammp.Spec.ScaleSetPriority) == "Regular" {
			ammp.Spec.ScaleSetPriority = to.StringPtr("Spot")
		} else {
			ammp.Spec.ScaleSetPriority = to.StringPtr("Regular")
		}
		err = mgmtClient.Update(ctx, ammp)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())

		By("Validating that we can't update AzureManagedMachinePool.Spec.EnableUltraSSD")
		ammp, err = getAzureManagedMachinePool(ctx, mgmtClient, mp)
		Expect(err).NotTo(HaveOccurred())
		ammp.Spec.EnableUltraSSD = to.BoolPtr(!to.Bool(ammp.Spec.EnableUltraSSD))
		err = mgmtClient.Update(ctx, ammp)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())

		By("Validating that we can't update AzureManagedMachinePool.Spec.EnableNodePublicIP")
		ammp, err = getAzureManagedMachinePool(ctx, mgmtClient, mp)
		Expect(err).NotTo(HaveOccurred())
		if to.Bool(ammp.Spec.EnableNodePublicIP) {
			ammp.Spec.EnableNodePublicIP = to.BoolPtr(false)
		} else {
			ammp.Spec.EnableNodePublicIP = to.BoolPtr(true)
		}
		err = mgmtClient.Update(ctx, ammp)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())

		By("Validating that we can't update AzureManagedMachinePool.Spec.NodePublicIPPrefixID")
		ammp, err = getAzureManagedMachinePool(ctx, mgmtClient, mp)
		Expect(err).NotTo(HaveOccurred())
		if to.String(ammp.Spec.NodePublicIPPrefixID) != "" {
			ammp.Spec.NodePublicIPPrefixID = to.StringPtr(to.String(ammp.Spec.NodePublicIPPrefixID) + "foo")
		} else {
			ammp.Spec.NodePublicIPPrefixID = to.StringPtr("foo")
		}
		err = mgmtClient.Update(ctx, ammp)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())

		By("Validating that we can't update AzureManagedMachinePool.Spec.KubeletConfig")
		ammp, err = getAzureManagedMachinePool(ctx, mgmtClient, mp)
		Expect(err).NotTo(HaveOccurred())
		if ammp.Spec.KubeletConfig != nil {
			ammp.Spec.KubeletConfig.FailSwapOn = to.BoolPtr(!to.Bool(ammp.Spec.KubeletConfig.FailSwapOn))
		} else {
			ammp.Spec.KubeletConfig = &infrav1exp.KubeletConfig{
				FailSwapOn: to.BoolPtr(true),
			}
		}
		err = mgmtClient.Update(ctx, ammp)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsInvalid(err)).To(BeTrue())

		By(fmt.Sprintf("Changing AzureManagedMachinePool %s mode to User", ammp.Name))
		ammp, err = getAzureManagedMachinePool(ctx, mgmtClient, mp)
		Expect(err).NotTo(HaveOccurred())
		ammp.Spec.Mode = "User"
		err = mgmtClient.Update(ctx, ammp)
		if err != nil {
			nodePoolModeToUserError = err
		}
	}
	By("Validating that we got a AzureManagedMachinePool 'must have at least one system node pool' error")
	Expect(nodePoolModeToUserError).To(HaveOccurred())
	Expect(apierrors.IsInvalid(nodePoolModeToUserError)).To(BeTrue())
}

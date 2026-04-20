//go:build e2e
// +build e2e

/*
Copyright 2025 The Kubernetes Authors.

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
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
)

const (
	AzureMachinePoolStoppedVMSpecName = "azure-machinepool-stopped-vm"
)

// AzureMachinePoolStoppedVMSpecInput is the input for AzureMachinePoolStoppedVMSpec.
type AzureMachinePoolStoppedVMSpecInput struct {
	Cluster               *clusterv1.Cluster
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	WaitIntervals         []interface{}
}

// AzureMachinePoolStoppedVMSpec verifies that a VMSS instance that is stopped
// (powered off) is detected by CAPZ and cleaned up.
func AzureMachinePoolStoppedVMSpec(ctx context.Context, inputGetter func() AzureMachinePoolStoppedVMSpecInput) {
	input := inputGetter()
	Expect(input.Cluster).NotTo(BeNil(), "Invalid argument. input.Cluster can't be nil when calling %s spec", AzureMachinePoolStoppedVMSpecName)
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", AzureMachinePoolStoppedVMSpecName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", AzureMachinePoolStoppedVMSpecName)
	Expect(input.ClusterName).NotTo(BeEmpty(), "Invalid argument. input.ClusterName can't be empty when calling %s spec", AzureMachinePoolStoppedVMSpecName)
	Expect(input.WaitIntervals).NotTo(BeEmpty(), "Invalid argument. input.WaitIntervals can't be empty when calling %s spec", AzureMachinePoolStoppedVMSpecName)

	mgmtClient := input.BootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	clusterLabels := map[string]string{clusterv1.ClusterNameLabel: input.ClusterName}

	By("Listing AzureMachinePoolMachines for the cluster")
	ampmList := &infrav1exp.AzureMachinePoolMachineList{}
	Expect(mgmtClient.List(ctx, ampmList, client.InNamespace(input.Namespace.Name), client.MatchingLabels(clusterLabels))).To(Succeed())
	Expect(ampmList.Items).NotTo(BeEmpty(), "expected at least one AzureMachinePoolMachine")

	// Pick the first ready instance to stop.
	var targetAMPM infrav1exp.AzureMachinePoolMachine
	for _, ampm := range ampmList.Items {
		if ampm.Status.Ready && ampm.Status.ProvisioningState != nil && *ampm.Status.ProvisioningState == infrav1.Succeeded {
			targetAMPM = ampm
			break
		}
	}
	Expect(targetAMPM.Name).NotTo(BeEmpty(), "expected at least one ready AzureMachinePoolMachine")

	// Parse the provider ID to extract VMSS name, instance ID, and resource group.
	providerID := targetAMPM.Spec.ProviderID
	Byf("Selected AzureMachinePoolMachine %s with provider ID %s", targetAMPM.Name, providerID)

	// Provider ID format: azure:///subscriptions/.../resourceGroups/<rg>/providers/Microsoft.Compute/virtualMachineScaleSets/<vmss>/virtualMachines/<id>
	resourceID := strings.TrimPrefix(providerID, "azure://")
	parsed, err := azureutil.ParseResourceID(resourceID)
	Expect(err).NotTo(HaveOccurred(), "failed to parse provider ID %s", providerID)

	resourceGroup := parsed.ResourceGroupName

	// Walk the parent chain to extract the VMSS name and instance ID.
	// For Uniform VMSS: the resource type is "virtualMachines" under "virtualMachineScaleSets".
	var vmssName, instanceID string
	switch parsed.ResourceType.Type {
	case "virtualMachineScaleSets/virtualMachines":
		vmssName = parsed.Parent.Name
		instanceID = parsed.Name
	case "virtualMachines":
		Skip("Stopping Flex VMSS VMs is not covered by this test")
	default:
		Fail("unexpected resource type in provider ID: " + parsed.ResourceType.Type)
	}
	Byf("VMSS: %s, Instance ID: %s, Resource Group: %s", vmssName, instanceID, resourceGroup)

	// Find the AzureMachinePool that owns the target AzureMachinePoolMachine.
	ampList := &infrav1exp.AzureMachinePoolList{}
	Expect(mgmtClient.List(ctx, ampList, client.InNamespace(input.Namespace.Name), client.MatchingLabels(clusterLabels))).To(Succeed())
	Expect(ampList.Items).NotTo(BeEmpty())
	var amp *infrav1exp.AzureMachinePool
	for i, candidate := range ampList.Items {
		for _, ref := range targetAMPM.OwnerReferences {
			if ref.Kind == "AzureMachinePool" && ref.Name == candidate.Name {
				amp = &ampList.Items[i]
				break
			}
		}
		if amp != nil {
			break
		}
	}
	Expect(amp).NotTo(BeNil(), "expected to find AzureMachinePool owning %s", targetAMPM.Name)
	mp, err := azureutil.FindParentMachinePool(amp.Name, mgmtClient)
	Expect(err).NotTo(HaveOccurred())
	Expect(mp).NotTo(BeNil())
	Byf("MachinePool %s has %d desired replicas", mp.Name, ptr.Deref(mp.Spec.Replicas, 0))

	By("Stopping the VMSS instance via Azure API")
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	Expect(err).NotTo(HaveOccurred())

	vmssVMClient, err := armcompute.NewVirtualMachineScaleSetVMsClient(getSubscriptionID(Default), cred, nil)
	Expect(err).NotTo(HaveOccurred())

	poller, err := vmssVMClient.BeginPowerOff(ctx, resourceGroup, vmssName, instanceID, nil)
	Expect(err).NotTo(HaveOccurred())
	_, err = poller.PollUntilDone(ctx, nil)
	Expect(err).NotTo(HaveOccurred())

	Byf("Successfully stopped VMSS instance %s/%s/%s", resourceGroup, vmssName, instanceID)

	By("Waiting for the stopped AzureMachinePoolMachine to be cleaned up")
	Eventually(func(g Gomega) {
		currentAMPM := &infrav1exp.AzureMachinePoolMachine{}
		err := mgmtClient.Get(ctx, client.ObjectKeyFromObject(&targetAMPM), currentAMPM)
		if apierrors.IsNotFound(err) {
			// Object is gone — cleanup succeeded.
			return
		}
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(currentAMPM.Status.Ready).To(BeFalse(),
			"expected stopped AzureMachinePoolMachine %s to be marked not ready", targetAMPM.Name)
	}, input.WaitIntervals...).Should(Succeed())

	By("Waiting for MachinePool to recover to the desired replica count")
	framework.WaitForMachinePoolNodesToExist(ctx, framework.WaitForMachinePoolNodesToExistInput{
		Getter:      mgmtClient,
		MachinePool: mp,
	}, input.WaitIntervals...)

	Byf("MachinePool %s recovered to %d nodes", mp.Name, ptr.Deref(mp.Spec.Replicas, 0))
}

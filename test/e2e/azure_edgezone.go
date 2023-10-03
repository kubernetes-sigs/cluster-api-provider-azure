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

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AzureEdgeZoneClusterSpecInput is the input for Azure
type AzureEdgeZoneClusterSpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	E2EConfig             *clusterctl.E2EConfig
}

func AzureEdgeZoneClusterSpec(ctx context.Context, inputGetter func() AzureEdgeZoneClusterSpecInput) {
	var (
		specName = "azure-edgezone-cluster"
		input    AzureEdgeZoneClusterSpecInput
	)

	input = inputGetter()
	Expect(input).NotTo(BeNil())
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)

	By("creating a Kubernetes client to the workload cluster")
	workloadClusterProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
	Expect(workloadClusterProxy).NotTo(BeNil())
	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	By("Retrieving all machines from the machine template spec")
	machineList := &infrav1.AzureMachineList{}
	// list all of the requested objects within the cluster namespace with the cluster name label
	Logf("Listing machines in namespace %s with label %s=%s", input.Namespace.Name, clusterv1.ClusterNameLabel, workloadClusterProxy.GetName())
	err := mgmtClient.List(ctx, machineList, client.InNamespace(input.Namespace.Name), client.MatchingLabels{clusterv1.ClusterNameLabel: workloadClusterProxy.GetName()})
	Expect(err).NotTo(HaveOccurred())

	By("Getting extendedLocation Name and Type from environment variables or e2e config file")
	extendedLocationType := input.E2EConfig.GetVariable(AzureExtendedLocationType)
	extendedLocationName := input.E2EConfig.GetVariable(AzureExtendedLocationName)

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	Expect(err).NotTo(HaveOccurred())

	if len(machineList.Items) > 0 {
		By("Creating a VM client")
		// create a VM client
		vmClient, err := armcompute.NewVirtualMachinesClient(getSubscriptionID(Default), cred, nil)
		Expect(err).NotTo(HaveOccurred())

		// get the resource group name
		resource, err := azureutil.ParseResourceID(*machineList.Items[0].Spec.ProviderID)
		Expect(err).NotTo(HaveOccurred())

		var vms []*armcompute.VirtualMachine
		pager := vmClient.NewListPager(resource.ResourceGroupName, nil)
		for pager.More() {
			nextResult, err := pager.NextPage(ctx)
			Expect(err).NotTo(HaveOccurred())
			vms = append(vms, nextResult.Value...)
		}

		By("Verifying VMs' extendedLocation property is correct")
		for _, machine := range vms {
			Expect(*machine.ExtendedLocation.Name).To(Equal(extendedLocationName))
			Expect(string(*machine.ExtendedLocation.Type)).To(Equal(extendedLocationType))
		}
	}
}

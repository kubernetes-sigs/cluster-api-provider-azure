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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AzureVMExtensionsSpecInput is the input for AzureVMExtensionsSpec.
type AzureVMExtensionsSpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
}

// AzureVMExtensionsSpec implements a test that verifies VM extensions are created and deleted.
func AzureVMExtensionsSpec(ctx context.Context, inputGetter func() AzureVMExtensionsSpecInput) {
	var (
		specName = "azure-vmextensions"
		input    AzureVMExtensionsSpecInput
	)

	Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).ToNot(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	Expect(input.ClusterName).ToNot(BeEmpty(), "Invalid argument. input.ClusterName can't be empty when calling %s spec", specName)

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

	// get subscription id
	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	auth, err := azureutil.GetAuthorizer(settings)
	Expect(err).NotTo(HaveOccurred())

	if len(machineList.Items) > 0 {
		By("Creating a mapping of machine IDs to array of expected VM extensions")
		expectedVMExtensionMap := make(map[string][]string)
		for _, machine := range machineList.Items {
			for _, extension := range machine.Spec.VMExtensions {
				expectedVMExtensionMap[*machine.Spec.ProviderID] = append(expectedVMExtensionMap[*machine.Spec.ProviderID], extension.Name)
			}
		}

		By("Creating a VM and VM extension client")
		// create a VM client
		vmClient := compute.NewVirtualMachinesClient(subscriptionID)
		vmClient.Authorizer = auth

		// create a VM extension client
		vmExtensionsClient := compute.NewVirtualMachineExtensionsClient(subscriptionID)
		vmExtensionsClient.Authorizer = auth

		// get the resource group name
		resource, err := azureutil.ParseResourceID(*machineList.Items[0].Spec.ProviderID)
		Expect(err).NotTo(HaveOccurred())

		vmListResults, err := vmClient.List(ctx, resource.ResourceGroupName, "")
		Expect(err).NotTo(HaveOccurred())

		By("Verifying specified VM extensions are created on Azure")
		for _, machine := range vmListResults.Values() {
			vmExtensionListResult, err := vmExtensionsClient.List(ctx, resource.ResourceGroupName, *machine.Name, "")
			Expect(err).NotTo(HaveOccurred())
			vmExtensionList := *vmExtensionListResult.Value
			var vmExtensionNames []string
			for _, vmExtension := range vmExtensionList {
				vmExtensionNames = append(vmExtensionNames, *vmExtension.Name)
			}
			osName := string(machine.VirtualMachineProperties.StorageProfile.OsDisk.OsType)
			Expect(vmExtensionNames).To(ContainElements("CAPZ." + osName + ".Bootstrapping"))
			Expect(vmExtensionNames).To(ContainElements(expectedVMExtensionMap[*machine.ID]))
		}
	}

	By("Retrieving all machine pools from the machine template spec")
	machinePoolList := &infrav1exp.AzureMachinePoolList{}
	// list all of the requested objects within the cluster namespace with the cluster name label
	Logf("Listing machine pools in namespace %s with label %s=%s", input.Namespace.Name, clusterv1.ClusterNameLabel, workloadClusterProxy.GetName())
	err = mgmtClient.List(ctx, machinePoolList, client.InNamespace(input.Namespace.Name), client.MatchingLabels{clusterv1.ClusterNameLabel: workloadClusterProxy.GetName()})
	Expect(err).NotTo(HaveOccurred())

	if len(machinePoolList.Items) > 0 {
		By("Creating a mapping of machine pool IDs to array of expected VMSS extensions")
		expectedVMSSExtensionMap := make(map[string][]string)
		for _, machinePool := range machinePoolList.Items {
			for _, extension := range machinePool.Spec.Template.VMExtensions {
				expectedVMSSExtensionMap[machinePool.Spec.ProviderID] = append(expectedVMSSExtensionMap[machinePool.Spec.ProviderID], extension.Name)
			}
		}

		By("Creating a VMSS and VMSS extension client")
		// create a VMSS client
		vmssClient := compute.NewVirtualMachineScaleSetsClient(subscriptionID)
		vmssClient.Authorizer = auth

		// create a VMSS extension client
		vmssExtensionsClient := compute.NewVirtualMachineScaleSetExtensionsClient(subscriptionID)
		vmssExtensionsClient.Authorizer = auth

		// get the resource group name
		resource, err := azureutil.ParseResourceID(machinePoolList.Items[0].Spec.ProviderID)
		Expect(err).NotTo(HaveOccurred())

		vmssListResults, err := vmssClient.List(ctx, resource.ResourceGroupName)
		Expect(err).NotTo(HaveOccurred())

		By("Verifying VMSS extensions are created on Azure")
		for _, machinePool := range vmssListResults.Values() {
			vmssExtensionListResult, err := vmssExtensionsClient.List(ctx, resource.ResourceGroupName, *machinePool.Name)
			Expect(err).NotTo(HaveOccurred())
			vmssExtensionList := vmssExtensionListResult.Values()
			var vmssExtensionNames []string
			for _, vmssExtension := range vmssExtensionList {
				vmssExtensionNames = append(vmssExtensionNames, *vmssExtension.Name)
			}
			osName := string(machinePool.VirtualMachineScaleSetProperties.VirtualMachineProfile.StorageProfile.OsDisk.OsType)
			Expect(vmssExtensionNames).To(ContainElements("CAPZ." + osName + ".Bootstrapping"))
			Expect(vmssExtensionNames).To(ContainElements(expectedVMSSExtensionMap[*machinePool.ID]))
		}
	}
}

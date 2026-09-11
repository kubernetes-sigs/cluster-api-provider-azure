//go:build e2e
// +build e2e

/*
Copyright 2026 The Kubernetes Authors.

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
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
)

const (
	// reapplyExtensionName is the name of the deliberately-failing extension used to
	// trigger the Azure VM "Failed" provisioning state in the e2e test.
	reapplyExtensionName = "e2e-reapply-test-failure"

	// reapplyTestTimeout is the total time allowed for the VM to recover via Reapply.
	reapplyTestTimeout = 20 * time.Minute

	// reapplyPollInterval is how often to poll for VM state changes.
	reapplyPollInterval = 30 * time.Second
)

// AzureVMReapplySpecInput is the input for AzureVMReapplySpec.
type AzureVMReapplySpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
}

// AzureVMReapplySpec validates CAPZ's VM Reapply recovery mechanism.
//
// Strategy:
//  1. Find a worker AzureMachine and its backing Azure VM.
//  2. Install a deliberately-failing Custom Script Extension on the VM.
//     A failing extension causes the VM's top-level ProvisioningState to become "Failed".
//  3. Once the VM is in "Failed" state, delete the failing extension so the next
//     provisioning attempt (Reapply) will succeed.
//  4. Wait for CAPZ's reconciler to detect the "Failed" state and invoke the Azure
//     Reapply API automatically.
//  5. Verify the VM's ProvisioningState returns to "Succeeded".
func AzureVMReapplySpec(ctx context.Context, inputGetter func() AzureVMReapplySpecInput) {
	var (
		specName = "azure-vm-reapply"
		input    AzureVMReapplySpecInput
	)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	Expect(input.ClusterName).NotTo(BeEmpty(), "Invalid argument. input.ClusterName can't be empty when calling %s spec", specName)

	mgmtClient := input.BootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	By("Finding a worker AzureMachine in the cluster")
	machineList := &infrav1.AzureMachineList{}
	Expect(mgmtClient.List(ctx, machineList,
		client.InNamespace(input.Namespace.Name),
		client.MatchingLabels{clusterv1.ClusterNameLabel: input.ClusterName},
	)).To(Succeed())

	var workerMachine *infrav1.AzureMachine
	for i := range machineList.Items {
		machine := &machineList.Items[i]
		if _, isControlPlane := machine.Labels[clusterv1.MachineControlPlaneLabel]; !isControlPlane {
			if machine.Spec.ProviderID != nil && *machine.Spec.ProviderID != "" {
				workerMachine = machine
				break
			}
		}
	}
	Expect(workerMachine).NotTo(BeNil(), "Expected at least one provisioned worker AzureMachine in the cluster")
	Logf("Using worker AzureMachine %q for Reapply test", workerMachine.Name)

	resource, err := azureutil.ParseResourceID(*workerMachine.Spec.ProviderID)
	Expect(err).NotTo(HaveOccurred())

	resourceGroup := resource.ResourceGroupName
	vmName := resource.Name
	subscriptionID := getSubscriptionID(Default)

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	Expect(err).NotTo(HaveOccurred())

	vmClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	Expect(err).NotTo(HaveOccurred())

	vmExtClient, err := armcompute.NewVirtualMachineExtensionsClient(subscriptionID, cred, nil)
	Expect(err).NotTo(HaveOccurred())

	By("Verifying the worker VM is initially in Succeeded provisioning state")
	initialVM, err := vmClient.Get(ctx, resourceGroup, vmName, nil)
	Expect(err).NotTo(HaveOccurred())
	Expect(initialVM.Properties).NotTo(BeNil())
	Expect(initialVM.Properties.ProvisioningState).NotTo(BeNil())
	Expect(*initialVM.Properties.ProvisioningState).To(Equal("Succeeded"), "VM must be in Succeeded state before the Reapply test")

	// Determine the VM's OS type to use the correct extension publisher/type.
	Expect(initialVM.Properties.StorageProfile).NotTo(BeNil())
	Expect(initialVM.Properties.StorageProfile.OSDisk).NotTo(BeNil())
	Expect(initialVM.Properties.StorageProfile.OSDisk.OSType).NotTo(BeNil())
	osType := *initialVM.Properties.StorageProfile.OSDisk.OSType

	extensionPublisher, extensionType, extensionVersion, failCommand := failingExtensionForOS(osType)
	Logf("VM %q OS type: %s; using extension %s/%s@%s with command %q", vmName, osType, extensionPublisher, extensionType, extensionVersion, failCommand)

	// Ensure the test extension is cleaned up even if the test fails mid-way.
	DeferCleanup(func(cleanCtx context.Context) {
		Logf("Cleaning up test extension %q from VM %q (if it still exists)", reapplyExtensionName, vmName)
		poller, cleanErr := vmExtClient.BeginDelete(cleanCtx, resourceGroup, vmName, reapplyExtensionName, nil)
		if cleanErr != nil {
			Logf("Skipping extension cleanup (extension may already be deleted): %v", cleanErr)
			return
		}
		if _, cleanErr = poller.PollUntilDone(cleanCtx, nil); cleanErr != nil {
			Logf("Extension cleanup failed (non-fatal): %v", cleanErr)
		}
	})

	By("Installing a deliberately-failing VM extension to trigger Failed provisioning state")
	beginCreatePoller, err := vmExtClient.BeginCreateOrUpdate(ctx, resourceGroup, vmName, reapplyExtensionName,
		armcompute.VirtualMachineExtension{
			Location: initialVM.Location,
			Properties: &armcompute.VirtualMachineExtensionProperties{
				Publisher:               ptr.To(extensionPublisher),
				Type:                    ptr.To(extensionType),
				TypeHandlerVersion:      ptr.To(extensionVersion),
				AutoUpgradeMinorVersion: ptr.To(true),
				Settings: map[string]interface{}{
					"commandToExecute": failCommand,
				},
			},
		}, nil)
	Expect(err).NotTo(HaveOccurred())

	// We expect the extension installation to fail. PollUntilDone will return an error.
	_, extensionErr := beginCreatePoller.PollUntilDone(ctx, nil)
	Logf("Extension installation completed with error (expected): %v", extensionErr)

	By("Waiting for the VM to enter Failed provisioning state")
	Eventually(func(g Gomega) string {
		vm, err := vmClient.Get(ctx, resourceGroup, vmName, nil)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(vm.Properties).NotTo(BeNil())
		g.Expect(vm.Properties.ProvisioningState).NotTo(BeNil())
		state := *vm.Properties.ProvisioningState
		Logf("VM %q provisioning state: %s", vmName, state)
		return state
	}, 10*time.Minute, reapplyPollInterval).Should(Equal("Failed"),
		"VM %q did not enter Failed provisioning state after failing extension installation", vmName)

	By("Removing the failing extension so Reapply will succeed")
	deletePoller, err := vmExtClient.BeginDelete(ctx, resourceGroup, vmName, reapplyExtensionName, nil)
	Expect(err).NotTo(HaveOccurred())
	_, err = deletePoller.PollUntilDone(ctx, nil)
	Expect(err).NotTo(HaveOccurred(), "Failed to delete the failing extension %q from VM %q", reapplyExtensionName, vmName)
	Logf("Failing extension %q deleted from VM %q", reapplyExtensionName, vmName)

	By("Waiting for CAPZ to detect the Failed state and recover the VM via the Azure Reapply API")
	// CAPZ's reconciler periodically checks the VM's provisioning state. When it sees
	// "Failed", it calls the Azure Reapply API (VirtualMachines.BeginReapply) instead
	// of CreateOrUpdate. After successful Reapply the VM returns to "Succeeded".
	Eventually(func(g Gomega) string {
		vm, err := vmClient.Get(ctx, resourceGroup, vmName, nil)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(vm.Properties).NotTo(BeNil())
		g.Expect(vm.Properties.ProvisioningState).NotTo(BeNil())
		state := *vm.Properties.ProvisioningState
		Logf("VM %q provisioning state: %s (waiting for Succeeded)", vmName, state)
		return state
	}, reapplyTestTimeout, reapplyPollInterval).Should(Equal("Succeeded"),
		"VM %q did not recover to Succeeded state after CAPZ Reapply", vmName)

	By("Verifying the AzureMachine condition reflects successful recovery")
	// Give CAPZ a moment to update the AzureMachine status after the VM recovers.
	Eventually(func(g Gomega) {
		updatedMachine := &infrav1.AzureMachine{}
		g.Expect(mgmtClient.Get(ctx, client.ObjectKey{
			Namespace: workerMachine.Namespace,
			Name:      workerMachine.Name,
		}, updatedMachine)).To(Succeed())
		g.Expect(updatedMachine.Status.VMState).NotTo(BeNil(), "AzureMachine VMState should be set")
		Logf("AzureMachine %q VMState: %s", workerMachine.Name, *updatedMachine.Status.VMState)
		g.Expect(string(*updatedMachine.Status.VMState)).To(Equal(string(infrav1.Succeeded)),
			"AzureMachine %q VMState should be Succeeded after Reapply recovery", workerMachine.Name)
	}, 5*time.Minute, reapplyPollInterval).Should(Succeed())
}

// failingExtensionForOS returns the publisher, type, version, and a command string for a
// custom script extension that will always fail, depending on the VM OS type.
func failingExtensionForOS(osType armcompute.OperatingSystemTypes) (publisher, extType, version, command string) {
	if osType == armcompute.OperatingSystemTypesWindows {
		return "Microsoft.Compute", "CustomScriptExtension", "1.10", "exit 1"
	}
	// Linux default
	return "Microsoft.Azure.Extensions", "CustomScript", "2.1", "exit 1"
}

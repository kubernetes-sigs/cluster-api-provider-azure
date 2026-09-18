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
	"fmt"
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
	// trigger the Azure VM "Failed" provisioning state.
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

// AzureVMReapplySpec validates CAPZ's VM Reapply recovery mechanism end-to-end.
//
// The test puts a worker VM into the Azure "Failed" provisioning state by installing
// a Run Command extension (Microsoft.CPlat.Core/RunCommandHandlerLinux) configured
// to exit non-zero. This handler is distinct from CustomScript so it does not
// conflict with any extension CAPZ installs. Once the VM is Failed:
//
//  1. The failing extension is deleted, removing the cause of the failure.
//  2. CAPZ's reconciler detects the "Failed" state and calls the Azure Reapply API.
//  3. Reapply succeeds (nothing left to fail) and the VM returns to "Succeeded".
//  4. The AzureMachine VMState is verified to reflect the recovery.
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

	By("Finding a provisioned Linux worker AzureMachine in the cluster")
	machineList := &infrav1.AzureMachineList{}
	Expect(mgmtClient.List(ctx, machineList,
		client.InNamespace(input.Namespace.Name),
		client.MatchingLabels{clusterv1.ClusterNameLabel: input.ClusterName},
	)).To(Succeed())

	var workerMachine *infrav1.AzureMachine
	for i := range machineList.Items {
		machine := &machineList.Items[i]
		if _, isControlPlane := machine.Labels[clusterv1.MachineControlPlaneLabel]; isControlPlane {
			continue
		}
		if machine.Spec.ProviderID == nil || *machine.Spec.ProviderID == "" {
			continue
		}
		// Only Linux workers; RunCommandHandlerLinux is a Linux-only extension.
		if machine.Spec.OSDisk.OSType != "Linux" {
			continue
		}
		workerMachine = machine
		break
	}
	Expect(workerMachine).NotTo(BeNil(), "Expected at least one provisioned Linux worker AzureMachine in the cluster")
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
	Expect(*initialVM.Properties.ProvisioningState).To(Equal("Succeeded"),
		"VM %q must be in Succeeded state before the Reapply test", vmName)

	By("Checking existing VM extensions to avoid handler conflicts")
	// Azure allows only one extension per handler type (publisher + type pair) on a VM.
	// Microsoft.CPlat.Core/RunCommandHandlerLinux is a separate handler from the
	// CustomScript extensions CAPZ installs, so it does not conflict. Skip if it is
	// somehow already present.
	const (
		testExtPublisher = "Microsoft.CPlat.Core"
		testExtType      = "RunCommandHandlerLinux"
	)
	extListResult, err := vmExtClient.List(ctx, resourceGroup, vmName, nil)
	Expect(err).NotTo(HaveOccurred())
	for _, ext := range extListResult.Value {
		if ext.Properties == nil {
			continue
		}
		if ptr.Deref(ext.Properties.Publisher, "") == testExtPublisher &&
			ptr.Deref(ext.Properties.Type, "") == testExtType {
			Skip(fmt.Sprintf("VM %q already has %s/%s installed; skipping Reapply test to avoid handler conflict", vmName, testExtPublisher, testExtType))
		}
	}

	By("Resolving the latest available version of the test extension from the Azure API")
	// We query the extension image API rather than hardcoding a version, so the test
	// does not break when new versions are published and old ones are retired.
	// VirtualMachineExtensionImage.Name is the version string (e.g. "1.3.30").
	extImagesClient, err := armcompute.NewVirtualMachineExtensionImagesClient(subscriptionID, cred, nil)
	Expect(err).NotTo(HaveOccurred())
	location := ptr.Deref(initialVM.Location, "")
	versionsResp, err := extImagesClient.ListVersions(ctx, location, testExtPublisher, testExtType, nil)
	Expect(err).NotTo(HaveOccurred())
	Expect(versionsResp.VirtualMachineExtensionImageArray).NotTo(BeEmpty(), "No versions found for %s/%s in location %s", testExtPublisher, testExtType, location)
	versions := versionsResp.VirtualMachineExtensionImageArray
	testExtVersion := ptr.Deref(versions[len(versions)-1].Name, "")
	Expect(testExtVersion).NotTo(BeEmpty(), "Could not determine extension version for %s/%s", testExtPublisher, testExtType)
	Logf("Using extension %s/%s version %q", testExtPublisher, testExtType, testExtVersion)
	Logf("Have the following versions")
	for _, version := range versions {
		Logf("extension image info: %+v", *version)
	}

	// Ensure the test extension is removed even if the test fails mid-way.
	DeferCleanup(func(cleanCtx context.Context) {
		Logf("DeferCleanup: removing test extension %q from VM %q (if present)", reapplyExtensionName, vmName)
		poller, cleanErr := vmExtClient.BeginDelete(cleanCtx, resourceGroup, vmName, reapplyExtensionName, nil)
		if cleanErr != nil {
			Logf("DeferCleanup: extension already absent or delete failed (non-fatal): %v", cleanErr)
			return
		}
		if _, cleanErr = poller.PollUntilDone(cleanCtx, nil); cleanErr != nil {
			Logf("DeferCleanup: extension delete poll error (non-fatal): %v", cleanErr)
		}
	})

	By("Installing a deliberately-failing RunCommandHandlerLinux extension to trigger Failed provisioning state")
	// ARM accepts the extension resource; failure occurs when the extension agent on the
	// VM executes the script and "exit 1" returns a non-zero code. Azure then sets the
	// VM's top-level ProvisioningState to "Failed".
	beginCreatePoller, err := vmExtClient.BeginCreateOrUpdate(ctx, resourceGroup, vmName, reapplyExtensionName,
		armcompute.VirtualMachineExtension{
			Location: initialVM.Location,
			Properties: &armcompute.VirtualMachineExtensionProperties{
				Publisher:               ptr.To(testExtPublisher),
				Type:                    ptr.To(testExtType),
				TypeHandlerVersion:      ptr.To(testExtVersion),
				AutoUpgradeMinorVersion: ptr.To(true),
				Settings: map[string]any{
					"commandToExecute": "exit 1",
				},
			},
		}, nil)
	Expect(err).NotTo(HaveOccurred())

	// PollUntilDone will return an error when the extension's script fails.
	_, extensionErr := beginCreatePoller.PollUntilDone(ctx, nil)
	Logf("Extension installation completed with error (expected failure): %v", extensionErr)

	By("Waiting for the VM to enter Failed provisioning state")
	Eventually(func(g Gomega) string {
		vm, getErr := vmClient.Get(ctx, resourceGroup, vmName, nil)
		g.Expect(getErr).NotTo(HaveOccurred())
		g.Expect(vm.Properties).NotTo(BeNil())
		g.Expect(vm.Properties.ProvisioningState).NotTo(BeNil())
		state := *vm.Properties.ProvisioningState
		Logf("VM %q provisioning state: %s", vmName, state)
		return state
	}, 10*time.Minute, reapplyPollInterval).Should(Equal("Failed"),
		"VM %q did not enter Failed provisioning state after failing extension installation", vmName)

	By("Deleting the failing extension so that Reapply will succeed")
	// With the extension gone, the next provisioning attempt (Reapply) has nothing to fail on.
	deletePoller, err := vmExtClient.BeginDelete(ctx, resourceGroup, vmName, reapplyExtensionName, nil)
	Expect(err).NotTo(HaveOccurred())
	_, err = deletePoller.PollUntilDone(ctx, nil)
	Expect(err).NotTo(HaveOccurred(), "Failed to delete the test extension %q from VM %q", reapplyExtensionName, vmName)
	Logf("Test extension %q deleted from VM %q; VM remains in Failed state", reapplyExtensionName, vmName)

	By("Waiting for CAPZ to detect the Failed state and recover the VM via the Azure Reapply API")
	// CAPZ's VM reconciler checks ProvisioningState on each reconcile. When it sees
	// "Failed" it calls VirtualMachines.BeginReapply instead of CreateOrUpdate.
	// After successful Reapply the VM's ProvisioningState returns to "Succeeded".
	Eventually(func(g Gomega) string {
		vm, getErr := vmClient.Get(ctx, resourceGroup, vmName, nil)
		g.Expect(getErr).NotTo(HaveOccurred())
		g.Expect(vm.Properties).NotTo(BeNil())
		g.Expect(vm.Properties.ProvisioningState).NotTo(BeNil())
		state := *vm.Properties.ProvisioningState
		Logf("VM %q provisioning state: %s (waiting for Succeeded after CAPZ Reapply)", vmName, state)
		return state
	}, reapplyTestTimeout, reapplyPollInterval).Should(Equal("Succeeded"),
		"VM %q did not recover to Succeeded state after CAPZ Reapply", vmName)

	By("Verifying the AzureMachine VMState reflects successful recovery")
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

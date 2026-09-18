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
	// reapplyTestTimeout is the total time allowed for the VM to recover via Reapply.
	reapplyTestTimeout = 20 * time.Minute

	// reapplyPollInterval is how often to poll for VM and AzureMachine state changes.
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
// # Why we update the existing CustomScript extension
//
// The CI cluster template installs a no-op CustomScript extension
// (Microsoft.Azure.Extensions/CustomScript) on every worker VM for extension
// testing purposes. Azure allows only one extension per handler type, so we
// cannot add a second CustomScript extension. Instead we update the existing
// one to run "exit 1", causing the VM's ProvisioningState to become "Failed".
//
// Microsoft.Azure.Extensions/CustomScript is chosen specifically because it
// propagates script failures to the VM's top-level ProvisioningState.
// Other extension types (e.g. Microsoft.CPlat.Core/RunCommandHandlerLinux)
// record the failure internally but leave the VM in "Succeeded" state.
//
// # Test flow
//
//  1. Find a Linux worker AzureMachine and its backing Azure VM.
//  2. Locate the existing CustomScript extension on the VM.
//  3. Update the extension to run "exit 1" → script fails → VM = Failed.
//  4. Delete the extension so the next provisioning attempt succeeds.
//  5. Wait for CAPZ's reconciler to detect "Failed" and call the Reapply API.
//  6. Reapply succeeds (no failing extension) → VM = Succeeded.
//  7. CAPZ then reconciles extensions and reinstalls CustomScript with the
//     original no-op command from the AzureMachine spec.
//  8. Verify AzureMachine VMState reflects the recovery.
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

	By("Locating the existing CustomScript extension to use for failure injection")
	// The CI cluster template installs a no-op CustomScript extension on every
	// worker VM. We update it to fail rather than adding a second extension of the
	// same handler type (which Azure prohibits with a 409 Conflict).
	extListResult, err := vmExtClient.List(ctx, resourceGroup, vmName, nil)
	Expect(err).NotTo(HaveOccurred())

	var existingExtName string
	var existingExtVersion string
	for _, ext := range extListResult.Value {
		if ext.Properties == nil || ext.Name == nil {
			continue
		}
		if ptr.Deref(ext.Properties.Publisher, "") == "Microsoft.Azure.Extensions" &&
			ptr.Deref(ext.Properties.Type, "") == "CustomScript" {
			existingExtName = *ext.Name
			existingExtVersion = ptr.Deref(ext.Properties.TypeHandlerVersion, "2.1")
			break
		}
	}
	if existingExtName == "" {
		Skip("No Microsoft.Azure.Extensions/CustomScript extension found on VM; this test requires the CI cluster template which installs one")
	}
	Logf("Found existing CustomScript extension %q (version %s) on VM %q", existingExtName, existingExtVersion, vmName)

	// DeferCleanup reinstates the extension with a no-op command if the test exits
	// before CAPZ's extension reconciler restores it automatically.
	DeferCleanup(func(cleanCtx context.Context) {
		Logf("DeferCleanup: reinstating CustomScript extension %q on VM %q", existingExtName, vmName)
		poller, cleanErr := vmExtClient.BeginCreateOrUpdate(cleanCtx, resourceGroup, vmName, existingExtName,
			armcompute.VirtualMachineExtension{
				Location: initialVM.Location,
				Properties: &armcompute.VirtualMachineExtensionProperties{
					Publisher:               ptr.To("Microsoft.Azure.Extensions"),
					Type:                    ptr.To("CustomScript"),
					TypeHandlerVersion:      ptr.To(existingExtVersion),
					AutoUpgradeMinorVersion: ptr.To(false),
					Settings: map[string]any{
						"commandToExecute": "exit 0",
					},
				},
			}, nil)
		if cleanErr != nil {
			Logf("DeferCleanup: reinstate failed (non-fatal, CAPZ will reconcile): %v", cleanErr)
			return
		}
		if _, cleanErr = poller.PollUntilDone(cleanCtx, nil); cleanErr != nil {
			Logf("DeferCleanup: reinstate poll error (non-fatal): %v", cleanErr)
		}
	})

	By("Deleting the existing CustomScript extension to free the handler slot")
	// Azure allows only one extension per handler type. Deleting the existing one
	// frees the slot so we can install a new failing extension immediately after.
	// Deleting an extension does not trigger VM re-provisioning — the VM stays Succeeded.
	preDeletePoller, err := vmExtClient.BeginDelete(ctx, resourceGroup, vmName, existingExtName, nil)
	Expect(err).NotTo(HaveOccurred())
	_, err = preDeletePoller.PollUntilDone(ctx, nil)
	Expect(err).NotTo(HaveOccurred(), "Failed to delete the existing CustomScript extension from VM %q", vmName)
	Logf("Existing CustomScript extension %q deleted from VM %q; installing failing replacement", existingExtName, vmName)

	By("Installing a new CustomScript extension that exits with code 1 to trigger Failed provisioning state")
	// Installing a NEW extension (not updating an existing one) and having it fail
	// sets the VM's top-level ProvisioningState to "Failed". An extension UPDATE that
	// fails leaves the VM at "Succeeded" — only an initial INSTALL failure causes the
	// VM itself to be marked Failed.
	failPoller, err := vmExtClient.BeginCreateOrUpdate(ctx, resourceGroup, vmName, existingExtName,
		armcompute.VirtualMachineExtension{
			Location: initialVM.Location,
			Properties: &armcompute.VirtualMachineExtensionProperties{
				Publisher:               ptr.To("Microsoft.Azure.Extensions"),
				Type:                    ptr.To("CustomScript"),
				TypeHandlerVersion:      ptr.To(existingExtVersion),
				AutoUpgradeMinorVersion: ptr.To(false),
				Settings: map[string]any{
					"commandToExecute": "exit 1",
				},
			},
		}, nil)
	Expect(err).NotTo(HaveOccurred())

	_, extensionErr := failPoller.PollUntilDone(ctx, nil)
	Logf("Failing extension install completed with error (expected): %v", extensionErr)

	By("Waiting for the VM to enter Failed provisioning state")
	Eventually(func(g Gomega) string {
		resp, getErr := vmClient.Get(ctx, resourceGroup, vmName, nil)
		g.Expect(getErr).NotTo(HaveOccurred())
		g.Expect(resp.Properties).NotTo(BeNil())
		g.Expect(resp.Properties.ProvisioningState).NotTo(BeNil())
		state := *resp.Properties.ProvisioningState
		Logf("VM %q provisioning state: %s", vmName, state)
		return state
	}, 10*time.Minute, reapplyPollInterval).Should(Equal("Failed"),
		"VM %q did not enter Failed provisioning state after failing extension update", vmName)

	By("Deleting the failing extension so Reapply will succeed")
	// Deleting an extension does not trigger re-provisioning — the VM stays in
	// "Failed" state until an explicit operation (Reapply) is issued.
	deletePoller, err := vmExtClient.BeginDelete(ctx, resourceGroup, vmName, existingExtName, nil)
	Expect(err).NotTo(HaveOccurred())
	_, err = deletePoller.PollUntilDone(ctx, nil)
	Expect(err).NotTo(HaveOccurred(), "Failed to delete the CustomScript extension from VM %q", vmName)
	Logf("Extension %q deleted from VM %q; VM remains in Failed state, waiting for CAPZ Reapply", existingExtName, vmName)

	By("Waiting for CAPZ to detect the Failed state and recover the VM via the Azure Reapply API")
	Eventually(func(g Gomega) string {
		resp, getErr := vmClient.Get(ctx, resourceGroup, vmName, nil)
		g.Expect(getErr).NotTo(HaveOccurred())
		g.Expect(resp.Properties).NotTo(BeNil())
		g.Expect(resp.Properties.ProvisioningState).NotTo(BeNil())
		state := *resp.Properties.ProvisioningState
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

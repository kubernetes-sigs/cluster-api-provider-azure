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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	certificatesv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
)

// byoNodeDiagnosticScript is run on each self-managed (BYO) VMSS instance to
// capture why its kubelet never registered with the AKS control plane. It is
// best-effort: every section is guarded so a single missing file or command
// doesn't abort the rest.
const byoNodeDiagnosticScript = `set +e
echo '##### kubeadm/kubelet versions #####'
kubeadm version 2>&1; kubelet --version 2>&1
echo '##### cloud-init status #####'
cloud-init status --long 2>&1
echo '##### bootstrap sentinel (/run/cluster-api) #####'
ls -la /run/cluster-api/ 2>&1
echo '##### /var/log/cloud-init-output.log (last 500 lines) #####'
tail -n 500 /var/log/cloud-init-output.log 2>&1
echo '##### kubelet journal (last 500 lines) #####'
journalctl -u kubelet --no-pager -n 500 2>&1
echo '##### containerd journal (last 100 lines) #####'
journalctl -u containerd --no-pager -n 100 2>&1
echo '##### kubeadm-join config #####'
cat /run/kubeadm/kubeadm-join-config.yaml 2>&1
echo '##### /etc/kubernetes contents #####'
ls -la /etc/kubernetes/ 2>&1; ls -la /etc/kubernetes/pki/ 2>&1
`

// collectBYONodeDiagnostics gathers diagnostics that explain why the BYO
// self-managed VMSS nodes failed to join the AKS control plane. The standard
// e2e log collector relies on SSH'ing through a control-plane bastion, which
// doesn't exist for a managed (AKS) control plane, so the BYO node logs are
// never captured on failure. This collects them without SSH:
//   - the AKS cluster's CSRs (to spot a stuck kubelet client-cert TLS bootstrap),
//     nodes, and recent warning events;
//   - per-instance kubelet/cloud-init logs via the Azure RunCommand API; and
//   - per-instance serial console boot logs via Azure boot diagnostics.
//
// It is best-effort and never fails the spec; collection errors are logged.
func collectBYONodeDiagnostics(ctx context.Context, clientset kubernetes.Interface, subscriptionID, nodeResourceGroup, vmssName, outputPath string) {
	Logf("Collecting BYO node diagnostics into %q (VMSS %q in resource group %q)", outputPath, vmssName, nodeResourceGroup)
	if err := os.MkdirAll(outputPath, 0o750); err != nil {
		Logf("failed to create BYO diagnostics output dir %q: %v", outputPath, err)
		return
	}

	collectAKSClusterDiagnostics(ctx, clientset, outputPath)
	collectBYOVMSSInstanceLogs(ctx, subscriptionID, nodeResourceGroup, vmssName, outputPath)
}

// collectAKSClusterDiagnostics dumps CSRs, nodes, and warning events from the
// AKS control plane. A kubelet that can't complete TLS bootstrap typically
// leaves a Pending kube-apiserver-client-kubelet CSR, so this is the fastest
// signal for the BYO-join failure.
func collectAKSClusterDiagnostics(ctx context.Context, clientset kubernetes.Interface, outputPath string) {
	var sb strings.Builder
	csrs, err := clientset.CertificatesV1().CertificateSigningRequests().List(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(&sb, "failed to list CSRs: %v\n", err)
	} else {
		fmt.Fprintf(&sb, "found %d CSR(s)\n\n", len(csrs.Items))
		for i := range csrs.Items {
			csr := &csrs.Items[i]
			state := "Pending"
			for _, c := range csr.Status.Conditions {
				if c.Type == certificatesv1.CertificateApproved || c.Type == certificatesv1.CertificateDenied {
					state = string(c.Type)
				}
			}
			fmt.Fprintf(&sb, "name=%s state=%s signerName=%s username=%q groups=%v\n",
				csr.Name, state, csr.Spec.SignerName, csr.Spec.Username, csr.Spec.Groups)
		}
	}
	writeStringToFile(filepath.Join(outputPath, "aks-csrs.txt"), sb.String())

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		writeStringToFile(filepath.Join(outputPath, "aks-nodes.txt"), fmt.Sprintf("failed to list nodes: %v\n", err))
	} else {
		var nb strings.Builder
		fmt.Fprintf(&nb, "found %d node(s)\n\n", len(nodes.Items))
		for i := range nodes.Items {
			n := &nodes.Items[i]
			fmt.Fprintf(&nb, "name=%s providerID=%q kubeletVersion=%s\n", n.Name, n.Spec.ProviderID, n.Status.NodeInfo.KubeletVersion)
		}
		writeStringToFile(filepath.Join(outputPath, "aks-nodes.txt"), nb.String())
	}

	var eb strings.Builder
	for _, ns := range []string{metav1.NamespaceDefault, metav1.NamespaceSystem} {
		events, err := clientset.CoreV1().Events(ns).List(ctx, metav1.ListOptions{FieldSelector: "type=Warning"})
		if err != nil {
			fmt.Fprintf(&eb, "[%s] failed to list events: %v\n", ns, err)
			continue
		}
		for i := range events.Items {
			e := &events.Items[i]
			fmt.Fprintf(&eb, "[%s] %s %s/%s: %s\n", ns, e.LastTimestamp.Time.Format(time.RFC3339), e.InvolvedObject.Kind, e.InvolvedObject.Name, e.Message)
		}
	}
	writeStringToFile(filepath.Join(outputPath, "aks-warning-events.txt"), eb.String())
}

// collectBYOVMSSInstanceLogs pulls kubelet/cloud-init logs (via RunCommand) and
// serial console boot logs (via boot diagnostics) from each BYO VMSS instance.
func collectBYOVMSSInstanceLogs(ctx context.Context, subscriptionID, resourceGroup, vmssName, outputPath string) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		Logf("failed to create Azure credential for BYO diagnostics: %v", err)
		return
	}
	vmssVMClient, err := armcompute.NewVirtualMachineScaleSetVMsClient(subscriptionID, cred, nil)
	if err != nil {
		Logf("failed to create VMSS VMs client for BYO diagnostics: %v", err)
		return
	}

	pager := vmssVMClient.NewListPager(resourceGroup, vmssName, nil)
	var instanceIDs []string
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			Logf("failed to list VMSS %q instances for BYO diagnostics: %v", vmssName, err)
			break
		}
		for _, vm := range page.Value {
			if vm.InstanceID != nil {
				instanceIDs = append(instanceIDs, *vm.InstanceID)
			}
		}
	}
	if len(instanceIDs) == 0 {
		Logf("no VMSS %q instances found for BYO diagnostics", vmssName)
		return
	}

	for _, instanceID := range instanceIDs {
		instanceDir := filepath.Join(outputPath, fmt.Sprintf("instance-%s", instanceID))
		if err := os.MkdirAll(instanceDir, 0o750); err != nil {
			Logf("failed to create BYO instance dir %q: %v", instanceDir, err)
			continue
		}

		// Boot diagnostics (serial console) works even if the VM is unhealthy.
		if err := collectVMSSInstanceBootLog(ctx, vmssVMClient, resourceGroup, vmssName, instanceID, instanceDir); err != nil {
			Logf("failed to collect boot log for VMSS %q instance %q: %v", vmssName, instanceID, err)
		}

		// RunCommand captures kubelet/cloud-init logs that aren't on the console.
		output, err := runShellScriptOnVMSSInstance(ctx, vmssVMClient, resourceGroup, vmssName, instanceID, byoNodeDiagnosticScript)
		if err != nil {
			Logf("failed to run diagnostic script on VMSS %q instance %q: %v", vmssName, instanceID, err)
			output = fmt.Sprintf("RunCommand failed: %v", err)
		}
		writeStringToFile(filepath.Join(instanceDir, "node-diagnostics.txt"), output)
	}
}

// runShellScriptOnVMSSInstance runs a shell script on a single VMSS instance via
// the Azure RunCommand API and returns the combined output messages.
func runShellScriptOnVMSSInstance(ctx context.Context, client *armcompute.VirtualMachineScaleSetVMsClient, resourceGroup, vmssName, instanceID, script string) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, 8*time.Minute)
	defer cancel()

	poller, err := client.BeginRunCommand(runCtx, resourceGroup, vmssName, instanceID, armcompute.RunCommandInput{
		CommandID: ptr.To("RunShellScript"),
		Script:    []*string{ptr.To(script)},
	}, nil)
	if err != nil {
		return "", err
	}
	resp, err := poller.PollUntilDone(runCtx, nil)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	for _, status := range resp.Value {
		if status != nil && status.Message != nil {
			sb.WriteString(*status.Message)
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}

func writeStringToFile(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		Logf("failed to write %q: %v", path, err)
	}
}

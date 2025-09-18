//go:build e2e
// +build e2e

/*
Copyright 2020 The Kubernetes Authors.

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
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kinderrors "sigs.k8s.io/kind/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
)

// AzureLogCollector collects logs from a CAPZ workload cluster.
type AzureLogCollector struct{}

const (
	collectLogInterval = 3 * time.Second
	collectLogTimeout  = 1 * time.Minute
)

var _ framework.ClusterLogCollector = &AzureLogCollector{}

// CollectMachineLog collects logs from a machine.
func (k AzureLogCollector) CollectMachineLog(ctx context.Context, managementClusterClient client.Client, m *clusterv1.Machine, outputPath string) error {
	infraGV, err := schema.ParseGroupVersion(m.Spec.InfrastructureRef.APIVersion)
	if err != nil {
		return fmt.Errorf("invalid spec.infrastructureRef.apiVersion %q: %w", m.Spec.InfrastructureRef.APIVersion, err)
	}
	infraKind := m.Spec.InfrastructureRef.Kind
	infraGK := schema.GroupKind{
		Group: infraGV.Group,
		Kind:  infraKind,
	}

	switch infraGK {
	case infrav1.GroupVersion.WithKind(infrav1.AzureMachineKind).GroupKind():
		return collectAzureMachineLog(ctx, managementClusterClient, m, outputPath)
	case infrav1exp.GroupVersion.WithKind(infrav1exp.AzureMachinePoolMachineKind).GroupKind():
		// Logs collected for AzureMachinePool
	default:
		Logf("Unknown machine infra kind: %s", infraGV.WithKind(infraKind))
	}

	return nil
}

// CollectMachinePoolLog collects logs from a machine pool.
func (k AzureLogCollector) CollectMachinePoolLog(ctx context.Context, managementClusterClient client.Client, mp *expv1.MachinePool, outputPath string) error {
	infraGV, err := schema.ParseGroupVersion(mp.Spec.Template.Spec.InfrastructureRef.APIVersion)
	if err != nil {
		return fmt.Errorf("invalid spec.infrastructureRef.apiVersion %q: %w", mp.Spec.Template.Spec.InfrastructureRef.APIVersion, err)
	}
	infraKind := mp.Spec.Template.Spec.InfrastructureRef.Kind
	infraGK := schema.GroupKind{
		Group: infraGV.Group,
		Kind:  infraKind,
	}

	switch infraGK {
	case infrav1exp.GroupVersion.WithKind(infrav1.AzureMachinePoolKind).GroupKind():
		return collectAzureMachinePoolLog(ctx, managementClusterClient, mp, outputPath)
	case infrav1.GroupVersion.WithKind(infrav1.AzureManagedMachinePoolKind).GroupKind():
		// AKS node logs aren't accessible.
		Logf("Skipping logs for %s", infrav1.AzureManagedMachinePoolKind)
	case infrav1.GroupVersion.WithKind(infrav1.AzureASOManagedMachinePoolKind).GroupKind():
		// AKS node logs aren't accessible.
		Logf("Skipping logs for %s", infrav1.AzureASOManagedMachinePoolKind)
	default:
		Logf("Unknown machine pool infra kind: %s", infraGV.WithKind(infraKind))
	}

	return nil
}

// CollectInfrastructureLogs collects log from the infrastructure.
// This is currently a no-op implementation to satisfy the LogCollector interface.
func (k AzureLogCollector) CollectInfrastructureLogs(_ context.Context, _ client.Client, _ *clusterv1.Cluster, _ string) error {
	return nil
}

func collectAzureMachineLog(ctx context.Context, managementClusterClient client.Client, m *clusterv1.Machine, outputPath string) error {
	am, err := getAzureMachine(ctx, managementClusterClient, m)
	if err != nil {
		return fmt.Errorf("get AzureMachine %s/%s: %w", m.Spec.InfrastructureRef.Namespace, m.Spec.InfrastructureRef.Name, err)
	}

	cluster, err := util.GetClusterFromMetadata(ctx, managementClusterClient, m.ObjectMeta)
	if err != nil {
		return err
	}

	azureCluster, err := getAzureCluster(ctx, managementClusterClient, cluster.Spec.InfrastructureRef.Namespace, cluster.Spec.InfrastructureRef.Name)
	if err != nil {
		return fmt.Errorf("get AzureCluster %s/%s: %w", cluster.Spec.InfrastructureRef.Namespace, cluster.Spec.InfrastructureRef.Name, err)
	}
	subscriptionID := azureCluster.Spec.SubscriptionID
	resourceGroup := azureCluster.Spec.ResourceGroup
	name := (&scope.MachineScope{AzureMachine: am}).Name()

	return collectVMLog(ctx, cluster, subscriptionID, resourceGroup, name, outputPath)
}

func collectAzureMachinePoolLog(ctx context.Context, managementClusterClient client.Client, mp *expv1.MachinePool, outputPath string) error {
	am, err := getAzureMachinePool(ctx, managementClusterClient, mp)
	if err != nil {
		return fmt.Errorf("get AzureMachinePool %s/%s: %w", mp.Spec.Template.Spec.InfrastructureRef.Namespace, mp.Spec.Template.Spec.InfrastructureRef.Name, err)
	}

	cluster, err := util.GetClusterFromMetadata(ctx, managementClusterClient, mp.ObjectMeta)
	if err != nil {
		return err
	}

	azureCluster, err := getAzureCluster(ctx, managementClusterClient, cluster.Spec.InfrastructureRef.Namespace, cluster.Spec.InfrastructureRef.Name)
	if err != nil {
		return fmt.Errorf("get AzureCluster %s/%s: %w", cluster.Spec.InfrastructureRef.Namespace, cluster.Spec.InfrastructureRef.Name, err)
	}
	subscriptionID := azureCluster.Spec.SubscriptionID
	resourceGroup := azureCluster.Spec.ResourceGroup
	name := (&scope.MachinePoolScope{AzureMachinePool: am}).Name()

	return collectVMSSLog(ctx, cluster, subscriptionID, resourceGroup, name, outputPath)
}

func collectVMLog(ctx context.Context, cluster *clusterv1.Cluster, subscriptionID, resourceGroup, name, outputPath string) error {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return errors.Wrap(err, "failed to get default azure credential")
	}
	vmClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create virtual machine scale sets client: %w", err)
	}

	vm, err := vmClient.Get(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("get virtual machine %s in resource group %s: %w", name, resourceGroup, err)
	}
	isWindows := vm.Properties != nil &&
		vm.Properties.StorageProfile != nil &&
		vm.Properties.StorageProfile.OSDisk != nil &&
		vm.Properties.StorageProfile.OSDisk.OSType != nil &&
		*vm.Properties.StorageProfile.OSDisk.OSType == armcompute.OperatingSystemTypesWindows

	var errs []error

	if vm.Properties == nil ||
		vm.Properties.OSProfile == nil ||
		vm.Properties.OSProfile.ComputerName == nil {
		errs = append(errs, fmt.Errorf("virtual machine %s in resource group %s has no computer name, can't collect logs via SSH", name, resourceGroup))
	} else {
		hostname := *vm.Properties.OSProfile.ComputerName
		if err := collectLogsFromNode(cluster, hostname, isWindows, outputPath); err != nil {
			errs = append(errs, err)
		}
	}

	if err := collectVMBootLog(ctx, vmClient, resourceGroup, name, outputPath); err != nil {
		errs = append(errs, errors.Wrap(err, "Unable to collect VM Boot Diagnostic logs"))
	}

	return kinderrors.NewAggregate(errs)
}

func collectVMSSLog(ctx context.Context, cluster *clusterv1.Cluster, subscriptionID, resourceGroup, name, outputPath string) error {
	vmssID := azure.VMSSID(subscriptionID, resourceGroup, name)

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return errors.Wrap(err, "failed to get default azure credential")
	}
	vmssClient, err := armcompute.NewVirtualMachineScaleSetsClient(subscriptionID, cred, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create virtual machine scale sets client")
	}

	vmss, err := vmssClient.Get(ctx, resourceGroup, name, nil)
	if err != nil {
		return fmt.Errorf("get VMSS %s: %w", vmssID, err)
	}

	var mode armcompute.OrchestrationMode
	if vmss.Properties != nil &&
		vmss.Properties.OrchestrationMode != nil {
		mode = *vmss.Properties.OrchestrationMode
	}
	switch mode {
	case armcompute.OrchestrationModeUniform:
		isWindows := vmss.Properties != nil &&
			vmss.Properties.VirtualMachineProfile != nil &&
			vmss.Properties.VirtualMachineProfile.StorageProfile != nil &&
			vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk != nil &&
			vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk.OSType != nil &&
			*vmss.Properties.VirtualMachineProfile.StorageProfile.OSDisk.OSType == armcompute.OperatingSystemTypesWindows

		instanceClient, err := armcompute.NewVirtualMachineScaleSetVMsClient(subscriptionID, cred, nil)
		if err != nil {
			return errors.Wrap(err, "failed to create virtual machine scale set client")
		}
		pager := instanceClient.NewListPager(resourceGroup, name, nil)
		var errs []error
		for pager.More() {
			instances, err := pager.NextPage(ctx)
			if err != nil {
				errs = append(errs, fmt.Errorf("list VMSS %s instances: %w", vmssID, err))
				continue
			}
			for _, instance := range instances.Value {
				var hostname string

				if instance == nil ||
					instance.Properties == nil ||
					instance.Properties.OSProfile == nil ||
					instance.Properties.OSProfile.ComputerName == nil {
					errs = append(errs, fmt.Errorf("instance of VMSS %s in resource group %s has no computer name, can't collect logs via SSH", name, resourceGroup))
				} else {
					hostname = *instance.Properties.OSProfile.ComputerName
					if err := collectLogsFromNode(cluster, hostname, isWindows, filepath.Join(outputPath, hostname)); err != nil {
						errs = append(errs, err)
					}
				}

				if instance == nil ||
					instance.InstanceID == nil {
					errs = append(errs, fmt.Errorf("VMSS instance has no ID"))
				} else {
					outputDir := hostname
					if outputDir == "" {
						outputDir = "instance-" + *instance.InstanceID
					}
					if err := collectVMSSInstanceBootLog(ctx, instanceClient, resourceGroup, ptr.Deref(vmss.Name, ""), *instance.InstanceID, filepath.Join(outputPath, outputDir)); err != nil {
						errs = append(errs, err)
					}
				}
			}
		}
		return kinderrors.NewAggregate(errs)
	case armcompute.OrchestrationModeFlexible:
		var vmssID string
		if vmss.ID != nil {
			vmssID = *vmss.ID
		} else {
			vmssID = azure.VMSSID(subscriptionID, resourceGroup, name)
			Logf("VMSS has no ID, guessing it's %q", vmssID)
		}

		vmClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
		if err != nil {
			return errors.Wrap(err, "failed to create virtual machine client")
		}
		pager := vmClient.NewListPager(resourceGroup, nil)
		var errs []error
		for pager.More() {
			vms, err := pager.NextPage(ctx)
			if err != nil {
				errs = append(errs, fmt.Errorf("list VMs in resource group %s: %w", resourceGroup, err))
				continue
			}
			for _, vm := range vms.Value {
				if vm == nil ||
					vm.Properties == nil ||
					vm.Properties.VirtualMachineScaleSet == nil ||
					vm.Properties.VirtualMachineScaleSet.ID == nil ||
					*vm.Properties.VirtualMachineScaleSet.ID != vmssID {
					continue
				}

				if vm.Name == nil {
					errs = append(errs, fmt.Errorf("VM for VMSS %s in resource group %s has no name, skipping log collection", name, resourceGroup))
				} else {
					outputDir := *vm.Name
					if vm.Properties != nil &&
						vm.Properties.OSProfile != nil &&
						vm.Properties.OSProfile.ComputerName != nil {
						outputDir = *vm.Properties.OSProfile.ComputerName
					}
					if err := collectVMLog(ctx, cluster, subscriptionID, resourceGroup, *vm.Name, filepath.Join(outputPath, outputDir)); err != nil {
						errs = append(errs, err)
					}
				}
			}
		}
		return kinderrors.NewAggregate(errs)
	default:
		return fmt.Errorf("unknown orchestration mode %q", mode)
	}
}

// collectLogsFromNode collects logs from various sources by ssh'ing into the node
func collectLogsFromNode(cluster *clusterv1.Cluster, hostname string, isWindows bool, outputPath string) error {
	nodeOSType := azure.LinuxOS
	if isWindows {
		nodeOSType = azure.WindowsOS
	}
	Logf("Collecting logs for %s node %s in cluster %s in namespace %s", nodeOSType, hostname, cluster.Name, cluster.Namespace)

	controlPlaneEndpoint := cluster.Spec.ControlPlaneEndpoint.Host

	execToPathFn := func(outputFileName, command string, args ...string) func() error {
		return func() error {
			return retryWithTimeout(collectLogInterval, collectLogTimeout, func() error {
				f, err := fileOnHost(filepath.Join(outputPath, outputFileName))
				if err != nil {
					return err
				}
				defer f.Close()
				return execOnHost(controlPlaneEndpoint, hostname, sshPort, collectLogTimeout, f, command, args...)
			})
		}
	}

	if isWindows {
		// if we initiate to many ssh connections they get dropped (default is 10) so split it up
		var errors []error
		errors = append(errors, kinderrors.AggregateConcurrent(windowsInfo(execToPathFn)))
		errors = append(errors, kinderrors.AggregateConcurrent(windowsK8sLogs(execToPathFn)))
		errors = append(errors, kinderrors.AggregateConcurrent(windowsNetworkLogs(execToPathFn)))
		errors = append(errors, kinderrors.AggregateConcurrent(windowsCrashDumpLogs(execToPathFn)))
		errors = append(errors, sftpCopyFile(controlPlaneEndpoint, hostname, sshPort, collectLogTimeout, "/c:/crashdumps.tar", filepath.Join(outputPath, "crashdumps.tar")))

		return kinderrors.NewAggregate(errors)
	}

	return kinderrors.AggregateConcurrent(linuxLogs(execToPathFn))
}

func getAzureCluster(ctx context.Context, managementClusterClient client.Client, namespace, name string) (*infrav1.AzureCluster, error) {
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	azCluster := &infrav1.AzureCluster{}
	err := managementClusterClient.Get(ctx, key, azCluster)
	return azCluster, err
}

func getAzureManagedControlPlane(ctx context.Context, managementClusterClient client.Client, namespace, name string) (*infrav1.AzureManagedControlPlane, error) {
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	azManagedControlPlane := &infrav1.AzureManagedControlPlane{}
	err := managementClusterClient.Get(ctx, key, azManagedControlPlane)
	return azManagedControlPlane, err
}

func getAzureASOManagedCluster(ctx context.Context, managementClusterClient client.Client, namespace, name string) (*infrav1.AzureASOManagedCluster, error) {
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	azManagedCluster := &infrav1.AzureASOManagedCluster{}
	err := managementClusterClient.Get(ctx, key, azManagedCluster)
	return azManagedCluster, err
}

func getAzureMachine(ctx context.Context, managementClusterClient client.Client, m *clusterv1.Machine) (*infrav1.AzureMachine, error) {
	key := client.ObjectKey{
		Namespace: m.Spec.InfrastructureRef.Namespace,
		Name:      m.Spec.InfrastructureRef.Name,
	}

	azMachine := &infrav1.AzureMachine{}
	err := managementClusterClient.Get(ctx, key, azMachine)
	return azMachine, err
}

func getAzureMachinePool(ctx context.Context, managementClusterClient client.Client, mp *expv1.MachinePool) (*infrav1exp.AzureMachinePool, error) {
	key := client.ObjectKey{
		Namespace: mp.Spec.Template.Spec.InfrastructureRef.Namespace,
		Name:      mp.Spec.Template.Spec.InfrastructureRef.Name,
	}

	azMachinePool := &infrav1exp.AzureMachinePool{}
	err := managementClusterClient.Get(ctx, key, azMachinePool)
	return azMachinePool, err
}

func linuxLogs(execToPathFn func(outputFileName string, command string, args ...string) func() error) []func() error {
	return []func() error{
		execToPathFn(
			"journal.log",
			"sudo", "journalctl", "--no-pager", "--output=short-precise",
		),
		execToPathFn(
			"kern.log",
			"sudo", "journalctl", "--no-pager", "--output=short-precise", "-k",
		),
		execToPathFn(
			"kubelet-version.txt",
			"PATH=/opt/bin:${PATH}", "kubelet", "--version",
		),
		execToPathFn(
			"kubelet.log",
			"sudo", "journalctl", "--no-pager", "--output=short-precise", "-u", "kubelet.service",
		),
		execToPathFn(
			"containerd.log",
			"sudo", "journalctl", "--no-pager", "--output=short-precise", "-u", "containerd.service",
		),
		execToPathFn(
			"ignition.log",
			"sudo", "journalctl", "--no-pager", "--output=short-precise", "-at", "ignition",
		),
		execToPathFn(
			"cloud-init.log",
			"sudo", "sh", "-c", "if [ -f /var/log/cloud-init.log ]; then sudo cat /var/log/cloud-init.log; else echo 'cloud-init.log not found'; fi",
		),
		execToPathFn(
			"cloud-init-output.log",
			"sudo", "sh", "-c", "echo 'Waiting for cloud-init to complete before collecting output log...' && timeout 60 cloud-init status --wait || echo 'Cloud-init wait timed out, proceeding with log collection...' && if [ -f /var/log/cloud-init-output.log ]; then echo 'Found cloud-init-output.log, reading contents:' && sudo cat /var/log/cloud-init-output.log; else echo 'cloud-init-output.log not found'; fi",
		),
		execToPathFn(
			"sentinel-file-dir.txt",
			"ls", "-la", "/run/cluster-api/",
		),
		execToPathFn(
			"cni.log",
			"sudo", "cat", "/var/log/calico/cni/cni.log",
		),
		// If kube-apiserver fails to come up, its logs aren't accessible via `kubectl logs`.
		// Grab them from the node instead.
		execToPathFn(
			"kube-apiserver.log",
			crictlPodLogsCmd("kube-apiserver"),
		),
	}
}

func windowsK8sLogs(execToPathFn func(outputFileName string, command string, args ...string) func() error) []func() error {
	return []func() error{
		execToPathFn(
			"hyperv-operation.log",
			"Get-WinEvent", "-LogName Microsoft-Windows-Hyper-V-Compute-Operational | Select-Object -Property TimeCreated, Id, LevelDisplayName, Message | Sort-Object TimeCreated | Format-Table -Wrap -Autosize",
		),
		execToPathFn(
			"containerd-containers.log",
			"ctr.exe", "-n k8s.io containers list",
		),
		execToPathFn(
			"containerd-tasks.log",
			"ctr.exe", "-n k8s.io tasks list",
		),
		execToPathFn(
			"containers-hcs.log",
			"hcsdiag", "list",
		),
		execToPathFn(
			"kubelet.log",
			`Get-ChildItem "C:\\var\\log\\kubelet\\"  | ForEach-Object { if ($_ -match 'log.INFO|err.*.log') { write-output "$_";cat "c:\\var\\log\\kubelet\\$_" } }`,
		),
		execToPathFn(
			"cni.log",
			`Get-Content "C:\\cni.log"`,
		),
		execToPathFn(
			"containerd.log",
			`Get-Content "C:\\var\\log\\containerd\\containerd.log"`,
		),
	}
}

func windowsInfo(execToPathFn func(outputFileName string, command string, args ...string) func() error) []func() error {
	return []func() error{
		execToPathFn(
			"reboots.log",
			"Get-WinEvent", `-ErrorAction Ignore -FilterHashtable @{logname = 'System'; id = 1074, 1076, 2004, 6005, 6006, 6008 } | Select-Object -Property TimeCreated, Id, LevelDisplayName, Message | Format-Table -Wrap -Autosize`,
		),
		execToPathFn(
			"scm.log",
			"Get-WinEvent", `-FilterHashtable @{logname = 'System'; ProviderName = 'Service Control Manager' } | Select-Object -Property TimeCreated, Id, LevelDisplayName, Message | Format-Table -Wrap -Autosize`,
		),
		execToPathFn(
			"pagefile.log",
			"Get-CimInstance", "win32_pagefileusage | Format-List *",
		),
		execToPathFn(
			"cloudbase-init-unattend.log",
			"get-content 'C:\\Program Files\\Cloudbase Solutions\\Cloudbase-Init\\log\\cloudbase-init-unattend.log'",
		),
		execToPathFn(
			"cloudbase-init.log",
			"get-content 'C:\\Program Files\\Cloudbase Solutions\\Cloudbase-Init\\log\\cloudbase-init.log'",
		),
		execToPathFn(
			"services.log",
			"get-service",
		),
	}
}

func windowsNetworkLogs(execToPathFn func(outputFileName string, command string, args ...string) func() error) []func() error {
	return []func() error{
		execToPathFn(
			"network.log",
			"Get-HnsNetwork | Select Name, Type, Id, AddressPrefix | Format-Table -Wrap -Autosize",
		),
		execToPathFn(
			"network-detailed.log",
			"Get-hnsnetwork | Convertto-json -Depth 20",
		),
		execToPathFn(
			"network-individual-detailed.log",
			"Get-hnsnetwork | % { Get-HnsNetwork -Id $_.ID -Detailed } | Convertto-json -Depth 20",
		),
		execToPathFn(
			"hnsendpoints.log",
			"Get-HnsEndpoint | Select IpAddress, MacAddress, IsRemoteEndpoint, State",
		),
		execToPathFn(
			"hnsendpolicy-detailed.log",
			"Get-hnspolicylist | Convertto-json -Depth 20",
		),
		execToPathFn(
			"ipconfig.log",
			"ipconfig /allcompartments /all",
		),
		execToPathFn(
			"ips.log",
			"Get-NetIPAddress -IncludeAllCompartments",
		),
		execToPathFn(
			"interfaces.log",
			"Get-NetIPInterface -IncludeAllCompartments",
		),
		execToPathFn(
			"hnsdiag.txt",
			"hnsdiag list all -d",
		),
	}
}

func windowsCrashDumpLogs(execToPathFn func(outputFileName string, command string, args ...string) func() error) []func() error {
	return []func() error{
		execToPathFn(
			"dir-localdumps.log",
			// note: the powershell 'ls' alias will not have any output if the target directory is empty.
			// we're logging the contents of the c:\localdumps directory because the command that invokes tar.exe below is
			// not providing output when run in powershell over ssh for some reason.
			"ls 'c:\\localdumps' -Recurse",
		),
		execToPathFn(
			// capture any crashdump files created by windows into a .tar to be collected via sftp
			"tar-crashdumps.log",
			"$p = 'c:\\localdumps' ; if (Test-Path $p) { tar.exe -cvzf c:\\crashdumps.tar $p *>&1 | %{ Write-Output \"$_\"} } else { Write-Host \"No crash dumps found at $p\" }",
		),
	}
}

// collectVMBootLog collects boot logs of the vm by using azure boot diagnostics.
func collectVMBootLog(ctx context.Context, vmClient *armcompute.VirtualMachinesClient, resourceGroup, name, outputPath string) error {
	Logf("Collecting boot logs for resource group %s, VM %s", resourceGroup, name)

	bootDiagnostics, err := vmClient.RetrieveBootDiagnosticsData(ctx, resourceGroup, name, nil)
	if err != nil {
		return errors.Wrap(err, "failed to get boot diagnostics data")
	}

	return writeBootLog(bootDiagnostics.RetrieveBootDiagnosticsDataResult, outputPath)
}

// collectVMSSInstanceBootLog collects boot logs of the vmss instance by using azure boot diagnostics.
func collectVMSSInstanceBootLog(ctx context.Context, instanceClient *armcompute.VirtualMachineScaleSetVMsClient, resourceGroup, vmssName, instanceID, outputPath string) error {
	Logf("Collecting boot logs for resource group %s, VMSS %s, instance %s", resourceGroup, vmssName, instanceID)

	bootDiagnostics, err := instanceClient.RetrieveBootDiagnosticsData(ctx, resourceGroup, vmssName, instanceID, nil)
	if err != nil {
		return errors.Wrap(err, "failed to get boot diagnostics data")
	}

	return writeBootLog(bootDiagnostics.RetrieveBootDiagnosticsDataResult, outputPath)
}

func writeBootLog(bootDiagnostics armcompute.RetrieveBootDiagnosticsDataResult, outputPath string) error {
	var err error
	req, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, *bootDiagnostics.SerialConsoleLogBlobURI, http.NoBody)
	if err != nil {
		return errors.Wrap(err, "failed to create HTTP request")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return errors.Wrap(err, "failed to get logs from serial console uri")
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response body")
	}

	if err := os.WriteFile(filepath.Join(outputPath, "boot.log"), content, 0o600); err != nil {
		return errors.Wrap(err, "failed to write response to file")
	}

	return nil
}

func crictlPodLogsCmd(podNamePattern string) string {
	//nolint: dupword // this is bash, not english
	return `sudo crictl pods --name "` + podNamePattern + `" -o json | jq -c '.items[]' | while read -r pod; do
    sudo crictl ps -a --pod $(jq -r .id <<< $pod) -o json | jq -c '.containers[]' | while read -r ctr; do
        echo "========= Pod $(jq -r .metadata.name <<< $pod), container $(jq -r .metadata.name <<< $ctr) ========="
        sudo crictl logs "$(jq -r .id <<< $ctr)" 2>&1
    done
done`
}

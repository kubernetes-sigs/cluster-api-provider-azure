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
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kinderrors "sigs.k8s.io/kind/pkg/errors"
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
	var errs []error

	am, err := getAzureMachine(ctx, managementClusterClient, m)
	if err != nil {
		return err
	}

	cluster, err := util.GetClusterFromMetadata(ctx, managementClusterClient, m.ObjectMeta)
	if err != nil {
		return err
	}

	hostname := getHostname(m, isAzureMachineWindows(am))

	if err := collectLogsFromNode(cluster, hostname, isAzureMachineWindows(am), outputPath); err != nil {
		errs = append(errs, err)
	}

	if err := collectVMBootLog(ctx, am, outputPath); err != nil {
		errs = append(errs, errors.Wrap(err, "Unable to collect VM Boot Diagnostic logs"))
	}

	return kinderrors.NewAggregate(errs)
}

// CollectMachinePoolLog collects logs from a machine pool.
func (k AzureLogCollector) CollectMachinePoolLog(ctx context.Context, managementClusterClient client.Client, mp *expv1.MachinePool, outputPath string) error {
	var errs []error
	var isWindows bool

	am, err := getAzureMachinePool(ctx, managementClusterClient, mp)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		// Machine pool can be an AzureManagedMachinePool for AKS clusters.
		_, err = getAzureManagedMachinePool(ctx, managementClusterClient, mp)
		if err != nil {
			return err
		}
	} else {
		isWindows = isAzureMachinePoolWindows(am)
	}

	cluster, err := util.GetClusterFromMetadata(ctx, managementClusterClient, mp.ObjectMeta)
	if err != nil {
		return err
	}

	for i, instance := range mp.Spec.ProviderIDList {
		if mp.Status.NodeRefs != nil && len(mp.Status.NodeRefs) >= (i+1) {
			hostname := mp.Status.NodeRefs[i].Name

			if err := collectLogsFromNode(cluster, hostname, isWindows, filepath.Join(outputPath, hostname)); err != nil {
				errs = append(errs, err)
			}

			if err := collectVMSSBootLog(ctx, instance, filepath.Join(outputPath, hostname)); err != nil {
				errs = append(errs, errors.Wrap(err, "Unable to collect VMSS Boot Diagnostic logs"))
			}
		} else {
			Logf("MachinePool instance %s does not have a corresponding NodeRef", instance)
			Logf("Skipping log collection for MachinePool instance %s", instance)
		}
	}

	return kinderrors.NewAggregate(errs)
}

// CollectInfrastructureLogs collects log from the infrastructure.
// This is currently a no-op implementation to satisfy the LogCollector interface.
func (k AzureLogCollector) CollectInfrastructureLogs(ctx context.Context, managementClusterClient client.Client, c *clusterv1.Cluster, outputPath string) error {
	return nil
}

// collectLogsFromNode collects logs from various sources by ssh'ing into the node
func collectLogsFromNode(cluster *clusterv1.Cluster, hostname string, isWindows bool, outputPath string) error {
	nodeOSType := azure.LinuxOS
	if isWindows {
		nodeOSType = azure.WindowsOS
	}
	Logf("Collecting logs for %s node %s in cluster %s in namespace %s\n", nodeOSType, hostname, cluster.Name, cluster.Namespace)

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

func getHostname(m *clusterv1.Machine, isWindows bool) string {
	hostname := m.Spec.InfrastructureRef.Name
	if isWindows {
		// Windows host name ends up being different than the infra machine name
		// due to Windows name limitations in Azure so use ip address instead.
		if len(m.Status.Addresses) > 0 {
			hostname = m.Status.Addresses[0].Address
		} else {
			Logf("Unable to collect logs as node doesn't have addresses")
		}
	}
	return hostname
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

func getAzureManagedMachinePool(ctx context.Context, managementClusterClient client.Client, mp *expv1.MachinePool) (*infrav1.AzureManagedMachinePool, error) {
	key := client.ObjectKey{
		Namespace: mp.Spec.Template.Spec.InfrastructureRef.Namespace,
		Name:      mp.Spec.Template.Spec.InfrastructureRef.Name,
	}

	azManagedMachinePool := &infrav1.AzureManagedMachinePool{}
	err := managementClusterClient.Get(ctx, key, azManagedMachinePool)
	return azManagedMachinePool, err
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
			"cat", "/var/log/cloud-init.log",
		),
		execToPathFn(
			"cloud-init-output.log",
			"cat", "/var/log/cloud-init-output.log",
		),
		execToPathFn(
			"sentinel-file-dir.txt",
			"ls", "/run/cluster-api/",
		),
		execToPathFn(
			"cni.log",
			"cat", "/var/log/calico/cni/cni.log",
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
func collectVMBootLog(ctx context.Context, am *infrav1.AzureMachine, outputPath string) error {
	if am == nil {
		return errors.New("AzureMachine is nil")
	}
	Logf("Collecting boot logs for AzureMachine %s\n", am.GetName())

	if am.Spec.ProviderID == nil {
		return errors.New("AzureMachine provider ID is nil")
	}

	resource, err := azureutil.ParseResourceID(*am.Spec.ProviderID)
	if err != nil {
		return errors.Wrap(err, "failed to parse resource id")
	}

	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	if subscriptionID == "" {
		return errors.New("AZURE_SUBSCRIPTION_ID is not set")
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return errors.Wrap(err, "failed to get default azure credential")
	}

	vmClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create virtual machines client")
	}

	bootDiagnostics, err := vmClient.RetrieveBootDiagnosticsData(ctx, resource.ResourceGroupName, resource.Name, nil)
	if err != nil {
		return errors.Wrap(err, "failed to get boot diagnostics data")
	}

	return writeBootLog(bootDiagnostics.RetrieveBootDiagnosticsDataResult, outputPath)
}

// collectVMSSBootLog collects boot logs of the scale set by using azure boot diagnostics.
func collectVMSSBootLog(ctx context.Context, providerID string, outputPath string) error {
	resourceID := strings.TrimPrefix(providerID, azureutil.ProviderIDPrefix)
	v := strings.Split(resourceID, "/")
	instanceID := v[len(v)-1]
	resourceID = strings.TrimSuffix(resourceID, "/virtualMachines/"+instanceID)
	resource, err := azureutil.ParseResourceID(resourceID)
	if err != nil {
		return errors.Wrap(err, "failed to parse resource id")
	}

	Logf("Collecting boot logs for VMSS instance %s of scale set %s\n", instanceID, resource.Name)

	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	if subscriptionID == "" {
		return errors.New("AZURE_SUBSCRIPTION_ID is not set")
	}

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return errors.Wrap(err, "failed to get default azure credential")
	}
	vmssClient, err := armcompute.NewVirtualMachineScaleSetVMsClient(subscriptionID, cred, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create virtual machine scale set VMs client")
	}

	bootDiagnostics, err := vmssClient.RetrieveBootDiagnosticsData(ctx, resource.ResourceGroupName, resource.Name, instanceID, nil)
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
	if err != nil || resp.StatusCode != 200 {
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

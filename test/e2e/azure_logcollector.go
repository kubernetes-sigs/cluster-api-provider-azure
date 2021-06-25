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
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	expv1alpha4 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1alpha4"
	"sigs.k8s.io/cluster-api/test/framework"

	"sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/azure"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	autorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/pkg/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kinderrors "sigs.k8s.io/kind/pkg/errors"
)

// AzureLogCollector collects logs from a CAPZ workload cluster.
type AzureLogCollector struct{}

var _ framework.ClusterLogCollector = &AzureLogCollector{}

// CollectMachineLog collects logs from a machine.
func (k AzureLogCollector) CollectMachineLog(ctx context.Context, managementClusterClient client.Client, m *clusterv1.Machine, outputPath string) error {
	var errors []error

	am, err := getAzureMachine(ctx, managementClusterClient, m)
	if err != nil {
		return err
	}

	cluster, err := util.GetClusterFromMetadata(ctx, managementClusterClient, m.ObjectMeta)
	if err != nil {
		return err
	}

	hostname := getHostname(m, isAzureMachineWindows(am))

	if err := collectLogsFromNode(ctx, managementClusterClient, cluster, hostname, isAzureMachineWindows(am), outputPath); err != nil {
		errors = append(errors, err)
	}

	if err := collectVMBootLog(ctx, am, outputPath); err != nil {
		errors = append(errors, err)
	}

	return kinderrors.NewAggregate(errors)
}

// CollectMachinePoolLog collects logs from a machine pool.
func (k AzureLogCollector) CollectMachinePoolLog(ctx context.Context, managementClusterClient client.Client, mp *expv1.MachinePool, outputPath string) error {
	var errors []error
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
		hostname := mp.Status.NodeRefs[i].Name

		if err := collectLogsFromNode(ctx, managementClusterClient, cluster, hostname, isWindows, filepath.Join(outputPath, hostname)); err != nil {
			errors = append(errors, err)
		}

		if err := collectVMSSBootLog(ctx, instance, filepath.Join(outputPath, hostname)); err != nil {
			errors = append(errors, err)
		}
	}

	return kinderrors.NewAggregate(errors)
}

// collectLogsFromNode collects logs from various sources by ssh'ing into the node
func collectLogsFromNode(ctx context.Context, managementClusterClient client.Client, cluster *clusterv1.Cluster, hostname string, isWindows bool, outputPath string) error {
	Logf("INFO: Collecting logs for node %s in cluster %s in namespace %s\n", hostname, cluster.Name, cluster.Namespace)

	controlPlaneEndpoint := cluster.Spec.ControlPlaneEndpoint.Host

	execToPathFn := func(outputFileName, command string, args ...string) func() error {
		return func() error {
			f, err := fileOnHost(filepath.Join(outputPath, outputFileName))
			if err != nil {
				return err
			}
			defer f.Close()
			return retryWithExponentialBackOff(func() error {
				return execOnHost(controlPlaneEndpoint, hostname, sshPort, f, command, args...)
			})
		}
	}

	if isWindows {
		// if we initiate to many ssh connections they get dropped (default is 10) so split it up
		var errors []error
		errors = append(errors, kinderrors.AggregateConcurrent(windowsInfo(execToPathFn)))
		errors = append(errors, kinderrors.AggregateConcurrent(windowsK8sLogs(execToPathFn)))
		errors = append(errors, kinderrors.AggregateConcurrent(windowsNetworkLogs(execToPathFn)))
		return kinderrors.NewAggregate(errors)
	}

	return kinderrors.AggregateConcurrent(linuxLogs(execToPathFn))
}

func isAzureMachineWindows(am *v1alpha4.AzureMachine) bool {
	return am.Spec.OSDisk.OSType == azure.WindowsOS
}

func isAzureMachinePoolWindows(amp *expv1alpha4.AzureMachinePool) bool {
	return amp.Spec.Template.OSDisk.OSType == azure.WindowsOS
}

func getHostname(m *clusterv1.Machine, isWindows bool) string {
	hostname := m.Spec.InfrastructureRef.Name
	if isWindows {
		// Windows host name ends up being different than the infra machine name
		// due to Windows name limitations in Azure so use ip address instead.
		if len(m.Status.Addresses) > 0 {
			hostname = m.Status.Addresses[0].Address
		} else {
			Logf("INFO: Unable to collect logs as node doesn't have addresses")
		}
	}
	return hostname
}

func getAzureMachine(ctx context.Context, managementClusterClient client.Client, m *clusterv1.Machine) (*v1alpha4.AzureMachine, error) {
	key := client.ObjectKey{
		Namespace: m.Spec.InfrastructureRef.Namespace,
		Name:      m.Spec.InfrastructureRef.Name,
	}

	azMachine := &v1alpha4.AzureMachine{}
	err := managementClusterClient.Get(ctx, key, azMachine)
	return azMachine, err
}

func getAzureMachinePool(ctx context.Context, managementClusterClient client.Client, mp *expv1.MachinePool) (*expv1alpha4.AzureMachinePool, error) {
	key := client.ObjectKey{
		Namespace: mp.Spec.Template.Spec.InfrastructureRef.Namespace,
		Name:      mp.Spec.Template.Spec.InfrastructureRef.Name,
	}

	azMachinePool := &expv1alpha4.AzureMachinePool{}
	err := managementClusterClient.Get(ctx, key, azMachinePool)
	return azMachinePool, err
}

func getAzureManagedMachinePool(ctx context.Context, managementClusterClient client.Client, mp *expv1.MachinePool) (*expv1alpha4.AzureManagedMachinePool, error) {
	key := client.ObjectKey{
		Namespace: mp.Spec.Template.Spec.InfrastructureRef.Namespace,
		Name:      mp.Spec.Template.Spec.InfrastructureRef.Name,
	}

	azManagedMachinePool := &expv1alpha4.AzureManagedMachinePool{}
	err := managementClusterClient.Get(ctx, key, azManagedMachinePool)
	return azManagedMachinePool, err
}

func linuxLogs(execToPathFn func(outputFileName string, command string, args ...string) func() error) []func() error {
	return []func() error{
		execToPathFn(
			"journal.log",
			"journalctl", "--no-pager", "--output=short-precise",
		),
		execToPathFn(
			"kern.log",
			"journalctl", "--no-pager", "--output=short-precise", "-k",
		),
		execToPathFn(
			"kubelet-version.txt",
			"kubelet", "--version",
		),
		execToPathFn(
			"kubelet.log",
			"journalctl", "--no-pager", "--output=short-precise", "-u", "kubelet.service",
		),
		execToPathFn(
			"containerd.log",
			"journalctl", "--no-pager", "--output=short-precise", "-u", "containerd.service",
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
	}
}

func windowsK8sLogs(execToPathFn func(outputFileName string, command string, args ...string) func() error) []func() error {
	return []func() error{
		execToPathFn(
			"hyperv-operation.log",
			"Get-WinEvent", "-LogName Microsoft-Windows-Hyper-V-Compute-Operational | Select-Object -Property TimeCreated, Id, LevelDisplayName, Message | Sort-Object TimeCreated | Format-Table -Wrap -Autosize",
		),
		execToPathFn(
			"docker.log",
			"get-eventlog", "-LogName Application -Source Docker | Select-Object Index, TimeGenerated, EntryType, Message | Sort-Object Index | Format-Table -Wrap -Autosize",
		),
		execToPathFn(
			"containers.log",
			"docker", "ps -a",
		),
		execToPathFn(
			"containers-hcs.log",
			"hcsdiag", "list",
		),
		execToPathFn(
			"kubelet.log",
			`Get-ChildItem "C:\\var\\log\\kubelet\\"  | ForEach-Object { write-output "$_"  ;cat "c:\\var\\log\\kubelet\\$_" }`,
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

// collectVMBootLog collects boot logs of the vm by using azure boot diagnostics.
func collectVMBootLog(ctx context.Context, am *v1alpha4.AzureMachine, outputPath string) error {
	Logf("INFO: Collecting boot logs for AzureMachine %s\n", am.GetName())

	resourceId := strings.TrimPrefix(*am.Spec.ProviderID, azure.ProviderIDPrefix)
	resource, err := autorest.ParseResourceID(resourceId)
	if err != nil {
		return errors.Wrap(err, "failed to parse resource id")
	}

	settings, err := auth.GetSettingsFromEnvironment()
	if err != nil {
		return errors.Wrap(err, "failed to get settings from environment")
	}

	vmClient := compute.NewVirtualMachinesClient(settings.GetSubscriptionID())
	vmClient.Authorizer, err = settings.GetAuthorizer()
	if err != nil {
		return errors.Wrap(err, "failed to get authorizer")
	}

	bootDiagnostics, err := vmClient.RetrieveBootDiagnosticsData(ctx, resource.ResourceGroup, resource.ResourceName, nil)
	if err != nil {
		return errors.Wrap(err, "failed to get boot diagnostics data")
	}

	return writeBootLog(bootDiagnostics, outputPath)
}

// collectVMSSBootLog collects boot logs of the scale set by using azure boot diagnostics.
func collectVMSSBootLog(ctx context.Context, providerID string, outputPath string) error {
	resourceId := strings.TrimPrefix(providerID, azure.ProviderIDPrefix)
	v := strings.Split(resourceId, "/")
	instanceId := v[len(v)-1]
	resourceId = strings.TrimSuffix(resourceId, "/virtualMachines/"+instanceId)
	resource, err := autorest.ParseResourceID(resourceId)
	if err != nil {
		return errors.Wrap(err, "failed to parse resource id")
	}

	Logf("INFO: Collecting boot logs for VMSS instance %s of scale set %s\n", instanceId, resource.ResourceName)

	settings, err := auth.GetSettingsFromEnvironment()
	if err != nil {
		return errors.Wrap(err, "failed to get settings from environment")
	}

	vmssClient := compute.NewVirtualMachineScaleSetVMsClient(settings.GetSubscriptionID())
	vmssClient.Authorizer, err = settings.GetAuthorizer()
	if err != nil {
		return errors.Wrap(err, "failed to get authorizer")
	}

	bootDiagnostics, err := vmssClient.RetrieveBootDiagnosticsData(ctx, resource.ResourceGroup, resource.ResourceName, instanceId, nil)
	if err != nil {
		return errors.Wrap(err, "failed to get boot diagnostics data")
	}

	return writeBootLog(bootDiagnostics, outputPath)
}

func writeBootLog(bootDiagnostics compute.RetrieveBootDiagnosticsDataResult, outputPath string) error {
	var err error
	resp, err := http.Get(*bootDiagnostics.SerialConsoleLogBlobURI)
	if err != nil || resp.StatusCode != 200 {
		return errors.Wrap(err, "failed to get logs from serial console uri")
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read response body")
	}

	if err := ioutil.WriteFile(filepath.Join(outputPath, "boot.log"), content, 0644); err != nil {
		return errors.Wrap(err, "failed to write response to file")
	}

	return nil
}

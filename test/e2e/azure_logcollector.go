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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/pkg/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kinderrors "sigs.k8s.io/kind/pkg/errors"
)

// AzureLogCollector collects logs from a CAPZ workload cluster.
type AzureLogCollector struct{}

// CollectMachineLog collects logs from a machine.
func (k AzureLogCollector) CollectMachineLog(ctx context.Context,
	managementClusterClient client.Client, m *clusterv1.Machine, outputPath string) error {
	var errors []error

	if err := collectLogsFromNode(ctx, managementClusterClient, m, outputPath); err != nil {
		errors = append(errors, err)
	}

	if err := collectBootLog(ctx, m, outputPath); err != nil {
		errors = append(errors, err)
	}

	return kinderrors.NewAggregate(errors)

}

// collectLogsFromNode collects logs from various sources by ssh'ing into the node
func collectLogsFromNode(ctx context.Context, managementClusterClient client.Client, m *clusterv1.Machine, outputPath string) error {
	cluster, err := util.GetClusterFromMetadata(ctx, managementClusterClient, m.ObjectMeta)
	if err != nil {
		return err
	}
	controlPlaneEndpoint := cluster.Spec.ControlPlaneEndpoint.Host
	hostname := m.Spec.InfrastructureRef.Name
	port := e2eConfig.GetVariable(VMSSHPort)
	execToPathFn := func(outputFileName, command string, args ...string) func() error {
		return func() error {
			f, err := fileOnHost(filepath.Join(outputPath, outputFileName))
			if err != nil {
				return err
			}
			defer f.Close()
			return retryWithExponentialBackOff(func() error {
				return execOnHost(controlPlaneEndpoint, hostname, port, f, command, args...)
			})
		}
	}

	return kinderrors.AggregateConcurrent([]func() error{
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
	})
}

// collectBootLog collects boot logs of the vm by using azure boot diagnostics
func collectBootLog(ctx context.Context, m *clusterv1.Machine, outputPath string) error {
	resourceId := strings.TrimPrefix(*m.Spec.ProviderID, "azure:///")
	resource, err := azure.ParseResourceID(resourceId)
	if err != nil {
		return errors.Wrapf(err, "failed to parse resource id")
	}

	settings, err := auth.GetSettingsFromEnvironment()
	if err != nil {
		return errors.Wrapf(err, "failed to get settings from environment")
	}

	vmClient := compute.NewVirtualMachinesClient(settings.GetSubscriptionID())
	vmClient.Authorizer, err = settings.GetAuthorizer()
	if err != nil {
		return errors.Wrapf(err, "failed to get authorizer")
	}

	bootDiagnostics, err := vmClient.RetrieveBootDiagnosticsData(ctx, resource.ResourceGroup, resource.ResourceName, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to get boot diagnostics data")
	}

	resp, err := http.Get(*bootDiagnostics.SerialConsoleLogBlobURI)
	if err != nil || resp.StatusCode != 200 {
		return errors.Wrapf(err, "failed to get logs from serial console uri")
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return errors.Wrapf(err, "failed to read response body")
	}

	if err := ioutil.WriteFile(filepath.Join(outputPath, "boot.log"), content, 0644); err != nil {
		return errors.Wrapf(err, "failed to write response to file")
	}

	return nil
}

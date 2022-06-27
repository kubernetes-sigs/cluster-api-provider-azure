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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

const (
	cloudProviderAzureHelmRepoURL     = "https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-azure/master/helm/repo"
	cloudProviderAzureChartName       = "cloud-provider-azure"
	cloudProviderAzureHelmReleaseName = "cloud-provider-azure-oot"
	azureDiskCSIDriverHelmRepoURL     = "https://raw.githubusercontent.com/kubernetes-sigs/azuredisk-csi-driver/master/charts"
	azureDiskCSIDriverChartName       = "azuredisk-csi-driver"
	azureDiskCSIDriverHelmReleaseName = "azuredisk-csi-driver-oot"
)

// InstallCloudProviderAzureHelmChart installs the official cloud-provider-azure helm chart
// and validates that expected pods exist and are Ready.
func InstallCloudProviderAzureHelmChart(ctx context.Context, input clusterctl.ApplyClusterTemplateAndWaitInput) {
	specName := "cloud-provider-azure-install"
	By("Installing the correct version of cloud-provider-azure components via helm")
	values := []string{fmt.Sprintf("infra.clusterName=%s", input.ConfigCluster.ClusterName)}
	InstallHelmChart(ctx, input, cloudProviderAzureHelmRepoURL, cloudProviderAzureChartName, cloudProviderAzureHelmReleaseName, values)
	clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.ConfigCluster.Namespace, input.ConfigCluster.ClusterName)
	workloadClusterClient := clusterProxy.GetClient()
	By("Waiting for Ready cloud-controller-manager deployment pods")
	for _, d := range []string{"cloud-controller-manager"} {
		waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, d, "kube-system", specName)
		WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
	}
	By("Waiting for Ready cloud-node-manager daemonset pods")
	for _, ds := range []string{"cloud-node-manager", "cloud-node-manager-windows"} {
		WaitForDaemonset(ctx, input, workloadClusterClient, ds, "kube-system")
	}
}

// InstallAzureDiskCSIDriverHelmChart installs the official azure-disk CSI driver helm chart
func InstallAzureDiskCSIDriverHelmChart(ctx context.Context, input clusterctl.ApplyClusterTemplateAndWaitInput) {
	specName := "azuredisk-csi-drivers-install"
	By("Installing the correct version of azure-disk CSI driver components via helm")
	values := []string{}
	InstallHelmChart(ctx, input, azureDiskCSIDriverHelmRepoURL, azureDiskCSIDriverChartName, azureDiskCSIDriverHelmReleaseName, values)
	clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.ConfigCluster.Namespace, input.ConfigCluster.ClusterName)
	workloadClusterClient := clusterProxy.GetClient()
	By("Waiting for Ready csi-azuredisk-controller deployment pods")
	for _, d := range []string{"csi-azuredisk-controller"} {
		waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, d, "default", specName)
		WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
	}
	By("Waiting for Running azure-disk-csi node pods")
	for _, ds := range []string{"csi-azuredisk-node", "csi-azuredisk-node-win"} {
		WaitForDaemonset(ctx, input, workloadClusterClient, ds, "default")
	}
}

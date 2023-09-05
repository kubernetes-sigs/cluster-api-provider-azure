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

	. "github.com/onsi/ginkgo/v2"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

const (
	azureDiskCSIDriverHelmRepoURL     = "https://raw.githubusercontent.com/kubernetes-sigs/azuredisk-csi-driver/master/charts"
	azureDiskCSIDriverChartName       = "azuredisk-csi-driver"
	azureDiskCSIDriverHelmReleaseName = "azuredisk-csi-driver-oot"
)

// InstallAzureDiskCSIDriverHelmChart installs the official azure-disk CSI driver helm chart
func InstallAzureDiskCSIDriverHelmChart(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, hasWindows bool) {
	specName := "azuredisk-csi-drivers-install"
	By("Installing azure-disk CSI driver components via helm")
	options := &HelmOptions{
		Values: []string{"controller.replicas=1", "controller.runOnControlPlane=true"},
	}
	// TODO: make this always true once HostProcessContainers are on for all supported k8s versions.
	if hasWindows {
		options.Values = append(options.Values, "windows.useHostProcessContainers=true")
	}
	clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.Namespace, input.ClusterName)
	InstallHelmChart(ctx, clusterProxy, kubesystem, azureDiskCSIDriverHelmRepoURL, azureDiskCSIDriverChartName, azureDiskCSIDriverHelmReleaseName, options, "")
	By("Waiting for Ready csi-azuredisk-controller deployment pods")
	for _, d := range []string{"csi-azuredisk-controller"} {
		waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, d, kubesystem, specName)
		WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
	}
}

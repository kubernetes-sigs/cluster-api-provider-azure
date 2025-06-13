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
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

const (
	cloudProviderAzureHelmRepoURL     = "https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-azure/master/helm/repo"
	cloudProviderAzureChartName       = "cloud-provider-azure"
	cloudProviderAzureHelmReleaseName = "cloud-provider-azure-oot"
	azureDiskCSIDriverHelmRepoURL     = "https://raw.githubusercontent.com/kubernetes-sigs/azuredisk-csi-driver/master/charts"
	azureDiskCSIDriverChartName       = "azuredisk-csi-driver"
	azureDiskCSIDriverHelmReleaseName = "azuredisk-csi-driver-oot"
	azureDiskCSIDriverCAAPHLabelName  = "azuredisk-csi"
)

// EnsureCNIAndCloudProviderAzureHelmChart installs the official cloud-provider-azure helm chart
// and a CNI and validates that expected pods exist and are Ready.
func EnsureCNIAndCloudProviderAzureHelmChart(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, hasWindows bool) {
	specName := "ensure-cloud-provider-azure"
	clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.Namespace, input.ClusterName)

	By("Ensuring cloud-provider-azure is installed via CAAPH")

	// We do this before waiting for the pods to be ready because there is a co-dependency between CNI (nodes ready) and cloud-provider being initialized.
	EnsureCNI(ctx, input, hasWindows)

	By("Waiting for Ready cloud-controller-manager deployment pods")
	for _, d := range []string{"cloud-controller-manager"} {
		waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, d, kubesystem, specName)
		WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
	}
}

// EnsureAzureDiskCSIDriverHelmChart installs the official azure-disk CSI driver helm chart
func EnsureAzureDiskCSIDriverHelmChart(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput) {
	specName := "ensure-azuredisk-csi-drivers"
	clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.Namespace, input.ClusterName)
	mgmtClient := input.ClusterProxy.GetClient()

	By("Ensuring azure-disk CSI driver is installed via CAAPH")
	cluster := &clusterv1.Cluster{}
	Eventually(func(g Gomega) {
		g.Expect(mgmtClient.Get(ctx, types.NamespacedName{
			Namespace: input.Namespace,
			Name:      input.ClusterName,
		}, cluster)).To(Succeed())
		// Label the cluster so that CAAPH installs the azuredisk-csi helm chart via existing HelmChartProxy resource
		if cluster.Labels != nil {
			cluster.Labels[azureDiskCSIDriverCAAPHLabelName] = "true"
		} else {
			cluster.Labels = map[string]string{
				azureDiskCSIDriverCAAPHLabelName: "true",
			}
		}
		g.Expect(mgmtClient.Update(ctx, cluster)).To(Succeed())
	}, e2eConfig.GetIntervals(specName, "wait-deployment")...).Should(Succeed())

	By("Waiting for Ready csi-azuredisk-controller deployment pods")
	for _, d := range []string{"csi-azuredisk-controller"} {
		waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, d, kubesystem, specName)
		WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
	}
}

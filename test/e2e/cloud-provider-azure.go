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

	. "github.com/onsi/ginkgo"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

const (
	cloudProviderAzureHelmRepoURL     = "https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-azure/master/helm/repo"
	cloudProviderAzureChartName       = "cloud-provider-azure"
	cloudProviderAzureHelmReleaseName = "cloud-provider-azure-oot"
)

// InstallCloudProviderAzureHelmChart installs the official cloud-provider-azure helm chart
// and validates that expected pods exist and are Ready
func InstallCloudProviderAzureHelmChart(ctx context.Context, input clusterctl.ApplyClusterTemplateAndWaitInput) {
	By("Installing the correct version of cloud-provider-azure components via helm")
	values := []string{fmt.Sprintf("infra.clusterName=%s", input.ConfigCluster.ClusterName)}
	InstallHelmChart(ctx, input, cloudProviderAzureHelmRepoURL, cloudProviderAzureChartName, cloudProviderAzureHelmReleaseName, values)
	By("Waiting for a Running cloud-controller-manager pod")
	clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.ConfigCluster.Namespace, input.ConfigCluster.ClusterName)
	workloadClusterClient := clusterProxy.GetClient()
	By("Waiting for Ready cloud-controller-manager deployment pods")
	for _, d := range []string{"cloud-controller-manager"} {
		WaitForDeployment(ctx, input, workloadClusterClient, d, "kube-system")
	}
	By("Waiting for Ready cloud-node-manager daemonset pods")
	for _, ds := range []string{"cloud-node-manager", "cloud-node-manager-windows"} {
		WaitForDaemonset(ctx, input, workloadClusterClient, ds, "kube-system")
	}
}

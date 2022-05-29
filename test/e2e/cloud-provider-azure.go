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
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	cloudProviderAzureHelmRepoURL    = "https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-azure/master/helm/repo"
	cloudProviderAzureChartName      = "cloud-provider-azure"
	cloudProvierAzureHelmReleaseName = "cloud-provider-azure-oot"
)

// InstallCloudProviderAzureHelmChart installs the official cloud-provider-azure helm chart
// Fulfills the clusterctl.Waiter type so that it can be used as ApplyClusterTemplateAndWaitInput data
// in the flow of a clusterctl.ApplyClusterTemplateAndWait E2E test scenario
func InstallCloudProviderAzureHelmChart(ctx context.Context, input clusterctl.ApplyClusterTemplateAndWaitInput, result *clusterctl.ApplyClusterTemplateAndWaitResult) {
	By(fmt.Sprintf("Ensuring the kubeconfig secret for cluster %s/%s exists before installing cloud-provider-azure components", input.ConfigCluster.Namespace, input.ConfigCluster.ClusterName))
	WaitForWorkloadClusterKubeconfigSecret(ctx, input)
	By("Installing the correct version of cloud-provider-azure components via helm")
	values := []string{fmt.Sprintf("infra.clusterName=%s", input.ConfigCluster.ClusterName)}
	InstallHelmChart(ctx, input, cloudProviderAzureHelmRepoURL, cloudProviderAzureChartName, cloudProvierAzureHelmReleaseName, values)
	By("Waiting for a Running cloud-controller-manager pod")
	clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.ConfigCluster.Namespace, input.ConfigCluster.ClusterName)
	workloadClusterClient := clusterProxy.GetClient()
	cloudControllerManagerPodLabel, err := labels.Parse("component=cloud-controller-manager")
	Expect(err).NotTo(HaveOccurred())
	framework.WaitForPodListCondition(ctx, framework.WaitForPodListConditionInput{
		Lister: workloadClusterClient,
		ListOptions: &client.ListOptions{
			LabelSelector: cloudControllerManagerPodLabel,
			Namespace:     "kube-system",
		},
		Condition: podListHasNumPods(1),
	}, input.WaitForControlPlaneIntervals...)
	Expect(err).NotTo(HaveOccurred())
	By(fmt.Sprintf("Waiting for Ready cloud-node-manager daemonset pods"))
	for _, ds := range []string{"cloud-node-manager", "cloud-node-manager-windows"} {
		WaitForDaemonset(ctx, input, workloadClusterClient, ds, "kube-system")
	}
	By("Done installing cloud-provider-azure components, ensuring control plane is initialized")
	result.ControlPlane = framework.DiscoveryAndWaitForControlPlaneInitialized(ctx, framework.DiscoveryAndWaitForControlPlaneInitializedInput{
		Lister:  input.ClusterProxy.GetClient(),
		Cluster: result.Cluster,
	}, input.WaitForControlPlaneIntervals...)
}

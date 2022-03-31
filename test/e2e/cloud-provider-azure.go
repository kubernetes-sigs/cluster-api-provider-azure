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

	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	helmAction "helm.sh/helm/v3/pkg/action"
	helmLoader "helm.sh/helm/v3/pkg/chart/loader"
	helmCli "helm.sh/helm/v3/pkg/cli"
	helmVals "helm.sh/helm/v3/pkg/cli/values"
	helmGetter "helm.sh/helm/v3/pkg/getter"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// InstallCloudProviderAzureHelmChart installs the official cloud-provider-azure helm chart
func InstallCloudProviderAzureHelmChart(ctx context.Context, input clusterctl.ApplyClusterTemplateAndWaitInput, result *clusterctl.ApplyClusterTemplateAndWaitResult) {
	By("Waiting for workload cluster kubeconfig secret")
	Eventually(func() error {
		client := input.ClusterProxy.GetClient()
		secret := &corev1.Secret{}
		key := crclient.ObjectKey{
			Name:      fmt.Sprintf("%s-kubeconfig", input.ConfigCluster.ClusterName),
			Namespace: input.ConfigCluster.Namespace,
		}
		return client.Get(ctx, key, secret)
	}, input.WaitForControlPlaneIntervals...).Should(Succeed())
	clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.ConfigCluster.Namespace, input.ConfigCluster.ClusterName)
	By("Waiting for nodes to come online indicating that the cluster is ready to accept work")
	Eventually(func() error {
		clientSet := clusterProxy.GetClientSet()
		var runningNodes int
		list, err := clientSet.CoreV1().Nodes().List(ctx, v1.ListOptions{})
		if err != nil {
			return err
		}
		for _, n := range list.Items {
			if n.Status.Phase == corev1.NodeRunning {
				runningNodes++
			}
		}
		if runningNodes > 0 {
			return nil
		}
		return err
	}, input.WaitForControlPlaneIntervals...).Should(Succeed())
	By("Installing the correct version of cloud-provider-azure components via helm")
	kubeConfigPath := clusterProxy.GetKubeconfigPath()
	clusterName := input.ClusterProxy.GetName()
	settings := helmCli.New()
	settings.KubeConfig = kubeConfigPath
	actionConfig := new(helmAction.Configuration)
	err := actionConfig.Init(settings.RESTClientGetter(), "default", "secret", Logf)
	Expect(err).To(BeNil())
	i := helmAction.NewInstall(actionConfig)
	i.RepoURL = "https://raw.githubusercontent.com/kubernetes-sigs/cloud-provider-azure/master/helm/repo"
	i.ReleaseName = "cloud-provider-azure-oot"
	Eventually(func() error {
		cp, err := i.ChartPathOptions.LocateChart("cloud-provider-azure", helmCli.New())
		if err != nil {
			return err
		}
		p := helmGetter.All(settings)
		valueOpts := &helmVals.Options{}
		valueOpts.Values = []string{fmt.Sprintf("infra.clusterName=%s", clusterName)}
		vals, err := valueOpts.MergeValues(p)
		if err != nil {
			return err
		}
		chartRequested, err := helmLoader.Load(cp)
		if err != nil {
			return err
		}
		release, err := i.RunWithContext(ctx, chartRequested, vals)
		if err != nil {
			return err
		}
		Logf(release.Info.Description)
		return nil
	}, input.WaitForControlPlaneIntervals...).Should(Succeed())
	By("Waiting for a Running cloud-controller-manager pod")
	Eventually(func() bool {
		clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.ConfigCluster.Namespace, input.ConfigCluster.ClusterName)
		clientSet := clusterProxy.GetClientSet()
		var runningPods int
		list, err := clientSet.CoreV1().Pods("kube-system").List(ctx, v1.ListOptions{
			LabelSelector: "component=cloud-controller-manager",
		})
		if err != nil {
			return false
		}
		for _, p := range list.Items {
			if p.Status.Phase == corev1.PodRunning {
				runningPods++
			}
		}
		return runningPods > 0
	}, input.WaitForControlPlaneIntervals...).Should(BeTrue())
	By("Waiting for Running cloud-node-manager pods")
	Eventually(func() bool {
		clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.ConfigCluster.Namespace, input.ConfigCluster.ClusterName)
		clientSet := clusterProxy.GetClientSet()
		var runningPods int64
		list, err := clientSet.CoreV1().Pods("kube-system").List(ctx, v1.ListOptions{
			LabelSelector: "k8s-app=cloud-node-manager",
		})
		if err != nil {
			return false
		}
		for _, p := range list.Items {
			if p.Status.Phase == corev1.PodRunning {
				runningPods++
			}
		}
		return runningPods >= to.Int64(input.ConfigCluster.ControlPlaneMachineCount)
	}, input.WaitForControlPlaneIntervals...).Should(BeTrue())
	By("Done installing cloud-provider-azure components, ensuring control plane is initialized")
	result.ControlPlane = framework.DiscoveryAndWaitForControlPlaneInitialized(ctx, framework.DiscoveryAndWaitForControlPlaneInitializedInput{
		Lister:  input.ClusterProxy.GetClient(),
		Cluster: result.Cluster,
	}, input.WaitForControlPlaneIntervals...)
}

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

	. "github.com/onsi/ginkgo"
	kubeadmv1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

const (
	calicoChartFilepath = "../../helm/calico-capz"
	calicoChartName     = "calico-capz"
	calicoReleaseName   = "calico-cni"
)

// InstallCalicoHelmChart installs calico CNI components
func InstallCalicoHelmChart(ctx context.Context, input clusterctl.ApplyClusterTemplateAndWaitInput, kubeadmControlPlane *kubeadmv1.KubeadmControlPlane) {
	values := []string{}
	isIPv6 := isIPv6Cluster(kubeadmControlPlane.Spec.KubeadmConfigSpec.ClusterConfiguration.ControllerManager.ExtraArgs["cluster-cidr"])
	if isDualStackCluster(kubeadmControlPlane.Spec.KubeadmConfigSpec.ClusterConfiguration.ControllerManager.ExtraArgs["cluster-cidr"]) {
		By("Configuring calico CNI helm chart for dual-stack configuration")
		values = []string{"dualstack=true"}
	} else if isIPv6 {
		By("Configuring calico CNI helm chart for ipv6 configuration")
		values = []string{"ipv6=true"}
	}
	By("Installing calico CNI via helm")
	InstallHelmChart(ctx, input, "", calicoChartFilepath, calicoReleaseName, values)
	clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.ConfigCluster.Namespace, input.ConfigCluster.ClusterName)
	workloadClusterClient := clusterProxy.GetClient()
	By("Waiting for Ready calico-node daemonset pods")
	for _, ds := range []string{"calico-node", "calico-node-windows"} {
		WaitForDaemonset(ctx, input, workloadClusterClient, ds, "kube-system")
	}
	By("Waiting for Ready calico-kube-controllers deployment pods")
	WaitForDeployment(ctx, input, workloadClusterClient, "calico-kube-controllers", "kube-system")
	if isIPv6 {
		By("Waiting for Ready calico-typha deployment pods")
		WaitForDeployment(ctx, input, workloadClusterClient, "calico-typha", "kube-system")
	}
}

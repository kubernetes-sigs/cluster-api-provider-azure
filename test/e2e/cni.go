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
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	helmVals "helm.sh/helm/v3/pkg/cli/values"
	k8snet "k8s.io/utils/net"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

const (
	calicoHelmChartRepoURL   string = "https://docs.tigera.io/calico/charts"
	calicoOperatorNamespace  string = "tigera-operator"
	CalicoSystemNamespace    string = "calico-system"
	CalicoAPIServerNamespace string = "calico-apiserver"
	calicoHelmReleaseName    string = "projectcalico"
	calicoHelmChartName      string = "tigera-operator"
	kubeadmConfigMapName     string = "kubeadm-config"
)

// InstallCalicoHelmChart installs the official calico helm chart
// and validates that expected pods exist and are Ready.
func InstallCalicoHelmChart(ctx context.Context, input clusterctl.ApplyClusterTemplateAndWaitInput, cidrBlocks []string, hasWindows bool) {
	specName := "calico-install"

	By("Installing Calico CNI via helm")
	values := getCalicoValues(cidrBlocks)
	InstallHelmChart(ctx, input, calicoOperatorNamespace, calicoHelmChartRepoURL, calicoHelmChartName, calicoHelmReleaseName, values)
	clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.ConfigCluster.Namespace, input.ConfigCluster.ClusterName)
	workloadClusterClient := clusterProxy.GetClient()

	// Copy the kubeadm configmap to the calico-system namespace. This is a workaround needed for the calico-node-windows daemonset to be able to run in the calico-system namespace.
	CopyConfigMap(ctx, workloadClusterClient, kubeadmConfigMapName, kubesystem, CalicoSystemNamespace)

	By("Waiting for Ready tigera-operator deployment pods")
	for _, d := range []string{"tigera-operator"} {
		waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, d, calicoOperatorNamespace, specName)
		WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
	}

	// Add FeatureOverride for ChecksumOffloadBroken in FelixConfiguration.
	// This is the recommended workaround for https://github.com/projectcalico/calico/issues/3145.
	felixYaml, err := os.ReadFile(filepath.Join(e2eConfig.GetVariable(AddonsPath), "calico", "felix-override.yaml"))
	Expect(err).NotTo(HaveOccurred())
	Eventually(func() error {
		return clusterProxy.Apply(ctx, felixYaml)
	}, 10*time.Second).Should(Succeed(), "Failed to apply the felix configurations patch")

	By("Waiting for Ready calico-system deployment pods")
	for _, d := range []string{"calico-kube-controllers", "calico-typha"} {
		waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, d, CalicoSystemNamespace, specName)
		WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
	}
	By("Waiting for Ready calico-apiserver deployment pods")
	for _, d := range []string{"calico-apiserver"} {
		waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, d, CalicoAPIServerNamespace, specName)
		WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
	}
	By("Waiting for Ready calico-node daemonset pods")
	for _, ds := range []string{"calico-node"} {
		waitInput := GetWaitForDaemonsetAvailableInput(ctx, clusterProxy, ds, CalicoSystemNamespace, specName)
		WaitForDaemonset(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-daemonset")...)
	}
	// TODO: enable this for all clusters once calico for windows is part of the helm chart.
	if hasWindows {
		By("Waiting for Ready calico windows pods")
		for _, ds := range []string{"calico-node-windows"} {
			waitInput := GetWaitForDaemonsetAvailableInput(ctx, clusterProxy, ds, CalicoSystemNamespace, specName)
			WaitForDaemonset(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-daemonset")...)
		}

		By("Waiting for Ready calico windows pods")
		for _, ds := range []string{"kube-proxy-windows"} {
			waitInput := GetWaitForDaemonsetAvailableInput(ctx, clusterProxy, ds, kubesystem, specName)
			WaitForDaemonset(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-daemonset")...)
		}
	}
}

func getCalicoValues(cidrBlocks []string) *helmVals.Options {
	var ipv6CidrBlock, ipv4CidrBlock string
	var values *helmVals.Options
	for _, cidr := range cidrBlocks {
		if k8snet.IsIPv6CIDRString(cidr) {
			ipv6CidrBlock = cidr
		} else {
			Expect(k8snet.IsIPv4CIDRString(cidr)).To(BeTrue(), "CIDR %s is not a valid IPv4 or IPv6 CIDR", cidr)
			ipv4CidrBlock = cidr
		}
	}
	addonsPath := e2eConfig.GetVariable(AddonsPath)
	switch {
	case ipv6CidrBlock != "" && ipv4CidrBlock != "":
		By("Configuring calico CNI helm chart for dual-stack configuration")
		values = &helmVals.Options{
			StringValues: []string{fmt.Sprintf("installation.calicoNetwork.ipPools[0].cidr=%s", ipv4CidrBlock), fmt.Sprintf("installation.calicoNetwork.ipPools[1].cidr=%s", ipv6CidrBlock)},
			ValueFiles:   []string{filepath.Join(addonsPath, "calico-dual-stack", "values.yaml")},
		}
	case ipv6CidrBlock != "":
		By("Configuring calico CNI helm chart for IPv6 configuration")
		values = &helmVals.Options{
			StringValues: []string{fmt.Sprintf("installation.calicoNetwork.ipPools[0].cidr=%s", ipv6CidrBlock)},
			ValueFiles:   []string{filepath.Join(addonsPath, "calico-ipv6", "values.yaml")},
		}
	default:
		By("Configuring calico CNI helm chart for IPv4 configuration")
		values = &helmVals.Options{
			StringValues: []string{fmt.Sprintf("installation.calicoNetwork.ipPools[0].cidr=%s", ipv4CidrBlock)},
			ValueFiles:   []string{filepath.Join(addonsPath, "calico", "values.yaml")},
		}
	}
	return values
}

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
	AzureCNIv1               string = "azure-cni-v1"
)

// EnsureCNI installs the CNI plugin depending on the input.CNIManifestPath
func EnsureCNI(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, installHelmChart bool, cidrBlocks []string, hasWindows bool) {
	if input.CNIManifestPath != "" {
		InstallCNIManifest(ctx, input, cidrBlocks, hasWindows)
	} else {
		EnsureCalicoIsReady(ctx, input, installHelmChart, cidrBlocks, hasWindows)
	}
}

// InstallCNIManifest installs the CNI manifest provided by the user
func InstallCNIManifest(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, cidrBlocks []string, hasWindows bool) {
	By("Installing a CNI plugin to the workload cluster")
	workloadCluster := input.ClusterProxy.GetWorkloadCluster(ctx, input.Namespace, input.ClusterName)

	cniYaml, err := os.ReadFile(input.CNIManifestPath)
	Expect(err).ShouldNot(HaveOccurred())

	Expect(workloadCluster.Apply(ctx, cniYaml)).To(Succeed())
}

// EnsureCalicoIsReady copies the kubeadm configmap to the calico-system namespace and waits for the calico pods to be ready.
func EnsureCalicoIsReady(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, installHelmChart bool, cidrBlocks []string, hasWindows bool) {
	specName := "ensure-calico"

	clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.Namespace, input.ClusterName)
	if installHelmChart {
		By("Installing Calico CNI via Helm Chart")
		values := getCalicoValues(cidrBlocks)
		InstallHelmChart(ctx, clusterProxy, calicoOperatorNamespace, calicoHelmChartRepoURL, calicoHelmChartName, calicoHelmReleaseName, values, os.Getenv(CalicoVersion))
	} else {
		By("Ensuring Calico CNI is installed via CAAPH")
	}

	By("Copying kubeadm config map to calico-system namespace")
	workloadClusterClient := clusterProxy.GetClient()

	// Copy the kubeadm configmap to the calico-system namespace. This is a workaround needed for the calico-node-windows daemonset to be able to run in the calico-system namespace.
	CopyConfigMap(ctx, input, workloadClusterClient, kubeadmConfigMapName, kubesystem, CalicoSystemNamespace)

	By("Waiting for Ready tigera-operator deployment pods")
	for _, d := range []string{"tigera-operator"} {
		waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, d, calicoOperatorNamespace, specName)
		WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
	}

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
}

func getCalicoValues(cidrBlocks []string) *HelmOptions {
	var ipv6CidrBlock, ipv4CidrBlock string
	var values *HelmOptions
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
		values = &HelmOptions{
			StringValues: []string{fmt.Sprintf("installation.calicoNetwork.ipPools[0].cidr=%s", ipv4CidrBlock), fmt.Sprintf("installation.calicoNetwork.ipPools[1].cidr=%s", ipv6CidrBlock)},
			ValueFiles:   []string{filepath.Join(addonsPath, "calico-dual-stack", "values.yaml")},
		}
	case ipv6CidrBlock != "":
		By("Configuring calico CNI helm chart for IPv6 configuration")
		values = &HelmOptions{
			StringValues: []string{fmt.Sprintf("installation.calicoNetwork.ipPools[0].cidr=%s", ipv6CidrBlock)},
			ValueFiles:   []string{filepath.Join(addonsPath, "calico-ipv6", "values.yaml")},
		}
	default:
		By("Configuring calico CNI helm chart for IPv4 configuration")
		values = &HelmOptions{
			StringValues: []string{fmt.Sprintf("installation.calicoNetwork.ipPools[0].cidr=%s", ipv4CidrBlock)},
			ValueFiles:   []string{filepath.Join(addonsPath, "calico", "values.yaml")},
		}
	}
	return values
}

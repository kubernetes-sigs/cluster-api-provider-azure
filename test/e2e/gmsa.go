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
	"path/filepath"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/2020-09-01/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/services/keyvault/v7.0/keyvault"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	capz "sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func configureGmsa(ctx context.Context, workloadProxy, bootstrapClusterProxy framework.ClusterProxy, namespace, clusterName string, config *clusterctl.E2EConfig) {
	settings, err := auth.GetSettingsFromEnvironment()
	authorizer, err := settings.GetAuthorizer()
	Expect(err).NotTo(HaveOccurred())
	subId := settings.GetSubscriptionID()

	Expect(err).NotTo(HaveOccurred())
	keyVaultClient := keyvault.New()

	vmClient := compute.NewVirtualMachinesClient(subId)
	vmClient.Authorizer = authorizer

	networkClient := network.NewVirtualNetworkPeeringsClient(subId)
	networkClient.Authorizer = authorizer

	//override to use keyvault management endpoint
	settings.Values[auth.Resource] = fmt.Sprintf("%s%s", "https://", azure.PublicCloud.KeyVaultDNSSuffix)
	keyvaultAuthorizer, err := settings.GetAuthorizer()
	keyVaultClient.Authorizer = keyvaultAuthorizer

	// Wait for the Cluster nodes to be ready (this is different than capi's ready as cni needs to finish initializing)
	windowsCalico := &appsv1.DaemonSet{
		ObjectMeta: v1.ObjectMeta{Name: "calico-node-windows", Namespace: "kube-system"},
	}
	WaitForDaemonSetAvailable(ctx, WaitForDeamonSetAvailableInput{
		Getter:    daemonSetClientAdapter{client: workloadProxy.GetClientSet().AppsV1().DaemonSets("kube-system")},
		Deamonset: windowsCalico,
		Clientset: workloadProxy.GetClientSet(),
	}, "20m", "10s")

	// Wait for the Domain to finish provisioning.  The existence of the spec file is the marker
	gmsaSpecName := "gmsa-cred-spec-gmsa-e2e-" + config.GetVariable("GMSA_ID")
	fmt.Fprintf(GinkgoWriter, "INFO: Getting the gmsa gmsaSpecFile %s from %s\n", gmsaSpecName, config.GetVariable("GMSA_KEYVAULT_URL"))
	var gmsaSpecFile keyvault.SecretBundle
	Eventually(func() error {
		gmsaSpecFile, err = keyVaultClient.GetSecret(ctx, config.GetVariable("GMSA_KEYVAULT_URL"), gmsaSpecName, "")
		if capz.ResourceNotFound(err) {
			fmt.Fprintf(GinkgoWriter, "INFO: Waiting for gmsaSpecFile %s to be created by Domain controller\n", config.GetVariable("GMSA_KEYVAULT_URL"))
			return err
		}

		if err != nil {
			fmt.Fprintf(GinkgoWriter, "INFO: error when retrieving gmsaSpecFile %s\n", err)
			return err
		}
		return nil
	}, 10*time.Second, 15*time.Minute).Should(Succeed())
	Expect(gmsaSpecFile.Value).ToNot(BeNil())

	workloadCluster, err := util.GetClusterByName(ctx, bootstrapClusterProxy.GetClient(), namespace, clusterName)
	Expect(err).NotTo(HaveOccurred())
	clusterHostName := workloadCluster.Spec.ControlPlaneEndpoint.Host

	gmsaNode, windowsNodes := labelGmsaTestNode(ctx, workloadProxy)
	dropGmsaSpecOnTestNode(gmsaNode, clusterHostName, gmsaSpecFile)
	configureCoreDNS(ctx, workloadProxy, config)

	for _, n := range windowsNodes.Items {
		hostname := getHostName(&n)
		// until https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/2182
		updateWorkerNodeDNS(config, clusterHostName, hostname)
	}

	fmt.Fprintf(GinkgoWriter, "INFO: GMSA configuration complete\n")
}

func updateWorkerNodeDNS(config *clusterctl.E2EConfig, clusterHostName string, workerNodeHostName string) {
	fmt.Fprintf(GinkgoWriter, "INFO: Update node vm dns to %s\n", config.GetVariable("GMSA_DNS_IP"))
	dnsCmd := fmt.Sprintf("$currentDNS = (Get-DnsClientServerAddress -AddressFamily ipv4); Set-DnsClientServerAddress -InterfaceIndex $currentDNS[0].InterfaceIndex -ServerAddresses %s, $currentDNS[0].Address", config.GetVariable("GMSA_DNS_IP"))
	f, err := fileOnHost(filepath.Join("", "gmsa-spec-writer-output.txt"))
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()
	err = execOnHost(clusterHostName, workerNodeHostName, "22", f, dnsCmd)
	Expect(err).NotTo(HaveOccurred())
}

func configureCoreDNS(ctx context.Context, workloadProxy framework.ClusterProxy, config *clusterctl.E2EConfig) {
	fmt.Fprintf(GinkgoWriter, "INFO: Update coredns with domain ip %s\n", config.GetVariable("GMSA_DNS_IP"))

	corednsConfigMap := &corev1.ConfigMap{}
	key := client.ObjectKey{
		Namespace: "kube-system",
		Name:      "coredns",
	}
	err := workloadProxy.GetClient().Get(ctx, key, corednsConfigMap)
	Expect(err).NotTo(HaveOccurred())

	corefile, ok := corednsConfigMap.Data["Corefile"]
	Expect(ok).Should(BeTrue())

	gmsaDns := fmt.Sprintf(`k8sgmsa.lan:53 {
	errors
	cache 30
	log
	forward . %s
}`, config.GetVariable("GMSA_DNS_IP"))
	corefile = corefile + gmsaDns

	corednsConfigMap.Data["Corefile"] = corefile
	err = workloadProxy.GetClient().Update(ctx, corednsConfigMap)
	Expect(err).NotTo(HaveOccurred())

	//rollout restart to refresh the configuration
	patch := []byte(`{"spec": {"template":{ "metadata": { "annotations": { "restartedBy": "gmsa" } } } } }`)
	_, err = workloadProxy.GetClientSet().AppsV1().Deployments("kube-system").Patch(ctx, "coredns", types.MergePatchType, patch, v1.PatchOptions{})
	Expect(err).NotTo(HaveOccurred())
}

func dropGmsaSpecOnTestNode(gmsaNode *corev1.Node, clusterHostName string, secret keyvault.SecretBundle) {
	fmt.Fprintf(GinkgoWriter, "INFO: Writing gmsa spec to disk\n")
	f, err := fileOnHost(filepath.Join("", "gmsa-spec-writer-output.txt"))
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()
	hostname := getHostName(gmsaNode)

	cmd := fmt.Sprintf("mkdir -force /gmsa; rm -force c:/gmsa/gmsa-cred-spec-gmsa-e2e.yml; $input='%s'; [System.Text.Encoding]::Unicode.GetString([System.Convert]::FromBase64String($input)) >> c:/gmsa/gmsa-cred-spec-gmsa-e2e.yml", *secret.Value)
	err = execOnHost(clusterHostName, hostname, "22", f, cmd)
	Expect(err).NotTo(HaveOccurred())
}

func labelGmsaTestNode(ctx context.Context, workloadProxy framework.ClusterProxy) (*corev1.Node, *corev1.NodeList) {
	windowsNodeOptions := v1.ListOptions{
		LabelSelector: "kubernetes.io/os=windows",
	}

	var gmsaNode *corev1.Node = nil
	var windowsNodes *corev1.NodeList
	var err error
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		windowsNodes, err = workloadProxy.GetClientSet().CoreV1().Nodes().List(ctx, windowsNodeOptions)
		if err != nil {
			return err
		}

		Expect(len(windowsNodes.Items)).Should(BeNumerically(">", 0))
		gmsaNode = &windowsNodes.Items[0]
		gmsaNode.Labels["agentpool"] = "windowsgmsa"
		fmt.Fprintf(GinkgoWriter, "INFO: Labeling node %s as 'windowsgmsa'\n", gmsaNode.Name)
		_, err = workloadProxy.GetClientSet().CoreV1().Nodes().Update(ctx, gmsaNode, v1.UpdateOptions{})
		return err
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(gmsaNode).NotTo(BeNil())
	return gmsaNode, windowsNodes
}

func getHostName(gmsaNode *corev1.Node) string {
	hostname := ""
	for _, address := range gmsaNode.Status.Addresses {
		if address.Type == corev1.NodeHostName {
			hostname = address.Address
		}
	}
	return hostname
}

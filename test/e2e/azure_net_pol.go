// +build e2e

/*
Copyright 2020 The Kubernetes Authors.

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
	"io/ioutil"
	"math/rand"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	deploymentBuilder "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/deployment"
	e2e_namespace "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/namespace"
	e2e_networkpolicy "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/networkpolicy"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cluster-api/test/framework"
)

const (
	PolicyDir = "workloads/policies"
)

// AzureLBSpecInput is the input for AzureLBSpec.
type AzureNetPolSpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	SkipCleanup           bool
}

// AzureNetPolSpec implements a test that verifies that the Network Policies Deployed in the managed cluster works.
func AzureNetPolSpec(ctx context.Context, inputGetter func() AzureNetPolSpecInput) {
	var (
		specName     = "azure-netpol"
		input        AzureNetPolSpecInput
		clusterProxy framework.ClusterProxy
		clientset    *kubernetes.Clientset
		config       *rest.Config
	)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	By("creating a Kubernetes client to the workload cluster")
	clusterProxy = input.BootstrapClusterProxy.GetWorkloadCluster(context.TODO(), input.Namespace.Name, input.ClusterName)
	Expect(clusterProxy).NotTo(BeNil())
	clientset = clusterProxy.GetClientSet()
	Expect(clientset).NotTo(BeNil())
	testTmpDir, err := ioutil.TempDir("/tmp", "azure-test")
	defer os.RemoveAll(testTmpDir)
	Expect(err).NotTo(HaveOccurred())
	config = createRestConfig(testTmpDir, input.Namespace.Name, input.ClusterName)
	Expect(config).NotTo(BeNil())

	nsDev, nsProd := "development", "production"
	By("Creating development namespace")
	namespaceDev, err := e2e_namespace.CreateNamespaceDeleteIfExist(clientset, nsDev, map[string]string{"purpose": "development"})
	Expect(err).NotTo(HaveOccurred())

	By("Creating production namespace")
	namespaceProd, err := e2e_namespace.CreateNamespaceDeleteIfExist(clientset, nsProd, map[string]string{"purpose": "production"})
	Expect(err).NotTo(HaveOccurred())

	By("Creating frontendProd, backend and network-policy pod deployments")
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randInt := r.Intn(99999)

	// Front end
	frontendLabels := map[string]string{"app": "webapp", "role": "frontend"}

	// Front end Prod
	frontendProdDeploymentName := fmt.Sprintf("frontend-prod-%v", randInt)
	frontEndProd := deploymentBuilder.CreateDeployment("library/nginx:latest", frontendProdDeploymentName, namespaceProd.GetName())
	frontEndProd.AddLabels(frontendLabels)
	frontendProdDeployment, err := frontEndProd.Deploy(clientset)
	Expect(err).NotTo(HaveOccurred())

	// Front end Dev
	frontendDevDeploymentName := fmt.Sprintf("frontend-dev-%v", randInt+100000)
	frontEndDev := deploymentBuilder.CreateDeployment("library/nginx:latest", frontendDevDeploymentName, namespaceDev.GetName())
	frontEndDev.AddLabels(frontendLabels)
	frontendDevDeployment, err := frontEndDev.Deploy(clientset)
	Expect(err).NotTo(HaveOccurred())

	// Backend
	backendDeploymentName := fmt.Sprintf("backend-%v", randInt+200000)
	backendLabels := map[string]string{"app": "webapp", "role": "backend"}
	backendDev := deploymentBuilder.CreateDeployment("library/nginx:latest", backendDeploymentName, namespaceDev.GetName())
	backendDev.AddLabels(backendLabels)
	backendDeployment, err := backendDev.Deploy(clientset)
	Expect(err).NotTo(HaveOccurred())

	// Network policy
	nwpolicyDeploymentName := fmt.Sprintf("network-policy-%v", randInt+300000)
	nwpolicyLabels := map[string]string{"app": "webapp", "role": "any"}
	nwpolicy := deploymentBuilder.CreateDeployment("library/nginx:latest", nwpolicyDeploymentName, namespaceDev.GetName())
	nwpolicy.AddLabels(nwpolicyLabels)
	nwpolicyDeployment, err := nwpolicy.Deploy(clientset)
	Expect(err).NotTo(HaveOccurred())

	By("Ensure there is a running frontend-prod pod")
	frontendProdDeploymentInput := framework.WaitForDeploymentsAvailableInput{
		Getter:     deploymentsClientAdapter{client: clientset.AppsV1().Deployments(namespaceProd.GetName())},
		Deployment: frontendProdDeployment,
	}
	framework.WaitForDeploymentsAvailable(context.TODO(), frontendProdDeploymentInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)

	By("Ensure there is a running frontend-dev pod")
	frontendDevDeploymentInput := framework.WaitForDeploymentsAvailableInput{
		Getter:     deploymentsClientAdapter{client: clientset.AppsV1().Deployments(namespaceDev.GetName())},
		Deployment: frontendDevDeployment,
	}
	framework.WaitForDeploymentsAvailable(context.TODO(), frontendDevDeploymentInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)

	By("Ensure there is a running backend pod")
	backendDeploymentInput := framework.WaitForDeploymentsAvailableInput{
		Getter:     deploymentsClientAdapter{client: clientset.AppsV1().Deployments(namespaceDev.GetName())},
		Deployment: backendDeployment,
	}
	framework.WaitForDeploymentsAvailable(context.TODO(), backendDeploymentInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)

	By("Ensure there is a running network-policy pod")
	nwpolicyDeploymentInput := framework.WaitForDeploymentsAvailableInput{
		Getter:     deploymentsClientAdapter{client: clientset.AppsV1().Deployments(namespaceDev.GetName())},
		Deployment: nwpolicyDeployment,
	}
	framework.WaitForDeploymentsAvailable(context.TODO(), nwpolicyDeploymentInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)

	By("Ensuring we have outbound internet access from the frontend-prod pods")
	frontendProdPods, err := frontEndProd.GetPodsFromDeployment(clientset)
	Expect(err).NotTo(HaveOccurred())
	e2e_networkpolicy.EnsureOutboundInternetAccess(clientset, config, frontendProdPods)

	By("Ensuring we have outbound internet access from the frontend-dev pods")
	frontendDevPods, err := frontEndDev.GetPodsFromDeployment(clientset)
	Expect(err).NotTo(HaveOccurred())
	e2e_networkpolicy.EnsureOutboundInternetAccess(clientset, config, frontendDevPods)

	By("Ensuring we have outbound internet access from the backend pods")
	backendPods, err := backendDev.GetPodsFromDeployment(clientset)
	Expect(err).NotTo(HaveOccurred())
	e2e_networkpolicy.EnsureOutboundInternetAccess(clientset, config, backendPods)

	By("Ensuring we have outbound internet access from the network-policy pods")
	nwpolicyPods, err := nwpolicy.GetPodsFromDeployment(clientset)
	Expect(err).NotTo(HaveOccurred())
	e2e_networkpolicy.EnsureOutboundInternetAccess(clientset, config, nwpolicyPods)

	By("Ensuring we have connectivity from network-policy pods to frontend-prod pods")
	e2e_networkpolicy.EnsureConnectivityResultBetweenPods(clientset, config, nwpolicyPods, frontendProdPods, true)

	By("Ensuring we have connectivity from network-policy pods to backend pods")
	e2e_networkpolicy.EnsureConnectivityResultBetweenPods(clientset, config, nwpolicyPods, backendPods, true)

	By("Applying a network policy to deny ingress access to app: webapp, role: backend pods in development namespace")
	nwpolicyName, namespaceE2E, nwpolicyFileName := "backend-deny-ingress", nsDev, "backend-policy-deny-ingress.yaml"
	e2e_networkpolicy.ApplyNetworkPolicy(clientset, nwpolicyName, namespaceE2E, nwpolicyFileName, PolicyDir)

	By("Ensuring we no longer have ingress access from the network-policy pods to backend pods")
	e2e_networkpolicy.EnsureConnectivityResultBetweenPods(clientset, config, nwpolicyPods, backendPods, false)

	By("Cleaning up after ourselves")
	e2e_networkpolicy.DeleteNetworkPolicy(clientset, nwpolicyName, namespaceE2E)

	By("Applying a network policy to deny egress access in development namespace")
	nwpolicyName, namespaceE2E, nwpolicyFileName = "backend-deny-egress", nsDev, "backend-policy-deny-egress.yaml"
	e2e_networkpolicy.ApplyNetworkPolicy(clientset, nwpolicyName, nsDev, nwpolicyFileName, PolicyDir)

	By("Ensuring we no longer have egress access from the network-policy pods to backend pods")
	e2e_networkpolicy.EnsureConnectivityResultBetweenPods(clientset, config, nwpolicyPods, backendPods, false)
	e2e_networkpolicy.EnsureConnectivityResultBetweenPods(clientset, config, frontendDevPods, backendPods, false)

	By("Cleaning up after ourselves")
	e2e_networkpolicy.DeleteNetworkPolicy(clientset, nwpolicyName, namespaceE2E)

	By("Applying a network policy to allow egress access to app: webapp, role: frontend pods in any namespace from pods with app: webapp, role: backend labels in development namespace")
	nwpolicyName, namespaceE2E, nwpolicyFileName = "backend-allow-egress-pod-label", nsDev, "backend-policy-allow-egress-pod-label.yaml"
	e2e_networkpolicy.ApplyNetworkPolicy(clientset, nwpolicyName, namespaceE2E, nwpolicyFileName, PolicyDir)

	By("Ensuring we have egress access from pods with matching labels")
	e2e_networkpolicy.EnsureConnectivityResultBetweenPods(clientset, config, backendPods, frontendDevPods, true)
	e2e_networkpolicy.EnsureConnectivityResultBetweenPods(clientset, config, backendPods, frontendProdPods, true)

	By("Ensuring we don't have ingress access from pods without matching labels")
	e2e_networkpolicy.EnsureConnectivityResultBetweenPods(clientset, config, backendPods, nwpolicyPods, false)

	By("Cleaning up after ourselves")
	e2e_networkpolicy.DeleteNetworkPolicy(clientset, nwpolicyName, namespaceE2E)

	By("Applying a network policy to allow egress access to app: webapp, role: frontend pods from pods with app: webapp, role: backend labels in same development namespace")
	nwpolicyName, namespaceE2E, nwpolicyFileName = "backend-allow-egress-pod-namespace-label", nsDev, "backend-policy-allow-egress-pod-namespace-label.yaml"
	e2e_networkpolicy.ApplyNetworkPolicy(clientset, nwpolicyName, namespaceE2E, nwpolicyFileName, PolicyDir)

	By("Ensuring we have egress access from pods with matching labels")
	e2e_networkpolicy.EnsureConnectivityResultBetweenPods(clientset, config, backendPods, frontendDevPods, true)

	By("Ensuring we don't have ingress access from pods without matching labels")
	e2e_networkpolicy.EnsureConnectivityResultBetweenPods(clientset, config, backendPods, frontendProdPods, false)
	e2e_networkpolicy.EnsureConnectivityResultBetweenPods(clientset, config, backendPods, nwpolicyPods, false)

	By("Cleaning up after ourselves")
	e2e_networkpolicy.DeleteNetworkPolicy(clientset, nwpolicyName, namespaceE2E)

	By("Applying a network policy to only allow ingress access to app: webapp, role: backend pods in development namespace from pods in any namespace with the same labels")
	nwpolicyName, namespaceE2E, nwpolicyFileName = "backend-allow-ingress-pod-label", nsDev, "backend-policy-allow-ingress-pod-label.yaml"
	e2e_networkpolicy.ApplyNetworkPolicy(clientset, nwpolicyName, namespaceE2E, nwpolicyFileName, PolicyDir)

	By("Ensuring we have ingress access from pods with matching labels")
	e2e_networkpolicy.EnsureConnectivityResultBetweenPods(clientset, config, backendPods, backendPods, true)

	By("Ensuring we don't have ingress access from pods without matching labels")
	e2e_networkpolicy.EnsureConnectivityResultBetweenPods(clientset, config, nwpolicyPods, backendPods, false)

	By("Cleaning up after ourselves")
	e2e_networkpolicy.DeleteNetworkPolicy(clientset, nwpolicyName, namespaceE2E)

	By("Applying a network policy to only allow ingress access to app: webapp role:backends in development namespace from pods with label app:webapp, role: frontendProd within namespace with label purpose: development")
	nwpolicyName, namespaceE2E, nwpolicyFileName = "backend-policy-allow-ingress-pod-namespace-label", nsDev, "backend-policy-allow-ingress-pod-namespace-label.yaml"
	e2e_networkpolicy.ApplyNetworkPolicy(clientset, nwpolicyName, namespaceE2E, nwpolicyFileName, PolicyDir)

	By("Ensuring we don't have ingress access from role:frontend pods in production namespace")
	e2e_networkpolicy.EnsureConnectivityResultBetweenPods(clientset, config, frontendProdPods, backendPods, false)

	By("Ensuring we have ingress access from role:frontend pods in development namespace")
	e2e_networkpolicy.EnsureConnectivityResultBetweenPods(clientset, config, frontendDevPods, backendPods, true)

}

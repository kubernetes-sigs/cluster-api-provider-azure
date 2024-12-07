//go:build e2e
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
	"bytes"
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/utils/ptr"
	"os"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"strings"
)

// AzureAPIServerILBSpecInput is the input for AzureAPIServerILBSpec.
type AzureAPIServerILBSpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Cluster               *clusterv1.Cluster
	Namespace             *corev1.Namespace
	ClusterName           string
	ExpectedWorkerNodes   int32
	WaitIntervals         []interface{}
}

// AzureAPIServerILBSpec implements a test that verifies the Azure API server ILB is created.
func AzureAPIServerILBSpec(ctx context.Context, inputGetter func() AzureAPIServerILBSpecInput) {
	var (
		specName = "azure-apiserver-ilb"
		input    AzureAPIServerILBSpecInput
	)

	input = inputGetter()
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	Expect(input.ClusterName).NotTo(BeEmpty(), "Invalid argument. input.ClusterName can't be empty when calling %s spec", specName)

	By("1. Fetching new Azure Credentials")
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	Expect(err).NotTo(HaveOccurred())

	By("2. Getting azureLoadBalancerClient")
	azureLoadBalancerClient, err := armnetwork.NewLoadBalancersClient(getSubscriptionID(Default), cred, nil)
	Expect(err).NotTo(HaveOccurred())

	By("3. Verifying the Azure API Server Internal Load Balancer is created")
	groupName := os.Getenv(AzureResourceGroup)
	internalLoadbalancerName := fmt.Sprintf("%s-%s", input.ClusterName, "apiserver-ilb-public-lb-internal")

	backoff := wait.Backoff{
		Duration: retryBackoffInitialDuration, // TODO: retryBackoffInitialDuration is not readable. Update it to a more readable value.
		Factor:   retryBackoffFactor,
		Jitter:   retryBackoffJitter,
		Steps:    retryBackoffSteps,
	}
	retryFn := func(ctx context.Context) (bool, error) {
		defer GinkgoRecover()
		resp, err := azureLoadBalancerClient.Get(ctx, groupName, internalLoadbalancerName, nil)
		if err != nil {
			return false, err
		}

		internalLoadbalancer := resp.LoadBalancer
		Expect(ptr.Deref(internalLoadbalancer.Name, "g")).To(Equal(internalLoadbalancerName))

		switch ptr.Deref(internalLoadbalancer.Properties.ProvisioningState, "") {
		case armnetwork.ProvisioningStateSucceeded:
			return true, nil
		case armnetwork.ProvisioningStateUpdating:
			// Wait for operation to complete.
			return false, nil
		default:
			defer ctx.Done() // TODO: close the context if the function returns an error. Is this right?
			return false, fmt.Errorf("azure internal loadbalancer provisioning failed with state: %q", ptr.Deref(internalLoadbalancer.Properties.ProvisioningState, "(nil)"))
		}
	}
	err = wait.ExponentialBackoffWithContext(ctx, backoff, retryFn)

	By("4. Creating a dynamic client for accessing custom resources in the management cluster")
	mgmtRestConfig := input.BootstrapClusterProxy.GetRESTConfig()
	mgmtDynamicClientSet, err := dynamic.NewForConfig(mgmtRestConfig)
	Expect(err).NotTo(HaveOccurred())
	Expect(mgmtDynamicClientSet).NotTo(BeNil())

	By("5. Getting the AzureCluster using the dynamic client set")
	azureClusterGVR := schema.GroupVersionResource{
		Group:    "infrastructure.cluster.x-k8s.io",
		Version:  "v1beta1",
		Resource: "azureclusters",
	}

	azureCluster, err := mgmtDynamicClientSet.Resource(azureClusterGVR).
		Namespace(input.Namespace.Name).
		Get(ctx, input.ClusterName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	deployedAzureCluster := &infrav1.AzureCluster{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(
		azureCluster.UnstructuredContent(),
		deployedAzureCluster,
	)
	By("6. Getting the controlplane endpoint name")
	controlPlaneEndpointName, apiServerILBPrivateIP := "", ""
	for _, frontendIP := range deployedAzureCluster.Spec.NetworkSpec.APIServerLB.FrontendIPs {
		if frontendIP.PublicIP != nil && frontendIP.PublicIP.DNSName != "" {
			controlPlaneEndpointName = frontendIP.PublicIP.DNSName
		} else if frontendIP.PrivateIPAddress != "" {
			apiServerILBPrivateIP = frontendIP.PrivateIPAddress
		}
	}
	Expect(controlPlaneEndpointName).NotTo(BeEmpty(), "controlPlaneEndpointName should be found at AzureCluster.Spec.NetworkSpec.APIServerLB.FrontendIPs with a valid DNS name")
	// ${CLUSTER_NAME}-${APISERVER_LB_DNS_SUFFIX}.${AZURE_LOCATION}.cloudapp.azure.com
	Expect(controlPlaneEndpointName).To(Equal(fmt.Sprintf("%s-%s.%s.cloudapp.azure.com", input.ClusterName, os.Getenv("APISERVER_LB_DNS_SUFFIX"), os.Getenv("AZURE_LOCATION"))))
	Expect(apiServerILBPrivateIP).NotTo(BeEmpty(), "apiServerILBPrivateIP should be found at AzureCluster.Spec.NetworkSpec.APIServerLB.FrontendIPs when apiserver ilb feature flag is enabled")

	// By("Creating a K8s client for the management cluster")
	// mgmtClient := input.BootstrapClusterProxy.GetClient()
	// Expect(mgmtClient).NotTo(BeNil())
	//
	// By("Getting the AzureCluster") // TODO: switch to a RESTClient instead of using the mgmtClient
	// deployedAzureCluster := &infrav1.AzureCluster{}
	// err = mgmtClient.Get(ctx, types.NamespacedName{
	// 	Name:      input.ClusterName,
	// 	Namespace: input.Namespace.Name,
	// }, deployedAzureCluster)
	// Expect(err).NotTo(HaveOccurred())
	//
	// By("Getting the controlplane endpoint name")
	// controlPlaneEndpointName, apiServerILBPrivateIP := "", ""
	// for _, frontendIP := range deployedAzureCluster.Spec.NetworkSpec.APIServerLB.FrontendIPs {
	// 	if frontendIP.PublicIP != nil && frontendIP.PublicIP.DNSName != "" {
	// 		controlPlaneEndpointName = frontendIP.PublicIP.DNSName
	// 	} else if frontendIP.PrivateIPAddress != "" {
	// 		apiServerILBPrivateIP = frontendIP.PrivateIPAddress
	// 	}
	// }
	// Expect(controlPlaneEndpointName).NotTo(BeEmpty(), "controlPlaneEndpointName should be found at AzureCluster.Spec.NetworkSpec.APIServerLB.FrontendIPs with a valid DNS name")
	// Expect(apiServerILBPrivateIP).NotTo(BeEmpty(), "apiServerILBPrivateIP should be found at AzureCluster.Spec.NetworkSpec.APIServerLB.FrontendIPs when apiserver ilb feature flag is enabled")

	By("7. Creating a Kubernetes client set to the workload cluster")
	workloadClusterProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
	Expect(workloadClusterProxy).NotTo(BeNil())
	workloadClusterClientSet := workloadClusterProxy.GetClientSet()
	Expect(workloadClusterClientSet).NotTo(BeNil())

	// Deploy node-debug daemonset to workload cluster
	By("7.1 Deploying node-debug daemonset to the workload cluster")
	nodeDebugDS := &v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-debug",
			Namespace: "kube-system",
		},
		Spec: v1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "node-debug",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "node-debug",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "node-debug",
							Image: "busybox:1.35",
							SecurityContext: &corev1.SecurityContext{
								Privileged: ptr.To(true),
							},
							Command: []string{
								"sh",
								"-c",
								"sleep infinity",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "etc-hosts",
									MountPath: "/host/etc",
									ReadOnly:  true,
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "etc-hosts",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/etc/hosts",
									Type: ptr.To(corev1.HostPathFile),
								},
							},
						},
					},
				},
			},
		},
	}
	nodeDebugDS, err = workloadClusterClientSet.AppsV1().DaemonSets("kube-system").Create(ctx, nodeDebugDS, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	By("8. Probing worker nodes")
	Eventually(func(g Gomega) {
		allNodes, err := workloadClusterClientSet.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		g.Expect(err).NotTo(HaveOccurred())

		By("8.1 Saving all the nodes")
		workerNodes := make(map[string]corev1.Node, 0)
		for i, node := range allNodes.Items {
			if strings.Contains(node.Name, input.ClusterName+"-md-0") {
				workerNodes[node.Name] = allNodes.Items[i]
			}
		}
		g.Expect(len(workerNodes)).To(Equal(int(input.ExpectedWorkerNodes)), "Expected number of worker nodes not found")

		By("8.2 Saving all the worker nodes")
		allNodeDebugPods, err := workloadClusterClientSet.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
			LabelSelector: "app=node-debug",
		})
		g.Expect(err).NotTo(HaveOccurred())

		By("8.3 Saving all the node-debug daemonset pods running on the worker nodes")
		workerDSPods := make(map[string]corev1.Pod, 0)
		for _, daemonsetPod := range allNodeDebugPods.Items {
			if _, ok := workerNodes[daemonsetPod.Spec.NodeName]; ok {
				workerDSPods[daemonsetPod.Name] = daemonsetPod
			}
		}
		Expect(len(workerDSPods)).To(Equal(int(input.ExpectedWorkerNodes)), "Expected number of worker node-debug daemonset pods not found")

		By("8.4 Checking the /etc/hosts file in each of the worker nodes")
		// get the kubeconfig for the workload cluster
		workloadClusterKubeConfigPath := workloadClusterProxy.GetKubeconfigPath()
		workloadClusterKubeConfig, err := clientcmd.BuildConfigFromFlags("", workloadClusterKubeConfigPath)
		g.Expect(err).NotTo(HaveOccurred())

		for _, pod := range workerDSPods {
			By("8.5.1 Exec into the node-debug pod to check the /etc/hosts file")
			catEtcHostsCommand := "cat /host/etc/hosts" // /etc/host is mounted as /host/etc/hosts in the node-debug pod
			req := workloadClusterClientSet.CoreV1().RESTClient().Post().
				Resource("pods").
				Name(pod.Name).
				Namespace(pod.Namespace).
				SubResource("exec").
				Param("container", pod.Spec.Containers[0].Name).
				Param("stdin", "true").
				Param("stdout", "true").
				Param("tty", "true").
				Param("command", catEtcHostsCommand)

			// create the executor
			executor, err := remotecommand.NewSPDYExecutor(workloadClusterKubeConfig, "POST", req.URL())
			g.Expect(err).NotTo(HaveOccurred())

			// cat the /etc/hosts file
			var stdout, stderr bytes.Buffer
			err = executor.Stream(remotecommand.StreamOptions{
				Stdin:  nil,
				Stdout: &stdout,
				Stderr: &stderr,
				Tty:    false,
			})
			g.Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed to exec into pod: %s, stderr: %s", pod.Name, stderr.String()))

			output := stdout.String()
			fmt.Printf("Captured output:\n%s\n", output)
			Expect(output).To(ContainSubstring(apiServerILBPrivateIP), "Expected the /etc/hosts file to contain the updated DNS entry for the Internal LB for the API Server")

			// TODO: run netcat command to check if the DNS entry is resolvable
		}
	}, input.WaitIntervals...).Should(Succeed())
}

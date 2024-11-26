//go:build e2e
// +build e2e

/*
Copyright 2024 The Kubernetes Authors.

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
/*
                                  +--------------------------------+
                                  |           Start Test           |
                                  +--------------------------------+
                                             |
                                             v
+-------------------------------------------+--------------------------------------------+
| Fetch Azure Credentials                   | Get Azure Load Balancer Client             |
+-------------------------------------------+--------------------------------------------+
                                             |
                                             v
+-------------------------------------------+--------------------------------------------+
| Verify Azure Internal Load Balancer       | - Check Load Balancer Name                 |
| - Verify Provisioning State               | - Confirm Succeeded State                  |
+-------------------------------------------+--------------------------------------------+
                                             |
                                             v
+-------------------------------------------+--------------------------------------------+
| Create Dynamic Client for Management      | Get Azure Cluster Resource                 |
| Cluster                                   | - Extract Control Plane Endpoint           |
|                                           | - Extract API Server ILB Private IP         |
+-------------------------------------------+--------------------------------------------+
                                             |
                                             v
+-------------------------------------------+--------------------------------------------+
| Create Kubernetes Client Set for          | Create Workload Cluster Proxy              |
| Workload Cluster                          |                                            |
+-------------------------------------------+--------------------------------------------+
                                             |
                                             v
+-------------------------------------------+--------------------------------------------+
| Deploy Node-Debug DaemonSet               | - Add to Default Namespace                 |
| - Mount Host /etc/hosts                   | - Configure Privileged Container           |
+-------------------------------------------+--------------------------------------------+
                                             |
                                             v
+-------------------------------------------+--------------------------------------------+
| List and Verify Worker Nodes              | - Ensure Expected Number of Nodes          |
| - Identify Worker Nodes                   | - Confirm Node Names Match Cluster         |
+-------------------------------------------+--------------------------------------------+
                                             |
                                             v
+-------------------------------------------+--------------------------------------------+
| List Node-Debug Pods                      | - Verify Pods on Worker Nodes              |
| - Check Pod Status                        | - Ensure Running Phase                     |
+-------------------------------------------+--------------------------------------------+
                                             |
                                             v
+-------------------------------------------+--------------------------------------------+
| Execute Test Commands on Each Pod         | 1. Verify "Hello from node-debug pod"      |
| - Use Remote Exec                         | 2. Check /host/etc contents                |
| - Stream stdout/stderr                    | 3. Validate Against Expected Outputs       |
+-------------------------------------------+--------------------------------------------+
                                             |
                                             v
+-------------------------------------------+--------------------------------------------+
| Validate Test Results                     | - Check All Pods Pass Tests                |
| - Retry if Any Pod Fails                  | - Ensure Consistent Results                |
+-------------------------------------------+--------------------------------------------+
                                             |
                                             v
                                  +--------------------------------+
                                  |        Test Complete           |
                                  +--------------------------------+
*/

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

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
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

// AzureAPIServerILBSpecInput is the input for AzureAPIServerILBSpec.
type AzureAPIServerILBSpecInput struct {
	BootstrapClusterProxy                   framework.ClusterProxy
	Cluster                                 *clusterv1.Cluster
	Namespace                               *corev1.Namespace
	ClusterName                             string
	ExpectedWorkerNodes                     int32
	WaitIntervals                           []interface{}
	TemplateHasPrivateIPCustomDNSResolution bool
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

	By("Fetching new Azure Credentials")
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	Expect(err).NotTo(HaveOccurred())

	By("Getting azureLoadBalancerClient")
	azureLoadBalancerClient, err := armnetwork.NewLoadBalancersClient(getSubscriptionID(Default), cred, nil)
	Expect(err).NotTo(HaveOccurred())

	By("Verifying the Azure API Server Internal Load Balancer is created")
	groupName := os.Getenv(AzureResourceGroup)
	Logf("Azure Resource Group: %s\n", groupName)
	internalLoadbalancerName := fmt.Sprintf("%s-%s", input.ClusterName, "public-lb-internal")

	backoff := wait.Backoff{
		Duration: 300 * time.Second,
		Factor:   0.5,
		Jitter:   0.5,
		Steps:    7,
	}
	retryFn := func(ctx context.Context) (bool, error) {
		defer GinkgoRecover()
		resp, err := azureLoadBalancerClient.Get(ctx, groupName, internalLoadbalancerName, nil)
		if err != nil {
			return false, err
		}

		By("Verifying the Azure API Server Internal Load Balancer is the right one created")
		internalLoadbalancer := resp.LoadBalancer
		Expect(ptr.Deref(internalLoadbalancer.Name, "")).To(Equal(internalLoadbalancerName))

		By("Verifying the Azure API Server Internal Load Balancer is in a succeeded state")
		switch ptr.Deref(internalLoadbalancer.Properties.ProvisioningState, "") {
		case armnetwork.ProvisioningStateSucceeded:
			return true, nil
		case armnetwork.ProvisioningStateUpdating:
			// Wait for operation to complete.
			return false, nil
		default:
			return false, fmt.Errorf("azure internal loadbalancer provisioning failed with state: %q", ptr.Deref(internalLoadbalancer.Properties.ProvisioningState, "(nil)"))
		}
	}
	err = wait.ExponentialBackoffWithContext(ctx, backoff, retryFn)
	Expect(err).NotTo(HaveOccurred())

	// ------------------------ //
	By("Creating a dynamic client for accessing custom resources in the management cluster")
	mgmtRestConfig := input.BootstrapClusterProxy.GetRESTConfig()
	mgmtDynamicClientSet, err := dynamic.NewForConfig(mgmtRestConfig)
	Expect(err).NotTo(HaveOccurred())
	Expect(mgmtDynamicClientSet).NotTo(BeNil())

	By("Getting the AzureCluster using the dynamic client set")
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
	Expect(err).NotTo(HaveOccurred())

	By("Getting the controlplane endpoint name")
	controlPlaneEndpointDNSName, apiServerILBPrivateIP := "", ""
	for _, frontendIP := range deployedAzureCluster.Spec.NetworkSpec.APIServerLB.FrontendIPs {
		if frontendIP.PublicIP != nil && frontendIP.PublicIP.DNSName != "" {
			Logf("Control Plane Endpoint Name: %s\n", frontendIP.PublicIP.DNSName)
			controlPlaneEndpointDNSName = frontendIP.PublicIP.DNSName
		} else if frontendIP.PrivateIPAddress != "" {
			Logf("API Server ILB Private IP: %s\n", frontendIP.PrivateIPAddress)
			apiServerILBPrivateIP = frontendIP.PrivateIPAddress
		}
	}

	// skip checking the controlPlaneEndpointDNSName in case of default template
	// TODO: remove this block when the underlying infra spec is also updated with private IP of the internal load balancer
	Expect(controlPlaneEndpointDNSName).NotTo(BeEmpty(), "controlPlaneEndpointDNSName should be found at AzureCluster.Spec.NetworkSpec.APIServerLB.FrontendIPs with a valid DNS name")
	if input.TemplateHasPrivateIPCustomDNSResolution {
		Expect(controlPlaneEndpointDNSName).To(Equal(fmt.Sprintf("%s-%s.%s.cloudapp.azure.com", input.ClusterName, os.Getenv("APISERVER_LB_DNS_SUFFIX"), os.Getenv("AZURE_LOCATION"))))
	}
	Expect(apiServerILBPrivateIP).NotTo(BeEmpty(), "apiServerILBPrivateIP should be found at AzureCluster.Spec.NetworkSpec.APIServerLB.FrontendIPs when apiserver ilb feature flag is enabled")
	// ------------------------ //

	By("Creating a Kubernetes client set to the workload cluster")
	workloadClusterProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
	Expect(workloadClusterProxy).NotTo(BeNil())
	workloadClusterClient := workloadClusterProxy.GetClient()
	Expect(workloadClusterClient).NotTo(BeNil())
	workloadClusterClientSet := workloadClusterProxy.GetClientSet()
	Expect(workloadClusterClientSet).NotTo(BeNil())

	// Deploy node-debug daemonset to workload cluster
	By("Deploying node-debug daemonset to the workload cluster")
	nodeDebugDS := &v1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-debug",
			Namespace: "default",
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
							Image: "docker.io/library/busybox:latest",
							SecurityContext: &corev1.SecurityContext{
								Privileged: ptr.To(true),
							},
							Command: []string{
								"sh",
								"-c",
								"tail -f /dev/null",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "etc-hosts",
									MountPath: "/host/etc",
									ReadOnly:  true,
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"ls"},
									},
								},
								InitialDelaySeconds: 0,
								PeriodSeconds:       1,
								TimeoutSeconds:      60,
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
	err = workloadClusterClient.Create(ctx, nodeDebugDS)
	Expect(err).NotTo(HaveOccurred())

	backoff = wait.Backoff{
		Duration: 100 * time.Second,
		Factor:   0.5,
		Jitter:   0.5,
		Steps:    5,
	}
	retryDSFn := func(ctx context.Context) (bool, error) {
		defer GinkgoRecover()

		By("Saving all the nodes")
		allNodes := &corev1.NodeList{}
		err = workloadClusterClient.List(ctx, allNodes)
		if err != nil {
			return false, fmt.Errorf("failed to list nodes in the workload cluster: %v", err)
		}

		if len(allNodes.Items) == 0 {
			return false, fmt.Errorf("no nodes found in the workload cluster")
		}

		By("Saving all the worker nodes")
		workerNodes := make(map[string]corev1.Node, 0)
		for i, node := range allNodes.Items {
			if strings.Contains(node.Name, input.ClusterName+"-md-0") {
				workerNodes[node.Name] = allNodes.Items[i]
			}
		}
		if len(workerNodes) != int(input.ExpectedWorkerNodes) {
			return false, fmt.Errorf("expected number of worker nodes: %d, got: %d", input.ExpectedWorkerNodes, len(workerNodes))
		}

		By("Saving all the node-debug pods running on the worker nodes")
		allNodeDebugPods, err := workloadClusterClientSet.CoreV1().Pods("default").List(ctx, metav1.ListOptions{
			LabelSelector: "app=node-debug",
		})
		if err != nil {
			return false, fmt.Errorf("failed to list node-debug pods in the workload cluster: %v", err)
		}

		workerDSPods := make(map[string]corev1.Pod, 0)
		workerDSPodsTestResult := make(map[string]bool, 0)
		for _, daemonsetPod := range allNodeDebugPods.Items {
			if _, ok := workerNodes[daemonsetPod.Spec.NodeName]; ok {
				workerDSPods[daemonsetPod.Name] = daemonsetPod
				workerDSPodsTestResult[daemonsetPod.Name] = false
			}
		}
		if len(workerDSPods) != int(input.ExpectedWorkerNodes) {
			return false, fmt.Errorf("expected number of worker node-debug daemonset pods: %d, got: %d", input.ExpectedWorkerNodes, len(workerDSPods))
		}

		By("Getting the kubeconfig path for the workload cluster")
		workloadClusterKubeConfigPath := workloadClusterProxy.GetKubeconfigPath()
		workloadClusterKubeConfig, err := clientcmd.BuildConfigFromFlags("", workloadClusterKubeConfigPath)

		if err != nil {
			return false, fmt.Errorf("failed to build workload cluster kubeconfig from flags: %v", err)
		}

		Logf("Number of node debug pods deployed on worker nodes: %v\n", len(workerDSPods))
		for _, nodeDebugPod := range workerDSPods {
			Logf("node-debug pod %v is deployed on node %v\n", nodeDebugPod.Name, nodeDebugPod.Spec.NodeName)

			By("Checking the status of the node-debug pod")
			switch nodeDebugPod.Status.Phase {
			case corev1.PodPending:
				Logf("Pod %s is in Pending phase. Retrying\n", nodeDebugPod.Name)
				return false /* retry */, nil
			case corev1.PodRunning:
				Logf("Pod %s is in Running phase. Proceeding\n", nodeDebugPod.Name)
			default:
				return false, fmt.Errorf("node-debug pod %s is in an unexpected phase: %v", nodeDebugPod.Name, nodeDebugPod.Status.Phase)
			}

			helloFromTheNodeDebugPod := "Hello from node-debug pod"
			listOfCommands := map[string][]string{
				helloFromTheNodeDebugPod: {"sh", "-c", "echo \"Hello from node-debug pod\""},
				apiServerILBPrivateIP:    {"sh", "-c", "test -f /host/etc && cat /host/etc || echo 'File not found'"}, // /etc/host is mounted as /host/etc/hosts in the node-debug pod
			}
			testResult := map[string]bool{
				helloFromTheNodeDebugPod: false,
				apiServerILBPrivateIP:    false,
			}
			for expectedCmdOutput, execCommand := range listOfCommands {
				Logf("Trying to exec into the pod %s at namespace %s and running the command %s\n", nodeDebugPod.Name, nodeDebugPod.Namespace, strings.Join(execCommand, " "))
				execRequest := workloadClusterClientSet.CoreV1().RESTClient().Post().Resource("pods").Name(nodeDebugPod.Name).
					Namespace(nodeDebugPod.Namespace).
					SubResource("exec")

				option := &corev1.PodExecOptions{
					Command: execCommand,
					Stdin:   false,
					Stdout:  true,
					Stderr:  true,
					TTY:     false,
				}

				execRequest.VersionedParams(
					option,
					scheme.ParameterCodec,
				)

				Logf("Creating executor for the pod %s using the URL %v\n", nodeDebugPod.Name, execRequest.URL())
				exec, err := remotecommand.NewSPDYExecutor(workloadClusterKubeConfig, "POST", execRequest.URL())
				if err != nil {
					return false, fmt.Errorf("failed to create executor: %v", err)
				}

				By("Streaming stdout/err from the daemonset")
				var stdout, stderr bytes.Buffer
				err = exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
					Stdin:  nil,
					Stdout: &stdout,
					Stderr: &stderr,
					Tty:    false,
				})
				if err != nil {
					return false, fmt.Errorf("failed to stream stdout/err from the daemonset: %v", err)
				}
				output := stdout.String()
				Logf("Captured output:\n%s\n", output)

				if strings.Contains(output, expectedCmdOutput) {
					testResult[expectedCmdOutput] = true
				}

				// TODO: remove this below block when the underlying infra spec is also updated with
				// private IP of the internal load balancer
				// For now, a default template will not have the private IP of the internal load balancer in its /etc/hosts
				// So, we will skip the test for default templates.
				if !input.TemplateHasPrivateIPCustomDNSResolution {
					testResult[expectedCmdOutput] = true
				}
			}

			if testResult[apiServerILBPrivateIP] && testResult[helloFromTheNodeDebugPod] {
				Logf("Tests passed for the pod %s\n", nodeDebugPod.Name)
				workerDSPodsTestResult[nodeDebugPod.Name] = true
			} else {
				Logf("Tests did not pass for the pod %s\n", nodeDebugPod.Name)
				Logf("Tests update: %v, %v\n", testResult[helloFromTheNodeDebugPod], testResult[apiServerILBPrivateIP])
				return false /* retry */, nil
			}
		}

		checkTestOutputForAllWorkerPods := true
		for podName, testResult := range workerDSPodsTestResult {
			Logf("Test result for pod %s: %v\n", podName, testResult)
			checkTestOutputForAllWorkerPods = checkTestOutputForAllWorkerPods && testResult
		}

		if checkTestOutputForAllWorkerPods {
			return true, nil
		}
		return false /* retry */, nil
	}
	err = wait.ExponentialBackoffWithContext(ctx, backoff, retryDSFn)
	Expect(err).NotTo(HaveOccurred())
}

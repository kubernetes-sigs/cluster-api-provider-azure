//go:build e2e
// +build e2e

/*
Copyright 2026 The Kubernetes Authors.

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
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cluster-api/test/framework"
)

const (
	kubeRayOperatorHelmRepoURL     = "https://ray-project.github.io/kuberay-helm/"
	kubeRayOperatorHelmChartName   = "kuberay-operator"
	kubeRayOperatorHelmReleaseName = "kuberay-operator"
	kubeRayOperatorNamespace       = "default"
	kubeRayVersion                 = "1.3.0"
	rayImage                       = "rayproject/ray:2.41.0"
)

var rayClusterGVR = schema.GroupVersionResource{
	Group:    "ray.io",
	Version:  "v1",
	Resource: "rayclusters",
}

var rayJobGVR = schema.GroupVersionResource{
	Group:    "ray.io",
	Version:  "v1",
	Resource: "rayjobs",
}

// KubeRayClusterSpecInput is the input for KubeRayClusterSpec.
type KubeRayClusterSpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	SkipCleanup           bool
}

// KubeRayClusterSpec implements a test that verifies the KubeRay operator can be installed
// on a workload cluster and a RayCluster can be created and become ready.
// This corresponds to the "Test RayCluster and GCS E2E" case from the KubeRay buildkite CI.
func KubeRayClusterSpec(ctx context.Context, inputGetter func() KubeRayClusterSpecInput) {
	var (
		specName = "kuberay-cluster"
		input    KubeRayClusterSpecInput
	)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	Expect(input.ClusterName).NotTo(BeEmpty(), "Invalid argument. input.ClusterName can't be empty when calling %s spec", specName)

	By("creating a Kubernetes client to the workload cluster")
	clusterProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
	Expect(clusterProxy).NotTo(BeNil())
	clientset := clusterProxy.GetClientSet()
	Expect(clientset).NotTo(BeNil())

	By("installing the KubeRay operator via Helm")
	InstallKubeRayOperator(ctx, clusterProxy, specName)

	By("creating a RayCluster")
	dynamicClient := newDynamicClient(clusterProxy)
	rayCluster := newRayClusterUnstructured("raycluster-e2e", corev1.NamespaceDefault)
	_, err := dynamicClient.Resource(rayClusterGVR).Namespace(corev1.NamespaceDefault).Create(ctx, rayCluster, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	By("waiting for the RayCluster to become ready")
	Eventually(func() bool {
		rc, err := dynamicClient.Resource(rayClusterGVR).Namespace(corev1.NamespaceDefault).Get(ctx, "raycluster-e2e", metav1.GetOptions{})
		if err != nil {
			return false
		}
		state, found, err := unstructured.NestedString(rc.Object, "status", "state")
		if err != nil || !found {
			return false
		}
		return state == "ready"
	}, e2eConfig.GetIntervals(specName, "wait-raycluster-ready")...).Should(BeTrue(), func() string {
		return describeKubeRayOperatorLogs(ctx, clientset)
	})

	By("verifying the head pod is running")
	Eventually(func() bool {
		pods, err := clientset.CoreV1().Pods(corev1.NamespaceDefault).List(ctx, metav1.ListOptions{
			LabelSelector: "ray.io/node-type=head",
		})
		if err != nil || len(pods.Items) == 0 {
			return false
		}
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning {
				return true
			}
		}
		return false
	}, e2eConfig.GetIntervals(specName, "wait-deployment")...).Should(BeTrue(), "head pod did not reach Running state")

	By("verifying the worker pod is running")
	Eventually(func() bool {
		pods, err := clientset.CoreV1().Pods(corev1.NamespaceDefault).List(ctx, metav1.ListOptions{
			LabelSelector: "ray.io/node-type=worker",
		})
		if err != nil || len(pods.Items) == 0 {
			return false
		}
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning {
				return true
			}
		}
		return false
	}, e2eConfig.GetIntervals(specName, "wait-deployment")...).Should(BeTrue(), "worker pod did not reach Running state")

	if !input.SkipCleanup {
		By("deleting the RayCluster")
		err = dynamicClient.Resource(rayClusterGVR).Namespace(corev1.NamespaceDefault).Delete(ctx, "raycluster-e2e", metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	}
}

// KubeRayJobSpecInput is the input for KubeRayJobSpec.
type KubeRayJobSpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	SkipCleanup           bool
}

// KubeRayJobSpec implements a test that verifies the KubeRay operator can be installed
// on a workload cluster and a RayJob can be created and completed successfully.
// This corresponds to the "Test RayJob E2E" case from the KubeRay buildkite CI.
func KubeRayJobSpec(ctx context.Context, inputGetter func() KubeRayJobSpecInput) {
	var (
		specName = "kuberay-job"
		input    KubeRayJobSpecInput
	)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	Expect(input.ClusterName).NotTo(BeEmpty(), "Invalid argument. input.ClusterName can't be empty when calling %s spec", specName)

	By("creating a Kubernetes client to the workload cluster")
	clusterProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
	Expect(clusterProxy).NotTo(BeNil())
	clientset := clusterProxy.GetClientSet()
	Expect(clientset).NotTo(BeNil())

	By("installing the KubeRay operator via Helm")
	InstallKubeRayOperator(ctx, clusterProxy, specName)

	By("creating a RayJob")
	dynamicClient := newDynamicClient(clusterProxy)
	rayJob := newRayJobUnstructured("rayjob-e2e", corev1.NamespaceDefault)
	_, err := dynamicClient.Resource(rayJobGVR).Namespace(corev1.NamespaceDefault).Create(ctx, rayJob, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	By("waiting for the RayJob to complete successfully")
	Eventually(func() bool {
		rj, err := dynamicClient.Resource(rayJobGVR).Namespace(corev1.NamespaceDefault).Get(ctx, "rayjob-e2e", metav1.GetOptions{})
		if err != nil {
			return false
		}
		deploymentStatus, found, err := unstructured.NestedString(rj.Object, "status", "jobDeploymentStatus")
		if err != nil || !found {
			return false
		}
		return deploymentStatus == "Complete"
	}, e2eConfig.GetIntervals(specName, "wait-rayjob-complete")...).Should(BeTrue(), func() string {
		return describeRayJobStatus(ctx, dynamicClient, "rayjob-e2e", corev1.NamespaceDefault, clientset)
	})

	By("verifying the RayJob completed with SUCCEEDED status")
	rj, err := dynamicClient.Resource(rayJobGVR).Namespace(corev1.NamespaceDefault).Get(ctx, "rayjob-e2e", metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	jobStatus, _, _ := unstructured.NestedString(rj.Object, "status", "jobStatus")
	Expect(jobStatus).To(Equal("SUCCEEDED"), "expected RayJob status to be SUCCEEDED but got %s", jobStatus)

	if !input.SkipCleanup {
		By("deleting the RayJob")
		err = dynamicClient.Resource(rayJobGVR).Namespace(corev1.NamespaceDefault).Delete(ctx, "rayjob-e2e", metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	}
}

// InstallKubeRayOperator installs the KubeRay operator Helm chart onto the workload cluster
// and waits for the operator deployment to become available.
func InstallKubeRayOperator(ctx context.Context, clusterProxy framework.ClusterProxy, specName string) {
	By("Adding the KubeRay Helm repo and installing the operator")
	clientset := clusterProxy.GetClientSet()
	Expect(clientset).NotTo(BeNil())

	// Install via Helm using the clusterProxy's kubeconfig
	InstallHelmChart(ctx, clusterProxy, kubeRayOperatorNamespace, kubeRayOperatorHelmRepoURL, "kuberay", kubeRayOperatorHelmChartName, kubeRayOperatorHelmReleaseName)

	By("waiting for the KubeRay operator deployment to become available")
	waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, kubeRayOperatorHelmReleaseName, kubeRayOperatorNamespace, specName)
	WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
}

// InstallHelmChart installs a Helm chart from a repo URL onto a cluster via the given ClusterProxy.
// It uses the workload cluster kubeconfig to run helm commands directly from the test runner.
func InstallHelmChart(ctx context.Context, clusterProxy framework.ClusterProxy, namespace, repoURL, repoName, chartName, releaseName string) {
	kubeconfigPath := clusterProxy.GetKubeconfigPath()
	By(fmt.Sprintf("Installing Helm chart %s/%s as release %s using kubeconfig %s", repoName, chartName, releaseName, kubeconfigPath))

	// Add the Helm repo
	repoAddCmd := exec.CommandContext(ctx, "helm", "repo", "add", repoName, repoURL)
	repoAddCmd.Env = append(repoAddCmd.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err := repoAddCmd.CombinedOutput()
	Logf("helm repo add output: %s", string(output))
	Expect(err).NotTo(HaveOccurred(), "failed to add Helm repo: %s", string(output))

	// Update the Helm repos
	repoUpdateCmd := exec.CommandContext(ctx, "helm", "repo", "update")
	repoUpdateCmd.Env = append(repoUpdateCmd.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err = repoUpdateCmd.CombinedOutput()
	Logf("helm repo update output: %s", string(output))
	Expect(err).NotTo(HaveOccurred(), "failed to update Helm repos: %s", string(output))

	// Install the chart
	installCmd := exec.CommandContext(ctx, "helm", "install", releaseName, //nolint:gosec
		fmt.Sprintf("%s/%s", repoName, chartName),
		"--namespace", namespace,
		"--create-namespace",
		"--wait",
		"--timeout", "5m0s",
	)
	installCmd.Env = append(installCmd.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err = installCmd.CombinedOutput()
	Logf("helm install output: %s", string(output))
	Expect(err).NotTo(HaveOccurred(), "failed to install Helm chart: %s", string(output))
}

// newDynamicClient creates a dynamic Kubernetes client from a ClusterProxy.
func newDynamicClient(clusterProxy framework.ClusterProxy) dynamic.Interface {
	config := clusterProxy.GetRESTConfig()
	dynamicClient, err := dynamic.NewForConfig(config)
	Expect(err).NotTo(HaveOccurred())
	return dynamicClient
}

// newRayClusterUnstructured creates an unstructured RayCluster object with a head node and one worker.
func newRayClusterUnstructured(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "ray.io/v1",
			"kind":       "RayCluster",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"rayVersion": "2.41.0",
				"headGroupSpec": map[string]interface{}{
					"rayStartParams": map[string]interface{}{
						"dashboard-host": "0.0.0.0",
					},
					"template": map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []interface{}{
								map[string]interface{}{
									"name":  "ray-head",
									"image": rayImage,
									"ports": []interface{}{
										map[string]interface{}{
											"containerPort": int64(6379),
											"name":          "gcs-server",
										},
										map[string]interface{}{
											"containerPort": int64(8265),
											"name":          "dashboard",
										},
										map[string]interface{}{
											"containerPort": int64(10001),
											"name":          "client",
										},
									},
									"resources": map[string]interface{}{
										"requests": map[string]interface{}{
											"cpu":    "300m",
											"memory": "1Gi",
										},
										"limits": map[string]interface{}{
											"cpu":    "500m",
											"memory": "2Gi",
										},
									},
								},
							},
						},
					},
				},
				"workerGroupSpecs": []interface{}{
					map[string]interface{}{
						"replicas":    int64(1),
						"minReplicas": int64(1),
						"maxReplicas": int64(1),
						"groupName":   "small-group",
						"rayStartParams": map[string]interface{}{
							"num-cpus": "1",
						},
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name":  "ray-worker",
										"image": rayImage,
										"resources": map[string]interface{}{
											"requests": map[string]interface{}{
												"cpu":    "300m",
												"memory": "1Gi",
											},
											"limits": map[string]interface{}{
												"cpu":    "500m",
												"memory": "1Gi",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// newRayJobUnstructured creates an unstructured RayJob object with an inline RayCluster spec
// and a simple Python entrypoint that verifies Ray is working.
func newRayJobUnstructured(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "ray.io/v1",
			"kind":       "RayJob",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"entrypoint":               "python -c \"import ray; ray.init(); print(ray.cluster_resources()); ray.shutdown()\"",
				"shutdownAfterJobFinishes": true,
				"ttlSecondsAfterFinished":  600,
				"submitterPodTemplate": map[string]interface{}{
					"spec": map[string]interface{}{
						"restartPolicy": "Never",
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "ray-job-submitter",
								"image": rayImage,
								"resources": map[string]interface{}{
									"requests": map[string]interface{}{
										"cpu":    "200m",
										"memory": "200Mi",
									},
									"limits": map[string]interface{}{
										"cpu":    "500m",
										"memory": "500Mi",
									},
								},
							},
						},
					},
				},
				"rayClusterSpec": map[string]interface{}{
					"rayVersion": "2.41.0",
					"headGroupSpec": map[string]interface{}{
						"rayStartParams": map[string]interface{}{
							"dashboard-host": "0.0.0.0",
						},
						"template": map[string]interface{}{
							"spec": map[string]interface{}{
								"containers": []interface{}{
									map[string]interface{}{
										"name":  "ray-head",
										"image": rayImage,
										"ports": []interface{}{
											map[string]interface{}{
												"containerPort": int64(6379),
												"name":          "gcs-server",
											},
											map[string]interface{}{
												"containerPort": int64(8265),
												"name":          "dashboard",
											},
											map[string]interface{}{
												"containerPort": int64(10001),
												"name":          "client",
											},
										},
										"resources": map[string]interface{}{
											"requests": map[string]interface{}{
												"cpu":    "300m",
												"memory": "1Gi",
											},
											"limits": map[string]interface{}{
												"cpu":    "500m",
												"memory": "2Gi",
											},
										},
									},
								},
							},
						},
					},
					"workerGroupSpecs": []interface{}{
						map[string]interface{}{
							"replicas":    int64(1),
							"minReplicas": int64(1),
							"maxReplicas": int64(1),
							"groupName":   "small-group",
							"rayStartParams": map[string]interface{}{
								"num-cpus": "1",
							},
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"containers": []interface{}{
										map[string]interface{}{
											"name":  "ray-worker",
											"image": rayImage,
											"resources": map[string]interface{}{
												"requests": map[string]interface{}{
													"cpu":    "300m",
													"memory": "1Gi",
												},
												"limits": map[string]interface{}{
													"cpu":    "500m",
													"memory": "1Gi",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// describeKubeRayOperatorLogs returns the logs of the KubeRay operator pod for debug output.
func describeKubeRayOperatorLogs(ctx context.Context, clientset *kubernetes.Clientset) string {
	podsClient := clientset.CoreV1().Pods(corev1.NamespaceAll)
	pods, err := podsClient.List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=kuberay-operator",
	})
	if err != nil {
		return fmt.Sprintf("failed to list KubeRay operator pods: %v", err)
	}
	b := strings.Builder{}
	for _, pod := range pods.Items {
		b.WriteString(fmt.Sprintf("\nLogs for KubeRay operator pod %s:\n", pod.Name))
		b.WriteString(getPodLogs(ctx, clientset, pod))
	}
	return b.String()
}

// describeRayJobStatus returns debug information about a RayJob and the KubeRay operator logs.
func describeRayJobStatus(ctx context.Context, dynamicClient dynamic.Interface, name, namespace string, clientset *kubernetes.Clientset) string {
	b := strings.Builder{}
	rj, err := dynamicClient.Resource(rayJobGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		b.WriteString(fmt.Sprintf("failed to get RayJob %s/%s: %v\n", namespace, name, err))
	} else {
		b.WriteString(fmt.Sprintf("RayJob %s/%s status: %v\n", namespace, name, rj.Object["status"]))
	}
	b.WriteString(describeKubeRayOperatorLogs(ctx, clientset))
	return b.String()
}

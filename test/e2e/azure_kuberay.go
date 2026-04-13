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
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cluster-api/test/framework"
)

const (
	kubeRayOperatorHelmRepoURL     = "https://ray-project.github.io/kuberay-helm/"
	kubeRayOperatorHelmChartName   = "kuberay-operator"
	kubeRayOperatorHelmReleaseName = "kuberay-operator"
	kubeRayOperatorNamespace       = "default"
	kubeRayVersion                 = "1.6.0"
	rayVersion                     = "2.54.1"
	rayImage                       = "rayproject/ray:" + rayVersion
	objectStoreMemory              = "200000000" // ~200MB, prevents Ray from consuming all of /dev/shm
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

var workloadGVR = schema.GroupVersionResource{
	Group:    "scheduling.k8s.io",
	Version:  "v1alpha2",
	Resource: "workloads",
}

var podGroupGVR = schema.GroupVersionResource{
	Group:    "scheduling.k8s.io",
	Version:  "v1alpha2",
	Resource: "podgroups",
}

var podGVR = schema.GroupVersionResource{
	Group:    "",
	Version:  "v1",
	Resource: "pods",
}

// KubeRaySpecInput is the shared input type for all KubeRay test specs.
type KubeRaySpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	SkipCleanup           bool
}

// KubeRayClusterSpec implements a test that verifies the KubeRay operator can be installed
// on a workload cluster and a RayCluster can be created and become ready.
// This corresponds to the "Test RayCluster and GCS E2E" case from the KubeRay buildkite CI.
func KubeRayClusterSpec(ctx context.Context, inputGetter func() KubeRaySpecInput) {
	var (
		specName = "kuberay-cluster"
		input    KubeRaySpecInput
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

	waitForRayClusterReady(ctx, dynamicClient, "raycluster-e2e", specName, clientset)
	waitForRayPodRunning(ctx, clientset, "ray.io/node-type=head", specName, "head pod did not reach Running state")
	waitForRayPodRunning(ctx, clientset, "ray.io/node-type=worker", specName, "worker pod did not reach Running state")

	if !input.SkipCleanup {
		By("deleting the RayCluster")
		err = dynamicClient.Resource(rayClusterGVR).Namespace(corev1.NamespaceDefault).Delete(ctx, "raycluster-e2e", metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	}
}

// KubeRayJobSpec implements a test that verifies the KubeRay operator can be installed
// on a workload cluster and a RayJob can be created and completed successfully.
// This corresponds to the "Test RayJob E2E" case from the KubeRay buildkite CI.
func KubeRayJobSpec(ctx context.Context, inputGetter func() KubeRaySpecInput) {
	var (
		specName = "kuberay-job"
		input    KubeRaySpecInput
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
	InstallHelmChart(ctx, clusterProxy, kubeRayOperatorNamespace, kubeRayOperatorHelmRepoURL, "kuberay", kubeRayOperatorHelmChartName, kubeRayOperatorHelmReleaseName,
		"--version", kubeRayVersion,
		"--set", "nodeSelector.kubernetes\\.io/os=linux",
	)

	By("waiting for the KubeRay operator deployment to become available")
	waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, kubeRayOperatorHelmReleaseName, kubeRayOperatorNamespace, specName)
	WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
}

// InstallHelmChart installs a Helm chart from a repo URL onto a cluster via the given ClusterProxy.
// It uses the workload cluster kubeconfig to run helm commands directly from the test runner.
func InstallHelmChart(ctx context.Context, clusterProxy framework.ClusterProxy, namespace, repoURL, repoName, chartName, releaseName string, extraArgs ...string) {
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
	helmArgs := []string{"install", releaseName,
		fmt.Sprintf("%s/%s", repoName, chartName),
		"--namespace", namespace,
		"--create-namespace",
		"--wait",
		"--timeout", "5m0s",
	}
	helmArgs = append(helmArgs, extraArgs...)
	installCmd := exec.CommandContext(ctx, "helm", helmArgs...) //nolint:gosec
	installCmd.Env = append(installCmd.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err = installCmd.CombinedOutput()
	Logf("helm install output: %s", string(output))
	Expect(err).NotTo(HaveOccurred(), "failed to install Helm chart: %s", string(output))
}

// InstallHelmChartFromPath installs a Helm chart from a local directory path onto a cluster
// via the given ClusterProxy. This is used when installing charts built from source rather
// than from a remote Helm repository.
func InstallHelmChartFromPath(ctx context.Context, clusterProxy framework.ClusterProxy, namespace, chartPath, releaseName string, extraArgs ...string) {
	kubeconfigPath := clusterProxy.GetKubeconfigPath()
	By(fmt.Sprintf("Installing Helm chart from %s as release %s using kubeconfig %s", chartPath, releaseName, kubeconfigPath))

	helmArgs := []string{"install", releaseName,
		chartPath,
		"--namespace", namespace,
		"--create-namespace",
		"--wait",
		"--timeout", "10m0s",
		"--debug",
	}
	helmArgs = append(helmArgs, extraArgs...)
	installCmd := exec.CommandContext(ctx, "helm", helmArgs...) //nolint:gosec
	installCmd.Env = append(installCmd.Environ(), fmt.Sprintf("KUBECONFIG=%s", kubeconfigPath))
	output, err := installCmd.CombinedOutput()
	Logf("helm install output: %s", string(output))
	if err != nil {
		dumpHelmInstallDiagnostics(ctx, clusterProxy, namespace, releaseName)
	}
	Expect(err).NotTo(HaveOccurred(), "failed to install Helm chart from path: %s", string(output))
}

// dumpHelmInstallDiagnostics logs pod status and events in the target namespace to help diagnose Helm install failures.
func dumpHelmInstallDiagnostics(ctx context.Context, clusterProxy framework.ClusterProxy, namespace, releaseName string) {
	clientset := clusterProxy.GetClientSet()
	if clientset == nil {
		Logf("WARNING: could not get clientset for diagnostics")
		return
	}

	Logf("=== Helm install diagnostics for release %s in namespace %s ===", releaseName, namespace)

	// List pods in the namespace
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		Logf("WARNING: failed to list pods in namespace %s: %v", namespace, err)
	} else {
		for i := range pods.Items {
			pod := &pods.Items[i]
			Logf("Pod %s: Phase=%s", pod.Name, pod.Status.Phase)
			for _, cs := range pod.Status.ContainerStatuses {
				Logf("  Container %s: Ready=%v, RestartCount=%d, State=%+v", cs.Name, cs.Ready, cs.RestartCount, cs.State)
			}
			for _, cond := range pod.Status.Conditions {
				if cond.Status != corev1.ConditionTrue {
					Logf("  Condition %s=%s: %s", cond.Type, cond.Status, cond.Message)
				}
			}
			Logf("  Events:\n%s", describeEvents(ctx, clientset, namespace, pod.Name))
			Logf("  Logs:\n%s", getPodLogs(ctx, clientset, *pod))
		}
	}

	// Check for CRD readiness
	Logf("=== CRD status ===")
	kubeconfigPath := clusterProxy.GetKubeconfigPath()
	crdCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath, "get", "crds", "-o", "wide") //nolint:gosec
	crdOutput, crdErr := crdCmd.CombinedOutput()
	if crdErr != nil {
		Logf("WARNING: failed to list CRDs: %v", crdErr)
	} else {
		Logf("CRDs:\n%s", string(crdOutput))
	}
}

// InstallKubeRayOperatorFromSource installs the KubeRay operator Helm chart from a local kuberay source tree,
// using a custom-built operator image. This is used for testing unreleased kuberay features like NativeWorkloadScheduling.
// The kuberay source directory must contain the helm-chart/kuberay-operator chart and the image must already
// be built and pushed to the registry (see scripts/ci-build-kuberay-operator.sh).
func InstallKubeRayOperatorFromSource(ctx context.Context, clusterProxy framework.ClusterProxy, specName string) {
	By("Installing the KubeRay operator from source with NativeWorkloadScheduling enabled")
	clientset := clusterProxy.GetClientSet()
	Expect(clientset).NotTo(BeNil())

	kuberaySourceDir := os.Getenv("KUBERAY_SOURCE_DIR")
	Expect(kuberaySourceDir).NotTo(BeEmpty(), "KUBERAY_SOURCE_DIR must be set to the kuberay repo root")

	chartPath := filepath.Join(kuberaySourceDir, "helm-chart", "kuberay-operator")
	_, err := os.Stat(filepath.Join(chartPath, "Chart.yaml"))
	Expect(err).NotTo(HaveOccurred(), "kuberay-operator Helm chart not found at %s", chartPath)

	registry := os.Getenv("REGISTRY")
	Expect(registry).NotTo(BeEmpty(), "REGISTRY must be set")
	imageTag := os.Getenv("KUBERAY_OPERATOR_IMAGE_TAG")
	Expect(imageTag).NotTo(BeEmpty(), "KUBERAY_OPERATOR_IMAGE_TAG must be set")

	operatorImage := fmt.Sprintf("%s/kuberay-operator", registry)

	InstallHelmChartFromPath(ctx, clusterProxy, kubeRayOperatorNamespace, chartPath, kubeRayOperatorHelmReleaseName,
		"--set", fmt.Sprintf("image.repository=%s", operatorImage),
		"--set", fmt.Sprintf("image.tag=%s", imageTag),
		"--set", "image.pullPolicy=Always",
		"--set", "nodeSelector.kubernetes\\.io/os=linux",
		"--set", "featureGates[0].name=RayClusterStatusConditions",
		"--set", "featureGates[0].enabled=true",
		"--set", "featureGates[1].name=RayJobDeletionPolicy",
		"--set", "featureGates[1].enabled=true",
		"--set", "featureGates[2].name=NativeWorkloadScheduling",
		"--set", "featureGates[2].enabled=true",
	)

	By("waiting for the KubeRay operator deployment to become available")
	waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, kubeRayOperatorHelmReleaseName, kubeRayOperatorNamespace, specName)
	WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
}

// KubeRayNativeSchedulingSpec implements a test that verifies the NativeWorkloadScheduling feature
// of the KubeRay operator. It creates a RayCluster with the opt-in annotation, verifies that
// Workload and PodGroup resources are created by the operator, and confirms that all pods are
// scheduled and running via the native gang scheduling mechanism.
func KubeRayNativeSchedulingSpec(ctx context.Context, inputGetter func() KubeRaySpecInput) {
	var (
		specName       = "kuberay-native-scheduling"
		input          KubeRaySpecInput
		rayClusterName = "raycluster-scheduling-e2e"
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

	By("installing the KubeRay operator from source with NativeWorkloadScheduling enabled")
	InstallKubeRayOperatorFromSource(ctx, clusterProxy, specName)

	By("creating a RayCluster with native workload scheduling annotation")
	dynamicClient := newDynamicClient(clusterProxy)
	rayCluster := newRayClusterWithNativeScheduling(rayClusterName, corev1.NamespaceDefault)
	_, err := dynamicClient.Resource(rayClusterGVR).Namespace(corev1.NamespaceDefault).Create(ctx, rayCluster, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	waitForRayClusterReady(ctx, dynamicClient, rayClusterName, specName, clientset)

	By("verifying a Workload resource was created for the RayCluster")
	var workload *unstructured.Unstructured
	Eventually(func() error {
		var err error
		workload, err = dynamicClient.Resource(workloadGVR).Namespace(corev1.NamespaceDefault).Get(ctx, rayClusterName, metav1.GetOptions{})
		return err
	}, e2eConfig.GetIntervals(specName, "wait-workload-ready")...).Should(Succeed(), "Workload resource was not created for the RayCluster")
	Logf("Found Workload %s for RayCluster %s", workload.GetName(), rayClusterName)

	By("validating Workload .spec fields")

	// Validate controllerRef points to the RayCluster
	controllerRefAPIGroup, _, _ := unstructured.NestedString(workload.Object, "spec", "controllerRef", "apiGroup")
	Expect(controllerRefAPIGroup).To(Equal("ray.io"), "Workload controllerRef.apiGroup should be ray.io")
	controllerRefKind, _, _ := unstructured.NestedString(workload.Object, "spec", "controllerRef", "kind")
	Expect(controllerRefKind).To(Equal("RayCluster"), "Workload controllerRef.kind should be RayCluster")
	controllerRefName, _, _ := unstructured.NestedString(workload.Object, "spec", "controllerRef", "name")
	Expect(controllerRefName).To(Equal(rayClusterName), "Workload controllerRef.name should match RayCluster name")

	// Validate podGroupTemplates: expect 2 entries (head + 1 worker group)
	podGroupTemplates, found, err := unstructured.NestedSlice(workload.Object, "spec", "podGroupTemplates")
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue(), "Workload should have podGroupTemplates")
	Expect(podGroupTemplates).To(HaveLen(2), "Workload should have 2 podGroupTemplates (head + 1 worker group)")

	templateNames := make([]string, 0, len(podGroupTemplates))
	for _, t := range podGroupTemplates {
		tMap, ok := t.(map[string]interface{})
		Expect(ok).To(BeTrue())
		name, _, _ := unstructured.NestedString(tMap, "name")
		templateNames = append(templateNames, name)
	}
	Expect(templateNames).To(ConsistOf("head", "worker-small-group"), "Workload podGroupTemplates should contain head and worker-small-group")
	Logf("Workload .spec validated: controllerRef=%s/%s/%s, podGroupTemplates=%v", controllerRefAPIGroup, controllerRefKind, controllerRefName, templateNames)

	headPGName := rayClusterName + "-head"
	workerPGName := rayClusterName + "-worker-small-group"

	By("verifying PodGroup resources were created for the RayCluster")
	Eventually(func() bool {
		_, headErr := dynamicClient.Resource(podGroupGVR).Namespace(corev1.NamespaceDefault).Get(ctx, headPGName, metav1.GetOptions{})
		_, workerErr := dynamicClient.Resource(podGroupGVR).Namespace(corev1.NamespaceDefault).Get(ctx, workerPGName, metav1.GetOptions{})
		return headErr == nil && workerErr == nil
	}, e2eConfig.GetIntervals(specName, "wait-workload-ready")...).Should(BeTrue(), "PodGroup resources were not created for the RayCluster")
	Logf("Found PodGroups %s and %s", headPGName, workerPGName)

	By("validating PodGroup .spec fields")
	headPG, err := dynamicClient.Resource(podGroupGVR).Namespace(corev1.NamespaceDefault).Get(ctx, headPGName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to get head PodGroup")
	headWorkloadName, _, _ := unstructured.NestedString(headPG.Object, "spec", "podGroupTemplateRef", "workload", "workloadName")
	Expect(headWorkloadName).To(Equal(rayClusterName), "head PodGroup should reference the Workload")
	headTemplateName, _, _ := unstructured.NestedString(headPG.Object, "spec", "podGroupTemplateRef", "workload", "podGroupTemplateName")
	Expect(headTemplateName).To(Equal("head"), "head PodGroup should reference the 'head' template")
	Logf("Head PodGroup validated: workloadName=%s, podGroupTemplateName=%s", headWorkloadName, headTemplateName)

	workerPG, err := dynamicClient.Resource(podGroupGVR).Namespace(corev1.NamespaceDefault).Get(ctx, workerPGName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to get worker PodGroup")
	workerWorkloadName, _, _ := unstructured.NestedString(workerPG.Object, "spec", "podGroupTemplateRef", "workload", "workloadName")
	Expect(workerWorkloadName).To(Equal(rayClusterName), "worker PodGroup should reference the Workload")
	workerTemplateName, _, _ := unstructured.NestedString(workerPG.Object, "spec", "podGroupTemplateRef", "workload", "podGroupTemplateName")
	Expect(workerTemplateName).To(Equal("worker-small-group"), "worker PodGroup should reference the 'worker-small-group' template")

	// Validate worker PodGroup has gang scheduling policy with correct minCount
	workerGangMinCount, found, err := unstructured.NestedFieldNoCopy(workerPG.Object, "spec", "schedulingPolicy", "gang", "minCount")
	Expect(err).NotTo(HaveOccurred())
	Expect(found).To(BeTrue(), "worker PodGroup should have gang scheduling policy with minCount")
	Expect(workerGangMinCount).To(BeNumerically("==", 1), "worker PodGroup gang minCount should be 1 (matching replicas)")
	Logf("Worker PodGroup validated: workloadName=%s, podGroupTemplateName=%s, gang.minCount=%v", workerWorkloadName, workerTemplateName, workerGangMinCount)

	waitForRayPodRunning(ctx, clientset, "ray.io/node-type=head", specName, "head pod did not reach Running state")
	waitForRayPodRunning(ctx, clientset, "ray.io/node-type=worker", specName, "worker pod did not reach Running state")

	By("verifying head pod has schedulingGroup referencing its PodGroup")
	headPods, err := dynamicClient.Resource(podGVR).Namespace(corev1.NamespaceDefault).List(ctx, metav1.ListOptions{
		LabelSelector: "ray.io/node-type=head",
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(headPods.Items).NotTo(BeEmpty(), "expected at least one head pod")
	headSchedulingGroup, _, _ := unstructured.NestedString(headPods.Items[0].Object, "spec", "schedulingGroup", "podGroupName")
	Expect(headSchedulingGroup).To(Equal(rayClusterName+"-head"), "head pod should reference its PodGroup via schedulingGroup")
	Logf("Head pod %s has schedulingGroup.podGroupName=%s", headPods.Items[0].GetName(), headSchedulingGroup)

	By("verifying worker pod has schedulingGroup referencing its PodGroup")
	workerPods, err := dynamicClient.Resource(podGVR).Namespace(corev1.NamespaceDefault).List(ctx, metav1.ListOptions{
		LabelSelector: "ray.io/node-type=worker",
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(workerPods.Items).NotTo(BeEmpty(), "expected at least one worker pod")
	workerSchedulingGroup, _, _ := unstructured.NestedString(workerPods.Items[0].Object, "spec", "schedulingGroup", "podGroupName")
	Expect(workerSchedulingGroup).To(Equal(rayClusterName+"-worker-small-group"), "worker pod should reference its PodGroup via schedulingGroup")
	Logf("Worker pod %s has schedulingGroup.podGroupName=%s", workerPods.Items[0].GetName(), workerSchedulingGroup)

	By("testing gang scheduling: scaling workers beyond available resources")
	scaleUpPatch := []byte(`[
		{"op": "replace", "path": "/spec/workerGroupSpecs/0/replicas", "value": 20},
		{"op": "replace", "path": "/spec/workerGroupSpecs/0/minReplicas", "value": 20},
		{"op": "replace", "path": "/spec/workerGroupSpecs/0/maxReplicas", "value": 20}
	]`)
	_, err = dynamicClient.Resource(rayClusterGVR).Namespace(corev1.NamespaceDefault).Patch(ctx, rayClusterName, types.JSONPatchType, scaleUpPatch, metav1.PatchOptions{})
	Expect(err).NotTo(HaveOccurred())

	workerPGName = rayClusterName + "-worker-small-group"

	By("verifying the worker PodGroup minCount is updated to 20")
	Eventually(func() interface{} {
		pg, err := dynamicClient.Resource(podGroupGVR).Namespace(corev1.NamespaceDefault).Get(ctx, workerPGName, metav1.GetOptions{})
		if err != nil {
			return nil
		}
		minCount, found, _ := unstructured.NestedFieldNoCopy(pg.Object, "spec", "schedulingPolicy", "gang", "minCount")
		if !found {
			return nil
		}
		return minCount
	}, e2eConfig.GetIntervals(specName, "wait-workload-ready")...).Should(BeNumerically("==", 20), "worker PodGroup minCount should be updated to 20")

	By("verifying worker pods are Pending due to gang scheduling (all-or-nothing)")
	Eventually(func() bool {
		pods, err := clientset.CoreV1().Pods(corev1.NamespaceDefault).List(ctx, metav1.ListOptions{
			LabelSelector: "ray.io/node-type=worker",
		})
		if err != nil {
			return false
		}
		runningCount := 0
		pendingCount := 0
		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case corev1.PodRunning:
				runningCount++
			case corev1.PodPending:
				pendingCount++
			}
		}
		Logf("Worker pods: %d total, %d Running, %d Pending", len(pods.Items), runningCount, pendingCount)
		// Gang scheduling all-or-nothing: with minCount=20 and insufficient resources,
		// the scheduler should not schedule any new pods in the gang. At most 1 worker
		// may remain Running from before the scale-up.
		return runningCount <= 1 && pendingCount >= 19
	}, e2eConfig.GetIntervals(specName, "wait-workload-ready")...).Should(BeTrue(), "expected gang scheduling to prevent new workers from Running (all-or-nothing)")
	Logf("Gang scheduling verified: at most 1 worker Running, rest Pending (all-or-nothing)")

	By("scaling workers back to 1 replica to verify recovery")
	scaleDownPatch := []byte(`[
		{"op": "replace", "path": "/spec/workerGroupSpecs/0/replicas", "value": 1},
		{"op": "replace", "path": "/spec/workerGroupSpecs/0/minReplicas", "value": 1},
		{"op": "replace", "path": "/spec/workerGroupSpecs/0/maxReplicas", "value": 1}
	]`)
	_, err = dynamicClient.Resource(rayClusterGVR).Namespace(corev1.NamespaceDefault).Patch(ctx, rayClusterName, types.JSONPatchType, scaleDownPatch, metav1.PatchOptions{})
	Expect(err).NotTo(HaveOccurred())

	By("verifying worker pod is Running again after scaling back down")
	waitForRayPodRunning(ctx, clientset, "ray.io/node-type=worker", specName, "worker pod did not become Running after scaling back to 1")
	Logf("Gang scheduling recovery verified: worker pod is Running after scaling back to 1 replica")

	if !input.SkipCleanup {
		By("deleting the RayCluster")
		err = dynamicClient.Resource(rayClusterGVR).Namespace(corev1.NamespaceDefault).Delete(ctx, rayClusterName, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("verifying the Workload is cleaned up after RayCluster deletion")
		Eventually(func() bool {
			_, err := dynamicClient.Resource(workloadGVR).Namespace(corev1.NamespaceDefault).Get(ctx, rayClusterName, metav1.GetOptions{})
			return apierrors.IsNotFound(err)
		}, e2eConfig.GetIntervals(specName, "wait-workload-ready")...).Should(BeTrue(), "Workload was not cleaned up after RayCluster deletion")

		By("verifying PodGroups are cleaned up after RayCluster deletion")
		Eventually(func() bool {
			_, headErr := dynamicClient.Resource(podGroupGVR).Namespace(corev1.NamespaceDefault).Get(ctx, rayClusterName+"-head", metav1.GetOptions{})
			_, workerErr := dynamicClient.Resource(podGroupGVR).Namespace(corev1.NamespaceDefault).Get(ctx, workerPGName, metav1.GetOptions{})
			return apierrors.IsNotFound(headErr) && apierrors.IsNotFound(workerErr)
		}, e2eConfig.GetIntervals(specName, "wait-workload-ready")...).Should(BeTrue(), "PodGroups were not cleaned up after RayCluster deletion")

		Logf("Cleanup cascade verified: Workload and PodGroups deleted with RayCluster")
	}
}

// KubeRayNativeSchedulingNegativeSpec implements a negative test that verifies the NativeWorkloadScheduling
// feature does NOT create Workload or PodGroup resources when the opt-in annotation is absent.
// It creates a RayCluster without the ray.io/native-workload-scheduling annotation, waits for
// the cluster to become ready, and confirms that no scheduling.k8s.io/v1alpha2 resources exist.
func KubeRayNativeSchedulingNegativeSpec(ctx context.Context, inputGetter func() KubeRaySpecInput) {
	var (
		specName       = "kuberay-native-scheduling"
		input          KubeRaySpecInput
		rayClusterName = "raycluster-no-scheduling-e2e"
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

	By("installing the KubeRay operator from source with NativeWorkloadScheduling enabled")
	InstallKubeRayOperatorFromSource(ctx, clusterProxy, specName)

	By("creating a RayCluster WITHOUT the native workload scheduling annotation")
	dynamicClient := newDynamicClient(clusterProxy)
	rayCluster := newRayClusterUnstructured(rayClusterName, corev1.NamespaceDefault)
	_, err := dynamicClient.Resource(rayClusterGVR).Namespace(corev1.NamespaceDefault).Create(ctx, rayCluster, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	waitForRayClusterReady(ctx, dynamicClient, rayClusterName, specName, clientset)
	waitForRayPodRunning(ctx, clientset, "ray.io/node-type=head", specName, "head pod did not reach Running state")
	waitForRayPodRunning(ctx, clientset, "ray.io/node-type=worker", specName, "worker pod did not reach Running state")

	By("verifying NO Workload resources were created")
	workloads, err := dynamicClient.Resource(workloadGVR).Namespace(corev1.NamespaceDefault).List(ctx, metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(workloads.Items).To(BeEmpty(), "expected no Workload resources when annotation is absent, but found %d", len(workloads.Items))
	Logf("Negative test verified: no Workload resources created without annotation")

	By("verifying NO PodGroup resources were created")
	podGroups, err := dynamicClient.Resource(podGroupGVR).Namespace(corev1.NamespaceDefault).List(ctx, metav1.ListOptions{})
	Expect(err).NotTo(HaveOccurred())
	Expect(podGroups.Items).To(BeEmpty(), "expected no PodGroup resources when annotation is absent, but found %d", len(podGroups.Items))
	Logf("Negative test verified: no PodGroup resources created without annotation")

	By("verifying pods do NOT have schedulingGroup set")
	headPods, err := dynamicClient.Resource(podGVR).Namespace(corev1.NamespaceDefault).List(ctx, metav1.ListOptions{
		LabelSelector: "ray.io/node-type=head",
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(headPods.Items).NotTo(BeEmpty())
	_, found, _ := unstructured.NestedString(headPods.Items[0].Object, "spec", "schedulingGroup", "podGroupName")
	Expect(found).To(BeFalse(), "head pod should not have schedulingGroup when annotation is absent")

	workerPods, err := dynamicClient.Resource(podGVR).Namespace(corev1.NamespaceDefault).List(ctx, metav1.ListOptions{
		LabelSelector: "ray.io/node-type=worker",
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(workerPods.Items).NotTo(BeEmpty())
	_, found, _ = unstructured.NestedString(workerPods.Items[0].Object, "spec", "schedulingGroup", "podGroupName")
	Expect(found).To(BeFalse(), "worker pod should not have schedulingGroup when annotation is absent")
	Logf("Negative test verified: pods do not have schedulingGroup without annotation")

	if !input.SkipCleanup {
		By("deleting the RayCluster")
		err = dynamicClient.Resource(rayClusterGVR).Namespace(corev1.NamespaceDefault).Delete(ctx, rayClusterName, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	}
}

// newRayClusterWithNativeScheduling creates a RayCluster with the native workload scheduling
// opt-in annotation. This triggers the KubeRay operator to create Workload and PodGroup resources
// for gang scheduling via the Kubernetes-native scheduling.k8s.io/v1alpha2 API.
func newRayClusterWithNativeScheduling(name, namespace string) *unstructured.Unstructured {
	rc := newRayClusterUnstructured(name, namespace)
	annotations := map[string]interface{}{
		"ray.io/native-workload-scheduling": "true",
	}
	err := unstructured.SetNestedField(rc.Object, annotations, "metadata", "annotations")
	Expect(err).NotTo(HaveOccurred())
	return rc
}

// waitForRayClusterReady polls until the RayCluster's status.state is "ready".
func waitForRayClusterReady(ctx context.Context, dynamicClient dynamic.Interface, name, specName string, clientset *kubernetes.Clientset) {
	By("waiting for the RayCluster to become ready")
	Eventually(func() bool {
		rc, err := dynamicClient.Resource(rayClusterGVR).Namespace(corev1.NamespaceDefault).Get(ctx, name, metav1.GetOptions{})
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
}

// waitForRayPodRunning polls until at least one pod matching the label selector is Running.
func waitForRayPodRunning(ctx context.Context, clientset *kubernetes.Clientset, labelSelector, specName, failMessage string) {
	By(fmt.Sprintf("verifying a %s pod is running", labelSelector))
	Eventually(func() bool {
		pods, err := clientset.CoreV1().Pods(corev1.NamespaceDefault).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
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
	}, e2eConfig.GetIntervals(specName, "wait-deployment")...).Should(BeTrue(), failMessage)
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
			"spec": rayClusterSpec(),
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
				"rayClusterSpec": rayClusterSpec(),
			},
		},
	}
}

// rayClusterSpec returns the shared RayCluster spec used by both RayCluster and RayJob resources.
func rayClusterSpec() map[string]interface{} {
	return map[string]interface{}{
		"rayVersion": rayVersion,
		"headGroupSpec": map[string]interface{}{
			"rayStartParams": map[string]interface{}{
				"dashboard-host":      "0.0.0.0",
				"object-store-memory": objectStoreMemory,
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
									"cpu":    "500m",
									"memory": "1Gi",
								},
								"limits": map[string]interface{}{
									"cpu":    "1",
									"memory": "4Gi",
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
					"num-cpus":            "1",
					"object-store-memory": objectStoreMemory,
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

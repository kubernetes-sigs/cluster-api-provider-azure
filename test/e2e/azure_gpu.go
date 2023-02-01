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
	"context"
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cluster-api/test/framework"
)

// AzureGPUSpecInput is the input for AzureGPUSpec.
type AzureGPUSpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	SkipCleanup           bool
}

// AzureGPUSpec implements a test that verifies a GPU-enabled application runs on an
// "nvidia-gpu"-flavored CAPZ cluster.
func AzureGPUSpec(ctx context.Context, inputGetter func() AzureGPUSpecInput) {
	var (
		specName    = "azure-gpu"
		input       AzureGPUSpecInput
		machineType = os.Getenv("AZURE_GPU_NODE_MACHINE_TYPE")
	)

	input = inputGetter()
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	Expect(input.ClusterName).NotTo(BeEmpty(), "Invalid argument. input.ClusterName can't be empty when calling %s spec", specName)
	if machineType != "" {
		Expect(machineType).To(HavePrefix("Standard_N"), "AZURE_GPU_NODE_MACHINE_TYPE is \"%s\" which isn't a GPU SKU in %s spec", machineType, specName)
	}

	By("creating a Kubernetes client to the workload cluster")
	clusterProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
	Expect(clusterProxy).NotTo(BeNil())
	clientset := clusterProxy.GetClientSet()
	Expect(clientset).NotTo(BeNil())

	By("Waiting for a node to have an \"nvidia.com/gpu\" allocatable resource")
	Eventually(func() bool {
		nodeList, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		for _, node := range nodeList.Items {
			for k, v := range node.Status.Allocatable {
				if k == "nvidia.com/gpu" && v.Value() > 0 {
					return true
				}
			}
		}
		return false
	}, e2eConfig.GetIntervals(specName, "wait-worker-nodes")...).Should(BeTrue(), func() string {
		return getGPUOperatorPodLogs(ctx, clientset)
	})

	By("running a CUDA vector calculation job")
	jobsClient := clientset.BatchV1().Jobs(corev1.NamespaceDefault)
	jobName := "cuda-vector-add"
	gpuJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: corev1.NamespaceDefault,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:  jobName,
							Image: "nvcr.io/nvidia/k8s/cuda-sample:vectoradd-cuda11.2.1",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									"nvidia.com/gpu": resource.MustParse("1"),
								},
							},
						},
					},
				},
			},
		},
	}
	Log("starting to create CUDA vector calculation job")
	_, err := jobsClient.Create(ctx, gpuJob, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
	gpuJobInput := WaitForJobCompleteInput{
		Getter:    jobsClientAdapter{client: jobsClient},
		Job:       gpuJob,
		Clientset: clientset,
	}
	WaitForJobComplete(ctx, gpuJobInput, e2eConfig.GetIntervals(specName, "wait-job")...)
}

// getGPUOperatorPodLogs returns the logs of the Nvidia GPU operator pods.
func getGPUOperatorPodLogs(ctx context.Context, clientset *kubernetes.Clientset) string {
	podsClient := clientset.CoreV1().Pods(corev1.NamespaceAll)
	var pods *corev1.PodList
	var err error
	Eventually(func(g Gomega) {
		pods, err = podsClient.List(ctx, metav1.ListOptions{LabelSelector: "app.kubernetes.io/instance=gpu-operator"})
		if err != nil {
			LogWarning(err.Error())
		}
		g.Expect(err).NotTo(HaveOccurred())
	}, retryableOperationTimeout, retryableOperationSleepBetweenRetries).Should(Succeed())
	b := strings.Builder{}
	for _, pod := range pods.Items {
		b.WriteString(fmt.Sprintf("\nLogs for pod %s:\n", pod.Name))
		b.WriteString(getPodLogs(ctx, clientset, pod))
	}
	return b.String()
}

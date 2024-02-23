//go:build e2e
// +build e2e

/*
Copyright 2023 The Kubernetes Authors.

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/test/framework"
)

const (
	nvidiaHelmChartRepoURL           string = "https://helm.ngc.nvidia.com/nvidia"
	nvidiaGPUOperatorNamespace       string = "default"
	nvidiaGPUOperatorHelmReleaseName string = "nvidia-gpu-operator"
	nvidiaGPUOperatorHelmChartName   string = "gpu-operator"
)

// GPUOperatorSpecInput is the input for InstallGPUOperator.
type GPUOperatorSpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
}

// InstallGPUOperator installs the official nvidia/gpu-operator helm chart.
func InstallGPUOperator(ctx context.Context, inputGetter func() GPUOperatorSpecInput) {
	var (
		specName = "nvidia-gpu-operator"
		input    GPUOperatorSpecInput
	)

	Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	Expect(input.ClusterName).NotTo(BeEmpty(), "Invalid argument. input.ClusterName can't be empty when calling %s spec", specName)
	clusterProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
	InstallNvidiaGPUOperatorChart(ctx, clusterProxy)
}

// InstallNvidiaGPUOperatorChart installs the official nvidia/gpu-operator helm chart
func InstallNvidiaGPUOperatorChart(ctx context.Context, clusterProxy framework.ClusterProxy) {
	By("Installing nvidia/gpu-operator via helm")
	values := &HelmOptions{}
	InstallHelmChart(ctx, clusterProxy, nvidiaGPUOperatorNamespace, nvidiaHelmChartRepoURL, nvidiaGPUOperatorHelmChartName, nvidiaGPUOperatorHelmReleaseName, values, "")
}

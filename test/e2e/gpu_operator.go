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
	nvidiaGPUOperatorNamespace string = "default"
)

// EnsureGPUOperatorInput is the input for InstallGPUOperator.
type EnsureGPUOperatorInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
}

// EnsureGPUOperator installs the official nvidia/gpu-operator helm chart.
func EnsureGPUOperator(ctx context.Context, inputGetter func() EnsureGPUOperatorInput) {
	var (
		specName = "nvidia-gpu-operator"
		input    EnsureGPUOperatorInput
	)

	Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	Expect(input.ClusterName).NotTo(BeEmpty(), "Invalid argument. input.ClusterName can't be empty when calling %s spec", specName)
	clusterProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)

	By("Ensuring GPU Operator is installed via CAAPH")

	By("Waiting for Ready gpu-operator deployment pods")
	for _, d := range []string{"gpu-operator"} {
		waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, d, nvidiaGPUOperatorNamespace, specName)
		WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
	}
}

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
	"io"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

// E2E config variables for the KubeRay compatibility tests. Each Prow job pins one
// KubeRay/Kubernetes version pair by overriding these with environment variables.
const (
	KubeRayCompatAKSKubernetesVersion = "KUBERAY_COMPAT_AKS_KUBERNETES_VERSION"
	KubeRayCompatKubeRayVersion       = "KUBERAY_COMPAT_KUBERAY_VERSION"
	KubeRayCompatRayImage             = "KUBERAY_COMPAT_RAY_IMAGE"
	KubeRayCompatNodeMachineType      = "KUBERAY_COMPAT_NODE_MACHINE_TYPE"
	KubeRayCompatSuites               = "KUBERAY_COMPAT_SUITES"
	KubeRayCompatScript               = "KUBERAY_COMPAT_SCRIPT"
)

// KubeRayCompatSpecInput is the input for KubeRayCompatSpec.
type KubeRayCompatSpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	E2EConfig             *clusterctl.E2EConfig
	ArtifactFolder        string
}

// KubeRayCompatSpec runs the upstream KubeRay e2e suites against a workload cluster using
// scripts/kuberay-compat.sh. The script installs the released kuberay-operator Helm chart and
// writes one junit.kuberay-compat.<suite>.xml per suite to the artifacts folder so Prow reports
// each upstream test individually.
func KubeRayCompatSpec(ctx context.Context, inputGetter func() KubeRayCompatSpecInput) {
	var (
		specName = "kuberay-compat"
		input    KubeRayCompatSpecInput
	)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	Expect(input.ClusterName).NotTo(BeEmpty(), "Invalid argument. input.ClusterName can't be empty when calling %s spec", specName)
	Expect(input.E2EConfig).NotTo(BeNil(), "Invalid argument. input.E2EConfig can't be nil when calling %s spec", specName)
	Expect(input.ArtifactFolder).NotTo(BeEmpty(), "Invalid argument. input.ArtifactFolder can't be empty when calling %s spec", specName)

	clusterProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
	Expect(clusterProxy).NotTo(BeNil())

	logDir := filepath.Join(input.ArtifactFolder, "clusters", input.ClusterName, specName)
	Expect(os.MkdirAll(logDir, 0o755)).To(Succeed())
	logFile, err := os.Create(filepath.Join(logDir, specName+".log"))
	Expect(err).NotTo(HaveOccurred())
	defer logFile.Close()

	Byf("Running the upstream KubeRay %s e2e suites", input.E2EConfig.MustGetVariable(KubeRayCompatKubeRayVersion))
	cmd := exec.CommandContext(ctx, input.E2EConfig.MustGetVariable(KubeRayCompatScript))
	cmd.Env = append(os.Environ(),
		"KUBECONFIG="+clusterProxy.GetKubeconfigPath(),
		"ARTIFACTS="+input.ArtifactFolder,
		"KUBERAY_COMPAT_LOG_DIR="+logDir,
		KubeRayCompatKubeRayVersion+"="+input.E2EConfig.MustGetVariable(KubeRayCompatKubeRayVersion),
		KubeRayCompatRayImage+"="+input.E2EConfig.GetVariableOrEmpty(KubeRayCompatRayImage),
		KubeRayCompatSuites+"="+input.E2EConfig.GetVariableOrEmpty(KubeRayCompatSuites),
	)
	out := io.MultiWriter(GinkgoWriter, logFile)
	cmd.Stdout = out
	cmd.Stderr = out
	Expect(cmd.Run()).To(Succeed(), "KubeRay e2e suites failed; see %s/junit.kuberay-compat.*.xml and %s", input.ArtifactFolder, logDir)
}

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
	"os/exec"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api/test/framework"
)

// HelmOptions handles arguments to a `helm install` command.
type HelmOptions struct {
	StringValues []string // --set-string
	ValueFiles   []string // --values
	Values       []string // --set
}

// InstallHelmChart takes a Helm repo URL, a chart name, and release name, and creates a Helm release on the E2E workload cluster.
func InstallHelmChart(_ context.Context, clusterProxy framework.ClusterProxy, namespace, repoURL, chartName, releaseName string, options *HelmOptions, version string) {
	// Check that Helm v3 is installed
	helm, err := exec.LookPath("helm")
	Expect(err).NotTo(HaveOccurred(), "No helm binary found in PATH")
	cmd := exec.Command(helm, "version", "--short") //nolint:gosec // Suppress G204: Subprocess launched with variable warning since this is a test file
	stdout, err := cmd.Output()
	Expect(err).NotTo(HaveOccurred())
	Logf("Helm version: %s", stdout)
	Expect(stdout).To(HavePrefix("v3."), "Helm v3 is required")

	// Set up the Helm command arguments.
	args := []string{
		"upgrade", releaseName, chartName, "--install",
		"--kubeconfig", clusterProxy.GetKubeconfigPath(),
		"--create-namespace", "--namespace", namespace,
	}
	if repoURL != "" {
		args = append(args, "--repo", repoURL)
	}
	for _, stringValue := range options.StringValues {
		args = append(args, "--set-string", stringValue)
	}
	for _, valueFile := range options.ValueFiles {
		args = append(args, "--values", valueFile)
	}
	for _, value := range options.Values {
		args = append(args, "--set", value)
	}
	if version != "" {
		args = append(args, "--version", version)
	}

	// Install the chart and retry if needed
	Eventually(func() error {
		cmd := exec.Command(helm, args...) //nolint:gosec // Suppress G204: Subprocess launched with variable warning since this is a test file
		Logf("Helm command: %s", cmd.String())
		output, err := cmd.CombinedOutput()
		Logf("Helm install output: %s", string(output))
		return err
	}, helmInstallTimeout, retryableOperationSleepBetweenRetries).Should(Succeed())
}

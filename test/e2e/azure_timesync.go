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
	"strings"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/cluster-api/test/framework"
	kinderrors "sigs.k8s.io/kind/pkg/errors"
)

// AzureTimeSyncSpecInput is the input for AzureTimeSyncSpec.
type AzureTimeSyncSpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
}

// AzureTimeSyncSpec implements a test that verifies time synchronization is healthy for
// the nodes in a cluster.
func AzureTimeSyncSpec(ctx context.Context, inputGetter func() AzureTimeSyncSpecInput) {
	var (
		specName = "azure-timesync"
		input    AzureTimeSyncSpecInput
	)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)

	namespace, name := input.Namespace.Name, input.ClusterName
	managementClusterClient := input.BootstrapClusterProxy.GetClient()

	sshInfo, err := getClusterSSHInfo(ctx, managementClusterClient, namespace, name)
	Expect(err).NotTo(HaveOccurred())
	Expect(len(sshInfo)).To(BeNumerically(">", 0))

	testfuncs := []func() error{}
	for _, s := range sshInfo {
		Byf("checking that time synchronization is healthy on %s", s.Hostname)

		execToStringFn := func(expected, command string, args ...string) func() error {
			// don't assert in this test func, just return errors
			return func() error {
				f := &strings.Builder{}
				if err := execOnHost(s.Endpoint, s.Hostname, s.Port, f, command, args...); err != nil {
					return err
				}
				if !strings.Contains(f.String(), expected) {
					return fmt.Errorf("expected \"%s\" in command output:\n%s", expected, f.String())
				}
				return nil
			}
		}

		testfuncs = append(testfuncs,
			execToStringFn(
				"✓ chronyd is active",
				"systemctl", "is-active", "chronyd", "&&",
				"echo", "✓ chronyd is active",
			),
			execToStringFn(
				"Reference ID",
				"chronyc", "tracking",
			),
		)
	}

	Expect(kinderrors.AggregateConcurrent(testfuncs)).To(Succeed())
}

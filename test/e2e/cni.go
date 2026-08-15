//go:build e2e
// +build e2e

/*
Copyright 2022 The Kubernetes Authors.

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
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

const (
	calicoHelmChartRepoURL   string = "https://docs.tigera.io/calico/charts"
	calicoOperatorNamespace  string = "tigera-operator"
	CalicoSystemNamespace    string = "calico-system"
	CalicoAPIServerNamespace string = "calico-apiserver"
	calicoHelmReleaseName    string = "projectcalico"
	calicoHelmChartName      string = "tigera-operator"
	kubeadmConfigMapName     string = "kubeadm-config"
	AzureCNIv1               string = "azure-cni-v1"
)

// EnsureCNI installs the CNI plugin depending on the input.CNIManifestPath
func EnsureCNI(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput) {
	if input.CNIManifestPath != "" {
		InstallCNIManifest(ctx, input)
	} else {
		EnsureCalicoIsReady(ctx, input)
	}
}

// cniInstallBackoff is the backoff used when retrying CNI manifest installation on conflict.
var cniInstallBackoff = wait.Backoff{
	Duration: 500 * time.Millisecond,
	Factor:   2,
	Jitter:   0.1,
	Steps:    6,
}

// isConflictError reports whether err is retryable due to a resource-version conflict.
//
// Semantics:
//   - nil is not a conflict.
//   - A direct apierrors.IsConflict error is retryable.
//   - A utilerrors.Aggregate is retryable only when it is non-empty and every
//     contained error satisfies apierrors.IsConflict.
//   - A mixed aggregate (some conflict, some not) is not retryable.
func isConflictError(err error) bool {
	if err == nil {
		return false
	}
	if agg, ok := err.(utilerrors.Aggregate); ok {
		errs := agg.Errors()
		if len(errs) == 0 {
			return false
		}
		for _, e := range errs {
			if !apierrors.IsConflict(e) {
				return false
			}
		}
		return true
	}
	return apierrors.IsConflict(err)
}

// retryCreateOrUpdate runs fn with bounded exponential backoff, retrying only
// on conflict errors. lastErr is set to the most recent error returned by fn.
// It returns the terminal error from the backoff loop (nil on success,
// the non-conflict error if fn returned one, or wait.ErrWaitTimeout if all
// attempts were exhausted due to conflicts).
func retryCreateOrUpdate(ctx context.Context, fn func() error, lastErr *error) error {
	backoff := cniInstallBackoff
	return wait.ExponentialBackoffWithContext(ctx, backoff, func(_ context.Context) (bool, error) {
		*lastErr = fn()
		if *lastErr == nil {
			return true, nil
		}
		if isConflictError(*lastErr) {
			// Retryable: the caller will perform a fresh Get on the next attempt.
			return false, nil
		}
		// Non-conflict errors are terminal.
		return false, *lastErr
	})
}

// InstallCNIManifest installs the CNI manifest provided by the user.
// It retries on HTTP 409 Conflict (stale resourceVersion) using bounded
// exponential backoff so that a second call after a controller has mutated
// the DaemonSet does not fail the test.
func InstallCNIManifest(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput) {
	By("Installing a CNI plugin to the workload cluster")
	workloadCluster := input.ClusterProxy.GetWorkloadCluster(ctx, input.Namespace, input.ClusterName)

	cniYaml, err := os.ReadFile(input.CNIManifestPath)
	Expect(err).NotTo(HaveOccurred())

	var lastErr error
	retryErr := retryCreateOrUpdate(ctx, func() error {
		return workloadCluster.CreateOrUpdate(ctx, cniYaml)
	}, &lastErr)
	if retryErr != nil {
		// ExponentialBackoffWithContext returns wait.ErrWaitTimeout when steps are
		// exhausted without the condition returning true; surface the last conflict
		// error instead for a clear failure message.
		if lastErr != nil {
			Expect(lastErr).To(Succeed())
		}
		Expect(retryErr).To(Succeed())
	}
}

// EnsureCalicoIsReady copies the kubeadm configmap to the calico-system namespace and waits for the calico pods to be ready.
func EnsureCalicoIsReady(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput) {
	specName := "ensure-calico"

	clusterProxy := input.ClusterProxy.GetWorkloadCluster(ctx, input.Namespace, input.ClusterName)
	By("Ensuring Calico CNI is installed via CAAPH")

	By("Waiting for Ready tigera-operator deployment pods")
	for _, d := range []string{"tigera-operator"} {
		waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, d, calicoOperatorNamespace, specName)
		WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
	}

	By("Waiting for Ready calico-system deployment pods")
	for _, d := range []string{"calico-kube-controllers", "calico-typha"} {
		waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, d, CalicoSystemNamespace, specName)
		WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
	}
	By("Waiting for Ready calico-apiserver deployment pods")
	for _, d := range []string{"calico-apiserver"} {
		waitInput := GetWaitForDeploymentsAvailableInput(ctx, clusterProxy, d, CalicoAPIServerNamespace, specName)
		WaitForDeploymentsAvailable(ctx, waitInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
	}
}

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	kubeadmv1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DaemonsetsSpecInput is the input for EnsureDaemonsets.
type DaemonsetsSpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
}

// EnsureDaemonsets implements a test that verifies expected Daemonset Pods are running.
func EnsureDaemonsets(ctx context.Context, inputGetter func() DaemonsetsSpecInput) {
	var (
		specName = "daemonsets"
		input    DaemonsetsSpecInput
	)

	Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).ToNot(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	Expect(input.ClusterName).ToNot(BeEmpty(), "Invalid argument. input.ClusterName can't be empty when calling %s spec", specName)

	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())
	cluster := framework.GetClusterByName(ctx, framework.GetClusterByNameInput{
		Getter:    mgmtClient,
		Name:      input.ClusterName,
		Namespace: input.Namespace.Name,
	})
	kubeadmControlPlane := &kubeadmv1.KubeadmControlPlane{}
	key := client.ObjectKey{
		Namespace: cluster.Spec.ControlPlaneRef.Namespace,
		Name:      cluster.Spec.ControlPlaneRef.Name,
	}
	Eventually(func() error {
		return mgmtClient.Get(ctx, key, kubeadmControlPlane)
	}, e2eConfig.GetIntervals(specName, "wait-daemonset")...).Should(Succeed(), "Failed to get KubeadmControlPlane object %s/%s", cluster.Spec.ControlPlaneRef.Namespace, cluster.Spec.ControlPlaneRef.Name)

	workloadClusterProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
	By("Waiting for all DaemonSet Pods to be Running")
	WaitForDaemonsets(ctx, workloadClusterProxy, specName, e2eConfig.GetIntervals(specName, "wait-daemonset")...)
}

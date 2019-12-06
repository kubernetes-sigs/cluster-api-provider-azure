/*
Copyright 2019 The Kubernetes Authors.

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

package e2e_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/framework"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/framework/management/kind"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OneNodeClusterInput are all the dependencies of the OneNodeCluster test.
type OneNodeClusterInput struct {
	Management    *kind.Cluster
	Cluster       *clusterv1.Cluster
	InfraCluster  runtime.Object
	Node          framework.Node
	CreateTimeout time.Duration
}

// SetDefaults defaults the struct fields if necessary.
func (o *OneNodeClusterInput) SetDefaults() {
	if o.CreateTimeout == 0*time.Second {
		o.CreateTimeout = 60 * time.Minute
	}
}

// OneNodeCluster creates a single control plane node.
// Assertions:
//   * The created cluster has exactly one node.
//   * The created machines reach the 'running' state.
func OneNodeCluster(input *OneNodeClusterInput) {
	input.SetDefaults()
	ctx := context.Background()
	Expect(input.Management).ToNot(BeNil())

	mgmtClient, err := input.Management.GetClient()
	Expect(err).NotTo(HaveOccurred(), "stack: %+v", err)

	By("Creating infra cluster resource")
	Expect(mgmtClient.Create(ctx, input.InfraCluster)).NotTo(HaveOccurred())

	By("Creating cluster resource that owns the infra cluster")
	Expect(mgmtClient.Create(ctx, input.Cluster)).NotTo(HaveOccurred())

	Eventually(func() string {
		cluster := &clusterv1.Cluster{}
		key := client.ObjectKey{
			Namespace: input.Cluster.GetNamespace(),
			Name:      input.Cluster.GetName(),
		}
		if err := mgmtClient.Get(ctx, key, cluster); err != nil {
			return err.Error()
		}
		return cluster.Status.Phase
	}, input.CreateTimeout, 10*time.Second).Should(Equal(string(clusterv1.ClusterPhaseProvisioned)))

	By("Creating infra machine resource")
	Expect(mgmtClient.Create(ctx, input.Node.InfraMachine)).NotTo(HaveOccurred())

	By("Creating bootstrap config resource")
	Expect(mgmtClient.Create(ctx, input.Node.BootstrapConfig)).NotTo(HaveOccurred())

	By("Creating machine resource that owns infra machine and its bootstrap")
	Expect(mgmtClient.Create(ctx, input.Node.Machine)).NotTo(HaveOccurred())

	By("Waiting for nodes to reach the Running phase")
	Eventually(func() string {
		machine := &clusterv1.Machine{}
		key := client.ObjectKey{
			Namespace: input.Node.Machine.GetNamespace(),
			Name:      input.Node.Machine.GetName(),
		}
		if err := mgmtClient.Get(ctx, key, machine); err != nil {
			return err.Error()
		}
		return machine.Status.Phase
	}, input.CreateTimeout, 20*time.Second).Should(Equal(string(clusterv1.MachinePhaseRunning)))

	Eventually(func() []v1.Node {
		nodes := v1.NodeList{}
		err := wait.PollImmediate(10*time.Second, 5*time.Minute, func() (bool, error) {
			_, err := input.Management.GetWorkloadClient(ctx, input.Cluster.Namespace, input.Cluster.Name)
			switch {
			case apierrors.IsNotFound(err):
				return false, nil
			default:
				return true, nil
			}
		})
		Expect(err).NotTo(HaveOccurred())
		workloadClient, err := input.Management.GetWorkloadClient(ctx, input.Cluster.Namespace, input.Cluster.Name)
		Expect(err).NotTo(HaveOccurred(), "Stack:\n%+v\n", err)
		Expect(workloadClient.List(ctx, &nodes)).NotTo(HaveOccurred())
		return nodes.Items
	}, input.CreateTimeout, 10*time.Second).Should(HaveLen(1))
}

// +build e2e

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

package e2e

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/exec"
	"sigs.k8s.io/cluster-api/test/framework/management/kind"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	eventuallyInterval = 10 * time.Second
)

// ControlPlaneClusterInput are all the dependencies of the ControlPlaneCluster test.
type ControlPlaneClusterInput struct {
	Management        *kind.Cluster
	Cluster           *clusterv1.Cluster
	InfraCluster      runtime.Object
	Nodes             []framework.Node
	MachineDeployment framework.MachineDeployment
	ControlPlane      *controlplanev1.KubeadmControlPlane
	MachineTemplate   runtime.Object
	CreateTimeout     time.Duration
	DeleteTimeout     time.Duration
}

// SetDefaults defaults the struct fields if necessary.
func (o *ControlPlaneClusterInput) SetDefaults() {
	if o.CreateTimeout == 0*time.Second {
		o.CreateTimeout = 60 * time.Minute
	}

	if o.DeleteTimeout == 0 {
		o.DeleteTimeout = 30 * time.Minute
	}
}

// ControlPlaneCluster creates a single control plane node.
// Assertions:
//   * The created cluster has exactly three nodes.
//   * The created machines reach the 'running' state.
func ControlPlaneCluster(input *ControlPlaneClusterInput) {
	input.SetDefaults()
	ctx := context.Background()
	Expect(input.Management).ToNot(BeNil())

	mgmtClient, err := input.Management.GetClient()
	Expect(err).NotTo(HaveOccurred(), "stack: %+v", err)

	By("Creating infra cluster resource")
	Expect(mgmtClient.Create(ctx, input.InfraCluster)).NotTo(HaveOccurred())

	// This call happens in an eventually because of a race condition with the
	// webhook server. If the latter isn't fully online then this call will
	// fail.
	By("Creating cluster resource that owns the infra cluster")
	Eventually(func() error {
		return mgmtClient.Create(ctx, input.Cluster)
	}, input.CreateTimeout, eventuallyInterval).Should(Succeed())

	By("creating the machine template")
	Expect(mgmtClient.Create(ctx, input.MachineTemplate)).To(Succeed())

	By("creating a KubeadmControlPlane")
	Eventually(func() error {
		err := mgmtClient.Create(ctx, input.ControlPlane)
		if err != nil {
			fmt.Println(err)
		}
		return err
	}, input.CreateTimeout, 10*time.Second).Should(BeNil())

	// expectedNumberOfNodes is the number of nodes that should be deployed to
	// the cluster. This is the control plane nodes plus the number of replicas
	// defined for a possible MachineDeployment.
	expectedNumberOfNodes := int(*input.ControlPlane.Spec.Replicas)

	// Create the control plane nodes.
	for i, node := range input.Nodes {
		By(fmt.Sprintf("creating %d control plane node's InfrastructureMachine resource", i+1))
		Expect(mgmtClient.Create(ctx, node.InfraMachine)).To(Succeed())

		By(fmt.Sprintf("creating %d control plane node's BootstrapConfig resource", i+1))
		Expect(mgmtClient.Create(ctx, node.BootstrapConfig)).To(Succeed())

		By(fmt.Sprintf("creating %d control plane node's Machine resource with a linked InfrastructureMachine and BootstrapConfig", i+1))
		Expect(mgmtClient.Create(ctx, node.Machine)).To(Succeed())
	}

	By("waiting for cluster to enter the provisioned phase")
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

	// Create the machine deployment if the replica count >0.
	if machineDeployment := input.MachineDeployment.MachineDeployment; machineDeployment != nil {
		if replicas := machineDeployment.Spec.Replicas; replicas != nil && *replicas > 0 {
			expectedNumberOfNodes += int(*replicas)

			By("creating a core MachineDeployment resource")
			Expect(mgmtClient.Create(ctx, machineDeployment)).To(Succeed())

			By("creating a BootstrapConfigTemplate resource")
			Expect(mgmtClient.Create(ctx, input.MachineDeployment.BootstrapConfigTemplate)).To(Succeed())

			By("creating an InfrastructureMachineTemplate resource")
			Expect(mgmtClient.Create(ctx, input.MachineDeployment.InfraMachineTemplate)).To(Succeed())
		}
	}

	By("waiting for workload node(s) to exist")
	Eventually(func() ([]v1.Node, error) {
		workloadClient, err := input.Management.GetWorkloadClient(ctx, input.Cluster.Namespace, input.Cluster.Name)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get workload client")
		}
		nodeList := v1.NodeList{}
		if err := workloadClient.List(ctx, &nodeList); err != nil {
			return nil, err
		}
		return nodeList.Items, nil
	}, input.CreateTimeout, 10*time.Second).Should(HaveLen(expectedNumberOfNodes))

	By("deploy a CNI soution, Calico")
	config := &v1.Secret{}
	key := client.ObjectKey{
		Name:      fmt.Sprintf("%s-kubeconfig", input.Cluster.Name),
		Namespace: input.Cluster.Namespace,
	}
	if err := mgmtClient.Get(ctx, key, config); err != nil {
		Expect(err).NotTo(HaveOccurred(), "stack: %+v", err)
	}

	tmpdir, err := ioutil.TempDir("", "")
	f, err := ioutil.TempFile(tmpdir, "worker-kubeconfig")
	if err != nil {
		Expect(err).NotTo(HaveOccurred(), "stack: %+v", err)
	}
	defer os.RemoveAll(tmpdir)
	data := config.Data["value"]
	if _, err := f.Write(data); err != nil {
		Expect(err).NotTo(HaveOccurred(), "stack: %+v", err)
	}
	calicoManifestPath := "../../templates/addons/calico.yaml"
	applyCmd := exec.NewCommand(
		exec.WithCommand("kubectl"),
		exec.WithArgs("apply", "--kubeconfig", f.Name(), "-f", calicoManifestPath),
	)

	Eventually(func() error {
		_, _, err = applyCmd.Run(ctx)
		return err
	}, 5*time.Minute, 10*time.Second).Should(BeNil())

	By("waiting for all machines to be running")
	inClustersNamespaceListOption := client.InNamespace(input.Cluster.Namespace)
	matchClusterListOption := client.MatchingLabels{clusterv1.ClusterLabelName: input.Cluster.Name}
	Eventually(func() (bool, error) {
		// Get a list of all the Machine resources that belong to the Cluster.
		machineList := &clusterv1.MachineList{}
		if err := mgmtClient.List(ctx, machineList, inClustersNamespaceListOption, matchClusterListOption); err != nil {
			return false, err
		}
		if len(machineList.Items) != expectedNumberOfNodes {
			return false, errors.Errorf("number of Machines %d != expected number of nodes %d", len(machineList.Items), expectedNumberOfNodes)
		}
		for _, machine := range machineList.Items {
			if machine.Status.Phase != string(clusterv1.MachinePhaseRunning) {
				return false, errors.Errorf("machine %s is not running, it's %s", machine.Name, machine.Status.Phase)
			}
		}
		return true, nil
	}, input.CreateTimeout, 10*time.Second).Should(BeTrue())
}

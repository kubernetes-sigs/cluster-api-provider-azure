// +build e2e

/*
Copyright 2021 The Kubernetes Authors.

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

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	infraexpv1 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha4"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WaitForControlPlaneInitialized waits for the azure managed control plane to be initialized.
// This will be invoked by cluster api e2e framework.
func WaitForControlPlaneInitialized(ctx context.Context, input clusterctl.ApplyClusterTemplateAndWaitInput, result *clusterctl.ApplyClusterTemplateAndWaitResult) {
	client := input.ClusterProxy.GetClient()
	DiscoverAndWaitForControlPlaneInitialized(ctx, DiscoverAndWaitForControlPlaneMachinesInput{
		Lister:  client,
		Getter:  client,
		Cluster: result.Cluster,
	}, input.WaitForControlPlaneIntervals...)
}

// WaitForControlPlaneMachinesReady waits for the azure managed control plane to be ready.
// This will be invoked by cluster api e2e framework.
func WaitForControlPlaneMachinesReady(ctx context.Context, input clusterctl.ApplyClusterTemplateAndWaitInput, result *clusterctl.ApplyClusterTemplateAndWaitResult) {
	client := input.ClusterProxy.GetClient()
	DiscoverAndWaitForControlPlaneReady(ctx, DiscoverAndWaitForControlPlaneMachinesInput{
		Lister:  client,
		Getter:  client,
		Cluster: result.Cluster,
	}, input.WaitForControlPlaneIntervals...)
}

// DiscoverAndWaitForControlPlaneMachinesInput contains the fields the required for checking the status of azure managed control plane.
type DiscoverAndWaitForControlPlaneMachinesInput struct {
	Lister  framework.Lister
	Getter  framework.Getter
	Cluster *clusterv1.Cluster
}

// DiscoverAndWaitForControlPlaneInitialized gets the azure managed control plane associated with the cluster,
// and waits for atleast one control plane machine to be up.
func DiscoverAndWaitForControlPlaneInitialized(ctx context.Context, input DiscoverAndWaitForControlPlaneMachinesInput, intervals ...interface{}) {
	gomega.Expect(ctx).NotTo(gomega.BeNil(), "ctx is required for DiscoverAndWaitForControlPlaneInitialized")
	gomega.Expect(input.Lister).ToNot(gomega.BeNil(), "Invalid argument. input.Lister can't be nil when calling DiscoverAndWaitForControlPlaneInitialized")
	gomega.Expect(input.Cluster).ToNot(gomega.BeNil(), "Invalid argument. input.Cluster can't be nil when calling DiscoverAndWaitForControlPlaneInitialized")

	controlPlane := GetAzureManagedControlPlaneByCluster(ctx, GetAzureManagedControlPlaneByClusterInput{
		Lister:      input.Lister,
		ClusterName: input.Cluster.Name,
		Namespace:   input.Cluster.Namespace,
	})
	gomega.Expect(controlPlane).ToNot(gomega.BeNil())

	Logf("Waiting for the first control plane machine managed by %s/%s to be provisioned", controlPlane.Namespace, controlPlane.Name)
	WaitForAtLeastOneControlPlaneAndMachineToExist(ctx, WaitForControlPlaneAndMachinesReadyInput{
		Getter:       input.Getter,
		ControlPlane: controlPlane,
		ClusterName:  input.Cluster.Name,
		Namespace:    input.Cluster.Namespace,
	}, intervals...)
}

// DiscoverAndWaitForControlPlaneReady gets the azure managed control plane associated with the cluster,
// and waits for all the control plane machines to be up.
func DiscoverAndWaitForControlPlaneReady(ctx context.Context, input DiscoverAndWaitForControlPlaneMachinesInput, intervals ...interface{}) {
	gomega.Expect(ctx).NotTo(gomega.BeNil(), "ctx is required for DiscoverAndWaitForControlPlaneReady")
	gomega.Expect(input.Lister).ToNot(gomega.BeNil(), "Invalid argument. input.Lister can't be nil when calling DiscoverAndWaitForControlPlaneReady")
	gomega.Expect(input.Cluster).ToNot(gomega.BeNil(), "Invalid argument. input.Cluster can't be nil when calling DiscoverAndWaitForControlPlaneReady")

	controlPlane := GetAzureManagedControlPlaneByCluster(ctx, GetAzureManagedControlPlaneByClusterInput{
		Lister:      input.Lister,
		ClusterName: input.Cluster.Name,
		Namespace:   input.Cluster.Namespace,
	})
	gomega.Expect(controlPlane).ToNot(gomega.BeNil())

	Logf("Waiting for the first control plane machine managed by %s/%s to be provisioned", controlPlane.Namespace, controlPlane.Name)
	WaitForAllControlPlaneAndMachinesToExist(ctx, WaitForControlPlaneAndMachinesReadyInput{
		Getter:       input.Getter,
		ControlPlane: controlPlane,
		ClusterName:  input.Cluster.Name,
		Namespace:    input.Cluster.Namespace,
	}, intervals...)
}

// GetAzureManagedControlPlaneByClusterInput contains the fields the required for fetching the azure managed control plane.
type GetAzureManagedControlPlaneByClusterInput struct {
	Lister      framework.Lister
	ClusterName string
	Namespace   string
}

// GetAzureManagedControlPlaneByCluster returns the AzureManagedControlPlane object for a cluster.
// Important! this method relies on labels that are created by the CAPI controllers during the first reconciliation, so
// it is necessary to ensure this is already happened before calling it.
func GetAzureManagedControlPlaneByCluster(ctx context.Context, input GetAzureManagedControlPlaneByClusterInput) *infraexpv1.AzureManagedControlPlane {
	controlPlaneList := &infraexpv1.AzureManagedControlPlaneList{}
	gomega.Expect(input.Lister.List(ctx, controlPlaneList, byClusterOptions(input.ClusterName, input.Namespace)...)).To(gomega.Succeed(), "Failed to list AzureManagedControlPlane object for Cluster %s/%s", input.Namespace, input.ClusterName)
	gomega.Expect(len(controlPlaneList.Items)).ToNot(gomega.BeNumerically(">", 1), "Cluster %s/%s should not have more than 1 AzureManagedControlPlane object", input.Namespace, input.ClusterName)
	if len(controlPlaneList.Items) == 1 {
		return &controlPlaneList.Items[0]
	}
	return nil
}

// WaitForControlPlaneAndMachinesReadyInput contains the fields required for checking the status of azure managed control plane machines.
type WaitForControlPlaneAndMachinesReadyInput struct {
	Getter       framework.Getter
	ControlPlane *infraexpv1.AzureManagedControlPlane
	ClusterName  string
	Namespace    string
}

// WaitForAtLeastOneControlPlaneAndMachineToExist waits for atleast one control plane machine to be provisioned.
func WaitForAtLeastOneControlPlaneAndMachineToExist(ctx context.Context, input WaitForControlPlaneAndMachinesReadyInput, intervals ...interface{}) {
	ginkgo.By("Waiting for atleast one control plane node to exist")
	WaitForControlPlaneMachinesToExist(ctx, input, atLeastOne, intervals...)
}

// WaitForAllControlPlaneAndMachinesToExist waits for all control plane machines to be provisioned.
func WaitForAllControlPlaneAndMachinesToExist(ctx context.Context, input WaitForControlPlaneAndMachinesReadyInput, intervals ...interface{}) {
	ginkgo.By("Waiting for all control plane nodes to exist")
	WaitForControlPlaneMachinesToExist(ctx, input, all, intervals...)
}

// controlPlaneReplicas represents the count of control plane machines.
type controlPlaneReplicas string

const (
	atLeastOne controlPlaneReplicas = "atLeastOne"
	all        controlPlaneReplicas = "all"
)

// value returns the integer equivalent of controlPlaneReplicas
func (r controlPlaneReplicas) value(mp *clusterv1exp.MachinePool) int {
	switch r {
	case atLeastOne:
		return 1
	case all:
		return int(*mp.Spec.Replicas)
	}
	return 0
}

// WaitForControlPlaneMachinesToExist waits for a certain number of control plane machines to be provisioned represented.
func WaitForControlPlaneMachinesToExist(ctx context.Context, input WaitForControlPlaneAndMachinesReadyInput, minReplicas controlPlaneReplicas, intervals ...interface{}) {
	gomega.Eventually(func() (bool, error) {
		controlPlaneMachinePool := &clusterv1exp.MachinePool{}
		if err := input.Getter.Get(ctx, types.NamespacedName{Namespace: input.Namespace, Name: input.ControlPlane.Spec.DefaultPoolRef.Name},
			controlPlaneMachinePool); err != nil {
			Logf("Failed to get machinePool: %+v", err)
			return false, err
		}
		return len(controlPlaneMachinePool.Status.NodeRefs) >= minReplicas.value(controlPlaneMachinePool), nil

	}, intervals...).Should(gomega.Equal(true))
}

// byClusterOptions returns a set of ListOptions that allows to identify all the objects belonging to a Cluster.
func byClusterOptions(name, namespace string) []client.ListOption {
	return []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels{
			clusterv1.ClusterLabelName: name,
		},
	}
}

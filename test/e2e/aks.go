//go:build e2e
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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DiscoverAndWaitForAKSControlPlaneInput contains the fields the required for checking the status of azure managed control plane.
type DiscoverAndWaitForAKSControlPlaneInput struct {
	Lister  framework.Lister
	Getter  framework.Getter
	Cluster *clusterv1.Cluster
}

// WaitForAKSControlPlaneInitialized waits for the Azure managed control plane to be initialized.
// This will be invoked by cluster api e2e framework.
func WaitForAKSControlPlaneInitialized(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) {
	client := input.ClusterProxy.GetClient()
	cluster := framework.GetClusterByName(ctx, framework.GetClusterByNameInput{
		Getter:    client,
		Name:      input.ClusterName,
		Namespace: input.Namespace,
	})

	DiscoverAndWaitForAKSControlPlaneInitialized(ctx, DiscoverAndWaitForAKSControlPlaneInput{
		Lister:  client,
		Getter:  client,
		Cluster: result.Cluster,
	}, input.WaitForControlPlaneIntervals...)
	InstallCNIManifest(ctx, input, cluster.Spec.ClusterNetwork.Services.CIDRBlocks, true)
}

// WaitForAKSControlPlaneReady waits for the azure managed control plane to be ready.
// This will be invoked by cluster api e2e framework.
func WaitForAKSControlPlaneReady(ctx context.Context, input clusterctl.ApplyCustomClusterTemplateAndWaitInput, result *clusterctl.ApplyCustomClusterTemplateAndWaitResult) {
	client := input.ClusterProxy.GetClient()
	DiscoverAndWaitForAKSControlPlaneReady(ctx, DiscoverAndWaitForAKSControlPlaneInput{
		Lister:  client,
		Getter:  client,
		Cluster: result.Cluster,
	}, input.WaitForControlPlaneIntervals...)
}

// DiscoverAndWaitForAKSControlPlaneInitialized gets the Azure managed control plane associated with the cluster
// and waits for at least one machine in the "system" node pool to exist.
func DiscoverAndWaitForAKSControlPlaneInitialized(ctx context.Context, input DiscoverAndWaitForAKSControlPlaneInput, intervals ...interface{}) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for DiscoverAndWaitForAKSControlPlaneInitialized")
	Expect(input.Lister).NotTo(BeNil(), "Invalid argument. input.Lister can't be nil when calling DiscoverAndWaitForAKSControlPlaneInitialized")
	Expect(input.Cluster).NotTo(BeNil(), "Invalid argument. input.Cluster can't be nil when calling DiscoverAndWaitForAKSControlPlaneInitialized")

	controlPlane := GetAzureManagedControlPlaneByCluster(ctx, GetAzureManagedControlPlaneByClusterInput{
		Lister:      input.Lister,
		ClusterName: input.Cluster.Name,
		Namespace:   input.Cluster.Namespace,
	})
	Expect(controlPlane).NotTo(BeNil())

	Logf("Waiting for the first AKS machine in the %s/%s 'system' node pool to exist", controlPlane.Namespace, controlPlane.Name)
	WaitForAtLeastOneSystemNodePoolMachineToExist(ctx, WaitForControlPlaneAndMachinesReadyInput{
		Lister:       input.Lister,
		Getter:       input.Getter,
		ControlPlane: controlPlane,
		ClusterName:  input.Cluster.Name,
		Namespace:    input.Cluster.Namespace,
	}, intervals...)
}

// DiscoverAndWaitForAKSControlPlaneReady gets the Azure managed control plane associated with the cluster
// and waits for all the machines in the 'system' node pool to exist.
func DiscoverAndWaitForAKSControlPlaneReady(ctx context.Context, input DiscoverAndWaitForAKSControlPlaneInput, intervals ...interface{}) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for DiscoverAndWaitForAKSControlPlaneReady")
	Expect(input.Lister).NotTo(BeNil(), "Invalid argument. input.Lister can't be nil when calling DiscoverAndWaitForAKSControlPlaneReady")
	Expect(input.Cluster).NotTo(BeNil(), "Invalid argument. input.Cluster can't be nil when calling DiscoverAndWaitForAKSControlPlaneReady")

	controlPlane := GetAzureManagedControlPlaneByCluster(ctx, GetAzureManagedControlPlaneByClusterInput{
		Lister:      input.Lister,
		ClusterName: input.Cluster.Name,
		Namespace:   input.Cluster.Namespace,
	})
	Expect(controlPlane).NotTo(BeNil())

	Logf("Waiting for all AKS machines in the %s/%s 'system' node pool to exist", controlPlane.Namespace, controlPlane.Name)
	WaitForAllControlPlaneAndMachinesToExist(ctx, WaitForControlPlaneAndMachinesReadyInput{
		Lister:       input.Lister,
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
func GetAzureManagedControlPlaneByCluster(ctx context.Context, input GetAzureManagedControlPlaneByClusterInput) *infrav1.AzureManagedControlPlane {
	controlPlaneList := &infrav1.AzureManagedControlPlaneList{}
	Expect(input.Lister.List(ctx, controlPlaneList, byClusterOptions(input.ClusterName, input.Namespace)...)).To(Succeed(), "Failed to list AzureManagedControlPlane object for Cluster %s/%s", input.Namespace, input.ClusterName)
	Expect(len(controlPlaneList.Items)).NotTo(BeNumerically(">", 1), "Cluster %s/%s should not have more than 1 AzureManagedControlPlane object", input.Namespace, input.ClusterName)
	if len(controlPlaneList.Items) == 1 {
		return &controlPlaneList.Items[0]
	}
	return nil
}

// WaitForControlPlaneAndMachinesReadyInput contains the fields required for checking the status of azure managed control plane machines.
type WaitForControlPlaneAndMachinesReadyInput struct {
	Lister       framework.Lister
	Getter       framework.Getter
	ControlPlane *infrav1.AzureManagedControlPlane
	ClusterName  string
	Namespace    string
}

// WaitForAtLeastOneSystemNodePoolMachineToExist waits for at least one machine in the "system" node pool to exist.
func WaitForAtLeastOneSystemNodePoolMachineToExist(ctx context.Context, input WaitForControlPlaneAndMachinesReadyInput, intervals ...interface{}) {
	By("Waiting for at least one node to exist in the 'system' node pool")
	WaitForAKSSystemNodePoolMachinesToExist(ctx, input, atLeastOne, intervals...)
}

// WaitForAllControlPlaneAndMachinesToExist waits for all machines in the "system" node pool to exist.
func WaitForAllControlPlaneAndMachinesToExist(ctx context.Context, input WaitForControlPlaneAndMachinesReadyInput, intervals ...interface{}) {
	By("Waiting for all nodes to exist in the 'system' node pool")
	WaitForAKSSystemNodePoolMachinesToExist(ctx, input, all, intervals...)
}

// controlPlaneReplicas represents the count of control plane machines.
type controlPlaneReplicas string

const (
	atLeastOne controlPlaneReplicas = "atLeastOne"
	all        controlPlaneReplicas = "all"
)

// value returns the integer equivalent of controlPlaneReplicas
func (r controlPlaneReplicas) value(mp *expv1.MachinePool) int {
	switch r {
	case atLeastOne:
		return 1
	case all:
		return int(*mp.Spec.Replicas)
	}
	return 0
}

// WaitForAKSSystemNodePoolMachinesToExist waits for a certain number of machines in the "system" node pool to exist.
func WaitForAKSSystemNodePoolMachinesToExist(ctx context.Context, input WaitForControlPlaneAndMachinesReadyInput, minReplicas controlPlaneReplicas, intervals ...interface{}) {
	Eventually(func() bool {
		opt1 := client.InNamespace(input.Namespace)
		opt2 := client.MatchingLabels(map[string]string{
			infrav1.LabelAgentPoolMode: string(infrav1.NodePoolModeSystem),
			clusterv1.ClusterNameLabel: input.ClusterName,
		})

		ammpList := &infrav1.AzureManagedMachinePoolList{}

		if err := input.Lister.List(ctx, ammpList, opt1, opt2); err != nil {
			LogWarningf("Failed to get machinePool: %+v", err)
			return false
		}

		for _, pool := range ammpList.Items {
			// Fetch the owning MachinePool.
			for _, ref := range pool.OwnerReferences {
				if ref.Kind != "MachinePool" {
					continue
				}

				ownerMachinePool := &expv1.MachinePool{}
				if err := input.Getter.Get(ctx, types.NamespacedName{Namespace: input.Namespace, Name: ref.Name},
					ownerMachinePool); err != nil {
					LogWarningf("Failed to get machinePool: %+v", err)
					return false
				}
				if len(ownerMachinePool.Status.NodeRefs) >= minReplicas.value(ownerMachinePool) {
					return true
				}
			}
		}

		return false
	}, intervals...).Should(Equal(true), "System machine pools not detected")
}

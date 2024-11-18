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

	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231001"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1alpha "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
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
	DiscoverAndWaitForAKSControlPlaneInitialized(ctx, DiscoverAndWaitForAKSControlPlaneInput{
		Lister:  client,
		Getter:  client,
		Cluster: result.Cluster,
	}, input.WaitForControlPlaneIntervals...)
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

	controlPlaneNamespace := input.Cluster.Spec.ControlPlaneRef.Namespace
	controlPlaneName := input.Cluster.Spec.ControlPlaneRef.Name

	Logf("Waiting for the first AKS machine in the %s/%s 'system' node pool to exist", controlPlaneNamespace, controlPlaneName)
	WaitForAtLeastOneSystemNodePoolMachineToExist(ctx, WaitForControlPlaneAndMachinesReadyInput{
		Lister:      input.Lister,
		Getter:      input.Getter,
		ClusterName: input.Cluster.Name,
		Namespace:   input.Cluster.Namespace,
	}, intervals...)
}

// DiscoverAndWaitForAKSControlPlaneReady gets the Azure managed control plane associated with the cluster
// and waits for all the machines in the 'system' node pool to exist.
func DiscoverAndWaitForAKSControlPlaneReady(ctx context.Context, input DiscoverAndWaitForAKSControlPlaneInput, intervals ...interface{}) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for DiscoverAndWaitForAKSControlPlaneReady")
	Expect(input.Lister).NotTo(BeNil(), "Invalid argument. input.Lister can't be nil when calling DiscoverAndWaitForAKSControlPlaneReady")
	Expect(input.Cluster).NotTo(BeNil(), "Invalid argument. input.Cluster can't be nil when calling DiscoverAndWaitForAKSControlPlaneReady")

	controlPlaneNamespace := input.Cluster.Spec.ControlPlaneRef.Namespace
	controlPlaneName := input.Cluster.Spec.ControlPlaneRef.Name

	Logf("Waiting for all AKS machines in the %s/%s 'system' node pool to exist", controlPlaneNamespace, controlPlaneName)
	WaitForAllControlPlaneAndMachinesToExist(ctx, WaitForControlPlaneAndMachinesReadyInput{
		Lister:      input.Lister,
		Getter:      input.Getter,
		ClusterName: input.Cluster.Name,
		Namespace:   input.Cluster.Namespace,
	}, intervals...)
}

// WaitForControlPlaneAndMachinesReadyInput contains the fields required for checking the status of azure managed control plane machines.
type WaitForControlPlaneAndMachinesReadyInput struct {
	Lister      framework.Lister
	Getter      framework.Getter
	ClusterName string
	Namespace   string
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
			clusterv1.ClusterNameLabel: input.ClusterName,
		})
		opt3 := client.MatchingLabels(map[string]string{
			infrav1.LabelAgentPoolMode: string(infrav1.NodePoolModeSystem),
		})

		var capzMPs []client.Object

		ammpList := &infrav1.AzureManagedMachinePoolList{}
		asommpList := &infrav1alpha.AzureASOManagedMachinePoolList{}

		if err := input.Lister.List(ctx, ammpList, opt1, opt2, opt3); err != nil {
			LogWarningf("Failed to list AzureManagedMachinePools: %+v", err)
			return false
		}
		for _, ammp := range ammpList.Items {
			capzMPs = append(capzMPs, ptr.To(ammp))
		}

		if err := input.Lister.List(ctx, asommpList, opt1, opt2); err != nil {
			LogWarningf("Failed to list AzureASOManagedMachinePools: %+v", err)
			return false
		}
		for _, asommp := range asommpList.Items {
			var resources []*unstructured.Unstructured
			for _, resource := range asommp.Spec.Resources {
				u := &unstructured.Unstructured{}
				Expect(u.UnmarshalJSON(resource.Raw)).To(Succeed())
				resources = append(resources, u)
			}
			for _, resource := range resources {
				if resource.GroupVersionKind().Group != asocontainerservicev1.GroupVersion.Group ||
					resource.GroupVersionKind().Kind != "ManagedClustersAgentPool" {
					continue
				}
				mode, _, err := unstructured.NestedString(resource.UnstructuredContent(), "spec", "mode")
				if err != nil {
					LogWarningf("Failed to get spec.mode for AzureASOManagedMachinePools %s/%s: %v", asommp.Namespace, asommp.Name, err)
					continue
				}
				if mode == string(asocontainerservicev1.AgentPoolMode_System) {
					capzMPs = append(capzMPs, ptr.To(asommp))
				}
				break
			}
		}

		for _, pool := range capzMPs {
			// Fetch the owning MachinePool.
			for _, ref := range pool.GetOwnerReferences() {
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
	}, intervals...).Should(BeTrue(), "System machine pools not detected")
}

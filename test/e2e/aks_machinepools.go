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
	"strings"
	"time"

	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231001"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/mutators"
)

type AKSMachinePoolSpecInput struct {
	MgmtCluster   framework.ClusterProxy
	Cluster       *clusterv1.Cluster
	MachinePools  []*clusterv1.MachinePool
	WaitIntervals []interface{}
}

func AKSMachinePoolSpec(ctx context.Context, inputGetter func() AKSMachinePoolSpecInput) {
	input := inputGetter()

	mgmtClient := input.MgmtCluster.GetClient()

	patchMachinePoolReplicas := func(mp *clusterv1.MachinePool, replicas int32) {
		GinkgoHelper()

		patchHelper, err := patch.NewHelper(mp, input.MgmtCluster.GetClient())
		Expect(err).NotTo(HaveOccurred())

		mp.Spec.Replicas = &replicas
		Eventually(func(ctx context.Context) error {
			return patchHelper.Patch(ctx, mp)
		}, 3*time.Minute, 10*time.Second).WithContext(ctx).Should(Succeed())
	}

	// separate list from input.MachinePools to avoid side-effects
	machinepools := make([]*clusterv1.MachinePool, len(input.MachinePools))
	for i, mp := range input.MachinePools {
		machinepools[i] = mp.DeepCopy()
	}

	// [framework.ScaleMachinePoolAndWait] wraps a similar "change replica count
	// + wait" sequence. The difference is that here we bump the replica count
	// by a relative amount vs. setting an absolute replica count. This way we
	// make sure we're actually changing the replica count in the direction we
	// want without having to care about the initial state of each individual
	// MachinePool.

	// MachinePool name -> replicas. We're only dealing in one namespace.
	originalReplicas := map[string]int32{}

	// Scale out
	for _, mp := range machinepools {
		originalReplicas[mp.Name] = ptr.Deref(mp.Spec.Replicas, 0)

		goalReplicas := ptr.Deref(mp.Spec.Replicas, 0) + 1
		Byf("Scaling machine pool %s out from %d to %d", mp.Name, *mp.Spec.Replicas, goalReplicas)
		patchMachinePoolReplicas(mp, goalReplicas)
	}
	for _, mp := range machinepools {
		framework.WaitForMachinePoolNodesToExist(ctx, framework.WaitForMachinePoolNodesToExistInput{
			Getter:      mgmtClient,
			MachinePool: mp,
		}, input.WaitIntervals...)
	}

	// Scale in
	for _, mp := range machinepools {
		goalReplicas := ptr.Deref(mp.Spec.Replicas, 0) - 1
		Byf("Scaling machine pool %s in from %d to %d", mp.Name, *mp.Spec.Replicas, goalReplicas)
		patchMachinePoolReplicas(mp, goalReplicas)
	}
	for _, mp := range machinepools {
		framework.WaitForMachinePoolNodesToExist(ctx, framework.WaitForMachinePoolNodesToExistInput{
			Getter:      mgmtClient,
			MachinePool: mp,
		}, input.WaitIntervals...)
	}

	var userPools []*clusterv1.MachinePool
	var userPoolNames []string
	for _, mp := range machinepools {
		// System node pools cannot be scaled to 0, so only include user node pools.
		switch mp.Spec.Template.Spec.InfrastructureRef.Kind {
		case infrav1.AzureManagedMachinePoolKind:
			ammp := &infrav1.AzureManagedMachinePool{}
			err := input.MgmtCluster.GetClient().Get(ctx, types.NamespacedName{
				Namespace: mp.Namespace,
				Name:      mp.Spec.Template.Spec.InfrastructureRef.Name,
			}, ammp)
			Expect(err).NotTo(HaveOccurred())

			if ammp.Spec.Mode != string(infrav1.NodePoolModeSystem) {
				userPools = append(userPools, mp)
				userPoolNames = append(userPoolNames, mp.Name)
			}
		case infrav1.AzureASOManagedMachinePoolKind:
			ammp := &infrav1.AzureASOManagedMachinePool{}
			err := input.MgmtCluster.GetClient().Get(ctx, types.NamespacedName{
				Namespace: mp.Namespace,
				Name:      mp.Spec.Template.Spec.InfrastructureRef.Name,
			}, ammp)
			Expect(err).NotTo(HaveOccurred())

			resources, err := mutators.ToUnstructured(ctx, ammp.Spec.Resources)
			Expect(err).NotTo(HaveOccurred())
			for _, resource := range resources {
				if resource.GetKind() != "ManagedClustersAgentPool" {
					continue
				}
				// mode may not be set in spec. Get the ASO object and check in status.
				resource.SetNamespace(ammp.Namespace)
				agentPool := &asocontainerservicev1.ManagedClustersAgentPool{}
				Expect(input.MgmtCluster.GetClient().Get(ctx, client.ObjectKeyFromObject(resource), agentPool)).To(Succeed())
				if ptr.Deref(agentPool.Status.Mode, "") != asocontainerservicev1.AgentPoolMode_STATUS_System {
					userPools = append(userPools, mp)
					userPoolNames = append(userPoolNames, mp.Name)
				}
				break
			}
		}
	}

	// ScaleMachinePoolAndWait can be used here since all MachinePools are
	// targeting the same number of replicas.
	Byf("Scaling the User mode machine pools %s to zero", strings.Join(userPoolNames, ", "))
	framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
		ClusterProxy:              input.MgmtCluster,
		Cluster:                   input.Cluster,
		Replicas:                  0,
		MachinePools:              userPools,
		WaitForMachinePoolToScale: input.WaitIntervals,
	})

	// Reset replica count
	for _, mp := range machinepools {
		goalReplicas := originalReplicas[mp.Name]
		Byf("Scaling machine pool %s to original count from %d to %d", mp.Name, *mp.Spec.Replicas, goalReplicas)
		patchMachinePoolReplicas(mp, goalReplicas)
	}
	for _, mp := range machinepools {
		framework.WaitForMachinePoolNodesToExist(ctx, framework.WaitForMachinePoolNodesToExistInput{
			Getter:      mgmtClient,
			MachinePool: mp,
		}, input.WaitIntervals...)
	}
}

type AKSMachinePoolPostUpgradeSpecInput struct {
	MgmtCluster      framework.ClusterProxy
	ClusterName      string
	ClusterNamespace string
}

func AKSMachinePoolPostUpgradeSpec(ctx context.Context, inputGetter func() AKSMachinePoolPostUpgradeSpecInput) {
	input := inputGetter()

	cluster := framework.GetClusterByName(ctx, framework.GetClusterByNameInput{
		Getter:    input.MgmtCluster.GetClient(),
		Name:      input.ClusterName,
		Namespace: input.ClusterNamespace,
	})
	mps := framework.GetMachinePoolsByCluster(ctx, framework.GetMachinePoolsByClusterInput{
		Lister:      input.MgmtCluster.GetClient(),
		ClusterName: input.ClusterName,
		Namespace:   input.ClusterNamespace,
	})
	AKSMachinePoolSpec(ctx, func() AKSMachinePoolSpecInput {
		return AKSMachinePoolSpecInput{
			MgmtCluster:   input.MgmtCluster,
			Cluster:       cluster,
			MachinePools:  mps,
			WaitIntervals: e2eConfig.GetIntervals("default", "wait-machine-pool-nodes"),
		}
	})
}

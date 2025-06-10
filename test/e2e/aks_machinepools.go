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
	"sync"

	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231001"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/mutators"
)

type AKSMachinePoolSpecInput struct {
	Cluster       *clusterv1.Cluster
	MachinePools  []*expv1.MachinePool
	WaitIntervals []interface{}
}

func AKSMachinePoolSpec(ctx context.Context, inputGetter func() AKSMachinePoolSpecInput) {
	input := inputGetter()
	var wg sync.WaitGroup

	for _, mp := range input.MachinePools {
		wg.Add(1)
		go func(mp *expv1.MachinePool) {
			defer GinkgoRecover()
			defer wg.Done()

			originalReplicas := ptr.Deref(mp.Spec.Replicas, 0)

			Byf("Scaling machine pool %s out", mp.Name)
			framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
				ClusterProxy:              bootstrapClusterProxy,
				Cluster:                   input.Cluster,
				Replicas:                  ptr.Deref(mp.Spec.Replicas, 0) + 1,
				MachinePools:              []*expv1.MachinePool{mp},
				WaitForMachinePoolToScale: input.WaitIntervals,
			})

			Byf("Scaling machine pool %s in", mp.Name)
			framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
				ClusterProxy:              bootstrapClusterProxy,
				Cluster:                   input.Cluster,
				Replicas:                  ptr.Deref(mp.Spec.Replicas, 0) - 1,
				MachinePools:              []*expv1.MachinePool{mp},
				WaitForMachinePoolToScale: input.WaitIntervals,
			})

			// System node pools cannot be scaled to 0, so only include user node pools.
			isUserPool := false
			switch mp.Spec.Template.Spec.InfrastructureRef.Kind {
			case infrav1.AzureManagedMachinePoolKind:
				ammp := &infrav1.AzureManagedMachinePool{}
				err := bootstrapClusterProxy.GetClient().Get(ctx, types.NamespacedName{
					Namespace: mp.Spec.Template.Spec.InfrastructureRef.Namespace,
					Name:      mp.Spec.Template.Spec.InfrastructureRef.Name,
				}, ammp)
				Expect(err).NotTo(HaveOccurred())

				if ammp.Spec.Mode != string(infrav1.NodePoolModeSystem) {
					isUserPool = true
				}
			case infrav1.AzureASOManagedMachinePoolKind:
				ammp := &infrav1.AzureASOManagedMachinePool{}
				err := bootstrapClusterProxy.GetClient().Get(ctx, types.NamespacedName{
					Namespace: mp.Spec.Template.Spec.InfrastructureRef.Namespace,
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
					Expect(bootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKeyFromObject(resource), agentPool)).To(Succeed())
					if ptr.Deref(agentPool.Status.Mode, "") != asocontainerservicev1.AgentPoolMode_STATUS_System {
						isUserPool = true
					}
					break
				}
			}

			if isUserPool {
				Byf("Scaling the machine pool %s to zero", mp.Name)
				framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
					ClusterProxy:              bootstrapClusterProxy,
					Cluster:                   input.Cluster,
					Replicas:                  0,
					MachinePools:              []*expv1.MachinePool{mp},
					WaitForMachinePoolToScale: input.WaitIntervals,
				})
			}

			Byf("Restoring initial replica count for machine pool %s", mp.Name)
			framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
				ClusterProxy:              bootstrapClusterProxy,
				Cluster:                   input.Cluster,
				Replicas:                  originalReplicas,
				MachinePools:              []*expv1.MachinePool{mp},
				WaitForMachinePoolToScale: input.WaitIntervals,
			})
		}(mp)
	}

	wg.Wait()
}

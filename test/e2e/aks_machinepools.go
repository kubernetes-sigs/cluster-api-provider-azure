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

	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AKSMachinePoolSpecInput struct {
	Cluster       *clusterv1.Cluster
	MachinePools  []*expv1.MachinePool
	WaitIntervals []interface{}
}

func AKSMachinePoolSpec(ctx context.Context, inputGetter func() AKSMachinePoolSpecInput) {
	input := inputGetter()
	var wg sync.WaitGroup

	originalReplicas := map[types.NamespacedName]int32{}
	for _, mp := range input.MachinePools {
		originalReplicas[client.ObjectKeyFromObject(mp)] = to.Int32(mp.Spec.Replicas)
	}

	By("Scaling the machine pools out")
	for _, mp := range input.MachinePools {
		wg.Add(1)
		go func(mp *expv1.MachinePool) {
			defer GinkgoRecover()
			defer wg.Done()
			framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
				ClusterProxy:              bootstrapClusterProxy,
				Cluster:                   input.Cluster,
				Replicas:                  to.Int32(mp.Spec.Replicas) + 1,
				MachinePools:              []*expv1.MachinePool{mp},
				WaitForMachinePoolToScale: input.WaitIntervals,
			})
		}(mp)
	}
	wg.Wait()

	By("Scaling the machine pools in")
	for _, mp := range input.MachinePools {
		wg.Add(1)
		go func(mp *expv1.MachinePool) {
			defer GinkgoRecover()
			defer wg.Done()
			framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
				ClusterProxy:              bootstrapClusterProxy,
				Cluster:                   input.Cluster,
				Replicas:                  to.Int32(mp.Spec.Replicas) - 1,
				MachinePools:              []*expv1.MachinePool{mp},
				WaitForMachinePoolToScale: input.WaitIntervals,
			})
		}(mp)
	}
	wg.Wait()

	By("Scaling the machine pools to zero")
	// System node pools cannot be scaled to 0, so only include user node pools.
	var machinePoolsToScale []*expv1.MachinePool
	for _, mp := range input.MachinePools {
		ammp := &infrav1.AzureManagedMachinePool{}
		err := bootstrapClusterProxy.GetClient().Get(ctx, types.NamespacedName{
			Namespace: mp.Spec.Template.Spec.InfrastructureRef.Namespace,
			Name:      mp.Spec.Template.Spec.InfrastructureRef.Name,
		}, ammp)
		Expect(err).NotTo(HaveOccurred())

		if ammp.Spec.Mode != string(infrav1.NodePoolModeSystem) {
			machinePoolsToScale = append(machinePoolsToScale, mp)
		}
	}

	framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
		ClusterProxy:              bootstrapClusterProxy,
		Cluster:                   input.Cluster,
		Replicas:                  0,
		MachinePools:              machinePoolsToScale,
		WaitForMachinePoolToScale: input.WaitIntervals,
	})

	By("Restoring initial replica count")
	for _, mp := range input.MachinePools {
		wg.Add(1)
		go func(mp *expv1.MachinePool) {
			defer GinkgoRecover()
			defer wg.Done()
			framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
				ClusterProxy:              bootstrapClusterProxy,
				Cluster:                   input.Cluster,
				Replicas:                  originalReplicas[client.ObjectKeyFromObject(mp)],
				MachinePools:              []*expv1.MachinePool{mp},
				WaitForMachinePoolToScale: input.WaitIntervals,
			})
		}(mp)
	}
	wg.Wait()
}

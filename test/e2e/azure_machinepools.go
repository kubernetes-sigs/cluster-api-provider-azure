//go:build e2e
// +build e2e

/*
Copyright 2023 The Kubernetes Authors.

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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AzureMachinePoolsSpecName = "azure-machinepools"
	regexpFlexibleVM          = `^azure:\/\/\/subscriptions\/[0-9a-fA-F]{8}-([0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}\/resourceGroups\/.+\/providers\/Microsoft.Compute\/virtualMachines\/.+$`
	regexpUniformInstance     = `^azure:\/\/\/subscriptions\/[0-9a-fA-F]{8}-([0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}\/resourceGroups\/.+\/providers\/Microsoft.Compute\/virtualMachineScaleSets\/.+\/virtualMachines\/\d+$`
)

// AzureMachinePoolsSpecInput is the input for AzureMachinePoolsSpec.
type (
	AzureMachinePoolsSpecInput struct {
		Cluster               *clusterv1.Cluster
		BootstrapClusterProxy framework.ClusterProxy
		Namespace             *corev1.Namespace
		ClusterName           string
		WaitIntervals         []interface{}
	}
)

// AzureMachinePoolsSpec tests that the expected machinepool resources exist.
func AzureMachinePoolsSpec(ctx context.Context, inputGetter func() AzureMachinePoolsSpecInput) {
	input := inputGetter()
	Expect(input.Cluster).NotTo(BeNil(), "Invalid argument. input.Cluster can't be nil when calling %s spec", AzureMachinePoolsSpecName)
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", AzureMachinePoolsSpecName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", AzureMachinePoolsSpecName)
	Expect(input.ClusterName).NotTo(BeEmpty(), "Invalid argument. input.ClusterName can't be empty when calling %s spec", AzureMachinePoolsSpecName)
	Expect(input.WaitIntervals).NotTo(BeEmpty(), "Invalid argument. input.WaitIntervals can't be empty when calling %s spec", AzureMachinePoolsSpecName)

	var (
		bootstrapClusterProxy = input.BootstrapClusterProxy
		workloadClusterProxy  = bootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
		clusterLabels         = map[string]string{clusterv1.ClusterNameLabel: workloadClusterProxy.GetName()}
	)

	Expect(workloadClusterProxy).NotTo(BeNil())
	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	Byf("listing AzureMachinePools for cluster %s in namespace %s", input.ClusterName, input.Namespace.Name)
	ampList := &infrav1exp.AzureMachinePoolList{}
	Expect(mgmtClient.List(ctx, ampList, client.InNamespace(input.Namespace.Name), client.MatchingLabels(clusterLabels))).To(Succeed())
	Expect(ampList.Items).NotTo(BeEmpty())
	machinepools := []*expv1.MachinePool{}
	for _, amp := range ampList.Items {
		Byf("checking AzureMachinePool %s in %s orchestration mode", amp.Name, amp.Spec.OrchestrationMode)
		Expect(amp.Status.Replicas).To(BeNumerically("==", len(amp.Spec.ProviderIDList)))
		for _, providerID := range amp.Spec.ProviderIDList {
			switch amp.Spec.OrchestrationMode {
			case infrav1.OrchestrationModeType(compute.OrchestrationModeFlexible):
				Expect(providerID).To(MatchRegexp(regexpFlexibleVM))
			default:
				Expect(providerID).To(MatchRegexp(regexpUniformInstance))
			}
		}
		mp, err := azureutil.FindParentMachinePool(amp.Name, bootstrapClusterProxy.GetClient())
		Expect(err).NotTo(HaveOccurred())
		Expect(mp).NotTo(BeNil())
		machinepools = append(machinepools, mp)
	}

	var wg sync.WaitGroup
	for _, mp := range machinepools {
		goalReplicas := ptr.Deref[int32](mp.Spec.Replicas, 0) + 1
		Byf("Scaling machine pool %s out from %d to %d", mp.Name, *mp.Spec.Replicas, goalReplicas)
		wg.Add(1)
		go func(mp *expv1.MachinePool) {
			defer GinkgoRecover()
			defer wg.Done()
			framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
				ClusterProxy:              bootstrapClusterProxy,
				Cluster:                   input.Cluster,
				Replicas:                  goalReplicas,
				MachinePools:              []*expv1.MachinePool{mp},
				WaitForMachinePoolToScale: input.WaitIntervals,
			})
		}(mp)
	}
	wg.Wait()

	for _, mp := range machinepools {
		goalReplicas := ptr.Deref[int32](mp.Spec.Replicas, 0) - 1
		Byf("Scaling machine pool %s in from %d to %d", mp.Name, *mp.Spec.Replicas, goalReplicas)
		wg.Add(1)
		go func(mp *expv1.MachinePool) {
			defer GinkgoRecover()
			defer wg.Done()
			framework.ScaleMachinePoolAndWait(ctx, framework.ScaleMachinePoolAndWaitInput{
				ClusterProxy:              bootstrapClusterProxy,
				Cluster:                   input.Cluster,
				Replicas:                  goalReplicas,
				MachinePools:              []*expv1.MachinePool{mp},
				WaitForMachinePoolToScale: input.WaitIntervals,
			})
		}(mp)
	}
	wg.Wait()

	By("verifying that workload nodes are schedulable")
	clientset := workloadClusterProxy.GetClientSet()
	Expect(clientset).NotTo(BeNil())
	workloadNodeRequirement, err := labels.NewRequirement("node-role.kubernetes.io/control-plane", selection.DoesNotExist, nil)
	Expect(err).NotTo(HaveOccurred())
	selector := labels.NewSelector().Add(*workloadNodeRequirement)
	nodeList, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	Expect(err).NotTo(HaveOccurred())
	Expect(len(nodeList.Items)).NotTo(BeZero())
	for _, node := range nodeList.Items {
		for _, taint := range node.Spec.Taints {
			Expect(taint.Effect).NotTo(BeElementOf(
				corev1.TaintEffectNoSchedule, corev1.TaintEffectPreferNoSchedule, corev1.TaintEffectNoExecute),
				"node %s has %s taint", node.Name, taint.Effect)
		}
	}
}

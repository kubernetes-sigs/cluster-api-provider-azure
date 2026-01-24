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
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
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
	machinepools := []*clusterv1.MachinePool{}
	for _, amp := range ampList.Items {
		Byf("checking AzureMachinePool %s in %s orchestration mode", amp.Name, amp.Spec.OrchestrationMode)
		Expect(amp.Status.Replicas).To(BeNumerically("==", len(amp.Spec.ProviderIDList)))
		for _, providerID := range amp.Spec.ProviderIDList {
			switch amp.Spec.OrchestrationMode {
			case infrav1.OrchestrationModeType(armcompute.OrchestrationModeFlexible):
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

	patchMachinePoolReplicas := func(mp *clusterv1.MachinePool, replicas int32) {
		GinkgoHelper()

		patchHelper, err := patch.NewHelper(mp, bootstrapClusterProxy.GetClient())
		Expect(err).NotTo(HaveOccurred())

		mp.Spec.Replicas = &replicas
		Eventually(func(ctx context.Context) error {
			return patchHelper.Patch(ctx, mp)
		}, 3*time.Minute, 10*time.Second).WithContext(ctx).Should(Succeed())
	}

	// [framework.ScaleMachinePoolAndWait] wraps a similar "change replica count
	// + wait" sequence. The difference is that here we bump the replica count
	// by a relative amount vs. setting an absolute replica count. This way we
	// make sure we're actually changing the replica count in the direction we
	// want without having to care about the initial state of each individual
	// MachinePool.

	// Scale out
	for _, mp := range machinepools {
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

	By("verifying that workload nodes are schedulable")
	clientset := workloadClusterProxy.GetClientSet()
	Expect(clientset).NotTo(BeNil())
	workloadNodeRequirement, err := labels.NewRequirement("node-role.kubernetes.io/control-plane", selection.DoesNotExist, nil)
	Expect(err).NotTo(HaveOccurred())
	selector := labels.NewSelector().Add(*workloadNodeRequirement)
	nodeList, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	Expect(err).NotTo(HaveOccurred())
	Expect(nodeList.Items).NotTo(BeEmpty())
	for _, node := range nodeList.Items {
		for _, taint := range node.Spec.Taints {
			Expect(taint.Effect).NotTo(BeElementOf(
				corev1.TaintEffectNoSchedule, corev1.TaintEffectPreferNoSchedule, corev1.TaintEffectNoExecute),
				"node %s has %s taint", node.Name, taint.Effect)
		}
	}
}

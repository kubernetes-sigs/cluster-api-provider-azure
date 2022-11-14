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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
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
		BootstrapClusterProxy framework.ClusterProxy
		Namespace             *corev1.Namespace
		ClusterName           string
	}
)

// AzureMachinePoolsSpec tests that the expected machinepool resources exist.
func AzureMachinePoolsSpec(ctx context.Context, inputGetter func() AzureMachinePoolsSpecInput) {
	input := inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", AzureMachinePoolsSpecName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", AzureMachinePoolsSpecName)
	Expect(input.ClusterName).NotTo(BeEmpty(), "Invalid argument. input.ClusterName can't be empty when calling %s spec", AzureMachinePoolsSpecName)

	var (
		bootstrapClusterProxy = input.BootstrapClusterProxy
		workloadClusterProxy  = bootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
		labels                = map[string]string{clusterv1.ClusterLabelName: workloadClusterProxy.GetName()}
	)

	Expect(workloadClusterProxy).NotTo(BeNil())

	Byf("listing AzureMachinePools for cluster %s in namespace %s", input.ClusterName, input.Namespace.Name)
	ampList := &infrav1exp.AzureMachinePoolList{}
	Expect(bootstrapClusterProxy.GetClient().List(ctx, ampList, client.InNamespace(input.Namespace.Name), client.MatchingLabels(labels))).To(Succeed())
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
	}
}

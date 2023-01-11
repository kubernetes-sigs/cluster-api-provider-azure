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

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-05-01/containerservice"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AKSAutoscaleSpecInput struct {
	Cluster       *clusterv1.Cluster
	MachinePool   *expv1.MachinePool
	WaitIntervals []interface{}
}

func AKSAutoscaleSpec(ctx context.Context, inputGetter func() AKSAutoscaleSpecInput) {
	input := inputGetter()

	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	auth, err := azureutil.GetAuthorizer(settings)
	Expect(err).NotTo(HaveOccurred())
	agentpoolClient := containerservice.NewAgentPoolsClient(subscriptionID)
	agentpoolClient.Authorizer = auth
	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	amcp := &infrav1.AzureManagedControlPlane{}
	err = mgmtClient.Get(ctx, types.NamespacedName{
		Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace,
		Name:      input.Cluster.Spec.ControlPlaneRef.Name,
	}, amcp)
	Expect(err).NotTo(HaveOccurred())

	ammp := &infrav1.AzureManagedMachinePool{}
	err = mgmtClient.Get(ctx, client.ObjectKeyFromObject(input.MachinePool), ammp)
	Expect(err).NotTo(HaveOccurred())

	resourceGroupName := amcp.Spec.ResourceGroupName
	managedClusterName := amcp.Name
	agentPoolName := *ammp.Spec.Name
	getAgentPool := func() (containerservice.AgentPool, error) {
		return agentpoolClient.Get(ctx, resourceGroupName, managedClusterName, agentPoolName)
	}

	toggleAutoscaling := func() {
		Eventually(func(g Gomega) {
			err = mgmtClient.Get(ctx, client.ObjectKeyFromObject(ammp), ammp)
			g.Expect(err).NotTo(HaveOccurred())

			enabled := ammp.Spec.Scaling != nil
			var enabling string
			if enabled {
				enabling = "Disabling"
				ammp.Spec.Scaling = nil
			} else {
				enabling = "Enabling"
				ammp.Spec.Scaling = &infrav1.ManagedMachinePoolScaling{
					MinSize: pointer.Int32(1),
					MaxSize: pointer.Int32(2),
				}
			}
			By(enabling + " autoscaling")
			err = mgmtClient.Update(ctx, ammp)
			g.Expect(err).NotTo(HaveOccurred())
		}, inputGetter().WaitIntervals...).Should(Succeed())
	}

	validateUntoggled := validateAKSAutoscaleDisabled
	validateToggled := validateAKSAutoscaleEnabled
	autoscalingInitiallyEnabled := ammp.Spec.Scaling != nil
	if autoscalingInitiallyEnabled {
		validateToggled, validateUntoggled = validateUntoggled, validateToggled
	}

	validateUntoggled(getAgentPool, inputGetter)
	toggleAutoscaling()
	validateToggled(getAgentPool, inputGetter)
	toggleAutoscaling()
	validateUntoggled(getAgentPool, inputGetter)
}

func validateAKSAutoscaleDisabled(agentPoolGetter func() (containerservice.AgentPool, error), inputGetter func() AKSAutoscaleSpecInput) {
	By("Validating autoscaler disabled")
	Eventually(func(g Gomega) {
		agentpool, err := agentPoolGetter()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(pointer.BoolDeref(agentpool.EnableAutoScaling, false)).To(BeFalse())
	}, inputGetter().WaitIntervals...).Should(Succeed())
}

func validateAKSAutoscaleEnabled(agentPoolGetter func() (containerservice.AgentPool, error), inputGetter func() AKSAutoscaleSpecInput) {
	By("Validating autoscaler enabled")
	Eventually(func(g Gomega) {
		agentpool, err := agentPoolGetter()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(pointer.BoolDeref(agentpool.EnableAutoScaling, false)).To(BeTrue())
	}, inputGetter().WaitIntervals...).Should(Succeed())
}

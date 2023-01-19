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

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2022-03-01/containerservice"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

type AKSAzureClusterAutoscalerSettingsSpecInput struct {
	Cluster       *clusterv1.Cluster
	WaitIntervals []interface{}
}

func AKSAzureClusterAutoscalerSettingsSpec(ctx context.Context, inputGetter func() AKSAzureClusterAutoscalerSettingsSpecInput) {
	input := inputGetter()

	settings, err := auth.GetSettingsFromEnvironment()
	Expect(err).NotTo(HaveOccurred())
	subscriptionID := settings.GetSubscriptionID()
	auth, err := settings.GetAuthorizer()
	Expect(err).NotTo(HaveOccurred())
	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())
	containerserviceClient := containerservice.NewManagedClustersClient(subscriptionID)
	containerserviceClient.Authorizer = auth

	var expectedAksExpander containerservice.Expander
	var newExpanderValue infrav1.Expander
	var amcpInitialAutoScalerProfile = &infrav1.AutoScalerProfile{}
	amcp := &infrav1.AzureManagedControlPlane{}
	Eventually(func(g Gomega) {
		err = mgmtClient.Get(ctx, types.NamespacedName{
			Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace,
			Name:      input.Cluster.Spec.ControlPlaneRef.Name,
		}, amcp)
		g.Expect(err).NotTo(HaveOccurred())
		amcpInitialAutoScalerProfile = amcp.Spec.AutoScalerProfile

		aks, err := containerserviceClient.Get(ctx, amcp.Spec.ResourceGroupName, amcp.Name)
		g.Expect(err).NotTo(HaveOccurred())
		aksInitialAutoScalerProfile := aks.AutoScalerProfile

		// Conditional is based off of the actual AKS settings not the AzureManagedControlPlane
		if aksInitialAutoScalerProfile == nil {
			expectedAksExpander = containerservice.ExpanderLeastWaste
			newExpanderValue = infrav1.ExpanderLeastWaste
		} else if aksInitialAutoScalerProfile.Expander == containerservice.ExpanderLeastWaste {
			expectedAksExpander = containerservice.ExpanderMostPods
			newExpanderValue = infrav1.ExpanderMostPods
		}

		amcp.Spec.AutoScalerProfile = nil
		err = mgmtClient.Get(ctx, types.NamespacedName{
			Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace,
			Name:      input.Cluster.Spec.ControlPlaneRef.Name,
		}, amcp)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(mgmtClient.Update(ctx, amcp)).To(Succeed())
	}, input.WaitIntervals...).Should(Succeed())
	Eventually(func(g Gomega) {
		err = mgmtClient.Get(ctx, types.NamespacedName{
			Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace,
			Name:      input.Cluster.Spec.ControlPlaneRef.Name,
		}, amcp)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(amcp.Spec.AutoScalerProfile).To(BeNil())
	}, input.WaitIntervals...).Should(Succeed())

	// Now set to the new value
	Eventually(func(g Gomega) {
		err = mgmtClient.Get(ctx, types.NamespacedName{
			Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace,
			Name:      input.Cluster.Spec.ControlPlaneRef.Name,
		}, amcp)
		g.Expect(err).NotTo(HaveOccurred())
		amcp.Spec.AutoScalerProfile = &infrav1.AutoScalerProfile{
			Expander: (*infrav1.Expander)(to.StringPtr(string(newExpanderValue))),
		}
		g.Expect(mgmtClient.Update(ctx, amcp)).To(Succeed())
	}, input.WaitIntervals...).Should(Succeed())
	By("Verifying the cluster-autoscaler settings have changed")
	Eventually(func(g Gomega) {
		// Check that the autoscaler settings have been sync'd to AKS
		aks, err := containerserviceClient.Get(ctx, amcp.Spec.ResourceGroupName, amcp.Name)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(aks.AutoScalerProfile).ToNot(BeNil())
		g.Expect(aks.AutoScalerProfile.Expander).To(Equal(expectedAksExpander))
	}, input.WaitIntervals...).Should(Succeed())

	Eventually(func(g Gomega) {
		err = mgmtClient.Get(ctx, types.NamespacedName{
			Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace,
			Name:      input.Cluster.Spec.ControlPlaneRef.Name,
		}, amcp)
		g.Expect(err).NotTo(HaveOccurred())
		amcp.Spec.AutoScalerProfile = amcpInitialAutoScalerProfile
		g.Expect(mgmtClient.Update(ctx, amcp)).To(Succeed())
	}, input.WaitIntervals...).Should(Succeed())
	Eventually(func(g Gomega) {
		err = mgmtClient.Get(ctx, types.NamespacedName{
			Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace,
			Name:      input.Cluster.Spec.ControlPlaneRef.Name,
		}, amcp)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(amcp.Spec.AutoScalerProfile).To(Equal(amcpInitialAutoScalerProfile))
	}, input.WaitIntervals...).Should(Succeed())
}

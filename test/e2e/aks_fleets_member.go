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
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservicefleet/armcontainerservicefleet"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AKSFleetsMemberInput struct {
	Cluster       *clusterv1.Cluster
	WaitIntervals []interface{}
}

const (
	fleetName       = "capz-aks-fleets-manager"
	updateGroupName = "capz-aks-fleets-member-update"
)

func AKSFleetsMemberSpec(ctx context.Context, inputGetter func() AKSFleetsMemberInput) {
	input := inputGetter()

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	Expect(err).NotTo(HaveOccurred())

	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	containerserviceClient, err := armcontainerservice.NewManagedClustersClient(getSubscriptionID(Default), cred, nil)
	Expect(err).NotTo(HaveOccurred())

	amcp := &infrav1.AzureManagedControlPlane{}
	err = mgmtClient.Get(ctx, types.NamespacedName{
		Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace,
		Name:      input.Cluster.Spec.ControlPlaneRef.Name,
	}, amcp)
	Expect(err).NotTo(HaveOccurred())

	groupClient, err := armresources.NewResourceGroupsClient(getSubscriptionID(Default), cred, nil)
	Expect(err).NotTo(HaveOccurred())

	By("Creating a resource group")
	groupName := "capz-aks-fleets-member-" + amcp.Spec.ResourceGroupName
	_, err = groupClient.CreateOrUpdate(ctx, groupName, armresources.ResourceGroup{
		Location: ptr.To(os.Getenv(AzureLocation)),
		Tags: map[string]*string{
			"jobName":           ptr.To(os.Getenv(JobName)),
			"creationTimestamp": ptr.To(os.Getenv(Timestamp)),
		},
	}, nil)
	Expect(err).NotTo(HaveOccurred())

	fleetClient, err := armcontainerservicefleet.NewFleetsClient(getSubscriptionID(Default), cred, nil)
	Expect(err).NotTo(HaveOccurred())

	fleetsMemberClient, err := armcontainerservicefleet.NewFleetMembersClient(getSubscriptionID(Default), cred, nil)
	Expect(err).NotTo(HaveOccurred())

	By("Creating a fleet manager")
	poller, err := fleetClient.BeginCreateOrUpdate(ctx, groupName, fleetName, armcontainerservicefleet.Fleet{
		Location: ptr.To(os.Getenv(AzureLocation)),
	}, nil)
	Expect(err).NotTo(HaveOccurred())
	_, err = poller.PollUntilDone(ctx, nil)
	Expect(err).NotTo(HaveOccurred())

	By("Joining the cluster to the fleet hub")
	var infraControlPlane = &infrav1.AzureManagedControlPlane{}
	Eventually(func(g Gomega) {
		err = mgmtClient.Get(ctx, client.ObjectKey{Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace, Name: input.Cluster.Spec.ControlPlaneRef.Name}, infraControlPlane)
		g.Expect(err).NotTo(HaveOccurred())
		infraControlPlane.Spec.FleetsMember = &infrav1.FleetsMember{
			FleetsMemberClassSpec: infrav1.FleetsMemberClassSpec{
				ManagerName:          fleetName,
				ManagerResourceGroup: groupName,
				Group:                updateGroupName,
			},
		}
		g.Expect(mgmtClient.Update(ctx, infraControlPlane)).To(Succeed())
		g.Expect(conditions.IsTrue(infraControlPlane, infrav1.FleetReadyCondition)).To(BeTrue())
	}, input.WaitIntervals...).Should(Succeed())

	By("Ensuring the fleet member is created and attached to the managed cluster")
	Eventually(func(g Gomega) {
		resp, err := fleetsMemberClient.Get(ctx, groupName, fleetName, input.Cluster.Name, nil)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp.Properties.ProvisioningState).To(Equal(ptr.To(armcontainerservicefleet.FleetMemberProvisioningStateSucceeded)))
		fleetsMember := resp.FleetMember
		g.Expect(fleetsMember.Properties).NotTo(BeNil())
		expectedID := azure.ManagedClusterID(getSubscriptionID(Default), infraControlPlane.Spec.ResourceGroupName, input.Cluster.Name)
		g.Expect(fleetsMember.Properties.ClusterResourceID).To(Equal(ptr.To(expectedID)))
		g.Expect(fleetsMember.Properties.ProvisioningState).To(Equal(ptr.To(armcontainerservicefleet.FleetMemberProvisioningStateSucceeded)))
	}, input.WaitIntervals...).Should(Succeed())

	By("Remove the FleetsMember spec from the AzureManagedControlPlane")
	Eventually(func(g Gomega) {
		err = mgmtClient.Get(ctx, client.ObjectKey{Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace, Name: input.Cluster.Spec.ControlPlaneRef.Name}, infraControlPlane)
		g.Expect(err).NotTo(HaveOccurred())
		infraControlPlane.Spec.FleetsMember = nil
		g.Expect(mgmtClient.Update(ctx, infraControlPlane)).To(Succeed())
	}, input.WaitIntervals...).Should(Succeed())

	By("Waiting for the managed cluster to finish updating")
	Eventually(func(g Gomega) {
		aks, err := containerserviceClient.Get(ctx, amcp.Spec.ResourceGroupName, amcp.Name, nil)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(aks.ManagedCluster.Properties.ProvisioningState).NotTo(Equal(ptr.To("Updating")))
	}, input.WaitIntervals...).Should(Succeed())

	By("Waiting for the FleetsMember to be deleted")
	Eventually(func(g Gomega) {
		_, err := fleetsMemberClient.Get(ctx, groupName, fleetName, input.Cluster.Name, nil)
		g.Expect(azure.ResourceNotFound(err)).To(BeTrue(), "expected err to be 'not found', got %v", err)
	}, input.WaitIntervals...).Should(Succeed())

	Logf("Deleting the fleet manager resource group %q", groupName)
	grpPoller, err := groupClient.BeginDelete(ctx, groupName, nil)
	Expect(err).NotTo(HaveOccurred())
	_, err = grpPoller.PollUntilDone(ctx, nil)
	Expect(err).NotTo(HaveOccurred())
}

//go:build e2e
// +build e2e

/*
Copyright 2026 The Kubernetes Authors.

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
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
	asocontainerservicev1mc "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20240901"
	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	asoannotations "github.com/Azure/azure-service-operator/v2/pkg/common/annotations"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	asoconditions "github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

type AKSMaintenanceConfigurationSpecInput struct {
	Cluster       *clusterv1.Cluster
	WaitForUpdate []interface{}
}

func AKSMaintenanceConfigurationSpec(ctx context.Context, inputGetter func() AKSMaintenanceConfigurationSpecInput) {
	input := inputGetter()

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	Expect(err).NotTo(HaveOccurred())

	mcClient, err := armcontainerservice.NewMaintenanceConfigurationsClient(getSubscriptionID(Default), cred, nil)
	Expect(err).NotTo(HaveOccurred())

	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	namespace := input.Cluster.Namespace
	managedClusterName := input.Cluster.Spec.ControlPlaneRef.Name

	By("Discovering the AKS resource group from the AzureASOManagedCluster")
	asoCluster := &infrav1.AzureASOManagedCluster{}
	Expect(mgmtClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: input.Cluster.Spec.InfrastructureRef.Name}, asoCluster)).To(Succeed())
	var resourceGroup string
	for _, raw := range asoCluster.Spec.Resources {
		u := &unstructured.Unstructured{}
		Expect(u.UnmarshalJSON(raw.Raw)).To(Succeed())
		if u.GroupVersionKind().Kind != "ResourceGroup" {
			continue
		}
		rg := &asoresourcesv1.ResourceGroup{}
		Expect(mgmtClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: u.GetName()}, rg)).To(Succeed())
		resourceGroup = rg.AzureName()
		break
	}
	Expect(resourceGroup).NotTo(BeEmpty())

	infraControlPlane := &infrav1.AzureASOManagedControlPlane{}
	Expect(mgmtClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: managedClusterName}, infraControlPlane)).To(Succeed())
	originalResources := append([]runtime.RawExtension(nil), infraControlPlane.Spec.Resources...)

	var credentialFrom string
	for _, raw := range originalResources {
		u := &unstructured.Unstructured{}
		Expect(u.UnmarshalJSON(raw.Raw)).To(Succeed())
		if u.GroupVersionKind().Kind == "ManagedCluster" {
			credentialFrom = u.GetAnnotations()[asoannotations.PerResourceSecret]
			break
		}
	}

	newMC := func(name, azureName string, spec asocontainerservicev1mc.MaintenanceConfiguration_Spec) *asocontainerservicev1mc.MaintenanceConfiguration {
		spec.AzureName = azureName
		spec.Owner = &genruntime.KnownResourceReference{Name: managedClusterName}
		mc := &asocontainerservicev1mc.MaintenanceConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: asocontainerservicev1mc.GroupVersion.String(),
				Kind:       "MaintenanceConfiguration",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: input.Cluster.Name + "-" + name,
			},
			Spec: spec,
		}
		if credentialFrom != "" {
			mc.Annotations = map[string]string{asoannotations.PerResourceSecret: credentialFrom}
		}
		return mc
	}

	weekly := func(day asocontainerservicev1mc.WeekDay) *asocontainerservicev1mc.MaintenanceWindow {
		return &asocontainerservicev1mc.MaintenanceWindow{
			DurationHours: ptr.To(4),
			UtcOffset:     ptr.To("-05:00"),
			StartTime:     ptr.To("02:00"),
			Schedule: &asocontainerservicev1mc.Schedule{
				Weekly: &asocontainerservicev1mc.WeeklySchedule{
					IntervalWeeks: ptr.To(1),
					DayOfWeek:     ptr.To(day),
				},
			},
		}
	}

	defaultMC := newMC("mc-default", "default", asocontainerservicev1mc.MaintenanceConfiguration_Spec{
		TimeInWeek: []asocontainerservicev1mc.TimeInWeek{{
			Day:       ptr.To(asocontainerservicev1mc.WeekDay_Sunday),
			HourSlots: []asocontainerservicev1mc.HourInDay{0, 1, 2, 3},
		}},
	})
	autoUpgradeMC := newMC("mc-auto-upgrade", "aksManagedAutoUpgradeSchedule", asocontainerservicev1mc.MaintenanceConfiguration_Spec{
		MaintenanceWindow: weekly(asocontainerservicev1mc.WeekDay_Sunday),
	})
	nodeOSMC := newMC("mc-node-os-upgrade", "aksManagedNodeOSUpgradeSchedule", asocontainerservicev1mc.MaintenanceConfiguration_Spec{
		MaintenanceWindow: weekly(asocontainerservicev1mc.WeekDay_Sunday),
	})

	setMCs := func(mcs ...*asocontainerservicev1mc.MaintenanceConfiguration) {
		Eventually(func(g Gomega) {
			g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(infraControlPlane), infraControlPlane)).To(Succeed())
			resources := append([]runtime.RawExtension(nil), originalResources...)
			for _, mc := range mcs {
				bs, err := json.Marshal(mc)
				g.Expect(err).NotTo(HaveOccurred())
				resources = append(resources, runtime.RawExtension{Raw: bs})
			}
			infraControlPlane.Spec.Resources = resources
			g.Expect(mgmtClient.Update(ctx, infraControlPlane)).To(Succeed())
		}, input.WaitForUpdate...).Should(Succeed())
	}

	isReady := func(c asoconditions.Conditioner) bool {
		conds := c.GetConditions()
		if i, ok := conds.FindIndexByType(asoconditions.ConditionTypeReady); ok {
			return conds[i].Status == metav1.ConditionTrue
		}
		return false
	}

	By("Appending three MaintenanceConfigurations to the AzureASOManagedControlPlane")
	setMCs(defaultMC, autoUpgradeMC, nodeOSMC)

	By("Waiting for each ASO MaintenanceConfiguration to reach Ready=True")
	for _, mc := range []*asocontainerservicev1mc.MaintenanceConfiguration{defaultMC, autoUpgradeMC, nodeOSMC} {
		Eventually(func(g Gomega) {
			got := &asocontainerservicev1mc.MaintenanceConfiguration{}
			g.Expect(mgmtClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: mc.Name}, got)).To(Succeed())
			g.Expect(isReady(got)).To(BeTrue(), "expected ASO MaintenanceConfiguration %q to be Ready", mc.Name)
		}, input.WaitForUpdate...).Should(Succeed())
	}

	By("Verifying the default MaintenanceConfiguration in Azure")
	Eventually(func(g Gomega) {
		resp, err := mcClient.Get(ctx, resourceGroup, managedClusterName, defaultMC.Spec.AzureName, nil)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp.Properties).NotTo(BeNil())
		expected := defaultMC.Spec.TimeInWeek
		g.Expect(resp.Properties.TimeInWeek).To(HaveLen(len(expected)))
		for i, want := range expected {
			g.Expect(resp.Properties.TimeInWeek[i].Day).To(HaveValue(BeEquivalentTo(*want.Day)))
			gotHours := make([]int32, 0, len(resp.Properties.TimeInWeek[i].HourSlots))
			for _, h := range resp.Properties.TimeInWeek[i].HourSlots {
				gotHours = append(gotHours, *h)
			}
			wantHours := make([]int32, 0, len(want.HourSlots))
			for _, h := range want.HourSlots {
				wantHours = append(wantHours, int32(h))
			}
			g.Expect(gotHours).To(ConsistOf(wantHours))
		}
	}, input.WaitForUpdate...).Should(Succeed())

	for _, mc := range []*asocontainerservicev1mc.MaintenanceConfiguration{autoUpgradeMC, nodeOSMC} {
		Byf("Verifying the %s MaintenanceConfiguration in Azure", mc.Spec.AzureName)
		Eventually(func(g Gomega) {
			resp, err := mcClient.Get(ctx, resourceGroup, managedClusterName, mc.Spec.AzureName, nil)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(resp.Properties).NotTo(BeNil())
			g.Expect(resp.Properties.MaintenanceWindow).NotTo(BeNil())
			want := mc.Spec.MaintenanceWindow
			g.Expect(resp.Properties.MaintenanceWindow.DurationHours).To(HaveValue(BeEquivalentTo(*want.DurationHours)))
			g.Expect(resp.Properties.MaintenanceWindow.UTCOffset).To(HaveValue(Equal(*want.UtcOffset)))
			g.Expect(resp.Properties.MaintenanceWindow.StartTime).To(HaveValue(Equal(*want.StartTime)))
			g.Expect(resp.Properties.MaintenanceWindow.Schedule).NotTo(BeNil())
			g.Expect(resp.Properties.MaintenanceWindow.Schedule.Weekly).NotTo(BeNil())
			g.Expect(resp.Properties.MaintenanceWindow.Schedule.Weekly.IntervalWeeks).To(HaveValue(BeEquivalentTo(*want.Schedule.Weekly.IntervalWeeks)))
			g.Expect(resp.Properties.MaintenanceWindow.Schedule.Weekly.DayOfWeek).To(HaveValue(BeEquivalentTo(*want.Schedule.Weekly.DayOfWeek)))
		}, input.WaitForUpdate...).Should(Succeed())
	}

	By("Updating the node-OS upgrade schedule from Sunday to Saturday")
	nodeOSMC.Spec.MaintenanceWindow.Schedule.Weekly.DayOfWeek = ptr.To(asocontainerservicev1mc.WeekDay_Saturday)
	setMCs(defaultMC, autoUpgradeMC, nodeOSMC)
	Eventually(func(g Gomega) {
		resp, err := mcClient.Get(ctx, resourceGroup, managedClusterName, nodeOSMC.Spec.AzureName, nil)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resp.Properties.MaintenanceWindow.Schedule.Weekly.DayOfWeek).To(HaveValue(BeEquivalentTo(*nodeOSMC.Spec.MaintenanceWindow.Schedule.Weekly.DayOfWeek)))
	}, input.WaitForUpdate...).Should(Succeed())

	By("Listing all maintenance configurations on the managed cluster")
	Eventually(func(g Gomega) {
		pager := mcClient.NewListByManagedClusterPager(resourceGroup, managedClusterName, nil)
		seen := map[string]bool{}
		for pager.More() {
			page, err := pager.NextPage(ctx)
			g.Expect(err).NotTo(HaveOccurred())
			for _, item := range page.Value {
				if item != nil && item.Name != nil {
					seen[*item.Name] = true
				}
			}
		}
		for _, mc := range []*asocontainerservicev1mc.MaintenanceConfiguration{defaultMC, autoUpgradeMC, nodeOSMC} {
			g.Expect(seen).To(HaveKey(mc.Spec.AzureName))
		}
	}, input.WaitForUpdate...).Should(Succeed())

	By("Removing the default MaintenanceConfiguration from spec.resources")
	setMCs(autoUpgradeMC, nodeOSMC)
	Eventually(func(g Gomega) {
		_, err := mcClient.Get(ctx, resourceGroup, managedClusterName, defaultMC.Spec.AzureName, nil)
		g.Expect(azure.ResourceNotFound(err)).To(BeTrue(), "expected MaintenanceConfiguration %q to be deleted from Azure, got err=%v", defaultMC.Spec.AzureName, err)
	}, input.WaitForUpdate...).Should(Succeed())

	By("Restoring the original spec.resources")
	setMCs()
}

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
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20250801"
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
)

type AKSAzureClusterAutoscalerSettingsSpecInput struct {
	Cluster       *clusterv1.Cluster
	WaitIntervals []interface{}
}

func AKSAzureClusterAutoscalerSettingsSpec(ctx context.Context, inputGetter func() AKSAzureClusterAutoscalerSettingsSpecInput) {
	input := inputGetter()

	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	Expect(err).NotTo(HaveOccurred())
	containerserviceClient, err := armcontainerservice.NewManagedClustersClient(getSubscriptionID(Default), cred, nil)
	Expect(err).NotTo(HaveOccurred())

	controlPlane, err := getAKSControlPlane(ctx, mgmtClient, input.Cluster)
	Expect(err).NotTo(HaveOccurred())
	resourceGroup, clusterName, err := getAKSClusterNames(ctx, mgmtClient, controlPlane)
	Expect(err).NotTo(HaveOccurred())
	initialProfile, err := getAKSAutoScalerProfile(controlPlane)
	Expect(err).NotTo(HaveOccurred())

	var initialChildProfile *asocontainerservicev1.ManagedClusterProperties_AutoScalerProfile
	var newExpander armcontainerservice.Expander
	Eventually(func(g Gomega) {
		managedCluster, err := getAKSManagedCluster(ctx, mgmtClient, controlPlane)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(aksManagedClusterReady(managedCluster)).To(BeTrue())
		aks, err := containerserviceClient.Get(ctx, resourceGroup, clusterName, nil)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(aks.Properties).NotTo(BeNil())
		g.Expect(aks.Properties.ProvisioningState).To(HaveValue(Equal("Succeeded")))
		initialChildProfile = managedCluster.Spec.AutoScalerProfile.DeepCopy()
		newExpander = nextAKSExpander(aks.Properties.AutoScalerProfile)
	}, input.WaitIntervals...).Should(Succeed())

	updateProfile := func(profile map[string]interface{}) {
		GinkgoHelper()
		Eventually(func(g Gomega) {
			g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(controlPlane), controlPlane)).To(Succeed())
			g.Expect(setAKSAutoScalerProfile(controlPlane, profile)).To(Succeed())
			g.Expect(mgmtClient.Update(ctx, controlPlane)).To(Succeed())
		}, input.WaitIntervals...).Should(Succeed())
	}
	waitForProfile := func(checkProfile func(Gomega, *asocontainerservicev1.ManagedClusterProperties_AutoScalerProfile)) {
		GinkgoHelper()
		Eventually(func(g Gomega) {
			managedCluster, err := getAKSManagedCluster(ctx, mgmtClient, controlPlane)
			g.Expect(err).NotTo(HaveOccurred())
			checkProfile(g, managedCluster.Spec.AutoScalerProfile)
			g.Expect(aksManagedClusterReady(managedCluster)).To(BeTrue(), "waiting for the current ASO ManagedCluster generation")
			aks, err := containerserviceClient.Get(ctx, resourceGroup, clusterName, nil)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(aks.Properties).NotTo(BeNil())
			g.Expect(aks.Properties.ProvisioningState).To(HaveValue(Equal("Succeeded")))
		}, input.WaitIntervals...).Should(Succeed())
	}

	defer func() {
		By("Restoring the original cluster-autoscaler profile")
		updateProfile(initialProfile)
		waitForProfile(func(g Gomega, profile *asocontainerservicev1.ManagedClusterProperties_AutoScalerProfile) {
			g.Expect(profile).To(Equal(initialChildProfile))
		})
	}()

	By("Clearing the cluster-autoscaler profile")
	updateProfile(nil)
	waitForProfile(func(g Gomega, profile *asocontainerservicev1.ManagedClusterProperties_AutoScalerProfile) {
		g.Expect(profile).To(BeNil())
	})

	By("Updating the cluster-autoscaler expander")
	updateProfile(map[string]interface{}{"expander": string(newExpander)})
	waitForProfile(func(g Gomega, profile *asocontainerservicev1.ManagedClusterProperties_AutoScalerProfile) {
		g.Expect(profile).NotTo(BeNil())
		g.Expect(profile.Expander).To(HaveValue(BeEquivalentTo(newExpander)))
	})

	By("Verifying the cluster-autoscaler settings have changed")
	Eventually(func(g Gomega) {
		aks, err := containerserviceClient.Get(ctx, resourceGroup, clusterName, nil)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(aks.Properties).NotTo(BeNil())
		g.Expect(aks.Properties.ProvisioningState).To(HaveValue(Equal("Succeeded")))
		g.Expect(aks.Properties.AutoScalerProfile).NotTo(BeNil())
		g.Expect(aks.Properties.AutoScalerProfile.Expander).To(HaveValue(Equal(newExpander)))
	}, input.WaitIntervals...).Should(Succeed())
}

func nextAKSExpander(profile *armcontainerservice.ManagedClusterPropertiesAutoScalerProfile) armcontainerservice.Expander {
	if profile != nil && ptr.Deref(profile.Expander, "") == armcontainerservice.ExpanderLeastWaste {
		return armcontainerservice.ExpanderMostPods
	}
	return armcontainerservice.ExpanderLeastWaste
}

func getAKSAutoScalerProfile(controlPlane client.Object) (map[string]interface{}, error) {
	switch cp := controlPlane.(type) {
	case *infrav1.AzureManagedControlPlane:
		if cp.Spec.AutoScalerProfile == nil {
			return nil, nil
		}
		return runtime.DefaultUnstructuredConverter.ToUnstructured(cp.Spec.AutoScalerProfile)
	case *infrav1.AzureASOManagedControlPlane:
		resource, _, err := findAKSResource(cp.Spec.Resources, "ManagedCluster")
		if err != nil {
			return nil, err
		}
		profile, _, err := unstructured.NestedMap(resource.Object, "spec", "autoScalerProfile")
		return profile, err
	default:
		return nil, fmt.Errorf("unsupported AKS control plane %T", controlPlane)
	}
}

func setAKSAutoScalerProfile(controlPlane client.Object, profile map[string]interface{}) error {
	switch cp := controlPlane.(type) {
	case *infrav1.AzureManagedControlPlane:
		if profile == nil {
			cp.Spec.AutoScalerProfile = nil
			return nil
		}
		updated := &infrav1.AutoScalerProfile{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(profile, updated); err != nil {
			return err
		}
		cp.Spec.AutoScalerProfile = updated
		return nil
	case *infrav1.AzureASOManagedControlPlane:
		resource, index, err := findAKSResource(cp.Spec.Resources, "ManagedCluster")
		if err != nil {
			return err
		}
		if profile == nil {
			unstructured.RemoveNestedField(resource.Object, "spec", "autoScalerProfile")
		} else if err := unstructured.SetNestedMap(resource.Object, profile, "spec", "autoScalerProfile"); err != nil {
			return err
		}
		raw, err := resource.MarshalJSON()
		if err != nil {
			return err
		}
		cp.Spec.Resources[index] = runtime.RawExtension{Raw: raw}
		return nil
	default:
		return fmt.Errorf("unsupported AKS control plane %T", controlPlane)
	}
}

func aksManagedClusterReady(managedCluster *asocontainerservicev1.ManagedCluster) bool {
	conditions := managedCluster.GetConditions()
	index, found := conditions.FindIndexByType(asoconditions.ConditionTypeReady)
	return found && conditions[index].Status == metav1.ConditionTrue && conditions[index].ObservedGeneration == managedCluster.Generation
}

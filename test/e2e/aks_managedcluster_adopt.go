//go:build e2e
// +build e2e

/*
Copyright 2024 The Kubernetes Authors.

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

	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231001"
	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

const (
	adoptAnnotation      = "sigs.k8s.io/cluster-api-provider-azure-adopt"
	adoptAnnotationValue = "true"
)

// AKSManagedClusterAdoptSpecInput contains the fields the required for testing managedclusteradopt controller.
type AKSManagedClusterAdoptSpecInput struct {
	MgmtCluster   framework.ClusterProxy
	ClusterName   string
	Namespace     string
	Location      string
	WaitIntervals []interface{}
}

// AKSManagedClusterAdoptSpec tests the managedclusteradopt controller's ability to adopt
// existing ASO ManagedCluster resources into CAPI management.
func AKSManagedClusterAdoptSpec(ctx context.Context, inputGetter func() AKSManagedClusterAdoptSpecInput) {
	input := inputGetter()
	mgmtClient := input.MgmtCluster.GetClient()

	Expect(ctx).NotTo(BeNil(), "ctx is required for AKSManagedClusterAdoptSpec")
	Expect(mgmtClient).NotTo(BeNil(), "Invalid argument. mgmtClient can't be nil when calling AKSManagedClusterAdoptSpec")
	Expect(input.ClusterName).NotTo(BeEmpty(), "Invalid argument. ClusterName can't be empty when calling AKSManagedClusterAdoptSpec")
	Expect(input.Namespace).NotTo(BeEmpty(), "Invalid argument. Namespace can't be empty when calling AKSManagedClusterAdoptSpec")
	Expect(input.Location).NotTo(BeEmpty(), "Invalid argument. Location can't be empty when calling AKSManagedClusterAdoptSpec")

	By("Creating an ASO ResourceGroup")
	resourceGroup := &asoresourcesv1.ResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: input.Namespace,
			Name:      input.ClusterName,
		},
		Spec: asoresourcesv1.ResourceGroup_Spec{
			Location: &input.Location,
		},
	}
	Expect(mgmtClient.Create(ctx, resourceGroup)).To(Succeed())

	By("Creating an ASO ManagedCluster without adoption annotation")
	managedCluster := &asocontainerservicev1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: input.Namespace,
			Name:      input.ClusterName,
		},
		Spec: asocontainerservicev1.ManagedCluster_Spec{
			Location: &input.Location,
			Owner: &genruntime.KnownResourceReference{
				Name: input.ClusterName,
			},
			DnsPrefix: &input.ClusterName,
			Identity: &asocontainerservicev1.ManagedClusterIdentity{
				Type: ptr.To(asocontainerservicev1.ManagedClusterIdentity_Type("SystemAssigned")),
			},
			NetworkProfile: &asocontainerservicev1.ContainerServiceNetworkProfile{
				NetworkPlugin: ptr.To(asocontainerservicev1.NetworkPlugin("azure")),
			},
			ServicePrincipalProfile: &asocontainerservicev1.ManagedClusterServicePrincipalProfile{
				ClientId: ptr.To("msi"),
			},
		},
	}
	Expect(mgmtClient.Create(ctx, managedCluster)).To(Succeed())

	By("Verifying that no CAPI resources are created initially")
	cluster := &clusterv1.Cluster{}
	err := mgmtClient.Get(ctx, types.NamespacedName{
		Namespace: input.Namespace,
		Name:      input.ClusterName,
	}, cluster)
	Expect(err).To(HaveOccurred(), "CAPI Cluster should not exist before adoption")

	By("Adding adoption annotation to trigger the managedclusteradopt controller")
	Eventually(func(g Gomega) {
		g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(managedCluster), managedCluster)).To(Succeed())
		if managedCluster.Annotations == nil {
			managedCluster.Annotations = make(map[string]string)
		}
		managedCluster.Annotations[adoptAnnotation] = adoptAnnotationValue
		g.Expect(mgmtClient.Update(ctx, managedCluster)).To(Succeed())
	}, input.WaitIntervals...).Should(Succeed())

	By("Waiting for the managedclusteradopt controller to create CAPI Cluster")
	Eventually(func(g Gomega) {
		err := mgmtClient.Get(ctx, types.NamespacedName{
			Namespace: input.Namespace,
			Name:      input.ClusterName,
		}, cluster)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(cluster.Spec.InfrastructureRef).NotTo(BeNil())
		g.Expect(cluster.Spec.InfrastructureRef.Kind).To(Equal(infrav1.AzureASOManagedClusterKind))
		g.Expect(cluster.Spec.ControlPlaneRef).NotTo(BeNil())
		g.Expect(cluster.Spec.ControlPlaneRef.Kind).To(Equal(infrav1.AzureASOManagedControlPlaneKind))
	}, input.WaitIntervals...).Should(Succeed())

	By("Verifying AzureASOManagedCluster is created")
	asoManagedCluster := &infrav1.AzureASOManagedCluster{}
	Eventually(func(g Gomega) {
		err := mgmtClient.Get(ctx, types.NamespacedName{
			Namespace: input.Namespace,
			Name:      input.ClusterName,
		}, asoManagedCluster)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(asoManagedCluster.Spec.Resources).To(HaveLen(1))
	}, input.WaitIntervals...).Should(Succeed())

	By("Verifying AzureASOManagedControlPlane is created")
	asoManagedControlPlane := &infrav1.AzureASOManagedControlPlane{}
	Eventually(func(g Gomega) {
		err := mgmtClient.Get(ctx, types.NamespacedName{
			Namespace: input.Namespace,
			Name:      input.ClusterName,
		}, asoManagedControlPlane)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(asoManagedControlPlane.Spec.Resources).To(HaveLen(1))
	}, input.WaitIntervals...).Should(Succeed())

	By("Verifying that agent pools were removed from the original ManagedCluster")
	Eventually(func(g Gomega) {
		g.Expect(mgmtClient.Get(ctx, client.ObjectKeyFromObject(managedCluster), managedCluster)).To(Succeed())
		g.Expect(managedCluster.Spec.AgentPoolProfiles).To(BeNil())
	}, input.WaitIntervals...).Should(Succeed())

	By("Cleaning up resources")
	Expect(mgmtClient.Delete(ctx, cluster)).To(Succeed())
	Expect(mgmtClient.Delete(ctx, asoManagedCluster)).To(Succeed())
	Expect(mgmtClient.Delete(ctx, asoManagedControlPlane)).To(Succeed())
	Expect(mgmtClient.Delete(ctx, managedCluster)).To(Succeed())
	Expect(mgmtClient.Delete(ctx, resourceGroup)).To(Succeed())
}
/*
Copyright 2025 The Kubernetes Authors.

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

package controllers

import (
	"context"
	"encoding/json"
	"testing"

	asoredhatopenshiftv1 "github.com/Azure/azure-service-operator/v2/api/redhatopenshift/v1api20240610preview"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	basecontrollers "sigs.k8s.io/cluster-api-provider-azure/controllers"
	cplane "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
	infrav2exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
)

// TestProviderIDListSync_AzureNameMatchesK8sName tests the case where
// HcpOpenShiftClustersNodePool.spec.azureName matches metadata.name.
// This is the backward-compatible case that should continue to work.
func TestProviderIDListSync_AzureNameMatchesK8sName(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	const (
		clusterName       = "test-cluster"
		namespace         = "default"
		nodePoolName      = "test-cluster-mp1"
		azureName         = "test-cluster-mp1" // Same as k8s name
		subscriptionID    = "00000000-0000-0000-0000-000000000000"
		resourceGroupName = "test-rg"
	)

	// Create nodes in the managed cluster with the expected HyperShift labels
	// Label: hypershift.openshift.io/nodePool=<clusterName>-<azureName>
	nodes := []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test-cluster-test-cluster-mp1-abc123",
				Labels: expectedNodeLabels(clusterName + "-" + azureName),
			},
			Spec: corev1.NodeSpec{
				ProviderID: "azure:///subscriptions/" + subscriptionID + "/resourceGroups/" + resourceGroupName + "-managed/providers/Microsoft.Compute/virtualMachines/test-cluster-test-cluster-mp1-abc123",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test-cluster-test-cluster-mp1-def456",
				Labels: expectedNodeLabels(clusterName + "-" + azureName),
			},
			Spec: corev1.NodeSpec{
				ProviderID: "azure:///subscriptions/" + subscriptionID + "/resourceGroups/" + resourceGroupName + "-managed/providers/Microsoft.Compute/virtualMachines/test-cluster-test-cluster-mp1-def456",
			},
		},
	}

	// Create fake client for managed cluster
	managedClusterScheme := runtime.NewScheme()
	_ = corev1.AddToScheme(managedClusterScheme)
	managedClusterClient := fake.NewClientBuilder().
		WithScheme(managedClusterScheme).
		WithObjects(&nodes[0], &nodes[1]).
		Build()

	// Create HcpOpenShiftClustersNodePool with azureName matching k8s name
	nodePool := &asoredhatopenshiftv1.HcpOpenShiftClustersNodePool{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HcpOpenShiftClustersNodePool",
			APIVersion: asoredhatopenshiftv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodePoolName,
			Namespace: namespace,
		},
		Spec: asoredhatopenshiftv1.HcpOpenShiftClustersNodePool_Spec{
			AzureName: azureName,
			Owner: &genruntime.KnownResourceReference{
				Name: clusterName,
			},
		},
		Status: asoredhatopenshiftv1.HcpOpenShiftClustersNodePool_STATUS{
			Properties: &asoredhatopenshiftv1.NodePoolProperties_STATUS{
				Replicas: ptr.To(2),
			},
		},
	}

	// Create management cluster objects
	aroMachinePool := createTestAROMachinePool(namespace, nodePoolName, clusterName, nodePool)
	machinePool := createTestMachinePool(namespace, nodePoolName, clusterName)
	cluster := createTestCluster(namespace, clusterName)
	aroControlPlane := createTestAROControlPlane(namespace, clusterName)
	aroCluster := createTestAROCluster(namespace, clusterName)
	aroClusterIdentity := createTestAROClusterIdentity(namespace, "test-identity")

	// Create fake client for management cluster
	mgmtScheme := runtime.NewScheme()
	_ = infrav1.AddToScheme(mgmtScheme)
	_ = infrav2exp.AddToScheme(mgmtScheme)
	_ = cplane.AddToScheme(mgmtScheme)
	_ = clusterv1.AddToScheme(mgmtScheme)
	_ = asoredhatopenshiftv1.AddToScheme(mgmtScheme)

	mgmtClient := fake.NewClientBuilder().
		WithScheme(mgmtScheme).
		WithObjects(aroMachinePool, machinePool, cluster, aroControlPlane, aroCluster, aroClusterIdentity, nodePool).
		WithStatusSubresource(aroMachinePool).
		Build()

	// Create scope
	mpScope, err := scope.NewAROMachinePoolScope(ctx, scope.AROMachinePoolScopeParams{
		Client:         mgmtClient,
		Cluster:        cluster,
		ControlPlane:   aroControlPlane,
		MachinePool:    machinePool,
		AROMachinePool: aroMachinePool,
	})
	g.Expect(err).NotTo(HaveOccurred())

	// Create fake tracker that returns our managed cluster client
	tracker := &FakeClusterTracker{
		getClientFunc: func(ctx context.Context, name types.NamespacedName) (client.Client, error) {
			return managedClusterClient, nil
		},
	}

	// Create reconciler service
	reconcilerService := &aroMachinePoolService{
		scope:      mpScope,
		kubeclient: mgmtClient,
		tracker:    tracker,
		cluster:    cluster,
		newResourceReconciler: func(machinePool *infrav2exp.AROMachinePool, resources []*unstructured.Unstructured) resourceReconciler {
			return basecontrollers.NewResourceReconciler(mgmtClient, resources, machinePool)
		},
	}

	// Run reconcile
	err = reconcilerService.Reconcile(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	// Verify providerIDList was populated
	g.Expect(aroMachinePool.Spec.ProviderIDList).To(HaveLen(2))
	g.Expect(aroMachinePool.Spec.ProviderIDList).To(ContainElement(nodes[0].Spec.ProviderID))
	g.Expect(aroMachinePool.Spec.ProviderIDList).To(ContainElement(nodes[1].Spec.ProviderID))
}

// TestProviderIDListSync_AzureNameDiffersFromK8sName tests the case where
// HcpOpenShiftClustersNodePool.spec.azureName differs from metadata.name.
// This is the Adobe use case that was previously broken.
func TestProviderIDListSync_AzureNameDiffersFromK8sName(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	const (
		clusterName       = "aro04-sbx-va7"
		namespace         = "sbx-clusters"
		nodePoolName      = "aro04-sbx-va7-workeraro-1" // K8s resource name
		azureName         = "workeraro1"                // Different Azure name
		subscriptionID    = "00000000-0000-0000-0000-000000000000"
		resourceGroupName = "aro_04_sbx_va7"
	)

	// Create nodes in the managed cluster with ARO HCP naming pattern
	// Pattern: <clusterName>-<azureName>-<suffix> (uses azureName, NOT k8s name!)
	nodes := []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "aro04-sbx-va7-workeraro1-9cg4d-qjx25",
				Labels: expectedNodeLabels(clusterName + "-" + azureName),
			},
			Spec: corev1.NodeSpec{
				ProviderID: "azure:///subscriptions/" + subscriptionID + "/resourceGroups/" + resourceGroupName + "-managed/providers/Microsoft.Compute/virtualMachines/aro04-sbx-va7-workeraro1-9cg4d-qjx25",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "aro04-sbx-va7-workeraro1-9cg4d-slvd6",
				Labels: expectedNodeLabels(clusterName + "-" + azureName),
			},
			Spec: corev1.NodeSpec{
				ProviderID: "azure:///subscriptions/" + subscriptionID + "/resourceGroups/" + resourceGroupName + "-managed/providers/Microsoft.Compute/virtualMachines/aro04-sbx-va7-workeraro1-9cg4d-slvd6",
			},
		},
	}

	// Create fake client for managed cluster
	managedClusterScheme := runtime.NewScheme()
	_ = corev1.AddToScheme(managedClusterScheme)
	managedClusterClient := fake.NewClientBuilder().
		WithScheme(managedClusterScheme).
		WithObjects(&nodes[0], &nodes[1]).
		Build()

	// Create HcpOpenShiftClustersNodePool with azureName different from k8s name
	nodePool := &asoredhatopenshiftv1.HcpOpenShiftClustersNodePool{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HcpOpenShiftClustersNodePool",
			APIVersion: asoredhatopenshiftv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodePoolName,
			Namespace: namespace,
		},
		Spec: asoredhatopenshiftv1.HcpOpenShiftClustersNodePool_Spec{
			AzureName: azureName, // Different from metadata.name!
			Owner: &genruntime.KnownResourceReference{
				Name: clusterName,
			},
		},
		Status: asoredhatopenshiftv1.HcpOpenShiftClustersNodePool_STATUS{
			Properties: &asoredhatopenshiftv1.NodePoolProperties_STATUS{
				Replicas: ptr.To(2),
			},
		},
	}

	// Create management cluster objects
	aroMachinePool := createTestAROMachinePool(namespace, nodePoolName, clusterName, nodePool)
	machinePool := createTestMachinePool(namespace, nodePoolName, clusterName)
	cluster := createTestCluster(namespace, clusterName)
	aroControlPlane := createTestAROControlPlane(namespace, clusterName)
	aroCluster := createTestAROCluster(namespace, clusterName)
	aroClusterIdentity := createTestAROClusterIdentity(namespace, "test-identity")

	// Create fake client for management cluster
	mgmtScheme := runtime.NewScheme()
	_ = infrav1.AddToScheme(mgmtScheme)
	_ = infrav2exp.AddToScheme(mgmtScheme)
	_ = cplane.AddToScheme(mgmtScheme)
	_ = clusterv1.AddToScheme(mgmtScheme)
	_ = asoredhatopenshiftv1.AddToScheme(mgmtScheme)

	mgmtClient := fake.NewClientBuilder().
		WithScheme(mgmtScheme).
		WithObjects(aroMachinePool, machinePool, cluster, aroControlPlane, aroCluster, aroClusterIdentity, nodePool).
		WithStatusSubresource(aroMachinePool).
		Build()

	// Create scope
	mpScope, err := scope.NewAROMachinePoolScope(ctx, scope.AROMachinePoolScopeParams{
		Client:         mgmtClient,
		Cluster:        cluster,
		ControlPlane:   aroControlPlane,
		MachinePool:    machinePool,
		AROMachinePool: aroMachinePool,
	})
	g.Expect(err).NotTo(HaveOccurred())

	// Create fake tracker that returns our managed cluster client
	tracker := &FakeClusterTracker{
		getClientFunc: func(ctx context.Context, name types.NamespacedName) (client.Client, error) {
			return managedClusterClient, nil
		},
	}

	// Create reconciler service
	reconcilerService := &aroMachinePoolService{
		scope:      mpScope,
		kubeclient: mgmtClient,
		tracker:    tracker,
		cluster:    cluster,
		newResourceReconciler: func(machinePool *infrav2exp.AROMachinePool, resources []*unstructured.Unstructured) resourceReconciler {
			return basecontrollers.NewResourceReconciler(mgmtClient, resources, machinePool)
		},
	}

	// Run reconcile
	err = reconcilerService.Reconcile(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	// Verify providerIDList was populated correctly
	// This test would FAIL before the fix because pattern would be:
	//   "aro04-sbx-va7-aro04-sbx-va7-workeraro-1-" (using k8s name)
	// But nodes are named:
	//   "aro04-sbx-va7-workeraro1-..." (using azureName)
	// After the fix, pattern should be:
	//   "aro04-sbx-va7-workeraro1-" (using azureName)
	g.Expect(aroMachinePool.Spec.ProviderIDList).To(HaveLen(2))
	g.Expect(aroMachinePool.Spec.ProviderIDList).To(ContainElement(nodes[0].Spec.ProviderID))
	g.Expect(aroMachinePool.Spec.ProviderIDList).To(ContainElement(nodes[1].Spec.ProviderID))
}

// TestProviderIDListSync_BaseDomainPrefixDiffersFromCAPIName tests the case where
// HcpOpenShiftCluster.status.properties.dns.baseDomainPrefix differs from the
// CAPI cluster name. The node label uses the baseDomainPrefix, so we must read
// it from the HcpOpenShiftCluster on the management cluster.
func TestProviderIDListSync_BaseDomainPrefixDiffersFromCAPIName(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	const (
		clusterName       = "ethos700-dev-va7"
		namespace         = "ethos700-dev-va7"
		nodePoolName      = "ethos700-dev-va7-watsonxcomp-1"
		azureName         = "watsonxcomp1"
		baseDomainPrefix  = "f4k6p2z2p0z9b3a" // differs from CAPI cluster name
		subscriptionID    = "00000000-0000-0000-0000-000000000000"
		resourceGroupName = "ethos_700_dev_va7"
	)

	// Nodes use baseDomainPrefix in labels, NOT the CAPI cluster name
	nodes := []corev1.Node{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:   baseDomainPrefix + "-" + azureName + "-wpcr6-95pzn",
				Labels: expectedNodeLabels(baseDomainPrefix + "-" + azureName),
			},
			Spec: corev1.NodeSpec{
				ProviderID: "azure:///subscriptions/" + subscriptionID + "/resourceGroups/" + resourceGroupName + "-managed/providers/Microsoft.Compute/virtualMachines/" + baseDomainPrefix + "-" + azureName + "-wpcr6-95pzn",
			},
		},
	}

	managedClusterScheme := runtime.NewScheme()
	_ = corev1.AddToScheme(managedClusterScheme)
	managedClusterClient := fake.NewClientBuilder().
		WithScheme(managedClusterScheme).
		WithObjects(&nodes[0]).
		Build()

	nodePool := &asoredhatopenshiftv1.HcpOpenShiftClustersNodePool{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HcpOpenShiftClustersNodePool",
			APIVersion: asoredhatopenshiftv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodePoolName,
			Namespace: namespace,
		},
		Spec: asoredhatopenshiftv1.HcpOpenShiftClustersNodePool_Spec{
			AzureName: azureName,
			Owner: &genruntime.KnownResourceReference{
				Name: clusterName,
			},
		},
		Status: asoredhatopenshiftv1.HcpOpenShiftClustersNodePool_STATUS{
			Properties: &asoredhatopenshiftv1.NodePoolProperties_STATUS{
				Replicas: ptr.To(1),
			},
		},
	}

	// HcpOpenShiftCluster on the management cluster with baseDomainPrefix in status
	hcpCluster := &asoredhatopenshiftv1.HcpOpenShiftCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
		},
		Status: asoredhatopenshiftv1.HcpOpenShiftCluster_STATUS{
			Properties: &asoredhatopenshiftv1.HcpOpenShiftClusterProperties_STATUS{
				Dns: &asoredhatopenshiftv1.DnsProfile_STATUS{
					BaseDomainPrefix: ptr.To(baseDomainPrefix),
				},
			},
		},
	}

	aroMachinePool := createTestAROMachinePool(namespace, nodePoolName, clusterName, nodePool)
	machinePool := createTestMachinePool(namespace, nodePoolName, clusterName)
	cluster := createTestCluster(namespace, clusterName)
	aroControlPlane := createTestAROControlPlane(namespace, clusterName)
	aroCluster := createTestAROCluster(namespace, clusterName)
	aroClusterIdentity := createTestAROClusterIdentity(namespace, "test-identity")

	mgmtScheme := runtime.NewScheme()
	_ = infrav1.AddToScheme(mgmtScheme)
	_ = infrav2exp.AddToScheme(mgmtScheme)
	_ = cplane.AddToScheme(mgmtScheme)
	_ = clusterv1.AddToScheme(mgmtScheme)
	_ = asoredhatopenshiftv1.AddToScheme(mgmtScheme)

	mgmtClient := fake.NewClientBuilder().
		WithScheme(mgmtScheme).
		WithObjects(aroMachinePool, machinePool, cluster, aroControlPlane, aroCluster, aroClusterIdentity, nodePool, hcpCluster).
		WithStatusSubresource(aroMachinePool).
		Build()

	mpScope, err := scope.NewAROMachinePoolScope(ctx, scope.AROMachinePoolScopeParams{
		Client:         mgmtClient,
		Cluster:        cluster,
		ControlPlane:   aroControlPlane,
		MachinePool:    machinePool,
		AROMachinePool: aroMachinePool,
	})
	g.Expect(err).NotTo(HaveOccurred())

	tracker := &FakeClusterTracker{
		getClientFunc: func(ctx context.Context, name types.NamespacedName) (client.Client, error) {
			return managedClusterClient, nil
		},
	}

	reconcilerService := &aroMachinePoolService{
		scope:      mpScope,
		kubeclient: mgmtClient,
		tracker:    tracker,
		cluster:    cluster,
		newResourceReconciler: func(machinePool *infrav2exp.AROMachinePool, resources []*unstructured.Unstructured) resourceReconciler {
			return basecontrollers.NewResourceReconciler(mgmtClient, resources, machinePool)
		},
	}

	err = reconcilerService.Reconcile(ctx)
	g.Expect(err).NotTo(HaveOccurred())

	// Without reading baseDomainPrefix, the code would construct
	// "ethos700-dev-va7-watsonxcomp1" and find zero matches.
	// With the fix, it reads "f4k6p2z2p0z9b3a" from HcpOpenShiftCluster
	// and constructs "f4k6p2z2p0z9b3a-watsonxcomp1" which matches.
	g.Expect(aroMachinePool.Spec.ProviderIDList).To(HaveLen(1))
	g.Expect(aroMachinePool.Spec.ProviderIDList).To(ContainElement(nodes[0].Spec.ProviderID))
}

// Helper functions to create test objects

func createTestAROMachinePool(namespace, name, clusterName string, nodePool *asoredhatopenshiftv1.HcpOpenShiftClustersNodePool) *infrav2exp.AROMachinePool {
	// Marshal nodePool to JSON for runtime.RawExtension
	nodePoolJSON, err := json.Marshal(nodePool)
	if err != nil {
		panic(err)
	}

	return &infrav2exp.AROMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
			},
		},
		Spec: infrav2exp.AROMachinePoolSpec{
			Resources: []runtime.RawExtension{
				{
					Raw:    nodePoolJSON,
					Object: nodePool,
				},
			},
		},
	}
}

func createTestMachinePool(namespace, name, clusterName string) *clusterv1.MachinePool {
	return &clusterv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
			},
		},
		Spec: clusterv1.MachinePoolSpec{
			ClusterName: clusterName,
			Replicas:    ptr.To[int32](2),
		},
	}
}

func createTestCluster(namespace, name string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: infrav2exp.GroupVersion.Group,
				Kind:     "AROCluster",
				Name:     name,
			},
			ControlPlaneRef: clusterv1.ContractVersionedObjectReference{
				APIGroup: cplane.GroupVersion.Group,
				Kind:     "AROControlPlane",
				Name:     name,
			},
		},
	}
}

func createTestAROControlPlane(namespace, name string) *cplane.AROControlPlane {
	return &cplane.AROControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: cplane.AROControlPlaneSpec{
			SubscriptionID: "00000000-0000-0000-0000-000000000000",
			IdentityRef: &corev1.ObjectReference{
				Name:      "test-identity",
				Namespace: namespace,
			},
		},
	}
}

func createTestAROCluster(namespace, name string) *infrav2exp.AROCluster {
	return &infrav2exp.AROCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func createTestAROClusterIdentity(namespace, name string) *infrav1.AzureClusterIdentity {
	return &infrav1.AzureClusterIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: infrav1.AzureClusterIdentitySpec{
			Type:     infrav1.ServicePrincipal,
			TenantID: "00000000-0000-0000-0000-000000000000",
			ClientID: "00000000-0000-0000-0000-000000000000",
		},
	}
}

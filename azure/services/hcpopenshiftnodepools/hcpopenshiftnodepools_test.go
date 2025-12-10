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

package hcpopenshiftnodepools

import (
	"testing"

	asoredhatopenshiftv1 "github.com/Azure/azure-service-operator/v2/api/redhatopenshift/v1api20240610preview"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	cplane "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
	v1beta2 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
)

func TestServiceName(t *testing.T) {
	g := NewWithT(t)

	s := &Service{}
	name := s.Name()

	g.Expect(name).To(Equal(serviceName))
	g.Expect(name).To(Equal("hcpopenshiftnodepools"))
}

func TestNew(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = cplane.AddToScheme(scheme)
	_ = expv1.AddToScheme(scheme)
	_ = v1beta2.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = asoredhatopenshiftv1.AddToScheme(scheme)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	controlPlane := &cplane.AROControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cp",
			Namespace: "default",
		},
		Spec: cplane.AROControlPlaneSpec{
			AroClusterName:   "test-aro-cluster",
			SubscriptionID:   "12345678-1234-1234-1234-123456789012",
			AzureEnvironment: "AzurePublicCloud",
			ChannelGroup:     "stable",
			Platform: cplane.AROPlatformProfileControlPlane{
				Location:               "eastus",
				ResourceGroup:          "test-rg",
				Subnet:                 "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.Network/virtualNetworks/test-vnet/subnets/test-subnet",
				NetworkSecurityGroupID: "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.Network/networkSecurityGroups/test-nsg",
			},
			IdentityRef: &corev1.ObjectReference{
				Name:      "test-identity",
				Namespace: "default",
				Kind:      "AzureClusterIdentity",
			},
		},
	}

	machinePool := &expv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mp",
			Namespace: "default",
		},
		Spec: expv1.MachinePoolSpec{
			Replicas: ptr.To[int32](3),
		},
	}

	aroMachinePool := &v1beta2.AROMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-amp",
			Namespace: "default",
		},
		Spec: v1beta2.AROMachinePoolSpec{
			NodePoolName: "test-amp",
			Version:      "4.14.5",
			Platform: v1beta2.AROPlatformProfileMachinePool{
				VMSize: "Standard_D4s_v3",
			},
		},
	}

	fakeIdentity := &infrav1.AzureClusterIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-identity",
			Namespace: "default",
		},
		Spec: infrav1.AzureClusterIdentitySpec{
			Type:     infrav1.WorkloadIdentity,
			ClientID: "fake-client-id",
			TenantID: "fake-tenant-id",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cluster, controlPlane, machinePool, aroMachinePool, fakeIdentity).
		Build()

	params := scope.AROMachinePoolScopeParams{
		Client:          fakeClient,
		Cluster:         cluster,
		ControlPlane:    controlPlane,
		MachinePool:     machinePool,
		AROMachinePool:  aroMachinePool,
		CredentialCache: azure.NewCredentialCache(),
	}

	aroScope, err := scope.NewAROMachinePoolScope(t.Context(), params)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(aroScope).NotTo(BeNil())

	service, err := New(aroScope)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(service).NotTo(BeNil())
	g.Expect(service.Scope).To(Equal(aroScope))
	g.Expect(service.client).To(Equal(fakeClient))
}

func TestBuildHcpOpenShiftNodePool(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = cplane.AddToScheme(scheme)
	_ = expv1.AddToScheme(scheme)
	_ = v1beta2.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = asoredhatopenshiftv1.AddToScheme(scheme)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	controlPlane := &cplane.AROControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cp",
			Namespace: "default",
			UID:       "cp-uid",
		},
		Spec: cplane.AROControlPlaneSpec{
			AroClusterName:   "test-aro-cluster",
			SubscriptionID:   "12345678-1234-1234-1234-123456789012",
			AzureEnvironment: "AzurePublicCloud",
			ChannelGroup:     "stable",
			Platform: cplane.AROPlatformProfileControlPlane{
				Location:               "eastus",
				ResourceGroup:          "test-rg",
				Subnet:                 "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.Network/virtualNetworks/test-vnet/subnets/test-subnet",
				NetworkSecurityGroupID: "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.Network/networkSecurityGroups/test-nsg",
			},
			IdentityRef: &corev1.ObjectReference{
				Name:      "test-identity",
				Namespace: "default",
				Kind:      "AzureClusterIdentity",
			},
		},
	}

	machinePool := &expv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mp",
			Namespace: "default",
		},
		Spec: expv1.MachinePoolSpec{
			Replicas: ptr.To[int32](3),
		},
	}

	aroMachinePool := &v1beta2.AROMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-amp",
			Namespace: "default",
			UID:       "amp-uid",
		},
		Spec: v1beta2.AROMachinePoolSpec{
			NodePoolName: "test-amp",
			Version:      "4.14.5",
			AutoRepair:   true,
			Labels: map[string]string{
				"key1": "value1",
			},
			Platform: v1beta2.AROPlatformProfileMachinePool{
				VMSize:                 "Standard_D4s_v3",
				AvailabilityZone:       "1",
				DiskSizeGiB:            128,
				DiskStorageAccountType: "Premium_LRS",
				Subnet:                 "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.Network/virtualNetworks/test-vnet/subnets/worker-subnet",
			},
		},
	}

	fakeIdentity := &infrav1.AzureClusterIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-identity",
			Namespace: "default",
		},
		Spec: infrav1.AzureClusterIdentitySpec{
			Type:     infrav1.WorkloadIdentity,
			ClientID: "fake-client-id",
			TenantID: "fake-tenant-id",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cluster, controlPlane, machinePool, aroMachinePool, fakeIdentity).
		Build()

	params := scope.AROMachinePoolScopeParams{
		Client:          fakeClient,
		Cluster:         cluster,
		ControlPlane:    controlPlane,
		MachinePool:     machinePool,
		AROMachinePool:  aroMachinePool,
		CredentialCache: azure.NewCredentialCache(),
	}

	aroScope, err := scope.NewAROMachinePoolScope(t.Context(), params)
	g.Expect(err).NotTo(HaveOccurred())

	service, err := New(aroScope)
	g.Expect(err).NotTo(HaveOccurred())

	nodePool, err := service.buildHcpOpenShiftNodePool(t.Context())
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(nodePool).NotTo(BeNil())
	g.Expect(nodePool.Name).To(Equal("test-amp"))
	g.Expect(nodePool.Namespace).To(Equal("default"))
	g.Expect(nodePool.Spec.AzureName).To(Equal("test-amp"))
	g.Expect(nodePool.Spec.Properties).NotTo(BeNil())
	g.Expect(nodePool.Spec.Properties.AutoRepair).To(Equal(ptr.To(true)))
	g.Expect(nodePool.Spec.Properties.Replicas).To(Equal(ptr.To(3)))
	g.Expect(nodePool.Spec.Properties.Version).NotTo(BeNil())
	g.Expect(nodePool.Spec.Properties.Version.Id).To(Equal(ptr.To("4.14.5")))
	g.Expect(nodePool.Spec.Properties.Version.ChannelGroup).To(Equal(ptr.To("stable")))
	g.Expect(nodePool.Spec.Properties.Platform).NotTo(BeNil())
	g.Expect(nodePool.Spec.Properties.Platform.VmSize).To(Equal(ptr.To("Standard_D4s_v3")))
	g.Expect(nodePool.Spec.Properties.Platform.AvailabilityZone).To(Equal(ptr.To("1")))
	g.Expect(nodePool.Spec.Properties.Labels).To(HaveLen(1))
	g.Expect(nodePool.OwnerReferences).To(HaveLen(1))
	g.Expect(nodePool.OwnerReferences[0].UID).To(Equal(aroMachinePool.UID))
}

func TestGetNodePoolName(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = cplane.AddToScheme(scheme)
	_ = expv1.AddToScheme(scheme)
	_ = v1beta2.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "default",
		},
	}

	controlPlane := &cplane.AROControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cp",
			Namespace: "default",
		},
		Spec: cplane.AROControlPlaneSpec{
			AroClusterName:   "test-aro-cluster",
			SubscriptionID:   "12345678-1234-1234-1234-123456789012",
			AzureEnvironment: "AzurePublicCloud",
			Platform: cplane.AROPlatformProfileControlPlane{
				Location:               "eastus",
				ResourceGroup:          "test-rg",
				Subnet:                 "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.Network/virtualNetworks/test-vnet/subnets/test-subnet",
				NetworkSecurityGroupID: "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/test-rg/providers/Microsoft.Network/networkSecurityGroups/test-nsg",
			},
			IdentityRef: &corev1.ObjectReference{
				Name:      "test-identity",
				Namespace: "default",
				Kind:      "AzureClusterIdentity",
			},
		},
	}

	machinePool := &expv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mp",
			Namespace: "default",
		},
		Spec: expv1.MachinePoolSpec{
			Replicas: ptr.To[int32](3),
		},
	}

	aroMachinePool := &v1beta2.AROMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-worker-pool",
			Namespace: "default",
		},
		Spec: v1beta2.AROMachinePoolSpec{
			NodePoolName: "my-worker-pool",
			Platform: v1beta2.AROPlatformProfileMachinePool{
				VMSize: "Standard_D4s_v3",
			},
		},
	}

	fakeIdentity := &infrav1.AzureClusterIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-identity",
			Namespace: "default",
		},
		Spec: infrav1.AzureClusterIdentitySpec{
			Type:     infrav1.WorkloadIdentity,
			ClientID: "fake-client-id",
			TenantID: "fake-tenant-id",
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cluster, controlPlane, machinePool, aroMachinePool, fakeIdentity).
		Build()

	params := scope.AROMachinePoolScopeParams{
		Client:          fakeClient,
		Cluster:         cluster,
		ControlPlane:    controlPlane,
		MachinePool:     machinePool,
		AROMachinePool:  aroMachinePool,
		CredentialCache: azure.NewCredentialCache(),
	}

	aroScope, err := scope.NewAROMachinePoolScope(t.Context(), params)
	g.Expect(err).NotTo(HaveOccurred())

	service, err := New(aroScope)
	g.Expect(err).NotTo(HaveOccurred())

	nodePoolName := service.getNodePoolName()
	g.Expect(nodePoolName).To(Equal("my-worker-pool"))
}

func TestFindCondition(t *testing.T) {
	g := NewWithT(t)

	conditionsList := []conditions.Condition{
		{
			Type:   conditions.ConditionTypeReady,
			Status: metav1.ConditionTrue,
		},
		{
			Type:   "CustomCondition",
			Status: metav1.ConditionFalse,
		},
	}

	// Test finding existing condition
	condition := findCondition(conditionsList, conditions.ConditionTypeReady)
	g.Expect(condition).NotTo(BeNil())
	g.Expect(string(condition.Type)).To(Equal(string(conditions.ConditionTypeReady)))
	g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))

	// Test finding non-existent condition
	condition = findCondition(conditionsList, "NonExistent")
	g.Expect(condition).To(BeNil())
}

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

package hcpopenshiftclusters

import (
	"testing"

	asoredhatopenshiftv1 "github.com/Azure/azure-service-operator/v2/api/redhatopenshift/v1api20240610preview"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	cplane "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
)

func TestServiceName(t *testing.T) {
	g := NewWithT(t)

	s := &Service{}
	name := s.Name()

	g.Expect(name).To(Equal(serviceName))
	g.Expect(name).To(Equal("hcpopenshiftclusters"))
}

func TestNew(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = cplane.AddToScheme(scheme)
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
		WithObjects(cluster, controlPlane, fakeIdentity).
		Build()

	params := scope.AROControlPlaneScopeParams{
		Client:          fakeClient,
		Cluster:         cluster,
		ControlPlane:    controlPlane,
		CredentialCache: azure.NewCredentialCache(),
	}

	aroScope, err := scope.NewAROControlPlaneScope(t.Context(), params)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(aroScope).NotTo(BeNil())

	service, err := New(aroScope)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(service).NotTo(BeNil())
	g.Expect(service.Scope).To(Equal(aroScope))
	g.Expect(service.client).To(Equal(fakeClient))
}

func TestBuildHcpOpenShiftCluster(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = cplane.AddToScheme(scheme)
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
			UID:       "test-uid",
		},
		Spec: cplane.AROControlPlaneSpec{
			AroClusterName:   "test-aro-cluster",
			SubscriptionID:   "12345678-1234-1234-1234-123456789012",
			AzureEnvironment: "AzurePublicCloud",
			Version:          "4.14.5",
			ChannelGroup:     "stable",
			Visibility:       "Public",
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
		WithObjects(cluster, controlPlane, fakeIdentity).
		Build()

	params := scope.AROControlPlaneScopeParams{
		Client:          fakeClient,
		Cluster:         cluster,
		ControlPlane:    controlPlane,
		CredentialCache: azure.NewCredentialCache(),
	}

	aroScope, err := scope.NewAROControlPlaneScope(t.Context(), params)
	g.Expect(err).NotTo(HaveOccurred())

	service, err := New(aroScope)
	g.Expect(err).NotTo(HaveOccurred())

	hcpCluster, err := service.buildHcpOpenShiftCluster(t.Context())
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(hcpCluster).NotTo(BeNil())
	g.Expect(hcpCluster.Name).To(Equal("test-aro-cluster"))
	g.Expect(hcpCluster.Namespace).To(Equal("default"))
	g.Expect(hcpCluster.Spec.AzureName).To(Equal("test-aro-cluster"))
	g.Expect(hcpCluster.Spec.Location).To(Equal(ptr.To("eastus")))
	g.Expect(hcpCluster.Spec.Properties).NotTo(BeNil())
	g.Expect(hcpCluster.Spec.Properties.Version).NotTo(BeNil())
	g.Expect(hcpCluster.Spec.Properties.Version.Id).To(Equal(ptr.To("4.14.5")))
	g.Expect(hcpCluster.Spec.Properties.Version.ChannelGroup).To(Equal(ptr.To("stable")))
	g.Expect(hcpCluster.Spec.OperatorSpec).NotTo(BeNil())
	g.Expect(hcpCluster.Spec.OperatorSpec.Secrets).NotTo(BeNil())
	g.Expect(hcpCluster.Spec.OperatorSpec.Secrets.AdminCredentials).NotTo(BeNil())
	g.Expect(hcpCluster.Spec.OperatorSpec.Secrets.AdminCredentials.Name).To(Equal(secret.Name(cluster.Name, secret.Kubeconfig)))
	g.Expect(hcpCluster.Spec.OperatorSpec.Secrets.AdminCredentials.Key).To(Equal(secret.KubeconfigDataName))
	g.Expect(hcpCluster.OwnerReferences).To(HaveLen(1))
	g.Expect(hcpCluster.OwnerReferences[0].UID).To(Equal(controlPlane.UID))
}

func TestGetClusterName(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = cplane.AddToScheme(scheme)
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
			AroClusterName:   "my-aro-cluster",
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
		WithObjects(cluster, controlPlane, fakeIdentity).
		Build()

	params := scope.AROControlPlaneScopeParams{
		Client:          fakeClient,
		Cluster:         cluster,
		ControlPlane:    controlPlane,
		CredentialCache: azure.NewCredentialCache(),
	}

	aroScope, err := scope.NewAROControlPlaneScope(t.Context(), params)
	g.Expect(err).NotTo(HaveOccurred())

	service, err := New(aroScope)
	g.Expect(err).NotTo(HaveOccurred())

	clusterName := service.getClusterName()
	g.Expect(clusterName).To(Equal("my-aro-cluster"))
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

func TestEnsureKubeconfigSecretWithOwner(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = cplane.AddToScheme(scheme)
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

	hcpCluster := &asoredhatopenshiftv1.HcpOpenShiftCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-aro-cluster",
			Namespace: "default",
			UID:       "hcp-cluster-uid",
		},
	}

	t.Run("creates secret when it doesn't exist", func(t *testing.T) {
		g := NewWithT(t)

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(cluster, controlPlane, fakeIdentity, hcpCluster).
			Build()

		params := scope.AROControlPlaneScopeParams{
			Client:          fakeClient,
			Cluster:         cluster,
			ControlPlane:    controlPlane,
			CredentialCache: azure.NewCredentialCache(),
		}

		aroScope, err := scope.NewAROControlPlaneScope(t.Context(), params)
		g.Expect(err).NotTo(HaveOccurred())

		service, err := New(aroScope)
		g.Expect(err).NotTo(HaveOccurred())

		created, err := service.ensureKubeconfigSecretWithOwner(t.Context(), hcpCluster)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(created).To(BeTrue())

		// Verify secret was created
		secretName := secret.Name(cluster.Name, secret.Kubeconfig)
		createdSecret := &corev1.Secret{}
		err = fakeClient.Get(t.Context(), client.ObjectKey{
			Namespace: "default",
			Name:      secretName,
		}, createdSecret)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(createdSecret.OwnerReferences).To(HaveLen(1))
		g.Expect(createdSecret.OwnerReferences[0].UID).To(Equal(hcpCluster.UID))
	})

	t.Run("adds owner when secret exists without owner", func(t *testing.T) {
		g := NewWithT(t)

		existingSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secret.Name(cluster.Name, secret.Kubeconfig),
				Namespace: "default",
			},
			Data: map[string][]byte{
				secret.KubeconfigDataName: []byte("fake-kubeconfig"),
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(cluster, controlPlane, fakeIdentity, hcpCluster, existingSecret).
			Build()

		params := scope.AROControlPlaneScopeParams{
			Client:          fakeClient,
			Cluster:         cluster,
			ControlPlane:    controlPlane,
			CredentialCache: azure.NewCredentialCache(),
		}

		aroScope, err := scope.NewAROControlPlaneScope(t.Context(), params)
		g.Expect(err).NotTo(HaveOccurred())

		service, err := New(aroScope)
		g.Expect(err).NotTo(HaveOccurred())

		created, err := service.ensureKubeconfigSecretWithOwner(t.Context(), hcpCluster)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(created).To(BeTrue())

		// Verify owner was added
		updatedSecret := &corev1.Secret{}
		err = fakeClient.Get(t.Context(), client.ObjectKey{
			Namespace: "default",
			Name:      existingSecret.Name,
		}, updatedSecret)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(updatedSecret.OwnerReferences).To(HaveLen(1))
		g.Expect(updatedSecret.OwnerReferences[0].UID).To(Equal(hcpCluster.UID))
	})

	t.Run("returns false when secret already has correct owner", func(t *testing.T) {
		g := NewWithT(t)

		existingSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secret.Name(cluster.Name, secret.Kubeconfig),
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: asoredhatopenshiftv1.GroupVersion.Identifier(),
						Kind:       "HcpOpenShiftCluster",
						Name:       hcpCluster.Name,
						UID:        hcpCluster.UID,
					},
				},
			},
			Data: map[string][]byte{
				secret.KubeconfigDataName: []byte("fake-kubeconfig"),
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(cluster, controlPlane, fakeIdentity, hcpCluster, existingSecret).
			Build()

		params := scope.AROControlPlaneScopeParams{
			Client:          fakeClient,
			Cluster:         cluster,
			ControlPlane:    controlPlane,
			CredentialCache: azure.NewCredentialCache(),
		}

		aroScope, err := scope.NewAROControlPlaneScope(t.Context(), params)
		g.Expect(err).NotTo(HaveOccurred())

		service, err := New(aroScope)
		g.Expect(err).NotTo(HaveOccurred())

		created, err := service.ensureKubeconfigSecretWithOwner(t.Context(), hcpCluster)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(created).To(BeFalse())
	})
}

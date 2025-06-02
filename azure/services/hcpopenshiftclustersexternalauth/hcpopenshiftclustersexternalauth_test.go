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

package hcpopenshiftclustersexternalauth

import (
	"testing"

	asoredhatopenshiftv1 "github.com/Azure/azure-service-operator/v2/api/redhatopenshift/v1api20240610preview"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
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
	g.Expect(name).To(Equal("hcpopenshiftclustersexternalauth"))
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

func TestBuildHcpOpenShiftClustersExternalAuth(t *testing.T) {
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
			EnableExternalAuthProviders: true,
			ExternalAuthProviders: []cplane.ExternalAuthProvider{
				{
					Name: "test-provider",
					Issuer: cplane.TokenIssuer{
						URL:       "https://issuer.example.com",
						Audiences: []cplane.TokenAudience{"audience1", "audience2"},
					},
					ClaimMappings: &cplane.TokenClaimMappings{
						Username: &cplane.UsernameClaimMapping{
							Claim:        "sub",
							PrefixPolicy: cplane.Prefix,
							Prefix:       ptr.To("test-prefix:"),
						},
						Groups: &cplane.PrefixedClaimMapping{
							Claim:  "groups",
							Prefix: "group-prefix:",
						},
					},
					ClaimValidationRules: []cplane.TokenClaimValidationRule{
						{
							Type: cplane.TokenValidationRuleTypeRequiredClaim,
							RequiredClaim: cplane.TokenRequiredClaim{
								Claim:         "iss",
								RequiredValue: "https://issuer.example.com",
							},
						},
					},
					OIDCClients: []cplane.OIDCClientConfig{
						{
							ComponentName:      "console",
							ComponentNamespace: "openshift-console",
							ClientID:           "console-client",
							ClientSecret: cplane.LocalObjectReference{
								Name: "console-client-secret",
							},
							ExtraScopes: []string{"profile", "email"},
						},
					},
				},
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

	externalAuth, err := service.buildHcpOpenShiftClustersExternalAuth(t.Context(), controlPlane.Spec.ExternalAuthProviders[0])
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(externalAuth).NotTo(BeNil())
	g.Expect(externalAuth.Name).To(Equal("test-cluster-test-provider"))
	g.Expect(externalAuth.Namespace).To(Equal("default"))
	g.Expect(externalAuth.Spec.AzureName).To(Equal("test-provider"))
	g.Expect(externalAuth.Spec.Owner).NotTo(BeNil())
	g.Expect(externalAuth.Spec.Owner.Name).To(Equal("test-cluster"))

	// Verify issuer configuration
	g.Expect(externalAuth.Spec.Properties).NotTo(BeNil())
	g.Expect(externalAuth.Spec.Properties.Issuer).NotTo(BeNil())
	g.Expect(externalAuth.Spec.Properties.Issuer.Url).To(Equal(ptr.To("https://issuer.example.com")))
	g.Expect(externalAuth.Spec.Properties.Issuer.Audiences).To(Equal([]string{"audience1", "audience2"}))

	// Verify claim mappings
	g.Expect(externalAuth.Spec.Properties.Claim).NotTo(BeNil())
	g.Expect(externalAuth.Spec.Properties.Claim.Mappings).NotTo(BeNil())
	g.Expect(externalAuth.Spec.Properties.Claim.Mappings.Username).NotTo(BeNil())
	g.Expect(externalAuth.Spec.Properties.Claim.Mappings.Username.Claim).To(Equal(ptr.To("sub")))
	g.Expect(externalAuth.Spec.Properties.Claim.Mappings.Username.PrefixPolicy).To(Equal(ptr.To(asoredhatopenshiftv1.UsernameClaimPrefixPolicy_Prefix)))
	g.Expect(externalAuth.Spec.Properties.Claim.Mappings.Username.Prefix).To(Equal(ptr.To("test-prefix:")))

	g.Expect(externalAuth.Spec.Properties.Claim.Mappings.Groups).NotTo(BeNil())
	g.Expect(externalAuth.Spec.Properties.Claim.Mappings.Groups.Claim).To(Equal(ptr.To("groups")))
	g.Expect(externalAuth.Spec.Properties.Claim.Mappings.Groups.Prefix).To(Equal(ptr.To("group-prefix:")))

	// Verify validation rules
	g.Expect(externalAuth.Spec.Properties.Claim.ValidationRules).To(HaveLen(1))
	g.Expect(externalAuth.Spec.Properties.Claim.ValidationRules[0].Type).To(Equal(ptr.To(asoredhatopenshiftv1.TokenClaimValidationRule_Type_RequiredClaim)))
	g.Expect(externalAuth.Spec.Properties.Claim.ValidationRules[0].RequiredClaim).NotTo(BeNil())
	g.Expect(externalAuth.Spec.Properties.Claim.ValidationRules[0].RequiredClaim.Claim).To(Equal(ptr.To("iss")))
	g.Expect(externalAuth.Spec.Properties.Claim.ValidationRules[0].RequiredClaim.RequiredValue).To(Equal(ptr.To("https://issuer.example.com")))

	// Verify OIDC clients
	g.Expect(externalAuth.Spec.Properties.Clients).To(HaveLen(1))
	g.Expect(externalAuth.Spec.Properties.Clients[0].ClientId).To(Equal(ptr.To("console-client")))
	g.Expect(externalAuth.Spec.Properties.Clients[0].Component).NotTo(BeNil())
	g.Expect(externalAuth.Spec.Properties.Clients[0].Component.Name).To(Equal(ptr.To("console")))
	g.Expect(externalAuth.Spec.Properties.Clients[0].Component.AuthClientNamespace).To(Equal(ptr.To("openshift-console")))
	g.Expect(externalAuth.Spec.Properties.Clients[0].ExtraScopes).To(Equal([]string{"profile", "email"}))
}

func TestIsManaged(t *testing.T) {
	tests := []struct {
		name                        string
		enableExternalAuthProviders bool
		expected                    bool
	}{
		{
			name:                        "external auth enabled",
			enableExternalAuthProviders: true,
			expected:                    true,
		},
		{
			name:                        "external auth disabled",
			enableExternalAuthProviders: false,
			expected:                    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
					AroClusterName:              "test-aro-cluster",
					SubscriptionID:              "12345678-1234-1234-1234-123456789012",
					AzureEnvironment:            "AzurePublicCloud",
					EnableExternalAuthProviders: tt.enableExternalAuthProviders,
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

			managed, err := service.IsManaged(t.Context())
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(managed).To(Equal(tt.expected))
		})
	}
}

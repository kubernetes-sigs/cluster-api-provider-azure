/*
Copyright 2021 The Kubernetes Authors.

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

package scope

import (
	"context"
	"testing"

	aadpodid "github.com/Azure/aad-pod-identity/pkg/apis/aadpodidentity"
	aadpodv1 "github.com/Azure/aad-pod-identity/pkg/apis/aadpodidentity/v1"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAllowedNamespaces(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	_ = infrav1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name             string
		identity         *infrav1.AzureClusterIdentity
		clusterNamespace string
		expected         bool
	}{
		{
			name: "allow any cluster namespace when empty",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					AllowedNamespaces: &infrav1.AllowedNamespaces{},
				},
			},
			clusterNamespace: "default",
			expected:         true,
		},
		{
			name: "no namespaces allowed when list is empty",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					AllowedNamespaces: &infrav1.AllowedNamespaces{
						NamespaceList: []string{},
					},
				},
			},
			clusterNamespace: "default",
			expected:         false,
		},
		{
			name: "allow cluster with namespace in list",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					AllowedNamespaces: &infrav1.AllowedNamespaces{
						NamespaceList: []string{"namespace24", "namespace32"},
					},
				},
			},
			clusterNamespace: "namespace24",
			expected:         true,
		},
		{
			name: "don't allow cluster with namespace not in list",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					AllowedNamespaces: &infrav1.AllowedNamespaces{
						NamespaceList: []string{"namespace24", "namespace32"},
					},
				},
			},
			clusterNamespace: "namespace8",
			expected:         false,
		},
		{
			name: "allow cluster when namespace has selector with matching label",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					AllowedNamespaces: &infrav1.AllowedNamespaces{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"c": "d"},
						},
					},
				},
			},
			clusterNamespace: "namespace8",
			expected:         true,
		},
		{
			name: "don't allow cluster when namespace has selector with different label",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					AllowedNamespaces: &infrav1.AllowedNamespaces{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"x": "y"},
						},
					},
				},
			},
			clusterNamespace: "namespace8",
			expected:         false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeNamespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "namespace8",
					Labels: map[string]string{"c": "d"},
				},
			}
			initObjects := []runtime.Object{tc.identity, fakeNamespace}
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

			actual := IsClusterNamespaceAllowed(context.TODO(), fakeClient, tc.identity.Spec.AllowedNamespaces, tc.clusterNamespace)
			g.Expect(actual).To(Equal(tc.expected))
		})
	}
}

func TestCreateAzureIdentityWithBindings(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	_ = infrav1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = aadpodv1.AddToScheme(scheme)

	tests := []struct {
		name                    string
		identity                *infrav1.AzureClusterIdentity
		identityType            aadpodv1.IdentityType
		resourceManagerEndpoint string
		activeDirectoryEndpoint string
		clusterMeta             metav1.ObjectMeta
		copiedIdentity          metav1.ObjectMeta
		bindings                []metav1.ObjectMeta
		expectedErr             bool
	}{
		{
			name: "create service principal identity",
			identity: &infrav1.AzureClusterIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-identity",
				},
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:         infrav1.ServicePrincipal,
					ResourceID:   "my-resource-id",
					ClientID:     "my-client-id",
					ClientSecret: corev1.SecretReference{Name: "my-client-secret"},
					TenantID:     "my-tenant-id",
				},
			},
			identityType:            aadpodv1.ServicePrincipal,
			resourceManagerEndpoint: "public-cloud-endpoint",
			activeDirectoryEndpoint: "active-directory-endpoint",
			clusterMeta: metav1.ObjectMeta{
				Name:      "cluster-name",
				Namespace: "my-namespace",
			},
			copiedIdentity: metav1.ObjectMeta{
				Name:      "cluster-name-my-namespace-test-identity",
				Namespace: "capz-system",
			},
			bindings: []metav1.ObjectMeta{
				{
					Name:      "cluster-name-my-namespace-test-identity-binding",
					Namespace: "capz-system",
				},
				{
					Name:      "cluster-name-my-namespace-test-identity-aso-binding",
					Namespace: "capz-system",
				},
			},
		},
		{
			name: "create UAMI identity",
			identity: &infrav1.AzureClusterIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-identity",
				},
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:         infrav1.UserAssignedMSI,
					ResourceID:   "my-resource-id",
					ClientID:     "my-client-id",
					ClientSecret: corev1.SecretReference{Name: "my-client-secret"},
					TenantID:     "my-tenant-id",
				},
			},
			identityType:            aadpodv1.UserAssignedMSI,
			resourceManagerEndpoint: "public-cloud-endpoint",
			activeDirectoryEndpoint: "active-directory-endpoint",
			clusterMeta: metav1.ObjectMeta{
				Name:      "cluster-name",
				Namespace: "my-namespace",
			},
			copiedIdentity: metav1.ObjectMeta{
				Name:      "cluster-name-my-namespace-test-identity",
				Namespace: "capz-system",
			},
			bindings: []metav1.ObjectMeta{
				{
					Name:      "cluster-name-my-namespace-test-identity-binding",
					Namespace: "capz-system",
				},
				{
					Name:      "cluster-name-my-namespace-test-identity-aso-binding",
					Namespace: "capz-system",
				},
			},
		},
		{
			name: "create service principal with certificate identity",
			identity: &infrav1.AzureClusterIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-identity",
				},
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:         infrav1.ServicePrincipalCertificate,
					ResourceID:   "my-resource-id",
					ClientID:     "my-client-id",
					ClientSecret: corev1.SecretReference{Name: "my-client-secret"},
					TenantID:     "my-tenant-id",
				},
			},
			identityType:            aadpodv1.IdentityType(aadpodid.ServicePrincipalCertificate),
			resourceManagerEndpoint: "public-cloud-endpoint",
			activeDirectoryEndpoint: "active-directory-endpoint",
			clusterMeta: metav1.ObjectMeta{
				Name:      "cluster-name",
				Namespace: "my-namespace",
			},
			copiedIdentity: metav1.ObjectMeta{
				Name:      "cluster-name-my-namespace-test-identity",
				Namespace: "capz-system",
			},
			bindings: []metav1.ObjectMeta{
				{
					Name:      "cluster-name-my-namespace-test-identity-binding",
					Namespace: "capz-system",
				},
				{
					Name:      "cluster-name-my-namespace-test-identity-aso-binding",
					Namespace: "capz-system",
				},
			},
		},
		{
			name: "invalid identity type",
			identity: &infrav1.AzureClusterIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-identity",
				},
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:         "fooIdentity",
					ResourceID:   "my-resource-id",
					ClientID:     "my-client-id",
					ClientSecret: corev1.SecretReference{Name: "my-client-secret"},
					TenantID:     "my-tenant-id",
				},
			},
			identityType: -1,
			expectedErr:  true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			initObjects := []runtime.Object{tc.identity}
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

			err := createAzureIdentityWithBindings(context.TODO(), tc.identity, tc.resourceManagerEndpoint, tc.activeDirectoryEndpoint, tc.clusterMeta, fakeClient)
			if !tc.expectedErr {
				g.Expect(err).To(BeNil())

				resultIdentity := &aadpodv1.AzureIdentity{}
				key := client.ObjectKey{Name: tc.copiedIdentity.Name, Namespace: tc.copiedIdentity.Namespace}
				g.Expect(fakeClient.Get(context.TODO(), key, resultIdentity)).To(Succeed())
				g.Expect(resultIdentity.Spec.Type).To(Equal(tc.identityType))
				g.Expect(resultIdentity.Spec.ResourceID).To(Equal(tc.identity.Spec.ResourceID))
				g.Expect(resultIdentity.Spec.ClientID).To(Equal(tc.identity.Spec.ClientID))
				g.Expect(resultIdentity.Spec.ClientPassword).To(Equal(tc.identity.Spec.ClientSecret))
				g.Expect(resultIdentity.Spec.TenantID).To(Equal(tc.identity.Spec.TenantID))
				g.Expect(resultIdentity.Spec.ADResourceID).To(Equal(tc.resourceManagerEndpoint))
				g.Expect(resultIdentity.Spec.ADEndpoint).To(Equal(tc.activeDirectoryEndpoint))

				for _, binding := range tc.bindings {
					resultIdentityBinding := &aadpodv1.AzureIdentityBinding{}
					key = client.ObjectKey{Name: binding.Name, Namespace: binding.Namespace}
					g.Expect(fakeClient.Get(context.TODO(), key, resultIdentityBinding)).To(Succeed())
				}

				// no error if identity already exists
				err = createAzureIdentityWithBindings(context.TODO(), tc.identity, tc.resourceManagerEndpoint, tc.activeDirectoryEndpoint, tc.clusterMeta, fakeClient)
				g.Expect(err).To(BeNil())
			} else {
				g.Expect(err).NotTo(BeNil())
			}
		})
	}
}

func TestHasClientSecret(t *testing.T) {
	tests := []struct {
		name     string
		identity *infrav1.AzureClusterIdentity
		want     bool
	}{
		{
			name: "user assigned identity",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:       infrav1.UserAssignedMSI,
					ResourceID: "my-resource-id",
				},
			},
			want: false,
		},
		{
			name: "service principal with secret",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:         infrav1.ServicePrincipal,
					ClientSecret: corev1.SecretReference{Name: "my-client-secret"},
				},
			},
			want: true,
		},
		{
			name: "service principal with certificate",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:         infrav1.ServicePrincipalCertificate,
					ClientSecret: corev1.SecretReference{Name: "my-client-secret"},
				},
			},
			want: false,
		},
		{
			name: "manual service principal",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:         infrav1.ManualServicePrincipal,
					ClientSecret: corev1.SecretReference{Name: "my-client-secret"},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &AzureCredentialsProvider{
				Identity: tt.identity,
			}
			if got := p.hasClientSecret(); got != tt.want {
				t.Errorf("AzureCredentialsProvider.hasClientSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

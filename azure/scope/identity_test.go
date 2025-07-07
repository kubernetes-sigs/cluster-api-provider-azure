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
	"os"
	"reflect"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
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

			actual := IsClusterNamespaceAllowed(t.Context(), fakeClient, tc.identity.Spec.AllowedNamespaces, tc.clusterNamespace)
			g.Expect(actual).To(Equal(tc.expected))
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
					Type: infrav1.UserAssignedMSI,
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
					Type: infrav1.ServicePrincipalCertificate,
				},
			},
			want: true,
		},
		{
			name: "service principal with certificate path",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.ServicePrincipalCertificate,
					CertPath: "something",
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

func TestGetTokenCredential(t *testing.T) {
	testCertPath := "../../test/setup/certificate"

	tests := []struct {
		name                         string
		cluster                      *infrav1.AzureCluster
		secret                       *corev1.Secret
		identity                     *infrav1.AzureClusterIdentity
		ActiveDirectoryAuthorityHost string
		cacheExpect                  func(*mock_azure.MockCredentialCache)
	}{
		{
			name: "workload identity",
			cluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
				},
			},
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.WorkloadIdentity,
					ClientID: fakeClientID,
					TenantID: fakeTenantID,
				},
			},
			cacheExpect: func(cache *mock_azure.MockCredentialCache) {
				cache.EXPECT().GetOrStoreWorkloadIdentity(gomock.Cond(func(opts *azidentity.WorkloadIdentityCredentialOptions) bool {
					// ignore tracing provider
					return opts.TenantID == fakeTenantID &&
						opts.ClientID == fakeClientID &&
						opts.TokenFilePath == GetProjectedTokenPath()
				}))
			},
		},
		{
			name: "manual service principal",
			cluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
				},
			},
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.ManualServicePrincipal,
					TenantID: fakeTenantID,
					ClientID: fakeClientID,
					ClientSecret: corev1.SecretReference{
						Name: "test-identity-secret",
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-identity-secret",
				},
				Data: map[string][]byte{
					"clientSecret": []byte("fooSecret"),
				},
			},
			ActiveDirectoryAuthorityHost: "https://login.microsoftonline.com",
			cacheExpect: func(cache *mock_azure.MockCredentialCache) {
				cache.EXPECT().GetOrStoreClientSecret(fakeTenantID, fakeClientID, "fooSecret", gomock.Cond(func(opts *azidentity.ClientSecretCredentialOptions) bool {
					// ignore tracing provider
					return reflect.DeepEqual(opts.ClientOptions.Cloud, cloud.Configuration{
						ActiveDirectoryAuthorityHost: "https://login.microsoftonline.com",
						Services: map[cloud.ServiceName]cloud.ServiceConfiguration{
							cloud.ResourceManager: {
								Audience: "",
								Endpoint: "",
							},
						},
					})
				}))
			},
		},
		{
			name: "service principal",
			cluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
				},
			},
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.ServicePrincipal,
					TenantID: fakeTenantID,
					ClientID: fakeClientID,
					ClientSecret: corev1.SecretReference{
						Name: "test-identity-secret",
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-identity-secret",
				},
				Data: map[string][]byte{
					"clientSecret": []byte("fooSecret"),
				},
			},
			ActiveDirectoryAuthorityHost: "https://login.microsoftonline.com",
			cacheExpect: func(cache *mock_azure.MockCredentialCache) {
				cache.EXPECT().GetOrStoreClientSecret(fakeTenantID, fakeClientID, "fooSecret", gomock.Cond(func(opts *azidentity.ClientSecretCredentialOptions) bool {
					// ignore tracing provider
					return reflect.DeepEqual(opts.ClientOptions.Cloud, cloud.Configuration{
						ActiveDirectoryAuthorityHost: "https://login.microsoftonline.com",
						Services: map[cloud.ServiceName]cloud.ServiceConfiguration{
							cloud.ResourceManager: {
								Audience: "",
								Endpoint: "",
							},
						},
					})
				}))
			},
		},
		{
			name: "service principal certificate",
			cluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
				},
			},
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.ServicePrincipalCertificate,
					TenantID: fakeTenantID,
					ClientID: fakeClientID,
					ClientSecret: corev1.SecretReference{
						Name: "test-identity-secret",
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-identity-secret",
				},
				Data: map[string][]byte{
					"clientSecret": []byte("fooSecret"),
				},
			},
			cacheExpect: func(cache *mock_azure.MockCredentialCache) {
				cache.EXPECT().GetOrStoreClientCert(fakeTenantID, fakeClientID, []byte("fooSecret"), gomock.Nil(), gomock.Any())
			},
		},
		{
			name: "service principal certificate with certificate filepath",
			cluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
				},
			},
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.ServicePrincipalCertificate,
					TenantID: fakeTenantID,
					ClientID: fakeClientID,
					CertPath: testCertPath,
				},
			},
			cacheExpect: func(cache *mock_azure.MockCredentialCache) {
				expectedCert, err := os.ReadFile(testCertPath)
				if err != nil {
					panic(err)
				}
				cache.EXPECT().GetOrStoreClientCert(fakeTenantID, fakeClientID, expectedCert, gomock.Nil(), gomock.Any())
			},
		},
		{
			name: "user-assigned identity",
			cluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
				},
			},
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.UserAssignedMSI,
					TenantID: fakeTenantID,
					ClientID: fakeClientID,
				},
			},
			cacheExpect: func(cache *mock_azure.MockCredentialCache) {
				cache.EXPECT().GetOrStoreManagedIdentity(gomock.Cond(func(opts *azidentity.ManagedIdentityCredentialOptions) bool {
					// ignore tracing provider
					return opts.ID == azidentity.ClientID(fakeClientID)
				}))
			},
		},
		{
			name: "UserAssignedIdentityCredential",
			cluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
				},
			},
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:                                     infrav1.UserAssignedIdentityCredential,
					UserAssignedIdentityCredentialsPath:      "../../test/setup/credentials.json",
					UserAssignedIdentityCredentialsCloudType: "public",
				},
			},
			cacheExpect: func(cache *mock_azure.MockCredentialCache) {
				ctx := context.Background()                      //nolint:usetesting
				credsPath := "../../test/setup/credentials.json" //nolint:gosec
				clientOptions := azcore.ClientOptions{
					Cloud: cloud.Configuration{
						ActiveDirectoryAuthorityHost: "https://login.microsoftonline.com/",
						Services: map[cloud.ServiceName]cloud.ServiceConfiguration{
							cloud.ResourceManager: {
								Audience: "https://management.core.windows.net/",
								Endpoint: "https://management.azure.com",
							},
						},
					},
				}
				cache.EXPECT().GetOrStoreUserAssignedManagedIdentityCredentials(ctx, credsPath, gomock.Cond(func(opts azcore.ClientOptions) bool {
					return opts.Cloud.ActiveDirectoryAuthorityHost == clientOptions.Cloud.ActiveDirectoryAuthorityHost &&
						opts.Cloud.Services[cloud.ResourceManager].Audience == clientOptions.Cloud.Services[cloud.ResourceManager].Audience &&
						opts.Cloud.Services[cloud.ResourceManager].Endpoint == clientOptions.Cloud.Services[cloud.ResourceManager].Endpoint
				}), gomock.Any())
			},
		},
	}

	scheme := runtime.NewScheme()
	_ = infrav1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			initObjects := []runtime.Object{tt.cluster}
			if tt.identity != nil {
				initObjects = append(initObjects, tt.identity)
			}
			if tt.secret != nil {
				initObjects = append(initObjects, tt.secret)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

			mockCtrl := gomock.NewController(t)
			cache := mock_azure.NewMockCredentialCache(mockCtrl)
			tt.cacheExpect(cache)

			provider, err := NewAzureCredentialsProvider(t.Context(), cache, fakeClient, tt.cluster.Spec.IdentityRef, "")
			g.Expect(err).NotTo(HaveOccurred())
			_, err = provider.GetTokenCredential(t.Context(), "", tt.ActiveDirectoryAuthorityHost, "")
			g.Expect(err).NotTo(HaveOccurred())
		})
	}
}

func TestParseCloudType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected cloud.Configuration
	}{
		{
			name:     "when the input is public, expect AzurePublic",
			input:    "public",
			expected: cloud.AzurePublic,
		},
		{
			name:     "when the input is China, expect AzureChina",
			input:    "china",
			expected: cloud.AzureChina,
		},
		{
			name:     "when the input is usgovernment, expect AzureGovernment",
			input:    "usgovernment",
			expected: cloud.AzureGovernment,
		},
		{
			name:     "when the input is empty, expect AzurePublic",
			input:    "", // Test case for default value
			expected: cloud.AzurePublic,
		},
		{
			name:     "when the input is PUBLIC, expect AzurePublic",
			input:    "PUBLIC", // Test case for uppercased input
			expected: cloud.AzurePublic,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			g.Expect(parseCloudType(tt.input)).To(Equal(tt.expected))
		})
	}
}

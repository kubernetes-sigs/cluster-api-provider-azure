/*
Copyright The Kubernetes Authors.

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

package webhooks

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

const fakeClientID = "fake-client-id"
const fakeTenantID = "fake-tenant-id"
const fakeResourceID = "fake-resource-id"

func TestAzureClusterIdentity_ValidateCreate(t *testing.T) {
	tests := []struct {
		name            string
		clusterIdentity *infrav1.AzureClusterIdentity
		wantErr         bool
	}{
		{
			name: "azureclusteridentity with service principal",
			clusterIdentity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.ServicePrincipal,
					ClientID: fakeClientID,
					TenantID: fakeTenantID,
				},
			},
			wantErr: false,
		},
		{
			name: "azureclusteridentity with service principal and empty client id",
			clusterIdentity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.ServicePrincipal,
					TenantID: fakeTenantID,
				},
			},
			wantErr: false,
		},
		{
			name: "azureclusteridentity with service principal and resource id",
			clusterIdentity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:       infrav1.ServicePrincipal,
					ClientID:   fakeClientID,
					TenantID:   fakeTenantID,
					ResourceID: fakeResourceID,
				},
			},
			wantErr: true,
		},
		{
			name: "azureclusteridentity with user assigned msi and resource id",
			clusterIdentity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:       infrav1.UserAssignedMSI,
					ClientID:   fakeClientID,
					TenantID:   fakeTenantID,
					ResourceID: fakeResourceID,
				},
			},
			wantErr: false,
		},
		{
			name: "azureclusteridentity with user assigned msi and no resource id",
			clusterIdentity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.UserAssignedMSI,
					ClientID: fakeClientID,
					TenantID: fakeTenantID,
				},
			},
			wantErr: false,
		},
		{
			name: "azureclusteridentity with azure cli",
			clusterIdentity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.AzureCLI,
					TenantID: fakeTenantID,
				},
			},
			wantErr: false,
		},
		{
			name: "azureclusteridentity with azure cli and client secret",
			clusterIdentity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.AzureCLI,
					TenantID: fakeTenantID,
					ClientSecret: corev1.SecretReference{
						Name: "azure-cli-secret",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "azureclusteridentity with azure cli and cert path",
			clusterIdentity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.AzureCLI,
					TenantID: fakeTenantID,
					CertPath: "/tmp/cert.pem",
				},
			},
			wantErr: true,
		},
		{
			name: "azureclusteridentity with azure cli and user assigned identity credentials path",
			clusterIdentity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:                                infrav1.AzureCLI,
					TenantID:                            fakeTenantID,
					UserAssignedIdentityCredentialsPath: "/tmp/uai.json",
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			_, err := (&AzureClusterIdentityWebhook{}).ValidateCreate(t.Context(), tc.clusterIdentity)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

// TestAzureClusterIdentity_ClientIDValidation exercises the CRD CEL rule with
// raw objects so an omitted clientID remains distinct from an empty string.
func TestAzureClusterIdentity_ClientIDValidation(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to get test file path")
	}
	repoRoot := filepath.Join(filepath.Dir(testFile), "..", "..")
	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join(
			repoRoot,
			"config",
			"crd",
			"bases",
			"infrastructure.cluster.x-k8s.io_azureclusteridentities.yaml",
		)},
		ErrorIfCRDPathMissing: true,
	}
	config, err := testEnv.Start()
	if err != nil {
		t.Fatalf("failed to start envtest: %v", err)
	}
	t.Cleanup(func() {
		if err := testEnv.Stop(); err != nil {
			t.Errorf("failed to stop envtest: %v", err)
		}
	})

	scheme := k8sruntime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	if err := infrav1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add CAPZ API to scheme: %v", err)
	}
	kubeClient, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("failed to create Kubernetes client: %v", err)
	}

	ctx := t.Context()
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "clientid-validation-test"}}
	if err := kubeClient.Create(ctx, namespace); err != nil {
		t.Fatalf("failed to create test namespace: %v", err)
	}

	emptyClientID := ""
	nonEmptyClientID := fakeClientID
	tests := []struct {
		name     string
		typeName infrav1.IdentityType
		clientID *string
		wantErr  bool
	}{
		{name: "empty ClientID is accepted for existing identity types", typeName: infrav1.ServicePrincipal, clientID: &emptyClientID},
		{name: "non-empty ClientID is accepted", typeName: infrav1.ServicePrincipal, clientID: &nonEmptyClientID},
		{name: "missing ClientID is rejected for existing identity types", typeName: infrav1.ServicePrincipal, wantErr: true},
		{name: "missing ClientID is accepted for AzureCLI", typeName: infrav1.AzureCLI},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := fmt.Sprintf("identity-%d", i)
			var err error
			if tt.clientID != nil {
				// Use the typed API for present values. In particular, the empty
				// value must serialize as a present clientID property.
				identity := &infrav1.AzureClusterIdentity{
					ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace.Name},
					Spec: infrav1.AzureClusterIdentitySpec{
						Type:     tt.typeName,
						ClientID: *tt.clientID,
						TenantID: fakeTenantID,
					},
				}
				err = kubeClient.Create(ctx, identity)
			} else {
				// Use raw objects to distinguish an omitted property from an empty string.
				identity := &unstructured.Unstructured{Object: map[string]any{
					"apiVersion": infrav1.GroupVersion.String(),
					"kind":       infrav1.AzureClusterIdentityKind,
					"metadata": map[string]any{
						"name":      name,
						"namespace": namespace.Name,
					},
					"spec": map[string]any{
						"type":     string(tt.typeName),
						"tenantID": fakeTenantID,
					},
				}}
				err = kubeClient.Create(ctx, identity)
			}
			if tt.wantErr {
				if err == nil || !apierrors.IsInvalid(err) {
					t.Fatalf("expected API server validation error, got: %v", err)
				}
				NewWithT(t).Expect(err.Error()).To(ContainSubstring("clientID must be present when type is not AzureCLI"))
				return
			}
			if err != nil {
				t.Fatalf("failed to create identity: %v", err)
			}
		})
	}
}

func TestAzureClusterIdentity_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name               string
		oldClusterIdentity *infrav1.AzureClusterIdentity
		clusterIdentity    *infrav1.AzureClusterIdentity
		wantErr            bool
	}{
		{
			name: "azureclusteridentity with no change",
			clusterIdentity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.ServicePrincipal,
					ClientID: fakeClientID,
					TenantID: fakeTenantID,
				},
			},
			oldClusterIdentity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.ServicePrincipal,
					ClientID: fakeClientID,
					TenantID: fakeTenantID,
				},
			},
			wantErr: false,
		},
		{
			name: "azureclusteridentity with a change in type",
			clusterIdentity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:       infrav1.ServicePrincipal,
					ClientID:   fakeClientID,
					TenantID:   fakeTenantID,
					ResourceID: fakeResourceID,
				},
			},
			oldClusterIdentity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:       infrav1.WorkloadIdentity,
					ClientID:   fakeClientID,
					TenantID:   fakeTenantID,
					ResourceID: fakeResourceID,
				},
			},
			wantErr: true,
		},
		{
			name: "azureclusteridentity with a change in client ID",
			clusterIdentity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:       infrav1.ServicePrincipal,
					ClientID:   fakeClientID,
					TenantID:   fakeTenantID,
					ResourceID: fakeResourceID,
				},
			},
			oldClusterIdentity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:       infrav1.WorkloadIdentity,
					ClientID:   "diff-fake-Client-ID",
					TenantID:   fakeTenantID,
					ResourceID: fakeResourceID,
				},
			},
			wantErr: true,
		},
		{
			name: "azureclusteridentity azure cli with no change",
			oldClusterIdentity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.AzureCLI,
					TenantID: fakeTenantID,
				},
			},
			clusterIdentity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.AzureCLI,
					TenantID: fakeTenantID,
				},
			},
			wantErr: false,
		},
		{
			name: "azureclusteridentity azure cli adding client secret",
			oldClusterIdentity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.AzureCLI,
					TenantID: fakeTenantID,
				},
			},
			clusterIdentity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.AzureCLI,
					TenantID: fakeTenantID,
					ClientSecret: corev1.SecretReference{
						Name: "azure-cli-secret",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			_, err := (&AzureClusterIdentityWebhook{}).ValidateUpdate(t.Context(), tc.oldClusterIdentity, tc.clusterIdentity)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

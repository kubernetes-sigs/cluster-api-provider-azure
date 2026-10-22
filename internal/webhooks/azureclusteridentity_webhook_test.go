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
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

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
			name: "azureclusteridentity with service principal and no client id",
			clusterIdentity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.ServicePrincipal,
					TenantID: fakeTenantID,
				},
			},
			wantErr: true,
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

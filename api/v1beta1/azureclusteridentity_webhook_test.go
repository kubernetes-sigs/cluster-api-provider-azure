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

package v1beta1

import (
	"testing"

	. "github.com/onsi/gomega"
)

const fakeClientID = "fake-client-id"
const fakeTenantID = "fake-tenant-id"
const fakeResourceID = "fake-resource-id"

func TestAzureClusterIdentity_ValidateCreate(t *testing.T) {
	tests := []struct {
		name            string
		clusterIdentity *AzureClusterIdentity
		wantErr         bool
	}{
		{
			name: "azureclusteridentity with service principal",
			clusterIdentity: &AzureClusterIdentity{
				Spec: AzureClusterIdentitySpec{
					Type:     ServicePrincipal,
					ClientID: fakeClientID,
					TenantID: fakeTenantID,
				},
			},
			wantErr: false,
		},
		{
			name: "azureclusteridentity with service principal and resource id",
			clusterIdentity: &AzureClusterIdentity{
				Spec: AzureClusterIdentitySpec{
					Type:       ServicePrincipal,
					ClientID:   fakeClientID,
					TenantID:   fakeTenantID,
					ResourceID: fakeResourceID,
				},
			},
			wantErr: true,
		},
		{
			name: "azureclusteridentity with user assigned msi and resource id",
			clusterIdentity: &AzureClusterIdentity{
				Spec: AzureClusterIdentitySpec{
					Type:       UserAssignedMSI,
					ClientID:   fakeClientID,
					TenantID:   fakeTenantID,
					ResourceID: fakeResourceID,
				},
			},
			wantErr: false,
		},
		{
			name: "azureclusteridentity with user assigned msi and no resource id",
			clusterIdentity: &AzureClusterIdentity{
				Spec: AzureClusterIdentitySpec{
					Type:     UserAssignedMSI,
					ClientID: fakeClientID,
					TenantID: fakeTenantID,
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			_, err := tc.clusterIdentity.ValidateCreate()
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
		oldClusterIdentity *AzureClusterIdentity
		clusterIdentity    *AzureClusterIdentity
		wantErr            bool
	}{
		{
			name: "azureclusteridentity with no change",
			clusterIdentity: &AzureClusterIdentity{
				Spec: AzureClusterIdentitySpec{
					Type:     ServicePrincipal,
					ClientID: fakeClientID,
					TenantID: fakeTenantID,
				},
			},
			oldClusterIdentity: &AzureClusterIdentity{
				Spec: AzureClusterIdentitySpec{
					Type:     ServicePrincipal,
					ClientID: fakeClientID,
					TenantID: fakeTenantID,
				},
			},
			wantErr: false,
		},
		{
			name: "azureclusteridentity with a change in type",
			clusterIdentity: &AzureClusterIdentity{
				Spec: AzureClusterIdentitySpec{
					Type:       ServicePrincipal,
					ClientID:   fakeClientID,
					TenantID:   fakeTenantID,
					ResourceID: fakeResourceID,
				},
			},
			oldClusterIdentity: &AzureClusterIdentity{
				Spec: AzureClusterIdentitySpec{
					Type:       WorkloadIdentity,
					ClientID:   fakeClientID,
					TenantID:   fakeTenantID,
					ResourceID: fakeResourceID,
				},
			},
			wantErr: true,
		},
		{
			name: "azureclusteridentity with a change in client ID",
			clusterIdentity: &AzureClusterIdentity{
				Spec: AzureClusterIdentitySpec{
					Type:       ServicePrincipal,
					ClientID:   fakeClientID,
					TenantID:   fakeTenantID,
					ResourceID: fakeResourceID,
				},
			},
			oldClusterIdentity: &AzureClusterIdentity{
				Spec: AzureClusterIdentitySpec{
					Type:       WorkloadIdentity,
					ClientID:   "diff-fake-Client-ID",
					TenantID:   fakeTenantID,
					ResourceID: fakeResourceID,
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			_, err := tc.clusterIdentity.ValidateUpdate(tc.oldClusterIdentity)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

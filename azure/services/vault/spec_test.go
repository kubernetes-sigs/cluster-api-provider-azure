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

package vault

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/mgmt/2019-09-01/keyvault"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/gofrs/uuid"
	. "github.com/onsi/gomega"
)

const (
	tenantUUIDStr = "d95e9be6-a642-11ec-b909-0242ac120002"
)

var (
	vaultSpec = Spec{
		Name:          "my-vault",
		ResourceGroup: "my-rg",
		Location:      "eastus",
		ClusterName:   "my-cluster",
		TenantID:      tenantUUIDStr,
	}
	tenantUUID = uuid.FromStringOrNil(tenantUUIDStr)
)

func TestVaultSpec_ResourceName(t *testing.T) {
	g := NewWithT(t)
	g.Expect(vaultSpec.ResourceName()).Should(Equal("my-vault"))
}

func TestVaultSpec_ResourceGroupName(t *testing.T) {
	g := NewWithT(t)
	g.Expect(vaultSpec.ResourceGroupName()).Should(Equal("my-rg"))
}

func TestVaultSpec_OwnerResourceName(t *testing.T) {
	g := NewWithT(t)
	g.Expect(vaultSpec.OwnerResourceName()).Should(Equal(""))
}

func TestVaultSpec_Parameters(t *testing.T) {
	testcases := []struct {
		name          string
		spec          Spec
		existing      interface{}
		expect        func(g *WithT, result interface{})
		expectedError string
	}{
		{
			name:          "new vault spec",
			expectedError: "",
			spec:          vaultSpec,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(Equal(keyvault.VaultCreateOrUpdateParameters{
					Location: to.StringPtr("eastus"),
					Properties: &keyvault.VaultProperties{
						TenantID: &tenantUUID,
						Sku: &keyvault.Sku{
							Family: to.StringPtr("A"),
							Name:   "standard",
						},
						EnableRbacAuthorization: to.BoolPtr(true),
						EnableSoftDelete:        to.BoolPtr(false),
					},
					Tags: map[string]*string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
					},
				}))
			},
		},
		{
			name:          "existing vault",
			expectedError: "",
			spec:          vaultSpec,
			existing:      keyvault.Vault{},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
		},
		{
			name:          "type cast error",
			expectedError: "string is not a keyvault.Vault",
			spec:          vaultSpec,
			existing:      "I'm not keyvault.Vault",
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(tc.existing)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				tc.expect(g, result)
			}
		})
	}
}

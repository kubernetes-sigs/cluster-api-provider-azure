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

package vaults

import (
	"context"
	"testing"

	"github.com/Azure/azure-service-operator/v2/api/keyvault/v1api20230701"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

func TestList(t *testing.T) {
	g := NewWithT(t)

	ctx := t.Context()
	mockClient := &MockClient{}

	// Test successful list
	expectedVaults := []v1api20230701.Vault{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vault1",
				Namespace: "default",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "vault2",
				Namespace: "default",
			},
		},
	}

	mockClient.vaultList = &v1api20230701.VaultList{
		Items: expectedVaults,
	}

	result, err := list(ctx, mockClient)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(HaveLen(2))
	g.Expect(*result[0]).To(Equal(expectedVaults[0]))
	g.Expect(*result[1]).To(Equal(expectedVaults[1]))
}

func TestServiceName(t *testing.T) {
	g := NewWithT(t)
	g.Expect(serviceName).To(Equal("vaults"))
}

func TestVaultReadyCondition(t *testing.T) {
	g := NewWithT(t)
	g.Expect(string(VaultReadyCondition)).To(Equal("VaultReady"))
}

// MockClient implements client.Client for testing
type MockClient struct {
	client.Client
	vaultList *v1api20230701.VaultList
	listError error
}

func (m *MockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if m.listError != nil {
		return m.listError
	}

	if vaultList, ok := list.(*v1api20230701.VaultList); ok {
		if m.vaultList != nil {
			*vaultList = *m.vaultList
		}
		return nil
	}

	return nil
}

// MockKeyVaultScope implements KeyVaultScope for testing
type MockKeyVaultScope struct {
	specs []azure.ASOResourceSpecGetter[*v1api20230701.Vault]
}

func (m *MockKeyVaultScope) VaultSpecs() []azure.ASOResourceSpecGetter[*v1api20230701.Vault] {
	if m.specs == nil {
		return []azure.ASOResourceSpecGetter[*v1api20230701.Vault]{
			&VaultSpec{
				Name:          "test-vault",
				ResourceGroup: "test-rg",
				Location:      "eastus",
				TenantID:      "test-tenant-id",
				Tags: map[string]string{
					"environment": "test",
				},
			},
		}
	}
	return m.specs
}

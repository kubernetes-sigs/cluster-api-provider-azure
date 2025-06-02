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

package userassignedidentities

import (
	"context"
	"testing"

	"github.com/Azure/azure-service-operator/v2/api/managedidentity/v1api20230131"
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
	expectedIdentities := []v1api20230131.UserAssignedIdentity{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "identity1",
				Namespace: "default",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "identity2",
				Namespace: "default",
			},
		},
	}

	mockClient.identityList = &v1api20230131.UserAssignedIdentityList{
		Items: expectedIdentities,
	}

	result, err := list(ctx, mockClient)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(HaveLen(2))
	g.Expect(*result[0]).To(Equal(expectedIdentities[0]))
	g.Expect(*result[1]).To(Equal(expectedIdentities[1]))
}

func TestServiceName(t *testing.T) {
	g := NewWithT(t)
	g.Expect(serviceName).To(Equal("userassignedidentities"))
}

func TestUserIdentitiesReadyCondition(t *testing.T) {
	g := NewWithT(t)
	g.Expect(string(UserIdentitiesReadyCondition)).To(Equal("UserIdentitiesReady"))
}

// MockClient implements client.Client for testing
type MockClient struct {
	client.Client
	identityList *v1api20230131.UserAssignedIdentityList
	listError    error
}

func (m *MockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if m.listError != nil {
		return m.listError
	}

	if identityList, ok := list.(*v1api20230131.UserAssignedIdentityList); ok {
		if m.identityList != nil {
			*identityList = *m.identityList
		}
		return nil
	}

	return nil
}

// MockUserAssignedIdentityScope implements UserAssignedIdentityScope for testing
type MockUserAssignedIdentityScope struct {
	specs []azure.ASOResourceSpecGetter[*v1api20230131.UserAssignedIdentity]
}

func (m *MockUserAssignedIdentityScope) UserAssignedIdentitySpecs() []azure.ASOResourceSpecGetter[*v1api20230131.UserAssignedIdentity] {
	if m.specs == nil {
		return []azure.ASOResourceSpecGetter[*v1api20230131.UserAssignedIdentity]{
			&UserAssignedIdentitySpec{
				Name:          "test-identity",
				ResourceGroup: "test-rg",
				Location:      "eastus",
				Tags: map[string]*string{
					"environment": stringPtr("test"),
				},
				ConfigMapName: "test-identity-config",
			},
		}
	}
	return m.specs
}

func stringPtr(s string) *string {
	return &s
}

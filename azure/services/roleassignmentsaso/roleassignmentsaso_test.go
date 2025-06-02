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

package roleassignmentsaso

import (
	"context"
	"testing"

	"github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestList(t *testing.T) {
	g := NewWithT(t)

	ctx := t.Context()
	mockClient := &MockClient{}

	// Test successful list
	expectedRoleAssignments := []v1api20220401.RoleAssignment{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "role1",
				Namespace: "default",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "role2",
				Namespace: "default",
			},
		},
	}

	mockClient.roleAssignmentList = &v1api20220401.RoleAssignmentList{
		Items: expectedRoleAssignments,
	}

	result, err := list(ctx, mockClient)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(HaveLen(2))
	g.Expect(*result[0]).To(Equal(expectedRoleAssignments[0]))
	g.Expect(*result[1]).To(Equal(expectedRoleAssignments[1]))
}

func TestServiceName(t *testing.T) {
	g := NewWithT(t)
	g.Expect(serviceName).To(Equal("roleassignmentsaso"))
}

func TestServiceConstants(t *testing.T) {
	g := NewWithT(t)
	g.Expect(serviceName).To(Equal("roleassignmentsaso"))
}

// MockClient implements client.Client for testing
type MockClient struct {
	client.Client
	roleAssignmentList *v1api20220401.RoleAssignmentList
	listError          error
}

func (m *MockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if m.listError != nil {
		return m.listError
	}

	if roleAssignmentList, ok := list.(*v1api20220401.RoleAssignmentList); ok {
		if m.roleAssignmentList != nil {
			*roleAssignmentList = *m.roleAssignmentList
		}
		return nil
	}

	return nil
}

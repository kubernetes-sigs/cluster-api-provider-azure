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
	"testing"

	"github.com/Azure/azure-service-operator/v2/api/authorization/v1api20220401"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

func TestKubernetesRoleAssignmentSpec_ResourceRef(t *testing.T) {
	g := NewWithT(t)

	spec := &KubernetesRoleAssignmentSpec{
		Name:                     "test-role-assignment",
		Namespace:                "default",
		PrincipalIDConfigMapName: "test-principal-config",
		PrincipalIDConfigMapKey:  "principalId",
		PrincipalType:            "User",
		RoleDefinitionReference:  "test-role-definition-id",
		OwnerName:                "test-owner",
		OwnerGroup:               "test-group",
		OwnerKind:                "TestKind",
		ClusterName:              "test-cluster",
	}

	ref := spec.ResourceRef()
	g.Expect(ref).NotTo(BeNil())
	g.Expect(ref.Name).To(Equal(azure.GetNormalizedKubernetesName("test-role-assignment")))
}

func TestKubernetesRoleAssignmentSpec_Parameters(t *testing.T) {
	testCases := []struct {
		name     string
		spec     *KubernetesRoleAssignmentSpec
		existing *v1api20220401.RoleAssignment
	}{
		{
			name: "new role assignment",
			spec: &KubernetesRoleAssignmentSpec{
				Name:                     "test-role-assignment",
				Namespace:                "default",
				PrincipalIDConfigMapName: "test-principal-config",
				PrincipalIDConfigMapKey:  "principalId",
				PrincipalType:            "User",
				RoleDefinitionReference:  "test-role-definition-id",
				OwnerName:                "test-owner",
				OwnerGroup:               "test-group",
				OwnerKind:                "TestKind",
				ClusterName:              "test-cluster",
			},
			existing: nil,
		},
		{
			name: "existing role assignment",
			spec: &KubernetesRoleAssignmentSpec{
				Name:                     "existing-role-assignment",
				Namespace:                "default",
				PrincipalIDConfigMapName: "updated-principal-config",
				PrincipalIDConfigMapKey:  "principalId",
				PrincipalType:            "ServicePrincipal",
				RoleDefinitionReference:  "updated-role-definition-id",
				OwnerName:                "updated-owner",
				OwnerGroup:               "updated-group",
				OwnerKind:                "UpdatedKind",
				ClusterName:              "updated-cluster",
			},
			existing: &v1api20220401.RoleAssignment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "existing-role-assignment",
				},
				Spec: v1api20220401.RoleAssignment_Spec{
					PrincipalId: ptr.To("old-principal-id"),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			result, err := tc.spec.Parameters(t.Context(), tc.existing)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result).NotTo(BeNil())

			// Verify basic properties exist
			g.Expect(result.Spec).NotTo(BeNil())
		})
	}
}

func TestKubernetesRoleAssignmentSpec_WasManaged(t *testing.T) {
	g := NewWithT(t)

	spec := &KubernetesRoleAssignmentSpec{
		Name:                     "test-role-assignment",
		Namespace:                "default",
		PrincipalIDConfigMapName: "test-principal-config",
		PrincipalIDConfigMapKey:  "principalId",
		PrincipalType:            "User",
		RoleDefinitionReference:  "test-role-definition-id",
		OwnerName:                "test-owner",
		OwnerGroup:               "test-group",
		OwnerKind:                "TestKind",
		ClusterName:              "test-cluster",
	}

	// Should always return true for role assignments
	g.Expect(spec.WasManaged(nil)).To(BeTrue())
	g.Expect(spec.WasManaged(&v1api20220401.RoleAssignment{})).To(BeTrue())
}

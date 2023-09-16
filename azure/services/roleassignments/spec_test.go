/*
Copyright 2023 The Kubernetes Authors.

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

package roleassignments

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

var (
	fakeRoleAssignment = armauthorization.RoleAssignment{
		ID:   ptr.To("fake-id"),
		Name: ptr.To("fake-name"),
		Type: ptr.To("fake-type"),
	}
	fakeRoleAssignmentSpec = RoleAssignmentSpec{
		PrincipalID:      ptr.To("fake-principal-id"),
		RoleDefinitionID: "fake-role-definition-id",
	}
)

func TestRoleAssignmentSpec_Parameters(t *testing.T) {
	testCases := []struct {
		name          string
		spec          *RoleAssignmentSpec
		existing      interface{}
		expect        func(g *WithT, result interface{})
		expectedError string
	}{
		{
			name:     "error when existing is not of RoleAssignment type",
			spec:     &RoleAssignmentSpec{},
			existing: struct{}{},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "struct {} is not an armauthorization.RoleAssignment",
		},
		{
			name:     "get result as nil when existing NatGateway is present",
			spec:     &fakeRoleAssignmentSpec,
			existing: fakeRoleAssignment,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
		{
			name:     "get result as nil when existing NatGateway is present with empty data",
			spec:     &fakeRoleAssignmentSpec,
			existing: armauthorization.RoleAssignment{},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
		{
			name:     "get RoleAssignment when all values are present",
			spec:     &fakeRoleAssignmentSpec,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armauthorization.RoleAssignmentCreateParameters{}))
				g.Expect(result.(armauthorization.RoleAssignmentCreateParameters).Properties.RoleDefinitionID).To(Equal(ptr.To[string](fakeRoleAssignmentSpec.RoleDefinitionID)))
				g.Expect(result.(armauthorization.RoleAssignmentCreateParameters).Properties.PrincipalID).To(Equal(fakeRoleAssignmentSpec.PrincipalID))
			},
			expectedError: "",
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(context.TODO(), tc.existing)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			tc.expect(g, result)
		})
	}
}

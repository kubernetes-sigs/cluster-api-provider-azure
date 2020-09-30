/*
Copyright 2019 The Kubernetes Authors.

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
	"fmt"
	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/authorization/mgmt/authorization"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
)

const azureBuiltInContributorID = "b24988ac-6180-42a0-ab88-20f7382dd24c"

// Reconcile creates a role assignment.
func (s *Service) Reconcile(ctx context.Context) error {
	for _, roleSpec := range s.Scope.RoleAssignmentSpecs() {
		resultVM, err := s.VirtualMachinesClient.Get(ctx, s.Scope.ResourceGroup(), roleSpec.MachineName)
		if err != nil {
			return errors.Wrapf(err, "cannot get VM to assign role to system assigned identity")
		}

		scope := fmt.Sprintf("/subscriptions/%s/", s.Scope.SubscriptionID())
		// Azure built-in roles https://docs.microsoft.com/en-us/azure/role-based-access-control/built-in-roles
		contributorRoleDefinitionID := fmt.Sprintf("/subscriptions/%s/providers/Microsoft.Authorization/roleDefinitions/%s", s.Scope.SubscriptionID(), azureBuiltInContributorID)
		params := authorization.RoleAssignmentCreateParameters{
			Properties: &authorization.RoleAssignmentProperties{
				RoleDefinitionID: to.StringPtr(contributorRoleDefinitionID),
				PrincipalID:      resultVM.Identity.PrincipalID,
			},
		}
		_, err = s.Client.Create(ctx, scope, roleSpec.Name, params)
		if err != nil {
			return errors.Wrapf(err, "cannot assign role to VM system assigned identity")
		}

		s.Scope.V(2).Info("successfully created role assignment for generated Identity for VM", "virtual machine", roleSpec.MachineName)
	}
	return nil
}

// Delete is a no-op as the role assignments get deleted as part of VM deletion.
func (s *Service) Delete(ctx context.Context) error {
	return nil
}

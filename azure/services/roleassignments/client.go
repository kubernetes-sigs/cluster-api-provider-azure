/*
Copyright 2020 The Kubernetes Authors.

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

	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/authorization/mgmt/authorization"
	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// azureClient contains the Azure go-sdk Client.
type azureClient struct {
	roleassignments authorization.RoleAssignmentsClient
}

// newClient creates a new role assignment client from subscription ID.
func newClient(auth azure.Authorizer) *azureClient {
	c := newRoleAssignmentClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &azureClient{c}
}

// newRoleAssignmentClient creates a role assignments client from subscription ID.
func newRoleAssignmentClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) authorization.RoleAssignmentsClient {
	roleClient := authorization.NewRoleAssignmentsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&roleClient.Client, authorizer)
	return roleClient
}

// Get gets the specified role assignment by the role assignment name.
func (ac *azureClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (interface{}, error) {
	ctx, span := tele.Tracer().Start(ctx, "roleassignments.AzureClient.Get")
	defer span.End()
	return ac.roleassignments.Get(ctx, spec.OwnerResourceName(), spec.ResourceName())
}

// CreateOrUpdateAsync creates a roleassignment.
// Creating a roleassignment is not a long running operation, so we don't ever return a future.
func (ac *azureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, parameters interface{}) (interface{}, azureautorest.FutureAPI, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "groups.AzureClient.CreateOrUpdate")
	defer done()
	createParams, ok := parameters.(authorization.RoleAssignmentCreateParameters)
	if !ok {
		return nil, nil, errors.Errorf("%T is not a authorization.RoleAssignmentCreateParameters", parameters)
	}
	result, err := ac.roleassignments.Create(ctx, spec.OwnerResourceName(), spec.ResourceName(), createParams)
	return result, nil, err
}

// IsDone returns true if the long-running operation has completed.
func (ac *azureClient) IsDone(ctx context.Context, future azureautorest.FutureAPI) (bool, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "roleassignments.AzureClient.IsDone")
	defer done()

	isDone, err := future.DoneWithContext(ctx, ac.roleassignments)
	if err != nil {
		return false, errors.Wrap(err, "failed checking if the operation was complete")
	}
	return isDone, nil
}

// Result fetches the result of a long-running operation future.
func (ac *azureClient) Result(ctx context.Context, futureData azureautorest.FutureAPI, futureType string) (interface{}, error) {
	// Result is a no-op for role assignment as only Delete operations return a future.
	return nil, nil
}

// DeleteAsync is no-op for role assignments. It gets deleted as part of the VM deletion.
func (ac *azureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (azureautorest.FutureAPI, error) {
	return nil, nil
}

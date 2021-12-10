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

package securitygroups

import (
	"context"
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// client wraps go-sdk.
type client interface {
	Get(context.Context, azure.ResourceSpecGetter) (interface{}, error)
	CreateOrUpdateAsync(context.Context, azure.ResourceSpecGetter, interface{}) (interface{}, azureautorest.FutureAPI, error)
	DeleteAsync(context.Context, azure.ResourceSpecGetter) (azureautorest.FutureAPI, error)
	IsDone(context.Context, azureautorest.FutureAPI) (bool, error)
	Result(context.Context, azureautorest.FutureAPI, string) (interface{}, error)
}

// azureClient contains the Azure go-sdk Client.
type azureClient struct {
	securitygroups network.SecurityGroupsClient
}

var _ client = (*azureClient)(nil)

// newClient creates a new VM client from subscription ID.
func newClient(auth azure.Authorizer) *azureClient {
	c := newSecurityGroupsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &azureClient{c}
}

// newSecurityGroupsClient creates a new security groups client from subscription ID.
func newSecurityGroupsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) network.SecurityGroupsClient {
	securityGroupsClient := network.NewSecurityGroupsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&securityGroupsClient.Client, authorizer)
	return securityGroupsClient
}

// Get gets the specified network security group.
func (ac *azureClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (interface{}, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "securitygroups.AzureClient.Get")
	defer done()

	return ac.securitygroups.Get(ctx, spec.ResourceGroupName(), spec.ResourceName(), "")
}

// CreateOrUpdateAsync creates or updates a network security group in the specified resource group.
// It sends a PUT request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *azureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, parameters interface{}) (interface{}, azureautorest.FutureAPI, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "securitygroups.AzureClient.CreateOrUpdate")
	defer done()

	sg, ok := parameters.(network.SecurityGroup)
	if !ok {
		return nil, nil, errors.Errorf("%T is not a network.SecurityGroup", parameters)
	}

	var etag string
	if sg.Etag != nil {
		etag = *sg.Etag
	}
	req, err := ac.securitygroups.CreateOrUpdatePreparer(ctx, spec.ResourceGroupName(), spec.ResourceName(), sg)
	if err != nil {
		err = autorest.NewErrorWithError(err, "network.SecurityGroupsClient", "CreateOrUpdate", nil, "Failure preparing request")
		return nil, nil, err
	}
	if etag != "" {
		req.Header.Add("If-Match", etag)
	}

	future, err := ac.securitygroups.CreateOrUpdateSender(req)
	if err != nil {
		err = autorest.NewErrorWithError(err, "network.SecurityGroupsClient", "CreateOrUpdate", future.Response(), "Failure sending request")
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = future.WaitForCompletionRef(ctx, ac.securitygroups.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return nil, &future, err
	}
	result, err := future.Result(ac.securitygroups)
	// if the operation completed, return a nil future.
	return result, nil, err
}

// Delete deletes the specified network security group. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *azureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (azureautorest.FutureAPI, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "securitygroups.AzureClient.Delete")
	defer done()

	future, err := ac.securitygroups.Delete(ctx, spec.ResourceGroupName(), spec.ResourceName())
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = future.WaitForCompletionRef(ctx, ac.securitygroups.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return &future, err
	}
	_, err = future.Result(ac.securitygroups)
	// if the operation completed, return a nil future.
	return nil, err
}

// IsDone returns true if the long-running operation has completed.
func (ac *azureClient) IsDone(ctx context.Context, future azureautorest.FutureAPI) (bool, error) {
	ctx, span := tele.Tracer().Start(ctx, "securitygroups.AzureClient.IsDone")
	defer span.End()

	done, err := future.DoneWithContext(ctx, ac.securitygroups)
	if err != nil {
		return false, errors.Wrap(err, "failed checking if the operation was complete")
	}

	return done, nil
}

// Result fetches the result of a long-running operation future.
func (ac *azureClient) Result(ctx context.Context, futureData azureautorest.FutureAPI, futureType string) (interface{}, error) {
	if futureData == nil {
		return nil, errors.Errorf("cannot get result from nil future")
	}
	var result func(client network.SecurityGroupsClient) (sg network.SecurityGroup, err error)

	switch futureType {
	case infrav1.PutFuture:
		var future *network.SecurityGroupsCreateOrUpdateFuture
		jsonData, err := futureData.MarshalJSON()
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal future")
		}
		if err := json.Unmarshal(jsonData, &future); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal future data")
		}
		result = (*future).Result

	case infrav1.DeleteFuture:
		// Delete does not return a result vnet.
		return nil, nil

	default:
		return nil, errors.Errorf("unknown future type %q", futureType)
	}

	return result(ac.securitygroups)
}

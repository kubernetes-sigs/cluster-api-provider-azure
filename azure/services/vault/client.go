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
	"context"
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/services/keyvault/mgmt/2019-09-01/keyvault"
	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// azureClient contains the Azure go-sdk Client.
type azureClient struct {
	vaultsClient keyvault.VaultsClient
}

// newClient creates a new azure go-sdk client for vault.
func newClient(auth azure.Authorizer) *azureClient {
	vaultsClient := newVaultsClient(auth.BaseURI(), auth.SubscriptionID(), auth.Authorizer())
	return &azureClient{vaultsClient: vaultsClient}
}

// newVaultsClient creates a new key vault management client.
func newVaultsClient(baseURI, subscriptionID string, authorizer autorest.Authorizer) keyvault.VaultsClient {
	vaultsClient := keyvault.NewVaultsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&vaultsClient.Client, authorizer)
	return vaultsClient
}

// CreateOrUpdateAsync creates a azure keyvault.
// It sends a PUT request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
// NOTE: vault creation is a synchronous operation.
func (ac *azureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, parameters interface{}) (result interface{}, future azureautorest.FutureAPI, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "keyvault.AzureClient.CreateOrUpdateAsync")
	defer done()

	vaultParameters, ok := parameters.(keyvault.VaultCreateOrUpdateParameters)
	if !ok {
		return nil, nil, errors.Errorf("%T is not a keyvault.VaultCreateOrUpdateParameters", parameters)
	}

	createFuture, err := ac.vaultsClient.CreateOrUpdate(ctx, spec.ResourceGroupName(), spec.ResourceName(), vaultParameters)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = createFuture.WaitForCompletionRef(ctx, ac.vaultsClient.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return nil, &createFuture, err
	}

	result, err = createFuture.Result(ac.vaultsClient)
	// if the operation completed, return a nil future
	return result, nil, err
}

// IsDone returns true if the long-running operation has completed.
func (ac *azureClient) IsDone(ctx context.Context, future azureautorest.FutureAPI) (isDone bool, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "keyvault.AzureClient.IsDone")
	defer done()

	isDone, err = future.DoneWithContext(ctx, ac.vaultsClient)
	if err != nil {
		return false, errors.Wrap(err, "failed checking if the operation was complete")
	}

	return isDone, nil
}

// Result fetches the result of a long-running operation future.
func (ac *azureClient) Result(ctx context.Context, future azureautorest.FutureAPI, futureType string) (result interface{}, err error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "keyvault.AzureClient.Result")
	defer done()

	if future == nil {
		return nil, errors.Errorf("cannot get result from nil future")
	}

	switch futureType {
	case infrav1.PutFuture:
		// Marshal and Unmarshal the future to put it into the correct future type so we can access the Result function.
		// Unfortunately the FutureAPI can't be casted directly to InterfacesCreateOrUpdateFuture because it is a azureautorest.Future, which doesn't implement the Result function. See PR #1686 for discussion on alternatives.
		// It was converted back to a generic azureautorest.Future from the CAPZ infrav1.Future type stored in Status: https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/azure/converters/futures.go#L49.
		var createFuture *keyvault.VaultsCreateOrUpdateFuture
		jsonData, err := future.MarshalJSON()
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal future")
		}
		if err := json.Unmarshal(jsonData, &createFuture); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal future data")
		}
		return createFuture.Result(ac.vaultsClient)

	case infrav1.DeleteFuture:
		// Delete does not return a result network interface
		return nil, nil

	default:
		return nil, errors.Errorf("unknown future type %q", futureType)
	}
}

// Get gets the specified secret from azure keyvault.
func (ac *azureClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (result interface{}, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "keyvault.AzureClient.Get")
	defer done()

	return ac.vaultsClient.Get(ctx, spec.ResourceGroupName(), spec.ResourceName())
}

// DeleteAsync deletes a azure key vault asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
//
// NOTE: vault deletion is a synchronous operation.
func (ac *azureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (future azureautorest.FutureAPI, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "keyvault.AzureClient.DeleteAsync")
	defer done()

	_, err = ac.vaultsClient.Delete(ctx, spec.ResourceGroupName(), spec.ResourceName())
	if err != nil {
		if azure.ResourceGroupNotFound(err) {
			return nil, nil
		}

		return nil, err
	}
	return nil, nil
}

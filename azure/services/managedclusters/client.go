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

package managedclusters

import (
	"context"
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-05-01/containerservice"
	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// CredentialGetter is a helper interface for getting managed cluster credentials.
type CredentialGetter interface {
	GetCredentials(context.Context, string, string) ([]byte, error)
}

// azureClient contains the Azure go-sdk Client.
type azureClient struct {
	managedclusters containerservice.ManagedClustersClient
}

// newClient creates a new managed cluster client from an authorizer.
func newClient(auth azure.Authorizer) *azureClient {
	return &azureClient{
		managedclusters: newManagedClustersClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer()),
	}
}

// newManagedClustersClient creates a new managed clusters client from subscription ID.
func newManagedClustersClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) containerservice.ManagedClustersClient {
	managedClustersClient := containerservice.NewManagedClustersClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&managedClustersClient.Client, authorizer)
	return managedClustersClient
}

// Get gets a managed cluster.
func (ac *azureClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (result interface{}, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "managedclusters.azureClient.Get")
	defer done()

	return ac.managedclusters.Get(ctx, spec.ResourceGroupName(), spec.ResourceName())
}

// GetCredentials fetches the admin kubeconfig for a managed cluster.
func (ac *azureClient) GetCredentials(ctx context.Context, resourceGroupName, name string) ([]byte, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "managedclusters.azureClient.GetCredentials")
	defer done()

	credentialList, err := ac.managedclusters.ListClusterAdminCredentials(ctx, resourceGroupName, name, "")
	if err != nil {
		return nil, err
	}

	if credentialList.Kubeconfigs == nil || len(*credentialList.Kubeconfigs) < 1 {
		return nil, errors.New("no kubeconfigs available for the managed cluster cluster")
	}

	return *(*credentialList.Kubeconfigs)[0].Value, nil
}

// CreateOrUpdateAsync creates or updates a managed cluster.
// It sends a PUT request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *azureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, parameters interface{}) (result interface{}, future azureautorest.FutureAPI, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "managedclusters.azureClient.CreateOrUpdate")
	defer done()

	managedcluster, ok := parameters.(containerservice.ManagedCluster)
	if !ok {
		return nil, nil, errors.Errorf("%T is not a containerservice.ManagedCluster", parameters)
	}

	preparer, err := ac.managedclusters.CreateOrUpdatePreparer(ctx, spec.ResourceGroupName(), spec.ResourceName(), managedcluster)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to prepare operation")
	}

	headerSpec, ok := spec.(azure.ResourceSpecGetterWithHeaders)
	if !ok {
		return nil, nil, errors.Errorf("%T is not a azure.ResourceSpecGetterWithHeaders", spec)
	}

	for key, value := range headerSpec.CustomHeaders() {
		preparer.Header.Add(key, value)
	}

	createFuture, err := ac.managedclusters.CreateOrUpdateSender(preparer)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = createFuture.WaitForCompletionRef(ctx, ac.managedclusters.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return nil, &createFuture, err
	}

	result, err = createFuture.Result(ac.managedclusters)
	// if the operation completed, return a nil future
	return result, nil, err
}

// DeleteAsync deletes a managed cluster asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Future which can be used to track the ongoing
// progress of the operation.
func (ac *azureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (future azureautorest.FutureAPI, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "managedclusters.azureClient.DeleteAsync")
	defer done()

	deleteFuture, err := ac.managedclusters.Delete(ctx, spec.ResourceGroupName(), spec.ResourceName())
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = deleteFuture.WaitForCompletionRef(ctx, ac.managedclusters.Client)
	if err != nil {
		// if an error occurs, return the future.
		// this means the long-running operation didn't finish in the specified timeout.
		return &deleteFuture, err
	}
	_, err = deleteFuture.Result(ac.managedclusters)
	// if the operation completed, return a nil future.
	return nil, err
}

// IsDone returns true if the long-running operation has completed.
func (ac *azureClient) IsDone(ctx context.Context, future azureautorest.FutureAPI) (bool, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "managedclusters.azureClient.IsDone")
	defer done()

	isDone, err := future.DoneWithContext(ctx, ac.managedclusters)
	if err != nil {
		return false, errors.Wrap(err, "failed checking if the operation was complete")
	}

	return isDone, nil
}

// Result fetches the result of a long-running operation future.
func (ac *azureClient) Result(ctx context.Context, future azureautorest.FutureAPI, futureType string) (result interface{}, err error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "managedclusters.azureClient.Result")
	defer done()

	if future == nil {
		return nil, errors.Errorf("cannot get result from nil future")
	}

	switch futureType {
	case infrav1.PutFuture:
		// Marshal and Unmarshal the future to put it into the correct future type so we can access the Result function.
		// Unfortunately the FutureAPI can't be casted directly to ManagedClustersCreateOrUpdateFuture because it is a azureautorest.Future, which doesn't implement the Result function. See PR #1686 for discussion on alternatives.
		// It was converted back to a generic azureautorest.Future from the CAPZ infrav1.Future type stored in Status: https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/azure/converters/futures.go#L49.
		var createFuture *containerservice.ManagedClustersCreateOrUpdateFuture
		jsonData, err := future.MarshalJSON()
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal future")
		}
		if err := json.Unmarshal(jsonData, &createFuture); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal future data")
		}
		return createFuture.Result(ac.managedclusters)

	case infrav1.DeleteFuture:
		// Delete does not return a result managed cluster.
		return nil, nil

	default:
		return nil, errors.Errorf("unknown future type %q", futureType)
	}
}

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

	"github.com/Azure/azure-sdk-for-go/services/preview/containerservice/mgmt/2022-03-02-preview/containerservice"
	"github.com/Azure/go-autorest/autorest"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Client wraps go-sdk.
type Client interface {
	Get(context.Context, string, string) (containerservice.ManagedCluster, error)
	GetCredentials(context.Context, string, string) ([]byte, error)
	CreateOrUpdate(context.Context, string, string, containerservice.ManagedCluster, map[string]string) (containerservice.ManagedCluster, error)
	Delete(context.Context, string, string) error
}

// AzureClient contains the Azure go-sdk Client.
type AzureClient struct {
	managedclusters containerservice.ManagedClustersClient
}

var _ Client = &AzureClient{}

// NewClient creates a new VM client from subscription ID.
func NewClient(auth azure.Authorizer) *AzureClient {
	return &AzureClient{
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
func (ac *AzureClient) Get(ctx context.Context, resourceGroupName, name string) (containerservice.ManagedCluster, error) {
	return ac.managedclusters.Get(ctx, resourceGroupName, name)
}

// GetCredentials fetches the admin kubeconfig for a managed cluster.
func (ac *AzureClient) GetCredentials(ctx context.Context, resourceGroupName, name string) ([]byte, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "managedclusters.AzureClient.GetCredentials")
	defer done()

	credentialList, err := ac.managedclusters.ListClusterUserCredentials(ctx, resourceGroupName, name, "", containerservice.FormatExec)
	if err != nil {
		return nil, err
	}

	if credentialList.Kubeconfigs == nil || len(*credentialList.Kubeconfigs) < 1 {
		return nil, errors.New("no kubeconfigs available for the managed cluster cluster")
	}

	return *(*credentialList.Kubeconfigs)[0].Value, nil
}

// CreateOrUpdate creates or updates a managed cluster.
func (ac *AzureClient) CreateOrUpdate(ctx context.Context, resourceGroupName, name string, cluster containerservice.ManagedCluster, headers map[string]string) (containerservice.ManagedCluster, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "managedclusters.AzureClient.CreateOrUpdate")
	defer done()

	preparer, err := ac.managedclusters.CreateOrUpdatePreparer(ctx, resourceGroupName, name, cluster)
	if err != nil {
		return containerservice.ManagedCluster{}, errors.Wrap(err, "failed to prepare operation")
	}
	for key, value := range headers {
		preparer.Header.Add(key, value)
	}

	future, err := ac.managedclusters.CreateOrUpdateSender(preparer)
	if err != nil {
		return containerservice.ManagedCluster{}, errors.Wrap(err, "failed to begin operation")
	}
	if err := future.WaitForCompletionRef(ctx, ac.managedclusters.Client); err != nil {
		return containerservice.ManagedCluster{}, errors.Wrap(err, "failed to end operation")
	}
	managedCluster, err := future.Result(ac.managedclusters)
	return managedCluster, err
}

// Delete deletes a managed cluster.
func (ac *AzureClient) Delete(ctx context.Context, resourceGroupName, name string) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "managedclusters.AzureClient.Delete")
	defer done()

	ignorePodDisruptionBudget := false
	future, err := ac.managedclusters.Delete(ctx, resourceGroupName, name, &ignorePodDisruptionBudget)
	if err != nil {
		if azure.ResourceGroupNotFound(err) || azure.ResourceNotFound(err) {
			return nil
		}
		return errors.Wrap(err, "failed to begin operation")
	}
	if err := future.WaitForCompletionRef(ctx, ac.managedclusters.Client); err != nil {
		return errors.Wrap(err, "failed to end operation")
	}
	_, err = future.Result(ac.managedclusters)
	return err
}

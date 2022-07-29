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

package managedmachinepools

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-05-01/containerservice"
	"github.com/Azure/go-autorest/autorest"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Client wraps go-sdk.
type Client interface {
	Get(context.Context, string, string, string) (containerservice.AgentPool, error)
	CreateOrUpdate(context.Context, string, string, string, containerservice.AgentPool, map[string]string) error
	Delete(context.Context, string, string, string) error
}

// AzureClient contains the Azure go-sdk Client.
type AzureClient struct {
	aksAgentPools containerservice.AgentPoolsClient
}

var _ Client = &AzureClient{}

// NewClient creates a new AKS "AgentPools" (node pools) client from subscription ID.
func NewClient(auth azure.Authorizer) *AzureClient {
	c := newAKSAgentPoolsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &AzureClient{c}
}

// newAKSAgentPoolsClient creates a new AKS "AgentPool" (node pool) client from subscription ID.
func newAKSAgentPoolsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) containerservice.AgentPoolsClient {
	aksAgentPoolsClient := containerservice.NewAgentPoolsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&aksAgentPoolsClient.Client, authorizer)
	return aksAgentPoolsClient
}

// Get gets an AKS "AgentPool" (node pool).
func (ac *AzureClient) Get(ctx context.Context, resourceGroupName, cluster, name string) (containerservice.AgentPool, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "managedmachinepools.AzureClient.Get")
	defer done()

	return ac.aksAgentPools.Get(ctx, resourceGroupName, cluster, name)
}

// CreateOrUpdate creates or updates an AKS "AgentPool" (node pool).
func (ac *AzureClient) CreateOrUpdate(ctx context.Context, resourceGroupName, cluster, name string,
	properties containerservice.AgentPool, customHeaders map[string]string) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "managedmachinpools.AzureClient.CreateOrUpdate")
	defer done()

	preparer, err := ac.aksAgentPools.CreateOrUpdatePreparer(ctx, resourceGroupName, cluster, name, properties)
	if err != nil {
		return errors.Wrap(err, "failed to prepare operation")
	}
	for key, element := range customHeaders {
		preparer.Header.Add(key, element)
	}

	future, err := ac.aksAgentPools.CreateOrUpdateSender(preparer)
	if err != nil {
		return errors.Wrap(err, "failed to begin operation")
	}
	if err := future.WaitForCompletionRef(ctx, ac.aksAgentPools.Client); err != nil {
		return errors.Wrap(err, "failed to end operation")
	}
	_, err = future.Result(ac.aksAgentPools)
	return err
}

// Delete deletes an AKS "AgentPool" (node pool).
func (ac *AzureClient) Delete(ctx context.Context, resourceGroupName, cluster, name string) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "managedmachinepools.AzureClient.Delete")
	defer done()

	future, err := ac.aksAgentPools.Delete(ctx, resourceGroupName, cluster, name)
	if err != nil {
		return errors.Wrap(err, "failed to begin operation")
	}
	if err := future.WaitForCompletionRef(ctx, ac.aksAgentPools.Client); err != nil {
		return errors.Wrap(err, "failed to end operation")
	}
	_, err = future.Result(ac.aksAgentPools)
	return err
}

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

package agentpools

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
	Get(context.Context, string, string, string) (containerservice.AgentPool, error)
	CreateOrUpdate(context.Context, string, string, string, containerservice.AgentPool, map[string]string) error
	Delete(context.Context, string, string, string) error
}

// AzureClient contains the Azure go-sdk Client.
type AzureClient struct {
	agentpools containerservice.AgentPoolsClient
}

var _ Client = &AzureClient{}

// NewClient creates a new agent pools client from subscription ID.
func NewClient(auth azure.Authorizer) *AzureClient {
	c := newAgentPoolsClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &AzureClient{c}
}

// newAgentPoolsClient creates a new agent pool client from subscription ID.
func newAgentPoolsClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) containerservice.AgentPoolsClient {
	agentPoolsClient := containerservice.NewAgentPoolsClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&agentPoolsClient.Client, authorizer)
	return agentPoolsClient
}

// Get gets an agent pool.
func (ac *AzureClient) Get(ctx context.Context, resourceGroupName, cluster, name string) (containerservice.AgentPool, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "agentpools.AzureClient.Get")
	defer done()

	return ac.agentpools.Get(ctx, resourceGroupName, cluster, name)
}

// CreateOrUpdate creates or updates an agent pool.
func (ac *AzureClient) CreateOrUpdate(ctx context.Context, resourceGroupName, cluster, name string,
	properties containerservice.AgentPool, customHeaders map[string]string) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "agentpools.AzureClient.CreateOrUpdate")
	defer done()

	preparer, err := ac.agentpools.CreateOrUpdatePreparer(ctx, resourceGroupName, cluster, name, properties)
	if err != nil {
		return errors.Wrap(err, "failed to prepare operation")
	}
	for key, element := range customHeaders {
		preparer.Header.Add(key, element)
	}

	future, err := ac.agentpools.CreateOrUpdateSender(preparer)
	if err != nil {
		return errors.Wrap(err, "failed to begin operation")
	}
	if err := future.WaitForCompletionRef(ctx, ac.agentpools.Client); err != nil {
		return errors.Wrap(err, "failed to end operation")
	}
	_, err = future.Result(ac.agentpools)
	return err
}

// Delete deletes an agent pool.
func (ac *AzureClient) Delete(ctx context.Context, resourceGroupName, cluster, name string) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "agentpools.AzureClient.Delete")
	defer done()
	ignorePodDisruptionBudget := false
	future, err := ac.agentpools.Delete(ctx, resourceGroupName, cluster, name, &ignorePodDisruptionBudget)
	if err != nil {
		return errors.Wrap(err, "failed to begin operation")
	}
	if err := future.WaitForCompletionRef(ctx, ac.agentpools.Client); err != nil {
		return errors.Wrap(err, "failed to end operation")
	}
	_, err = future.Result(ac.agentpools)
	return err
}

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

package loadbalancers

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Client wraps go-sdk
type Client interface {
	Get(context.Context, string, string) (network.LoadBalancer, error)
	CreateOrUpdate(context.Context, string, string, network.LoadBalancer) error
	Delete(context.Context, string, string) error
}

// AzureClient contains the Azure go-sdk Client
type AzureClient struct {
	loadbalancers network.LoadBalancersClient
}

var _ Client = &AzureClient{}

// NewClient creates a new load balancer client from subscription ID.
func NewClient(auth azure.Authorizer) *AzureClient {
	c := newLoadBalancersClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &AzureClient{c}
}

// newLoadbalancersClient creates a new load balancer client from subscription ID.
func newLoadBalancersClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) network.LoadBalancersClient {
	loadBalancersClient := network.NewLoadBalancersClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&loadBalancersClient.Client, authorizer)
	return loadBalancersClient
}

// Get gets the specified load balancer.
func (ac *AzureClient) Get(ctx context.Context, resourceGroupName, lbName string) (network.LoadBalancer, error) {
	ctx, span := tele.Tracer().Start(ctx, "loadbalancers.AzureClient.Get")
	defer span.End()

	return ac.loadbalancers.Get(ctx, resourceGroupName, lbName, "")
}

// CreateOrUpdate creates or updates a load balancer.
func (ac *AzureClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, lbName string, lb network.LoadBalancer) error {
	ctx, span := tele.Tracer().Start(ctx, "loadbalancers.AzureClient.CreateOrUpdate")
	defer span.End()

	var etag string
	if lb.Etag != nil {
		etag = *lb.Etag
	}

	req, err := ac.loadbalancers.CreateOrUpdatePreparer(ctx, resourceGroupName, lbName, lb)
	if err != nil {
		err = autorest.NewErrorWithError(err, "network.LoadBalancersClient", "CreateOrUpdate", nil, "Failure preparing request")
		return err
	}

	if etag != "" {
		req.Header.Add("If-Match", etag)
	}

	future, err := ac.loadbalancers.CreateOrUpdateSender(req)
	if err != nil {
		err = autorest.NewErrorWithError(err, "network.LoadBalancersClient", "CreateOrUpdate", future.Response(), "Failure sending request")
		return err
	}

	err = future.WaitForCompletionRef(ctx, ac.loadbalancers.Client)
	if err != nil {
		return err
	}

	_, err = future.Result(ac.loadbalancers)
	return err
}

// Delete deletes the specified load balancer.
func (ac *AzureClient) Delete(ctx context.Context, resourceGroupName, lbName string) error {
	ctx, span := tele.Tracer().Start(ctx, "loadbalancers.AzureClient.Delete")
	defer span.End()

	future, err := ac.loadbalancers.Delete(ctx, resourceGroupName, lbName)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.loadbalancers.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.loadbalancers)
	return err
}

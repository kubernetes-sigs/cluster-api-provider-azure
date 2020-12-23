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

package inboundnatrules

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// client wraps go-sdk
type client interface {
	Get(context.Context, string, string, string) (network.InboundNatRule, error)
	CreateOrUpdate(context.Context, string, string, string, network.InboundNatRule) error
	Delete(context.Context, string, string, string) error
}

// azureClient contains the Azure go-sdk Client
type azureClient struct {
	inboundnatrules network.InboundNatRulesClient
}

var _ client = (*azureClient)(nil)

// newClient creates a new inbound NAT rules client from subscription ID.
func newClient(auth azure.Authorizer) *azureClient {
	c := newInboundNatRulesClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &azureClient{c}
}

// newLoadbalancersClient creates a new inbound NAT rules client from subscription ID.
func newInboundNatRulesClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) network.InboundNatRulesClient {
	inboundNatRulesClient := network.NewInboundNatRulesClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&inboundNatRulesClient.Client, authorizer)
	return inboundNatRulesClient
}

// Get gets the specified inbound NAT rules.
func (ac *azureClient) Get(ctx context.Context, resourceGroupName, lbName, inboundNatRuleName string) (network.InboundNatRule, error) {
	ctx, span := tele.Tracer().Start(ctx, "inboundnatrules.AzureClient.Get")
	defer span.End()

	return ac.inboundnatrules.Get(ctx, resourceGroupName, lbName, inboundNatRuleName, "")
}

// CreateOrUpdate creates or updates a inbound NAT rules.
func (ac *azureClient) CreateOrUpdate(ctx context.Context, resourceGroupName string, lbName string, inboundNatRuleName string, inboundNatRuleParameters network.InboundNatRule) error {
	ctx, span := tele.Tracer().Start(ctx, "inboundnatrules.AzureClient.CreateOrUpdate")
	defer span.End()

	future, err := ac.inboundnatrules.CreateOrUpdate(ctx, resourceGroupName, lbName, inboundNatRuleName, inboundNatRuleParameters)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.inboundnatrules.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.inboundnatrules)
	return err
}

// Delete deletes the specified inbound NAT rules.
func (ac *azureClient) Delete(ctx context.Context, resourceGroupName, lbName, inboundNatRuleName string) error {
	ctx, span := tele.Tracer().Start(ctx, "inboundnatrules.AzureClient.Delete")
	defer span.End()

	future, err := ac.inboundnatrules.Delete(ctx, resourceGroupName, lbName, inboundNatRuleName)
	if err != nil {
		return err
	}
	err = future.WaitForCompletionRef(ctx, ac.inboundnatrules.Client)
	if err != nil {
		return err
	}
	_, err = future.Result(ac.inboundnatrules)
	return err
}

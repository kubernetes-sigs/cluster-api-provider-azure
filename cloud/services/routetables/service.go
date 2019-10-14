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

package routetables

import (
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-08-01/network"
	"github.com/Azure/go-autorest/autorest"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
)

var _ azure.Service = (*Service)(nil)

// Service provides operations on resource groups
type Service struct {
	Client network.RouteTablesClient
	Scope  *scope.ClusterScope
}

// getGroupsClient creates a new groups client from subscriptionid.
func getRouteTablesClient(subscriptionID string, authorizer autorest.Authorizer) network.RouteTablesClient {
	routeTablesClient := network.NewRouteTablesClient(subscriptionID)
	routeTablesClient.Authorizer = authorizer
	routeTablesClient.AddToUserAgent(azure.UserAgent)
	return routeTablesClient
}

// NewService creates a new groups service.
func NewService(scope *scope.ClusterScope) *Service {
	return &Service{
		Client: getRouteTablesClient(scope.SubscriptionID, scope.Authorizer),
		Scope:  scope,
	}
}

/*
Copyright 2018 The Kubernetes Authors.

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

package azure

import (
	"context"

	"github.com/Azure/go-autorest/autorest"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
)

// Service is a generic interface used by components offering a type of service.
// Example: virtualnetworks service would offer Reconcile/Delete methods.
type Service interface {
	Reconcile(ctx context.Context) error
	Delete(ctx context.Context) error
}

// OldService is a generic interface for services that have not yet been refactored.
// Once all services have been converted to use Service, this should be removed.
// Example: virtualnetworks service would offer Reconcile/Delete methods.
type OldService interface {
	Reconcile(ctx context.Context, spec interface{}) error
	Delete(ctx context.Context, spec interface{}) error
}

// CredentialGetter is a Service which knows how to retrieve credentials for an Azure
// resource in a resource group.
type CredentialGetter interface {
	Service
	GetCredentials(ctx context.Context, group string, cluster string) ([]byte, error)
}

// Authorizer is an interface which can get the subscription ID, base URI, and authorizer for an Azure service.
type Authorizer interface {
	SubscriptionID() string
	ClientID() string
	ClientSecret() string
	CloudEnvironment() string
	TenantID() string
	BaseURI() string
	Authorizer() autorest.Authorizer
}

// ClusterDescriber is an interface which can get common Azure Cluster information
type ClusterDescriber interface {
	Authorizer
	ResourceGroup() string
	ClusterName() string
	Location() string
	AdditionalTags() infrav1.Tags
	Vnet() *infrav1.VnetSpec
	IsVnetManaged() bool
	NodeSubnet() *infrav1.SubnetSpec
	ControlPlaneSubnet() *infrav1.SubnetSpec
	RouteTable() *infrav1.RouteTable
	IsIPv6Enabled() bool
}

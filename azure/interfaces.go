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

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// Reconciler is a generic interface used by components offering a type of service.
// Example: virtualnetworks service would offer Reconcile/Delete methods.
type Reconciler interface {
	Reconcile(ctx context.Context) error
	Delete(ctx context.Context) error
}

// CredentialGetter is a Service which knows how to retrieve credentials for an Azure
// resource in a resource group.
type CredentialGetter interface {
	Reconciler
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
	HashKey() string
}

// NetworkDescriber is an interface which can get common Azure Cluster Networking information.
type NetworkDescriber interface {
	Vnet() *infrav1.VnetSpec
	IsVnetManaged() bool
	ControlPlaneSubnet() infrav1.SubnetSpec
	Subnets() infrav1.Subnets
	Subnet(string) infrav1.SubnetSpec
	NodeSubnets() []infrav1.SubnetSpec
	SetSubnet(infrav1.SubnetSpec)
	IsIPv6Enabled() bool
	ControlPlaneRouteTable() infrav1.RouteTable
	APIServerLB() *infrav1.LoadBalancerSpec
	APIServerLBName() string
	APIServerLBPoolName(string) string
	IsAPIServerPrivate() bool
	GetPrivateDNSZoneName() string
	OutboundLBName(string) string
	OutboundPoolName(string) string
}

// ClusterDescriber is an interface which can get common Azure Cluster information.
type ClusterDescriber interface {
	Authorizer
	ResourceGroup() string
	ClusterName() string
	Location() string
	AdditionalTags() infrav1.Tags
	AvailabilitySetEnabled() bool
	CloudProviderConfigOverrides() *infrav1.CloudProviderConfigOverrides
	FailureDomains() []string
}

// AsyncStatusUpdater is an interface used to keep track of long running operations in Status that has Conditions and Futures.
type AsyncStatusUpdater interface {
	SetLongRunningOperationState(*infrav1.Future)
	GetLongRunningOperationState(string, string) *infrav1.Future
	DeleteLongRunningOperationState(string, string)
	UpdatePutStatus(clusterv1.ConditionType, string, error)
	UpdateDeleteStatus(clusterv1.ConditionType, string, error)
	UpdatePatchStatus(clusterv1.ConditionType, string, error)
}

// ClusterScoper combines the ClusterDescriber and NetworkDescriber interfaces.
type ClusterScoper interface {
	ClusterDescriber
	NetworkDescriber
}

// ResourceSpecGetter is an interface for getting all the required information to create/update/delete an Azure resource.
type ResourceSpecGetter interface {
	// ResourceName returns the name of the resource.
	ResourceName() string
	// OwnerResourceName returns the name of the resource that owns the resource
	// in the case that the resource is an Azure subresource.
	OwnerResourceName() string
	// ResourceGroupName returns the name of the resource group the resource is in.
	ResourceGroupName() string
	// Parameters takes the existing resource and returns the desired parameters of the resource.
	// If the resource does not exist, or we do not care about existing parameters to update the resource, existing should be nil.
	// If no update is needed on the resource, Parameters should return nil.
	Parameters(existing interface{}) (params interface{}, err error)
}

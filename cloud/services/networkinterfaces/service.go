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

package networkinterfaces

import (
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/inboundnatrules"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/internalloadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicloadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/subnets"
)

// Service provides operations on azure resources
type Service struct {
	Scope        *scope.ClusterScope
	MachineScope *scope.MachineScope
	Client
	SubnetsClient               subnets.Client
	PublicLoadBalancersClient   publicloadbalancers.Client
	InternalLoadBalancersClient internalloadbalancers.Client
	PublicIPsClient             publicips.Client
	InboundNATRulesClient       inboundnatrules.Client
}

// NewService creates a new service.
func NewService(scope *scope.ClusterScope, machineScope *scope.MachineScope) *Service {
	return &Service{
		Scope:                       scope,
		MachineScope:                machineScope,
		Client:                      NewClient(scope.SubscriptionID, scope.Authorizer),
		SubnetsClient:               subnets.NewClient(scope.SubscriptionID, scope.Authorizer),
		PublicLoadBalancersClient:   publicloadbalancers.NewClient(scope.SubscriptionID, scope.Authorizer),
		InternalLoadBalancersClient: internalloadbalancers.NewClient(scope.SubscriptionID, scope.Authorizer),
		PublicIPsClient:             publicips.NewClient(scope.SubscriptionID, scope.Authorizer),
		InboundNATRulesClient:       inboundnatrules.NewClient(scope.SubscriptionID, scope.Authorizer),
	}
}

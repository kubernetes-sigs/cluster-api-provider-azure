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
	"github.com/go-logr/logr"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/inboundnatrules"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/loadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/subnets"
)

// NICScope defines the scope interface for a network interfaces service.
type NICScope interface {
	azure.ClusterDescriber
	logr.Logger
	NICSpecs() []azure.NICSpec
}

// Service provides operations on azure resources
type Service struct {
	Scope NICScope
	Client
	SubnetsClient         subnets.Client
	LoadBalancersClient   loadbalancers.Client
	PublicIPsClient       publicips.Client
	InboundNATRulesClient inboundnatrules.Client
	ResourceSkusClient    resourceskus.Client
}

// NewService creates a new service.
func NewService(scope NICScope) *Service {
	return &Service{
		Scope:                 scope,
		Client:                NewClient(scope),
		SubnetsClient:         subnets.NewClient(scope),
		LoadBalancersClient:   loadbalancers.NewClient(scope),
		PublicIPsClient:       publicips.NewClient(scope),
		InboundNATRulesClient: inboundnatrules.NewClient(scope),
		ResourceSkusClient:    resourceskus.NewClient(scope),
	}
}

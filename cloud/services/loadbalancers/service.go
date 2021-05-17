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
	"github.com/go-logr/logr"
	azure "github.com/niachary/cluster-api-provider-azure/cloud"
	"github.com/niachary/cluster-api-provider-azure/cloud/services/virtualnetworks"
)

// LBScope defines the scope interface for a load balancer service.
type LBScope interface {
	azure.ClusterDescriber
	logr.Logger
	LBSpecs() []azure.LBSpec
}

// Service provides operations on azure resources
type Service struct {
	Scope LBScope
	Client
	VirtualNetworksClient virtualnetworks.Client
}

// NewService creates a new service.
func NewService(scope LBScope) *Service {
	return &Service{
		Scope:                 scope,
		Client:                NewClient(scope),
		VirtualNetworksClient: virtualnetworks.NewClient(scope),
	}
}

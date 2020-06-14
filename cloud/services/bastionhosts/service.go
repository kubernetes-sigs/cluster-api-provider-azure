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

package bastionhosts

import (
	"github.com/go-logr/logr"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/subnets"
)

// BastionScope defines the scope interface for a bastion host service.
type BastionScope interface {
	azure.ClusterDescriber
	logr.Logger
	BastionSpecs() []azure.BastionSpec
}

// Service provides operations on azure resources
type Service struct {
	Scope BastionScope
	Client
	SubnetsClient   subnets.Client
	PublicIPsClient publicips.Client
}

// NewService creates a new service.
func NewService(scope BastionScope) *Service {
	return &Service{
		Scope:           scope,
		Client:          NewClient(scope),
		SubnetsClient:   subnets.NewClient(scope),
		PublicIPsClient: publicips.NewClient(scope),
	}
}

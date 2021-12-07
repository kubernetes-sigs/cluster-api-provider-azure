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
	"context"

	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/subnets"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// BastionScope defines the scope interface for a bastion host service.
type BastionScope interface {
	azure.ClusterDescriber
	azure.NetworkDescriber
	BastionSpec() azure.BastionSpec
}

// Service provides operations on Azure resources.
type Service struct {
	Scope BastionScope
	client
	subnetsClient   subnets.Client
	publicIPsClient publicips.Client
}

// New creates a new service.
func New(scope BastionScope) *Service {
	return &Service{
		Scope:           scope,
		client:          newClient(scope),
		subnetsClient:   subnets.NewClient(scope),
		publicIPsClient: publicips.NewClient(scope),
	}
}

// Reconcile gets/creates/updates a bastion host.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "bastionhosts.Service.Reconcile")
	defer done()

	azureBastionSpec := s.Scope.BastionSpec().AzureBastion
	if azureBastionSpec != nil {
		err := s.ensureAzureBastion(ctx, *azureBastionSpec)
		if err != nil {
			return errors.Wrap(err, "error creating Azure Bastion")
		}
	}

	return nil
}

// Delete deletes the bastion host with the provided scope.
func (s *Service) Delete(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "bastionhosts.Service.Delete")
	defer done()

	azureBastionSpec := s.Scope.BastionSpec().AzureBastion
	if azureBastionSpec != nil {
		err := s.ensureAzureBastionDeleted(ctx, *azureBastionSpec)
		if err != nil {
			return errors.Wrap(err, "error deleting Azure Bastion")
		}
	}
	return nil
}

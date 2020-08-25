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

package natgateways

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Reconcile gets/creates/updates a nat gateway.
func (s *Service) Reconcile(ctx context.Context) error {
	if !s.Scope.Vnet().IsManaged(s.Scope.ClusterName()) {
		s.Scope.V(4).Info("Skipping nat gateways reconcile in custom vnet mode")
		return nil
	}

	for _, natGatewaySpec := range s.Scope.NatGatewaySpecs() {
		existingNatGateway, err := s.Get(ctx, s.Scope.ResourceGroup(), natGatewaySpec.Name)
		if !azure.ResourceNotFound(err) {
			if err != nil {
				return errors.Wrapf(err, "failed to get nat gateway %s in %s", natGatewaySpec.Name, s.Scope.ResourceGroup())
			}

			// nat gateway already exists
			// currently don't support:
			//  1. creating separate control plane and node (#718) so update both
			s.Scope.NodeSubnet().NatGateway.Name = to.String(existingNatGateway.Name)
			s.Scope.NodeSubnet().NatGateway.ID = to.String(existingNatGateway.ID)
			s.Scope.ControlPlaneSubnet().NatGateway.Name = to.String(existingNatGateway.Name)
			s.Scope.ControlPlaneSubnet().NatGateway.ID = to.String(existingNatGateway.ID)

			return nil
		}

		s.Scope.V(2).Info("creating nat gateway", "nat gateway", natGatewaySpec.Name)
		err = s.Client.CreateOrUpdate(
			ctx,
			s.Scope.ResourceGroup(),
			natGatewaySpec.Name,
			network.NatGateway{
				NatGatewayPropertiesFormat: &network.NatGatewayPropertiesFormat{},
				Location:                   to.StringPtr(s.Scope.Location()),
			},
		)
		if err != nil {
			return errors.Wrapf(err, "failed to create nat gateway %s in resource group %s", natGatewaySpec.Name, s.Scope.ResourceGroup())
		}

		s.Scope.V(2).Info("successfully created nat gateway", "nat gateway", natGatewaySpec.Name)
	}
	return nil
}

// Delete deletes the nat gateway with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	if !s.Scope.Vnet().IsManaged(s.Scope.ClusterName()) {
		s.Scope.V(4).Info("Skipping nat gateway deletion in custom vnet mode")
		return nil
	}
	for _, natGatewaySpec := range s.Scope.NatGatewaySpecs() {
		s.Scope.V(2).Info("deleting nat gateway", "nat gateway", natGatewaySpec.Name)
		err := s.Client.Delete(ctx, s.Scope.ResourceGroup(), natGatewaySpec.Name)
		if err != nil && azure.ResourceNotFound(err) {
			// already deleted
			continue
		}
		if err != nil {
			return errors.Wrapf(err, "failed to delete nat gateway %s in resource group %s", natGatewaySpec.Name, s.Scope.ResourceGroup())
		}

		s.Scope.V(2).Info("successfully deleted nat gateway", "nat gateway", natGatewaySpec.Name)
	}
	return nil
}

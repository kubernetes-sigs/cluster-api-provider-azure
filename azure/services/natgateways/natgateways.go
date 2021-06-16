/*
Copyright 2021 The Kubernetes Authors.

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
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// NatGatewayScope defines the scope interface for nat gateway service.
type NatGatewayScope interface {
	logr.Logger
	azure.ClusterScoper
	NatGatewaySpecs() []azure.NatGatewaySpec
}

// Service provides operations on azure resources.
type Service struct {
	Scope NatGatewayScope
	client
}

// New creates a new service.
func New(scope NatGatewayScope) *Service {
	return &Service{
		Scope:  scope,
		client: newClient(scope),
	}
}

// Reconcile gets/creates/updates a nat gateway.
// Only when the Nat Gateway 'Name' property is defined we create the Nat Gateway: it's opt-in.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "natgateways.Service.Reconcile")
	defer span.End()

	if !s.Scope.Vnet().IsManaged(s.Scope.ClusterName()) {
		s.Scope.V(4).Info("Skipping nat gateways reconcile in custom vnet mode")
		return nil
	}

	for _, natGatewaySpec := range s.Scope.NatGatewaySpecs() {
		existingNatGateway, err := s.getExisting(ctx, natGatewaySpec)

		switch {
		case err != nil && !azure.ResourceNotFound(err):
			return errors.Wrapf(err, "failed to get nat gateway %s in %s", natGatewaySpec.Name, s.Scope.ResourceGroup())
		case err == nil:
			// nat gateway already exists
			s.Scope.V(4).Info("nat gateway already exists", "nat gateway", natGatewaySpec.Name)
			natGatewaySpec.Subnet.NatGateway.ID = existingNatGateway.ID

			if existingNatGateway.NatGatewayIP.Name == natGatewaySpec.NatGatewayIP.Name {
				// Skip update for Nat Gateway as it exists with expected values
				s.Scope.V(4).Info("Nat Gateway exists with expected values, skipping update", "nat gateway", natGatewaySpec.Name)
				natGatewaySpec.Subnet.NatGateway = *existingNatGateway
				s.Scope.SetSubnet(natGatewaySpec.Subnet)
				continue
			}
		default:
			// nat gateway doesn't exist but its name was specified in the subnet, let's create it
			s.Scope.V(2).Info("nat gateway doesn't exist yet, creating it", "nat gateway", natGatewaySpec.Name)
		}

		natGatewayToCreate := network.NatGateway{
			Location: to.StringPtr(s.Scope.Location()),
			Sku:      &network.NatGatewaySku{Name: network.Standard},
			NatGatewayPropertiesFormat: &network.NatGatewayPropertiesFormat{
				PublicIPAddresses: &[]network.SubResource{
					{
						ID: to.StringPtr(natGatewaySpec.NatGatewayIP.Name),
					},
				},
			},
		}
		err = s.client.CreateOrUpdate(ctx, s.Scope.ResourceGroup(), natGatewaySpec.Name, natGatewayToCreate)
		if err != nil {
			return errors.Wrapf(err, "failed to create nat gateway %s in resource group %s", natGatewaySpec.Name, s.Scope.ResourceGroup())
		}
		s.Scope.V(2).Info("successfully created nat gateway", "nat gateway", natGatewaySpec.Name)
		natGateway := infrav1.NatGateway{
			ID:   azure.NatGatewayID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), natGatewaySpec.Name),
			Name: natGatewaySpec.Name,
			NatGatewayIP: infrav1.PublicIPSpec{
				Name: *(*natGatewayToCreate.NatGatewayPropertiesFormat.PublicIPAddresses)[0].ID,
			},
		}
		natGatewaySpec.Subnet.NatGateway = natGateway
		s.Scope.SetSubnet(natGatewaySpec.Subnet)
	}
	return nil
}

func (s *Service) getExisting(ctx context.Context, spec azure.NatGatewaySpec) (*infrav1.NatGateway, error) {
	existingNatGateway, err := s.Get(ctx, s.Scope.ResourceGroup(), spec.Name)
	if err != nil {
		return nil, err
	}

	return &infrav1.NatGateway{
		ID:   to.String(existingNatGateway.ID),
		Name: to.String(existingNatGateway.Name),
		NatGatewayIP: infrav1.PublicIPSpec{
			Name: to.String((*existingNatGateway.PublicIPAddresses)[0].ID),
		},
	}, nil
}

// Delete deletes the nat gateway with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "natgateways.Service.Delete")
	defer span.End()

	if !s.Scope.Vnet().IsManaged(s.Scope.ClusterName()) {
		s.Scope.V(4).Info("Skipping nat gateway deletion in custom vnet mode")
		return nil
	}
	for _, natGatewaySpec := range s.Scope.NatGatewaySpecs() {
		s.Scope.V(2).Info("deleting nat gateway", "nat gateway", natGatewaySpec.Name)
		err := s.client.Delete(ctx, s.Scope.ResourceGroup(), natGatewaySpec.Name)
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

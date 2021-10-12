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

package vnetpeerings

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// VnetPeeringScope defines the scope interface for a subnet service.
type VnetPeeringScope interface {
	logr.Logger
	azure.Authorizer
	Vnet() *infrav1.VnetSpec
	ClusterName() string
	SubscriptionID() string
	VnetPeeringSpecs() []azure.VnetPeeringSpec
}

// Service provides operations on Azure resources.
type Service struct {
	Scope VnetPeeringScope
	Client
}

// New creates a new service.
func New(scope VnetPeeringScope) *Service {
	return &Service{
		Scope:  scope,
		Client: NewClient(scope),
	}
}

// Reconcile gets/creates/updates a peering.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "vnetpeerings.Service.Reconcile")
	defer span.End()

	for _, peeringSpec := range s.Scope.VnetPeeringSpecs() {
		vnetID := azure.VNetID(s.Scope.SubscriptionID(), peeringSpec.RemoteResourceGroup, peeringSpec.RemoteVnetName)
		peeringProperties := network.VirtualNetworkPeeringPropertiesFormat{
			RemoteVirtualNetwork: &network.SubResource{
				ID: to.StringPtr(vnetID),
			},
		}

		s.Scope.V(2).Info("creating peering", "peering", peeringSpec.PeeringName, "from", "vnet", peeringSpec.SourceVnetName, "to", "vnet", peeringSpec.RemoteVnetName)
		err := s.Client.CreateOrUpdate(
			ctx,
			peeringSpec.SourceResourceGroup,
			peeringSpec.SourceVnetName,
			peeringSpec.PeeringName,
			network.VirtualNetworkPeering{
				VirtualNetworkPeeringPropertiesFormat: &peeringProperties,
			},
		)

		if err != nil {
			return errors.Wrapf(err, "failed to create peering %s in resource group %s", peeringSpec.PeeringName, s.Scope.Vnet().ResourceGroup)
		}

		s.Scope.V(2).Info("successfully created peering", "peering", peeringSpec.PeeringName, "from", "vnet", peeringSpec.SourceVnetName, "to", "vnet", peeringSpec.RemoteVnetName)
	}

	return nil
}

// Delete deletes the peering with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "vnetpeerings.Service.Delete")
	defer span.End()
	for _, peeringSpec := range s.Scope.VnetPeeringSpecs() {
		s.Scope.V(2).Info("deleting peering in vnets", "vnet1", peeringSpec.SourceVnetName, "and", "vnet2", peeringSpec.RemoteVnetName)
		err := s.Client.Delete(ctx, peeringSpec.SourceResourceGroup, peeringSpec.SourceVnetName, peeringSpec.PeeringName)
		if err != nil && azure.ResourceNotFound(err) {
			// already deleted
			continue
		}
		if err != nil {
			return errors.Wrapf(err, "failed to delete peering %s in vnet %s and resource group %s", peeringSpec.PeeringName, peeringSpec.SourceVnetName, peeringSpec.SourceResourceGroup)
		}

		s.Scope.V(2).Info("successfully deleted peering in vnet", "peering", peeringSpec.PeeringName, "vnet", peeringSpec.SourceVnetName)
	}

	return nil
}

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

package routetables

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// RouteTableScope defines the scope interface for route table service
type RouteTableScope interface {
	logr.Logger
	azure.ClusterDescriber
	azure.NetworkDescriber
	RouteTableSpecs() []azure.RouteTableSpec
}

// Service provides operations on azure resources
type Service struct {
	Scope RouteTableScope
	client
}

// New creates a new service.
func New(scope *scope.ClusterScope) *Service {
	return &Service{
		Scope:  scope,
		client: newClient(scope),
	}
}

// Reconcile gets/creates/updates a route table.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "routetables.Service.Reconcile")
	defer span.End()

	if !s.Scope.Vnet().IsManaged(s.Scope.ClusterName()) {
		s.Scope.V(4).Info("Skipping route tables reconcile in custom vnet mode")
		return nil
	}

	for _, routeTableSpec := range s.Scope.RouteTableSpecs() {
		existingRouteTable, err := s.Get(ctx, s.Scope.ResourceGroup(), routeTableSpec.Name)
		if !azure.ResourceNotFound(err) {
			if err != nil {
				return errors.Wrapf(err, "failed to get route table %s in %s", routeTableSpec.Name, s.Scope.ResourceGroup())
			}

			// route table already exists
			// currently don't support specifying your own routes via spec
			routeTableSpec.Subnet.RouteTable.Name = to.String(existingRouteTable.Name)
			routeTableSpec.Subnet.RouteTable.ID = to.String(existingRouteTable.ID)

			continue
		}

		s.Scope.V(2).Info("creating Route Table", "route table", routeTableSpec.Name)
		err = s.client.CreateOrUpdate(
			ctx,
			s.Scope.ResourceGroup(),
			routeTableSpec.Name,
			network.RouteTable{
				Location:                   to.StringPtr(s.Scope.Location()),
				RouteTablePropertiesFormat: &network.RouteTablePropertiesFormat{},
			},
		)
		if err != nil {
			return errors.Wrapf(err, "failed to create route table %s in resource group %s", routeTableSpec.Name, s.Scope.ResourceGroup())
		}
		s.Scope.V(2).Info("successfully created route table", "route table", routeTableSpec.Name)
	}
	return nil
}

// Delete deletes the route table with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "routetables.Service.Delete")
	defer span.End()

	if !s.Scope.Vnet().IsManaged(s.Scope.ClusterName()) {
		s.Scope.V(4).Info("Skipping route table deletion in custom vnet mode")
		return nil
	}
	for _, routeTableSpec := range s.Scope.RouteTableSpecs() {
		s.Scope.V(2).Info("deleting route table", "route table", routeTableSpec.Name)
		err := s.client.Delete(ctx, s.Scope.ResourceGroup(), routeTableSpec.Name)
		if err != nil && azure.ResourceNotFound(err) {
			// already deleted
			continue
		}
		if err != nil {
			return errors.Wrapf(err, "failed to delete route table %s in resource group %s", routeTableSpec.Name, s.Scope.ResourceGroup())
		}

		s.Scope.V(2).Info("successfully deleted route table", "route table", routeTableSpec.Name)
	}
	return nil
}

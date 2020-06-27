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
	"github.com/pkg/errors"
	"k8s.io/klog"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Spec specification for route table.
type Spec struct {
	Name string
}

// Reconcile gets/creates/updates a route table.
func (s *Service) Reconcile(ctx context.Context, spec interface{}) error {
	if !s.Scope.Vnet().IsManaged(s.Scope.ClusterName()) {
		s.Scope.V(4).Info("Skipping route tables reconcile in custom vnet mode")
		return nil
	}
	routeTableSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid Route Table Specification")
	}

	existingRouteTable, err := s.Get(ctx, s.Scope.ResourceGroup(), routeTableSpec.Name)
	if !azure.ResourceNotFound(err) {
		if err != nil {
			return errors.Wrapf(err, "failed to get route table %s in %s", routeTableSpec.Name, s.Scope.ResourceGroup())
		}

		// route table already exists
		// currently don't support:
		//  1. creating separate control plane and node (#718) so update both
		//  2. specifying your own routes via spec
		s.Scope.NodeSubnet().RouteTable.Name = to.String(existingRouteTable.Name)
		s.Scope.NodeSubnet().RouteTable.ID = to.String(existingRouteTable.ID)
		s.Scope.ControlPlaneSubnet().RouteTable.Name = to.String(existingRouteTable.Name)
		s.Scope.ControlPlaneSubnet().RouteTable.ID = to.String(existingRouteTable.ID)

		return nil
	}

	s.Scope.Logger.V(2).Info("creating route table", "route table", routeTableSpec.Name)
	err = s.Client.CreateOrUpdate(
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

	s.Scope.Logger.V(2).Info("successfully created route table", "route table", routeTableSpec.Name)
	return nil
}

// Delete deletes the route table with the provided name.
func (s *Service) Delete(ctx context.Context, spec interface{}) error {
	if !s.Scope.Vnet().IsManaged(s.Scope.ClusterName()) {
		s.Scope.V(4).Info("Skipping route table deletion in custom vnet mode")
		return nil
	}
	routeTableSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid Route Table Specification")
	}
	klog.V(2).Infof("deleting route table %s", routeTableSpec.Name)
	err := s.Client.Delete(ctx, s.Scope.ResourceGroup(), routeTableSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "failed to delete route table %s in resource group %s", routeTableSpec.Name, s.Scope.ResourceGroup())
	}

	klog.V(2).Infof("successfully deleted route table %s", routeTableSpec.Name)
	return nil
}

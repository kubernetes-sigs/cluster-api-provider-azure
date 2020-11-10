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

package subnets

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// SubnetScope defines the scope interface for a subnet service.
type SubnetScope interface {
	logr.Logger
	azure.ClusterScoper
	SubnetSpecs() []azure.SubnetSpec
}

// Service provides operations on azure resources
type Service struct {
	Scope SubnetScope
	Client
}

// New creates a new service.
func New(scope SubnetScope) *Service {
	return &Service{
		Scope:  scope,
		Client: NewClient(scope),
	}
}

// Reconcile gets/creates/updates a subnet.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "subnets.Service.Reconcile")
	defer span.End()

	for _, subnetSpec := range s.Scope.SubnetSpecs() {
		existingSubnet, err := s.getExisting(ctx, s.Scope.Vnet().ResourceGroup, subnetSpec)
		switch {
		case err != nil && !azure.ResourceNotFound(err):
			return errors.Wrapf(err, "failed to get subnet %s", subnetSpec.Name)
		case err == nil:
			// subnet already exists, update the spec and skip creation
			var subnet *infrav1.SubnetSpec
			if subnetSpec.Role == infrav1.SubnetControlPlane {
				subnet = s.Scope.ControlPlaneSubnet()
			} else if subnetSpec.Role == infrav1.SubnetNode {
				subnet = s.Scope.NodeSubnet()
			} else {
				continue
			}

			subnet.Role = subnetSpec.Role
			subnet.Name = existingSubnet.Name
			subnet.CIDRBlocks = existingSubnet.CIDRBlocks
			subnet.ID = existingSubnet.ID

		case !s.Scope.IsVnetManaged():
			return fmt.Errorf("vnet was provided but subnet %s is missing", subnetSpec.Name)

		default:

			subnetProperties := network.SubnetPropertiesFormat{
				AddressPrefixes: &subnetSpec.CIDRs,
			}

			// workaround needed to avoid SubscriptionNotRegisteredForFeature for feature Microsoft.Network/AllowMultipleAddressPrefixesOnSubnet.
			if len(subnetSpec.CIDRs) == 1 {
				subnetProperties = network.SubnetPropertiesFormat{
					AddressPrefix: &subnetSpec.CIDRs[0],
				}
			}

			if subnetSpec.RouteTableName != "" {
				subnetProperties.RouteTable = &network.RouteTable{
					ID: to.StringPtr(azure.RouteTableID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), subnetSpec.RouteTableName)),
				}
			}

			if subnetSpec.SecurityGroupName != "" {
				subnetProperties.NetworkSecurityGroup = &network.SecurityGroup{
					ID: to.StringPtr(azure.SecurityGroupID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), subnetSpec.SecurityGroupName)),
				}
			}

			s.Scope.V(2).Info("creating subnet in vnet", "subnet", subnetSpec.Name, "vnet", subnetSpec.VNetName)
			err = s.Client.CreateOrUpdate(
				ctx,
				s.Scope.Vnet().ResourceGroup,
				subnetSpec.VNetName,
				subnetSpec.Name,
				network.Subnet{
					SubnetPropertiesFormat: &subnetProperties,
				},
			)
			if err != nil {
				return errors.Wrapf(err, "failed to create subnet %s in resource group %s", subnetSpec.Name, s.Scope.Vnet().ResourceGroup)
			}

			s.Scope.V(2).Info("successfully created subnet in vnet", "subnet", subnetSpec.Name, "vnet", subnetSpec.VNetName)

		}

	}
	return nil
}

// Delete deletes the subnet with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "subnets.Service.Delete")
	defer span.End()

	for _, subnetSpec := range s.Scope.SubnetSpecs() {
		if !s.Scope.Vnet().IsManaged(s.Scope.ClusterName()) {
			s.Scope.V(4).Info("Skipping subnets deletion in custom vnet mode")
			continue
		}
		s.Scope.V(2).Info("deleting subnet in vnet", "subnet", subnetSpec.Name, "vnet", subnetSpec.VNetName)
		err := s.Client.Delete(ctx, s.Scope.Vnet().ResourceGroup, subnetSpec.VNetName, subnetSpec.Name)
		if err != nil && azure.ResourceNotFound(err) {
			// already deleted
			continue
		}
		if err != nil {
			return errors.Wrapf(err, "failed to delete subnet %s in resource group %s", subnetSpec.Name, s.Scope.Vnet().ResourceGroup)
		}

		s.Scope.V(2).Info("successfully deleted subnet in vnet", "subnet", subnetSpec.Name, "vnet", subnetSpec.VNetName)
	}
	return nil
}

// getExisting provides information about an existing subnet.
func (s *Service) getExisting(ctx context.Context, rgName string, spec azure.SubnetSpec) (*infrav1.SubnetSpec, error) {
	ctx, span := tele.Tracer().Start(ctx, "subnets.Service.getExisting")
	defer span.End()

	subnet, err := s.Client.Get(ctx, rgName, spec.VNetName, spec.Name)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch subnet named %s in vnet %s", spec.VNetName, spec.Name)
	}

	var addresses []string
	if subnet.SubnetPropertiesFormat != nil && subnet.SubnetPropertiesFormat.AddressPrefix != nil {
		addresses = []string{to.String(subnet.SubnetPropertiesFormat.AddressPrefix)}
	} else if subnet.SubnetPropertiesFormat != nil && subnet.SubnetPropertiesFormat.AddressPrefixes != nil {
		addresses = to.StringSlice(subnet.SubnetPropertiesFormat.AddressPrefixes)
	}

	subnetSpec := &infrav1.SubnetSpec{
		Role:       spec.Role,
		Name:       to.String(subnet.Name),
		ID:         to.String(subnet.ID),
		CIDRBlocks: addresses,
	}

	return subnetSpec, nil
}

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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/virtualnetworks"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// SubnetScope defines the scope interface for a subnet service.
type SubnetScope interface {
	azure.ClusterScoper
	SubnetSpecs() []azure.SubnetSpec
	virtualnetworks.VNetScope
}

// Service provides operations on Azure resources.
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
	ctx, log, done := tele.StartSpanWithLogger(ctx, "subnets.Service.Reconcile")
	defer done()

	for _, subnetSpec := range s.Scope.SubnetSpecs() {
		existingSubnet, err := s.getExisting(ctx, s.Scope.Vnet().ResourceGroup, subnetSpec)

		switch {
		case err != nil && !azure.ResourceNotFound(err):
			return errors.Wrapf(err, "failed to get subnet %s", subnetSpec.Name)
		case err == nil:
			// subnet already exists, update the spec and skip creation
			s.Scope.SetSubnet(*existingSubnet)
			continue
		default:
			managed, err := s.IsManaged(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to check if subnet is managed")
			} else if !managed {
				return fmt.Errorf("vnet was provided but subnet %s is missing", subnetSpec.Name)
			}

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

			if subnetSpec.NatGatewayName != "" {
				subnetProperties.NatGateway = &network.SubResource{
					ID: to.StringPtr(azure.NatGatewayID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), subnetSpec.NatGatewayName)),
				}
			}

			if subnetSpec.SecurityGroupName != "" {
				subnetProperties.NetworkSecurityGroup = &network.SecurityGroup{
					ID: to.StringPtr(azure.SecurityGroupID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), subnetSpec.SecurityGroupName)),
				}
			}

			log.V(2).Info("creating subnet in vnet", "subnet", subnetSpec.Name, "vnet", subnetSpec.VNetName)
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

			log.V(2).Info("successfully created subnet in vnet", "subnet", subnetSpec.Name, "vnet", subnetSpec.VNetName)
		}
	}
	return nil
}

// Delete deletes the subnet with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "subnets.Service.Delete")
	defer done()

	managed, err := s.IsManaged(ctx)
	if azure.ResourceNotFound(err) {
		// if the vnet doesn't exist, then we don't need to delete the subnets
		// since subnets are a sub resource of the vnet, they must have been already deleted as well.
		return nil
	} else if err != nil {
		return err
	} else if !managed {
		log.V(4).Info("Skipping subnets deletion in custom vnet mode")
		return nil
	}

	for _, subnetSpec := range s.Scope.SubnetSpecs() {
		log.V(2).Info("deleting subnet in vnet", "subnet", subnetSpec.Name, "vnet", subnetSpec.VNetName)
		err = s.Client.Delete(ctx, s.Scope.Vnet().ResourceGroup, subnetSpec.VNetName, subnetSpec.Name)
		if azure.ResourceNotFound(err) {
			// already deleted
			continue
		}
		if err != nil {
			return errors.Wrapf(err, "failed to delete subnet %s in resource group %s", subnetSpec.Name, s.Scope.Vnet().ResourceGroup)
		}

		log.V(2).Info("successfully deleted subnet in vnet", "subnet", subnetSpec.Name, "vnet", subnetSpec.VNetName)
	}
	return nil
}

// IsManaged returns true if the associated virtual network is managed,
// meaning that the subnet's lifecycle is managed, and caches the result in scope so that other services that depend on the vnet can check if it is managed.
func (s *Service) IsManaged(ctx context.Context) (bool, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "subnets.Service.IsManaged")
	defer done()

	vnetSvc := virtualnetworks.New(s.Scope)
	return vnetSvc.IsManaged(ctx)
}

// getExisting provides information about an existing subnet.
func (s *Service) getExisting(ctx context.Context, rgName string, spec azure.SubnetSpec) (*infrav1.SubnetSpec, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "subnets.Service.getExisting")
	defer done()

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

	subnetSpec := s.Scope.Subnet(spec.Name)
	subnetSpec.ID = to.String(subnet.ID)
	subnetSpec.CIDRBlocks = addresses

	return &subnetSpec, nil
}

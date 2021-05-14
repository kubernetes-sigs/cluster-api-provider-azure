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

	"github.com/Azure/azure-sdk-for-go/profiles/2018-03-01/network/mgmt/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"

	infrav1 "github.com/niachary/cluster-api-provider-azure/api/v1alpha3"
	azure "github.com/niachary/cluster-api-provider-azure/cloud"
)

// getExisting provides information about an existing subnet.
func (s *Service) getExisting(ctx context.Context, rgName string, spec azure.SubnetSpec) (*infrav1.SubnetSpec, error) {
	subnet, err := s.Client.Get(ctx, rgName, spec.VNetName, spec.Name)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch subnet named %s in vnet %s", spec.VNetName, spec.Name)
	}

	subnetSpec := &infrav1.SubnetSpec{
		Role:                spec.Role,
		InternalLBIPAddress: spec.InternalLBIPAddress,
		Name:                to.String(subnet.Name),
		ID:                  to.String(subnet.ID),
		CidrBlock:           to.String(subnet.SubnetPropertiesFormat.AddressPrefix),
	}

	return subnetSpec, nil
}

// Reconcile gets/creates/updates a subnet.
func (s *Service) Reconcile(ctx context.Context) error {
	//log := klogr.New()
	for _, subnetSpec := range s.Scope.SubnetSpecs() {
		//subnetSpec.Name = "subnet1"
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
			subnet.CidrBlock = existingSubnet.CidrBlock
			subnet.ID = existingSubnet.ID

		case !s.Scope.IsVnetManaged():
			return fmt.Errorf("vnet was provided but subnet %s is missing", subnetSpec.Name)

		default:
			subnetProperties := network.SubnetPropertiesFormat{
				AddressPrefix: to.StringPtr(subnetSpec.CIDR),
			}
			/*if subnetSpec.RouteTableName != "" {
				log.Info("Inside routetable name")
				subnetProperties.RouteTable = &network.RouteTable{
					ID: to.StringPtr(azure.RouteTableID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), subnetSpec.RouteTableName)),
				}
			}*/

			/*if subnetSpec.SecurityGroupName != "" {
				log.Info("Inside security group name")
				subnetProperties.NetworkSecurityGroup = &network.SecurityGroup{
					ID: to.StringPtr(azure.SecurityGroupID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), subnetSpec.SecurityGroupName)),
				}
			}*/

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
		break
	}
	return nil
}

// Delete deletes the subnet with the provided name.
func (s *Service) Delete(ctx context.Context) error {
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

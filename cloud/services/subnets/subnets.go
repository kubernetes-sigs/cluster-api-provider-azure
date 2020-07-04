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
	"github.com/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Spec input specification for Get/CreateOrUpdate/Delete calls
type Spec struct {
	Name                string
	CIDR                string
	VnetName            string
	RouteTableName      string
	SecurityGroupName   string
	Role                infrav1.SubnetRole
	InternalLBIPAddress string
}

// getExisting provides information about an existing subnet.
func (s *Service) getExisting(ctx context.Context, rgName string, spec *Spec) (*infrav1.SubnetSpec, error) {
	subnet, err := s.Client.Get(ctx, rgName, spec.VnetName, spec.Name)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch subnet named %q in vnet %q", spec.VnetName, spec.Name)
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
func (s *Service) Reconcile(ctx context.Context, spec interface{}) error {
	subnetSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("Invalid Subnet Specification")
	}
	existingSubnet, err := s.getExisting(ctx, s.Scope.Vnet().ResourceGroup, subnetSpec)
	if err == nil {
		// subnet already exists, update the spec and skip creation
		var subnet *infrav1.SubnetSpec
		if subnetSpec.Role == infrav1.SubnetControlPlane {
			subnet = s.Scope.ControlPlaneSubnet()
		} else if subnetSpec.Role == infrav1.SubnetNode {
			subnet = s.Scope.NodeSubnet()
		} else {
			return nil
		}

		subnet.Role = subnetSpec.Role
		subnet.Name = existingSubnet.Name
		subnet.CidrBlock = existingSubnet.CidrBlock
		subnet.ID = existingSubnet.ID

		return nil
	}
	if !azure.ResourceNotFound(err) {
		if err != nil {
			return errors.Wrap(err, "failed to get subnet")
		}
	}
	if !s.Scope.Vnet().IsManaged(s.Scope.ClusterName()) {
		// if the vnet is unmanaged, we expect all subnets to be created as well
		return fmt.Errorf("vnet was provided but subnet %s is missing", subnetSpec.Name)
	}

	subnetProperties := network.SubnetPropertiesFormat{
		AddressPrefix: to.StringPtr(subnetSpec.CIDR),
	}
	if subnetSpec.RouteTableName != "" {
		s.Scope.V(2).Info("getting route table", "route table", subnetSpec.RouteTableName)
		rt, err := s.RouteTablesClient.Get(ctx, s.Scope.ResourceGroup(), subnetSpec.RouteTableName)
		if err != nil {
			return err
		}
		s.Scope.V(2).Info("successfully got route table", "route table", subnetSpec.RouteTableName)
		subnetProperties.RouteTable = &rt
	}

	s.Scope.V(2).Info("getting security group", "security group", subnetSpec.SecurityGroupName)
	nsg, err := s.SecurityGroupsClient.Get(ctx, s.Scope.ResourceGroup(), subnetSpec.SecurityGroupName)
	if err != nil {
		return err
	}
	s.Scope.V(2).Info("successfully got security group", "security group", subnetSpec.SecurityGroupName)
	subnetProperties.NetworkSecurityGroup = &nsg

	s.Scope.V(2).Info("creating subnet in vnet", "subnet", subnetSpec.Name, "vnet", subnetSpec.VnetName)
	err = s.Client.CreateOrUpdate(
		ctx,
		s.Scope.Vnet().ResourceGroup,
		subnetSpec.VnetName,
		subnetSpec.Name,
		network.Subnet{
			Name:                   to.StringPtr(subnetSpec.Name),
			SubnetPropertiesFormat: &subnetProperties,
		},
	)
	if err != nil {
		return errors.Wrapf(err, "failed to create subnet %s in resource group %s", subnetSpec.Name, s.Scope.Vnet().ResourceGroup)
	}

	s.Scope.V(2).Info("successfully created subnet in vnet", "subnet", subnetSpec.Name, "vnet", subnetSpec.VnetName)
	return nil
}

// Delete deletes the subnet with the provided name.
func (s *Service) Delete(ctx context.Context, spec interface{}) error {
	if !s.Scope.Vnet().IsManaged(s.Scope.ClusterName()) {
		s.Scope.V(4).Info("Skipping subnets deletion in custom vnet mode")
		return nil
	}
	subnetSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("Invalid Subnet Specification")
	}
	s.Scope.V(2).Info("deleting subnet in vnet", "subnet", subnetSpec.Name, "vnet", subnetSpec.VnetName)
	err := s.Client.Delete(ctx, s.Scope.Vnet().ResourceGroup, subnetSpec.VnetName, subnetSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "failed to delete subnet %s in resource group %s", subnetSpec.Name, s.Scope.Vnet().ResourceGroup)
	}

	s.Scope.V(2).Info("successfully deleted subnet in vnet", "subnet", subnetSpec.Name, "vnet", subnetSpec.VnetName)
	return nil
}

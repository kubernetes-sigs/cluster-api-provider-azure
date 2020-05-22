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
	"k8s.io/klog"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
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

// Get provides information about a subnet.
func (s *Service) Get(ctx context.Context, spec interface{}) (*infrav1.SubnetSpec, error) {
	subnetSpec, ok := spec.(*Spec)
	if !ok {
		return nil, errors.New("Invalid Subnet Specification")
	}
	subnet, err := s.Client.Get(ctx, s.Scope.Vnet().ResourceGroup, subnetSpec.VnetName, subnetSpec.Name)
	if err != nil {
		return nil, err
	}
	var sg infrav1.SecurityGroup
	if subnet.SubnetPropertiesFormat != nil && subnet.SubnetPropertiesFormat.NetworkSecurityGroup != nil {
		sg = infrav1.SecurityGroup{
			Name: to.String(subnet.SubnetPropertiesFormat.NetworkSecurityGroup.Name),
			ID:   to.String(subnet.SubnetPropertiesFormat.NetworkSecurityGroup.ID),
			Tags: converters.MapToTags(subnet.SubnetPropertiesFormat.NetworkSecurityGroup.Tags),
		}
	}
	var rt infrav1.RouteTable
	if subnet.SubnetPropertiesFormat != nil && subnet.SubnetPropertiesFormat.RouteTable != nil {
		rt = infrav1.RouteTable{
			Name: to.String(subnet.SubnetPropertiesFormat.RouteTable.Name),
			ID:   to.String(subnet.SubnetPropertiesFormat.RouteTable.ID),
		}
	}
	return &infrav1.SubnetSpec{
		Role:                subnetSpec.Role,
		InternalLBIPAddress: subnetSpec.InternalLBIPAddress,
		Name:                to.String(subnet.Name),
		ID:                  to.String(subnet.ID),
		CidrBlock:           to.String(subnet.SubnetPropertiesFormat.AddressPrefix),
		SecurityGroup:       sg,
		RouteTable:          rt,
	}, nil
}

// Reconcile gets/creates/updates a subnet.
func (s *Service) Reconcile(ctx context.Context, spec interface{}) error {
	subnetSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("Invalid Subnet Specification")
	}
	if subnet, err := s.Get(ctx, subnetSpec); err == nil {
		// subnet already exists, skip creation
		if subnetSpec.Role == infrav1.SubnetControlPlane {
			subnet.DeepCopyInto(s.Scope.ControlPlaneSubnet())
		} else if subnetSpec.Role == infrav1.SubnetNode {
			subnet.DeepCopyInto(s.Scope.NodeSubnet())
		}
		return nil
	}
	if !s.Scope.Vnet().IsManaged(s.Scope.Name()) {
		// if vnet is unmanaged, we expect all subnets to be created as well
		return fmt.Errorf("vnet was provided but subnet %s is missing", subnetSpec.Name)
	}

	subnetProperties := network.SubnetPropertiesFormat{
		AddressPrefix: to.StringPtr(subnetSpec.CIDR),
	}
	if subnetSpec.RouteTableName != "" {
		klog.V(2).Infof("getting route table %s", subnetSpec.RouteTableName)
		rt, err := s.RouteTablesClient.Get(ctx, s.Scope.ResourceGroup(), subnetSpec.RouteTableName)
		if err != nil {
			return err
		}
		klog.V(2).Infof("successfully got route table %s", subnetSpec.RouteTableName)
		subnetProperties.RouteTable = &rt
	}

	klog.V(2).Infof("getting nsg %s", subnetSpec.SecurityGroupName)
	nsg, err := s.SecurityGroupsClient.Get(ctx, s.Scope.ResourceGroup(), subnetSpec.SecurityGroupName)
	if err != nil {
		return err
	}
	klog.V(2).Infof("got nsg %s", subnetSpec.SecurityGroupName)
	subnetProperties.NetworkSecurityGroup = &nsg

	klog.V(2).Infof("creating subnet %s in vnet %s", subnetSpec.Name, subnetSpec.VnetName)
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

	klog.V(2).Infof("successfully created subnet %s in vnet %s", subnetSpec.Name, subnetSpec.VnetName)
	return nil
}

// Delete deletes the subnet with the provided name.
func (s *Service) Delete(ctx context.Context, spec interface{}) error {
	if !s.Scope.Vnet().IsManaged(s.Scope.Name()) {
		s.Scope.V(4).Info("Skipping subnets deletion in custom vnet mode")
		return nil
	}
	subnetSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("Invalid Subnet Specification")
	}
	klog.V(2).Infof("deleting subnet %s in vnet %s", subnetSpec.Name, subnetSpec.VnetName)
	err := s.Client.Delete(ctx, s.Scope.Vnet().ResourceGroup, subnetSpec.VnetName, subnetSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "failed to delete subnet %s in resource group %s", subnetSpec.Name, s.Scope.Vnet().ResourceGroup)
	}

	klog.V(2).Infof("successfully deleted subnet %s in vnet %s", subnetSpec.Name, subnetSpec.VnetName)
	return nil
}

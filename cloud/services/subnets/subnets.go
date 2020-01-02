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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Spec input specification for Get/CreateOrUpdate/Delete calls
type Spec struct {
	Name              string
	CIDR              string
	VnetName          string
	RouteTableName    string
	SecurityGroupName string
}

// Get provides information about a subnet.
func (s *Service) Get(ctx context.Context, spec interface{}) (interface{}, error) {
	subnetSpec, ok := spec.(*Spec)
	if !ok {
		return network.Subnet{}, errors.New("Invalid Subnet Specification")
	}
	subnet, err := s.Client.Get(ctx, s.Scope.AzureCluster.Spec.ResourceGroup, subnetSpec.VnetName, subnetSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		return nil, errors.Wrapf(err, "subnet %s not found", subnetSpec.Name)
	} else if err != nil {
		return subnet, err
	}
	return subnet, nil
}

// Reconcile gets/creates/updates a subnet.
func (s *Service) Reconcile(ctx context.Context, spec interface{}) error {
	subnetSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("Invalid Subnet Specification")
	}
	subnetProperties := network.SubnetPropertiesFormat{
		AddressPrefix: to.StringPtr(subnetSpec.CIDR),
	}
	if subnetSpec.RouteTableName != "" {
		klog.V(2).Infof("getting route table %s", subnetSpec.RouteTableName)
		rt, err := s.RouteTablesClient.Get(ctx, s.Scope.AzureCluster.Spec.ResourceGroup, subnetSpec.RouteTableName)
		if err != nil {
			return err
		}
		klog.V(2).Infof("successfully got route table %s", subnetSpec.RouteTableName)
		subnetProperties.RouteTable = &rt
	}

	klog.V(2).Infof("getting nsg %s", subnetSpec.SecurityGroupName)
	nsg, err := s.SecurityGroupsClient.Get(ctx, s.Scope.AzureCluster.Spec.ResourceGroup, subnetSpec.SecurityGroupName)
	if err != nil {
		return err
	}
	klog.V(2).Infof("got nsg %s", subnetSpec.SecurityGroupName)
	subnetProperties.NetworkSecurityGroup = &nsg

	klog.V(2).Infof("creating subnet %s in vnet %s", subnetSpec.Name, subnetSpec.VnetName)
	err = s.Client.CreateOrUpdate(
		ctx,
		s.Scope.AzureCluster.Spec.ResourceGroup,
		subnetSpec.VnetName,
		subnetSpec.Name,
		network.Subnet{
			Name:                   to.StringPtr(subnetSpec.Name),
			SubnetPropertiesFormat: &subnetProperties,
		},
	)
	if err != nil {
		return errors.Wrapf(err, "failed to create subnet %s in resource group %s", subnetSpec.Name, s.Scope.AzureCluster.Spec.ResourceGroup)
	}

	klog.V(2).Infof("successfully created subnet %s in vnet %s", subnetSpec.Name, subnetSpec.VnetName)
	return nil
}

// Delete deletes the subnet with the provided name.
func (s *Service) Delete(ctx context.Context, spec interface{}) error {
	if s.Scope.Vnet().IsManaged(s.Scope.Name()) {
		s.Scope.V(4).Info("Skipping subnets deletion in unmanaged mode")
		return nil
	}
	subnetSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("Invalid Subnet Specification")
	}
	klog.V(2).Infof("deleting subnet %s in vnet %s", subnetSpec.Name, subnetSpec.VnetName)
	err := s.Client.Delete(ctx, s.Scope.AzureCluster.Spec.ResourceGroup, subnetSpec.VnetName, subnetSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "failed to delete subnet %s in resource group %s", subnetSpec.Name, s.Scope.AzureCluster.Spec.ResourceGroup)
	}

	klog.V(2).Infof("successfully deleted subnet %s in vnet %s", subnetSpec.Name, subnetSpec.VnetName)
	return nil
}

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

package virtualnetworks

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha2"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
)

// Spec input specification for Get/CreateOrUpdate/Delete calls
type Spec struct {
	Name string
	CIDR string
}

// Get provides information about a virtual network.
func (s *Service) Get(ctx context.Context, spec interface{}) (interface{}, error) {
	vnetSpec, ok := spec.(*Spec)
	if !ok {
		return network.VirtualNetwork{}, errors.New("Invalid VNET Specification")
	}
	vnet, err := s.Client.Get(ctx, s.Scope.AzureCluster.Spec.ResourceGroup, vnetSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		return nil, errors.Wrapf(err, "vnet %s not found", vnetSpec.Name)
	} else if err != nil {
		return vnet, err
	}
	return vnet, nil
}

// Reconcile gets/creates/updates a virtual network.
func (s *Service) Reconcile(ctx context.Context, spec interface{}) error {
	// Following should be created upstream and provided as an input to NewService
	// A vnet has following dependencies
	//    * Vnet Cidr
	//    * Control Plane Subnet Cidr
	//    * Node Subnet Cidr
	//    * Control Plane NSG
	//    * Node NSG
	//    * Node Routetable
	vnetSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("Invalid VNET Specification")
	}

	if _, err := s.Get(ctx, vnetSpec); err == nil {
		// vnet already exists, cannot update since its immutable
		return nil
	}

	klog.V(2).Infof("creating vnet %s ", vnetSpec.Name)
	vnet := network.VirtualNetwork{
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: s.Scope.Name(),
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Name:        to.StringPtr(fmt.Sprintf("%s-vnet", s.Scope.Name())),
			Role:        to.StringPtr(infrav1.CommonRoleTagValue),
			Additional:  s.Scope.AdditionalTags(),
		})),
		Location: to.StringPtr(s.Scope.Location()),
		VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
			AddressSpace: &network.AddressSpace{
				AddressPrefixes: &[]string{vnetSpec.CIDR},
			},
		},
	}
	err := s.Client.CreateOrUpdate(ctx, s.Scope.AzureCluster.Spec.ResourceGroup, vnetSpec.Name, vnet)
	if err != nil {
		return err
	}

	klog.V(2).Infof("successfully created vnet %s ", vnetSpec.Name)
	return nil
}

// Delete deletes the virtual network with the provided name.
func (s *Service) Delete(ctx context.Context, spec interface{}) error {
	vnetSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("Invalid VNET Specification")
	}
	klog.V(2).Infof("deleting vnet %s ", vnetSpec.Name)
	err := s.Client.Delete(ctx, s.Scope.AzureCluster.Spec.ResourceGroup, vnetSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "failed to delete vnet %s in resource group %s", vnetSpec.Name, s.Scope.AzureCluster.Spec.ResourceGroup)
	}

	klog.V(2).Infof("successfully deleted vnet %s ", vnetSpec.Name)
	return nil
}

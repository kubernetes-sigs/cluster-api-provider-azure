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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-12-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/converters"
)

const (
	// DefaultVnetCIDR is the default virtual network CIDR range.
	DefaultVnetCIDR = "10.0.0.0/8"
)

// Reconcile reconciles information about a resource.
func (s *Service) Reconcile(ctx context.Context, _ azure.Spec) error {
	if s.Scope.Vnet().Name == "" {
		s.Scope.Vnet().Name = s.RetrieveVnetName()
	}

	klog.V(2).Infof("reconciling virtual network %s", s.Scope.Vnet().Name)
	vnet, err := s.Get(s.Scope.Context, s.Scope.ResourceGroup().Name, s.Scope.Vnet().Name)
	if err != nil {
		switch {
		case s.Scope.Vnet().IsProvided():
			return errors.Wrapf(err, "failed to reconcile virtual network %s: an unmanaged virtual network was specified, but cannot be found", s.Scope.Vnet().Name)
		case !s.Scope.Vnet().IsProvided():
			if s.Scope.Vnet().CidrBlock == "" {
				s.Scope.Vnet().CidrBlock = s.RetrieveVnetCIDR()
			}

			vnet, err = s.CreateOrUpdate(s.Scope.Context, s.Scope.ResourceGroup().Name, s.Scope.Vnet())
			if err != nil {
				return errors.Wrapf(err, "failed to reconcile virtual network %s", s.Scope.Vnet().Name)
			}
		default:
			return errors.Wrapf(err, "failed to reconcile virtual network %s", s.Scope.Vnet().Name)
		}
	}

	vnet.DeepCopyInto(s.Scope.Vnet())
	klog.V(2).Infof("successfully reconciled virtual network %s", s.Scope.Vnet().Name)
	return nil
}

// Get provides information about a virtual network.
func (s *Service) Get(ctx context.Context, resourceGroup, vnetName string) (vnet *v1alpha1.VnetSpec, err error) {
	klog.V(2).Infof("checking for virtual network %s", s.Scope.Vnet().Name)
	existingVnet, err := s.Client.Get(ctx, resourceGroup, vnetName, "")
	if err != nil && azure.ResourceNotFound(err) {
		return vnet, errors.Wrapf(err, "vnet %s not found", vnetName)
	} else if err != nil {
		return vnet, err
	}

	klog.V(2).Infof("successfully retrieved virtual network %s", s.Scope.Vnet().Name)
	return converters.SDKToVnet(existingVnet, s.Scope.Vnet().Managed), nil
}

// CreateOrUpdate creates or updates a virtual network.
func (s *Service) CreateOrUpdate(ctx context.Context, resourceGroup string, spec *v1alpha1.VnetSpec) (*v1alpha1.VnetSpec, error) {
	klog.V(2).Infof("creating/updating virtual network %s ", spec.Name)
	future, err := s.Client.CreateOrUpdate(
		ctx,
		resourceGroup,
		spec.Name,
		network.VirtualNetwork{
			Location: to.StringPtr(s.Scope.ClusterConfig.Location),
			VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
				AddressSpace: &network.AddressSpace{
					AddressPrefixes: &[]string{spec.CidrBlock},
				},
			},
		})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create/update virtual network %s", spec.Name)
	}

	err = future.WaitForCompletionRef(ctx, s.Client.Client)
	if err != nil {
		return nil, err
	}

	vnet, err := future.Result(s.Client)
	if err != nil {
		return nil, err
	}

	klog.V(2).Infof("successfully created/updated virtual network %s ", spec.Name)
	return converters.SDKToVnet(vnet, s.Scope.Vnet().Managed), nil
}

// Delete deletes the virtual network with the provided name.
func (s *Service) Delete(ctx context.Context, _ azure.Spec) error {
	klog.V(2).Infof("deleting virtual network %s", s.Scope.Vnet().Name)
	if s.Scope.Vnet().IsProvided() {
		klog.V(2).Infof("virtual network %s is unmanaged; skipping deletion", s.Scope.Vnet().Name)
		return nil
	}

	future, err := s.Client.Delete(ctx, s.Scope.ResourceGroup().Name, s.Scope.Vnet().Name)
	if err != nil && azure.ResourceNotFound(err) {
		return errors.Wrapf(err, "virtual network %s may have already been deleted", s.Scope.ResourceGroup().Name)
	} else if err != nil {
		return errors.Wrapf(err, "failed to delete virtual network %s in resource group %s", s.Scope.Vnet().Name, s.Scope.ResourceGroup().Name)
	}

	err = future.WaitForCompletionRef(ctx, s.Client.Client)
	if err != nil {
		return errors.Wrap(err, "cannot delete, future response")
	}

	_, err = future.Result(s.Client)

	klog.V(2).Infof("successfully deleted virtual network %s", s.Scope.Vnet().Name)
	return err
}

// RetrieveVnetName retrieves a virtual network name, based on VnetSpec or generates one based on the cluster name.
func (s *Service) RetrieveVnetName() string {
	if s.Scope.Vnet().Name != "" {
		return s.Scope.Vnet().Name
	}

	return fmt.Sprintf("%s-%s", s.Scope.Cluster.Name, "vnet")
}

// RetrieveVnetCIDR retrieves a virtual network CIDR, based on VnetSpec or generates a default CIDR.
func (s *Service) RetrieveVnetCIDR() string {
	if s.Scope.Vnet().CidrBlock != "" {
		return s.Scope.Vnet().CidrBlock
	}

	return DefaultVnetCIDR
}

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

package networkinterfaces

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-12-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/internalloadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/publicloadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/subnets"
)

// Spec specification for routetable
type Spec struct {
	Name                     string
	SubnetName               string
	VnetName                 string
	StaticIPAddress          string
	PublicLoadBalancerName   string
	InternalLoadBalancerName string
	IsBastion                bool
}

// Get provides information about a network interface.
func (s *Service) Get(ctx context.Context, spec v1alpha1.ResourceSpec) (interface{}, error) {
	nicSpec, ok := spec.(*Spec)
	if !ok {
		return network.Interface{}, errors.New("invalid network interface specification")
	}
	nic, err := s.Client.Get(ctx, s.Scope.ClusterConfig.ResourceGroup, nicSpec.Name, "")
	if err != nil && azure.ResourceNotFound(err) {
		return nil, errors.Wrapf(err, "network interface %s not found", nicSpec.Name)
	} else if err != nil {
		return nic, err
	}
	return nic, nil
}

// Reconcile gets/creates/updates a network interface.
func (s *Service) Reconcile(ctx context.Context, spec v1alpha1.ResourceSpec) error {
	nicSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid network interface specification")
	}

	nicConfig := &network.InterfaceIPConfigurationPropertiesFormat{}

	subnetInterface, err := subnets.NewService(s.Scope).Get(ctx, &subnets.Spec{Name: nicSpec.SubnetName, VnetName: nicSpec.VnetName})
	if err != nil {
		return err
	}

	subnet, ok := subnetInterface.(network.Subnet)
	if !ok {
		return errors.New("subnet get returned invalid network interface")
	}

	nicConfig.Subnet = &network.Subnet{ID: subnet.ID}
	nicConfig.PrivateIPAllocationMethod = network.Dynamic
	if nicSpec.StaticIPAddress != "" {
		nicConfig.PrivateIPAllocationMethod = network.Static
		nicConfig.PrivateIPAddress = to.StringPtr(nicSpec.StaticIPAddress)
	}

	backendAddressPools := []network.BackendAddressPool{}
	if nicSpec.PublicLoadBalancerName != "" {
		lbInterface, lberr := publicloadbalancers.NewService(s.Scope).Get(ctx, &publicloadbalancers.Spec{Name: nicSpec.PublicLoadBalancerName})
		if lberr != nil {
			return lberr
		}

		lb, ok := lbInterface.(network.LoadBalancer)
		if !ok {
			return errors.New("public load balancer get returned invalid network interface")
		}

		if !nicSpec.IsBastion {
			backendAddressPools = append(backendAddressPools,
				network.BackendAddressPool{
					ID: (*lb.BackendAddressPools)[0].ID,
				})
		} else {
			nicConfig.LoadBalancerInboundNatRules = &[]network.InboundNatRule{
				{
					ID: (*lb.InboundNatRules)[0].ID,
				},
			}
		}
	}
	if nicSpec.InternalLoadBalancerName != "" {
		internallbInterface, ilberr := internalloadbalancers.NewService(s.Scope).Get(ctx, &internalloadbalancers.Spec{Name: nicSpec.InternalLoadBalancerName})
		if ilberr != nil {
			return ilberr
		}

		internallb, ok := internallbInterface.(network.LoadBalancer)
		if !ok {
			return errors.New("internal load balancer get returned invalid network interface")
		}
		backendAddressPools = append(backendAddressPools,
			network.BackendAddressPool{
				ID: (*internallb.BackendAddressPools)[0].ID,
			})
	}
	nicConfig.LoadBalancerBackendAddressPools = &backendAddressPools

	future, err := s.Client.CreateOrUpdate(ctx,
		s.Scope.ClusterConfig.ResourceGroup,
		nicSpec.Name,
		network.Interface{
			Location: to.StringPtr(s.Scope.ClusterConfig.Location),
			InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
				IPConfigurations: &[]network.InterfaceIPConfiguration{
					{
						Name:                                     to.StringPtr("pipConfig"),
						InterfaceIPConfigurationPropertiesFormat: nicConfig,
					},
				},
			},
		})

	if err != nil {
		return errors.Wrapf(err, "failed to create network interface %s in resource group %s", nicSpec.Name, s.Scope.ClusterConfig.ResourceGroup)
	}

	err = future.WaitForCompletionRef(ctx, s.Client.Client)
	if err != nil {
		return errors.Wrap(err, "cannot create, future response")
	}

	_, err = future.Result(s.Client)
	if err != nil {
		return errors.Wrap(err, "result error")
	}
	klog.V(2).Infof("successfully created network interface %s", nicSpec.Name)
	return err
}

// Delete deletes the network interface with the provided name.
func (s *Service) Delete(ctx context.Context, spec v1alpha1.ResourceSpec) error {
	nicSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid network interface Specification")
	}
	klog.V(2).Infof("deleting nic %s", nicSpec.Name)
	future, err := s.Client.Delete(ctx, s.Scope.ClusterConfig.ResourceGroup, nicSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "failed to delete network interface %s in resource group %s", nicSpec.Name, s.Scope.ClusterConfig.ResourceGroup)
	}

	err = future.WaitForCompletionRef(ctx, s.Client.Client)
	if err != nil {
		return errors.Wrap(err, "cannot create, future response")
	}

	_, err = future.Result(s.Client)
	if err != nil {
		return errors.Wrap(err, "result error")
	}
	klog.V(2).Infof("successfully deleted nic %s", nicSpec.Name)
	return err
}

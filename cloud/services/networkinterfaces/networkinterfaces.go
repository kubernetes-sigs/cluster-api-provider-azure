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
	"fmt"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Spec specification for routetable
type Spec struct {
	Name                     string
	MachineRole              string
	SubnetName               string
	VnetName                 string
	StaticIPAddress          string
	PublicLoadBalancerName   string
	InternalLoadBalancerName string
	PublicIPName             string
	AcceleratedNetworking    *bool
}

// Reconcile gets/creates/updates a network interface.
func (s *Service) Reconcile(ctx context.Context, spec interface{}) error {
	nicSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid network interface specification")
	}

	nicConfig := &network.InterfaceIPConfigurationPropertiesFormat{}

	subnet, err := s.SubnetsClient.Get(ctx, s.Scope.Vnet().ResourceGroup, nicSpec.VnetName, nicSpec.SubnetName)
	if err != nil {
		return errors.Wrap(err, "failed to get subnets")
	}

	nicConfig.Subnet = &network.Subnet{ID: subnet.ID}
	nicConfig.PrivateIPAllocationMethod = network.Dynamic
	if nicSpec.StaticIPAddress != "" {
		nicConfig.PrivateIPAllocationMethod = network.Static
		nicConfig.PrivateIPAddress = to.StringPtr(nicSpec.StaticIPAddress)
	}

	backendAddressPools := []network.BackendAddressPool{}
	if nicSpec.PublicLoadBalancerName != "" {
		lb, lberr := s.PublicLoadBalancersClient.Get(ctx, s.Scope.ResourceGroup(), nicSpec.PublicLoadBalancerName)
		if lberr != nil {
			return errors.Wrap(lberr, "failed to get public LB")
		}

		backendAddressPools = append(backendAddressPools,
			network.BackendAddressPool{
				ID: (*lb.BackendAddressPools)[0].ID,
			})

		if nicSpec.MachineRole == infrav1.ControlPlane {
			ruleName := s.MachineScope.Name()
			naterr := s.createInboundNatRule(ctx, lb, ruleName)
			if naterr != nil {
				return errors.Wrap(naterr, "failed to create NAT rule")
			}

			nicConfig.LoadBalancerInboundNatRules = &[]network.InboundNatRule{
				{
					ID: to.StringPtr(fmt.Sprintf("%s/inboundNatRules/%s", to.String(lb.ID), ruleName)),
				},
			}
		}
	}
	if nicSpec.InternalLoadBalancerName != "" {
		// only control planes have an attached internal LB
		internalLB, ilberr := s.InternalLoadBalancersClient.Get(ctx, s.Scope.ResourceGroup(), nicSpec.InternalLoadBalancerName)
		if ilberr != nil {
			return errors.Wrap(ilberr, "failed to get internalLB")
		}

		backendAddressPools = append(backendAddressPools,
			network.BackendAddressPool{
				ID: (*internalLB.BackendAddressPools)[0].ID,
			})
	}
	nicConfig.LoadBalancerBackendAddressPools = &backendAddressPools

	if nicSpec.PublicIPName != "" {
		publicIP, err := s.PublicIPsClient.Get(ctx, s.Scope.ResourceGroup(), nicSpec.PublicIPName)
		if err != nil {
			return errors.Wrap(err, "failed to get publicIP")
		}
		nicConfig.PublicIPAddress = &publicIP
	}

	if nicSpec.AcceleratedNetworking == nil {
		// set accelerated networking to the capability of the VMSize
		sku := s.MachineScope.AzureMachine.Spec.VMSize
		accelNet, err := s.ResourceSkusClient.HasAcceleratedNetworking(ctx, sku)
		if err != nil {
			return errors.Wrap(err, "failed to get accelerated networking capability")
		}
		nicSpec.AcceleratedNetworking = to.BoolPtr(accelNet)
	}

	err = s.Client.CreateOrUpdate(ctx,
		s.Scope.ResourceGroup(),
		nicSpec.Name,
		network.Interface{
			Location: to.StringPtr(s.Scope.Location()),
			InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
				IPConfigurations: &[]network.InterfaceIPConfiguration{
					{
						Name:                                     to.StringPtr("pipConfig"),
						InterfaceIPConfigurationPropertiesFormat: nicConfig,
					},
				},
				EnableAcceleratedNetworking: nicSpec.AcceleratedNetworking,
			},
		})

	if err != nil {
		return errors.Wrapf(err, "failed to create network interface %s in resource group %s", nicSpec.Name, s.Scope.ResourceGroup())
	}

	klog.V(2).Infof("successfully created network interface %s", nicSpec.Name)
	return nil
}

// Delete deletes the network interface with the provided name.
func (s *Service) Delete(ctx context.Context, spec interface{}) error {
	nicSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid network interface specification")
	}
	klog.V(2).Infof("deleting nic %s", nicSpec.Name)
	err := s.Client.Delete(ctx, s.Scope.ResourceGroup(), nicSpec.Name)
	if err != nil && !azure.ResourceNotFound(err) {
		return errors.Wrapf(err, "failed to delete network interface %s in resource group %s", nicSpec.Name, s.Scope.ResourceGroup())
	}
	NATRuleName := s.MachineScope.Name()
	err = s.InboundNATRulesClient.Delete(ctx, s.Scope.ResourceGroup(), nicSpec.PublicLoadBalancerName, NATRuleName)
	if err != nil && !azure.ResourceNotFound(err) {
		return errors.Wrapf(err, "failed to delete inbound NAT rule %s in load balancer %s", NATRuleName, nicSpec.PublicLoadBalancerName)
	}
	klog.V(2).Infof("successfully deleted NIC %s and NAT rule %s", nicSpec.Name, NATRuleName)
	return nil
}

func (s *Service) createInboundNatRule(ctx context.Context, lb network.LoadBalancer, ruleName string) error {
	var sshFrontendPort int32 = 22
	ports := make(map[int32]struct{})
	if lb.LoadBalancerPropertiesFormat == nil || lb.InboundNatRules == nil {
		return errors.Errorf("Could not get existing inbound NAT rules from load balancer %s properties", to.String(lb.Name))
	}
	for _, v := range *lb.InboundNatRules {
		if to.String(v.Name) == ruleName {
			// Inbound NAT Rule already exists, nothing to do here.
			klog.Infof("NAT rule %s already exists", ruleName)
			return nil
		}
		ports[*v.InboundNatRulePropertiesFormat.FrontendPort] = struct{}{}
	}
	if _, ok := ports[22]; ok {
		var i int32
		found := false
		for i = 2201; i < 2220; i++ {
			if _, ok := ports[i]; !ok {
				sshFrontendPort = i
				found = true
				break
			}
		}
		if !found {
			return errors.Errorf("Failed to find available SSH Frontend port for NAT Rule in load balancer %s for AzureMachine %s", to.String(lb.Name), ruleName)
		}
	}
	rule := network.InboundNatRule{
		Name: to.StringPtr(ruleName),
		InboundNatRulePropertiesFormat: &network.InboundNatRulePropertiesFormat{
			BackendPort:          to.Int32Ptr(22),
			EnableFloatingIP:     to.BoolPtr(false),
			IdleTimeoutInMinutes: to.Int32Ptr(4),
			FrontendIPConfiguration: &network.SubResource{
				ID: (*lb.FrontendIPConfigurations)[0].ID,
			},
			Protocol:     network.TransportProtocolTCP,
			FrontendPort: &sshFrontendPort,
		},
	}
	klog.V(3).Infof("Creating rule %s using port %d", ruleName, sshFrontendPort)
	return s.InboundNATRulesClient.CreateOrUpdate(ctx, s.Scope.ResourceGroup(), to.String(lb.Name), ruleName, rule)
}

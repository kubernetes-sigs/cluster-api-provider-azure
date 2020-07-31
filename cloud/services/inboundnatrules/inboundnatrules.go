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

package inboundnatrules

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Reconcile gets/creates/updates an inbound NAT rule.
func (s *Service) Reconcile(ctx context.Context) error {
	for _, inboundNatSpec := range s.Scope.InboundNatSpecs() {
		s.Scope.V(2).Info("creating inbound NAT rule", "NAT rule", inboundNatSpec.Name)

		var sshFrontendPort int32 = 22
		ports := make(map[int32]struct{})

		lb, err := s.LoadBalancersClient.Get(ctx, s.Scope.ResourceGroup(), inboundNatSpec.LoadBalancerName)
		if err != nil {
			return errors.Wrapf(err, "failed to get Load Balancer %s", inboundNatSpec.LoadBalancerName)
		}

		if lb.LoadBalancerPropertiesFormat == nil || lb.FrontendIPConfigurations == nil || lb.InboundNatRules == nil {
			return errors.Errorf("Could not get existing inbound NAT rules from load balancer %s properties", to.String(lb.Name))
		}
		for _, v := range *lb.InboundNatRules {
			if to.String(v.Name) == inboundNatSpec.Name {
				// Inbound NAT Rule already exists, nothing to do here.
				s.Scope.V(2).Info("NAT rule already exists", "NAT rule", inboundNatSpec.Name)
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
				return errors.Errorf("Failed to find available SSH Frontend port for NAT Rule %s in load balancer %s", inboundNatSpec.Name, to.String(lb.Name))
			}
		}
		rule := network.InboundNatRule{
			Name: to.StringPtr(inboundNatSpec.Name),
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
		s.Scope.V(3).Info("Creating rule %s using port %d", "NAT rule", inboundNatSpec.Name, "port", sshFrontendPort)

		err = s.Client.CreateOrUpdate(ctx, s.Scope.ResourceGroup(), to.String(lb.Name), inboundNatSpec.Name, rule)
		if err != nil {
			return errors.Wrapf(err, "failed to create inbound NAT rule %s", inboundNatSpec.Name)
		}

		s.Scope.V(2).Info("successfully created inbound NAT rule", "NAT rule", inboundNatSpec.Name)
	}
	return nil
}

// Delete deletes the inbound NAT rule with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	for _, inboundNatSpec := range s.Scope.InboundNatSpecs() {
		s.Scope.V(2).Info("deleting inbound NAT rule", "NAT rule", inboundNatSpec.Name)
		err := s.Client.Delete(ctx, s.Scope.ResourceGroup(), inboundNatSpec.LoadBalancerName, inboundNatSpec.Name)
		if err != nil && !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete inbound NAT rule %s", inboundNatSpec.Name)
		}

		s.Scope.V(2).Info("successfully deleted inbound NAT rule", "NAT rule", inboundNatSpec.Name)
	}
	return nil
}

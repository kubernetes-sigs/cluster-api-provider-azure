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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/loadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// InboundNatScope defines the scope interface for an inbound NAT service.
type InboundNatScope interface {
	logr.Logger
	azure.ClusterDescriber
	InboundNatSpecs() []azure.InboundNatSpec
}

// Service provides operations on Azure resources.
type Service struct {
	Scope InboundNatScope
	client
	loadBalancersClient loadbalancers.Client
}

// New creates a new service.
func New(scope InboundNatScope) *Service {
	return &Service{
		Scope:               scope,
		client:              newClient(scope),
		loadBalancersClient: loadbalancers.NewClient(scope),
	}
}

// Reconcile gets/creates/updates an inbound NAT rule.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "inboundnatrules.Service.Reconcile")
	defer span.End()

	for _, inboundNatSpec := range s.Scope.InboundNatSpecs() {
		s.Scope.V(2).Info("creating inbound NAT rule", "NAT rule", inboundNatSpec.Name)

		lb, err := s.loadBalancersClient.Get(ctx, s.Scope.ResourceGroup(), inboundNatSpec.LoadBalancerName)
		if err != nil {
			return errors.Wrapf(err, "failed to get Load Balancer %s", inboundNatSpec.LoadBalancerName)
		}

		if lb.LoadBalancerPropertiesFormat == nil || lb.FrontendIPConfigurations == nil || lb.InboundNatRules == nil {
			return errors.Errorf("Could not get existing inbound NAT rules from load balancer %s properties", to.String(lb.Name))
		}

		ports := make(map[int32]struct{})
		if s.natRuleExists(ports)(*lb.InboundNatRules, inboundNatSpec.Name) {
			// Inbound NAT Rule already exists, nothing to do here.
			continue
		}

		sshFrontendPort, err := s.getAvailablePort(ports)
		if err != nil {
			return errors.Wrapf(err, "failed to find available SSH Frontend port for NAT Rule %s in load balancer %s", inboundNatSpec.Name, to.String(lb.Name))
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

		err = s.client.CreateOrUpdate(ctx, s.Scope.ResourceGroup(), to.String(lb.Name), inboundNatSpec.Name, rule)
		if err != nil {
			return errors.Wrapf(err, "failed to create inbound NAT rule %s", inboundNatSpec.Name)
		}

		s.Scope.V(2).Info("successfully created inbound NAT rule", "NAT rule", inboundNatSpec.Name)
	}
	return nil
}

// Delete deletes the inbound NAT rule with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "inboundnatrules.Service.Delete")
	defer span.End()

	for _, inboundNatSpec := range s.Scope.InboundNatSpecs() {
		s.Scope.V(2).Info("deleting inbound NAT rule", "NAT rule", inboundNatSpec.Name)
		err := s.client.Delete(ctx, s.Scope.ResourceGroup(), inboundNatSpec.LoadBalancerName, inboundNatSpec.Name)
		if err != nil && !azure.ResourceNotFound(err) {
			return errors.Wrapf(err, "failed to delete inbound NAT rule %s", inboundNatSpec.Name)
		}

		s.Scope.V(2).Info("successfully deleted inbound NAT rule", "NAT rule", inboundNatSpec.Name)
	}
	return nil
}

func (s *Service) natRuleExists(ports map[int32]struct{}) func([]network.InboundNatRule, string) bool {
	return func(rules []network.InboundNatRule, name string) bool {
		for _, v := range rules {
			if to.String(v.Name) == name {
				s.Scope.V(2).Info("NAT rule already exists", "NAT rule", name)
				return true
			}
			ports[*v.InboundNatRulePropertiesFormat.FrontendPort] = struct{}{}
		}
		return false
	}
}

func (s *Service) getAvailablePort(ports map[int32]struct{}) (int32, error) {
	var i int32 = 22
	if _, ok := ports[22]; ok {
		for i = 2201; i < 2220; i++ {
			if _, ok := ports[i]; !ok {
				s.Scope.V(2).Info("Found available port", "port", i)
				return i, nil
			}
		}
		return i, errors.Errorf("No available SSH Frontend ports")
	}
	s.Scope.V(2).Info("Found available port", "port", i)
	return i, nil
}

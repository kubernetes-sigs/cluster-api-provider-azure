/*
Copyright 2020 The Kubernetes Authors.

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

package loadbalancers

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualnetworks"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const (
	httpsProbe  = "HTTPSProbe"
	lbRuleHTTPS = "LBRuleHTTPS"
	outboundNAT = "OutboundNATAllProtocols"
)

// LBScope defines the scope interface for a load balancer service.
type LBScope interface {
	logr.Logger
	azure.ClusterDescriber
	azure.NetworkDescriber
	LBSpecs() []azure.LBSpec
}

// Service provides operations on azure resources
type Service struct {
	Scope LBScope
	Client
	virtualNetworksClient virtualnetworks.Client
}

// New creates a new service.
func New(scope LBScope) *Service {
	return &Service{
		Scope:                 scope,
		Client:                NewClient(scope),
		virtualNetworksClient: virtualnetworks.NewClient(scope),
	}
}

// Reconcile gets/creates/updates a load balancer.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "loadbalancers.Service.Reconcile")
	defer span.End()

	for _, lbSpec := range s.Scope.LBSpecs() {
		var (
			etag                *string
			frontendIDs         []network.SubResource
			frontendIPConfigs   = make([]network.FrontendIPConfiguration, 0)
			loadBalancingRules  = make([]network.LoadBalancingRule, 0)
			backendAddressPools = make([]network.BackendAddressPool, 0)
			outboundRules       = make([]network.OutboundRule, 0)
			probes              = make([]network.Probe, 0)
		)

		existingLB, err := s.Client.Get(ctx, s.Scope.ResourceGroup(), lbSpec.Name)
		switch {
		case err != nil && !azure.ResourceNotFound(err):
			return errors.Wrapf(err, "failed to get LB %s in %s", lbSpec.Name, s.Scope.ResourceGroup())
		case err == nil:
			// LB already exists
			s.Scope.V(2).Info("found existing load balancer, checking if updates are needed", "load balancer", lbSpec.Name)
			// We append the existing LB etag to the header to ensure we only apply the updates if the LB has not been modified.
			etag = existingLB.Etag
			update := false

			// merge existing LB properties with desired properties
			frontendIPConfigs = *existingLB.FrontendIPConfigurations
			wantedIPs, wantedFrontendIDs := s.getFrontendIPConfigs(lbSpec)
			for _, ip := range wantedIPs {
				if !ipExists(frontendIPConfigs, ip) {
					update = true
					frontendIPConfigs = append(frontendIPConfigs, ip)
				}
			}

			loadBalancingRules = *existingLB.LoadBalancingRules
			for _, rule := range s.getLoadBalancingRules(lbSpec, wantedFrontendIDs) {
				if !lbRuleExists(loadBalancingRules, rule) {
					update = true
					loadBalancingRules = append(loadBalancingRules, rule)
				}
			}

			backendAddressPools = *existingLB.BackendAddressPools
			for _, pool := range s.getBackendAddressPools(lbSpec) {
				if !poolExists(backendAddressPools, pool) {
					update = true
					backendAddressPools = append(backendAddressPools, pool)
				}
			}

			outboundRules = *existingLB.OutboundRules
			for _, rule := range s.getOutboundRules(lbSpec, wantedFrontendIDs) {
				if !outboundRuleExists(outboundRules, rule) {
					update = true
					outboundRules = append(outboundRules, rule)
				}
			}

			probes = *existingLB.Probes
			for _, probe := range s.getProbes(lbSpec) {
				if !probeExists(probes, probe) {
					update = true
					probes = append(probes, probe)
				}
			}

			if !update {
				// Skip update for LB as the required defaults are present
				s.Scope.V(2).Info("LB exists and no defaults are missing, skipping update", "load balancer", lbSpec.Name)
				continue
			}
		default:
			s.Scope.V(2).Info("creating load balancer", "load balancer", lbSpec.Name)
			frontendIPConfigs, frontendIDs = s.getFrontendIPConfigs(lbSpec)
			loadBalancingRules = s.getLoadBalancingRules(lbSpec, frontendIDs)
			backendAddressPools = s.getBackendAddressPools(lbSpec)
			outboundRules = s.getOutboundRules(lbSpec, frontendIDs)
			probes = s.getProbes(lbSpec)
		}

		lb := network.LoadBalancer{
			Etag:     etag,
			Sku:      &network.LoadBalancerSku{Name: converters.SKUtoSDK(lbSpec.SKU)},
			Location: to.StringPtr(s.Scope.Location()),
			Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
				ClusterName: s.Scope.ClusterName(),
				Lifecycle:   infrav1.ResourceLifecycleOwned,
				Role:        to.StringPtr(lbSpec.Role),
				Additional:  s.Scope.AdditionalTags(),
			})),
			LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
				FrontendIPConfigurations: &frontendIPConfigs,
				BackendAddressPools:      &backendAddressPools,
				OutboundRules:            &outboundRules,
				Probes:                   &probes,
				LoadBalancingRules:       &loadBalancingRules,
			},
		}

		err = s.Client.CreateOrUpdate(ctx, s.Scope.ResourceGroup(), lbSpec.Name, lb)

		if err != nil {
			return errors.Wrapf(err, "failed to create load balancer \"%s\"", lbSpec.Name)
		}

		s.Scope.V(2).Info("successfully created load balancer", "load balancer", lbSpec.Name)
	}
	return nil
}

// Delete deletes the public load balancer with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "loadbalancers.Service.Delete")
	defer span.End()

	for _, lbSpec := range s.Scope.LBSpecs() {
		s.Scope.V(2).Info("deleting load balancer", "load balancer", lbSpec.Name)
		err := s.Client.Delete(ctx, s.Scope.ResourceGroup(), lbSpec.Name)
		if err != nil && azure.ResourceNotFound(err) {
			// already deleted
			continue
		}
		if err != nil {
			return errors.Wrapf(err, "failed to delete load balancer %s in resource group %s", lbSpec.Name, s.Scope.ResourceGroup())
		}

		s.Scope.V(2).Info("deleted public load balancer", "load balancer", lbSpec.Name)
	}
	return nil
}

func (s *Service) getFrontendIPConfigs(lbSpec azure.LBSpec) ([]network.FrontendIPConfiguration, []network.SubResource) {
	frontendIPConfigurations := make([]network.FrontendIPConfiguration, 0)
	frontendIDs := make([]network.SubResource, 0)
	for _, ipConfig := range lbSpec.FrontendIPConfigs {
		var properties network.FrontendIPConfigurationPropertiesFormat
		if lbSpec.Type == infrav1.Internal {
			properties = network.FrontendIPConfigurationPropertiesFormat{
				PrivateIPAllocationMethod: network.Static,
				Subnet: &network.Subnet{
					ID: to.StringPtr(azure.SubnetID(s.Scope.SubscriptionID(), s.Scope.Vnet().ResourceGroup, s.Scope.Vnet().Name, lbSpec.SubnetName)),
				},
				PrivateIPAddress: to.StringPtr(ipConfig.PrivateIPAddress),
			}
		} else {
			properties = network.FrontendIPConfigurationPropertiesFormat{
				PublicIPAddress: &network.PublicIPAddress{
					ID: to.StringPtr(azure.PublicIPID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), ipConfig.PublicIP.Name)),
				},
			}
		}
		frontendIPConfigurations = append(frontendIPConfigurations, network.FrontendIPConfiguration{
			FrontendIPConfigurationPropertiesFormat: &properties,
			Name:                                    to.StringPtr(ipConfig.Name),
		})
		frontendIDs = append(frontendIDs, network.SubResource{
			ID: to.StringPtr(azure.FrontendIPConfigID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), lbSpec.Name, ipConfig.Name)),
		})
	}
	return frontendIPConfigurations, frontendIDs
}

func (s *Service) getOutboundRules(lbSpec azure.LBSpec, frontendIDs []network.SubResource) []network.OutboundRule {
	if lbSpec.Type == infrav1.Internal {
		return []network.OutboundRule{}
	}
	return []network.OutboundRule{
		{
			Name: to.StringPtr(outboundNAT),
			OutboundRulePropertiesFormat: &network.OutboundRulePropertiesFormat{
				Protocol:                 network.LoadBalancerOutboundRuleProtocolAll,
				IdleTimeoutInMinutes:     to.Int32Ptr(4),
				FrontendIPConfigurations: &frontendIDs,
				BackendAddressPool: &network.SubResource{
					ID: to.StringPtr(azure.AddressPoolID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), lbSpec.Name, lbSpec.BackendPoolName)),
				},
			},
		},
	}
}

func (s *Service) getLoadBalancingRules(lbSpec azure.LBSpec, frontendIDs []network.SubResource) []network.LoadBalancingRule {
	if lbSpec.Role == infrav1.APIServerRole {
		// We disable outbound SNAT explicitly in the HTTPS LB rule and enable TCP and UDP outbound NAT with an outbound rule.
		// For more information on Standard LB outbound connections see https://docs.microsoft.com/en-us/azure/load-balancer/load-balancer-outbound-connections.
		var frontendIPConfig network.SubResource
		if len(frontendIDs) != 0 {
			frontendIPConfig = frontendIDs[0]
		}
		return []network.LoadBalancingRule{
			{
				Name: to.StringPtr(lbRuleHTTPS),
				LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
					DisableOutboundSnat:     to.BoolPtr(true),
					Protocol:                network.TransportProtocolTCP,
					FrontendPort:            to.Int32Ptr(lbSpec.APIServerPort),
					BackendPort:             to.Int32Ptr(lbSpec.APIServerPort),
					IdleTimeoutInMinutes:    to.Int32Ptr(4),
					EnableFloatingIP:        to.BoolPtr(false),
					LoadDistribution:        network.LoadDistributionDefault,
					FrontendIPConfiguration: &frontendIPConfig,
					BackendAddressPool: &network.SubResource{
						ID: to.StringPtr(azure.AddressPoolID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), lbSpec.Name, lbSpec.BackendPoolName)),
					},
					Probe: &network.SubResource{
						ID: to.StringPtr(azure.ProbeID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), lbSpec.Name, httpsProbe)),
					},
				},
			},
		}
	}
	return []network.LoadBalancingRule{}
}

func (s *Service) getBackendAddressPools(lbSpec azure.LBSpec) []network.BackendAddressPool {
	return []network.BackendAddressPool{
		{
			Name: to.StringPtr(lbSpec.BackendPoolName),
		},
	}
}

func (s *Service) getProbes(lbSpec azure.LBSpec) []network.Probe {
	if lbSpec.Role == infrav1.APIServerRole {
		return []network.Probe{
			{
				Name: to.StringPtr(httpsProbe),
				ProbePropertiesFormat: &network.ProbePropertiesFormat{
					Protocol:          network.ProbeProtocolHTTPS,
					RequestPath:       to.StringPtr("/healthz"),
					Port:              to.Int32Ptr(lbSpec.APIServerPort),
					IntervalInSeconds: to.Int32Ptr(15),
					NumberOfProbes:    to.Int32Ptr(4),
				},
			},
		}
	}
	return []network.Probe{}
}

func probeExists(probes []network.Probe, probe network.Probe) bool {
	for _, p := range probes {
		if to.String(p.Name) == to.String(probe.Name) {
			return true
		}
	}
	return false
}

func outboundRuleExists(rules []network.OutboundRule, rule network.OutboundRule) bool {
	for _, r := range rules {
		if to.String(r.Name) == to.String(rule.Name) {
			return true
		}
	}
	return false
}

func poolExists(pools []network.BackendAddressPool, pool network.BackendAddressPool) bool {
	for _, p := range pools {
		if to.String(p.Name) == to.String(pool.Name) {
			return true
		}
	}
	return false
}

func lbRuleExists(rules []network.LoadBalancingRule, rule network.LoadBalancingRule) bool {
	for _, r := range rules {
		if to.String(r.Name) == to.String(rule.Name) {
			return true
		}
	}
	return false
}

func ipExists(configs []network.FrontendIPConfiguration, config network.FrontendIPConfiguration) bool {
	for _, ip := range configs {
		if to.String(ip.Name) == to.String(config.Name) {
			return true
		}
	}
	return false
}

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
	"github.com/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Reconcile gets/creates/updates a load balancer.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "loadbalancers.Service.Reconcile")
	defer span.End()

	for _, lbSpec := range s.Scope.LBSpecs() {
		s.Scope.V(2).Info("creating load balancer", "load balancer", lbSpec.Name)

		frontendIPConfigs, frontendIDs, err := s.getFrontendIPConfigs(lbSpec)
		if err != nil {
			return err
		}

		lb := network.LoadBalancer{
			Name:     to.StringPtr(lbSpec.Name),
			Sku:      &network.LoadBalancerSku{Name: network.LoadBalancerSkuNameStandard},
			Location: to.StringPtr(s.Scope.Location()),
			Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
				ClusterName: s.Scope.ClusterName(),
				Lifecycle:   infrav1.ResourceLifecycleOwned,
				Role:        to.StringPtr(lbSpec.Role),
				Additional:  s.Scope.AdditionalTags(),
			})),
			LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
				FrontendIPConfigurations: &frontendIPConfigs,
				BackendAddressPools: &[]network.BackendAddressPool{
					{
						Name: to.StringPtr(lbSpec.BackendPoolName),
					},
				},
				OutboundRules: &[]network.OutboundRule{
					{
						Name: to.StringPtr("OutboundNATAllProtocols"),
						OutboundRulePropertiesFormat: &network.OutboundRulePropertiesFormat{
							Protocol:                 network.LoadBalancerOutboundRuleProtocolAll,
							IdleTimeoutInMinutes:     to.Int32Ptr(4),
							FrontendIPConfigurations: &frontendIDs,
							BackendAddressPool: &network.SubResource{
								ID: to.StringPtr(azure.AddressPoolID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), lbSpec.Name, lbSpec.BackendPoolName)),
							},
						},
					},
				},
			},
		}

		if lbSpec.Role == infrav1.APIServerRole {
			probeName := "HTTPSProbe"
			lb.LoadBalancerPropertiesFormat.Probes = &[]network.Probe{
				{
					Name: to.StringPtr(probeName),
					ProbePropertiesFormat: &network.ProbePropertiesFormat{
						Protocol:          network.ProbeProtocolHTTPS,
						RequestPath:       to.StringPtr("/healthz"),
						Port:              to.Int32Ptr(lbSpec.APIServerPort),
						IntervalInSeconds: to.Int32Ptr(15),
						NumberOfProbes:    to.Int32Ptr(4),
					},
				},
			}
			// We disable outbound SNAT explicitly in the HTTPS LB rule and enable TCP and UDP outbound NAT with an outbound rule.
			// For more information on Standard LB outbound connections see https://docs.microsoft.com/en-us/azure/load-balancer/load-balancer-outbound-connections.
			var frontendIPConfig network.SubResource
			if len(frontendIDs) != 0 {
				frontendIPConfig = frontendIDs[0]
			}
			lb.LoadBalancerPropertiesFormat.LoadBalancingRules = &[]network.LoadBalancingRule{
				{
					Name: to.StringPtr("LBRuleHTTPS"),
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
							ID: to.StringPtr(azure.ProbeID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), lbSpec.Name, probeName)),
						},
					},
				},
			}
			if lbSpec.Type == infrav1.Internal {
				lb.LoadBalancerPropertiesFormat.OutboundRules = nil
			}
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

func (s *Service) getFrontendIPConfigs(lbSpec azure.LBSpec) ([]network.FrontendIPConfiguration, []network.SubResource, error) {
	ctx, span := tele.Tracer().Start(ctx, "loadbalancers.Service.getFrontendIPConfigs")
	defer span.End()

	var frontendIPConfigurations []network.FrontendIPConfiguration
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
	return frontendIPConfigurations, frontendIDs, nil
}

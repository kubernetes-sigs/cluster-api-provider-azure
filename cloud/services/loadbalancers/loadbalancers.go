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
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
)

// Reconcile gets/creates/updates a load balancer.
func (s *Service) Reconcile(ctx context.Context) error {
	for _, lbSpec := range s.Scope.LBSpecs() {
		frontEndIPConfigName := azure.GenerateFrontendIPConfigName(lbSpec.Name)
		backEndAddressPoolName := azure.GenerateBackendAddressPoolName(lbSpec.Name)
		if lbSpec.Role == infrav1.NodeOutboundRole {
			backEndAddressPoolName = azure.GenerateOutboundBackendddressPoolName(lbSpec.Name)
		}

		s.Scope.V(2).Info("creating load balancer", "load balancer", lbSpec.Name)

		var frontIPConfig network.FrontendIPConfigurationPropertiesFormat
		if lbSpec.Role == infrav1.InternalRole {
			var privateIP string
			internalLB, err := s.Client.Get(ctx, s.Scope.ResourceGroup(), lbSpec.Name)
			if err == nil {
				ipConfigs := internalLB.LoadBalancerPropertiesFormat.FrontendIPConfigurations
				if ipConfigs != nil && len(*ipConfigs) > 0 {
					privateIP = to.String((*ipConfigs)[0].FrontendIPConfigurationPropertiesFormat.PrivateIPAddress)
				}
			} else if azure.ResourceNotFound(err) {
				s.Scope.V(2).Info("internalLB not found in RG", "internal lb", lbSpec.Name, "resource group", s.Scope.ResourceGroup())
				privateIP, err = s.getAvailablePrivateIP(ctx, s.Scope.Vnet().ResourceGroup, s.Scope.Vnet().Name, lbSpec.PrivateIPAddress, lbSpec.SubnetCidrs)
				if err != nil {
					return err
				}
				s.Scope.V(2).Info("setting internal load balancer IP", "private ip", privateIP)
			} else {
				return errors.Wrap(err, "failed to look for existing internal LB")
			}
			frontIPConfig = network.FrontendIPConfigurationPropertiesFormat{
				PrivateIPAllocationMethod: network.Static,
				Subnet: &network.Subnet{
					ID: to.StringPtr(azure.SubnetID(s.Scope.SubscriptionID(), s.Scope.Vnet().ResourceGroup, s.Scope.Vnet().Name, lbSpec.SubnetName)),
				},
				PrivateIPAddress: to.StringPtr(privateIP),
			}
		} else {
			frontIPConfig = network.FrontendIPConfigurationPropertiesFormat{
				PrivateIPAllocationMethod: network.Dynamic,
				PublicIPAddress: &network.PublicIPAddress{
					ID: to.StringPtr(azure.PublicIPID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), lbSpec.PublicIPName)),
				},
			}
		}

		lb := network.LoadBalancer{
			Sku:      &network.LoadBalancerSku{Name: network.LoadBalancerSkuNameStandard},
			Location: to.StringPtr(s.Scope.Location()),
			Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
				ClusterName: s.Scope.ClusterName(),
				Lifecycle:   infrav1.ResourceLifecycleOwned,
				Role:        to.StringPtr(lbSpec.Role),
				Additional:  s.Scope.AdditionalTags(),
			})),
			LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
				FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
					{
						Name:                                    &frontEndIPConfigName,
						FrontendIPConfigurationPropertiesFormat: &frontIPConfig,
					},
				},
				BackendAddressPools: &[]network.BackendAddressPool{
					{
						Name: &backEndAddressPoolName,
					},
				},
				OutboundRules: &[]network.OutboundRule{
					{
						Name: to.StringPtr("OutboundNATAllProtocols"),
						OutboundRulePropertiesFormat: &network.OutboundRulePropertiesFormat{
							Protocol:             network.LoadBalancerOutboundRuleProtocolAll,
							IdleTimeoutInMinutes: to.Int32Ptr(4),
							FrontendIPConfigurations: &[]network.SubResource{
								{
									ID: to.StringPtr(azure.FrontendIPConfigID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), lbSpec.Name, frontEndIPConfigName)),
								},
							},
							BackendAddressPool: &network.SubResource{
								ID: to.StringPtr(azure.AddressPoolID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), lbSpec.Name, backEndAddressPoolName)),
							},
						},
					},
				},
			},
		}

		if lbSpec.Role == infrav1.APIServerRole || lbSpec.Role == infrav1.InternalRole {
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
			lbRule := network.LoadBalancingRule{
				Name: to.StringPtr("LBRuleHTTPS"),
				LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
					Protocol:             network.TransportProtocolTCP,
					FrontendPort:         to.Int32Ptr(lbSpec.APIServerPort),
					BackendPort:          to.Int32Ptr(lbSpec.APIServerPort),
					IdleTimeoutInMinutes: to.Int32Ptr(4),
					EnableFloatingIP:     to.BoolPtr(false),
					LoadDistribution:     network.LoadDistributionDefault,
					FrontendIPConfiguration: &network.SubResource{
						ID: to.StringPtr(azure.FrontendIPConfigID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), lbSpec.Name, frontEndIPConfigName)),
					},
					BackendAddressPool: &network.SubResource{
						ID: to.StringPtr(azure.AddressPoolID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), lbSpec.Name, backEndAddressPoolName)),
					},
					Probe: &network.SubResource{
						ID: to.StringPtr(azure.ProbeID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), lbSpec.Name, probeName)),
					},
				},
			}

			if lbSpec.Role == infrav1.APIServerRole {
				// We disable outbound SNAT explicitly in the HTTPS LB rule and enable TCP and UDP outbound NAT with an outbound rule.
				// For more information on Standard LB outbound connections see https://docs.microsoft.com/en-us/azure/load-balancer/load-balancer-outbound-connections.
				lbRule.LoadBalancingRulePropertiesFormat.DisableOutboundSnat = to.BoolPtr(true)
			} else if lbSpec.Role == infrav1.InternalRole {
				lb.LoadBalancerPropertiesFormat.OutboundRules = nil
			}
			lb.LoadBalancerPropertiesFormat.LoadBalancingRules = &[]network.LoadBalancingRule{lbRule}
		}

		err := s.Client.CreateOrUpdate(ctx, s.Scope.ResourceGroup(), lbSpec.Name, lb)

		if err != nil {
			return errors.Wrapf(err, "failed to create load balancer %s", lbSpec.Name)
		}

		s.Scope.V(2).Info("successfully created load balancer", "load balancer", lbSpec.Name)
	}
	return nil
}

// Delete deletes the public load balancer with the provided name.
func (s *Service) Delete(ctx context.Context) error {
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

// getAvailablePrivateIP checks if the desired private IP address is available in a virtual network.
// If the IP address is taken or empty, it will make an attempt to find an available IP in the same subnet
// NOTE: this does not work for VNets with ipv6 CIDRs currently
func (s *Service) getAvailablePrivateIP(ctx context.Context, resourceGroup, vnetName, PreferredIPAddress string, subnetCIDRs []string) (string, error) {
	if len(subnetCIDRs) == 0 {
		return "", errors.Errorf("failed to find available IP: control plane subnet CIDRs should not be empty")
	}
	ip := PreferredIPAddress
	if ip == "" {
		ip = azure.DefaultInternalLBIPAddress
		subnetCIDR := subnetCIDRs[0]
		if subnetCIDR != infrav1.DefaultControlPlaneSubnetCIDR {
			// If the user provided a custom subnet CIDR without providing a private IP, try finding an available IP in the subnet space
			index := strings.LastIndex(subnetCIDR, ".")
			ip = subnetCIDR[0:(index+1)] + "0"
		}
	}

	result, err := s.VirtualNetworksClient.CheckIPAddressAvailability(ctx, resourceGroup, vnetName, ip)
	if err != nil {
		return "", errors.Wrap(err, "failed to check IP availability")
	}
	if !to.Bool(result.Available) {
		if len(to.StringSlice(result.AvailableIPAddresses)) == 0 {
			return "", errors.Errorf("IP %s is not available in VNet %s and there were no other available IPs found", ip, vnetName)
		}
		// TODO: make sure that the returned IP is in the right subnet since this check is done at the VNet level
		ip = to.StringSlice(result.AvailableIPAddresses)[0]
	}
	return ip, nil
}

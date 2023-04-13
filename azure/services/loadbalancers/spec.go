/*
Copyright 2021 The Kubernetes Authors.

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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"github.com/pkg/errors"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
)

// LBSpec defines the specification for a Load Balancer.
type LBSpec struct {
	Name                  string
	ResourceGroup         string
	SubscriptionID        string
	ClusterName           string
	Location              string
	ExtendedLocation      *infrav1.ExtendedLocationSpec
	Role                  string
	Type                  infrav1.LBType
	SKU                   infrav1.SKU
	VNetName              string
	VNetResourceGroup     string
	SubnetName            string
	BackendPoolName       string
	FrontendIPConfigs     []infrav1.FrontendIP
	APIServerFrontendPort int32
	APIServerBackendPort  int32
	IdleTimeoutInMinutes  *int32
	AdditionalTags        map[string]string
}

// ResourceName returns the name of the load balancer.
func (s *LBSpec) ResourceName() string {
	return s.Name
}

// ResourceGroupName returns the name of the resource group.
func (s *LBSpec) ResourceGroupName() string {
	return s.ResourceGroup
}

// OwnerResourceName is a no-op for load balancers.
func (s *LBSpec) OwnerResourceName() string {
	return ""
}

// Parameters returns the parameters for the load balancer.
func (s *LBSpec) Parameters(ctx context.Context, existing interface{}) (parameters interface{}, err error) {
	var (
		etag                *string
		frontendIDs         []network.SubResource
		frontendIPConfigs   = make([]network.FrontendIPConfiguration, 0)
		loadBalancingRules  = make([]network.LoadBalancingRule, 0)
		backendAddressPools = make([]network.BackendAddressPool, 0)
		outboundRules       = make([]network.OutboundRule, 0)
		probes              = make([]network.Probe, 0)
	)

	if existing != nil {
		existingLB, ok := existing.(network.LoadBalancer)
		if !ok {
			return nil, errors.Errorf("%T is not a network.LoadBalancer", existing)
		}
		// LB already exists
		// We append the existing LB etag to the header to ensure we only apply the updates if the LB has not been modified.
		etag = existingLB.Etag
		update := false

		// merge existing LB properties with desired properties
		frontendIPConfigs = *existingLB.FrontendIPConfigurations
		wantedIPs, wantedFrontendIDs := getFrontendIPConfigs(*s)
		for _, ip := range wantedIPs {
			if !ipExists(frontendIPConfigs, ip) {
				update = true
				frontendIPConfigs = append(frontendIPConfigs, ip)
			}
		}

		loadBalancingRules = *existingLB.LoadBalancingRules
		for _, rule := range getLoadBalancingRules(*s, wantedFrontendIDs) {
			if !lbRuleExists(loadBalancingRules, rule) {
				update = true
				loadBalancingRules = append(loadBalancingRules, rule)
			}
		}

		backendAddressPools = *existingLB.BackendAddressPools
		for _, pool := range getBackendAddressPools(*s) {
			if !poolExists(backendAddressPools, pool) {
				update = true
				backendAddressPools = append(backendAddressPools, pool)
			}
		}

		outboundRules = *existingLB.OutboundRules
		for _, rule := range getOutboundRules(*s, wantedFrontendIDs) {
			if !outboundRuleExists(outboundRules, rule) {
				update = true
				outboundRules = append(outboundRules, rule)
			}
		}

		probes = *existingLB.Probes
		for _, probe := range getProbes(*s) {
			if !probeExists(probes, probe) {
				update = true
				probes = append(probes, probe)
			}
		}

		if !update {
			// load balancer already exists with all required defaults
			return nil, nil
		}
	} else {
		frontendIPConfigs, frontendIDs = getFrontendIPConfigs(*s)
		loadBalancingRules = getLoadBalancingRules(*s, frontendIDs)
		backendAddressPools = getBackendAddressPools(*s)
		outboundRules = getOutboundRules(*s, frontendIDs)
		probes = getProbes(*s)
	}

	lb := network.LoadBalancer{
		Etag:             etag,
		Sku:              &network.LoadBalancerSku{Name: converters.SKUtoSDK(s.SKU)},
		Location:         pointer.String(s.Location),
		ExtendedLocation: converters.ExtendedLocationToNetworkSDK(s.ExtendedLocation),
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: s.ClusterName,
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Role:        pointer.String(s.Role),
			Additional:  s.AdditionalTags,
		})),
		LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
			FrontendIPConfigurations: &frontendIPConfigs,
			BackendAddressPools:      &backendAddressPools,
			OutboundRules:            &outboundRules,
			Probes:                   &probes,
			LoadBalancingRules:       &loadBalancingRules,
		},
	}

	return lb, nil
}

func getFrontendIPConfigs(lbSpec LBSpec) ([]network.FrontendIPConfiguration, []network.SubResource) {
	frontendIPConfigurations := make([]network.FrontendIPConfiguration, 0)
	frontendIDs := make([]network.SubResource, 0)
	for _, ipConfig := range lbSpec.FrontendIPConfigs {
		var properties network.FrontendIPConfigurationPropertiesFormat
		if lbSpec.Type == infrav1.Internal {
			properties = network.FrontendIPConfigurationPropertiesFormat{
				PrivateIPAllocationMethod: network.IPAllocationMethodStatic,
				Subnet: &network.Subnet{
					ID: pointer.String(azure.SubnetID(lbSpec.SubscriptionID, lbSpec.VNetResourceGroup, lbSpec.VNetName, lbSpec.SubnetName)),
				},
				PrivateIPAddress: pointer.String(ipConfig.PrivateIPAddress),
			}
		} else {
			properties = network.FrontendIPConfigurationPropertiesFormat{
				PublicIPAddress: &network.PublicIPAddress{
					ID: pointer.String(azure.PublicIPID(lbSpec.SubscriptionID, lbSpec.ResourceGroup, ipConfig.PublicIP.Name)),
				},
			}
		}
		frontendIPConfigurations = append(frontendIPConfigurations, network.FrontendIPConfiguration{
			FrontendIPConfigurationPropertiesFormat: &properties,
			Name:                                    pointer.String(ipConfig.Name),
		})
		frontendIDs = append(frontendIDs, network.SubResource{
			ID: pointer.String(azure.FrontendIPConfigID(lbSpec.SubscriptionID, lbSpec.ResourceGroup, lbSpec.Name, ipConfig.Name)),
		})
	}
	return frontendIPConfigurations, frontendIDs
}

func getOutboundRules(lbSpec LBSpec, frontendIDs []network.SubResource) []network.OutboundRule {
	if lbSpec.Type == infrav1.Internal {
		return []network.OutboundRule{}
	}
	return []network.OutboundRule{
		{
			Name: pointer.String(outboundNAT),
			OutboundRulePropertiesFormat: &network.OutboundRulePropertiesFormat{
				Protocol:                 network.LoadBalancerOutboundRuleProtocolAll,
				IdleTimeoutInMinutes:     lbSpec.IdleTimeoutInMinutes,
				FrontendIPConfigurations: &frontendIDs,
				BackendAddressPool: &network.SubResource{
					ID: pointer.String(azure.AddressPoolID(lbSpec.SubscriptionID, lbSpec.ResourceGroup, lbSpec.Name, lbSpec.BackendPoolName)),
				},
			},
		},
	}
}

func getLoadBalancingRules(lbSpec LBSpec, frontendIDs []network.SubResource) []network.LoadBalancingRule {
	if lbSpec.Role == infrav1.APIServerRole {
		// We disable outbound SNAT explicitly in the HTTPS LB rule and enable TCP and UDP outbound NAT with an outbound rule.
		// For more information on Standard LB outbound connections see https://docs.microsoft.com/en-us/azure/load-balancer/load-balancer-outbound-connections.
		var frontendIPConfig network.SubResource
		if len(frontendIDs) != 0 {
			frontendIPConfig = frontendIDs[0]
		}
		return []network.LoadBalancingRule{
			{
				Name: pointer.String(lbRuleHTTPS),
				LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
					DisableOutboundSnat:     pointer.Bool(true),
					Protocol:                network.TransportProtocolTCP,
					FrontendPort:            pointer.Int32(lbSpec.APIServerFrontendPort),
					BackendPort:             pointer.Int32(lbSpec.APIServerBackendPort),
					IdleTimeoutInMinutes:    lbSpec.IdleTimeoutInMinutes,
					EnableFloatingIP:        pointer.Bool(false),
					LoadDistribution:        network.LoadDistributionDefault,
					FrontendIPConfiguration: &frontendIPConfig,
					BackendAddressPool: &network.SubResource{
						ID: pointer.String(azure.AddressPoolID(lbSpec.SubscriptionID, lbSpec.ResourceGroup, lbSpec.Name, lbSpec.BackendPoolName)),
					},
					Probe: &network.SubResource{
						ID: pointer.String(azure.ProbeID(lbSpec.SubscriptionID, lbSpec.ResourceGroup, lbSpec.Name, tcpProbe)),
					},
				},
			},
		}
	}
	return []network.LoadBalancingRule{}
}

func getBackendAddressPools(lbSpec LBSpec) []network.BackendAddressPool {
	return []network.BackendAddressPool{
		{
			Name: pointer.String(lbSpec.BackendPoolName),
		},
	}
}

func getProbes(lbSpec LBSpec) []network.Probe {
	if lbSpec.Role == infrav1.APIServerRole {
		return []network.Probe{
			{
				Name: pointer.String(tcpProbe),
				ProbePropertiesFormat: &network.ProbePropertiesFormat{
					Protocol:          network.ProbeProtocolTCP,
					Port:              pointer.Int32(lbSpec.APIServerFrontendPort),
					IntervalInSeconds: pointer.Int32(15),
					NumberOfProbes:    pointer.Int32(4),
				},
			},
		}
	}
	return []network.Probe{}
}

func probeExists(probes []network.Probe, probe network.Probe) bool {
	for _, p := range probes {
		if pointer.StringDeref(p.Name, "") == pointer.StringDeref(probe.Name, "") {
			return true
		}
	}
	return false
}

func outboundRuleExists(rules []network.OutboundRule, rule network.OutboundRule) bool {
	for _, r := range rules {
		if pointer.StringDeref(r.Name, "") == pointer.StringDeref(rule.Name, "") {
			return true
		}
	}
	return false
}

func poolExists(pools []network.BackendAddressPool, pool network.BackendAddressPool) bool {
	for _, p := range pools {
		if pointer.StringDeref(p.Name, "") == pointer.StringDeref(pool.Name, "") {
			return true
		}
	}
	return false
}

func lbRuleExists(rules []network.LoadBalancingRule, rule network.LoadBalancingRule) bool {
	for _, r := range rules {
		if pointer.StringDeref(r.Name, "") == pointer.StringDeref(rule.Name, "") {
			return true
		}
	}
	return false
}

func ipExists(configs []network.FrontendIPConfiguration, config network.FrontendIPConfiguration) bool {
	for _, ip := range configs {
		if pointer.StringDeref(ip.Name, "") == pointer.StringDeref(config.Name, "") {
			return true
		}
	}
	return false
}

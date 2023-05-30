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
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func getExistingLBWithMissingFrontendIPConfigs() network.LoadBalancer {
	existingLB := newSamplePublicAPIServerLB(false, true, true, true, true)
	existingLB.FrontendIPConfigurations = &[]network.FrontendIPConfiguration{}

	return existingLB
}

func getExistingLBWithMissingBackendPool() network.LoadBalancer {
	existingLB := newSamplePublicAPIServerLB(true, false, true, true, true)
	existingLB.BackendAddressPools = &[]network.BackendAddressPool{}

	return existingLB
}

func getExistingLBWithMissingLBRules() network.LoadBalancer {
	existingLB := newSamplePublicAPIServerLB(true, true, false, true, true)
	existingLB.LoadBalancingRules = &[]network.LoadBalancingRule{}

	return existingLB
}

func getExistingLBWithMissingProbes() network.LoadBalancer {
	existingLB := newSamplePublicAPIServerLB(true, true, true, false, true)
	existingLB.Probes = &[]network.Probe{}

	return existingLB
}

func getExistingLBWithMissingOutboundRules() network.LoadBalancer {
	existingLB := newSamplePublicAPIServerLB(true, true, true, true, false)
	existingLB.OutboundRules = &[]network.OutboundRule{}

	return existingLB
}

func TestParameters(t *testing.T) {
	testcases := []struct {
		name          string
		spec          *LBSpec
		existing      interface{}
		expect        func(g *WithT, result interface{})
		expectedError string
	}{
		{
			name:     "public API load balancer exists with all expected values",
			spec:     &fakePublicAPILBSpec,
			existing: newSamplePublicAPIServerLB(false, false, false, false, false),
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
		{
			name:     "internal API load balancer with all expected values",
			spec:     &fakeInternalAPILBSpec,
			existing: newDefaultInternalAPIServerLB(),
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
		{
			name:     "node outbound load balancer exists with all expected values",
			spec:     &fakeNodeOutboundLBSpec,
			existing: newDefaultNodeOutboundLB(),
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
		{
			name:     "load balancer exists with missing frontend IP configs",
			spec:     &fakePublicAPILBSpec,
			existing: getExistingLBWithMissingFrontendIPConfigs(),
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(network.LoadBalancer{}))
				g.Expect(result.(network.LoadBalancer)).To(Equal(newSamplePublicAPIServerLB(false, true, true, true, true)))
			},
			expectedError: "",
		},
		{
			name:     "load balancer exists with missing backend pool",
			spec:     &fakePublicAPILBSpec,
			existing: getExistingLBWithMissingBackendPool(),
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(network.LoadBalancer{}))
				g.Expect(result.(network.LoadBalancer)).To(Equal(newSamplePublicAPIServerLB(true, false, true, true, true)))
			},
			expectedError: "",
		},
		{
			name:     "load balancer exists with missing load balancing rules",
			spec:     &fakePublicAPILBSpec,
			existing: getExistingLBWithMissingLBRules(),
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(network.LoadBalancer{}))
				g.Expect(result.(network.LoadBalancer)).To(Equal(newSamplePublicAPIServerLB(true, true, false, true, true)))
			},
			expectedError: "",
		},
		{
			name:     "load balancer exists with missing probes",
			spec:     &fakePublicAPILBSpec,
			existing: getExistingLBWithMissingProbes(),
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(network.LoadBalancer{}))
				g.Expect(result.(network.LoadBalancer)).To(Equal(newSamplePublicAPIServerLB(true, true, true, false, true)))
			},
			expectedError: "",
		},
		{
			name:     "load balancer exists with missing outbound rules",
			spec:     &fakePublicAPILBSpec,
			existing: getExistingLBWithMissingOutboundRules(),
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(network.LoadBalancer{}))
				g.Expect(result.(network.LoadBalancer)).To(Equal(newSamplePublicAPIServerLB(true, true, true, true, false)))
			},
			expectedError: "",
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(context.TODO(), tc.existing)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			tc.expect(g, result)
		})
	}
}

func newDefaultNodeOutboundLB() network.LoadBalancer {
	return network.LoadBalancer{
		Tags: map[string]*string{
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": pointer.String("owned"),
			"sigs.k8s.io_cluster-api-provider-azure_role":               pointer.String(infrav1.NodeOutboundRole),
		},
		Sku:      &network.LoadBalancerSku{Name: network.LoadBalancerSkuNameStandard},
		Location: pointer.String("my-location"),
		LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
			FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
				{
					Name: pointer.String("my-cluster-frontEnd"),
					FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
						PublicIPAddress: &network.PublicIPAddress{ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/publicIPAddresses/outbound-publicip")},
					},
				},
			},
			BackendAddressPools: &[]network.BackendAddressPool{
				{
					Name: pointer.String("my-cluster-outboundBackendPool"),
				},
			},
			LoadBalancingRules: &[]network.LoadBalancingRule{},
			Probes:             &[]network.Probe{},
			OutboundRules: &[]network.OutboundRule{
				{
					Name: pointer.String("OutboundNATAllProtocols"),
					OutboundRulePropertiesFormat: &network.OutboundRulePropertiesFormat{
						FrontendIPConfigurations: &[]network.SubResource{
							{ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-cluster/frontendIPConfigurations/my-cluster-frontEnd")},
						},
						BackendAddressPool: &network.SubResource{
							ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-cluster/backendAddressPools/my-cluster-outboundBackendPool"),
						},
						Protocol:             network.LoadBalancerOutboundRuleProtocolAll,
						IdleTimeoutInMinutes: pointer.Int32(30),
					},
				},
			},
		},
	}
}

func newSamplePublicAPIServerLB(verifyFrontendIP bool, verifyBackendAddressPools bool, verifyLBRules bool, verifyProbes bool, verifyOutboundRules bool) network.LoadBalancer {
	var subnet *network.Subnet
	var backendAddressPoolProps *network.BackendAddressPoolPropertiesFormat
	enableFloatingIP := pointer.Bool(false)
	numProbes := pointer.Int32(4)
	idleTimeout := pointer.Int32(4)

	if verifyFrontendIP {
		subnet = &network.Subnet{
			Name: pointer.String("fake-test-subnet"),
		}
	}
	if verifyBackendAddressPools {
		backendAddressPoolProps = &network.BackendAddressPoolPropertiesFormat{
			Location: pointer.String("fake-test-location"),
		}
	}
	if verifyLBRules {
		enableFloatingIP = pointer.Bool(true)
	}
	if verifyProbes {
		numProbes = pointer.Int32(999)
	}
	if verifyOutboundRules {
		idleTimeout = pointer.Int32(1000)
	}

	return network.LoadBalancer{
		Tags: map[string]*string{
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": pointer.String("owned"),
			"sigs.k8s.io_cluster-api-provider-azure_role":               pointer.String(infrav1.APIServerRole),
		},
		Sku:      &network.LoadBalancerSku{Name: network.LoadBalancerSkuNameStandard},
		Location: pointer.String("my-location"),
		LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
			FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
				{
					Name: pointer.String("my-publiclb-frontEnd"),
					FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
						PublicIPAddress: &network.PublicIPAddress{ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/publicIPAddresses/my-publicip")},
						Subnet:          subnet, // Add to verify that FrontendIPConfigurations aren't overwritten on update
					},
				},
			},
			BackendAddressPools: &[]network.BackendAddressPool{
				{
					Name:                               pointer.String("my-publiclb-backendPool"),
					BackendAddressPoolPropertiesFormat: backendAddressPoolProps, // Add to verify that BackendAddressPools aren't overwritten on update
				},
			},
			LoadBalancingRules: &[]network.LoadBalancingRule{
				{
					Name: pointer.String(lbRuleHTTPS),
					LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
						DisableOutboundSnat:  pointer.Bool(true),
						Protocol:             network.TransportProtocolTCP,
						FrontendPort:         pointer.Int32(6443),
						BackendPort:          pointer.Int32(6443),
						IdleTimeoutInMinutes: pointer.Int32(4),
						EnableFloatingIP:     enableFloatingIP, // Add to verify that LoadBalancingRules aren't overwritten on update
						LoadDistribution:     network.LoadDistributionDefault,
						FrontendIPConfiguration: &network.SubResource{
							ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/frontendIPConfigurations/my-publiclb-frontEnd"),
						},
						BackendAddressPool: &network.SubResource{
							ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/backendAddressPools/my-publiclb-backendPool"),
						},
						Probe: &network.SubResource{
							ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/probes/HTTPSProbe"),
						},
					},
				},
			},
			Probes: &[]network.Probe{
				{
					Name: pointer.String(httpsProbe),
					ProbePropertiesFormat: &network.ProbePropertiesFormat{
						Protocol:          network.ProbeProtocolHTTPS,
						Port:              pointer.Int32(6443),
						RequestPath:       pointer.String(httpsProbeRequestPath),
						IntervalInSeconds: pointer.Int32(15),
						NumberOfProbes:    numProbes, // Add to verify that Probes aren't overwritten on update
					},
				},
			},
			OutboundRules: &[]network.OutboundRule{
				{
					Name: pointer.String("OutboundNATAllProtocols"),
					OutboundRulePropertiesFormat: &network.OutboundRulePropertiesFormat{
						FrontendIPConfigurations: &[]network.SubResource{
							{ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/frontendIPConfigurations/my-publiclb-frontEnd")},
						},
						BackendAddressPool: &network.SubResource{
							ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/backendAddressPools/my-publiclb-backendPool"),
						},
						Protocol:             network.LoadBalancerOutboundRuleProtocolAll,
						IdleTimeoutInMinutes: idleTimeout, // Add to verify that OutboundRules aren't overwritten on update
					},
				},
			},
		},
	}
}

func newDefaultInternalAPIServerLB() network.LoadBalancer {
	return network.LoadBalancer{
		Tags: map[string]*string{
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": pointer.String("owned"),
			"sigs.k8s.io_cluster-api-provider-azure_role":               pointer.String(infrav1.APIServerRole),
		},
		Sku:      &network.LoadBalancerSku{Name: network.LoadBalancerSkuNameStandard},
		Location: pointer.String("my-location"),
		LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
			FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
				{
					Name: pointer.String("my-private-lb-frontEnd"),
					FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
						PrivateIPAllocationMethod: network.IPAllocationMethodStatic,
						Subnet: &network.Subnet{
							ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-cp-subnet"),
						},
						PrivateIPAddress: pointer.String("10.0.0.10"),
					},
				},
			},
			BackendAddressPools: &[]network.BackendAddressPool{
				{
					Name: pointer.String("my-private-lb-backendPool"),
				},
			},
			LoadBalancingRules: &[]network.LoadBalancingRule{
				{
					Name: pointer.String(lbRuleHTTPS),
					LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
						DisableOutboundSnat:  pointer.Bool(true),
						Protocol:             network.TransportProtocolTCP,
						FrontendPort:         pointer.Int32(6443),
						BackendPort:          pointer.Int32(6443),
						IdleTimeoutInMinutes: pointer.Int32(4),
						EnableFloatingIP:     pointer.Bool(false),
						LoadDistribution:     network.LoadDistributionDefault,
						FrontendIPConfiguration: &network.SubResource{
							ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-private-lb/frontendIPConfigurations/my-private-lb-frontEnd"),
						},
						BackendAddressPool: &network.SubResource{
							ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-private-lb/backendAddressPools/my-private-lb-backendPool"),
						},
						Probe: &network.SubResource{
							ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-private-lb/probes/HTTPSProbe"),
						},
					},
				},
			},
			OutboundRules: &[]network.OutboundRule{},
			Probes: &[]network.Probe{
				{
					Name: pointer.String(httpsProbe),
					ProbePropertiesFormat: &network.ProbePropertiesFormat{
						Protocol:          network.ProbeProtocolHTTPS,
						Port:              pointer.Int32(6443),
						RequestPath:       pointer.String(httpsProbeRequestPath),
						IntervalInSeconds: pointer.Int32(15),
						NumberOfProbes:    pointer.Int32(4),
					},
				},
			},
		},
	}
}

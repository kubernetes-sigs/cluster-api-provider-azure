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
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"
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

			result, err := tc.spec.Parameters(tc.existing)
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
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
			"sigs.k8s.io_cluster-api-provider-azure_role":               to.StringPtr(infrav1.NodeOutboundRole),
		},
		Sku:      &network.LoadBalancerSku{Name: network.LoadBalancerSkuNameStandard},
		Location: to.StringPtr("my-location"),
		LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
			FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
				{
					Name: to.StringPtr("my-cluster-frontEnd"),
					FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
						PublicIPAddress: &network.PublicIPAddress{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/publicIPAddresses/outbound-publicip")},
					},
				},
			},
			BackendAddressPools: &[]network.BackendAddressPool{
				{
					Name: to.StringPtr("my-cluster-outboundBackendPool"),
				},
			},
			LoadBalancingRules: &[]network.LoadBalancingRule{},
			Probes:             &[]network.Probe{},
			OutboundRules: &[]network.OutboundRule{
				{
					Name: to.StringPtr("OutboundNATAllProtocols"),
					OutboundRulePropertiesFormat: &network.OutboundRulePropertiesFormat{
						FrontendIPConfigurations: &[]network.SubResource{
							{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-cluster/frontendIPConfigurations/my-cluster-frontEnd")},
						},
						BackendAddressPool: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-cluster/backendAddressPools/my-cluster-outboundBackendPool"),
						},
						Protocol:             network.LoadBalancerOutboundRuleProtocolAll,
						IdleTimeoutInMinutes: to.Int32Ptr(30),
					},
				},
			},
		},
	}
}

func newSamplePublicAPIServerLB(verifyFrontendIP bool, verifyBackendAddressPools bool, verifyLBRules bool, verifyProbes bool, verifyOutboundRules bool) network.LoadBalancer {
	var subnet *network.Subnet
	var backendAddressPoolProps *network.BackendAddressPoolPropertiesFormat
	enableFloatingIP := to.BoolPtr(false)
	numProbes := to.Int32Ptr(4)
	idleTimeout := to.Int32Ptr(4)

	if verifyFrontendIP {
		subnet = &network.Subnet{
			Name: to.StringPtr("fake-test-subnet"),
		}
	}
	if verifyBackendAddressPools {
		backendAddressPoolProps = &network.BackendAddressPoolPropertiesFormat{
			Location: to.StringPtr("fake-test-location"),
		}
	}
	if verifyLBRules {
		enableFloatingIP = to.BoolPtr(true)
	}
	if verifyProbes {
		numProbes = to.Int32Ptr(999)
	}
	if verifyOutboundRules {
		idleTimeout = to.Int32Ptr(1000)
	}

	return network.LoadBalancer{
		Tags: map[string]*string{
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
			"sigs.k8s.io_cluster-api-provider-azure_role":               to.StringPtr(infrav1.APIServerRole),
		},
		Sku:      &network.LoadBalancerSku{Name: network.LoadBalancerSkuNameStandard},
		Location: to.StringPtr("my-location"),
		LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
			FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
				{
					Name: to.StringPtr("my-publiclb-frontEnd"),
					FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
						PublicIPAddress: &network.PublicIPAddress{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/publicIPAddresses/my-publicip")},
						Subnet:          subnet, // Add to verify that FrontendIPConfigurations aren't overwritten on update
					},
				},
			},
			BackendAddressPools: &[]network.BackendAddressPool{
				{
					Name:                               to.StringPtr("my-publiclb-backendPool"),
					BackendAddressPoolPropertiesFormat: backendAddressPoolProps, // Add to verify that BackendAddressPools aren't overwritten on update
				},
			},
			LoadBalancingRules: &[]network.LoadBalancingRule{
				{
					Name: to.StringPtr(lbRuleHTTPS),
					LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
						DisableOutboundSnat:  to.BoolPtr(true),
						Protocol:             network.TransportProtocolTCP,
						FrontendPort:         to.Int32Ptr(6443),
						BackendPort:          to.Int32Ptr(6443),
						IdleTimeoutInMinutes: to.Int32Ptr(4),
						EnableFloatingIP:     enableFloatingIP, // Add to verify that LoadBalancingRules aren't overwritten on update
						LoadDistribution:     network.LoadDistributionDefault,
						FrontendIPConfiguration: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/frontendIPConfigurations/my-publiclb-frontEnd"),
						},
						BackendAddressPool: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/backendAddressPools/my-publiclb-backendPool"),
						},
						Probe: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/probes/TCPProbe"),
						},
					},
				},
			},
			Probes: &[]network.Probe{
				{
					Name: to.StringPtr(tcpProbe),
					ProbePropertiesFormat: &network.ProbePropertiesFormat{
						Protocol:          network.ProbeProtocolTCP,
						Port:              to.Int32Ptr(6443),
						IntervalInSeconds: to.Int32Ptr(15),
						NumberOfProbes:    numProbes, // Add to verify that Probes aren't overwritten on update
					},
				},
			},
			OutboundRules: &[]network.OutboundRule{
				{
					Name: to.StringPtr("OutboundNATAllProtocols"),
					OutboundRulePropertiesFormat: &network.OutboundRulePropertiesFormat{
						FrontendIPConfigurations: &[]network.SubResource{
							{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/frontendIPConfigurations/my-publiclb-frontEnd")},
						},
						BackendAddressPool: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/backendAddressPools/my-publiclb-backendPool"),
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
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
			"sigs.k8s.io_cluster-api-provider-azure_role":               to.StringPtr(infrav1.APIServerRole),
		},
		Sku:      &network.LoadBalancerSku{Name: network.LoadBalancerSkuNameStandard},
		Location: to.StringPtr("my-location"),
		LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
			FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
				{
					Name: to.StringPtr("my-private-lb-frontEnd"),
					FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
						PrivateIPAllocationMethod: network.IPAllocationMethodStatic,
						Subnet: &network.Subnet{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-cp-subnet"),
						},
						PrivateIPAddress: to.StringPtr("10.0.0.10"),
					},
				},
			},
			BackendAddressPools: &[]network.BackendAddressPool{
				{
					Name: to.StringPtr("my-private-lb-backendPool"),
				},
			},
			LoadBalancingRules: &[]network.LoadBalancingRule{
				{
					Name: to.StringPtr(lbRuleHTTPS),
					LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
						DisableOutboundSnat:  to.BoolPtr(true),
						Protocol:             network.TransportProtocolTCP,
						FrontendPort:         to.Int32Ptr(6443),
						BackendPort:          to.Int32Ptr(6443),
						IdleTimeoutInMinutes: to.Int32Ptr(4),
						EnableFloatingIP:     to.BoolPtr(false),
						LoadDistribution:     network.LoadDistributionDefault,
						FrontendIPConfiguration: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-private-lb/frontendIPConfigurations/my-private-lb-frontEnd"),
						},
						BackendAddressPool: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-private-lb/backendAddressPools/my-private-lb-backendPool"),
						},
						Probe: &network.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-private-lb/probes/TCPProbe"),
						},
					},
				},
			},
			OutboundRules: &[]network.OutboundRule{},
			Probes: &[]network.Probe{
				{
					Name: to.StringPtr(tcpProbe),
					ProbePropertiesFormat: &network.ProbePropertiesFormat{
						Protocol:          network.ProbeProtocolTCP,
						Port:              to.Int32Ptr(6443),
						IntervalInSeconds: to.Int32Ptr(15),
						NumberOfProbes:    to.Int32Ptr(4),
					},
				},
			},
		},
	}
}

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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func getExistingLBWithMissingFrontendIPConfigs() armnetwork.LoadBalancer {
	existingLB := newSamplePublicAPIServerLB(false, true, true, true, true)
	existingLB.Properties.FrontendIPConfigurations = []*armnetwork.FrontendIPConfiguration{}

	return existingLB
}

func getExistingLBWithMissingBackendPool() armnetwork.LoadBalancer {
	existingLB := newSamplePublicAPIServerLB(true, false, true, true, true)
	existingLB.Properties.BackendAddressPools = []*armnetwork.BackendAddressPool{}

	return existingLB
}

func getExistingLBWithMissingLBRules() armnetwork.LoadBalancer {
	existingLB := newSamplePublicAPIServerLB(true, true, false, true, true)
	existingLB.Properties.LoadBalancingRules = []*armnetwork.LoadBalancingRule{}

	return existingLB
}

func getExistingLBWithMissingProbes() armnetwork.LoadBalancer {
	existingLB := newSamplePublicAPIServerLB(true, true, true, false, true)
	existingLB.Properties.Probes = []*armnetwork.Probe{}

	return existingLB
}

func getExistingLBWithMissingOutboundRules() armnetwork.LoadBalancer {
	existingLB := newSamplePublicAPIServerLB(true, true, true, true, false)
	existingLB.Properties.OutboundRules = []*armnetwork.OutboundRule{}

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
			name:     "load balancer exists with missing additional API server ports",
			spec:     &fakePublicAPILBSpecWithAdditionalPorts,
			existing: getExistingLBWithMissingFrontendIPConfigs(),
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.LoadBalancer{}))
				g.Expect(result.(armnetwork.LoadBalancer)).To(Equal(newSamplePublicAPIServerLB(false, true, true, true, true, func(lb *armnetwork.LoadBalancer) {
					lb.Properties.LoadBalancingRules = append(lb.Properties.LoadBalancingRules, &armnetwork.LoadBalancingRule{
						Name: ptr.To("rke2-agent"),
						Properties: &armnetwork.LoadBalancingRulePropertiesFormat{
							DisableOutboundSnat:  ptr.To(true),
							Protocol:             ptr.To(armnetwork.TransportProtocolTCP),
							FrontendPort:         ptr.To[int32](9345),
							BackendPort:          ptr.To[int32](9345),
							IdleTimeoutInMinutes: ptr.To[int32](4),
							EnableFloatingIP:     ptr.To(false),
							LoadDistribution:     ptr.To(armnetwork.LoadDistributionDefault),
							FrontendIPConfiguration: &armnetwork.SubResource{
								ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/frontendIPConfigurations/my-publiclb-frontEnd"),
							},
							BackendAddressPool: &armnetwork.SubResource{
								ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/backendAddressPools/my-publiclb-backendPool"),
							},
							Probe: &armnetwork.SubResource{
								ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/probes/HTTPSProbe"),
							},
						},
					})
				})))
			},
			expectedError: "",
		},
		{
			name:     "load balancer exists with missing frontend IP configs",
			spec:     &fakePublicAPILBSpec,
			existing: getExistingLBWithMissingFrontendIPConfigs(),
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.LoadBalancer{}))
				g.Expect(result.(armnetwork.LoadBalancer)).To(Equal(newSamplePublicAPIServerLB(false, true, true, true, true)))
			},
			expectedError: "",
		},
		{
			name:     "load balancer exists with missing backend pool",
			spec:     &fakePublicAPILBSpec,
			existing: getExistingLBWithMissingBackendPool(),
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.LoadBalancer{}))
				g.Expect(result.(armnetwork.LoadBalancer)).To(Equal(newSamplePublicAPIServerLB(true, false, true, true, true)))
			},
			expectedError: "",
		},
		{
			name:     "load balancer exists with missing load balancing rules",
			spec:     &fakePublicAPILBSpec,
			existing: getExistingLBWithMissingLBRules(),
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.LoadBalancer{}))
				g.Expect(result.(armnetwork.LoadBalancer)).To(Equal(newSamplePublicAPIServerLB(true, true, false, true, true)))
			},
			expectedError: "",
		},
		{
			name:     "load balancer exists with missing probes",
			spec:     &fakePublicAPILBSpec,
			existing: getExistingLBWithMissingProbes(),
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.LoadBalancer{}))
				g.Expect(result.(armnetwork.LoadBalancer)).To(Equal(newSamplePublicAPIServerLB(true, true, true, false, true)))
			},
			expectedError: "",
		},
		{
			name:     "load balancer exists with missing outbound rules",
			spec:     &fakePublicAPILBSpec,
			existing: getExistingLBWithMissingOutboundRules(),
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.LoadBalancer{}))
				g.Expect(result.(armnetwork.LoadBalancer)).To(Equal(newSamplePublicAPIServerLB(true, true, true, true, false)))
			},
			expectedError: "",
		},
	}
	for _, tc := range testcases {
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

func newDefaultNodeOutboundLB() armnetwork.LoadBalancer {
	return armnetwork.LoadBalancer{
		Tags: map[string]*string{
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
			"sigs.k8s.io_cluster-api-provider-azure_role":               ptr.To(infrav1.NodeOutboundRole),
		},
		SKU:      &armnetwork.LoadBalancerSKU{Name: ptr.To(armnetwork.LoadBalancerSKUNameStandard)},
		Location: ptr.To("my-location"),
		Properties: &armnetwork.LoadBalancerPropertiesFormat{
			FrontendIPConfigurations: []*armnetwork.FrontendIPConfiguration{
				{
					Name: ptr.To("my-cluster-frontEnd"),
					Properties: &armnetwork.FrontendIPConfigurationPropertiesFormat{
						PublicIPAddress: &armnetwork.PublicIPAddress{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/publicIPAddresses/outbound-publicip")},
					},
				},
			},
			BackendAddressPools: []*armnetwork.BackendAddressPool{
				{
					Name: ptr.To("my-cluster-outboundBackendPool"),
				},
			},
			LoadBalancingRules: []*armnetwork.LoadBalancingRule{},
			Probes:             []*armnetwork.Probe{},
			OutboundRules: []*armnetwork.OutboundRule{
				{
					Name: ptr.To("OutboundNATAllProtocols"),
					Properties: &armnetwork.OutboundRulePropertiesFormat{
						FrontendIPConfigurations: []*armnetwork.SubResource{
							{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-cluster/frontendIPConfigurations/my-cluster-frontEnd")},
						},
						BackendAddressPool: &armnetwork.SubResource{
							ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-cluster/backendAddressPools/my-cluster-outboundBackendPool"),
						},
						Protocol:             ptr.To(armnetwork.LoadBalancerOutboundRuleProtocolAll),
						IdleTimeoutInMinutes: ptr.To[int32](30),
					},
				},
			},
		},
	}
}

func newSamplePublicAPIServerLB(verifyFrontendIP bool, verifyBackendAddressPools bool, verifyLBRules bool, verifyProbes bool, verifyOutboundRules bool, modifications ...func(*armnetwork.LoadBalancer)) armnetwork.LoadBalancer {
	var subnet *armnetwork.Subnet
	var backendAddressPoolProps *armnetwork.BackendAddressPoolPropertiesFormat
	enableFloatingIP := ptr.To(false)
	numProbes := ptr.To[int32](4)
	idleTimeout := ptr.To[int32](4)

	if verifyFrontendIP {
		subnet = &armnetwork.Subnet{
			Name: ptr.To("fake-test-subnet"),
		}
	}
	if verifyBackendAddressPools {
		backendAddressPoolProps = &armnetwork.BackendAddressPoolPropertiesFormat{
			Location: ptr.To("fake-test-location"),
		}
	}
	if verifyLBRules {
		enableFloatingIP = ptr.To(true)
	}
	if verifyProbes {
		numProbes = ptr.To[int32](999)
	}
	if verifyOutboundRules {
		idleTimeout = ptr.To[int32](1000)
	}

	lb := armnetwork.LoadBalancer{
		Tags: map[string]*string{
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
			"sigs.k8s.io_cluster-api-provider-azure_role":               ptr.To(infrav1.APIServerRole),
		},
		SKU:      &armnetwork.LoadBalancerSKU{Name: ptr.To(armnetwork.LoadBalancerSKUNameStandard)},
		Location: ptr.To("my-location"),
		Properties: &armnetwork.LoadBalancerPropertiesFormat{
			FrontendIPConfigurations: []*armnetwork.FrontendIPConfiguration{
				{
					Name: ptr.To("my-publiclb-frontEnd"),
					Properties: &armnetwork.FrontendIPConfigurationPropertiesFormat{
						PublicIPAddress: &armnetwork.PublicIPAddress{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/publicIPAddresses/my-publicip")},
						Subnet:          subnet, // Add to verify that FrontendIPConfigurations aren't overwritten on update
					},
				},
			},
			BackendAddressPools: []*armnetwork.BackendAddressPool{
				{
					Name:       ptr.To("my-publiclb-backendPool"),
					Properties: backendAddressPoolProps, // Add to verify that BackendAddressPools aren't overwritten on update
				},
			},
			LoadBalancingRules: []*armnetwork.LoadBalancingRule{
				{
					Name: ptr.To(lbRuleHTTPS),
					Properties: &armnetwork.LoadBalancingRulePropertiesFormat{
						DisableOutboundSnat:  ptr.To(true),
						Protocol:             ptr.To(armnetwork.TransportProtocolTCP),
						FrontendPort:         ptr.To[int32](6443),
						BackendPort:          ptr.To[int32](6443),
						IdleTimeoutInMinutes: ptr.To[int32](4),
						EnableFloatingIP:     enableFloatingIP, // Add to verify that LoadBalancingRules aren't overwritten on update
						LoadDistribution:     ptr.To(armnetwork.LoadDistributionDefault),
						FrontendIPConfiguration: &armnetwork.SubResource{
							ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/frontendIPConfigurations/my-publiclb-frontEnd"),
						},
						BackendAddressPool: &armnetwork.SubResource{
							ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/backendAddressPools/my-publiclb-backendPool"),
						},
						Probe: &armnetwork.SubResource{
							ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/probes/HTTPSProbe"),
						},
					},
				},
			},
			Probes: []*armnetwork.Probe{
				{
					Name: ptr.To(httpsProbe),
					Properties: &armnetwork.ProbePropertiesFormat{
						Protocol:          ptr.To(armnetwork.ProbeProtocolHTTPS),
						Port:              ptr.To[int32](6443),
						RequestPath:       ptr.To(httpsProbeRequestPath),
						IntervalInSeconds: ptr.To[int32](15),
						NumberOfProbes:    numProbes, // Add to verify that Probes aren't overwritten on update
					},
				},
			},
			OutboundRules: []*armnetwork.OutboundRule{
				{
					Name: ptr.To("OutboundNATAllProtocols"),
					Properties: &armnetwork.OutboundRulePropertiesFormat{
						FrontendIPConfigurations: []*armnetwork.SubResource{
							{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/frontendIPConfigurations/my-publiclb-frontEnd")},
						},
						BackendAddressPool: &armnetwork.SubResource{
							ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-publiclb/backendAddressPools/my-publiclb-backendPool"),
						},
						Protocol:             ptr.To(armnetwork.LoadBalancerOutboundRuleProtocolAll),
						IdleTimeoutInMinutes: idleTimeout, // Add to verify that OutboundRules aren't overwritten on update
					},
				},
			},
		},
	}

	for _, modify := range modifications {
		modify(&lb)
	}

	return lb
}

func newDefaultInternalAPIServerLB() armnetwork.LoadBalancer {
	return armnetwork.LoadBalancer{
		Tags: map[string]*string{
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
			"sigs.k8s.io_cluster-api-provider-azure_role":               ptr.To(infrav1.APIServerRole),
		},
		SKU:      &armnetwork.LoadBalancerSKU{Name: ptr.To(armnetwork.LoadBalancerSKUNameStandard)},
		Location: ptr.To("my-location"),
		Properties: &armnetwork.LoadBalancerPropertiesFormat{
			FrontendIPConfigurations: []*armnetwork.FrontendIPConfiguration{
				{
					Name: ptr.To("my-private-lb-frontEnd"),
					Properties: &armnetwork.FrontendIPConfigurationPropertiesFormat{
						PrivateIPAllocationMethod: ptr.To(armnetwork.IPAllocationMethodStatic),
						Subnet: &armnetwork.Subnet{
							ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-cp-subnet"),
						},
						PrivateIPAddress: ptr.To("10.0.0.10"),
					},
				},
			},
			BackendAddressPools: []*armnetwork.BackendAddressPool{
				{
					Name: ptr.To("my-private-lb-backendPool"),
				},
			},
			LoadBalancingRules: []*armnetwork.LoadBalancingRule{
				{
					Name: ptr.To(lbRuleHTTPS),
					Properties: &armnetwork.LoadBalancingRulePropertiesFormat{
						DisableOutboundSnat:  ptr.To(true),
						Protocol:             ptr.To(armnetwork.TransportProtocolTCP),
						FrontendPort:         ptr.To[int32](6443),
						BackendPort:          ptr.To[int32](6443),
						IdleTimeoutInMinutes: ptr.To[int32](4),
						EnableFloatingIP:     ptr.To(false),
						LoadDistribution:     ptr.To(armnetwork.LoadDistributionDefault),
						FrontendIPConfiguration: &armnetwork.SubResource{
							ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-private-lb/frontendIPConfigurations/my-private-lb-frontEnd"),
						},
						BackendAddressPool: &armnetwork.SubResource{
							ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-private-lb/backendAddressPools/my-private-lb-backendPool"),
						},
						Probe: &armnetwork.SubResource{
							ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/my-private-lb/probes/HTTPSProbe"),
						},
					},
				},
			},
			OutboundRules: []*armnetwork.OutboundRule{},
			Probes: []*armnetwork.Probe{
				{
					Name: ptr.To(httpsProbe),
					Properties: &armnetwork.ProbePropertiesFormat{
						Protocol:          ptr.To(armnetwork.ProbeProtocolHTTPS),
						Port:              ptr.To[int32](6443),
						RequestPath:       ptr.To(httpsProbeRequestPath),
						IntervalInSeconds: ptr.To[int32](15),
						NumberOfProbes:    ptr.To[int32](4),
					},
				},
			},
		},
	}
}

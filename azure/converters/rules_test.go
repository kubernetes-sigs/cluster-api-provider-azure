/*
Copyright 2022 The Kubernetes Authors.

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

package converters

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func Test_SecurityRuleToSDK(t *testing.T) {
	cases := []struct {
		name         string
		securityRule infrav1.SecurityRule
		expect       func(*GomegaWithT, network.SecurityRule)
	}{
		{
			name: "convert security role with security group protocol all",
			securityRule: infrav1.SecurityRule{
				Name:             "fake-rule",
				Description:      "fake rule description",
				Source:           to.StringPtr("fake-source"),
				SourcePorts:      to.StringPtr("fake-source-ports"),
				Destination:      to.StringPtr("fake-destination"),
				DestinationPorts: to.StringPtr("fake-destination-port-ranges"),
				Priority:         2,
				Protocol:         infrav1.SecurityGroupProtocolAll,
			},
			expect: func(g *GomegaWithT, result network.SecurityRule) {
				g.Expect(result).To(Equal(network.SecurityRule{
					Name: to.StringPtr("fake-rule"),
					SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
						Description:              to.StringPtr("fake rule description"),
						SourceAddressPrefix:      to.StringPtr("fake-source"),
						SourcePortRange:          to.StringPtr("fake-source-ports"),
						DestinationAddressPrefix: to.StringPtr("fake-destination"),
						DestinationPortRange:     to.StringPtr("fake-destination-port-ranges"),
						Access:                   network.SecurityRuleAccessAllow,
						Priority:                 to.Int32Ptr(2),
						Protocol:                 network.SecurityRuleProtocolAsterisk,
					},
				}))
			},
		},
		{
			name: "convert security role with SecurityGroupProtocolTCP",
			securityRule: infrav1.SecurityRule{
				Name:             "fake-rule",
				Description:      "fake rule description",
				Source:           to.StringPtr("fake-source"),
				SourcePorts:      to.StringPtr("fake-source-ports"),
				Destination:      to.StringPtr("fake-destination"),
				DestinationPorts: to.StringPtr("fake-destination-port-ranges"),
				Priority:         2,
				Protocol:         infrav1.SecurityGroupProtocolTCP,
			},
			expect: func(g *GomegaWithT, result network.SecurityRule) {
				g.Expect(result).To(Equal(network.SecurityRule{
					Name: to.StringPtr("fake-rule"),
					SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
						Description:              to.StringPtr("fake rule description"),
						SourceAddressPrefix:      to.StringPtr("fake-source"),
						SourcePortRange:          to.StringPtr("fake-source-ports"),
						DestinationAddressPrefix: to.StringPtr("fake-destination"),
						DestinationPortRange:     to.StringPtr("fake-destination-port-ranges"),
						Access:                   network.SecurityRuleAccessAllow,
						Priority:                 to.Int32Ptr(2),
						Protocol:                 network.SecurityRuleProtocolTCP,
					},
				}))
			},
		},
		{
			name: "convert security role with SecurityGroupProtocolUDP",
			securityRule: infrav1.SecurityRule{
				Name:             "fake-rule",
				Description:      "fake rule description",
				Source:           to.StringPtr("fake-source"),
				SourcePorts:      to.StringPtr("fake-source-ports"),
				Destination:      to.StringPtr("fake-destination"),
				DestinationPorts: to.StringPtr("fake-destination-port-ranges"),
				Priority:         2,
				Protocol:         infrav1.SecurityGroupProtocolUDP,
			},
			expect: func(g *GomegaWithT, result network.SecurityRule) {
				g.Expect(result).To(Equal(network.SecurityRule{
					Name: to.StringPtr("fake-rule"),
					SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
						Description:              to.StringPtr("fake rule description"),
						SourceAddressPrefix:      to.StringPtr("fake-source"),
						SourcePortRange:          to.StringPtr("fake-source-ports"),
						DestinationAddressPrefix: to.StringPtr("fake-destination"),
						DestinationPortRange:     to.StringPtr("fake-destination-port-ranges"),
						Access:                   network.SecurityRuleAccessAllow,
						Priority:                 to.Int32Ptr(2),
						Protocol:                 network.SecurityRuleProtocolUDP,
					},
				}))
			},
		},
		{
			name: "convert security role with SecurityGroupProtocolICMP",
			securityRule: infrav1.SecurityRule{
				Name:             "fake-rule",
				Description:      "fake rule description",
				Source:           to.StringPtr("fake-source"),
				SourcePorts:      to.StringPtr("fake-source-ports"),
				Destination:      to.StringPtr("fake-destination"),
				DestinationPorts: to.StringPtr("fake-destination-port-ranges"),
				Priority:         2,
				Protocol:         infrav1.SecurityGroupProtocolICMP,
			},
			expect: func(g *GomegaWithT, result network.SecurityRule) {
				g.Expect(result).To(Equal(network.SecurityRule{
					Name: to.StringPtr("fake-rule"),
					SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
						Description:              to.StringPtr("fake rule description"),
						SourceAddressPrefix:      to.StringPtr("fake-source"),
						SourcePortRange:          to.StringPtr("fake-source-ports"),
						DestinationAddressPrefix: to.StringPtr("fake-destination"),
						DestinationPortRange:     to.StringPtr("fake-destination-port-ranges"),
						Access:                   network.SecurityRuleAccessAllow,
						Priority:                 to.Int32Ptr(2),
						Protocol:                 network.SecurityRuleProtocolIcmp,
					},
				}))
			},
		},
		{
			name: "convert security role with direction outbound",
			securityRule: infrav1.SecurityRule{
				Name:             "fake-rule",
				Description:      "fake rule description",
				Source:           to.StringPtr("fake-source"),
				SourcePorts:      to.StringPtr("fake-source-ports"),
				Destination:      to.StringPtr("fake-destination"),
				DestinationPorts: to.StringPtr("fake-destination-port-ranges"),
				Priority:         2,
				Direction:        infrav1.SecurityRuleDirectionOutbound,
			},
			expect: func(g *GomegaWithT, result network.SecurityRule) {
				g.Expect(result).To(Equal(network.SecurityRule{
					Name: to.StringPtr("fake-rule"),
					SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
						Description:              to.StringPtr("fake rule description"),
						SourceAddressPrefix:      to.StringPtr("fake-source"),
						SourcePortRange:          to.StringPtr("fake-source-ports"),
						DestinationAddressPrefix: to.StringPtr("fake-destination"),
						DestinationPortRange:     to.StringPtr("fake-destination-port-ranges"),
						Access:                   network.SecurityRuleAccessAllow,
						Priority:                 to.Int32Ptr(2),
						Direction:                network.SecurityRuleDirectionOutbound,
					},
				}))
			},
		},
		{
			name: "convert security role with direction inbound",
			securityRule: infrav1.SecurityRule{
				Name:             "fake-rule",
				Description:      "fake rule description",
				Source:           to.StringPtr("fake-source"),
				SourcePorts:      to.StringPtr("fake-source-ports"),
				Destination:      to.StringPtr("fake-destination"),
				DestinationPorts: to.StringPtr("fake-destination-port-ranges"),
				Priority:         2,
				Direction:        infrav1.SecurityRuleDirectionInbound,
			},
			expect: func(g *GomegaWithT, result network.SecurityRule) {
				g.Expect(result).To(Equal(network.SecurityRule{
					Name: to.StringPtr("fake-rule"),
					SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
						Description:              to.StringPtr("fake rule description"),
						SourceAddressPrefix:      to.StringPtr("fake-source"),
						SourcePortRange:          to.StringPtr("fake-source-ports"),
						DestinationAddressPrefix: to.StringPtr("fake-destination"),
						DestinationPortRange:     to.StringPtr("fake-destination-port-ranges"),
						Access:                   network.SecurityRuleAccessAllow,
						Priority:                 to.Int32Ptr(2),
						Direction:                network.SecurityRuleDirectionInbound,
					},
				}))
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := NewGomegaWithT(t)
			result := SecurityRuleToSDK(c.securityRule)
			c.expect(g, result)
		})
	}
}

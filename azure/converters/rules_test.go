/*
Copyright 2026 The Kubernetes Authors.

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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	"github.com/google/go-cmp/cmp"
	"k8s.io/utils/ptr"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestSecurityRuleToSDK(t *testing.T) {
	tests := []struct {
		name string
		rule infrav1.SecurityRule
		want *armnetwork.SecurityRule
	}{
		{
			name: "tcp inbound rule with single source and destination",
			rule: infrav1.SecurityRule{
				Name:             "allow-ssh",
				Description:      "allow ssh",
				Protocol:         infrav1.SecurityGroupProtocolTCP,
				Direction:        infrav1.SecurityRuleDirectionInbound,
				Priority:         100,
				Source:           ptr.To("10.0.0.0/8"),
				SourcePorts:      ptr.To("*"),
				Destination:      ptr.To("*"),
				DestinationPorts: ptr.To("22"),
				Action:           infrav1.SecurityRuleActionAllow,
			},
			want: &armnetwork.SecurityRule{
				Name: ptr.To("allow-ssh"),
				Properties: &armnetwork.SecurityRulePropertiesFormat{
					Description:              ptr.To("allow ssh"),
					SourceAddressPrefix:      ptr.To("10.0.0.0/8"),
					SourcePortRange:          ptr.To("*"),
					DestinationAddressPrefix: ptr.To("*"),
					DestinationPortRange:     ptr.To("22"),
					Access:                   ptr.To(armnetwork.SecurityRuleAccessAllow),
					Priority:                 ptr.To[int32](100),
					Protocol:                 ptr.To(armnetwork.SecurityRuleProtocolTCP),
					Direction:                ptr.To(armnetwork.SecurityRuleDirectionInbound),
				},
			},
		},
		{
			name: "udp outbound rule with multiple sources",
			rule: infrav1.SecurityRule{
				Name:      "allow-dns",
				Protocol:  infrav1.SecurityGroupProtocolUDP,
				Direction: infrav1.SecurityRuleDirectionOutbound,
				Priority:  200,
				Sources:   []*string{ptr.To("10.0.0.0/8"), ptr.To("192.168.0.0/16")},
				Action:    infrav1.SecurityRuleActionDeny,
			},
			want: &armnetwork.SecurityRule{
				Name: ptr.To("allow-dns"),
				Properties: &armnetwork.SecurityRulePropertiesFormat{
					Description:           ptr.To(""),
					SourceAddressPrefixes: []*string{ptr.To("10.0.0.0/8"), ptr.To("192.168.0.0/16")},
					Access:                ptr.To(armnetwork.SecurityRuleAccessDeny),
					Priority:              ptr.To[int32](200),
					Protocol:              ptr.To(armnetwork.SecurityRuleProtocolUDP),
					Direction:             ptr.To(armnetwork.SecurityRuleDirectionOutbound),
				},
			},
		},
		{
			name: "icmp rule",
			rule: infrav1.SecurityRule{
				Name:     "allow-icmp",
				Protocol: infrav1.SecurityGroupProtocolICMP,
				Priority: 300,
			},
			want: &armnetwork.SecurityRule{
				Name: ptr.To("allow-icmp"),
				Properties: &armnetwork.SecurityRulePropertiesFormat{
					Description: ptr.To(""),
					Access:      ptr.To(armnetwork.SecurityRuleAccess("")),
					Priority:    ptr.To[int32](300),
					Protocol:    ptr.To(armnetwork.SecurityRuleProtocolIcmp),
				},
			},
		},
		{
			name: "all protocols rule",
			rule: infrav1.SecurityRule{
				Name:     "allow-all",
				Protocol: infrav1.SecurityGroupProtocolAll,
				Priority: 400,
			},
			want: &armnetwork.SecurityRule{
				Name: ptr.To("allow-all"),
				Properties: &armnetwork.SecurityRulePropertiesFormat{
					Description: ptr.To(""),
					Access:      ptr.To(armnetwork.SecurityRuleAccess("")),
					Priority:    ptr.To[int32](400),
					Protocol:    ptr.To(armnetwork.SecurityRuleProtocolAsterisk),
				},
			},
		},
		{
			name: "unrecognized protocol and direction leave both nil",
			rule: infrav1.SecurityRule{
				Name:      "unknown-rule",
				Protocol:  infrav1.SecurityGroupProtocol("unknown"),
				Direction: infrav1.SecurityRuleDirection("unknown"),
				Priority:  500,
			},
			want: &armnetwork.SecurityRule{
				Name: ptr.To("unknown-rule"),
				Properties: &armnetwork.SecurityRulePropertiesFormat{
					Description: ptr.To(""),
					Access:      ptr.To(armnetwork.SecurityRuleAccess("")),
					Priority:    ptr.To[int32](500),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := SecurityRuleToSDK(tt.rule)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("SecurityRuleToSDK(%s) mismatch (-want +got):\n%s", tt.name, diff)
			}
		})
	}
}

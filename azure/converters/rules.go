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

package converters

import (
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
)

// SecurityRuleToSDK converts a CAPZ security rule to an Azure network security rule.
func SecurityRuleToSDK(rule infrav1.SecurityRule) network.SecurityRule {
	secRule := network.SecurityRule{
		Name: to.StringPtr(rule.Name),
		SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
			Description:              to.StringPtr(rule.Description),
			SourceAddressPrefix:      rule.Source,
			SourcePortRange:          rule.SourcePorts,
			DestinationAddressPrefix: rule.Destination,
			DestinationPortRange:     rule.DestinationPorts,
			Access:                   network.SecurityRuleAccessAllow,
			Priority:                 to.Int32Ptr(rule.Priority),
		},
	}

	switch rule.Protocol {
	case infrav1.SecurityGroupProtocolAll:
		secRule.Protocol = network.SecurityRuleProtocolAsterisk
	case infrav1.SecurityGroupProtocolTCP:
		secRule.Protocol = network.SecurityRuleProtocolTCP
	case infrav1.SecurityGroupProtocolUDP:
		secRule.Protocol = network.SecurityRuleProtocolUDP
	case infrav1.SecurityGroupProtocolICMP:
		secRule.Protocol = network.SecurityRuleProtocolIcmp
	}

	switch rule.Direction {
	case infrav1.SecurityRuleDirectionOutbound:
		secRule.Direction = network.SecurityRuleDirectionOutbound
	case infrav1.SecurityRuleDirectionInbound:
		secRule.Direction = network.SecurityRuleDirectionInbound
	}

	return secRule
}

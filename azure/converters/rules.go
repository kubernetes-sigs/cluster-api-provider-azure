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
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
)

// IngresstoSecurityRule converts a CAPI ingress rule to an Azure network security rule.
func IngresstoSecurityRule(ingress infrav1.IngressRule) network.SecurityRule {
	secRule := network.SecurityRule{
		Name: to.StringPtr(ingress.Name),
		SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
			Description:              to.StringPtr(ingress.Description),
			SourceAddressPrefix:      ingress.Source,
			SourcePortRange:          ingress.SourcePorts,
			DestinationAddressPrefix: ingress.Destination,
			DestinationPortRange:     ingress.DestinationPorts,
			Access:                   network.SecurityRuleAccessAllow,
			Direction:                network.SecurityRuleDirectionInbound,
			Priority:                 to.Int32Ptr(ingress.Priority),
		},
	}

	switch ingress.Protocol {
	case infrav1.SecurityGroupProtocolAll:
		secRule.Protocol = network.SecurityRuleProtocolAsterisk
	case infrav1.SecurityGroupProtocolTCP:
		secRule.Protocol = network.SecurityRuleProtocolTCP
	case infrav1.SecurityGroupProtocolUDP:
		secRule.Protocol = network.SecurityRuleProtocolUDP
	}

	return secRule
}

// SecuritytoIngressRule converts an Azure network security rule to a CAPI ingress rule.
func SecuritytoIngressRule(rule network.SecurityRule) infrav1.IngressRule {
	ingRule := infrav1.IngressRule{
		Name:             to.String(rule.Name),
		Description:      to.String(rule.Description),
		Priority:         to.Int32(rule.Priority),
		SourcePorts:      rule.SourcePortRange,
		DestinationPorts: rule.DestinationPortRange,
		Source:           rule.SourceAddressPrefix,
		Destination:      rule.DestinationAddressPrefix,
	}

	switch rule.Protocol {
	case network.SecurityRuleProtocolAsterisk:
		ingRule.Protocol = infrav1.SecurityGroupProtocolAll
	case network.SecurityRuleProtocolTCP:
		ingRule.Protocol = infrav1.SecurityGroupProtocolTCP
	case network.SecurityRuleProtocolUDP:
		ingRule.Protocol = infrav1.SecurityGroupProtocolUDP
	}

	return ingRule
}

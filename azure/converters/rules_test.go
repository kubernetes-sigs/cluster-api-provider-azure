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
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestSecurityRuleToSDK(t *testing.T) {
	// Test case for a security rule with all fields provided
	rule := infrav1.SecurityRule{
		Name: "test-rule",
		Description: "Test security rule",
		Source: "10.0.0.0/24",
		SourcePorts: "80,443",
		Destination: "192.168.0.0/16",
		DestinationPorts: "22",
		Protocol: infrav1.SecurityGroupProtocolTCP,
		Direction: infrav1.SecurityRuleDirectionInbound,
		Priority: 100,
	}

	expectedResult := network.SecurityRule{
		Name: pointer.String("test-rule"),
		SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
			Description: pointer.String("Test security rule"),
			SourceAddressPrefix: pointer.String("10.0.0.0/24"),
			SourcePortRange: pointer.String("80,443"),
			DestinationAddressPrefix: pointer.String("192.168.0.0/16"),
			DestinationPortRange: pointer.String("22"),
			Access: network.SecurityRuleAccessAllow,
			Priority: pointer.Int32(100),
		},
		Protocol: network.SecurityRuleProtocolTCP,
		Direction: network.SecurityRuleDirectionInbound,
	}

	actualResult := SecurityRuleToSDK(rule)
	assert.Equal(t, expectedResult, actualResult)

	// Test case for a security rule with minimum fields provided
	rule = infrav1.SecurityRule{
		Name: "test-rule",
		Protocol: infrav1.SecurityGroupProtocolAll,
		Direction: infrav1.SecurityRuleDirectionInbound,
	}

	expectedResult = network.SecurityRule{
		Name: pointer.String("test-rule"),
		SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
			Access: network.SecurityRuleAccessAllow,
		},
		Protocol: network.SecurityRuleProtocolAsterisk,
		Direction: network.SecurityRuleDirectionInbound,
	}

	actualResult = SecurityRuleToSDK(rule)
	assert.Equal(t, expectedResult, actualResult)

	// Test case for a security rule with invalid protocol
	rule = infrav1.SecurityRule{
		Name: "test-rule",
		Protocol: "invalid-protocol",
		Direction: infrav1.SecurityRuleDirectionInbound,
	}

	assert.PanicsWithError(t, "invalid protocol value", func() { SecurityRuleToSDK(rule) })

	// Test case for a security rule with invalid direction
	rule = infrav1.SecurityRule{
		Name: "test-rule",
		Protocol: infrav1.SecurityGroupProtocolAll,
		Direction: "invalid-direction",
	}

	assert.PanicsWithError(t, "invalid direction value", func() { SecurityRuleToSDK(rule) })
}

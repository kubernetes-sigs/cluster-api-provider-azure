/*
Copyright 2023 The Kubernetes Authors.

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

package privatelinks

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2022-07-01/network"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
)

const (
	fakeRegion            = "westeurope"
	fakeSubscriptionID1   = "abcd"
	fakeSubscriptionID2   = "efgh"
	fakeClusterName       = "my-cluster"
	fakeVNetResourceGroup = fakeClusterName
	fakeVNetName          = fakeClusterName + "-vnet"
	fakeSubnetName        = fakeClusterName + "-node-subnet"
	fakeLbName            = fakeClusterName + "-internal-lb"
	fakeLbIPConfigName1   = fakeLbName + "-frontend1"
	fakeLbIPConfigName2   = fakeLbName + "-frontend2"
	fakePrivateLinkName   = "apiserver-privatelink"
)

var (
	// fakePrivateLinkSpec1 is private link spec with:
	// - 1 allowed subscription,
	// - 1 auto-approved subscription,
	// - disabled proxy protocol and
	// - additional tag "hello:capz".
	fakePrivateLinkSpec1 = PrivateLinkSpec{
		Name:              fakePrivateLinkName,
		ResourceGroup:     fakeClusterName,
		SubscriptionID:    fakeSubscriptionID1,
		Location:          fakeRegion,
		VNetResourceGroup: fakeVNetResourceGroup,
		VNet:              fakeVNetName,
		NATIPConfiguration: []NATIPConfiguration{
			{
				AllocationMethod: string(network.Dynamic),
				Subnet:           fakeSubnetName,
			},
		},
		LoadBalancerName: fakeLbName,
		LBFrontendIPConfigNames: []string{
			fakeLbIPConfigName1,
		},
		AllowedSubscriptions: []string{
			fakeSubscriptionID1,
		},
		AutoApprovedSubscriptions: []string{
			fakeSubscriptionID1,
		},
		EnableProxyProtocol: ptr.To(false),
		ClusterName:         fakeClusterName,
		AdditionalTags: map[string]string{
			"hello": "capz",
		},
	}

	// fakePrivateLinkSpec2 is modified fakePrivateLinkSpec1 with following changes:
	// - 1 added allowed subscription.
	fakePrivateLinkSpec2 = PrivateLinkSpec{
		Name:              fakePrivateLinkName,
		ResourceGroup:     fakeClusterName,
		SubscriptionID:    fakeSubscriptionID1,
		Location:          fakeRegion,
		VNetResourceGroup: fakeVNetResourceGroup,
		VNet:              fakeVNetName,
		NATIPConfiguration: []NATIPConfiguration{
			{
				AllocationMethod: string(network.Dynamic),
				Subnet:           fakeSubnetName,
			},
		},
		LoadBalancerName: fakeLbName,
		LBFrontendIPConfigNames: []string{
			fakeLbIPConfigName1,
		},
		AllowedSubscriptions: []string{
			fakeSubscriptionID1,
			fakeSubscriptionID2,
		},
		AutoApprovedSubscriptions: []string{
			fakeSubscriptionID1,
		},
		EnableProxyProtocol: ptr.To(false),
		ClusterName:         fakeClusterName,
		AdditionalTags: map[string]string{
			"hello": "capz",
		},
	}

	// fakePrivateLinkSpec3 is modified fakePrivateLinkSpec2 with following changes:
	// - 1 added auto-approved subscription.
	fakePrivateLinkSpec3 = PrivateLinkSpec{
		Name:              fakePrivateLinkName,
		ResourceGroup:     fakeClusterName,
		SubscriptionID:    fakeSubscriptionID1,
		Location:          fakeRegion,
		VNetResourceGroup: fakeVNetResourceGroup,
		VNet:              fakeVNetName,
		NATIPConfiguration: []NATIPConfiguration{
			{
				AllocationMethod: string(network.Dynamic),
				Subnet:           fakeSubnetName,
			},
		},
		LoadBalancerName: fakeLbName,
		LBFrontendIPConfigNames: []string{
			fakeLbIPConfigName1,
		},
		AllowedSubscriptions: []string{
			fakeSubscriptionID1,
			fakeSubscriptionID2,
		},
		AutoApprovedSubscriptions: []string{
			fakeSubscriptionID1,
			fakeSubscriptionID2,
		},
		EnableProxyProtocol: ptr.To(false),
		ClusterName:         fakeClusterName,
		AdditionalTags: map[string]string{
			"hello": "capz",
		},
	}

	// fakePrivateLinkSpec4 is modified fakePrivateLinkSpec3 with following changes:
	// - enabled proxy protocol.
	fakePrivateLinkSpec4 = PrivateLinkSpec{
		Name:              fakePrivateLinkName,
		ResourceGroup:     fakeClusterName,
		SubscriptionID:    fakeSubscriptionID1,
		Location:          fakeRegion,
		VNetResourceGroup: fakeVNetResourceGroup,
		VNet:              fakeVNetName,
		NATIPConfiguration: []NATIPConfiguration{
			{
				AllocationMethod: string(network.Dynamic),
				Subnet:           fakeSubnetName,
			},
		},
		LoadBalancerName: fakeLbName,
		LBFrontendIPConfigNames: []string{
			fakeLbIPConfigName1,
		},
		AllowedSubscriptions: []string{
			fakeSubscriptionID1,
			fakeSubscriptionID2,
		},
		AutoApprovedSubscriptions: []string{
			fakeSubscriptionID1,
			fakeSubscriptionID2,
		},
		EnableProxyProtocol: ptr.To(true),
		ClusterName:         fakeClusterName,
		AdditionalTags: map[string]string{
			"hello": "capz",
		},
	}

	// fakePrivateLinkSpec5 is modified fakePrivateLinkSpec4 with following changes:
	// - changed LB frontend config name.
	fakePrivateLinkSpec5 = PrivateLinkSpec{
		Name:              fakePrivateLinkName,
		ResourceGroup:     fakeClusterName,
		SubscriptionID:    fakeSubscriptionID1,
		Location:          fakeRegion,
		VNetResourceGroup: fakeVNetResourceGroup,
		VNet:              fakeVNetName,
		NATIPConfiguration: []NATIPConfiguration{
			{
				AllocationMethod: string(network.Dynamic),
				Subnet:           fakeSubnetName,
			},
		},
		LoadBalancerName: fakeLbName,
		LBFrontendIPConfigNames: []string{
			fakeLbIPConfigName2,
		},
		AllowedSubscriptions: []string{
			fakeSubscriptionID1,
			fakeSubscriptionID2,
		},
		AutoApprovedSubscriptions: []string{
			fakeSubscriptionID1,
			fakeSubscriptionID2,
		},
		EnableProxyProtocol: ptr.To(true),
		ClusterName:         fakeClusterName,
		AdditionalTags: map[string]string{
			"hello": "capz",
		},
	}

	// fakePrivateLink1 is Azure PrivateLinkService that corresponds to fakePrivateLinkSpec1.
	fakePrivateLink1 = network.PrivateLinkService{
		Name:     ptr.To(fakePrivateLinkName),
		Location: ptr.To(fakeRegion),
		PrivateLinkServiceProperties: &network.PrivateLinkServiceProperties{
			IPConfigurations: &[]network.PrivateLinkServiceIPConfiguration{
				{
					Name: ptr.To(fmt.Sprintf("%s-natipconfig-1", fakeSubnetName)),
					PrivateLinkServiceIPConfigurationProperties: &network.PrivateLinkServiceIPConfigurationProperties{
						Subnet: &network.Subnet{
							ID: ptr.To(
								fmt.Sprintf(
									"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/%s",
									fakeSubscriptionID1,
									fakeVNetResourceGroup,
									fakeVNetName,
									fakeSubnetName)),
						},
						PrivateIPAllocationMethod: network.Dynamic,
						Primary:                   ptr.To(true),
					},
				},
			},
			LoadBalancerFrontendIPConfigurations: &[]network.FrontendIPConfiguration{
				{
					ID: ptr.To(
						fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers/%s/frontendIPConfigurations/%s",
							fakeSubscriptionID1,
							fakeClusterName,
							fakeLbName,
							fakeLbIPConfigName1)),
				},
			},
			Visibility: &network.PrivateLinkServicePropertiesVisibility{
				Subscriptions: &[]string{
					fakeSubscriptionID1,
				},
			},
			AutoApproval: &network.PrivateLinkServicePropertiesAutoApproval{
				Subscriptions: &[]string{
					fakeSubscriptionID1,
				},
			},
			EnableProxyProtocol: ptr.To(false),
		},
		Tags: map[string]*string{
			"sigs.k8s.io_cluster-api-provider-azure_cluster_" + fakeClusterName: ptr.To("owned"),
			"Name":  ptr.To(fakePrivateLinkName),
			"hello": ptr.To("capz"),
		},
	}

	// fakePrivateLink2 is Azure PrivateLinkService that corresponds to fakePrivateLinkSpec2.
	fakePrivateLink2 = network.PrivateLinkService{
		Name:     ptr.To(fakePrivateLinkName),
		Location: ptr.To(fakeRegion),
		PrivateLinkServiceProperties: &network.PrivateLinkServiceProperties{
			IPConfigurations: &[]network.PrivateLinkServiceIPConfiguration{
				{
					Name: ptr.To(fmt.Sprintf("%s-natipconfig-1", fakeSubnetName)),
					PrivateLinkServiceIPConfigurationProperties: &network.PrivateLinkServiceIPConfigurationProperties{
						Subnet: &network.Subnet{
							ID: ptr.To(
								fmt.Sprintf(
									"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/%s",
									fakeSubscriptionID1,
									fakeVNetResourceGroup,
									fakeVNetName,
									fakeSubnetName)),
						},
						PrivateIPAllocationMethod: network.Dynamic,
						Primary:                   ptr.To(true),
					},
				},
			},
			LoadBalancerFrontendIPConfigurations: &[]network.FrontendIPConfiguration{
				{
					ID: ptr.To(
						fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers/%s/frontendIPConfigurations/%s",
							fakeSubscriptionID1,
							fakeClusterName,
							fakeLbName,
							fakeLbIPConfigName1)),
				},
			},
			Visibility: &network.PrivateLinkServicePropertiesVisibility{
				Subscriptions: &[]string{
					fakeSubscriptionID1,
					fakeSubscriptionID2,
				},
			},
			AutoApproval: &network.PrivateLinkServicePropertiesAutoApproval{
				Subscriptions: &[]string{
					fakeSubscriptionID1,
				},
			},
			EnableProxyProtocol: ptr.To(false),
		},
		Tags: map[string]*string{
			"sigs.k8s.io_cluster-api-provider-azure_cluster_" + fakeClusterName: ptr.To("owned"),
			"Name":  ptr.To(fakePrivateLinkName),
			"hello": ptr.To("capz"),
		},
	}

	// fakePrivateLink3 is Azure PrivateLinkService that corresponds to fakePrivateLinkSpec3.
	fakePrivateLink3 = network.PrivateLinkService{
		Name:     ptr.To(fakePrivateLinkName),
		Location: ptr.To(fakeRegion),
		PrivateLinkServiceProperties: &network.PrivateLinkServiceProperties{
			IPConfigurations: &[]network.PrivateLinkServiceIPConfiguration{
				{
					Name: ptr.To(fmt.Sprintf("%s-natipconfig-1", fakeSubnetName)),
					PrivateLinkServiceIPConfigurationProperties: &network.PrivateLinkServiceIPConfigurationProperties{
						Subnet: &network.Subnet{
							ID: ptr.To(
								fmt.Sprintf(
									"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/%s",
									fakeSubscriptionID1,
									fakeVNetResourceGroup,
									fakeVNetName,
									fakeSubnetName)),
						},
						PrivateIPAllocationMethod: network.Dynamic,
						Primary:                   ptr.To(true),
					},
				},
			},
			LoadBalancerFrontendIPConfigurations: &[]network.FrontendIPConfiguration{
				{
					ID: ptr.To(
						fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers/%s/frontendIPConfigurations/%s",
							fakeSubscriptionID1,
							fakeClusterName,
							fakeLbName,
							fakeLbIPConfigName1)),
				},
			},
			Visibility: &network.PrivateLinkServicePropertiesVisibility{
				Subscriptions: &[]string{
					fakeSubscriptionID1,
					fakeSubscriptionID2,
				},
			},
			AutoApproval: &network.PrivateLinkServicePropertiesAutoApproval{
				Subscriptions: &[]string{
					fakeSubscriptionID1,
					fakeSubscriptionID2,
				},
			},
			EnableProxyProtocol: ptr.To(false),
		},
		Tags: map[string]*string{
			"sigs.k8s.io_cluster-api-provider-azure_cluster_" + fakeClusterName: ptr.To("owned"),
			"Name":  ptr.To(fakePrivateLinkName),
			"hello": ptr.To("capz"),
		},
	}

	// fakePrivateLink4 is Azure PrivateLinkService that corresponds to fakePrivateLinkSpec4.
	fakePrivateLink4 = network.PrivateLinkService{
		Name:     ptr.To(fakePrivateLinkName),
		Location: ptr.To(fakeRegion),
		PrivateLinkServiceProperties: &network.PrivateLinkServiceProperties{
			IPConfigurations: &[]network.PrivateLinkServiceIPConfiguration{
				{
					Name: ptr.To(fmt.Sprintf("%s-natipconfig-1", fakeSubnetName)),
					PrivateLinkServiceIPConfigurationProperties: &network.PrivateLinkServiceIPConfigurationProperties{
						Subnet: &network.Subnet{
							ID: ptr.To(
								fmt.Sprintf(
									"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/%s",
									fakeSubscriptionID1,
									fakeVNetResourceGroup,
									fakeVNetName,
									fakeSubnetName)),
						},
						PrivateIPAllocationMethod: network.Dynamic,
						Primary:                   ptr.To(true),
					},
				},
			},
			LoadBalancerFrontendIPConfigurations: &[]network.FrontendIPConfiguration{
				{
					ID: ptr.To(
						fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers/%s/frontendIPConfigurations/%s",
							fakeSubscriptionID1,
							fakeClusterName,
							fakeLbName,
							fakeLbIPConfigName1)),
				},
			},
			Visibility: &network.PrivateLinkServicePropertiesVisibility{
				Subscriptions: &[]string{
					fakeSubscriptionID1,
					fakeSubscriptionID2,
				},
			},
			AutoApproval: &network.PrivateLinkServicePropertiesAutoApproval{
				Subscriptions: &[]string{
					fakeSubscriptionID1,
					fakeSubscriptionID2,
				},
			},
			EnableProxyProtocol: ptr.To(true),
		},
		Tags: map[string]*string{
			"sigs.k8s.io_cluster-api-provider-azure_cluster_" + fakeClusterName: ptr.To("owned"),
			"Name":  ptr.To(fakePrivateLinkName),
			"hello": ptr.To("capz"),
		},
	}

	// fakePrivateLink5 is Azure PrivateLinkService that corresponds to fakePrivateLinkSpec5.
	fakePrivateLink5 = network.PrivateLinkService{
		Name:     ptr.To(fakePrivateLinkName),
		Location: ptr.To(fakeRegion),
		PrivateLinkServiceProperties: &network.PrivateLinkServiceProperties{
			IPConfigurations: &[]network.PrivateLinkServiceIPConfiguration{
				{
					Name: ptr.To(fmt.Sprintf("%s-natipconfig-1", fakeSubnetName)),
					PrivateLinkServiceIPConfigurationProperties: &network.PrivateLinkServiceIPConfigurationProperties{
						Subnet: &network.Subnet{
							ID: ptr.To(
								fmt.Sprintf(
									"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/%s",
									fakeSubscriptionID1,
									fakeVNetResourceGroup,
									fakeVNetName,
									fakeSubnetName)),
						},
						PrivateIPAllocationMethod: network.Dynamic,
						Primary:                   ptr.To(true),
					},
				},
			},
			LoadBalancerFrontendIPConfigurations: &[]network.FrontendIPConfiguration{
				{
					ID: ptr.To(
						fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers/%s/frontendIPConfigurations/%s",
							fakeSubscriptionID1,
							fakeClusterName,
							fakeLbName,
							fakeLbIPConfigName2)),
				},
			},
			Visibility: &network.PrivateLinkServicePropertiesVisibility{
				Subscriptions: &[]string{
					fakeSubscriptionID1,
					fakeSubscriptionID2,
				},
			},
			AutoApproval: &network.PrivateLinkServicePropertiesAutoApproval{
				Subscriptions: &[]string{
					fakeSubscriptionID1,
					fakeSubscriptionID2,
				},
			},
			EnableProxyProtocol: ptr.To(true),
		},
		Tags: map[string]*string{
			"sigs.k8s.io_cluster-api-provider-azure_cluster_" + fakeClusterName: ptr.To("owned"),
			"Name":  ptr.To(fakePrivateLinkName),
			"hello": ptr.To("capz"),
		},
	}
)

func TestBothPointerAreNil(t *testing.T) {
	testCases := []struct {
		name     string
		value1   *string
		value2   *string
		expected bool
	}{
		{
			name:     "Neither value is nil",
			value1:   ptr.To("a"),
			value2:   ptr.To("b"),
			expected: false,
		},
		{
			name:     "First value is nil",
			value1:   nil,
			value2:   ptr.To("b"),
			expected: false,
		},
		{
			name:     "Second value is nil",
			value1:   ptr.To("a"),
			value2:   nil,
			expected: false,
		},
		{
			name:     "Both values are nil",
			value1:   nil,
			value2:   nil,
			expected: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result := bothPointersAreNil[string](tc.value1, tc.value2)
			g.Expect(result).To(Equal(tc.expected))
		})
	}
}

func TestOnlyOnePointerIsNil(t *testing.T) {
	testCases := []struct {
		name     string
		value1   *string
		value2   *string
		expected bool
	}{
		{
			name:     "Neither value is nil",
			value1:   ptr.To("a"),
			value2:   ptr.To("b"),
			expected: false,
		},
		{
			name:     "First value is nil",
			value1:   nil,
			value2:   ptr.To("b"),
			expected: true,
		},
		{
			name:     "Second value is nil",
			value1:   ptr.To("a"),
			value2:   nil,
			expected: true,
		},
		{
			name:     "Both values are nil",
			value1:   nil,
			value2:   nil,
			expected: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result := onlyOnePointerIsNil[string](tc.value1, tc.value2)
			g.Expect(result).To(Equal(tc.expected))
		})
	}
}

func TestEqualStringSlicesIgnoreOrder(t *testing.T) {
	testCases := []struct {
		name     string
		value1   []string
		value2   []string
		expected bool
	}{
		{
			name:     "Nil slices",
			value1:   nil,
			value2:   nil,
			expected: true,
		},
		{
			name:     "Empty slices",
			value1:   []string{},
			value2:   []string{},
			expected: true,
		},
		{
			name:     "Nil and empty slice",
			value1:   nil,
			value2:   []string{},
			expected: true,
		},
		{
			name:     "Same slices with 1 element",
			value1:   []string{"a"},
			value2:   []string{"a"},
			expected: true,
		},
		{
			name:     "Same slices with 3 elements in same order",
			value1:   []string{"a", "b", "c"},
			value2:   []string{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "Same slices with 3 elements in different order",
			value1:   []string{"a", "b", "c"},
			value2:   []string{"c", "a", "b"},
			expected: true,
		},
		{
			name:     "Different slices with 1 element",
			value1:   []string{"a"},
			value2:   []string{"b"},
			expected: false,
		},
		{
			name:     "Different slices with 3 elements",
			value1:   []string{"a", "b", "c"},
			value2:   []string{"a", "b", "d"},
			expected: false,
		},
		{
			name:     "Slices with different lengths",
			value1:   []string{"a", "b", "c"},
			value2:   []string{"a", "b", "c", "d"},
			expected: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result := equalStringSlicesIgnoreOrder(tc.value1, tc.value2)
			g.Expect(result).To(Equal(tc.expected))
		})
	}
}

func TestEqualStringSlicesPtrIgnoreOrder(t *testing.T) {
	testCases := []struct {
		name     string
		value1   *[]string
		value2   *[]string
		expected bool
	}{
		{
			name:     "Nil slices",
			value1:   nil,
			value2:   nil,
			expected: true,
		},
		{
			name:     "Empty slices",
			value1:   &[]string{},
			value2:   &[]string{},
			expected: true,
		},
		{
			name:     "Nil and empty slice",
			value1:   nil,
			value2:   &[]string{},
			expected: true,
		},
		{
			name:     "Same slices with 1 element",
			value1:   &[]string{"a"},
			value2:   &[]string{"a"},
			expected: true,
		},
		{
			name:     "Same slices with 3 elements in same order",
			value1:   &[]string{"a", "b", "c"},
			value2:   &[]string{"a", "b", "c"},
			expected: true,
		},
		{
			name:     "Same slices with 3 elements in different order",
			value1:   &[]string{"a", "b", "c"},
			value2:   &[]string{"c", "a", "b"},
			expected: true,
		},
		{
			name:     "Different slices with 1 element",
			value1:   &[]string{"a"},
			value2:   &[]string{"b"},
			expected: false,
		},
		{
			name:     "Different slices with 3 elements",
			value1:   &[]string{"a", "b", "c"},
			value2:   &[]string{"a", "b", "d"},
			expected: false,
		},
		{
			name:     "Slices with different lengths",
			value1:   &[]string{"a", "b", "c"},
			value2:   &[]string{"a", "b", "c", "d"},
			expected: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result := equalStringSlicesPtrIgnoreOrder(tc.value1, tc.value2)
			g.Expect(result).To(Equal(tc.expected))
		})
	}
}

func TestParameters(t *testing.T) {
	testcases := []struct {
		name          string
		spec          PrivateLinkSpec
		existing      interface{}
		expect        func(g *WithT, result interface{})
		expectedError string
	}{
		{
			name:     "PrivateLink does not exist",
			spec:     fakePrivateLinkSpec1,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(network.PrivateLinkService{}))
				g.Expect(result).To(Equal(fakePrivateLink1))
			},
		},
		{
			name:     "PrivateLink already exists with the same config",
			spec:     fakePrivateLinkSpec1,
			existing: fakePrivateLink1,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
		},
		{
			name:     "PrivateLink changed and added one new allowed subscription",
			spec:     fakePrivateLinkSpec2, // spec with 2 allowed subscriptions
			existing: fakePrivateLink1,     // existing private link with 1 allowed subscription
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(network.PrivateLinkService{}))
				g.Expect(result).To(Equal(fakePrivateLink2)) // expects (updated) private link with 2 allowed subscriptions
			},
		},
		{
			name:     "PrivateLink changed and added one new auto-approved subscription",
			spec:     fakePrivateLinkSpec3, // spec with 2 auto-approved subscriptions
			existing: fakePrivateLink2,     // existing private link with 1 auto-approved subscription
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(network.PrivateLinkService{}))
				g.Expect(result).To(Equal(fakePrivateLink3)) // expects (updated) private link with 2 auto-approved subscriptions
			},
		},
		{
			name:     "PrivateLink changed and enabled proxy protocol",
			spec:     fakePrivateLinkSpec4, // spec with enabled proxy protocol
			existing: fakePrivateLink3,     // existing private link with disabled proxy protocol
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(network.PrivateLinkService{}))
				g.Expect(result).To(Equal(fakePrivateLink4)) // expects (updated) private link with enabled proxy protocol
			},
		},
		{
			name:     "PrivateLink changed LB frontend config name",
			spec:     fakePrivateLinkSpec5, // spec with changed LB frontend config name
			existing: fakePrivateLink4,     // existing private link with old LB frontend config name
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(network.PrivateLinkService{}))
				g.Expect(result).To(Equal(fakePrivateLink5)) // expects (updated) private link with changed LB frontend config name
			},
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

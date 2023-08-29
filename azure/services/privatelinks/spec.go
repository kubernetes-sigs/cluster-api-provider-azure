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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2022-07-01/network"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
)

// PrivateLinkSpec defines the specification for a private link service.
type PrivateLinkSpec struct {
	Name                      string
	ResourceGroup             string
	SubscriptionID            string
	Location                  string
	VNetResourceGroup         string
	VNet                      string
	NATIPConfiguration        []NATIPConfiguration
	LoadBalancerName          string
	LBFrontendIPConfigNames   []string
	AllowedSubscriptions      []string
	AutoApprovedSubscriptions []string
	EnableProxyProtocol       *bool
	ClusterName               string
	AdditionalTags            infrav1.Tags
}

// NATIPConfiguration defines the NAT IP configuration for the private link service.
type NATIPConfiguration struct {
	// AllocationMethod can be Static or Dynamic.
	AllocationMethod string

	// Subnet from the VNet from which the IP is allocated.
	Subnet string

	// PrivateIPAddress is the optional static private IP address from the specified Subnet.
	PrivateIPAddress string
}

// ResourceName returns the name of the NAT gateway.
func (s *PrivateLinkSpec) ResourceName() string {
	return s.Name
}

// ResourceGroupName returns the name of the resource group.
func (s *PrivateLinkSpec) ResourceGroupName() string {
	return s.ResourceGroup
}

// OwnerResourceName is a no-op for NAT gateways.
func (s *PrivateLinkSpec) OwnerResourceName() string {
	return ""
}

// Parameters returns the parameters for the NAT gateway.
func (s *PrivateLinkSpec) Parameters(ctx context.Context, existing interface{}) (params interface{}, err error) {
	if len(s.NATIPConfiguration) == 0 {
		return nil, errors.Errorf("At least one private link NAT IP configuration must be specified")
	}
	if len(s.LBFrontendIPConfigNames) == 0 {
		return nil, errors.Errorf("At least one load balancer front end name must be specified")
	}

	// NAT IP configurations
	ipConfigurations := make([]network.PrivateLinkServiceIPConfiguration, 0, len(s.NATIPConfiguration))
	for i, natIPConfiguration := range s.NATIPConfiguration {
		ipAllocationMethod := network.IPAllocationMethod(natIPConfiguration.AllocationMethod)
		if ipAllocationMethod != network.Dynamic && ipAllocationMethod != network.Static {
			return nil, errors.Errorf("%T is not a supported network.IPAllocationMethodStatic", natIPConfiguration.AllocationMethod)
		}
		var privateIPAddress *string
		if ipAllocationMethod == network.Static {
			if natIPConfiguration.PrivateIPAddress != "" {
				privateIPAddress = ptr.To(natIPConfiguration.PrivateIPAddress)
			} else {
				return nil, errors.Errorf("Private link NAT IP configuration with static IP allocation must specify a private address")
			}
		}
		ipConfiguration := network.PrivateLinkServiceIPConfiguration{
			Name: ptr.To(fmt.Sprintf("%s-natipconfig-%d", natIPConfiguration.Subnet, i+1)),
			PrivateLinkServiceIPConfigurationProperties: &network.PrivateLinkServiceIPConfigurationProperties{
				Subnet: &network.Subnet{
					ID: ptr.To(azure.SubnetID(s.SubscriptionID, s.VNetResourceGroup, s.VNet, natIPConfiguration.Subnet)),
				},
				PrivateIPAllocationMethod: ipAllocationMethod,
				PrivateIPAddress:          privateIPAddress,
			},
		}
		ipConfigurations = append(ipConfigurations, ipConfiguration)
		ipConfigurations[0].Primary = ptr.To(true)
	}

	// Load balancer front-end IP configurations
	frontendIPConfigurations := make([]network.FrontendIPConfiguration, 0, len(s.LBFrontendIPConfigNames))
	for _, frontendIPConfigName := range s.LBFrontendIPConfigNames {
		frontendIPConfig := network.FrontendIPConfiguration{
			ID: ptr.To(azure.FrontendIPConfigID(s.SubscriptionID, s.ResourceGroupName(), s.LoadBalancerName, frontendIPConfigName)),
		}
		frontendIPConfigurations = append(frontendIPConfigurations, frontendIPConfig)
	}

	privateLinkToCreate := network.PrivateLinkService{
		Name:     ptr.To(s.Name),
		Location: ptr.To(s.Location),
		PrivateLinkServiceProperties: &network.PrivateLinkServiceProperties{
			IPConfigurations:                     &ipConfigurations,
			LoadBalancerFrontendIPConfigurations: &frontendIPConfigurations,
			EnableProxyProtocol:                  s.EnableProxyProtocol,
		},
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: s.ClusterName,
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Name:        ptr.To(s.Name),
			Additional:  s.AdditionalTags,
		})),
	}

	if len(s.AllowedSubscriptions) > 0 {
		privateLinkToCreate.Visibility = &network.PrivateLinkServicePropertiesVisibility{
			Subscriptions: &s.AllowedSubscriptions,
		}
	}
	if len(s.AutoApprovedSubscriptions) > 0 {
		privateLinkToCreate.AutoApproval = &network.PrivateLinkServicePropertiesAutoApproval{
			Subscriptions: &s.AutoApprovedSubscriptions,
		}
	}

	if existing != nil {
		existingPrivateLink, ok := existing.(network.PrivateLinkService)
		if !ok {
			return nil, errors.Errorf("%T is not a network.PrivateLinkService", existing)
		}

		if isExistingUpToDate(existingPrivateLink, privateLinkToCreate) {
			return nil, nil
		}
	}

	return privateLinkToCreate, nil
}

func isExistingUpToDate(existing network.PrivateLinkService, wanted network.PrivateLinkService) bool {
	// NAT IP configuration is not checked as it cannot be changed.

	// Check load balancer configurations
	wantedFrontendIDs := make([]string, len(*wanted.LoadBalancerFrontendIPConfigurations))
	for _, wantedFrontendIPConfig := range *wanted.LoadBalancerFrontendIPConfigurations {
		wantedFrontendIDs = append(wantedFrontendIDs, *wantedFrontendIPConfig.ID)
	}
	existingFrontendIDs := make([]string, len(*existing.LoadBalancerFrontendIPConfigurations))
	for _, existingFrontendIPConfig := range *existing.LoadBalancerFrontendIPConfigurations {
		existingFrontendIDs = append(existingFrontendIDs, *existingFrontendIPConfig.ID)
	}
	if !equalStringSlicesIgnoreOrder(wantedFrontendIDs, existingFrontendIDs) {
		return false
	}

	// Check proxy protocol config
	if !ptr.Equal(wanted.EnableProxyProtocol, existing.EnableProxyProtocol) {
		return false
	}

	// Check allowed subscriptions
	if !equalStructWithStringSlicesPtrIgnoreOrder(
		wanted.Visibility,
		existing.Visibility,
		func(v *network.PrivateLinkServicePropertiesVisibility) *[]string {
			return v.Subscriptions
		}) {
		return false
	}

	// Check auto-approved subscriptions
	if !equalStructWithStringSlicesPtrIgnoreOrder(
		wanted.AutoApproval,
		existing.AutoApproval,
		func(v *network.PrivateLinkServicePropertiesAutoApproval) *[]string {
			return v.Subscriptions
		}) {
		return false
	}

	return true
}

func equalStructWithStringSlicesPtrIgnoreOrder[T any](a, b *T, getStringSlice func(*T) *[]string) bool {
	if bothPointersAreNil(a, b) {
		return true
	}
	if onlyOnePointerIsNil(a, b) {
		return false
	}

	return equalStringSlicesPtrIgnoreOrder(getStringSlice(a), getStringSlice(b))
}

func equalStringSlicesPtrIgnoreOrder(a, b *[]string) bool {
	// normalize nil to empty slice pointer
	if a == nil {
		a = &[]string{}
	}
	if b == nil {
		b = &[]string{}
	}

	// both are different from nil
	return equalStringSlicesIgnoreOrder(*a, *b)
}

func equalStringSlicesIgnoreOrder(a, b []string) bool {
	// manual length check, so we return true for any combination of nil and
	// empty slices in the parameters
	if len(a) == 0 && len(b) == 0 {
		return true
	}

	slicesDiff := cmp.Diff(a, b, cmpopts.SortSlices(func(s1, s2 string) bool {
		return s1 < s2
	}))

	return slicesDiff == ""
}

func bothPointersAreNil[T any](a, b *T) bool {
	return a == nil && b == nil
}

func onlyOnePointerIsNil[T any](a, b *T) bool {
	return a == nil && b != nil || a != nil && b == nil
}

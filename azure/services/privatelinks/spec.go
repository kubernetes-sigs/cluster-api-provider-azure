package privatelinks

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2022-07-01/network"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/pkg/errors"
	"k8s.io/utils/pointer"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
)

type PrivateLinkSpec struct {
	Name                      string
	ResourceGroup             string
	SubscriptionID            string
	Location                  string
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
	var ipConfigurations []network.PrivateLinkServiceIPConfiguration
	for i, natIPConfiguration := range s.NATIPConfiguration {
		ipAllocationMethod := network.IPAllocationMethod(natIPConfiguration.AllocationMethod)
		if ipAllocationMethod != network.Dynamic && ipAllocationMethod != network.Static {
			return nil, errors.Errorf("%T is not a supported network.IPAllocationMethodStatic", natIPConfiguration.AllocationMethod)
		}
		var privateIpAddress *string
		if ipAllocationMethod == network.Static {
			if natIPConfiguration.PrivateIPAddress != "" {
				privateIpAddress = pointer.String(natIPConfiguration.PrivateIPAddress)
			} else {
				return nil, errors.Errorf("Private link NAT IP configuration with static IP allocation must specify a private address")
			}
		}
		ipConfiguration := network.PrivateLinkServiceIPConfiguration{
			Name: pointer.String(fmt.Sprintf("%s-natipconfig-%d", natIPConfiguration.Subnet, i+1)),
			PrivateLinkServiceIPConfigurationProperties: &network.PrivateLinkServiceIPConfigurationProperties{
				Subnet: &network.Subnet{
					ID: pointer.String(
						fmt.Sprintf(
							"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/%s",
							s.SubscriptionID,
							s.ResourceGroup,
							s.VNet,
							natIPConfiguration.Subnet)),
				},
				PrivateIPAllocationMethod: ipAllocationMethod,
				PrivateIPAddress:          privateIpAddress,
			},
		}
		ipConfigurations = append(ipConfigurations, ipConfiguration)
		ipConfigurations[0].Primary = pointer.Bool(true)
	}

	// Load balancer front-end IP configurations
	var frontendIPConfigurations []network.FrontendIPConfiguration
	for _, frontendIPConfigName := range s.LBFrontendIPConfigNames {
		frontendIPConfig := network.FrontendIPConfiguration{
			ID: pointer.String(
				fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers/%s/frontendIPConfigurations/%s",
					s.SubscriptionID,
					s.ResourceGroupName(),
					s.LoadBalancerName,
					frontendIPConfigName)),
		}
		frontendIPConfigurations = append(frontendIPConfigurations, frontendIPConfig)
	}

	privateLinkToCreate := network.PrivateLinkService{
		Name:     pointer.String(s.Name),
		Location: pointer.String(s.Location),
		PrivateLinkServiceProperties: &network.PrivateLinkServiceProperties{
			IPConfigurations:                     &ipConfigurations,
			LoadBalancerFrontendIPConfigurations: &frontendIPConfigurations,
			EnableProxyProtocol:                  s.EnableProxyProtocol,
		},
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: s.ClusterName,
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Name:        pointer.String(s.Name),
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
	var wantedFrontendIDs []string
	for _, wantedFrontendIPConfig := range *wanted.LoadBalancerFrontendIPConfigurations {
		wantedFrontendIDs = append(wantedFrontendIDs, *wantedFrontendIPConfig.ID)
	}
	var existingFrontendIDs []string
	for _, existingFrontendIPConfig := range *existing.LoadBalancerFrontendIPConfigurations {
		existingFrontendIDs = append(existingFrontendIDs, *existingFrontendIPConfig.ID)
	}
	if !equalStringSlicesIgnoreOrder(wantedFrontendIDs, existingFrontendIDs) {
		return false
	}

	// Check proxy protocol config
	if !pointer.BoolEqual(wanted.EnableProxyProtocol, existing.EnableProxyProtocol) {
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
	if bothPointersAreNil(a, b) {
		return true
	}
	if onlyOnePointerIsNil(a, b) {
		return false
	}

	// both are different from nil
	return equalStringSlicesIgnoreOrder(*a, *b)
}

func equalStringSlicesIgnoreOrder(a, b []string) bool {
	slicesDiff := cmp.Diff(a, b, cmpopts.SortSlices(func(s1, s2 string) bool {
		return s1 < s2
	}))

	return slicesDiff == ""
}

func bothPointersAreNil[T any](a, b *T) bool {
	return a == nil && b == nil
}

func onlyOnePointerIsNil[T any](a, b *T) bool {
	return a == nil && b != nil || a != nil || b == nil
}

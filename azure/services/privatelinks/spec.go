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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
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
	AllowedSubscriptions      []*string
	AutoApprovedSubscriptions []*string
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

// ResourceName returns the name of the private link.
func (s *PrivateLinkSpec) ResourceName() string {
	return s.Name
}

// ResourceGroupName returns the name of the resource group.
func (s *PrivateLinkSpec) ResourceGroupName() string {
	return s.ResourceGroup
}

// OwnerResourceName is a no-op for private link.
func (s *PrivateLinkSpec) OwnerResourceName() string {
	return ""
}

// Parameters returns the parameters for the private link.
func (s *PrivateLinkSpec) Parameters(_ context.Context, existing any) (params any, err error) {
	if existing != nil {
		// Private link already exist, so we have to check if it should be updated.
		existingPrivateLink, ok := existing.(armnetwork.PrivateLinkService)
		if !ok {
			return nil, errors.Errorf("%T is not a armnetwork.PrivateLinkService", existing)
		}

		privateLinkToCreate, err := s.constructParameters()
		if err != nil {
			return nil, err
		}

		if isExistingUpToDate(existingPrivateLink, privateLinkToCreate) {
			// Existing private link is up-to-date.
			return nil, nil
		}

		// Existing private link is outdated, we return new updated parameters.
		return privateLinkToCreate, nil
	}

	// Private link does not exist, so we create it here.
	privateLinkToCreate, err := s.constructParameters()
	if err != nil {
		return nil, err
	}

	return privateLinkToCreate, nil
}

func (s *PrivateLinkSpec) constructParameters() (params armnetwork.PrivateLinkService, err error) {
	if len(s.NATIPConfiguration) == 0 {
		return armnetwork.PrivateLinkService{}, errors.Errorf("At least one private link NAT IP configuration must be specified")
	}
	if len(s.LBFrontendIPConfigNames) == 0 {
		return armnetwork.PrivateLinkService{}, errors.Errorf("At least one load balancer front end name must be specified")
	}

	// NAT IP configurations
	ipConfigurations := make([]*armnetwork.PrivateLinkServiceIPConfiguration, 0, len(s.NATIPConfiguration))
	for i, natIPConfiguration := range s.NATIPConfiguration {
		ipAllocationMethod := armnetwork.IPAllocationMethod(natIPConfiguration.AllocationMethod)
		if ipAllocationMethod != armnetwork.IPAllocationMethodDynamic && ipAllocationMethod != armnetwork.IPAllocationMethodStatic {
			return armnetwork.PrivateLinkService{}, errors.Errorf("%T is not a supported armnetwork.IPAllocationMethodStatic", natIPConfiguration.AllocationMethod)
		}
		var privateIPAddress *string
		if ipAllocationMethod == armnetwork.IPAllocationMethodStatic {
			if natIPConfiguration.PrivateIPAddress != "" {
				privateIPAddress = ptr.To(natIPConfiguration.PrivateIPAddress)
			} else {
				return armnetwork.PrivateLinkService{}, errors.Errorf("Private link NAT IP configuration with static IP allocation must specify a private address")
			}
		}
		ipConfiguration := armnetwork.PrivateLinkServiceIPConfiguration{
			Name: ptr.To(fmt.Sprintf("%s-natipconfig-%d", natIPConfiguration.Subnet, i+1)),
			Properties: &armnetwork.PrivateLinkServiceIPConfigurationProperties{
				Subnet: &armnetwork.Subnet{
					ID: ptr.To(azure.SubnetID(s.SubscriptionID, s.VNetResourceGroup, s.VNet, natIPConfiguration.Subnet)),
				},
				PrivateIPAllocationMethod: &ipAllocationMethod,
				PrivateIPAddress:          privateIPAddress,
			},
		}
		ipConfigurations = append(ipConfigurations, &ipConfiguration)
		ipConfigurations[0].Properties.Primary = ptr.To(true)
	}

	// Load balancer front-end IP configurations
	frontendIPConfigurations := make([]*armnetwork.FrontendIPConfiguration, 0, len(s.LBFrontendIPConfigNames))
	for _, frontendIPConfigName := range s.LBFrontendIPConfigNames {
		frontendIPConfig := armnetwork.FrontendIPConfiguration{
			ID: ptr.To(azure.FrontendIPConfigID(s.SubscriptionID, s.ResourceGroupName(), s.LoadBalancerName, frontendIPConfigName)),
		}
		frontendIPConfigurations = append(frontendIPConfigurations, &frontendIPConfig)
	}

	privateLinkToCreate := armnetwork.PrivateLinkService{
		Name:     ptr.To(s.Name),
		Location: ptr.To(s.Location),
		Properties: &armnetwork.PrivateLinkServiceProperties{
			IPConfigurations:                     ipConfigurations,
			LoadBalancerFrontendIPConfigurations: frontendIPConfigurations,
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
		privateLinkToCreate.Properties.Visibility = &armnetwork.PrivateLinkServicePropertiesVisibility{
			Subscriptions: s.AllowedSubscriptions,
		}
	}
	if len(s.AutoApprovedSubscriptions) > 0 {
		privateLinkToCreate.Properties.AutoApproval = &armnetwork.PrivateLinkServicePropertiesAutoApproval{
			Subscriptions: s.AutoApprovedSubscriptions,
		}
	}

	return privateLinkToCreate, nil
}

func isExistingUpToDate(existing armnetwork.PrivateLinkService, wanted armnetwork.PrivateLinkService) bool {
	// NAT IP configuration is not checked as it cannot be changed.

	// Check load balancer configurations
	wantedFrontendIDs := make([]*string, len(wanted.Properties.LoadBalancerFrontendIPConfigurations))
	for _, wantedFrontendIPConfig := range wanted.Properties.LoadBalancerFrontendIPConfigurations {
		wantedFrontendIDs = append(wantedFrontendIDs, wantedFrontendIPConfig.ID)
	}
	existingFrontendIDs := make([]*string, len(existing.Properties.LoadBalancerFrontendIPConfigurations))
	for _, existingFrontendIPConfig := range existing.Properties.LoadBalancerFrontendIPConfigurations {
		existingFrontendIDs = append(existingFrontendIDs, existingFrontendIPConfig.ID)
	}
	if !compareStringPointerSlicesUnordered(wantedFrontendIDs, existingFrontendIDs) {
		return false
	}

	// Check proxy protocol config
	if !ptr.Equal(wanted.Properties.EnableProxyProtocol, existing.Properties.EnableProxyProtocol) {
		return false
	}

	// Check allowed subscriptions
	if !compareStringPointerSlicesUnordered(
		wanted.Properties.Visibility.Subscriptions,
		existing.Properties.Visibility.Subscriptions) {
		return false
	}

	// Check auto-approved subscriptions
	if !compareStringPointerSlicesUnordered(
		wanted.Properties.AutoApproval.Subscriptions,
		existing.Properties.AutoApproval.Subscriptions) {
		return false
	}

	return true
}

func compareStringPointerSlicesUnordered(a, b []*string) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]struct{}, len(a))
	for _, x := range a {
		if x == nil {
			continue
		}
		m[*x] = struct{}{}
	}
	for _, y := range b {
		if y == nil {
			continue
		}
		if _, ok := m[*y]; !ok {
			return false
		}
	}
	return true
}

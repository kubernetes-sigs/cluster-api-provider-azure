/*
Copyright The Kubernetes Authors.

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

	asonetworkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20220701"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
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
	AllocationMethod infrav1.PrivateLinkNATIPAllocationMethod

	// Subnet from the VNet from which the IP is allocated.
	Subnet string

	// PrivateIPAddress is the optional static private IP address from the specified Subnet.
	PrivateIPAddress string
}

func (s *PrivateLinkSpec) ResourceRef() *asonetworkv1.PrivateLinkService {
	return &asonetworkv1.PrivateLinkService{
		ObjectMeta: metav1.ObjectMeta{
			Name: azure.GetNormalizedKubernetesName(s.Name),
		},
	}
}

// Parameters returns the parameters for the private link.
func (s *PrivateLinkSpec) Parameters(_ context.Context, existingPrivateLink *asonetworkv1.PrivateLinkService) (*asonetworkv1.PrivateLinkService, error) {
	// Private link already exists, so we have to check if it should be updated.
	if existingPrivateLink != nil {
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

func (s *PrivateLinkSpec) constructParameters() (params *asonetworkv1.PrivateLinkService, err error) {
	if len(s.NATIPConfiguration) == 0 {
		return nil, errors.Errorf("At least one private link NAT IP configuration must be specified")
	}
	if len(s.LBFrontendIPConfigNames) == 0 {
		return nil, errors.Errorf("At least one load balancer front end name must be specified")
	}

	// NAT IP configurations
	ipConfigurations := make([]asonetworkv1.PrivateLinkServiceIpConfiguration, 0, len(s.NATIPConfiguration))
	for i, natIPConfiguration := range s.NATIPConfiguration {
		ipAllocationMethod := asonetworkv1.IPAllocationMethod(natIPConfiguration.AllocationMethod)
		if ipAllocationMethod != asonetworkv1.IPAllocationMethod_Dynamic && ipAllocationMethod != asonetworkv1.IPAllocationMethod_Static {
			return nil, errors.Errorf("%q is not a supported NAT IP allocation method (must be %q or %q)",
				natIPConfiguration.AllocationMethod, infrav1.NATIPAllocationMethodStatic, infrav1.NATIPAllocationMethodDynamic)
		}

		var privateIPAddress *string
		if ipAllocationMethod == asonetworkv1.IPAllocationMethod_Static {
			if natIPConfiguration.PrivateIPAddress != "" {
				privateIPAddress = ptr.To(natIPConfiguration.PrivateIPAddress)
			} else {
				return nil, errors.Errorf("Private link NAT IP configuration with static IP allocation must specify a private address")
			}
		}

		ipConfiguration := asonetworkv1.PrivateLinkServiceIpConfiguration{
			Name: ptr.To(fmt.Sprintf("%s-natipconfig-%d", natIPConfiguration.Subnet, i+1)),
			Subnet: &asonetworkv1.Subnet_PrivateLinkService_SubResourceEmbedded{
				Reference: &genruntime.ResourceReference{
					ARMID: azure.SubnetID(s.SubscriptionID, s.VNetResourceGroup, s.VNet, natIPConfiguration.Subnet),
				},
			},
			PrivateIPAllocationMethod: &ipAllocationMethod,
			PrivateIPAddress:          privateIPAddress,
		}
		ipConfigurations = append(ipConfigurations, ipConfiguration)
		ipConfigurations[0].Primary = ptr.To(true)
	}

	// Load balancer front-end IP configurations
	frontendIPConfigurations := make([]asonetworkv1.FrontendIPConfiguration_PrivateLinkService_SubResourceEmbedded, 0, len(s.LBFrontendIPConfigNames))
	for _, frontendIPConfigName := range s.LBFrontendIPConfigNames {
		frontendIPConfig := asonetworkv1.FrontendIPConfiguration_PrivateLinkService_SubResourceEmbedded{
			Reference: &genruntime.ResourceReference{
				ARMID: azure.FrontendIPConfigID(s.SubscriptionID, s.ResourceGroup, s.LoadBalancerName, frontendIPConfigName),
			},
		}
		frontendIPConfigurations = append(frontendIPConfigurations, frontendIPConfig)
	}

	privateLinkToCreate := &asonetworkv1.PrivateLinkService{
		Spec: asonetworkv1.PrivateLinkService_Spec{
			AzureName:                            s.Name,
			Location:                             ptr.To(s.Location),
			IpConfigurations:                     ipConfigurations,
			LoadBalancerFrontendIpConfigurations: frontendIPConfigurations,
			EnableProxyProtocol:                  s.EnableProxyProtocol,
			Owner: &genruntime.KnownResourceReference{
				Name: azure.GetNormalizedKubernetesName(s.ResourceGroup),
			},
			Tags: infrav1.Build(infrav1.BuildParams{
				ClusterName: s.ClusterName,
				Lifecycle:   infrav1.ResourceLifecycleOwned,
				Name:        ptr.To(s.Name),
				Additional:  s.AdditionalTags,
			}),
		},
	}

	if len(s.AllowedSubscriptions) > 0 {
		privateLinkToCreate.Spec.Visibility = &asonetworkv1.ResourceSet{
			Subscriptions: s.AllowedSubscriptions,
		}
	}
	if len(s.AutoApprovedSubscriptions) > 0 {
		privateLinkToCreate.Spec.AutoApproval = &asonetworkv1.ResourceSet{
			Subscriptions: s.AutoApprovedSubscriptions,
		}
	}

	return privateLinkToCreate, nil
}

func isExistingUpToDate(existing *asonetworkv1.PrivateLinkService, wanted *asonetworkv1.PrivateLinkService) bool {
	// NAT IP configuration is not checked as it cannot be changed.

	// Check load balancer configurations
	wantedFrontendIDs := make([]string, len(wanted.Spec.LoadBalancerFrontendIpConfigurations))
	for _, wantedFrontendIPConfig := range wanted.Spec.LoadBalancerFrontendIpConfigurations {
		wantedFrontendIDs = append(wantedFrontendIDs, wantedFrontendIPConfig.Reference.ARMID)
	}

	existingFrontendIDs := make([]string, len(existing.Spec.LoadBalancerFrontendIpConfigurations))
	for _, existingFrontendIPConfig := range existing.Spec.LoadBalancerFrontendIpConfigurations {
		existingFrontendIDs = append(existingFrontendIDs, existingFrontendIPConfig.Reference.ARMID)
	}

	if !compareStringSlicesUnordered(wantedFrontendIDs, existingFrontendIDs) {
		return false
	}

	// Check proxy protocol config
	if !ptr.Equal(wanted.Spec.EnableProxyProtocol, existing.Spec.EnableProxyProtocol) {
		return false
	}

	// Check allowed subscriptions
	if !compareStringSlicesUnordered(
		visibilitySubscriptionsOrNil(wanted.Spec),
		visibilitySubscriptionsOrNil(existing.Spec)) {
		return false
	}

	// Check auto-approved subscriptions
	if !compareStringSlicesUnordered(
		autoApprovalSubscriptionsOrNil(wanted.Spec),
		autoApprovalSubscriptionsOrNil(existing.Spec)) {
		return false
	}

	return true
}

func compareStringSlicesUnordered(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]struct{}, len(a))
	for _, x := range a {
		if x == "" {
			continue
		}
		m[x] = struct{}{}
	}
	for _, y := range b {
		if y == "" {
			continue
		}
		if _, ok := m[y]; !ok {
			return false
		}
	}
	return true
}

func visibilitySubscriptionsOrNil(p asonetworkv1.PrivateLinkService_Spec) []string {
	if p.Visibility == nil {
		return nil
	}

	return p.Visibility.Subscriptions
}

func autoApprovalSubscriptionsOrNil(p asonetworkv1.PrivateLinkService_Spec) []string {
	if p.AutoApproval == nil {
		return nil
	}

	return p.AutoApproval.Subscriptions
}

// WasManaged implements azure.ASOResourceSpecGetter.
// It always returns true since CAPZ doesn't support BYO private links.
func (s *PrivateLinkSpec) WasManaged(_ *asonetworkv1.PrivateLinkService) bool {
	return true
}

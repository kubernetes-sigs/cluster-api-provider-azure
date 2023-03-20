/*
Copyright 2021 The Kubernetes Authors.

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

package natgateways

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"github.com/pkg/errors"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
)

// NatGatewaySpec defines the specification for a NAT gateway.
type NatGatewaySpec struct {
	Name           string
	ResourceGroup  string
	SubscriptionID string
	Location       string
	NatGatewayIP   infrav1.PublicIPSpec
	ClusterName    string
	AdditionalTags infrav1.Tags
}

// ResourceName returns the name of the NAT gateway.
func (s *NatGatewaySpec) ResourceName() string {
	return s.Name
}

// ResourceGroupName returns the name of the resource group.
func (s *NatGatewaySpec) ResourceGroupName() string {
	return s.ResourceGroup
}

// OwnerResourceName is a no-op for NAT gateways.
func (s *NatGatewaySpec) OwnerResourceName() string {
	return ""
}

// Parameters returns the parameters for the NAT gateway.
func (s *NatGatewaySpec) Parameters(ctx context.Context, existing interface{}) (params interface{}, err error) {
	if existing != nil {
		existingNatGateway, ok := existing.(network.NatGateway)
		if !ok {
			return nil, errors.Errorf("%T is not a network.NatGateway", existing)
		}

		if hasPublicIP(existingNatGateway, s.NatGatewayIP.Name) {
			// Skip update for NAT gateway as it exists with expected values
			return nil, nil
		}
	}

	natGatewayToCreate := network.NatGateway{
		Name:     pointer.String(s.Name),
		Location: pointer.String(s.Location),
		Sku:      &network.NatGatewaySku{Name: network.NatGatewaySkuNameStandard},
		NatGatewayPropertiesFormat: &network.NatGatewayPropertiesFormat{
			PublicIPAddresses: &[]network.SubResource{
				{
					ID: pointer.String(azure.PublicIPID(s.SubscriptionID, s.ResourceGroupName(), s.NatGatewayIP.Name)),
				},
			},
		},
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: s.ClusterName,
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Name:        pointer.String(s.Name),
			Additional:  s.AdditionalTags,
		})),
	}

	return natGatewayToCreate, nil
}

func hasPublicIP(natGateway network.NatGateway, publicIPName string) bool {
	// We must have a non-nil, non-"empty" PublicIPAddresses
	if !(natGateway.PublicIPAddresses != nil && len(*natGateway.PublicIPAddresses) > 0) {
		return false
	}

	for _, publicIP := range *natGateway.PublicIPAddresses {
		resource, err := arm.ParseResourceID(*publicIP.ID)
		if err != nil {
			continue
		}
		if resource.Name == publicIPName {
			return true
		}
	}
	return false
}

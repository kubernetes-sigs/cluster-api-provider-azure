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
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	autorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

// NatGatewaySpec defines the specification for a NAT gateway.
type NatGatewaySpec struct {
	Name           string
	ResourceGroup  string
	SubscriptionID string
	Location       string
	NatGatewayIP   infrav1.PublicIPSpec
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
func (s *NatGatewaySpec) Parameters(existing interface{}) (params interface{}, err error) {
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
		Name:     to.StringPtr(s.Name),
		Location: to.StringPtr(s.Location),
		Sku:      &network.NatGatewaySku{Name: network.NatGatewaySkuNameStandard},
		NatGatewayPropertiesFormat: &network.NatGatewayPropertiesFormat{
			PublicIPAddresses: &[]network.SubResource{
				{
					ID: to.StringPtr(azure.PublicIPID(s.SubscriptionID, s.ResourceGroupName(), s.NatGatewayIP.Name)),
				},
			},
		},
	}

	return natGatewayToCreate, nil
}

func hasPublicIP(natGateway network.NatGateway, publicIPName string) bool {
	// We must have a non-nil, non-"empty" PublicIPAddresses
	if !(natGateway.PublicIPAddresses != nil && len(*natGateway.PublicIPAddresses) > 0) {
		return false
	}

	for _, publicIP := range *natGateway.PublicIPAddresses {
		resource, err := autorest.ParseResourceID(*publicIP.ID)
		if err != nil {
			continue
		}
		if resource.ResourceName == publicIPName {
			return true
		}
	}
	return false
}

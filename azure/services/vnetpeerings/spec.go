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

package vnetpeerings

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"github.com/pkg/errors"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

// VnetPeeringSpec defines the specification for a virtual network peering.
type VnetPeeringSpec struct {
	SourceResourceGroup string
	SourceVnetName      string
	RemoteResourceGroup string
	RemoteVnetName      string
	PeeringName         string
	SubscriptionID      string
}

// ResourceName returns the name of the virtual network peering.
func (s *VnetPeeringSpec) ResourceName() string {
	return s.PeeringName
}

// ResourceGroupName returns the name of the resource group.
func (s *VnetPeeringSpec) ResourceGroupName() string {
	return s.SourceResourceGroup
}

// OwnerResourceName is a no-op for virtual network peerings.
func (s *VnetPeeringSpec) OwnerResourceName() string {
	return s.SourceVnetName
}

// Parameters returns the parameters for the virtual network peering.
func (s *VnetPeeringSpec) Parameters(ctx context.Context, existing interface{}) (params interface{}, err error) {
	if existing != nil {
		if _, ok := existing.(network.VirtualNetworkPeering); !ok {
			return nil, errors.Errorf("%T is not a network.VnetPeering", existing)
		}
		// virtual network peering already exists
		return nil, nil
	}
	vnetID := azure.VNetID(s.SubscriptionID, s.RemoteResourceGroup, s.RemoteVnetName)
	peeringProperties := network.VirtualNetworkPeeringPropertiesFormat{
		RemoteVirtualNetwork: &network.SubResource{
			ID: pointer.String(vnetID),
		},
	}
	return network.VirtualNetworkPeering{
		Name:                                  pointer.String(s.PeeringName),
		VirtualNetworkPeeringPropertiesFormat: &peeringProperties,
	}, nil
}

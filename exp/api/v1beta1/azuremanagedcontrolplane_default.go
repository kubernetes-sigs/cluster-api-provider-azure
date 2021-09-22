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

package v1beta1

import (
	"fmt"
)

const (
	// defaultAKSVnetCIDR is the default Vnet CIDR.
	defaultAKSVnetCIDR = "10.0.0.0/8"
	// defaultAKSNodeSubnetCIDR is the default Node Subnet CIDR.
	defaultAKSNodeSubnetCIDR = "10.240.0.0/16"
)

// setDefaultNodeResourceGroupName sets the default NodeResourceGroup for an AzureManagedControlPlane.
func (m *AzureManagedControlPlane) setDefaultNodeResourceGroupName() {
	if m.Spec.NodeResourceGroupName == "" {
		m.Spec.NodeResourceGroupName = fmt.Sprintf("MC_%s_%s_%s", m.Spec.ResourceGroupName, m.Name, m.Spec.Location)
	}
}

// setDefaultVirtualNetwork sets the default VirtualNetwork for an AzureManagedControlPlane.
func (m *AzureManagedControlPlane) setDefaultVirtualNetwork() {
	if m.Spec.VirtualNetwork.Name == "" {
		m.Spec.VirtualNetwork.Name = m.Name
	}
	if len(m.Spec.VirtualNetwork.CIDRBlocks) == 0 {
		if m.Spec.VirtualNetwork.CIDRBlock == "" {
			m.Spec.VirtualNetwork.CIDRBlocks = []string{defaultAKSVnetCIDR}
		} else {
			m.Spec.VirtualNetwork.CIDRBlocks = []string{m.Spec.VirtualNetwork.CIDRBlock}
		}
	}
}

// setDefaultSubnets sets the default Subnet for an AzureManagedControlPlane.
func (m *AzureManagedControlPlane) setDefaultSubnets() {
	if len(m.Spec.VirtualNetwork.Subnets) == 0 {
		if m.Spec.VirtualNetwork.Subnet.CIDRBlock == "" {
			m.Spec.VirtualNetwork.Subnets = []ManagedControlPlaneSubnet{
				{
					Name:       m.Name,
					CIDRBlocks: []string{defaultAKSNodeSubnetCIDR},
				},
			}
		} else {
			m.Spec.VirtualNetwork.Subnets = []ManagedControlPlaneSubnet{
				{
					Name:       m.Name,
					CIDRBlocks: []string{m.Spec.VirtualNetwork.Subnet.CIDRBlock},
				},
			}
		}
	}
}

func (m *AzureManagedControlPlane) setDefaultSku() {
	if m.Spec.SKU == nil {
		m.Spec.SKU = &SKU{
			Tier: FreeManagedControlPlaneTier,
		}
	}
}

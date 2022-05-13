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

package v1alpha4

import (
	"fmt"

	"github.com/Azure/go-autorest/autorest/to"
)

const (
	// defaultAKSVnetCIDR is the default Vnet CIDR.
	defaultAKSVnetCIDR = "10.0.0.0/8"
	// defaultAKSNodeSubnetCIDR is the default Node Subnet CIDR.
	defaultAKSNodeSubnetCIDR = "10.240.0.0/16"
)

// setDefaultNodeResourceGroupName sets the default NodeResourceGroup for an AzureManagedControlPlane.
func (r *AzureManagedControlPlane) setDefaultNodeResourceGroupName() {
	if r.Spec.NodeResourceGroupName == "" {
		r.Spec.NodeResourceGroupName = fmt.Sprintf("MC_%s_%s_%s", r.Spec.ResourceGroupName, r.Name, r.Spec.Location)
	}
}

// setDefaultVirtualNetwork sets the default VirtualNetwork for an AzureManagedControlPlane.
func (r *AzureManagedControlPlane) setDefaultVirtualNetwork() {
	if r.Spec.VirtualNetwork.Name == "" {
		r.Spec.VirtualNetwork.Name = r.Name
	}
	if len(r.Spec.VirtualNetwork.CIDRBlocks) == 0 {
		r.Spec.VirtualNetwork.CIDRBlocks = []string{defaultAKSVnetCIDR}
	}
}

// setDefaultSubnet sets the default Subnet for an AzureManagedControlPlane.
func (r *AzureManagedControlPlane) setDefaultSubnets() {
	if len(r.Spec.VirtualNetwork.Subnets) == 0 {
		r.Spec.VirtualNetwork.Subnets = []ManagedControlPlaneSubnet{
			{
				Name:       r.Name,
				CIDRBlocks: []string{defaultAKSNodeSubnetCIDR},
			},
		}
	}
}

// setDefaultLoadBalancerProfile sets the default LoadBalancerProfile for an AzureManagedControlPlane.
func (r *AzureManagedControlPlane) setDefaultLoadBalancerProfile() {
	if r.Spec.LoadBalancerProfile == nil {
		r.Spec.LoadBalancerProfile = &LoadBalancerProfile{
			ManagedOutboundIPs: to.Int32Ptr(1),
		}
	}
}

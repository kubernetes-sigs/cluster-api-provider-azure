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

package v1beta1

import (
	"fmt"

	. "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

// SetDefaultsAzureClusterTemplate sets default values for an AzureClusterTemplate.
func SetDefaultsAzureClusterTemplate(c *AzureClusterTemplate) {
	AzureClusterClassSpecSetDefaults(&c.Spec.Template.Spec.AzureClusterClassSpec)
	setDefaultAzureClusterTemplateNetworkTemplateSpec(c)
}

// setDefaultAzureClusterTemplateNetworkTemplateSpec sets default values for an AzureClusterTemplate's NetworkTemplateSpec.
func setDefaultAzureClusterTemplateNetworkTemplateSpec(c *AzureClusterTemplate) {
	setDefaultAzureClusterTemplateVnetTemplate(c)
	setDefaultAzureClusterTemplateBastionTemplate(c)
	setDefaultAzureClusterTemplateSubnetsTemplate(c)

	apiServerLB := &c.Spec.Template.Spec.NetworkSpec.APIServerLB
	setDefaultLoadBalancerClassSpecAPIServerLB(apiServerLB)
	setDefaultAzureClusterTemplateNodeOutboundLB(c)
	setDefaultAzureClusterTemplateControlPlaneOutboundLB(c)
}

// setDefaultAzureClusterTemplateVnetTemplate sets default values for an AzureClusterTemplate's VNet template.
func setDefaultAzureClusterTemplateVnetTemplate(c *AzureClusterTemplate) {
	VnetClassSpecSetDefaults(&c.Spec.Template.Spec.NetworkSpec.Vnet.VnetClassSpec)
}

// setDefaultAzureClusterTemplateBastionTemplate sets default values for an AzureClusterTemplate's bastion template.
func setDefaultAzureClusterTemplateBastionTemplate(c *AzureClusterTemplate) {
	if c.Spec.Template.Spec.BastionSpec.AzureBastion != nil {
		// Ensure defaults for Subnet settings.
		if len(c.Spec.Template.Spec.BastionSpec.AzureBastion.Subnet.CIDRBlocks) == 0 {
			c.Spec.Template.Spec.BastionSpec.AzureBastion.Subnet.CIDRBlocks = []string{DefaultAzureBastionSubnetCIDR}
		}
		if c.Spec.Template.Spec.BastionSpec.AzureBastion.Subnet.Role == "" {
			c.Spec.Template.Spec.BastionSpec.AzureBastion.Subnet.Role = DefaultAzureBastionSubnetRole
		}
	}
}

// setDefaultAzureClusterTemplateSubnetsTemplate sets default values for an AzureClusterTemplate's subnet templates.
func setDefaultAzureClusterTemplateSubnetsTemplate(c *AzureClusterTemplate) {
	clusterSubnet, err := c.Spec.Template.Spec.NetworkSpec.GetSubnetTemplate(SubnetCluster)
	clusterSubnetExists := err == nil
	if clusterSubnetExists {
		SubnetClassSpecSetDefaults(&clusterSubnet.SubnetClassSpec, DefaultClusterSubnetCIDR)
		SecurityGroupClassSetDefaults(&clusterSubnet.SecurityGroup)
		c.Spec.Template.Spec.NetworkSpec.UpdateSubnetTemplate(clusterSubnet, SubnetCluster)
	}

	cpSubnet, errcp := c.Spec.Template.Spec.NetworkSpec.GetSubnetTemplate(SubnetControlPlane)
	if errcp == nil {
		SubnetClassSpecSetDefaults(&cpSubnet.SubnetClassSpec, DefaultControlPlaneSubnetCIDR)
		SecurityGroupClassSetDefaults(&cpSubnet.SecurityGroup)
		c.Spec.Template.Spec.NetworkSpec.UpdateSubnetTemplate(cpSubnet, SubnetControlPlane)
	} else if errcp != nil && !clusterSubnetExists {
		cpSubnet = SubnetTemplateSpec{SubnetClassSpec: SubnetClassSpec{Role: SubnetControlPlane}}
		SubnetClassSpecSetDefaults(&cpSubnet.SubnetClassSpec, DefaultControlPlaneSubnetCIDR)
		SecurityGroupClassSetDefaults(&cpSubnet.SecurityGroup)
		c.Spec.Template.Spec.NetworkSpec.Subnets = append(c.Spec.Template.Spec.NetworkSpec.Subnets, cpSubnet)
	}

	var nodeSubnetFound bool
	var nodeSubnetCounter int
	for i, subnet := range c.Spec.Template.Spec.NetworkSpec.Subnets {
		if subnet.Role != SubnetNode {
			continue
		}
		nodeSubnetCounter++
		nodeSubnetFound = true
		SubnetClassSpecSetDefaults(&subnet.SubnetClassSpec, fmt.Sprintf(DefaultNodeSubnetCIDRPattern, nodeSubnetCounter))
		SecurityGroupClassSetDefaults(&subnet.SecurityGroup)
		c.Spec.Template.Spec.NetworkSpec.Subnets[i] = subnet
	}

	if !nodeSubnetFound && !clusterSubnetExists {
		nodeSubnet := SubnetTemplateSpec{
			SubnetClassSpec: SubnetClassSpec{
				Role:       SubnetNode,
				CIDRBlocks: []string{DefaultNodeSubnetCIDR},
			},
		}
		c.Spec.Template.Spec.NetworkSpec.Subnets = append(c.Spec.Template.Spec.NetworkSpec.Subnets, nodeSubnet)
	}
}

// setDefaultAzureClusterTemplateNodeOutboundLB sets default values for an AzureClusterTemplate's node outbound LB.
func setDefaultAzureClusterTemplateNodeOutboundLB(c *AzureClusterTemplate) {
	if c.Spec.Template.Spec.NetworkSpec.NodeOutboundLB == nil {
		if c.Spec.Template.Spec.NetworkSpec.APIServerLB.Type == Internal {
			return
		}

		var needsOutboundLB bool
		for _, subnet := range c.Spec.Template.Spec.NetworkSpec.Subnets {
			if (subnet.Role == SubnetNode || subnet.Role == SubnetCluster) && subnet.IsIPv6Enabled() {
				needsOutboundLB = true
				break
			}
		}

		// If we don't default the outbound LB when there are some subnets with NAT gateway,
		// and some without, those without wouldn't have outbound traffic. So taking the
		// safer route, we configure the outbound LB in that scenario.
		if !needsOutboundLB {
			return
		}

		c.Spec.Template.Spec.NetworkSpec.NodeOutboundLB = &LoadBalancerClassSpec{}
	}

	setDefaultLoadBalancerClassSpecNodeOutboundLB(c.Spec.Template.Spec.NetworkSpec.NodeOutboundLB)
}

// setDefaultAzureClusterTemplateControlPlaneOutboundLB sets default values for an AzureClusterTemplate's control plane outbound LB.
func setDefaultAzureClusterTemplateControlPlaneOutboundLB(c *AzureClusterTemplate) {
	lb := c.Spec.Template.Spec.NetworkSpec.ControlPlaneOutboundLB
	if lb == nil {
		return
	}
	setDefaultLoadBalancerClassSpecControlPlaneOutboundLB(lb)
}

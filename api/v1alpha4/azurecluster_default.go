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

	"k8s.io/utils/pointer"
)

const (
	// DefaultVnetCIDR is the default Vnet CIDR
	DefaultVnetCIDR = "10.0.0.0/8"
	// DefaultControlPlaneSubnetCIDR is the default Control Plane Subnet CIDR
	DefaultControlPlaneSubnetCIDR = "10.0.0.0/16"
	// DefaultNodeSubnetCIDR is the default Node Subnet CIDR
	DefaultNodeSubnetCIDR = "10.1.0.0/16"
	// DefaultInternalLBIPAddress is the default internal load balancer ip address
	DefaultInternalLBIPAddress = "10.0.0.100"
	// DefaultAzureCloud is the public cloud that will be used by most users
	DefaultAzureCloud = "AzurePublicCloud"
)

func (c *AzureCluster) setDefaults() {
	c.setResourceGroupDefault()
	c.setAzureEnvironmentDefault()
	c.setNetworkSpecDefaults()
}

func (c *AzureCluster) setNetworkSpecDefaults() {
	c.setVnetDefaults()
	c.setSubnetDefaults()
	c.setAPIServerLBDefaults()
	c.setNodeOutboundLBDefaults()
}

func (c *AzureCluster) setResourceGroupDefault() {
	if c.Spec.ResourceGroup == "" {
		c.Spec.ResourceGroup = c.Name
	}
}

func (c *AzureCluster) setAzureEnvironmentDefault() {
	if c.Spec.AzureEnvironment == "" {
		c.Spec.AzureEnvironment = DefaultAzureCloud
	}
}

func (c *AzureCluster) setVnetDefaults() {
	if c.Spec.NetworkSpec.Vnet.ResourceGroup == "" {
		c.Spec.NetworkSpec.Vnet.ResourceGroup = c.Spec.ResourceGroup
	}
	if c.Spec.NetworkSpec.Vnet.Name == "" {
		c.Spec.NetworkSpec.Vnet.Name = generateVnetName(c.ObjectMeta.Name)
	}
	if len(c.Spec.NetworkSpec.Vnet.CIDRBlocks) == 0 {
		c.Spec.NetworkSpec.Vnet.CIDRBlocks = []string{DefaultVnetCIDR}
	}
}

func (c *AzureCluster) setSubnetDefaults() {
	cpSubnet, err := c.Spec.NetworkSpec.GetControlPlaneSubnet()
	if err != nil {
		cpSubnet = SubnetSpec{Role: SubnetControlPlane}
		c.Spec.NetworkSpec.Subnets = append(c.Spec.NetworkSpec.Subnets, cpSubnet)
	}

	nodeSubnet, err := c.Spec.NetworkSpec.GetNodeSubnet()
	if err != nil {
		nodeSubnet = SubnetSpec{Role: SubnetNode}
		c.Spec.NetworkSpec.Subnets = append(c.Spec.NetworkSpec.Subnets, nodeSubnet)
	}

	if cpSubnet.Name == "" {
		cpSubnet.Name = generateControlPlaneSubnetName(c.ObjectMeta.Name)
	}
	if len(cpSubnet.CIDRBlocks) == 0 {
		cpSubnet.CIDRBlocks = []string{DefaultControlPlaneSubnetCIDR}
	}
	if cpSubnet.SecurityGroup.Name == "" {
		cpSubnet.SecurityGroup.Name = generateControlPlaneSecurityGroupName(c.ObjectMeta.Name)
	}
	setSecurityRuleDefaults(&cpSubnet.SecurityGroup)

	if nodeSubnet.Name == "" {
		nodeSubnet.Name = generateNodeSubnetName(c.ObjectMeta.Name)
	}
	if len(nodeSubnet.CIDRBlocks) == 0 {
		nodeSubnet.CIDRBlocks = []string{DefaultNodeSubnetCIDR}
	}
	if nodeSubnet.SecurityGroup.Name == "" {
		nodeSubnet.SecurityGroup.Name = generateNodeSecurityGroupName(c.ObjectMeta.Name)
	}
	setSecurityRuleDefaults(&nodeSubnet.SecurityGroup)
	if nodeSubnet.RouteTable.Name == "" {
		nodeSubnet.RouteTable.Name = generateNodeRouteTableName(c.ObjectMeta.Name)
	}

	c.Spec.NetworkSpec.UpdateControlPlaneSubnet(cpSubnet)
	c.Spec.NetworkSpec.UpdateNodeSubnet(nodeSubnet)
}

func setSecurityRuleDefaults(sg *SecurityGroup) {
	for i := range sg.SecurityRules {
		if sg.SecurityRules[i].Direction == "" {
			sg.SecurityRules[i].Direction = SecurityRuleDirectionInbound
		}
	}
}

func (c *AzureCluster) setAPIServerLBDefaults() {
	lb := &c.Spec.NetworkSpec.APIServerLB
	if lb.Type == "" {
		lb.Type = Public
	}
	if lb.SKU == "" {
		lb.SKU = SKUStandard
	}

	if lb.Type == Public {
		if lb.Name == "" {
			lb.Name = generatePublicLBName(c.ObjectMeta.Name)
		}
		if len(lb.FrontendIPs) == 0 {
			lb.FrontendIPs = []FrontendIP{
				{
					Name: generateFrontendIPConfigName(lb.Name),
					PublicIP: &PublicIPSpec{
						Name: generatePublicIPName(c.ObjectMeta.Name),
					},
				},
			}
		}

	} else if lb.Type == Internal {
		if lb.Name == "" {
			lb.Name = generateInternalLBName(c.ObjectMeta.Name)
		}
		if len(lb.FrontendIPs) == 0 {
			lb.FrontendIPs = []FrontendIP{
				{
					Name:             generateFrontendIPConfigName(lb.Name),
					PrivateIPAddress: DefaultInternalLBIPAddress,
				},
			}
		}
	}
}

func (c *AzureCluster) setNodeOutboundLBDefaults() {
	if c.Spec.NetworkSpec.NodeOutboundLB == nil {
		if c.Spec.NetworkSpec.APIServerLB.Type == Internal {
			return
		}
		c.Spec.NetworkSpec.NodeOutboundLB = &LoadBalancerSpec{}
	}

	lb := c.Spec.NetworkSpec.NodeOutboundLB
	lb.Type = Public
	lb.SKU = SKUStandard
	lb.Name = c.ObjectMeta.Name

	if lb.FrontendIPsCount == nil {
		lb.FrontendIPsCount = pointer.Int32Ptr(1)
	}

	switch *lb.FrontendIPsCount {
	case 0:
		lb.FrontendIPs = []FrontendIP{}
	case 1:
		lb.FrontendIPs = []FrontendIP{
			{
				Name: generateFrontendIPConfigName(lb.Name),
				PublicIP: &PublicIPSpec{
					Name: generateNodeOutboundIPName(c.ObjectMeta.Name),
				},
			},
		}
	default:
		lb.FrontendIPs = make([]FrontendIP, *lb.FrontendIPsCount)
		for i := 0; i < int(*lb.FrontendIPsCount); i++ {
			lb.FrontendIPs[i] = FrontendIP{
				Name: withIndex(generateFrontendIPConfigName(lb.Name), i+1),
				PublicIP: &PublicIPSpec{
					Name: withIndex(generateNodeOutboundIPName(c.ObjectMeta.Name), i+1),
				},
			}
		}

	}
}

// generateVnetName generates a virtual network name, based on the cluster name.
func generateVnetName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "vnet")
}

// generateControlPlaneSubnetName generates a node subnet name, based on the cluster name.
func generateControlPlaneSubnetName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "controlplane-subnet")
}

// generateNodeSubnetName generates a node subnet name, based on the cluster name.
func generateNodeSubnetName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "node-subnet")
}

// generateControlPlaneSecurityGroupName generates a control plane security group name, based on the cluster name.
func generateControlPlaneSecurityGroupName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "controlplane-nsg")
}

// generateNodeSecurityGroupName generates a node security group name, based on the cluster name.
func generateNodeSecurityGroupName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "node-nsg")
}

// generateNodeRouteTableName generates a node route table name, based on the cluster name.
func generateNodeRouteTableName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "node-routetable")
}

// generateInternalLBName generates a internal load balancer name, based on the cluster name.
func generateInternalLBName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "internal-lb")
}

// generatePublicLBName generates a public load balancer name, based on the cluster name.
func generatePublicLBName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "public-lb")
}

// generatePublicIPName generates a public IP name, based on the cluster name and a hash.
func generatePublicIPName(clusterName string) string {
	return fmt.Sprintf("pip-%s-apiserver", clusterName)
}

// generateFrontendIPConfigName generates a load balancer frontend IP config name.
func generateFrontendIPConfigName(lbName string) string {
	return fmt.Sprintf("%s-%s", lbName, "frontEnd")
}

// generateNodeOutboundIPName generates a public IP name, based on the cluster name.
func generateNodeOutboundIPName(clusterName string) string {
	return fmt.Sprintf("pip-%s-node-outbound", clusterName)
}

// withIndex appends the index as suffix to a generated name.
func withIndex(name string, n int) string {
	return fmt.Sprintf("%s-%d", name, n)
}

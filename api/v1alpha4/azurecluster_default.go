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
	// DefaultVnetCIDR is the default Vnet CIDR.
	DefaultVnetCIDR = "10.0.0.0/8"
	// DefaultControlPlaneSubnetCIDR is the default Control Plane Subnet CIDR.
	DefaultControlPlaneSubnetCIDR = "10.0.0.0/16"
	// DefaultNodeSubnetCIDR is the default Node Subnet CIDR.
	DefaultNodeSubnetCIDR = "10.1.0.0/16"
	// DefaultAzureBastionSubnetCIDR is the default Subnet CIDR for AzureBastion.
	DefaultAzureBastionSubnetCIDR = "10.255.255.224/27"
	// DefaultAzureBastionSubnetName is the default Subnet Name for AzureBastion.
	DefaultAzureBastionSubnetName = "AzureBastionSubnet"
	// DefaultInternalLBIPAddress is the default internal load balancer ip address.
	DefaultInternalLBIPAddress = "10.0.0.100"
	// DefaultOutboundRuleIdleTimeoutInMinutes is the default for IdleTimeoutInMinutes for the load balancer.
	DefaultOutboundRuleIdleTimeoutInMinutes = 4
	// DefaultAzureCloud is the public cloud that will be used by most users.
	DefaultAzureCloud = "AzurePublicCloud"
)

func (s *AzureClusterSpec) setDefaults(name string) {
	s.setResourceGroupDefault(name)
	s.setAzureEnvironmentDefault()
	s.setNetworkSpecDefaults(name)
}

func (s *AzureClusterSpec) setNetworkSpecDefaults(name string) {
	s.setVnetDefaults(name)
	s.setBastionDefaults(name)
	s.setSubnetDefaults(name)
	s.setAPIServerLBDefaults(name)
	s.setNodeOutboundLBDefaults(name)
	s.setControlPlaneOutboundLBDefaults(name)
}

func (s *AzureClusterSpec) setResourceGroupDefault(name string) {
	if s.ResourceGroup == "" {
		s.ResourceGroup = name
	}
}

func (s *AzureClusterSpec) setAzureEnvironmentDefault() {
	if s.AzureEnvironment == "" {
		s.AzureEnvironment = DefaultAzureCloud
	}
}

func (s *AzureClusterSpec) setVnetDefaults(name string) {
	if s.NetworkSpec.Vnet.ResourceGroup == "" {
		s.NetworkSpec.Vnet.ResourceGroup = s.ResourceGroup
	}
	if s.NetworkSpec.Vnet.Name == "" {
		s.NetworkSpec.Vnet.Name = generateVnetName(name)
	}
	if len(s.NetworkSpec.Vnet.CIDRBlocks) == 0 {
		s.NetworkSpec.Vnet.CIDRBlocks = []string{DefaultVnetCIDR}
	}
}

func (s *AzureClusterSpec) setSubnetDefaults(name string) {
	cpSubnet, err := s.NetworkSpec.GetControlPlaneSubnet()
	if err != nil {
		cpSubnet = SubnetSpec{Role: SubnetControlPlane}
		s.NetworkSpec.Subnets = append(s.NetworkSpec.Subnets, cpSubnet)
	}

	nodeSubnet, err := s.NetworkSpec.GetNodeSubnet()
	if err != nil {
		nodeSubnet = SubnetSpec{Role: SubnetNode}
		s.NetworkSpec.Subnets = append(s.NetworkSpec.Subnets, nodeSubnet)
	}

	if cpSubnet.Name == "" {
		cpSubnet.Name = generateControlPlaneSubnetName(name)
	}
	if len(cpSubnet.CIDRBlocks) == 0 {
		cpSubnet.CIDRBlocks = []string{DefaultControlPlaneSubnetCIDR}
	}
	if cpSubnet.SecurityGroup.Name == "" {
		cpSubnet.SecurityGroup.Name = generateControlPlaneSecurityGroupName(name)
	}
	setSecurityRuleDefaults(&cpSubnet.SecurityGroup)

	if nodeSubnet.Name == "" {
		nodeSubnet.Name = generateNodeSubnetName(name)
	}
	if len(nodeSubnet.CIDRBlocks) == 0 {
		nodeSubnet.CIDRBlocks = []string{DefaultNodeSubnetCIDR}
	}
	if nodeSubnet.SecurityGroup.Name == "" {
		nodeSubnet.SecurityGroup.Name = generateNodeSecurityGroupName(name)
	}
	setSecurityRuleDefaults(&nodeSubnet.SecurityGroup)
	if nodeSubnet.RouteTable.Name == "" {
		nodeSubnet.RouteTable.Name = generateNodeRouteTableName(name)
	}

	if nodeSubnet.NatGateway.Name != "" {
		if nodeSubnet.NatGateway.NatGatewayIP.Name == "" {
			nodeSubnet.NatGateway.NatGatewayIP.Name = generateNatGatewayIPName(name, nodeSubnet.Name)
		}
	}

	s.NetworkSpec.UpdateControlPlaneSubnet(cpSubnet)
	s.NetworkSpec.UpdateNodeSubnet(nodeSubnet)
}

func setSecurityRuleDefaults(sg *SecurityGroup) {
	for i := range sg.SecurityRules {
		if sg.SecurityRules[i].Direction == "" {
			sg.SecurityRules[i].Direction = SecurityRuleDirectionInbound
		}
	}
}

func (s *AzureClusterSpec) setAPIServerLBDefaults(name string) {
	lb := &s.NetworkSpec.APIServerLB
	if lb.Type == "" {
		lb.Type = Public
	}
	if lb.SKU == "" {
		lb.SKU = SKUStandard
	}
	if lb.IdleTimeoutInMinutes == nil {
		lb.IdleTimeoutInMinutes = pointer.Int32Ptr(DefaultOutboundRuleIdleTimeoutInMinutes)
	}

	if lb.Type == Public {
		if lb.Name == "" {
			lb.Name = generatePublicLBName(name)
		}
		if len(lb.FrontendIPs) == 0 {
			lb.FrontendIPs = []FrontendIP{
				{
					Name: generateFrontendIPConfigName(lb.Name),
					PublicIP: &PublicIPSpec{
						Name: generatePublicIPName(name),
					},
				},
			}
		}
	} else if lb.Type == Internal {
		if lb.Name == "" {
			lb.Name = generateInternalLBName(name)
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

func (s *AzureClusterSpec) setNodeOutboundLBDefaults(name string) {
	if s.NetworkSpec.NodeOutboundLB == nil {
		if s.NetworkSpec.APIServerLB.Type == Internal {
			return
		}
		nodeSubnet, err := s.NetworkSpec.GetNodeSubnet()
		// Only one outbound mechanism can be defined, so if Nat Gateway is defined, we don't default the LB.
		if err == nil && nodeSubnet.NatGateway.Name != "" {
			return
		}
		s.NetworkSpec.NodeOutboundLB = &LoadBalancerSpec{}
	}

	lb := s.NetworkSpec.NodeOutboundLB
	lb.Type = Public
	lb.SKU = SKUStandard
	lb.Name = name

	if lb.IdleTimeoutInMinutes == nil {
		lb.IdleTimeoutInMinutes = pointer.Int32Ptr(DefaultOutboundRuleIdleTimeoutInMinutes)
	}

	if lb.FrontendIPsCount == nil {
		lb.FrontendIPsCount = pointer.Int32Ptr(1)
	}

	s.setOutboundLBFrontendIPs(name, lb, generateNodeOutboundIPName)
}

func (s *AzureClusterSpec) setControlPlaneOutboundLBDefaults(name string) {
	// public clusters don't need control plane outbound lb
	if s.NetworkSpec.APIServerLB.Type == Public {
		return
	}

	// private clusters can disable control plane outbound lb by setting it to nil.
	if s.NetworkSpec.ControlPlaneOutboundLB == nil {
		return
	}

	lb := s.NetworkSpec.ControlPlaneOutboundLB
	lb.Type = Public
	lb.SKU = SKUStandard

	if lb.Name == "" {
		lb.Name = generateControlPlaneOutboundLBName(name)
	}

	if lb.IdleTimeoutInMinutes == nil {
		lb.IdleTimeoutInMinutes = pointer.Int32Ptr(DefaultOutboundRuleIdleTimeoutInMinutes)
	}

	if lb.FrontendIPsCount == nil {
		lb.FrontendIPsCount = pointer.Int32Ptr(1)
	}

	s.setOutboundLBFrontendIPs(name, lb, generateControlPlaneOutboundIPName)
}

// setOutboundLBFrontendIPs sets the frontend ips for the given load balancer.
// The name of the frontend ip is generated using generatePublicIPName function.
func (s *AzureClusterSpec) setOutboundLBFrontendIPs(name string, lb *LoadBalancerSpec, generatePublicIPName func(string) string) {
	switch *lb.FrontendIPsCount {
	case 0:
		lb.FrontendIPs = []FrontendIP{}
	case 1:
		lb.FrontendIPs = []FrontendIP{
			{
				Name: generateFrontendIPConfigName(lb.Name),
				PublicIP: &PublicIPSpec{
					Name: generatePublicIPName(name),
				},
			},
		}
	default:
		lb.FrontendIPs = make([]FrontendIP, *lb.FrontendIPsCount)
		for i := 0; i < int(*lb.FrontendIPsCount); i++ {
			lb.FrontendIPs[i] = FrontendIP{
				Name: withIndex(generateFrontendIPConfigName(lb.Name), i+1),
				PublicIP: &PublicIPSpec{
					Name: withIndex(generatePublicIPName(name), i+1),
				},
			}
		}
	}
}

func (s *AzureClusterSpec) setBastionDefaults(name string) {
	if s.BastionSpec.AzureBastion != nil {
		if s.BastionSpec.AzureBastion.Name == "" {
			s.BastionSpec.AzureBastion.Name = generateAzureBastionName(name)
		}
		// Ensure defaults for the Subnet settings.
		{
			if s.BastionSpec.AzureBastion.Subnet.Name == "" {
				s.BastionSpec.AzureBastion.Subnet.Name = DefaultAzureBastionSubnetName
			}
			if len(s.BastionSpec.AzureBastion.Subnet.CIDRBlocks) == 0 {
				s.BastionSpec.AzureBastion.Subnet.CIDRBlocks = []string{DefaultAzureBastionSubnetCIDR}
			}
		}
		// Ensure defaults for the PublicIP settings.
		{
			if s.BastionSpec.AzureBastion.PublicIP.Name == "" {
				s.BastionSpec.AzureBastion.PublicIP.Name = generateAzureBastionPublicIPName(name)
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

// generateAzureBastionName generates an azure bastion name.
func generateAzureBastionName(clusterName string) string {
	return fmt.Sprintf("%s-azure-bastion", clusterName)
}

// generateAzureBastionPublicIPName generates an azure bastion public ip name.
func generateAzureBastionPublicIPName(clusterName string) string {
	return fmt.Sprintf("%s-azure-bastion-pip", clusterName)
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

// generateControlPlaneOutboundLBName generates the name of the control plane outbound LB.
func generateControlPlaneOutboundLBName(clusterName string) string {
	return fmt.Sprintf("%s-outbound-lb", clusterName)
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

// generateControlPlaneOutboundIPName generates a public IP name, based on the cluster name.
func generateControlPlaneOutboundIPName(clusterName string) string {
	return fmt.Sprintf("pip-%s-controlplane-outbound", clusterName)
}

// generateNatGatewayIPName generates a nat gateway IP name.
func generateNatGatewayIPName(clusterName, subnetName string) string {
	return fmt.Sprintf("pip-%s-%s-natgw", clusterName, subnetName)
}

// withIndex appends the index as suffix to a generated name.
func withIndex(name string, n int) string {
	return fmt.Sprintf("%s-%d", name, n)
}

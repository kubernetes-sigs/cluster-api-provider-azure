/*
Copyright 2020 The Kubernetes Authors.

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

package v1alpha3

import (
	"fmt"
)

const (
	// DefaultVnetCIDR is the default Vnet CIDR
	DefaultVnetCIDR = "5.5.0.0/16"
	// DefaultControlPlaneSubnetCIDR is the default Control Plane Subnet CIDR
	DefaultControlPlaneSubnetCIDR = "5.5.0.0/16"
	// DefaultNodeSubnetCIDR is the default Node Subnet CIDR
	//DefaultNodeSubnetCIDR = "5.5.0.0/16"
)

func (c *AzureCluster) setDefaults() {
	c.setNetworkSpecDefaults()
}

func (c *AzureCluster) setNetworkSpecDefaults() {
	c.setResourceGroupDefault()
	c.setVnetDefaults()
	c.setSubnetDefaults()
}

func (c *AzureCluster) setResourceGroupDefault() {
	if c.Spec.ResourceGroup == "" {
		c.Spec.ResourceGroup = c.Name
	}
}

func (c *AzureCluster) setVnetDefaults() {
	if c.Spec.NetworkSpec.Vnet.ResourceGroup == "" {
		c.Spec.NetworkSpec.Vnet.ResourceGroup = c.Spec.ResourceGroup
	}
	if c.Spec.NetworkSpec.Vnet.Name == "" {
		c.Spec.NetworkSpec.Vnet.Name = generateVnetName(c.ObjectMeta.Name)
	}
	if c.Spec.NetworkSpec.Vnet.CidrBlock == "" {
		c.Spec.NetworkSpec.Vnet.CidrBlock = DefaultVnetCIDR
	}
}

func (c *AzureCluster) setSubnetDefaults() {
	cpSubnet := c.Spec.NetworkSpec.GetControlPlaneSubnet()
	if cpSubnet == nil {
		cpSubnet = &SubnetSpec{Role: SubnetControlPlane}
		c.Spec.NetworkSpec.Subnets = append(c.Spec.NetworkSpec.Subnets, cpSubnet)
	}

	/*nodeSubnet := c.Spec.NetworkSpec.GetNodeSubnet()
	if nodeSubnet == nil {
		nodeSubnet = &SubnetSpec{Role: SubnetNode}
		c.Spec.NetworkSpec.Subnets = append(c.Spec.NetworkSpec.Subnets, nodeSubnet)
	}*/

	if cpSubnet.Name == "" {
		cpSubnet.Name = generateControlPlaneSubnetName(c.ObjectMeta.Name)
	}
	if cpSubnet.CidrBlock == "" {
		cpSubnet.CidrBlock = DefaultControlPlaneSubnetCIDR
	}

	/*if cpSubnet.SecurityGroup.Name == "" {
		cpSubnet.SecurityGroup.Name = generateControlPlaneSecurityGroupName(c.ObjectMeta.Name)
	}
	if cpSubnet.RouteTable.Name == "" {
		cpSubnet.RouteTable.Name = generateRouteTableName(c.ObjectMeta.Name)
	}*/

	/*if nodeSubnet.Name == "" {
		nodeSubnet.Name = generateNodeSubnetName(c.ObjectMeta.Name)
	}
	if nodeSubnet.CidrBlock == "" {
		nodeSubnet.CidrBlock = DefaultNodeSubnetCIDR
	}*/

	/*if nodeSubnet.SecurityGroup.Name == "" {
		nodeSubnet.SecurityGroup.Name = generateNodeSecurityGroupName(c.ObjectMeta.Name)
	}
	if nodeSubnet.RouteTable.Name == "" {
		nodeSubnet.RouteTable.Name = generateRouteTableName(c.ObjectMeta.Name)
	}*/
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

// generateRouteTableName generates a route table name, based on the cluster name.
func generateRouteTableName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "node-routetable")
}

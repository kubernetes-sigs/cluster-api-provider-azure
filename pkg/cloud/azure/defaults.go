/*
Copyright 2019 The Kubernetes Authors.

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

package azure

const (
	// DefaultUserName is the default username for created vm
	DefaultUserName = "capi"
	// DefaultVnetName is the default name for the cluster's virtual network.
	DefaultVnetName = "ClusterAPIVnet"
	// DefaultVnetCIDR is the default Vnet CIDR
	DefaultVnetCIDR = "10.0.0.0/8"
	// DefaultControlPlaneSecurityGroupName is the default name for the control plane security group.
	DefaultControlPlaneSecurityGroupName = "ClusterAPIControlPlaneNSG"
	// DefaultNodeSecurityGroupName is the default name for the Node's security group.
	DefaultNodeSecurityGroupName = "ClusterAPINodeNSG"
	// DefaultNodeRouteTableName is the default name for the Node's route table.
	DefaultNodeRouteTableName = "ClusterAPINodeRouteTable"
	// DefaultControlPlaneSubnetName is the default name for the Node's subnet.
	DefaultControlPlaneSubnetName = "ClusterAPIControlPlaneSubnet"
	// DefaultControlPlaneSubnetCIDR is the default Control Plane Subnet CIDR
	DefaultControlPlaneSubnetCIDR = "10.0.0.0/16"
	// DefaultNodeSubnetName is the default name for the Node's subnet.
	DefaultNodeSubnetName = "ClusterAPINodeSubnet"
	// DefaultNodeSubnetCIDR is the default Node Subnet CIDR
	DefaultNodeSubnetCIDR = "10.1.0.0/16"
	// DefaultInternalLBName is the default internal load balancer name
	DefaultInternalLBName = "ClusterAPIInternalLB"
	// DefaultInternalLBIPAddress is the default internal load balancer ip address
	DefaultInternalLBIPAddress = "10.0.0.100"
	// DefaultPublicLBName is the default public load balancer name
	DefaultPublicLBName = "ClusterAPIPublicLB"
	// DefaultPublicIPPrefix is the default publicip prefix
	DefaultPublicIPPrefix = "ClusterAPI"
	// DefaultAzureDNSZone is the default provided azure dns zone
	DefaultAzureDNSZone = "cloudapp.azure.com"
)

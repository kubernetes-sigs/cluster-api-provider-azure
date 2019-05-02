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

import "fmt"

const (
	// DefaultUserName is the default username for created vm
	DefaultUserName = "capi"
	// DefaultVnetCIDR is the default Vnet CIDR
	DefaultVnetCIDR = "10.0.0.0/8"
	// DefaultControlPlaneSubnetCIDR is the default Control Plane Subnet CIDR
	DefaultControlPlaneSubnetCIDR = "10.0.0.0/16"
	// DefaultNodeSubnetCIDR is the default Node Subnet CIDR
	DefaultNodeSubnetCIDR = "10.1.0.0/16"
	// DefaultInternalLBIPAddress is the default internal load balancer ip address
	DefaultInternalLBIPAddress = "10.0.0.100"
	// DefaultAzureDNSZone is the default provided azure dns zone
	DefaultAzureDNSZone = "cloudapp.azure.com"
)

// GenerateVnetName generates a virtual network name, based on the cluster name.
func GenerateVnetName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "vnet")
}

// GenerateControlPlaneSecurityGroupName generates a control plane security group name, based on the cluster name.
func GenerateControlPlaneSecurityGroupName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "controlplane-nsg")
}

// GenerateNodeSecurityGroupName generates a node security group name, based on the cluster name.
func GenerateNodeSecurityGroupName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "node-nsg")
}

// GenerateNodeRouteTableName generates a node route table name, based on the cluster name.
func GenerateNodeRouteTableName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "node-routetable")
}

// GenerateControlPlaneSubnetName generates a node subnet name, based on the cluster name.
func GenerateControlPlaneSubnetName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "controlplane-subnet")
}

// GenerateNodeSubnetName generates a node subnet name, based on the cluster name.
func GenerateNodeSubnetName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "node-subnet")
}

// GenerateInternalLBName generates a internal load balancer name, based on the cluster name.
func GenerateInternalLBName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "internal-lb")
}

// GeneratePublicLBName generates a public load balancer name, based on the cluster name.
func GeneratePublicLBName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, "public-lb")
}

// GeneratePublicIPName generates a public IP name, based on the cluster name and a hash.
func GeneratePublicIPName(clusterName, hash string) string {
	return fmt.Sprintf("%s-%s", clusterName, hash)
}

// GenerateFQDN generates a fully qualified domain name, based on the public IP name and cluster location.
func GenerateFQDN(publicIPName, location string) string {
	return fmt.Sprintf("%s.%s.%s", publicIPName, location, DefaultAzureDNSZone)
}

// GenerateManagedIdentityName generates managed identity name.
func GenerateManagedIdentityName(subscriptionID, resourceGroupName, clusterName string) string {
	return fmt.Sprintf(
		"/subscriptions/%s/resourcegroups/%s/providers/Microsoft.ManagedIdentity/userAssignedIdentities/%s-identity",
		subscriptionID,
		resourceGroupName,
		clusterName)
}

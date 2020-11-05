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

package defaults

import (
	"fmt"

	"sigs.k8s.io/cluster-api-provider-azure/version"
)

const (
	// DefaultUserName is the default username for created vm
	DefaultUserName = "capi"
)

// GenerateBackendAddressPoolName generates a load balancer backend address pool name.
func GenerateBackendAddressPoolName(lbName string) string {
	return fmt.Sprintf("%s-%s", lbName, "backendPool")
}

// GenerateOutboundBackendAddressPoolName generates a load balancer outbound backend address pool name.
func GenerateOutboundBackendAddressPoolName(lbName string) string {
	return fmt.Sprintf("%s-%s", lbName, "outboundBackendPool")
}

// GenerateFrontendIPConfigName generates a load balancer frontend IP config name.
func GenerateFrontendIPConfigName(lbName string) string {
	return fmt.Sprintf("%s-%s", lbName, "frontEnd")
}

// GenerateNodeOutboundIPName generates a public IP name, based on the cluster name.
func GenerateNodeOutboundIPName(clusterName string) string {
	return fmt.Sprintf("pip-%s-node-outbound", clusterName)
}

// GenerateNodePublicIPName generates a node public IP name, based on the machine name.
func GenerateNodePublicIPName(machineName string) string {
	return fmt.Sprintf("pip-%s", machineName)
}

// GenerateControlPlaneOutboundLBName generates the name of the control plane outbound LB.
func GenerateControlPlaneOutboundLBName(clusterName string) string {
	return fmt.Sprintf("%s-outbound-lb", clusterName)
}

// GenerateControlPlaneOutboundIPName generates a public IP name, based on the cluster name.
func GenerateControlPlaneOutboundIPName(clusterName string) string {
	return fmt.Sprintf("pip-%s-controlplane-outbound", clusterName)
}

// GenerateNICName generates the name of a network interface based on the name of a VM.
func GenerateNICName(machineName string) string {
	return fmt.Sprintf("%s-nic", machineName)
}

// GeneratePublicNICName generates the name of a public network interface based on the name of a VM.
func GeneratePublicNICName(machineName string) string {
	return fmt.Sprintf("%s-public-nic", machineName)
}

// GenerateOSDiskName generates the name of an OS disk based on the name of a VM.
func GenerateOSDiskName(machineName string) string {
	return fmt.Sprintf("%s_OSDisk", machineName)
}

// GenerateDataDiskName generates the name of a data disk based on the name of a VM.
func GenerateDataDiskName(machineName, nameSuffix string) string {
	return fmt.Sprintf("%s_%s", machineName, nameSuffix)
}

// VMID returns the azure resource ID for a given VM.
func VMID(subscriptionID, resourceGroup, vmName string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachines/%s", subscriptionID, resourceGroup, vmName)
}

// SubnetID returns the azure resource ID for a given subnet.
func SubnetID(subscriptionID, resourceGroup, vnetName, subnetName string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s/subnets/%s", subscriptionID, resourceGroup, vnetName, subnetName)
}

// PublicIPID returns the azure resource ID for a given public IP.
func PublicIPID(subscriptionID, resourceGroup, ipName string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/publicIPAddresses/%s", subscriptionID, resourceGroup, ipName)
}

// RouteTableID returns the azure resource ID for a given route table.
func RouteTableID(subscriptionID, resourceGroup, routeTableName string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/routeTables/%s", subscriptionID, resourceGroup, routeTableName)
}

// SecurityGroupID returns the azure resource ID for a given security group.
func SecurityGroupID(subscriptionID, resourceGroup, nsgName string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/networkSecurityGroups/%s", subscriptionID, resourceGroup, nsgName)
}

// NetworkInterfaceID returns the azure resource ID for a given network interface.
func NetworkInterfaceID(subscriptionID, resourceGroup, nicName string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/networkInterfaces/%s", subscriptionID, resourceGroup, nicName)
}

// FrontendIPConfigID returns the azure resource ID for a given frontend IP config.
func FrontendIPConfigID(subscriptionID, resourceGroup, loadBalancerName, configName string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers/%s/frontendIPConfigurations/%s", subscriptionID, resourceGroup, loadBalancerName, configName)
}

// AddressPoolID returns the azure resource ID for a given backend address pool.
func AddressPoolID(subscriptionID, resourceGroup, loadBalancerName, backendPoolName string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers/%s/backendAddressPools/%s", subscriptionID, resourceGroup, loadBalancerName, backendPoolName)
}

// ProbeID returns the azure resource ID for a given probe.
func ProbeID(subscriptionID, resourceGroup, loadBalancerName, probeName string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers/%s/probes/%s", subscriptionID, resourceGroup, loadBalancerName, probeName)
}

// NATRuleID returns the azure resource ID for a inbound NAT rule.
func NATRuleID(subscriptionID, resourceGroup, loadBalancerName, natRuleName string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers/%s/inboundNatRules/%s", subscriptionID, resourceGroup, loadBalancerName, natRuleName)
}

// UserAgent specifies a string to append to the agent identifier.
func UserAgent() string {
	return fmt.Sprintf("cluster-api-provider-azure/%s", version.Get().String())
}

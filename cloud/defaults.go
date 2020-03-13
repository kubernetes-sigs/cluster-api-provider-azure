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

import (
	"fmt"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
)

const (
	// DefaultUserName is the default username for created VM.
	DefaultUserName = "capi"
	// DefaultVnetCIDR is the default Vnet CIDR.
	DefaultVnetCIDR = "10.0.0.0/8"
	// DefaultControlPlaneSubnetCIDR is the default Control Plane Subnet CIDR.
	DefaultControlPlaneSubnetCIDR = "10.0.0.0/16"
	// DefaultNodeSubnetCIDR is the default Node Subnet CIDR.
	DefaultNodeSubnetCIDR = "10.1.0.0/16"
	// DefaultInternalLBIPAddress is the default internal load balancer IP address.
	DefaultInternalLBIPAddress = "10.0.0.100"
	// DefaultAzureDNSZone is the default provided Azure DNS zone.
	DefaultAzureDNSZone = "cloudapp.azure.com"
	// UserAgent used for communicating with Azure.
	UserAgent = "cluster-api-azure-services"
	// MaxNumberOfControlPlanes is the maximum allowed number of control planes, used to determine number of needed Inbound NAT Rules.
	MaxNumberOfControlPlanes = 7
)

const (
	// DefaultImageOfferID is the default Azure Marketplace offer ID
	DefaultImageOfferID = "capi"
	// DefaultImagePublisherID is the default Azure Marketplace publisher ID
	DefaultImagePublisherID = "cncf-upstream"
	// LatestVersion is the image version latest
	LatestVersion = "latest"
)

// SupportedAvailabilityZoneLocations is a slice of the locations where Availability Zones are supported.
// This is used to validate whether a virtual machine should leverage an Availability Zone.
// Based on the Availability Zones listed in https://docs.microsoft.com/en-us/azure/availability-zones/az-overview
var SupportedAvailabilityZoneLocations = []string{
	// Americas
	"centralus",
	"eastus",
	"eastus2",
	"westus2",

	// Europe
	"francecentral",
	"northeurope",
	"uksouth",
	"westeurope",

	// Asia Pacific
	"japaneast",
	"southeastasia",
}

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

// GenerateNICName generates the name of a network interface based on the name of a VM.
func GenerateNICName(machineName string) string {
	return fmt.Sprintf("%s-nic", machineName)
}

// GenerateOSDiskName generates the name of an OS disk based on the name of a VM.
func GenerateOSDiskName(machineName string) string {
	return fmt.Sprintf("%s_OSDisk", machineName)
}

// GetDefaultImageSKUID gets the SKU ID of the image to use for the provided version of Kubernetes.
func getDefaultImageSKUID(k8sVersion string) (string, error) {
	version, err := semver.ParseTolerant(k8sVersion)
	if err != nil {
		return "", errors.Wrapf(err, "unable to parse Kubernetes version \"%s\" in spec, expected valid SemVer string", k8sVersion)
	}
	return fmt.Sprintf("k8s-%ddot%ddot%d-ubuntu-1804", version.Major, version.Minor, version.Patch), nil
}

// GetDefaultUbuntuImage returns the default image spec for Ubuntu.
func GetDefaultUbuntuImage(k8sVersion string) (*infrav1.Image, error) {
	skuID, err := getDefaultImageSKUID(k8sVersion)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get default image")
	}

	defaultImage := &infrav1.Image{
		Marketplace: &infrav1.AzureMarketplaceImage{
			Publisher: DefaultImagePublisherID,
			Offer:     DefaultImageOfferID,
			SKU:       skuID,
			Version:   LatestVersion,
		},
	}

	return defaultImage, nil
}

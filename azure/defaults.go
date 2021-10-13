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
	"net/http"

	"github.com/Azure/go-autorest/autorest/azure"

	"github.com/Azure/go-autorest/autorest"
	"github.com/blang/semver"
	"github.com/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
	"sigs.k8s.io/cluster-api-provider-azure/version"
)

const (
	// DefaultUserName is the default username for a created VM.
	DefaultUserName = "capi"
)

const (
	// DefaultImageOfferID is the default Azure Marketplace offer ID.
	DefaultImageOfferID = "capi"
	// DefaultWindowsImageOfferID is the default Azure Marketplace offer ID for Windows.
	DefaultWindowsImageOfferID = "capi-windows"
	// DefaultImagePublisherID is the default Azure Marketplace publisher ID.
	DefaultImagePublisherID = "cncf-upstream"
	// LatestVersion is the image version latest.
	LatestVersion = "latest"
)

const (
	// LinuxOS is Linux OS value for OSDisk.OSType.
	LinuxOS = "Linux"
	// WindowsOS is Windows OS value for OSDisk.OSType.
	WindowsOS = "Windows"
)

const (
	// Global is the Azure global location value.
	Global = "global"
)

const (
	// PrivateAPIServerHostname will be used as the api server hostname for private clusters.
	PrivateAPIServerHostname = "apiserver"
)

const (
	// ControlPlaneNodeGroup will be used to create availability set for control plane machines.
	ControlPlaneNodeGroup = "control-plane"
)

const (
	// bootstrapExtensionRetries is the number of retries in the BootstrapExtensionCommand.
	// NOTE: the overall timeout will be number of retries * retry sleep, in this case 60 * 5s = 300s.
	bootstrapExtensionRetries = 60
	// bootstrapExtensionSleep is the duration in seconds to sleep before each retry in the BootstrapExtensionCommand.
	bootstrapExtensionSleep = 5
	// bootstrapSentinelFile is the file written by bootstrap provider on machines to indicate successful bootstrapping,
	// as defined by the Cluster API Bootstrap Provider contract (https://cluster-api.sigs.k8s.io/developer/providers/bootstrap.html).
	bootstrapSentinelFile = "/run/cluster-api/bootstrap-success.complete"
)

const (
	// ProviderIDPrefix will be appended to the beginning of Azure resource IDs to form the Kubernetes Provider ID.
	// NOTE: this format matches the 2 slashes format used in cloud-provider and cluster-autoscaler.
	ProviderIDPrefix = "azure://"
)

var (
	// LinuxBootstrapExtensionCommand is the command the VM bootstrap extension will execute to verify Linux nodes bootstrap completes successfully.
	LinuxBootstrapExtensionCommand = fmt.Sprintf("for i in $(seq 1 %d); do test -f %s && break; if [ $i -eq %d ]; then return 1; else sleep %d; fi; done", bootstrapExtensionRetries, bootstrapSentinelFile, bootstrapExtensionRetries, bootstrapExtensionSleep)
	// WindowsBootstrapExtensionCommand is the command the VM bootstrap extension will execute to verify Windows nodes bootstrap completes successfully.
	WindowsBootstrapExtensionCommand = fmt.Sprintf("powershell.exe -Command \"for ($i = 0; $i -lt %d; $i++) {if (Test-Path '%s') {exit 0} else {Start-Sleep -Seconds %d}} exit -2\"",
		bootstrapExtensionRetries, bootstrapSentinelFile, bootstrapExtensionSleep)
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

// GenerateNatGatewayIPName generates a nat gateway IP name.
func GenerateNatGatewayIPName(clusterName, subnetName string) string {
	return fmt.Sprintf("pip-%s-%s-natgw", clusterName, subnetName)
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

// GeneratePrivateDNSZoneName generates the name of a private DNS zone based on the cluster name.
func GeneratePrivateDNSZoneName(clusterName string) string {
	return fmt.Sprintf("%s.capz.io", clusterName)
}

// GeneratePrivateFQDN generates FQDN for a private API Server.
func GeneratePrivateFQDN(clusterName string) string {
	return fmt.Sprintf("%s.%s", PrivateAPIServerHostname, GeneratePrivateDNSZoneName(clusterName))
}

// GenerateVNetLinkName generates the name of a virtual network link name based on the vnet name.
func GenerateVNetLinkName(vnetName string) string {
	return fmt.Sprintf("%s-link", vnetName)
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

// GenerateVnetPeeringName generates the name for a peering between two vnets.
func GenerateVnetPeeringName(sourceVnetName string, remoteVnetName string) string {
	return fmt.Sprintf("%s-To-%s", sourceVnetName, remoteVnetName)
}

// GenerateAvailabilitySetName generates the name of a availability set based on the cluster name and the node group.
// node group identifies the set of nodes that belong to this availability set:
// For control plane nodes, this will be `control-plane`.
// For worker nodes, this will be the machine deployment name.
func GenerateAvailabilitySetName(clusterName, nodeGroup string) string {
	return fmt.Sprintf("%s_%s-as", clusterName, nodeGroup)
}

// WithIndex appends the index as suffix to a generated name.
func WithIndex(name string, n int) string {
	return fmt.Sprintf("%s-%d", name, n)
}

// VMID returns the azure resource ID for a given VM.
func VMID(subscriptionID, resourceGroup, vmName string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/virtualMachines/%s", subscriptionID, resourceGroup, vmName)
}

// VNetID returns the azure resource ID for a given VNet.
func VNetID(subscriptionID, resourceGroup, vnetName string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/virtualNetworks/%s", subscriptionID, resourceGroup, vnetName)
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

// NatGatewayID returns the azure resource ID for a given nat gateway.
func NatGatewayID(subscriptionID, resourceGroup, natgatewayName string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/natGateways/%s", subscriptionID, resourceGroup, natgatewayName)
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

// AvailabilitySetID returns the azure resource ID for a given availability set.
func AvailabilitySetID(subscriptionID, resourceGroup, availabilitySetName string) string {
	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/availabilitySets/%s", subscriptionID, resourceGroup, availabilitySetName)
}

// GetDefaultImageSKUID gets the SKU ID of the image to use for the provided version of Kubernetes.
func getDefaultImageSKUID(k8sVersion, os, osVersion string) (string, error) {
	version, err := semver.ParseTolerant(k8sVersion)
	if err != nil {
		return "", errors.Wrapf(err, "unable to parse Kubernetes version \"%s\" in spec, expected valid SemVer string", k8sVersion)
	}
	return fmt.Sprintf("k8s-%ddot%ddot%d-%s-%s", version.Major, version.Minor, version.Patch, os, osVersion), nil
}

// GetDefaultUbuntuImage returns the default image spec for Ubuntu.
func GetDefaultUbuntuImage(k8sVersion string) (*infrav1.Image, error) {
	v, err := semver.ParseTolerant(k8sVersion)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse Kubernetes version \"%s\"", k8sVersion)
	}
	// Default to Ubuntu 20.04 LTS, except for k8s versions which have only 18.04 reference images.
	osVersion := "2004"
	if (v.Major == 1 && v.Minor == 21 && v.Patch < 2) ||
		(v.Major == 1 && v.Minor == 20 && v.Patch < 8) ||
		(v.Major == 1 && v.Minor == 19 && v.Patch < 12) ||
		(v.Major == 1 && v.Minor == 18 && v.Patch < 20) ||
		(v.Major == 1 && v.Minor < 18) {
		osVersion = "1804"
	}

	skuID, err := getDefaultImageSKUID(k8sVersion, "ubuntu", osVersion)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get default image")
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

// GetDefaultWindowsImage returns the default image spec for Windows.
func GetDefaultWindowsImage(k8sVersion string) (*infrav1.Image, error) {
	skuID, err := getDefaultImageSKUID(k8sVersion, "windows", "2019")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get default image")
	}

	defaultImage := &infrav1.Image{
		Marketplace: &infrav1.AzureMarketplaceImage{
			Publisher: DefaultImagePublisherID,
			Offer:     DefaultWindowsImageOfferID,
			SKU:       skuID,
			Version:   LatestVersion,
		},
	}

	return defaultImage, nil
}

// GetBootstrappingVMExtension returns the CAPZ Bootstrapping VM extension.
// The CAPZ Bootstrapping extension is a simple clone of https://github.com/Azure/custom-script-extension-linux for Linux or
// https://docs.microsoft.com/en-us/azure/virtual-machines/extensions/custom-script-windows for Windows.
// This extension allows running arbitrary scripts on the VM.
// Its role is to detect and report Kubernetes bootstrap failure or success.
func GetBootstrappingVMExtension(osType string, cloud string, vmName string) *ExtensionSpec {
	// currently, the bootstrap extension is only available in AzurePublicCloud.
	if osType == LinuxOS && cloud == azure.PublicCloud.Name {
		// The command checks for the existence of the bootstrapSentinelFile on the machine, with retries and sleep between retries.
		return &ExtensionSpec{
			Name:      "CAPZ.Linux.Bootstrapping",
			VMName:    vmName,
			Publisher: "Microsoft.Azure.ContainerUpstream",
			Version:   "1.0",
			ProtectedSettings: map[string]string{
				"commandToExecute": LinuxBootstrapExtensionCommand,
			},
		}
	} else if osType == WindowsOS && cloud == azure.PublicCloud.Name {
		// This command for the existence of the bootstrapSentinelFile on the machine, with retries and sleep between reties.
		// If the file is not present after the retries are exhausted the extension fails with return code '-2' - ERROR_FILE_NOT_FOUND.
		return &ExtensionSpec{
			Name:      "CAPZ.Windows.Bootstrapping",
			VMName:    vmName,
			Publisher: "Microsoft.Azure.ContainerUpstream",
			Version:   "1.0",
			ProtectedSettings: map[string]string{
				"commandToExecute": WindowsBootstrapExtensionCommand,
			},
		}
	}

	return nil
}

// UserAgent specifies a string to append to the agent identifier.
func UserAgent() string {
	return fmt.Sprintf("cluster-api-provider-azure/%s", version.Get().String())
}

// SetAutoRestClientDefaults set authorizer and user agent for autorest client.
func SetAutoRestClientDefaults(c *autorest.Client, auth autorest.Authorizer) {
	c.Authorizer = auth
	// Wrap the original Sender on the autorest.Client c.
	// The wrapped Sender should set the x-ms-correlation-request-id on the given
	// request, then pass the new request to the underlying Sender.
	c.Sender = autorest.DecorateSender(c.Sender, msCorrelationIDSendDecorator)
	// The default number of retries is 3. This means the client will attempt to retry operation results like resource
	// conflicts (HTTP 409). For a reconciling controller, this is undesirable behavior since if the controller runs
	// into an error reconciling, the controller would be better off to end with an error and try again later.
	//
	// Unfortunately, the naming of this field is a bit misleading. This is not actually "retry attempts", it actually
	// is attempts. Setting this to a value of 0 will cause a panic in Go AutoRest.
	c.RetryAttempts = 1
	AutoRestClientAppendUserAgent(c, UserAgent())
}

// AutoRestClientAppendUserAgent autorest client calls "AddToUserAgent" but ignores errors.
func AutoRestClientAppendUserAgent(c *autorest.Client, extension string) {
	_ = c.AddToUserAgent(extension) // intentionally ignore error as it doesn't matter
}

func msCorrelationIDSendDecorator(snd autorest.Sender) autorest.Sender {
	return autorest.SenderFunc(func(r *http.Request) (*http.Response, error) {
		// if the correlation ID was found in the request context, set
		// it in the header
		if corrID, ok := tele.CorrIDFromCtx(r.Context()); ok {
			r.Header.Set(string(tele.CorrIDKeyVal), string(corrID))
		}
		return snd.Do(r)
	})
}

/*
Copyright 2018 The Kubernetes Authors.

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

package scope

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/utils/net"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/bastionhosts"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/groups"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/loadbalancers"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/natgateways"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/privatedns"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/routetables"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/securitygroups"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/subnets"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/virtualnetworks"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/vnetpeerings"
	"sigs.k8s.io/cluster-api-provider-azure/util/futures"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterScopeParams defines the input parameters used to create a new Scope.
type ClusterScopeParams struct {
	AzureClients
	Client       client.Client
	Cluster      *clusterv1.Cluster
	AzureCluster *infrav1.AzureCluster
	Cache        *ClusterCache
}

// NewClusterScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewClusterScope(ctx context.Context, params ClusterScopeParams) (*ClusterScope, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "azure.clusterScope.NewClusterScope")
	defer done()

	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.AzureCluster == nil {
		return nil, errors.New("failed to generate new scope from nil AzureCluster")
	}

	if params.AzureCluster.Spec.IdentityRef == nil {
		err := params.AzureClients.setCredentials(params.AzureCluster.Spec.SubscriptionID, params.AzureCluster.Spec.AzureEnvironment)
		if err != nil {
			return nil, errors.Wrap(err, "failed to configure azure settings and credentials from environment")
		}
	} else {
		credentialsProvider, err := NewAzureClusterCredentialsProvider(ctx, params.Client, params.AzureCluster)
		if err != nil {
			return nil, errors.Wrap(err, "failed to init credentials provider")
		}
		err = params.AzureClients.setCredentialsWithProvider(ctx, params.AzureCluster.Spec.SubscriptionID, params.AzureCluster.Spec.AzureEnvironment, credentialsProvider)
		if err != nil {
			return nil, errors.Wrap(err, "failed to configure azure settings and credentials for Identity")
		}
	}

	if params.Cache == nil {
		params.Cache = &ClusterCache{}
	}

	helper, err := patch.NewHelper(params.AzureCluster, params.Client)
	if err != nil {
		return nil, errors.Errorf("failed to init patch helper: %v", err)
	}

	return &ClusterScope{
		Client:       params.Client,
		AzureClients: params.AzureClients,
		Cluster:      params.Cluster,
		AzureCluster: params.AzureCluster,
		patchHelper:  helper,
		cache:        params.Cache,
	}, nil
}

// ClusterScope defines the basic context for an actuator to operate upon.
type ClusterScope struct {
	Client      client.Client
	patchHelper *patch.Helper
	cache       *ClusterCache

	AzureClients
	Cluster      *clusterv1.Cluster
	AzureCluster *infrav1.AzureCluster
}

// ClusterCache stores ClusterCache data locally so we don't have to hit the API multiple times within the same reconcile loop.
type ClusterCache struct {
	isVnetManaged *bool
}

// BaseURI returns the Azure ResourceManagerEndpoint.
func (s *ClusterScope) BaseURI() string {
	return s.ResourceManagerEndpoint
}

// Authorizer returns the Azure client Authorizer.
func (s *ClusterScope) Authorizer() autorest.Authorizer {
	return s.AzureClients.Authorizer
}

// PublicIPSpecs returns the public IP specs.
func (s *ClusterScope) PublicIPSpecs() []azure.ResourceSpecGetter {
	var publicIPSpecs []azure.ResourceSpecGetter

	// Public IP specs for control plane lb
	var controlPlaneOutboundIPSpecs []azure.ResourceSpecGetter
	if s.IsAPIServerPrivate() {
		// Public IP specs for control plane outbound lb
		if s.ControlPlaneOutboundLB() != nil {
			for _, ip := range s.ControlPlaneOutboundLB().FrontendIPs {
				controlPlaneOutboundIPSpecs = append(controlPlaneOutboundIPSpecs, &publicips.PublicIPSpec{
					Name:             ip.PublicIP.Name,
					ResourceGroup:    s.ResourceGroup(),
					ClusterName:      s.ClusterName(),
					DNSName:          "",    // Set to default value
					IsIPv6:           false, // Set to default value
					Location:         s.Location(),
					ExtendedLocation: s.ExtendedLocation(),
					FailureDomains:   s.FailureDomains(),
					AdditionalTags:   s.AdditionalTags(),
				})
			}
		}
	} else {
		controlPlaneOutboundIPSpecs = []azure.ResourceSpecGetter{
			&publicips.PublicIPSpec{
				Name:             s.APIServerPublicIP().Name,
				ResourceGroup:    s.ResourceGroup(),
				DNSName:          s.APIServerPublicIP().DNSName,
				IsIPv6:           false, // Currently azure requires an IPv4 lb rule to enable IPv6
				ClusterName:      s.ClusterName(),
				Location:         s.Location(),
				ExtendedLocation: s.ExtendedLocation(),
				FailureDomains:   s.FailureDomains(),
				AdditionalTags:   s.AdditionalTags(),
				IPTags:           s.APIServerPublicIP().IPTags,
			},
		}
	}
	publicIPSpecs = append(publicIPSpecs, controlPlaneOutboundIPSpecs...)

	// Public IP specs for node outbound lb
	if s.NodeOutboundLB() != nil {
		for _, ip := range s.NodeOutboundLB().FrontendIPs {
			publicIPSpecs = append(publicIPSpecs, &publicips.PublicIPSpec{
				Name:             ip.PublicIP.Name,
				ResourceGroup:    s.ResourceGroup(),
				ClusterName:      s.ClusterName(),
				DNSName:          "",    // Set to default value
				IsIPv6:           false, // Set to default value
				Location:         s.Location(),
				ExtendedLocation: s.ExtendedLocation(),
				FailureDomains:   s.FailureDomains(),
				AdditionalTags:   s.AdditionalTags(),
			})
		}
	}

	// Public IP specs for node NAT gateways
	var nodeNatGatewayIPSpecs []azure.ResourceSpecGetter
	for _, subnet := range s.NodeSubnets() {
		if subnet.IsNatGatewayEnabled() {
			nodeNatGatewayIPSpecs = append(nodeNatGatewayIPSpecs, &publicips.PublicIPSpec{
				Name:             subnet.NatGateway.NatGatewayIP.Name,
				ResourceGroup:    s.ResourceGroup(),
				DNSName:          subnet.NatGateway.NatGatewayIP.DNSName,
				IsIPv6:           false, // Public IP is IPv4 by default
				ClusterName:      s.ClusterName(),
				Location:         s.Location(),
				ExtendedLocation: s.ExtendedLocation(),
				FailureDomains:   s.FailureDomains(),
				AdditionalTags:   s.AdditionalTags(),
				IPTags:           subnet.NatGateway.NatGatewayIP.IPTags,
			})
		}
		publicIPSpecs = append(publicIPSpecs, nodeNatGatewayIPSpecs...)
	}

	if azureBastion := s.AzureBastion(); azureBastion != nil {
		// public IP for Azure Bastion.
		azureBastionPublicIP := &publicips.PublicIPSpec{
			Name:             azureBastion.PublicIP.Name,
			ResourceGroup:    s.ResourceGroup(),
			DNSName:          azureBastion.PublicIP.DNSName,
			IsIPv6:           false, // Public IP is IPv4 by default
			ClusterName:      s.ClusterName(),
			Location:         s.Location(),
			ExtendedLocation: s.ExtendedLocation(),
			FailureDomains:   s.FailureDomains(),
			AdditionalTags:   s.AdditionalTags(),
			IPTags:           azureBastion.PublicIP.IPTags,
		}
		publicIPSpecs = append(publicIPSpecs, azureBastionPublicIP)
	}

	return publicIPSpecs
}

// LBSpecs returns the load balancer specs.
func (s *ClusterScope) LBSpecs() []azure.ResourceSpecGetter {
	specs := []azure.ResourceSpecGetter{
		&loadbalancers.LBSpec{
			// API Server LB
			Name:                 s.APIServerLB().Name,
			ResourceGroup:        s.ResourceGroup(),
			SubscriptionID:       s.SubscriptionID(),
			ClusterName:          s.ClusterName(),
			Location:             s.Location(),
			ExtendedLocation:     s.ExtendedLocation(),
			VNetName:             s.Vnet().Name,
			VNetResourceGroup:    s.Vnet().ResourceGroup,
			SubnetName:           s.ControlPlaneSubnet().Name,
			FrontendIPConfigs:    s.APIServerLB().FrontendIPs,
			APIServerPort:        s.APIServerPort(),
			Type:                 s.APIServerLB().Type,
			SKU:                  infrav1.SKUStandard,
			Role:                 infrav1.APIServerRole,
			BackendPoolName:      s.APIServerLBPoolName(s.APIServerLB().Name),
			IdleTimeoutInMinutes: s.APIServerLB().IdleTimeoutInMinutes,
			AdditionalTags:       s.AdditionalTags(),
		},
	}

	// Node outbound LB
	if s.NodeOutboundLB() != nil {
		specs = append(specs, &loadbalancers.LBSpec{
			Name:                 s.NodeOutboundLB().Name,
			ResourceGroup:        s.ResourceGroup(),
			SubscriptionID:       s.SubscriptionID(),
			ClusterName:          s.ClusterName(),
			Location:             s.Location(),
			ExtendedLocation:     s.ExtendedLocation(),
			VNetName:             s.Vnet().Name,
			VNetResourceGroup:    s.Vnet().ResourceGroup,
			FrontendIPConfigs:    s.NodeOutboundLB().FrontendIPs,
			Type:                 s.NodeOutboundLB().Type,
			SKU:                  s.NodeOutboundLB().SKU,
			BackendPoolName:      s.OutboundPoolName(s.NodeOutboundLB().Name),
			IdleTimeoutInMinutes: s.NodeOutboundLB().IdleTimeoutInMinutes,
			Role:                 infrav1.NodeOutboundRole,
			AdditionalTags:       s.AdditionalTags(),
		})
	}

	// Control Plane Outbound LB
	if s.ControlPlaneOutboundLB() != nil {
		specs = append(specs, &loadbalancers.LBSpec{
			Name:                 s.ControlPlaneOutboundLB().Name,
			ResourceGroup:        s.ResourceGroup(),
			SubscriptionID:       s.SubscriptionID(),
			ClusterName:          s.ClusterName(),
			Location:             s.Location(),
			ExtendedLocation:     s.ExtendedLocation(),
			VNetName:             s.Vnet().Name,
			VNetResourceGroup:    s.Vnet().ResourceGroup,
			FrontendIPConfigs:    s.ControlPlaneOutboundLB().FrontendIPs,
			Type:                 s.ControlPlaneOutboundLB().Type,
			SKU:                  s.ControlPlaneOutboundLB().SKU,
			BackendPoolName:      s.OutboundPoolName(azure.GenerateControlPlaneOutboundLBName(s.ClusterName())),
			IdleTimeoutInMinutes: s.NodeOutboundLB().IdleTimeoutInMinutes,
			Role:                 infrav1.ControlPlaneOutboundRole,
			AdditionalTags:       s.AdditionalTags(),
		})
	}

	return specs
}

// RouteTableSpecs returns the subnet route tables.
func (s *ClusterScope) RouteTableSpecs() []azure.ResourceSpecGetter {
	var specs []azure.ResourceSpecGetter
	for _, subnet := range s.AzureCluster.Spec.NetworkSpec.Subnets {
		if subnet.RouteTable.Name != "" {
			specs = append(specs, &routetables.RouteTableSpec{
				Name:           subnet.RouteTable.Name,
				Location:       s.Location(),
				ResourceGroup:  s.ResourceGroup(),
				ClusterName:    s.ClusterName(),
				AdditionalTags: s.AdditionalTags(),
			})
		}
	}

	return specs
}

// NatGatewaySpecs returns the node NAT gateway.
func (s *ClusterScope) NatGatewaySpecs() []azure.ResourceSpecGetter {
	natGatewaySet := make(map[string]struct{})
	var natGateways []azure.ResourceSpecGetter

	// We ignore the control plane NAT gateway, as we will always use a LB to enable egress on the control plane.
	for _, subnet := range s.NodeSubnets() {
		if subnet.IsNatGatewayEnabled() {
			if _, ok := natGatewaySet[subnet.NatGateway.Name]; !ok {
				natGatewaySet[subnet.NatGateway.Name] = struct{}{} // empty struct to represent hash set
				natGateways = append(natGateways, &natgateways.NatGatewaySpec{
					Name:           subnet.NatGateway.Name,
					ResourceGroup:  s.ResourceGroup(),
					SubscriptionID: s.SubscriptionID(),
					Location:       s.Location(),
					ClusterName:    s.ClusterName(),
					NatGatewayIP: infrav1.PublicIPSpec{
						Name: subnet.NatGateway.NatGatewayIP.Name,
					},
					AdditionalTags: s.AdditionalTags(),
				})
			}
		}
	}

	return natGateways
}

// NSGSpecs returns the security group specs.
func (s *ClusterScope) NSGSpecs() []azure.ResourceSpecGetter {
	nsgspecs := make([]azure.ResourceSpecGetter, len(s.AzureCluster.Spec.NetworkSpec.Subnets))
	for i, subnet := range s.AzureCluster.Spec.NetworkSpec.Subnets {
		nsgspecs[i] = &securitygroups.NSGSpec{
			Name:           subnet.SecurityGroup.Name,
			SecurityRules:  subnet.SecurityGroup.SecurityRules,
			ResourceGroup:  s.ResourceGroup(),
			Location:       s.Location(),
			ClusterName:    s.ClusterName(),
			AdditionalTags: s.AdditionalTags(),
		}
	}

	return nsgspecs
}

// SubnetSpecs returns the subnets specs.
func (s *ClusterScope) SubnetSpecs() []azure.ResourceSpecGetter {
	numberOfSubnets := len(s.AzureCluster.Spec.NetworkSpec.Subnets)
	if s.IsAzureBastionEnabled() {
		numberOfSubnets++
	}

	subnetSpecs := make([]azure.ResourceSpecGetter, 0, numberOfSubnets)

	for _, subnet := range s.AzureCluster.Spec.NetworkSpec.Subnets {
		subnetSpec := &subnets.SubnetSpec{
			Name:              subnet.Name,
			ResourceGroup:     s.ResourceGroup(),
			SubscriptionID:    s.SubscriptionID(),
			CIDRs:             subnet.CIDRBlocks,
			VNetName:          s.Vnet().Name,
			VNetResourceGroup: s.Vnet().ResourceGroup,
			IsVNetManaged:     s.IsVnetManaged(),
			RouteTableName:    subnet.RouteTable.Name,
			SecurityGroupName: subnet.SecurityGroup.Name,
			Role:              subnet.Role,
			NatGatewayName:    subnet.NatGateway.Name,
			ServiceEndpoints:  subnet.ServiceEndpoints,
		}
		subnetSpecs = append(subnetSpecs, subnetSpec)
	}

	if s.IsAzureBastionEnabled() {
		azureBastionSubnet := s.AzureCluster.Spec.BastionSpec.AzureBastion.Subnet
		subnetSpecs = append(subnetSpecs, &subnets.SubnetSpec{
			Name:              azureBastionSubnet.Name,
			ResourceGroup:     s.ResourceGroup(),
			SubscriptionID:    s.SubscriptionID(),
			CIDRs:             azureBastionSubnet.CIDRBlocks,
			VNetName:          s.Vnet().Name,
			VNetResourceGroup: s.Vnet().ResourceGroup,
			IsVNetManaged:     s.IsVnetManaged(),
			SecurityGroupName: azureBastionSubnet.SecurityGroup.Name,
			RouteTableName:    azureBastionSubnet.RouteTable.Name,
			Role:              azureBastionSubnet.Role,
			ServiceEndpoints:  azureBastionSubnet.ServiceEndpoints,
		})
	}

	return subnetSpecs
}

// GroupSpec returns the resource group spec.
func (s *ClusterScope) GroupSpec() azure.ResourceSpecGetter {
	return &groups.GroupSpec{
		Name:           s.ResourceGroup(),
		Location:       s.Location(),
		ClusterName:    s.ClusterName(),
		AdditionalTags: s.AdditionalTags(),
	}
}

// VnetPeeringSpecs returns the virtual network peering specs.
func (s *ClusterScope) VnetPeeringSpecs() []azure.ResourceSpecGetter {
	peeringSpecs := make([]azure.ResourceSpecGetter, 2*len(s.Vnet().Peerings))
	for i, peering := range s.Vnet().Peerings {
		forwardPeering := &vnetpeerings.VnetPeeringSpec{
			PeeringName:         azure.GenerateVnetPeeringName(s.Vnet().Name, peering.RemoteVnetName),
			SourceVnetName:      s.Vnet().Name,
			SourceResourceGroup: s.Vnet().ResourceGroup,
			RemoteVnetName:      peering.RemoteVnetName,
			RemoteResourceGroup: peering.ResourceGroup,
			SubscriptionID:      s.SubscriptionID(),
		}
		reversePeering := &vnetpeerings.VnetPeeringSpec{
			PeeringName:         azure.GenerateVnetPeeringName(peering.RemoteVnetName, s.Vnet().Name),
			SourceVnetName:      peering.RemoteVnetName,
			SourceResourceGroup: peering.ResourceGroup,
			RemoteVnetName:      s.Vnet().Name,
			RemoteResourceGroup: s.Vnet().ResourceGroup,
			SubscriptionID:      s.SubscriptionID(),
		}
		peeringSpecs[i*2] = forwardPeering
		peeringSpecs[i*2+1] = reversePeering
	}

	return peeringSpecs
}

// VNetSpec returns the virtual network spec.
func (s *ClusterScope) VNetSpec() azure.ResourceSpecGetter {
	return &virtualnetworks.VNetSpec{
		ResourceGroup:    s.Vnet().ResourceGroup,
		Name:             s.Vnet().Name,
		CIDRs:            s.Vnet().CIDRBlocks,
		ExtendedLocation: s.ExtendedLocation(),
		Location:         s.Location(),
		ClusterName:      s.ClusterName(),
		AdditionalTags:   s.AdditionalTags(),
	}
}

// PrivateDNSSpec returns the private dns zone spec.
func (s *ClusterScope) PrivateDNSSpec() (zoneSpec azure.ResourceSpecGetter, linkSpec, recordSpec []azure.ResourceSpecGetter) {
	if s.IsAPIServerPrivate() {
		zone := privatedns.ZoneSpec{
			Name:           s.GetPrivateDNSZoneName(),
			ResourceGroup:  s.ResourceGroup(),
			ClusterName:    s.ClusterName(),
			AdditionalTags: s.AdditionalTags(),
		}

		links := make([]azure.ResourceSpecGetter, 1+len(s.Vnet().Peerings))
		links[0] = privatedns.LinkSpec{
			Name:              azure.GenerateVNetLinkName(s.Vnet().Name),
			ZoneName:          s.GetPrivateDNSZoneName(),
			SubscriptionID:    s.SubscriptionID(),
			VNetResourceGroup: s.Vnet().ResourceGroup,
			VNetName:          s.Vnet().Name,
			ResourceGroup:     s.ResourceGroup(),
			ClusterName:       s.ClusterName(),
			AdditionalTags:    s.AdditionalTags(),
		}
		for i, peering := range s.Vnet().Peerings {
			links[i+1] = privatedns.LinkSpec{
				Name:              azure.GenerateVNetLinkName(peering.RemoteVnetName),
				ZoneName:          s.GetPrivateDNSZoneName(),
				SubscriptionID:    s.SubscriptionID(),
				VNetResourceGroup: peering.ResourceGroup,
				VNetName:          peering.RemoteVnetName,
				ResourceGroup:     s.ResourceGroup(),
				ClusterName:       s.ClusterName(),
				AdditionalTags:    s.AdditionalTags(),
			}
		}

		records := make([]azure.ResourceSpecGetter, 1)
		records[0] = privatedns.RecordSpec{
			Record: infrav1.AddressRecord{
				Hostname: azure.PrivateAPIServerHostname,
				IP:       s.APIServerPrivateIP(),
			},
			ZoneName:      s.GetPrivateDNSZoneName(),
			ResourceGroup: s.ResourceGroup(),
		}

		return zone, links, records
	}

	return nil, nil, nil
}

// IsAzureBastionEnabled returns true if the azure bastion is enabled.
func (s *ClusterScope) IsAzureBastionEnabled() bool {
	return s.AzureCluster.Spec.BastionSpec.AzureBastion != nil
}

// AzureBastion returns the cluster AzureBastion.
func (s *ClusterScope) AzureBastion() *infrav1.AzureBastion {
	return s.AzureCluster.Spec.BastionSpec.AzureBastion
}

// AzureBastionSpec returns the bastion spec.
func (s *ClusterScope) AzureBastionSpec() azure.ResourceSpecGetter {
	if s.IsAzureBastionEnabled() {
		subnetID := azure.SubnetID(s.SubscriptionID(), s.ResourceGroup(), s.Vnet().Name, s.AzureBastion().Subnet.Name)
		publicIPID := azure.PublicIPID(s.SubscriptionID(), s.ResourceGroup(), s.AzureBastion().PublicIP.Name)

		return &bastionhosts.AzureBastionSpec{
			Name:          s.AzureBastion().Name,
			ResourceGroup: s.ResourceGroup(),
			Location:      s.Location(),
			ClusterName:   s.ClusterName(),
			SubnetID:      subnetID,
			PublicIPID:    publicIPID,
		}
	}

	return nil
}

// Vnet returns the cluster Vnet.
func (s *ClusterScope) Vnet() *infrav1.VnetSpec {
	return &s.AzureCluster.Spec.NetworkSpec.Vnet
}

// IsVnetManaged returns true if the vnet is managed.
func (s *ClusterScope) IsVnetManaged() bool {
	if s.cache.isVnetManaged != nil {
		return to.Bool(s.cache.isVnetManaged)
	}
	isVnetManaged := s.Vnet().ID == "" || s.Vnet().Tags.HasOwned(s.ClusterName())
	s.cache.isVnetManaged = to.BoolPtr(isVnetManaged)
	return isVnetManaged
}

// IsIPv6Enabled returns true if IPv6 is enabled.
func (s *ClusterScope) IsIPv6Enabled() bool {
	for _, cidr := range s.AzureCluster.Spec.NetworkSpec.Vnet.CIDRBlocks {
		if net.IsIPv6CIDRString(cidr) {
			return true
		}
	}
	return false
}

// Subnets returns the cluster subnets.
func (s *ClusterScope) Subnets() infrav1.Subnets {
	return s.AzureCluster.Spec.NetworkSpec.Subnets
}

// ControlPlaneSubnet returns the cluster control plane subnet.
func (s *ClusterScope) ControlPlaneSubnet() infrav1.SubnetSpec {
	subnet, _ := s.AzureCluster.Spec.NetworkSpec.GetControlPlaneSubnet()
	return subnet
}

// NodeSubnets returns the subnets with the node role.
func (s *ClusterScope) NodeSubnets() []infrav1.SubnetSpec {
	subnets := []infrav1.SubnetSpec{}
	for _, subnet := range s.AzureCluster.Spec.NetworkSpec.Subnets {
		if subnet.Role == infrav1.SubnetNode {
			subnets = append(subnets, subnet)
		}
	}

	return subnets
}

// Subnet returns the subnet with the provided name.
func (s *ClusterScope) Subnet(name string) infrav1.SubnetSpec {
	for _, sn := range s.AzureCluster.Spec.NetworkSpec.Subnets {
		if sn.Name == name {
			return sn
		}
	}

	return infrav1.SubnetSpec{}
}

// SetSubnet sets the subnet spec for the subnet with the same name.
func (s *ClusterScope) SetSubnet(subnetSpec infrav1.SubnetSpec) {
	for i, sn := range s.AzureCluster.Spec.NetworkSpec.Subnets {
		if sn.Name == subnetSpec.Name {
			s.AzureCluster.Spec.NetworkSpec.Subnets[i] = subnetSpec
			return
		}
	}
}

// SetNatGatewayIDInSubnets sets the NAT Gateway ID in the subnets with the same name.
func (s *ClusterScope) SetNatGatewayIDInSubnets(name string, id string) {
	for _, subnet := range s.Subnets() {
		if subnet.NatGateway.Name == name {
			subnet.NatGateway.ID = id
			s.SetSubnet(subnet)
		}
	}
}

// UpdateSubnetCIDRs updates the subnet CIDRs for the subnet with the same name.
func (s *ClusterScope) UpdateSubnetCIDRs(name string, cidrBlocks []string) {
	subnetSpecInfra := s.Subnet(name)
	subnetSpecInfra.CIDRBlocks = cidrBlocks
	s.SetSubnet(subnetSpecInfra)
}

// UpdateSubnetID updates the subnet ID for the subnet with the same name.
func (s *ClusterScope) UpdateSubnetID(name string, id string) {
	subnetSpecInfra := s.Subnet(name)
	subnetSpecInfra.ID = id
	s.SetSubnet(subnetSpecInfra)
}

// ControlPlaneRouteTable returns the cluster controlplane routetable.
func (s *ClusterScope) ControlPlaneRouteTable() infrav1.RouteTable {
	subnet, _ := s.AzureCluster.Spec.NetworkSpec.GetControlPlaneSubnet()
	return subnet.RouteTable
}

// APIServerLB returns the cluster API Server load balancer.
func (s *ClusterScope) APIServerLB() *infrav1.LoadBalancerSpec {
	return &s.AzureCluster.Spec.NetworkSpec.APIServerLB
}

// NodeOutboundLB returns the cluster node outbound load balancer.
func (s *ClusterScope) NodeOutboundLB() *infrav1.LoadBalancerSpec {
	return s.AzureCluster.Spec.NetworkSpec.NodeOutboundLB
}

// ControlPlaneOutboundLB returns the cluster control plane outbound load balancer.
func (s *ClusterScope) ControlPlaneOutboundLB() *infrav1.LoadBalancerSpec {
	return s.AzureCluster.Spec.NetworkSpec.ControlPlaneOutboundLB
}

// APIServerLBName returns the API Server LB name.
func (s *ClusterScope) APIServerLBName() string {
	return s.APIServerLB().Name
}

// IsAPIServerPrivate returns true if the API Server LB is of type Internal.
func (s *ClusterScope) IsAPIServerPrivate() bool {
	return s.APIServerLB().Type == infrav1.Internal
}

// APIServerPublicIP returns the API Server public IP.
func (s *ClusterScope) APIServerPublicIP() *infrav1.PublicIPSpec {
	return s.APIServerLB().FrontendIPs[0].PublicIP
}

// APIServerPrivateIP returns the API Server private IP.
func (s *ClusterScope) APIServerPrivateIP() string {
	return s.APIServerLB().FrontendIPs[0].PrivateIPAddress
}

// GetPrivateDNSZoneName returns the Private DNS Zone from the spec or generate it from cluster name.
func (s *ClusterScope) GetPrivateDNSZoneName() string {
	if len(s.AzureCluster.Spec.NetworkSpec.PrivateDNSZoneName) > 0 {
		return s.AzureCluster.Spec.NetworkSpec.PrivateDNSZoneName
	}
	return azure.GeneratePrivateDNSZoneName(s.ClusterName())
}

// APIServerLBPoolName returns the API Server LB backend pool name.
func (s *ClusterScope) APIServerLBPoolName(loadBalancerName string) string {
	return azure.GenerateBackendAddressPoolName(loadBalancerName)
}

// OutboundLBName returns the name of the outbound LB.
func (s *ClusterScope) OutboundLBName(role string) string {
	if role == infrav1.Node {
		if s.NodeOutboundLB() == nil {
			return ""
		}
		return s.NodeOutboundLB().Name
	}
	if s.IsAPIServerPrivate() {
		if s.ControlPlaneOutboundLB() == nil {
			return ""
		}
		return s.ControlPlaneOutboundLB().Name
	}
	return s.APIServerLBName()
}

// OutboundPoolName returns the outbound LB backend pool name.
func (s *ClusterScope) OutboundPoolName(loadBalancerName string) string {
	if loadBalancerName == "" {
		return ""
	}
	return azure.GenerateOutboundBackendAddressPoolName(loadBalancerName)
}

// ResourceGroup returns the cluster resource group.
func (s *ClusterScope) ResourceGroup() string {
	return s.AzureCluster.Spec.ResourceGroup
}

// ClusterName returns the cluster name.
func (s *ClusterScope) ClusterName() string {
	return s.Cluster.Name
}

// Namespace returns the cluster namespace.
func (s *ClusterScope) Namespace() string {
	return s.Cluster.Namespace
}

// Location returns the cluster location.
func (s *ClusterScope) Location() string {
	return s.AzureCluster.Spec.Location
}

// AvailabilitySetEnabled informs machines that they should be part of an Availability Set.
func (s *ClusterScope) AvailabilitySetEnabled() bool {
	return len(s.AzureCluster.Status.FailureDomains) == 0
}

// CloudProviderConfigOverrides returns the cloud provider config overrides for the cluster.
func (s *ClusterScope) CloudProviderConfigOverrides() *infrav1.CloudProviderConfigOverrides {
	return s.AzureCluster.Spec.CloudProviderConfigOverrides
}

// ExtendedLocation returns the cluster extendedLocation.
func (s *ClusterScope) ExtendedLocation() *infrav1.ExtendedLocationSpec {
	return s.AzureCluster.Spec.ExtendedLocation
}

// GenerateFQDN generates a fully qualified domain name, based on a hash, cluster name and cluster location.
func (s *ClusterScope) GenerateFQDN(ipName string) string {
	h := fnv.New32a()
	if _, err := fmt.Fprintf(h, "%s/%s/%s", s.SubscriptionID(), s.ResourceGroup(), ipName); err != nil {
		return ""
	}
	hash := fmt.Sprintf("%x", h.Sum32())
	return strings.ToLower(fmt.Sprintf("%s-%s.%s.%s", s.ClusterName(), hash, s.Location(), s.AzureClients.ResourceManagerVMDNSSuffix))
}

// GenerateLegacyFQDN generates an IP name and a fully qualified domain name, based on a hash, cluster name and cluster location.
// Deprecated: use GenerateFQDN instead.
func (s *ClusterScope) GenerateLegacyFQDN() (ip string, domain string) {
	h := fnv.New32a()
	if _, err := fmt.Fprintf(h, "%s/%s/%s", s.SubscriptionID(), s.ResourceGroup(), s.ClusterName()); err != nil {
		return "", ""
	}
	ipName := fmt.Sprintf("%s-%x", s.ClusterName(), h.Sum32())
	fqdn := fmt.Sprintf("%s.%s.%s", ipName, s.Location(), s.AzureClients.ResourceManagerVMDNSSuffix)
	return ipName, fqdn
}

// ListOptionsLabelSelector returns a ListOptions with a label selector for clusterName.
func (s *ClusterScope) ListOptionsLabelSelector() client.ListOption {
	return client.MatchingLabels(map[string]string{
		clusterv1.ClusterLabelName: s.Cluster.Name,
	})
}

// PatchObject persists the cluster configuration and status.
func (s *ClusterScope) PatchObject(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scope.ClusterScope.PatchObject")
	defer done()

	conditions.SetSummary(s.AzureCluster)

	return s.patchHelper.Patch(
		ctx,
		s.AzureCluster,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			infrav1.ResourceGroupReadyCondition,
			infrav1.RouteTablesReadyCondition,
			infrav1.NetworkInfrastructureReadyCondition,
			infrav1.VnetPeeringReadyCondition,
			infrav1.DisksReadyCondition,
			infrav1.NATGatewaysReadyCondition,
			infrav1.LoadBalancersReadyCondition,
			infrav1.BastionHostReadyCondition,
			infrav1.VNetReadyCondition,
			infrav1.SubnetsReadyCondition,
			infrav1.SecurityGroupsReadyCondition,
			infrav1.PrivateDNSZoneReadyCondition,
			infrav1.PrivateDNSLinkReadyCondition,
			infrav1.PrivateDNSRecordReadyCondition,
		}})
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ClusterScope) Close(ctx context.Context) error {
	return s.PatchObject(ctx)
}

// AdditionalTags returns AdditionalTags from the scope's AzureCluster.
func (s *ClusterScope) AdditionalTags() infrav1.Tags {
	tags := make(infrav1.Tags)
	if s.AzureCluster.Spec.AdditionalTags != nil {
		tags = s.AzureCluster.Spec.AdditionalTags.DeepCopy()
	}
	return tags
}

// APIServerPort returns the APIServerPort to use when creating the load balancer.
func (s *ClusterScope) APIServerPort() int32 {
	if s.Cluster.Spec.ClusterNetwork != nil && s.Cluster.Spec.ClusterNetwork.APIServerPort != nil {
		return *s.Cluster.Spec.ClusterNetwork.APIServerPort
	}
	return 6443
}

// APIServerHost returns the hostname used to reach the API server.
func (s *ClusterScope) APIServerHost() string {
	if s.IsAPIServerPrivate() {
		return azure.GeneratePrivateFQDN(s.GetPrivateDNSZoneName())
	}
	return s.APIServerPublicIP().DNSName
}

// SetFailureDomain will set the spec for a for a given key.
func (s *ClusterScope) SetFailureDomain(id string, spec clusterv1.FailureDomainSpec) {
	if s.AzureCluster.Status.FailureDomains == nil {
		s.AzureCluster.Status.FailureDomains = make(clusterv1.FailureDomains)
	}
	s.AzureCluster.Status.FailureDomains[id] = spec
}

// FailureDomains returns the failure domains for the cluster.
func (s *ClusterScope) FailureDomains() []string {
	fds := make([]string, len(s.AzureCluster.Status.FailureDomains))
	i := 0
	for id := range s.AzureCluster.Status.FailureDomains {
		fds[i] = id
		i++
	}

	sort.Strings(fds)

	return fds
}

// SetControlPlaneSecurityRules sets the default security rules of the control plane subnet.
// Note that this is not done in a webhook as it requires a valid Cluster object to exist to get the API Server port.
func (s *ClusterScope) SetControlPlaneSecurityRules() {
	if s.ControlPlaneSubnet().SecurityGroup.SecurityRules == nil {
		subnet := s.ControlPlaneSubnet()
		subnet.SecurityGroup.SecurityRules = infrav1.SecurityRules{
			infrav1.SecurityRule{
				Name:             "allow_ssh",
				Description:      "Allow SSH",
				Priority:         2200,
				Protocol:         infrav1.SecurityGroupProtocolTCP,
				Direction:        infrav1.SecurityRuleDirectionInbound,
				Source:           to.StringPtr("*"),
				SourcePorts:      to.StringPtr("*"),
				Destination:      to.StringPtr("*"),
				DestinationPorts: to.StringPtr("22"),
			},
			infrav1.SecurityRule{
				Name:             "allow_apiserver",
				Description:      "Allow K8s API Server",
				Priority:         2201,
				Protocol:         infrav1.SecurityGroupProtocolTCP,
				Direction:        infrav1.SecurityRuleDirectionInbound,
				Source:           to.StringPtr("*"),
				SourcePorts:      to.StringPtr("*"),
				Destination:      to.StringPtr("*"),
				DestinationPorts: to.StringPtr(strconv.Itoa(int(s.APIServerPort()))),
			},
		}
		s.AzureCluster.Spec.NetworkSpec.UpdateControlPlaneSubnet(subnet)
	}
}

// SetDNSName sets the API Server public IP DNS name.
// Note: this logic exists only for purposes of ensuring backwards compatibility for old clusters created without an APIServerLB, and should be removed in the future.
func (s *ClusterScope) SetDNSName() {
	// for back compat, set the old API Server defaults if no API Server Spec has been set by new webhooks.
	lb := s.APIServerLB()
	if lb == nil || lb.Name == "" {
		lbName := fmt.Sprintf("%s-%s", s.ClusterName(), "public-lb")
		ip, dns := s.GenerateLegacyFQDN()
		lb = &infrav1.LoadBalancerSpec{
			Name: lbName,
			FrontendIPs: []infrav1.FrontendIP{
				{
					Name: azure.GenerateFrontendIPConfigName(lbName),
					PublicIP: &infrav1.PublicIPSpec{
						Name:    ip,
						DNSName: dns,
					},
				},
			},
			LoadBalancerClassSpec: infrav1.LoadBalancerClassSpec{
				SKU:  infrav1.SKUStandard,
				Type: infrav1.Public,
			},
		}
		lb.DeepCopyInto(s.APIServerLB())
	}
	// Generate valid FQDN if not set.
	// Note: this function uses the AzureCluster subscription ID.
	if !s.IsAPIServerPrivate() && s.APIServerPublicIP().DNSName == "" {
		s.APIServerPublicIP().DNSName = s.GenerateFQDN(s.APIServerPublicIP().Name)
	}
}

// SetLongRunningOperationState will set the future on the AzureCluster status to allow the resource to continue
// in the next reconciliation.
func (s *ClusterScope) SetLongRunningOperationState(future *infrav1.Future) {
	futures.Set(s.AzureCluster, future)
}

// GetLongRunningOperationState will get the future on the AzureCluster status.
func (s *ClusterScope) GetLongRunningOperationState(name, service, futureType string) *infrav1.Future {
	return futures.Get(s.AzureCluster, name, service, futureType)
}

// DeleteLongRunningOperationState will delete the future from the AzureCluster status.
func (s *ClusterScope) DeleteLongRunningOperationState(name, service, futureType string) {
	futures.Delete(s.AzureCluster, name, service, futureType)
}

// UpdateDeleteStatus updates a condition on the AzureCluster status after a DELETE operation.
func (s *ClusterScope) UpdateDeleteStatus(condition clusterv1.ConditionType, service string, err error) {
	switch {
	case err == nil:
		conditions.MarkFalse(s.AzureCluster, condition, infrav1.DeletedReason, clusterv1.ConditionSeverityInfo, "%s successfully deleted", service)
	case azure.IsOperationNotDoneError(err):
		conditions.MarkFalse(s.AzureCluster, condition, infrav1.DeletingReason, clusterv1.ConditionSeverityInfo, "%s deleting", service)
	default:
		conditions.MarkFalse(s.AzureCluster, condition, infrav1.DeletionFailedReason, clusterv1.ConditionSeverityError, "%s failed to delete. err: %s", service, err.Error())
	}
}

// UpdatePutStatus updates a condition on the AzureCluster status after a PUT operation.
func (s *ClusterScope) UpdatePutStatus(condition clusterv1.ConditionType, service string, err error) {
	switch {
	case err == nil:
		conditions.MarkTrue(s.AzureCluster, condition)
	case azure.IsOperationNotDoneError(err):
		conditions.MarkFalse(s.AzureCluster, condition, infrav1.CreatingReason, clusterv1.ConditionSeverityInfo, "%s creating or updating", service)
	default:
		conditions.MarkFalse(s.AzureCluster, condition, infrav1.FailedReason, clusterv1.ConditionSeverityError, "%s failed to create or update. err: %s", service, err.Error())
	}
}

// UpdatePatchStatus updates a condition on the AzureCluster status after a PATCH operation.
func (s *ClusterScope) UpdatePatchStatus(condition clusterv1.ConditionType, service string, err error) {
	switch {
	case err == nil:
		conditions.MarkTrue(s.AzureCluster, condition)
	case azure.IsOperationNotDoneError(err):
		conditions.MarkFalse(s.AzureCluster, condition, infrav1.UpdatingReason, clusterv1.ConditionSeverityInfo, "%s updating", service)
	default:
		conditions.MarkFalse(s.AzureCluster, condition, infrav1.FailedReason, clusterv1.ConditionSeverityError, "%s failed to update. err: %s", service, err.Error())
	}
}

// AnnotationJSON returns a map[string]interface from a JSON annotation.
func (s *ClusterScope) AnnotationJSON(annotation string) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	jsonAnnotation := s.AzureCluster.GetAnnotations()[annotation]
	if jsonAnnotation == "" {
		return out, nil
	}
	err := json.Unmarshal([]byte(jsonAnnotation), &out)
	if err != nil {
		return out, err
	}
	return out, nil
}

// UpdateAnnotationJSON updates the `annotation` with
// `content`. `content` in this case should be a `map[string]interface{}`
// suitable for turning into JSON. This `content` map will be marshalled into a
// JSON string before being set as the given `annotation`.
func (s *ClusterScope) UpdateAnnotationJSON(annotation string, content map[string]interface{}) error {
	b, err := json.Marshal(content)
	if err != nil {
		return err
	}
	s.SetAnnotation(annotation, string(b))
	return nil
}

// SetAnnotation sets a key value annotation on the AzureCluster.
func (s *ClusterScope) SetAnnotation(key, value string) {
	if s.AzureCluster.Annotations == nil {
		s.AzureCluster.Annotations = map[string]string{}
	}
	s.AzureCluster.Annotations[key] = value
}

// TagsSpecs returns the tag specs for the AzureCluster.
func (s *ClusterScope) TagsSpecs() []azure.TagsSpec {
	return []azure.TagsSpec{
		{
			Scope:      azure.ResourceGroupID(s.SubscriptionID(), s.ResourceGroup()),
			Tags:       s.AdditionalTags(),
			Annotation: azure.RGTagsLastAppliedAnnotation,
		},
	}
}

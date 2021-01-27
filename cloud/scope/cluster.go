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
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/klog/klogr"
	"k8s.io/utils/net"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// ClusterScopeParams defines the input parameters used to create a new Scope.
type ClusterScopeParams struct {
	AzureClients
	Client       client.Client
	Logger       logr.Logger
	Cluster      *clusterv1.Cluster
	AzureCluster *infrav1.AzureCluster
}

// NewClusterScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewClusterScope(ctx context.Context, params ClusterScopeParams) (*ClusterScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.AzureCluster == nil {
		return nil, errors.New("failed to generate new scope from nil AzureCluster")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	if params.AzureCluster.Spec.IdentityRef == nil {
		err := params.AzureClients.setCredentials(params.AzureCluster.Spec.SubscriptionID)
		if err != nil {
			return nil, errors.Wrap(err, "failed to configure azure settings and credentials from environment")
		}
	} else {
		credentailsProvider, err := NewAzureCredentialsProvider(ctx, params.Client, params.AzureCluster)
		if err != nil {
			return nil, errors.Wrap(err, "failed to init credentials provider")
		}
		err = params.AzureClients.setCredentialsWithProvider(ctx, params.AzureCluster.Spec.SubscriptionID, credentailsProvider)
		if err != nil {
			return nil, errors.Wrap(err, "failed to configure azure settings and credentials for Identity")
		}
	}

	helper, err := patch.NewHelper(params.AzureCluster, params.Client)
	if err != nil {
		return nil, errors.Errorf("failed to init patch helper: %v", err)
	}

	return &ClusterScope{
		Logger:       params.Logger,
		Client:       params.Client,
		AzureClients: params.AzureClients,
		Cluster:      params.Cluster,
		AzureCluster: params.AzureCluster,
		patchHelper:  helper,
	}, nil
}

// ClusterScope defines the basic context for an actuator to operate upon.
type ClusterScope struct {
	logr.Logger
	Client      client.Client
	patchHelper *patch.Helper

	AzureClients
	Cluster      *clusterv1.Cluster
	AzureCluster *infrav1.AzureCluster
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
func (s *ClusterScope) PublicIPSpecs() []azure.PublicIPSpec {
	var controlPlaneOutboundIP azure.PublicIPSpec
	if s.IsAPIServerPrivate() {
		controlPlaneOutboundIP = azure.PublicIPSpec{
			Name: azure.GenerateControlPlaneOutboundIPName(s.ClusterName()),
		}
	} else {
		controlPlaneOutboundIP = azure.PublicIPSpec{
			Name:    s.APIServerPublicIP().Name,
			DNSName: s.APIServerPublicIP().DNSName,
			IsIPv6:  false, // currently azure requires a ipv4 lb rule to enable ipv6
		}
	}

	return []azure.PublicIPSpec{
		controlPlaneOutboundIP,
		{
			Name: azure.GenerateNodeOutboundIPName(s.ClusterName()),
		},
	}
}

// LBSpecs returns the load balancer specs.
func (s *ClusterScope) LBSpecs() []azure.LBSpec {
	specs := []azure.LBSpec{
		{
			// Control Plane LB
			Name:              s.APIServerLB().Name,
			SubnetName:        s.ControlPlaneSubnet().Name,
			FrontendIPConfigs: s.APIServerLB().FrontendIPs,
			APIServerPort:     s.APIServerPort(),
			Type:              s.APIServerLB().Type,
			SKU:               infrav1.SKUStandard,
			Role:              infrav1.APIServerRole,
			BackendPoolName:   s.APIServerLBPoolName(s.APIServerLB().Name),
		},
		{
			// Public Node outbound LB
			Name: s.NodeOutboundLBName(),
			FrontendIPConfigs: []infrav1.FrontendIP{
				{
					Name: azure.GenerateFrontendIPConfigName(s.NodeOutboundLBName()),
					PublicIP: &infrav1.PublicIPSpec{
						Name: azure.GenerateNodeOutboundIPName(s.ClusterName()),
					},
				},
			},
			Type:            infrav1.Public,
			SKU:             infrav1.SKUStandard,
			BackendPoolName: s.OutboundPoolName(s.NodeOutboundLBName()),
			Role:            infrav1.NodeOutboundRole,
		},
	}

	if !s.IsAPIServerPrivate() {
		return specs
	}

	specs = append(specs, azure.LBSpec{
		// Public Control Plane outbound LB
		Name: azure.GenerateControlPlaneOutboundLBName(s.ClusterName()),
		FrontendIPConfigs: []infrav1.FrontendIP{
			{
				Name: azure.GenerateFrontendIPConfigName(azure.GenerateControlPlaneOutboundLBName(s.ClusterName())),
				PublicIP: &infrav1.PublicIPSpec{
					Name: azure.GenerateControlPlaneOutboundIPName(s.ClusterName()),
				},
			},
		},
		Type:            infrav1.Public,
		SKU:             infrav1.SKUStandard,
		BackendPoolName: s.OutboundPoolName(azure.GenerateControlPlaneOutboundLBName(s.ClusterName())),
		Role:            infrav1.ControlPlaneOutboundRole,
	})

	return specs
}

// RouteTableSpecs returns the node route table
func (s *ClusterScope) RouteTableSpecs() []azure.RouteTableSpec {
	routetables := []azure.RouteTableSpec{}
	if s.ControlPlaneRouteTable().Name != "" {
		routetables = append(routetables, azure.RouteTableSpec{Name: s.ControlPlaneRouteTable().Name, Subnet: s.ControlPlaneSubnet()})
	}
	if s.NodeRouteTable().Name != "" {
		routetables = append(routetables, azure.RouteTableSpec{Name: s.NodeRouteTable().Name, Subnet: s.NodeSubnet()})
	}
	return routetables
}

// NSGSpecs returns the security group specs.
func (s *ClusterScope) NSGSpecs() []azure.NSGSpec {
	return []azure.NSGSpec{
		{
			Name:         s.ControlPlaneSubnet().SecurityGroup.Name,
			IngressRules: s.ControlPlaneSubnet().SecurityGroup.IngressRules,
		},
		{
			Name:         s.NodeSubnet().SecurityGroup.Name,
			IngressRules: s.NodeSubnet().SecurityGroup.IngressRules,
		},
	}
}

// SubnetSpecs returns the subnets specs.
func (s *ClusterScope) SubnetSpecs() []azure.SubnetSpec {
	return []azure.SubnetSpec{
		{
			Name:              s.ControlPlaneSubnet().Name,
			CIDRs:             s.ControlPlaneSubnet().CIDRBlocks,
			VNetName:          s.Vnet().Name,
			SecurityGroupName: s.ControlPlaneSubnet().SecurityGroup.Name,
			Role:              s.ControlPlaneSubnet().Role,
			RouteTableName:    s.ControlPlaneSubnet().RouteTable.Name,
		},
		{
			Name:              s.NodeSubnet().Name,
			CIDRs:             s.NodeSubnet().CIDRBlocks,
			VNetName:          s.Vnet().Name,
			SecurityGroupName: s.NodeSubnet().SecurityGroup.Name,
			RouteTableName:    s.NodeSubnet().RouteTable.Name,
			Role:              s.NodeSubnet().Role,
		},
	}
}

// VNetSpec returns the virtual network spec.
func (s *ClusterScope) VNetSpec() azure.VNetSpec {
	return azure.VNetSpec{
		ResourceGroup: s.Vnet().ResourceGroup,
		Name:          s.Vnet().Name,
		CIDRs:         s.Vnet().CIDRBlocks,
	}
}

// PrivateDNSSpec returns the private dns zone spec.
func (s *ClusterScope) PrivateDNSSpec() *azure.PrivateDNSSpec {
	var spec *azure.PrivateDNSSpec
	if s.IsAPIServerPrivate() {
		spec = &azure.PrivateDNSSpec{
			ZoneName:          azure.GeneratePrivateDNSZoneName(s.ClusterName()),
			VNetName:          s.Vnet().Name,
			VNetResourceGroup: s.Vnet().ResourceGroup,
			LinkName:          azure.GenerateVNetLinkName(s.Vnet().Name),
			Records: []infrav1.AddressRecord{
				{
					Hostname: azure.PrivateAPIServerHostname,
					IP:       s.APIServerPrivateIP(),
				},
			},
		}
	}
	return spec
}

// Vnet returns the cluster Vnet.
func (s *ClusterScope) Vnet() *infrav1.VnetSpec {
	return &s.AzureCluster.Spec.NetworkSpec.Vnet
}

// IsVnetManaged returns true if the vnet is managed.
func (s *ClusterScope) IsVnetManaged() bool {
	return s.Vnet().ID == "" || s.Vnet().Tags.HasOwned(s.ClusterName())
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
func (s *ClusterScope) ControlPlaneSubnet() *infrav1.SubnetSpec {
	return s.AzureCluster.Spec.NetworkSpec.GetControlPlaneSubnet()
}

// NodeSubnet returns the cluster node subnet.
func (s *ClusterScope) NodeSubnet() *infrav1.SubnetSpec {
	return s.AzureCluster.Spec.NetworkSpec.GetNodeSubnet()
}

// ControlPlaneRouteTable returns the cluster controlplane routetable.
func (s *ClusterScope) ControlPlaneRouteTable() *infrav1.RouteTable {
	return &s.AzureCluster.Spec.NetworkSpec.GetControlPlaneSubnet().RouteTable
}

// NodeRouteTable returns the cluster node routetable.
func (s *ClusterScope) NodeRouteTable() *infrav1.RouteTable {
	return &s.AzureCluster.Spec.NetworkSpec.GetNodeSubnet().RouteTable
}

// APIServerLB returns the cluster API Server load balancer.
func (s *ClusterScope) APIServerLB() *infrav1.LoadBalancerSpec {
	return &s.AzureCluster.Spec.NetworkSpec.APIServerLB
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

// APIServerLBPoolName returns the API Server LB backend pool name.
func (s *ClusterScope) APIServerLBPoolName(loadBalancerName string) string {
	return azure.GenerateBackendAddressPoolName(loadBalancerName)
}

// NodeOutboundLBName returns the name of the node outbound LB.
func (s *ClusterScope) NodeOutboundLBName() string {
	return s.ClusterName()
}

// OutboundLBName returns the name of the outbound LB.
func (s *ClusterScope) OutboundLBName(role string) string {
	if role == infrav1.Node {
		return s.ClusterName()
	}
	if s.IsAPIServerPrivate() {
		return azure.GenerateControlPlaneOutboundLBName(s.ClusterName())
	}
	return s.APIServerLBName()
}

// OutboundPoolName returns the outbound LB backend pool name.
func (s *ClusterScope) OutboundPoolName(loadBalancerName string) string {
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

// GenerateFQDN generates a fully qualified domain name, based on a hash, cluster name and cluster location.
func (s *ClusterScope) GenerateFQDN(ipName string) string {
	h := fnv.New32a()
	if _, err := h.Write([]byte(fmt.Sprintf("%s/%s/%s", s.SubscriptionID(), s.ResourceGroup(), ipName))); err != nil {
		return ""
	}
	hash := fmt.Sprintf("%x", h.Sum32())
	return strings.ToLower(fmt.Sprintf("%s-%s.%s.%s", s.ClusterName(), hash, s.Location(), s.AzureClients.ResourceManagerVMDNSSuffix))
}

// GenerateLegacyFQDN generates an IP name and a fully qualified domain name, based on a hash, cluster name and cluster location.
// DEPRECATED: use GenerateFQDN instead.
func (s *ClusterScope) GenerateLegacyFQDN() (string, string) {
	h := fnv.New32a()
	if _, err := h.Write([]byte(fmt.Sprintf("%s/%s/%s", s.SubscriptionID(), s.ResourceGroup(), s.ClusterName()))); err != nil {
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
	conditions.SetSummary(s.AzureCluster,
		conditions.WithConditions(
			infrav1.NetworkInfrastructureReadyCondition,
		),
		conditions.WithStepCounterIfOnly(
			infrav1.NetworkInfrastructureReadyCondition,
		),
	)

	return s.patchHelper.Patch(
		ctx,
		s.AzureCluster,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			infrav1.NetworkInfrastructureReadyCondition,
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
		return azure.GeneratePrivateFQDN(s.ClusterName())
	}
	return s.APIServerPublicIP().DNSName
}

// SetFailureDomain will set the spec for a for a given key
func (s *ClusterScope) SetFailureDomain(id string, spec clusterv1.FailureDomainSpec) {
	if s.AzureCluster.Status.FailureDomains == nil {
		s.AzureCluster.Status.FailureDomains = make(clusterv1.FailureDomains)
	}
	s.AzureCluster.Status.FailureDomains[id] = spec
}

// SetControlPlaneIngressRules will set the ingress rules or the control plane subnet
func (s *ClusterScope) SetControlPlaneIngressRules() {
	if s.ControlPlaneSubnet().SecurityGroup.IngressRules == nil {
		s.ControlPlaneSubnet().SecurityGroup.IngressRules = infrav1.IngressRules{
			&infrav1.IngressRule{
				Name:             "allow_ssh",
				Description:      "Allow SSH",
				Priority:         2200,
				Protocol:         infrav1.SecurityGroupProtocolTCP,
				Source:           to.StringPtr("*"),
				SourcePorts:      to.StringPtr("*"),
				Destination:      to.StringPtr("*"),
				DestinationPorts: to.StringPtr("22"),
			},
			&infrav1.IngressRule{
				Name:             "allow_apiserver",
				Description:      "Allow K8s API Server",
				Priority:         2201,
				Protocol:         infrav1.SecurityGroupProtocolTCP,
				Source:           to.StringPtr("*"),
				SourcePorts:      to.StringPtr("*"),
				Destination:      to.StringPtr("*"),
				DestinationPorts: to.StringPtr(strconv.Itoa(int(s.APIServerPort()))),
			},
		}
	}
}

// SetDNSName sets the API Server public IP DNS name.
func (s *ClusterScope) SetDNSName() {
	// for back compat, set the old API Server defaults if no API Server Spec has been set by new webhooks.
	lb := s.APIServerLB()
	if lb == nil || lb.Name == "" {
		lbName := fmt.Sprintf("%s-%s", s.ClusterName(), "public-lb")
		ip, dns := s.GenerateLegacyFQDN()
		lb = &infrav1.LoadBalancerSpec{
			Name: lbName,
			SKU:  infrav1.SKUStandard,
			FrontendIPs: []infrav1.FrontendIP{
				{
					Name: azure.GenerateFrontendIPConfigName(lbName),
					PublicIP: &infrav1.PublicIPSpec{
						Name:    ip,
						DNSName: dns,
					},
				},
			},
			Type: infrav1.Public,
		}
		lb.DeepCopyInto(s.APIServerLB())
	}
	// Generate valid FQDN if not set.
	if !s.IsAPIServerPrivate() && s.APIServerPublicIP().DNSName == "" {
		s.APIServerPublicIP().DNSName = s.GenerateFQDN(s.APIServerPublicIP().Name)
	}
}

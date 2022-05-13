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

package scope

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"strings"

	"github.com/Azure/go-autorest/autorest/to"

	"github.com/Azure/go-autorest/autorest"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"golang.org/x/mod/semver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1alpha4"
	capiexputil "sigs.k8s.io/cluster-api/exp/util"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
)

// ManagedControlPlaneScopeParams defines the input parameters used to create a new managed
// control plane.
type ManagedControlPlaneScopeParams struct {
	AzureClients
	Client           client.Client
	Logger           logr.Logger
	Cluster          *clusterv1.Cluster
	ControlPlane     *infrav1exp.AzureManagedControlPlane
	InfraMachinePool *infrav1exp.AzureManagedMachinePool
	MachinePool      *expv1.MachinePool
	PatchTarget      client.Object
}

// NewManagedControlPlaneScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewManagedControlPlaneScope(ctx context.Context, params ManagedControlPlaneScopeParams) (*ManagedControlPlaneScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}

	if params.ControlPlane == nil {
		return nil, errors.New("failed to generate new scope from nil ControlPlane")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	if params.ControlPlane.Spec.IdentityRef == nil {
		if err := params.AzureClients.setCredentials(params.ControlPlane.Spec.SubscriptionID, ""); err != nil {
			return nil, errors.Wrap(err, "failed to create Azure session")
		}
	} else {
		credentialsProvider, err := NewManagedControlPlaneCredentialsProvider(ctx, params.Client, params.ControlPlane)
		if err != nil {
			return nil, errors.Wrap(err, "failed to init credentials provider")
		}

		if err := params.AzureClients.setCredentialsWithProvider(ctx, params.ControlPlane.Spec.SubscriptionID, "", credentialsProvider); err != nil {
			return nil, errors.Wrap(err, "failed to configure azure settings and credentials for Identity")
		}
	}

	helper, err := patch.NewHelper(params.PatchTarget, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &ManagedControlPlaneScope{
		Logger:           params.Logger,
		Client:           params.Client,
		AzureClients:     params.AzureClients,
		Cluster:          params.Cluster,
		ControlPlane:     params.ControlPlane,
		MachinePool:      params.MachinePool,
		InfraMachinePool: params.InfraMachinePool,
		PatchTarget:      params.PatchTarget,
		patchHelper:      helper,
	}, nil
}

// ManagedControlPlaneScope defines the basic context for an actuator to operate upon.
type ManagedControlPlaneScope struct {
	logr.Logger
	Client         client.Client
	patchHelper    *patch.Helper
	kubeConfigData []byte

	AzureClients
	Cluster          *clusterv1.Cluster
	MachinePool      *expv1.MachinePool
	ControlPlane     *infrav1exp.AzureManagedControlPlane
	InfraMachinePool *infrav1exp.AzureManagedMachinePool
	PatchTarget      client.Object

	AllNodePools []infrav1exp.AzureManagedMachinePool
}

// ResourceGroup returns the managed control plane's resource group.
func (s *ManagedControlPlaneScope) ResourceGroup() string {
	if s.ControlPlane == nil {
		return ""
	}
	return s.ControlPlane.Spec.ResourceGroupName
}

// NodeResourceGroup returns the managed control plane's node resource group.
func (s *ManagedControlPlaneScope) NodeResourceGroup() string {
	if s.ControlPlane == nil {
		return ""
	}
	return s.ControlPlane.Spec.NodeResourceGroupName
}

// ClusterName returns the managed control plane's name.
func (s *ManagedControlPlaneScope) ClusterName() string {
	return s.Cluster.Name
}

// Location returns the managed control plane's Azure location, or an empty string.
func (s *ManagedControlPlaneScope) Location() string {
	if s.ControlPlane == nil {
		return ""
	}
	return s.ControlPlane.Spec.Location
}

// AvailabilitySetEnabled is always false for a managed control plane.
func (s *ManagedControlPlaneScope) AvailabilitySetEnabled() bool {
	return false // not applicable for a managed control plane
}

// AdditionalTags returns AdditionalTags from the ControlPlane spec.
func (s *ManagedControlPlaneScope) AdditionalTags() infrav1.Tags {
	tags := make(infrav1.Tags)
	if s.ControlPlane.Spec.AdditionalTags != nil {
		tags = s.ControlPlane.Spec.AdditionalTags.DeepCopy()
	}
	return tags
}

// SubscriptionID returns the Azure client Subscription ID.
func (s *ManagedControlPlaneScope) SubscriptionID() string {
	return s.AzureClients.SubscriptionID()
}

// BaseURI returns the Azure ResourceManagerEndpoint.
func (s *ManagedControlPlaneScope) BaseURI() string {
	return s.AzureClients.ResourceManagerEndpoint
}

// Authorizer returns the Azure client Authorizer.
func (s *ManagedControlPlaneScope) Authorizer() autorest.Authorizer {
	return s.AzureClients.Authorizer
}

// PatchObject persists the cluster configuration and status.
func (s *ManagedControlPlaneScope) PatchObject(ctx context.Context) error {
	return s.patchHelper.Patch(ctx, s.PatchTarget)
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ManagedControlPlaneScope) Close(ctx context.Context) error {
	return s.PatchObject(ctx)
}

// GetVNetResourceGroup returns the vNet resource group name.
func (s *ManagedControlPlaneScope) GetVNetResourceGroup() string {
	if s.ControlPlane.Spec.VirtualNetwork.ResourceGroupName != nil {
		s.Logger.Info("User specified vNet ResourceGroupName %s", *s.ControlPlane.Spec.VirtualNetwork.ResourceGroupName)
		return *s.ControlPlane.Spec.VirtualNetwork.ResourceGroupName
	}
	return s.ControlPlane.Spec.ResourceGroupName
}

// Vnet returns the cluster Vnet.
func (s *ManagedControlPlaneScope) Vnet() *infrav1.VnetSpec {
	return &infrav1.VnetSpec{
		ResourceGroup: s.GetVNetResourceGroup(),
		Name:          s.ControlPlane.Spec.VirtualNetwork.Name,
		CIDRBlocks:    s.ControlPlane.Spec.VirtualNetwork.CIDRBlocks,
	}
}

// VNetSpec returns the virtual network spec.
func (s *ManagedControlPlaneScope) VNetSpec() azure.VNetSpec {
	return azure.VNetSpec{
		ResourceGroup: s.GetVNetResourceGroup(),
		Name:          s.Vnet().Name,
		CIDRs:         s.Vnet().CIDRBlocks,
	}
}

// ControlPlaneRouteTable returns the cluster controlplane routetable.
func (s *ManagedControlPlaneScope) ControlPlaneRouteTable() infrav1.RouteTable {
	return infrav1.RouteTable{}
}

// NodeRouteTable returns the cluster node routetable.
func (s *ManagedControlPlaneScope) NodeRouteTable() infrav1.RouteTable {
	return infrav1.RouteTable{}
}

// NodeNatGateway returns the cluster node nat gateway.
func (s *ManagedControlPlaneScope) NodeNatGateway() infrav1.NatGateway {
	return infrav1.NatGateway{}
}

// SubnetSpecs returns the subnets specs.
func (s *ManagedControlPlaneScope) SubnetSpecs() []azure.SubnetSpec {
	nodeSubnets := s.NodeSubnets()
	subnetSpecs := make([]azure.SubnetSpec, len(nodeSubnets))
	for i := range nodeSubnets {
		subnetSpecs[i] = azure.SubnetSpec{
			Name:     nodeSubnets[i].Name,
			CIDRs:    nodeSubnets[i].CIDRBlocks,
			VNetName: s.Vnet().Name,
		}
	}
	return subnetSpecs
}

// Subnets returns the subnets specs.
func (s *ManagedControlPlaneScope) Subnets() infrav1.Subnets {
	return infrav1.Subnets{}
}

// NodeSubnets returns the cluster node subnets.
func (s *ManagedControlPlaneScope) NodeSubnets() []infrav1.SubnetSpec {
	subnetSpecs := make([]infrav1.SubnetSpec, len(s.ControlPlane.Spec.VirtualNetwork.Subnets))
	for i := range s.ControlPlane.Spec.VirtualNetwork.Subnets {
		subnetSpecs[i] = infrav1.SubnetSpec{
			Name:       s.ControlPlane.Spec.VirtualNetwork.Subnets[i].Name,
			CIDRBlocks: s.ControlPlane.Spec.VirtualNetwork.Subnets[i].CIDRBlocks,
		}
	}
	return subnetSpecs
}

// SetSubnet sets the passed subnet spec into the scope.
// This is not used when using a managed control plane.
func (s *ManagedControlPlaneScope) SetSubnet(subnetSpec infrav1.SubnetSpec) {
	// no-op
}

// ControlPlaneSubnet returns the cluster control plane subnet.
func (s *ManagedControlPlaneScope) ControlPlaneSubnet() infrav1.SubnetSpec {
	return infrav1.SubnetSpec{}
}

// Subnet returns the subnet with the provided name.
func (s *ManagedControlPlaneScope) Subnet(name string) infrav1.SubnetSpec {
	subnet := infrav1.SubnetSpec{}
	for i := range s.ControlPlane.Spec.VirtualNetwork.Subnets {
		if name == s.ControlPlane.Spec.VirtualNetwork.Subnets[i].Name {
			subnet.Name = s.ControlPlane.Spec.VirtualNetwork.Subnets[i].Name
			subnet.CIDRBlocks = s.ControlPlane.Spec.VirtualNetwork.Subnets[i].CIDRBlocks
		}
	}

	return subnet
}

// IsIPv6Enabled returns true if a cluster is ipv6 enabled.
// Currently always false as managed control planes do not currently implement ipv6.
func (s *ManagedControlPlaneScope) IsIPv6Enabled() bool {
	return false
}

// IsVnetManaged returns true if the vnet is managed.
func (s *ManagedControlPlaneScope) IsVnetManaged() bool {
	return true
}

// APIServerLBName returns the API Server LB name.
func (s *ManagedControlPlaneScope) APIServerLBName() string {
	return "" // does not apply for AKS
}

// APIServerLBPoolName returns the API Server LB backend pool name.
func (s *ManagedControlPlaneScope) APIServerLBPoolName(loadBalancerName string) string {
	return "" // does not apply for AKS
}

// IsAPIServerPrivate returns true if the API Server LB is of type Internal.
// Currently always false as managed control planes do not currently implement private clusters.
func (s *ManagedControlPlaneScope) IsAPIServerPrivate() bool {
	return false
}

// OutboundLBName returns the name of the outbound LB.
// Note: for managed clusters, the outbound LB lifecycle is not managed.
func (s *ManagedControlPlaneScope) OutboundLBName(_ string) string {
	return "kubernetes"
}

// OutboundPoolName returns the outbound LB backend pool name.
func (s *ManagedControlPlaneScope) OutboundPoolName(_ string) string {
	return "aksOutboundBackendPool" // hard-coded in aks
}

// GetPrivateDNSZoneName returns the Private DNS Zone from the spec or generate it from cluster name.
// Currently always empty as managed control planes do not currently implement private clusters.
func (s *ManagedControlPlaneScope) GetPrivateDNSZoneName() string {
	return ""
}

// CloudProviderConfigOverrides returns the cloud provider config overrides for the cluster.
func (s *ManagedControlPlaneScope) CloudProviderConfigOverrides() *infrav1.CloudProviderConfigOverrides {
	return nil
}

// ManagedClusterSpec returns the managed cluster spec.
func (s *ManagedControlPlaneScope) ManagedClusterSpec() (azure.ManagedClusterSpec, error) {
	var sshPublicKey *string
	if s.ControlPlane.Spec.SSHPublicKey != nil {
		decodedSSHPublicKey, err := base64.StdEncoding.DecodeString(*s.ControlPlane.Spec.SSHPublicKey)
		if err != nil {
			return azure.ManagedClusterSpec{}, errors.Wrap(err, "failed to decode SSHPublicKey")
		}
		sshPublicKey = to.StringPtr(string(decodedSSHPublicKey))
	}

	managedClusterSpec := azure.ManagedClusterSpec{
		Name:                  s.ControlPlane.Name,
		ResourceGroupName:     s.ControlPlane.Spec.ResourceGroupName,
		NodeResourceGroupName: s.ControlPlane.Spec.NodeResourceGroupName,
		Location:              s.ControlPlane.Spec.Location,
		Tags:                  s.ControlPlane.Spec.AdditionalTags,
		Version:               strings.TrimPrefix(s.ControlPlane.Spec.Version, "v"),
		SSHPublicKey:          sshPublicKey,
		DNSServiceIP:          s.ControlPlane.Spec.DNSServiceIP,
		VnetSubnetID: azure.SubnetID(
			s.ControlPlane.Spec.SubscriptionID,
			s.GetVNetResourceGroup(),
			s.ControlPlane.Spec.VirtualNetwork.Name,
			s.ControlPlane.Spec.VirtualNetwork.Subnets[0].Name,
		),
		DisableLocalAccounts: s.ControlPlane.Spec.DisableLocalAccounts,
	}
	s.Logger.Info("managed cluster vnet subnet id %s", managedClusterSpec.VnetSubnetID)

	if s.ControlPlane.Spec.NetworkPlugin != nil {
		managedClusterSpec.NetworkPlugin = *s.ControlPlane.Spec.NetworkPlugin
	}
	if s.ControlPlane.Spec.NetworkPolicy != nil {
		managedClusterSpec.NetworkPolicy = *s.ControlPlane.Spec.NetworkPolicy
	}
	if s.ControlPlane.Spec.LoadBalancerSKU != nil {
		managedClusterSpec.LoadBalancerSKU = *s.ControlPlane.Spec.LoadBalancerSKU
	}

	if net := s.Cluster.Spec.ClusterNetwork; net != nil {
		if net.Services != nil {
			// A user may provide zero or one CIDR blocks. If they provide an empty array,
			// we ignore it and use the default. AKS doesn't support > 1 Service/Pod CIDR.
			if len(net.Services.CIDRBlocks) > 1 {
				return azure.ManagedClusterSpec{}, errors.New("managed control planes only allow one service cidr")
			}
			if len(net.Services.CIDRBlocks) == 1 {
				managedClusterSpec.ServiceCIDR = net.Services.CIDRBlocks[0]
			}
		}
		if net.Pods != nil {
			// A user may provide zero or one CIDR blocks. If they provide an empty array,
			// we ignore it and use the default. AKS doesn't support > 1 Service/Pod CIDR.
			if len(net.Pods.CIDRBlocks) > 1 {
				return azure.ManagedClusterSpec{}, errors.New("managed control planes only allow one service cidr")
			}
			if len(net.Pods.CIDRBlocks) == 1 {
				managedClusterSpec.PodCIDR = net.Pods.CIDRBlocks[0]
			}
		}
	}

	if s.ControlPlane.Spec.DNSServiceIP != nil {
		if managedClusterSpec.ServiceCIDR == "" {
			return azure.ManagedClusterSpec{}, fmt.Errorf(s.Cluster.Name + " cluster serviceCIDR must be specified if specifying DNSServiceIP")
		}
		_, cidr, err := net.ParseCIDR(managedClusterSpec.ServiceCIDR)
		if err != nil {
			return azure.ManagedClusterSpec{}, fmt.Errorf("failed to parse cluster service cidr: %w", err)
		}
		ip := net.ParseIP(*s.ControlPlane.Spec.DNSServiceIP)
		if !cidr.Contains(ip) {
			return azure.ManagedClusterSpec{}, fmt.Errorf(s.ControlPlane.Name + " DNSServiceIP must reside within the associated cluster serviceCIDR")
		}
	}

	if s.ControlPlane.Spec.AADProfile != nil {
		managedClusterSpec.AADProfile = &azure.AADProfile{
			Managed:             s.ControlPlane.Spec.AADProfile.Managed,
			EnableAzureRBAC:     s.ControlPlane.Spec.AADProfile.Managed,
			AdminGroupObjectIDs: s.ControlPlane.Spec.AADProfile.AdminGroupObjectIDs,
		}
	}

	if s.ControlPlane.Spec.Sku != nil {
		managedClusterSpec.Sku = &azure.SKU{
			Tier: s.ControlPlane.Spec.Sku.Tier,
		}
	}

	if s.ControlPlane.Spec.LoadBalancerProfile != nil {
		managedClusterSpec.LoadBalancerProfile = &azure.LoadBalancerProfile{
			ManagedOutboundIPs:     s.ControlPlane.Spec.LoadBalancerProfile.ManagedOutboundIPs,
			OutboundIPPrefixes:     s.ControlPlane.Spec.LoadBalancerProfile.OutboundIPPrefixes,
			OutboundIPs:            s.ControlPlane.Spec.LoadBalancerProfile.OutboundIPs,
			EffectiveOutboundIPs:   s.ControlPlane.Spec.LoadBalancerProfile.EffectiveOutboundIPs,
			AllocatedOutboundPorts: s.ControlPlane.Spec.LoadBalancerProfile.AllocatedOutboundPorts,
			IdleTimeoutInMinutes:   s.ControlPlane.Spec.LoadBalancerProfile.IdleTimeoutInMinutes,
		}
	}

	if s.ControlPlane.Spec.APIServerAccessProfile != nil {
		managedClusterSpec.APIServerAccessProfile = &azure.APIServerAccessProfile{
			AuthorizedIPRanges:             s.ControlPlane.Spec.APIServerAccessProfile.AuthorizedIPRanges,
			EnablePrivateCluster:           s.ControlPlane.Spec.APIServerAccessProfile.EnablePrivateCluster,
			PrivateDNSZone:                 s.ControlPlane.Spec.APIServerAccessProfile.PrivateDNSZone,
			EnablePrivateClusterPublicFQDN: s.ControlPlane.Spec.APIServerAccessProfile.EnablePrivateClusterPublicFQDN,
		}
	}

	return managedClusterSpec, nil
}

// GetAgentPoolSpecs gets azure.AgentPoolSpec for the list of agent pools.
func (s *ManagedControlPlaneScope) GetAgentPoolSpecs(ctx context.Context) ([]azure.AgentPoolSpec, error) {
	if len(s.AllNodePools) == 0 {
		opt1 := client.InNamespace(s.ControlPlane.Namespace)
		opt2 := client.MatchingLabels(map[string]string{
			clusterv1.ClusterLabelName: s.Cluster.Name,
		})

		ammpList := &infrav1exp.AzureManagedMachinePoolList{}

		if err := s.Client.List(ctx, ammpList, opt1, opt2); err != nil {
			return nil, err
		}

		s.AllNodePools = ammpList.Items
	}

	ammps := []azure.AgentPoolSpec{}

	foundSystemPool := false
	for _, pool := range s.AllNodePools {
		// Fetch the owning MachinePool.

		ownerPool, err := capiexputil.GetOwnerMachinePool(ctx, s.Client, pool.ObjectMeta)
		if err != nil {
			s.Logger.Error(err, "failed to fetch owner ref for system pool: %s", pool.Name)
			continue
		}
		if ownerPool == nil {
			s.Logger.Info("failed to fetch owner ref for system pool")
			continue
		}

		if pool.Spec.Mode == string(infrav1exp.NodePoolModeSystem) {
			foundSystemPool = true
		}

		var name string
		if pool.Spec.Name != nil {
			name = *pool.Spec.Name
		} else {
			name = pool.Name
		}

		ammp := azure.AgentPoolSpec{
			Name:         name,
			SKU:          pool.Spec.SKU,
			Replicas:     1,
			OSDiskSizeGB: 0,
			Mode:         pool.Spec.Mode,
		}

		// Set optional values
		if pool.Spec.OSDiskSizeGB != nil {
			ammp.OSDiskSizeGB = *pool.Spec.OSDiskSizeGB
		}

		if ownerPool.Spec.Replicas != nil {
			ammp.Replicas = *ownerPool.Spec.Replicas
		}

		if ownerPool.Spec.Template.Spec.Version != nil {
			version := *ownerPool.Spec.Template.Spec.Version
			if semver.Compare(version, s.ControlPlane.Spec.Version) > 0 {
				return nil, errors.New("MachinePool version cannot be greater than the AzureManagedControlPlane version")
			}
			ammp.Version = to.StringPtr(strings.TrimPrefix(version, "v"))
		}

		if pool.Spec.MaxCount != nil {
			ammp.MaxCount = pool.Spec.MaxCount
		}

		if pool.Spec.MinCount != nil {
			ammp.MinCount = pool.Spec.MinCount
		}

		if pool.Spec.EnableAutoScaling != nil {
			ammp.EnableAutoScaling = pool.Spec.EnableAutoScaling
		}

		if pool.Spec.EnableFIPS != nil {
			ammp.EnableFIPS = pool.Spec.EnableFIPS
		}

		if pool.Spec.EnableNodePublicIP != nil {
			ammp.EnableNodePublicIP = pool.Spec.EnableNodePublicIP
		}

		if pool.Spec.NodeLabels != nil {
			ammp.NodeLabels = pool.Spec.NodeLabels
		}

		if pool.Spec.NodeTaints != nil {
			ammp.NodeTaints = pool.Spec.NodeTaints
		}

		if pool.Spec.OsDiskType != nil {
			ammp.OsDiskType = pool.Spec.OsDiskType
		}

		if pool.Spec.AvailabilityZones != nil {
			ammp.AvailabilityZones = pool.Spec.AvailabilityZones
		}

		if pool.Spec.VnetSubnetID != nil {
			ammp.VnetSubnetID = *pool.Spec.VnetSubnetID
		}

		if pool.Spec.MaxPods != nil {
			ammp.MaxPods = pool.Spec.MaxPods
		}

		if pool.Spec.Mode == string(infrav1exp.NodePoolModeUser) && pool.Spec.ScaleSetPriority != nil {
			ammp.ScaleSetPriority = pool.Spec.ScaleSetPriority
		}

		if pool.Spec.KubeletConfig != nil {
			ammp.KubeletConfig = (*infrav1.KubeletConfig)(pool.Spec.KubeletConfig)
		}

		ammps = append(ammps, ammp)
	}

	if !foundSystemPool {
		return nil, errors.New("failed to fetch azuremanagedMachine pool with mode:System, require at least 1 system node pool")
	}

	return ammps, nil
}

// AgentPoolSpec returns an azure.AgentPoolSpec for currently reconciled AzureManagedMachinePool.
func (s *ManagedControlPlaneScope) AgentPoolSpec() azure.AgentPoolSpec {
	var normalizedVersion *string
	if s.MachinePool.Spec.Template.Spec.Version != nil {
		v := strings.TrimPrefix(*s.MachinePool.Spec.Template.Spec.Version, "v")
		normalizedVersion = &v
	}

	replicas := int32(1)
	if s.MachinePool.Spec.Replicas != nil {
		replicas = *s.MachinePool.Spec.Replicas
	}

	var name string
	if s.InfraMachinePool.Spec.Name != nil {
		name = *s.InfraMachinePool.Spec.Name
	} else {
		name = s.InfraMachinePool.Name
	}

	agentPoolSpec := azure.AgentPoolSpec{
		Name:          name,
		ResourceGroup: s.ControlPlane.Spec.ResourceGroupName,
		Cluster:       s.ControlPlane.Name,
		SKU:           s.InfraMachinePool.Spec.SKU,
		Replicas:      replicas,
		Version:       normalizedVersion,
		Mode:          s.InfraMachinePool.Spec.Mode,
	}

	if s.InfraMachinePool.Spec.OSDiskSizeGB != nil {
		agentPoolSpec.OSDiskSizeGB = *s.InfraMachinePool.Spec.OSDiskSizeGB
	}

	if s.InfraMachinePool.Spec.MaxCount != nil {
		agentPoolSpec.MaxCount = s.InfraMachinePool.Spec.MaxCount
	}

	if s.InfraMachinePool.Spec.MinCount != nil {
		agentPoolSpec.MinCount = s.InfraMachinePool.Spec.MinCount
	}

	if s.InfraMachinePool.Spec.EnableAutoScaling != nil {
		agentPoolSpec.EnableAutoScaling = s.InfraMachinePool.Spec.EnableAutoScaling
	}

	if s.InfraMachinePool.Spec.EnableFIPS != nil {
		agentPoolSpec.EnableFIPS = s.InfraMachinePool.Spec.EnableFIPS
	}

	if s.InfraMachinePool.Spec.EnableNodePublicIP != nil {
		agentPoolSpec.EnableNodePublicIP = s.InfraMachinePool.Spec.EnableNodePublicIP
	}

	if s.InfraMachinePool.Spec.NodeLabels != nil {
		agentPoolSpec.NodeLabels = s.InfraMachinePool.Spec.NodeLabels
	}

	if s.InfraMachinePool.Spec.NodeTaints != nil {
		agentPoolSpec.NodeTaints = s.InfraMachinePool.Spec.NodeTaints
	}

	if s.InfraMachinePool.Spec.OsDiskType != nil {
		agentPoolSpec.OsDiskType = s.InfraMachinePool.Spec.OsDiskType
	}

	if s.InfraMachinePool.Spec.VnetSubnetID != nil {
		agentPoolSpec.VnetSubnetID = *s.InfraMachinePool.Spec.VnetSubnetID
	} else {
		agentPoolSpec.VnetSubnetID = azure.SubnetID(
			s.ControlPlane.Spec.SubscriptionID,
			s.GetVNetResourceGroup(),
			s.ControlPlane.Spec.VirtualNetwork.Name,
			s.ControlPlane.Spec.VirtualNetwork.Subnets[0].Name,
		)
	}

	if s.InfraMachinePool.Spec.AvailabilityZones != nil {
		agentPoolSpec.AvailabilityZones = s.InfraMachinePool.Spec.AvailabilityZones
	}

	if s.InfraMachinePool.Spec.ScaleSetPriority != nil {
		agentPoolSpec.ScaleSetPriority = s.InfraMachinePool.Spec.ScaleSetPriority
	}

	if s.InfraMachinePool.Spec.MaxPods != nil {
		agentPoolSpec.MaxPods = s.InfraMachinePool.Spec.MaxPods
	}

	if s.InfraMachinePool.Spec.KubeletConfig != nil {
		agentPoolSpec.KubeletConfig = (*infrav1.KubeletConfig)(s.InfraMachinePool.Spec.KubeletConfig)
	}

	return agentPoolSpec
}

// SetAgentPoolProviderIDList sets a list of agent pool's Azure VM IDs.
func (s *ManagedControlPlaneScope) SetAgentPoolProviderIDList(providerIDs []string) {
	s.InfraMachinePool.Spec.ProviderIDList = providerIDs
}

// SetAgentPoolReplicas sets the number of agent pool replicas.
func (s *ManagedControlPlaneScope) SetAgentPoolReplicas(replicas int32) {
	s.InfraMachinePool.Status.Replicas = replicas
}

// SetAgentPoolReady sets the flag that indicates if the agent pool is ready or not.
func (s *ManagedControlPlaneScope) SetAgentPoolReady(ready bool) {
	s.InfraMachinePool.Status.Ready = ready
}

// SetControlPlaneEndpoint sets a control plane endpoint.
func (s *ManagedControlPlaneScope) SetControlPlaneEndpoint(endpoint clusterv1.APIEndpoint) {
	s.ControlPlane.Spec.ControlPlaneEndpoint = endpoint
}

// MakeEmptyKubeConfigSecret creates an empty secret object that is used for storing kubeconfig secret data.
func (s *ManagedControlPlaneScope) MakeEmptyKubeConfigSecret() corev1.Secret {
	return corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secret.Name(s.Cluster.Name, secret.Kubeconfig),
			Namespace: s.Cluster.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(s.ControlPlane, infrav1exp.GroupVersion.WithKind("AzureManagedControlPlane")),
			},
		},
	}
}

// GetKubeConfigData returns a []byte that contains kubeconfig.
func (s *ManagedControlPlaneScope) GetKubeConfigData() []byte {
	return s.kubeConfigData
}

// SetKubeConfigData sets kubeconfig data.
func (s *ManagedControlPlaneScope) SetKubeConfigData(kubeConfigData []byte) {
	s.kubeConfigData = kubeConfigData
}

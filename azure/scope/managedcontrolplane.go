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
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	capiexputil "sigs.k8s.io/cluster-api/exp/util"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/groups"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/util/futures"
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

// Vnet returns the cluster Vnet.
func (s *ManagedControlPlaneScope) Vnet() *infrav1.VnetSpec {
	return &infrav1.VnetSpec{
		ResourceGroup: s.ControlPlane.Spec.ResourceGroupName,
		Name:          s.ControlPlane.Spec.VirtualNetwork.Name,
		CIDRBlocks:    []string{s.ControlPlane.Spec.VirtualNetwork.CIDRBlock},
	}
}

// GroupSpec returns the resource group spec.
func (s *ManagedControlPlaneScope) GroupSpec() azure.ResourceSpecGetter {
	return &groups.GroupSpec{
		Name:           s.ResourceGroup(),
		Location:       s.Location(),
		ClusterName:    s.ClusterName(),
		AdditionalTags: s.AdditionalTags(),
	}
}

// VNetSpec returns the virtual network spec.
func (s *ManagedControlPlaneScope) VNetSpec() azure.VNetSpec {
	return azure.VNetSpec{
		ResourceGroup: s.Vnet().ResourceGroup,
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
	return []azure.SubnetSpec{
		{
			Name:     s.NodeSubnet().Name,
			CIDRs:    s.NodeSubnet().CIDRBlocks,
			VNetName: s.Vnet().Name,
		},
	}
}

// Subnets returns the subnets specs.
func (s *ManagedControlPlaneScope) Subnets() infrav1.Subnets {
	return infrav1.Subnets{}
}

// NodeSubnet returns the cluster node subnet.
func (s *ManagedControlPlaneScope) NodeSubnet() infrav1.SubnetSpec {
	return infrav1.SubnetSpec{
		Name:       s.ControlPlane.Spec.VirtualNetwork.Subnet.Name,
		CIDRBlocks: []string{s.ControlPlane.Spec.VirtualNetwork.Subnet.CIDRBlock},
	}
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

// NodeSubnets returns the subnets with the node role.
func (s *ManagedControlPlaneScope) NodeSubnets() []infrav1.SubnetSpec {
	return []infrav1.SubnetSpec{
		{
			Name:       s.ControlPlane.Spec.VirtualNetwork.Subnet.Name,
			CIDRBlocks: []string{s.ControlPlane.Spec.VirtualNetwork.Subnet.CIDRBlock},
		},
	}
}

// Subnet returns the subnet with the provided name.
func (s *ManagedControlPlaneScope) Subnet(name string) infrav1.SubnetSpec {
	subnet := infrav1.SubnetSpec{}
	if name == s.ControlPlane.Spec.VirtualNetwork.Subnet.Name {
		subnet.Name = s.ControlPlane.Spec.VirtualNetwork.Subnet.Name
		subnet.CIDRBlocks = []string{s.ControlPlane.Spec.VirtualNetwork.Subnet.CIDRBlock}
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

// FailureDomains returns the failure domains for the cluster.
func (s *ManagedControlPlaneScope) FailureDomains() []string {
	return []string{}
}

// ManagedClusterSpec returns the managed cluster spec.
func (s *ManagedControlPlaneScope) ManagedClusterSpec() (azure.ManagedClusterSpec, error) {
	decodedSSHPublicKey, err := base64.StdEncoding.DecodeString(s.ControlPlane.Spec.SSHPublicKey)
	if err != nil {
		return azure.ManagedClusterSpec{}, errors.Wrap(err, "failed to decode SSHPublicKey")
	}

	managedClusterSpec := azure.ManagedClusterSpec{
		Name:                  s.ControlPlane.Name,
		ResourceGroupName:     s.ControlPlane.Spec.ResourceGroupName,
		NodeResourceGroupName: s.ControlPlane.Spec.NodeResourceGroupName,
		Location:              s.ControlPlane.Spec.Location,
		Tags:                  s.ControlPlane.Spec.AdditionalTags,
		Version:               strings.TrimPrefix(s.ControlPlane.Spec.Version, "v"),
		SSHPublicKey:          string(decodedSSHPublicKey),
		DNSServiceIP:          s.ControlPlane.Spec.DNSServiceIP,
		VnetSubnetID: azure.SubnetID(
			s.ControlPlane.Spec.SubscriptionID,
			s.ControlPlane.Spec.ResourceGroupName,
			s.ControlPlane.Spec.VirtualNetwork.Name,
			s.ControlPlane.Spec.VirtualNetwork.Subnet.Name,
		),
	}

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

	if s.ControlPlane.Spec.SKU != nil {
		managedClusterSpec.SKU = &azure.SKU{
			Tier: s.ControlPlane.Spec.SKU.Tier,
		}
	}

	if s.ControlPlane.Spec.LoadBalancerProfile != nil {
		managedClusterSpec.LoadBalancerProfile = &azure.LoadBalancerProfile{
			ManagedOutboundIPs:     s.ControlPlane.Spec.LoadBalancerProfile.ManagedOutboundIPs,
			OutboundIPPrefixes:     s.ControlPlane.Spec.LoadBalancerProfile.OutboundIPPrefixes,
			OutboundIPs:            s.ControlPlane.Spec.LoadBalancerProfile.OutboundIPs,
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

// GetAgentPoolSpecs gets a slice of azure.AgentPoolSpec for the list of agent pools.
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

		ammp := azure.AgentPoolSpec{
			Name:         *pool.Spec.Name,
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

		if pool.Spec.VnetSubnetID != nil {
			ammp.VnetSubnetID = *pool.Spec.VnetSubnetID
		}

		if pool.Spec.KubeletConfig != nil {
			ammp.KubeletConfig = (*infrav1.KubeletConfig)(pool.Spec.KubeletConfig)
		}

		if pool.Spec.AutoScaling != nil {
			ammp.EnableAutoScaling = to.BoolPtr(true)
			ammp.MaxCount = pool.Spec.AutoScaling.MaxCount
			ammp.MinCount = pool.Spec.AutoScaling.MinCount
		}
		ammp.EnableFIPS = pool.Spec.EnableFIPS
		ammp.EnableNodePublicIP = pool.Spec.EnableNodePublicIP
		ammp.AvailabilityZones = pool.Spec.AvailabilityZones
		ammp.NodeLabels = pool.Spec.NodeLabels
		ammp.NodeTaints = pool.Spec.NodeTaints
		ammp.OsDiskType = pool.Spec.OsDiskType
		ammp.ScaleSetPriority = pool.Spec.ScaleSetPriority
		ammp.MaxPods = pool.Spec.MaxPods

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

	agentPoolSpec := azure.AgentPoolSpec{
		Name:          *s.InfraMachinePool.Spec.Name,
		ResourceGroup: s.ControlPlane.Spec.ResourceGroupName,
		Cluster:       s.ControlPlane.Name,
		SKU:           s.InfraMachinePool.Spec.SKU,
		Replicas:      replicas,
		Version:       normalizedVersion,
		VnetSubnetID: azure.SubnetID(
			s.ControlPlane.Spec.SubscriptionID,
			s.ControlPlane.Spec.ResourceGroupName,
			s.ControlPlane.Spec.VirtualNetwork.Name,
			s.ControlPlane.Spec.VirtualNetwork.Subnet.Name,
		),
		Mode: s.InfraMachinePool.Spec.Mode,
	}

	if s.InfraMachinePool.Spec.OSDiskSizeGB != nil {
		agentPoolSpec.OSDiskSizeGB = *s.InfraMachinePool.Spec.OSDiskSizeGB
	}

	if s.InfraMachinePool.Spec.VnetSubnetID != nil {
		agentPoolSpec.VnetSubnetID = *s.InfraMachinePool.Spec.VnetSubnetID
	}

	if s.InfraMachinePool.Spec.KubeletConfig != nil {
		agentPoolSpec.KubeletConfig = (*infrav1.KubeletConfig)(s.InfraMachinePool.Spec.KubeletConfig)
	}

	if s.InfraMachinePool.Spec.AutoScaling != nil {
		agentPoolSpec.EnableAutoScaling = to.BoolPtr(true)
		agentPoolSpec.MaxCount = s.InfraMachinePool.Spec.AutoScaling.MaxCount
		agentPoolSpec.MinCount = s.InfraMachinePool.Spec.AutoScaling.MinCount
	}
	agentPoolSpec.EnableFIPS = s.InfraMachinePool.Spec.EnableFIPS
	agentPoolSpec.EnableNodePublicIP = s.InfraMachinePool.Spec.EnableNodePublicIP
	agentPoolSpec.NodeLabels = s.InfraMachinePool.Spec.NodeLabels
	agentPoolSpec.NodeTaints = s.InfraMachinePool.Spec.NodeTaints
	agentPoolSpec.OsDiskType = s.InfraMachinePool.Spec.OsDiskType
	agentPoolSpec.AvailabilityZones = s.InfraMachinePool.Spec.AvailabilityZones
	agentPoolSpec.ScaleSetPriority = s.InfraMachinePool.Spec.ScaleSetPriority
	agentPoolSpec.MaxPods = s.InfraMachinePool.Spec.MaxPods

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

// SetLongRunningOperationState will set the future on the AzureManagedControlPlane status to allow the resource to continue
// in the next reconciliation.
func (s *ManagedControlPlaneScope) SetLongRunningOperationState(future *infrav1.Future) {
	futures.Set(s.ControlPlane, future)
}

// GetLongRunningOperationState will get the future on the AzureManagedControlPlane status.
func (s *ManagedControlPlaneScope) GetLongRunningOperationState(name, service string) *infrav1.Future {
	return futures.Get(s.ControlPlane, name, service)
}

// DeleteLongRunningOperationState will delete the future from the AzureManagedControlPlane status.
func (s *ManagedControlPlaneScope) DeleteLongRunningOperationState(name, service string) {
	futures.Delete(s.ControlPlane, name, service)
}

// UpdateDeleteStatus updates a condition on the AzureManagedControlPlane status after a DELETE operation.
func (s *ManagedControlPlaneScope) UpdateDeleteStatus(condition clusterv1.ConditionType, service string, err error) {
	// TODO: add condition to AzureManagedControlPlane status
}

// UpdatePutStatus updates a condition on the AzureManagedControlPlane status after a PUT operation.
func (s *ManagedControlPlaneScope) UpdatePutStatus(condition clusterv1.ConditionType, service string, err error) {
	// TODO: add condition to AzureManagedControlPlane status
}

// UpdatePatchStatus updates a condition on the AzureManagedControlPlane status after a PATCH operation.
func (s *ManagedControlPlaneScope) UpdatePatchStatus(condition clusterv1.ConditionType, service string, err error) {
	// TODO: add condition to AzureManagedControlPlane status
}

// AnnotationJSON returns a map[string]interface from a JSON annotation.
func (s *ManagedControlPlaneScope) AnnotationJSON(annotation string) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	jsonAnnotation := s.ControlPlane.GetAnnotations()[annotation]
	if len(jsonAnnotation) == 0 {
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
func (s *ManagedControlPlaneScope) UpdateAnnotationJSON(annotation string, content map[string]interface{}) error {
	b, err := json.Marshal(content)
	if err != nil {
		return err
	}
	s.SetAnnotation(annotation, string(b))
	return nil
}

// SetAnnotation sets a key value annotation on the ControlPlane.
func (s *ManagedControlPlaneScope) SetAnnotation(key, value string) {
	if s.ControlPlane.Annotations == nil {
		s.ControlPlane.Annotations = map[string]string{}
	}
	s.ControlPlane.Annotations[key] = value
}

// TagsSpecs returns the tag specs for the ManagedControlPlane.
func (s *ManagedControlPlaneScope) TagsSpecs() []azure.TagsSpec {
	return []azure.TagsSpec{
		{
			Scope:      azure.ResourceGroupID(s.SubscriptionID(), s.ResourceGroup()),
			Tags:       s.AdditionalTags(),
			Annotation: infrav1.RGTagsLastAppliedAnnotation,
		},
	}
}

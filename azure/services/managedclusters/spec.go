/*
Copyright 2022 The Kubernetes Authors.

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

package managedclusters

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2022-03-01/containerservice"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// ManagedClusterSpec contains properties to create a managed cluster.
type ManagedClusterSpec struct {
	// Name is the name of this AKS Cluster.
	Name string

	// ResourceGroup is the name of the Azure resource group for this AKS Cluster.
	ResourceGroup string

	// NodeResourceGroup is the name of the Azure resource group containing IaaS VMs.
	NodeResourceGroup string

	// ClusterName is the name of the owning Cluster API Cluster resource.
	ClusterName string

	// VnetSubnetID is the Azure Resource ID for the subnet which should contain nodes.
	VnetSubnetID string

	// Location is a string matching one of the canonical Azure region names. Examples: "westus2", "eastus".
	Location string

	// Tags is a set of tags to add to this cluster.
	Tags map[string]string

	// Version defines the desired Kubernetes version.
	Version string

	// LoadBalancerSKU for the managed cluster. Possible values include: 'Standard', 'Basic'. Defaults to Standard.
	LoadBalancerSKU string

	// NetworkPlugin used for building Kubernetes network. Possible values include: 'azure', 'kubenet'. Defaults to azure.
	NetworkPlugin string

	// NetworkPolicy used for building Kubernetes network. Possible values include: 'calico', 'azure'.
	NetworkPolicy string

	// OutboundType used for building Kubernetes network. Possible values include: 'loadBalancer', 'managedNATGateway', 'userAssignedNATGateway', 'userDefinedRouting'.
	OutboundType *infrav1.ManagedControlPlaneOutboundType

	// SSHPublicKey is a string literal containing an ssh public key. Will autogenerate and discard if not provided.
	SSHPublicKey string

	// GetAllAgentPools is a function that returns the list of agent pool specifications in this cluster.
	GetAllAgentPools func() ([]azure.ResourceSpecGetter, error)

	// PodCIDR is the CIDR block for IP addresses distributed to pods
	PodCIDR string

	// ServiceCIDR is the CIDR block for IP addresses distributed to services
	ServiceCIDR string

	// DNSServiceIP is an IP address assigned to the Kubernetes DNS service
	DNSServiceIP *string

	// AddonProfiles are the profiles of managed cluster add-on.
	AddonProfiles []AddonProfile

	// AADProfile is Azure Active Directory configuration to integrate with AKS, for aad authentication.
	AADProfile *AADProfile

	// SKU is the SKU of the AKS to be provisioned.
	SKU *SKU

	// LoadBalancerProfile is the profile of the cluster load balancer.
	LoadBalancerProfile *LoadBalancerProfile

	// APIServerAccessProfile is the access profile for AKS API server.
	APIServerAccessProfile *APIServerAccessProfile

	// Headers is the list of headers to add to the HTTP requests to update this resource.
	Headers map[string]string

	// AutoScalerProfile is the parameters to be applied to the cluster-autoscaler when enabled.
	AutoScalerProfile *AutoScalerProfile
}

// AADProfile is Azure Active Directory configuration to integrate with AKS, for aad authentication.
type AADProfile struct {
	// Managed defines whether to enable managed AAD.
	Managed bool

	// EnableAzureRBAC defines whether to enable Azure RBAC for Kubernetes authorization.
	EnableAzureRBAC bool

	// AdminGroupObjectIDs are the AAD group object IDs that will have admin role of the cluster.
	AdminGroupObjectIDs []string
}

// AddonProfile is the profile of a managed cluster add-on.
type AddonProfile struct {
	Name    string
	Config  map[string]string
	Enabled bool
}

// SKU is an AKS SKU.
type SKU struct {
	// Tier is the tier of a managed cluster SKU.
	Tier string
}

// LoadBalancerProfile is the profile of the cluster load balancer.
type LoadBalancerProfile struct {
	// Load balancer profile must specify at most one of ManagedOutboundIPs, OutboundIPPrefixes and OutboundIPs.
	// By default the AKS cluster automatically creates a public IP in the AKS-managed infrastructure resource group and assigns it to the load balancer outbound pool.
	// Alternatively, you can assign your own custom public IP or public IP prefix at cluster creation time.
	// See https://learn.microsoft.com/azure/aks/load-balancer-standard#provide-your-own-outbound-public-ips-or-prefixes

	// ManagedOutboundIPs are the desired managed outbound IPs for the cluster load balancer.
	ManagedOutboundIPs *int32

	// OutboundIPPrefixes are the desired outbound IP Prefix resources for the cluster load balancer.
	OutboundIPPrefixes []string

	// OutboundIPs are the desired outbound IP resources for the cluster load balancer.
	OutboundIPs []string

	// AllocatedOutboundPorts are the desired number of allocated SNAT ports per VM. Allowed values must be in the range of 0 to 64000 (inclusive). The default value is 0 which results in Azure dynamically allocating ports.
	AllocatedOutboundPorts *int32

	// IdleTimeoutInMinutes  are the desired outbound flow idle timeout in minutes. Allowed values must be in the range of 4 to 120 (inclusive). The default value is 30 minutes.
	IdleTimeoutInMinutes *int32
}

// APIServerAccessProfile is the access profile for AKS API server.
type APIServerAccessProfile struct {
	// AuthorizedIPRanges are the authorized IP Ranges to kubernetes API server.
	AuthorizedIPRanges []string
	// EnablePrivateCluster defines hether to create the cluster as a private cluster or not.
	EnablePrivateCluster *bool
	// PrivateDNSZone is the private dns zone for private clusters.
	PrivateDNSZone *string
	// EnablePrivateClusterPublicFQDN defines whether to create additional public FQDN for private cluster or not.
	EnablePrivateClusterPublicFQDN *bool
}

// AutoScalerProfile parameters to be applied to the cluster-autoscaler when enabled.
type AutoScalerProfile struct {
	// BalanceSimilarNodeGroups - Valid values are 'true' and 'false'
	BalanceSimilarNodeGroups *string
	// Expander - If not specified, the default is 'random'. See [expanders](https://github.com/kubernetes/autoscaler/blob/master/cluster-autoscaler/FAQ.md#what-are-expanders) for more information.
	Expander *string
	// MaxEmptyBulkDelete - The default is 10.
	MaxEmptyBulkDelete *string
	// MaxGracefulTerminationSec - The default is 600.
	MaxGracefulTerminationSec *string
	// MaxNodeProvisionTime - The default is '15m'. Values must be an integer followed by an 'm'. No unit of time other than minutes (m) is supported.
	MaxNodeProvisionTime *string
	// MaxTotalUnreadyPercentage - The default is 45. The maximum is 100 and the minimum is 0.
	MaxTotalUnreadyPercentage *string
	// NewPodScaleUpDelay - For scenarios like burst/batch scale where you don't want CA to act before the kubernetes scheduler could schedule all the pods, you can tell CA to ignore unscheduled pods before they're a certain age. The default is '0s'. Values must be an integer followed by a unit ('s' for seconds, 'm' for minutes, 'h' for hours, etc).
	NewPodScaleUpDelay *string
	// OkTotalUnreadyCount - This must be an integer. The default is 3.
	OkTotalUnreadyCount *string
	// ScanInterval - The default is '10s'. Values must be an integer number of seconds.
	ScanInterval *string
	// ScaleDownDelayAfterAdd - The default is '10m'. Values must be an integer followed by an 'm'. No unit of time other than minutes (m) is supported.
	ScaleDownDelayAfterAdd *string
	// ScaleDownDelayAfterDelete - The default is the scan-interval. Values must be an integer followed by an 'm'. No unit of time other than minutes (m) is supported.
	ScaleDownDelayAfterDelete *string
	// ScaleDownDelayAfterFailure - The default is '3m'. Values must be an integer followed by an 'm'. No unit of time other than minutes (m) is supported.
	ScaleDownDelayAfterFailure *string
	// ScaleDownUnneededTime - The default is '10m'. Values must be an integer followed by an 'm'. No unit of time other than minutes (m) is supported.
	ScaleDownUnneededTime *string
	// ScaleDownUnreadyTime - The default is '20m'. Values must be an integer followed by an 'm'. No unit of time other than minutes (m) is supported.
	ScaleDownUnreadyTime *string
	// ScaleDownUtilizationThreshold - The default is '0.5'.
	ScaleDownUtilizationThreshold *string
	// SkipNodesWithLocalStorage - The default is true.
	SkipNodesWithLocalStorage *string
	// SkipNodesWithSystemPods - The default is true.
	SkipNodesWithSystemPods *string
}

var _ azure.ResourceSpecGetterWithHeaders = (*ManagedClusterSpec)(nil)

// ResourceName returns the name of the AKS cluster.
func (s *ManagedClusterSpec) ResourceName() string {
	return s.Name
}

// ResourceGroupName returns the name of the resource group.
func (s *ManagedClusterSpec) ResourceGroupName() string {
	return s.ResourceGroup
}

// OwnerResourceName is a no-op for managed clusters.
func (s *ManagedClusterSpec) OwnerResourceName() string {
	return "" // not applicable
}

// CustomHeaders returns custom headers to be added to the Azure API calls.
func (s *ManagedClusterSpec) CustomHeaders() map[string]string {
	return s.Headers
}

// buildAutoScalerProfile builds the AutoScalerProfile for the ManagedClusterProperties.
func buildAutoScalerProfile(autoScalerProfile *AutoScalerProfile) *containerservice.ManagedClusterPropertiesAutoScalerProfile {
	if autoScalerProfile == nil {
		return nil
	}

	mcAutoScalerProfile := &containerservice.ManagedClusterPropertiesAutoScalerProfile{
		BalanceSimilarNodeGroups:      autoScalerProfile.BalanceSimilarNodeGroups,
		MaxEmptyBulkDelete:            autoScalerProfile.MaxEmptyBulkDelete,
		MaxGracefulTerminationSec:     autoScalerProfile.MaxGracefulTerminationSec,
		MaxNodeProvisionTime:          autoScalerProfile.MaxNodeProvisionTime,
		MaxTotalUnreadyPercentage:     autoScalerProfile.MaxTotalUnreadyPercentage,
		NewPodScaleUpDelay:            autoScalerProfile.NewPodScaleUpDelay,
		OkTotalUnreadyCount:           autoScalerProfile.OkTotalUnreadyCount,
		ScanInterval:                  autoScalerProfile.ScanInterval,
		ScaleDownDelayAfterAdd:        autoScalerProfile.ScaleDownDelayAfterAdd,
		ScaleDownDelayAfterDelete:     autoScalerProfile.ScaleDownDelayAfterDelete,
		ScaleDownDelayAfterFailure:    autoScalerProfile.ScaleDownDelayAfterFailure,
		ScaleDownUnneededTime:         autoScalerProfile.ScaleDownUnneededTime,
		ScaleDownUnreadyTime:          autoScalerProfile.ScaleDownUnreadyTime,
		ScaleDownUtilizationThreshold: autoScalerProfile.ScaleDownUtilizationThreshold,
		SkipNodesWithLocalStorage:     autoScalerProfile.SkipNodesWithLocalStorage,
		SkipNodesWithSystemPods:       autoScalerProfile.SkipNodesWithSystemPods,
	}
	if autoScalerProfile.Expander != nil {
		mcAutoScalerProfile.Expander = containerservice.Expander(*autoScalerProfile.Expander)
	}

	return mcAutoScalerProfile
}

// Parameters returns the parameters for the managed clusters.
// TODO: refactor this function to reduce cyclomatic complexity
//
//nolint:gocyclo // Function requires a lot of nil checks that raise complexity.
func (s *ManagedClusterSpec) Parameters(ctx context.Context, existing interface{}) (params interface{}, err error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "managedclusters.Service.Parameters")
	defer done()

	decodedSSHPublicKey, err := base64.StdEncoding.DecodeString(s.SSHPublicKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode SSHPublicKey")
	}
	managedCluster := containerservice.ManagedCluster{
		Identity: &containerservice.ManagedClusterIdentity{
			Type: containerservice.ResourceIdentityTypeSystemAssigned,
		},
		Location: &s.Location,
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			ClusterName: s.ClusterName,
			Name:        pointer.String(s.Name),
			Role:        pointer.String(infrav1.CommonRole),
			Additional:  s.Tags,
		})),
		ManagedClusterProperties: &containerservice.ManagedClusterProperties{
			NodeResourceGroup: &s.NodeResourceGroup,
			EnableRBAC:        pointer.Bool(true),
			DNSPrefix:         &s.Name,
			KubernetesVersion: &s.Version,
			LinuxProfile: &containerservice.LinuxProfile{
				AdminUsername: pointer.String(azure.DefaultAKSUserName),
				SSH: &containerservice.SSHConfiguration{
					PublicKeys: &[]containerservice.SSHPublicKey{
						{
							KeyData: pointer.String(string(decodedSSHPublicKey)),
						},
					},
				},
			},
			ServicePrincipalProfile: &containerservice.ManagedClusterServicePrincipalProfile{
				ClientID: pointer.String("msi"),
			},
			AgentPoolProfiles: &[]containerservice.ManagedClusterAgentPoolProfile{},
			NetworkProfile: &containerservice.NetworkProfile{
				NetworkPlugin:   containerservice.NetworkPlugin(s.NetworkPlugin),
				LoadBalancerSku: containerservice.LoadBalancerSku(s.LoadBalancerSKU),
				NetworkPolicy:   containerservice.NetworkPolicy(s.NetworkPolicy),
			},
		},
	}

	if s.PodCIDR != "" {
		managedCluster.NetworkProfile.PodCidr = &s.PodCIDR
	}

	if s.ServiceCIDR != "" {
		if s.DNSServiceIP == nil {
			managedCluster.NetworkProfile.ServiceCidr = &s.ServiceCIDR
			ip, _, err := net.ParseCIDR(s.ServiceCIDR)
			if err != nil {
				return nil, fmt.Errorf("failed to parse service cidr: %w", err)
			}
			// HACK: set the last octet of the IP to .10
			// This ensures the dns IP is valid in the service cidr without forcing the user
			// to specify it in both the Capi cluster and the Azure control plane.
			// https://golang.org/src/net/ip.go#L48
			ip[15] = byte(10)
			dnsIP := ip.String()
			managedCluster.NetworkProfile.DNSServiceIP = &dnsIP
		} else {
			managedCluster.NetworkProfile.DNSServiceIP = s.DNSServiceIP
		}
	}

	if s.AADProfile != nil {
		managedCluster.AadProfile = &containerservice.ManagedClusterAADProfile{
			Managed:             &s.AADProfile.Managed,
			EnableAzureRBAC:     &s.AADProfile.EnableAzureRBAC,
			AdminGroupObjectIDs: &s.AADProfile.AdminGroupObjectIDs,
		}
	}

	for i := range s.AddonProfiles {
		if managedCluster.AddonProfiles == nil {
			managedCluster.AddonProfiles = map[string]*containerservice.ManagedClusterAddonProfile{}
		}
		item := s.AddonProfiles[i]
		addonProfile := &containerservice.ManagedClusterAddonProfile{
			Enabled: &item.Enabled,
		}
		if item.Config != nil {
			addonProfile.Config = azure.StringMapPtr(item.Config)
		}
		managedCluster.AddonProfiles[item.Name] = addonProfile
	}

	if s.SKU != nil {
		tierName := containerservice.ManagedClusterSKUTier(s.SKU.Tier)
		managedCluster.Sku = &containerservice.ManagedClusterSKU{
			Name: containerservice.ManagedClusterSKUNameBasic,
			Tier: tierName,
		}
	}

	if s.LoadBalancerProfile != nil {
		managedCluster.NetworkProfile.LoadBalancerProfile = &containerservice.ManagedClusterLoadBalancerProfile{
			AllocatedOutboundPorts: s.LoadBalancerProfile.AllocatedOutboundPorts,
			IdleTimeoutInMinutes:   s.LoadBalancerProfile.IdleTimeoutInMinutes,
		}
		if s.LoadBalancerProfile.ManagedOutboundIPs != nil {
			managedCluster.NetworkProfile.LoadBalancerProfile.ManagedOutboundIPs = &containerservice.ManagedClusterLoadBalancerProfileManagedOutboundIPs{Count: s.LoadBalancerProfile.ManagedOutboundIPs}
		}
		if len(s.LoadBalancerProfile.OutboundIPPrefixes) > 0 {
			managedCluster.NetworkProfile.LoadBalancerProfile.OutboundIPPrefixes = &containerservice.ManagedClusterLoadBalancerProfileOutboundIPPrefixes{
				PublicIPPrefixes: convertToResourceReferences(s.LoadBalancerProfile.OutboundIPPrefixes),
			}
		}
		if len(s.LoadBalancerProfile.OutboundIPs) > 0 {
			managedCluster.NetworkProfile.LoadBalancerProfile.OutboundIPs = &containerservice.ManagedClusterLoadBalancerProfileOutboundIPs{
				PublicIPs: convertToResourceReferences(s.LoadBalancerProfile.OutboundIPs),
			}
		}
	}

	if s.APIServerAccessProfile != nil {
		managedCluster.APIServerAccessProfile = &containerservice.ManagedClusterAPIServerAccessProfile{
			EnablePrivateCluster:           s.APIServerAccessProfile.EnablePrivateCluster,
			PrivateDNSZone:                 s.APIServerAccessProfile.PrivateDNSZone,
			EnablePrivateClusterPublicFQDN: s.APIServerAccessProfile.EnablePrivateClusterPublicFQDN,
		}

		if s.APIServerAccessProfile.AuthorizedIPRanges != nil {
			managedCluster.APIServerAccessProfile.AuthorizedIPRanges = &s.APIServerAccessProfile.AuthorizedIPRanges
		}
	}

	if s.OutboundType != nil {
		managedCluster.NetworkProfile.OutboundType = containerservice.OutboundType(*s.OutboundType)
	}

	managedCluster.AutoScalerProfile = buildAutoScalerProfile(s.AutoScalerProfile)

	if existing != nil {
		existingMC, ok := existing.(containerservice.ManagedCluster)
		if !ok {
			return nil, fmt.Errorf("%T is not a containerservice.ManagedCluster", existing)
		}
		ps := *existingMC.ManagedClusterProperties.ProvisioningState
		if ps != string(infrav1.Canceled) && ps != string(infrav1.Failed) && ps != string(infrav1.Succeeded) {
			return nil, azure.WithTransientError(errors.Errorf("Unable to update existing managed cluster in non-terminal state. Managed cluster must be in one of the following provisioning states: Canceled, Failed, or Succeeded. Actual state: %s", ps), 20*time.Second)
		}

		// Normalize the LoadBalancerProfile so the diff below doesn't get thrown off by AKS added properties.
		if managedCluster.NetworkProfile.LoadBalancerProfile == nil {
			// If our LoadBalancerProfile generated by the spec is nil, then don't worry about what AKS has added.
			existingMC.NetworkProfile.LoadBalancerProfile = nil
		} else {
			// If our LoadBalancerProfile generated by the spec is not nil, then remove the effective outbound IPs from
			// AKS.
			existingMC.NetworkProfile.LoadBalancerProfile.EffectiveOutboundIPs = nil
		}

		// Avoid changing agent pool profiles through AMCP and just use the existing agent pool profiles
		// AgentPool changes are managed through AMMP.
		managedCluster.AgentPoolProfiles = existingMC.AgentPoolProfiles

		// if the AuthorizedIPRanges is nil in the user-updated spec, but not nil in the existing spec, then
		// we need to set the AuthorizedIPRanges to empty array (&[]string{}) once so that the Azure API will
		// update the existing authorized IP ranges to nil.
		if !isAuthIPRangesNilOrEmpty(existingMC) && isAuthIPRangesNilOrEmpty(managedCluster) {
			log.V(4).Info("managed cluster spec has nil AuthorizedIPRanges, updating existing authorized IP ranges to an empty list")
			managedCluster.APIServerAccessProfile = &containerservice.ManagedClusterAPIServerAccessProfile{
				AuthorizedIPRanges: &[]string{},
			}
		}

		diff := computeDiffOfNormalizedClusters(managedCluster, existingMC)
		if diff == "" {
			log.V(4).Info("no changes found between user-updated spec and existing spec")
			return nil, nil
		}
		log.V(4).Info("found a diff between the desired spec and the existing managed cluster", "difference", diff)
	} else {
		// Add all agent pools to cluster spec that will be submitted to the API
		agentPoolSpecs, err := s.GetAllAgentPools()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get agent pool specs for managed cluster %s", s.Name)
		}

		for _, spec := range agentPoolSpecs {
			params, err := spec.Parameters(ctx, nil)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get agent pool parameters for managed cluster %s", s.Name)
			}
			agentPool, ok := params.(containerservice.AgentPool)
			if !ok {
				return nil, fmt.Errorf("%T is not a containerservice.AgentPool", agentPool)
			}
			agentPool.Name = pointer.String(spec.ResourceName())
			profile := converters.AgentPoolToManagedClusterAgentPoolProfile(agentPool)
			*managedCluster.AgentPoolProfiles = append(*managedCluster.AgentPoolProfiles, profile)
		}
	}

	return managedCluster, nil
}

func convertToResourceReferences(resources []string) *[]containerservice.ResourceReference {
	resourceReferences := make([]containerservice.ResourceReference, len(resources))
	for i := range resources {
		resourceReferences[i] = containerservice.ResourceReference{ID: &resources[i]}
	}
	return &resourceReferences
}

func computeDiffOfNormalizedClusters(managedCluster containerservice.ManagedCluster, existingMC containerservice.ManagedCluster) string {
	// Normalize properties for the desired (CR spec) and existing managed
	// cluster, so that we check only those fields that were specified in
	// the initial CreateOrUpdate request and that can be modified.
	// Without comparing to normalized properties, we would always get a
	// difference in desired and existing, which would result in sending
	// unnecessary Azure API requests.
	propertiesNormalized := &containerservice.ManagedClusterProperties{
		KubernetesVersion: managedCluster.ManagedClusterProperties.KubernetesVersion,
		NetworkProfile:    &containerservice.NetworkProfile{},
		AutoScalerProfile: &containerservice.ManagedClusterPropertiesAutoScalerProfile{},
	}

	existingMCPropertiesNormalized := &containerservice.ManagedClusterProperties{
		KubernetesVersion: existingMC.ManagedClusterProperties.KubernetesVersion,
		NetworkProfile:    &containerservice.NetworkProfile{},
		AutoScalerProfile: &containerservice.ManagedClusterPropertiesAutoScalerProfile{},
	}

	if managedCluster.AadProfile != nil {
		propertiesNormalized.AadProfile = &containerservice.ManagedClusterAADProfile{
			Managed:             managedCluster.AadProfile.Managed,
			EnableAzureRBAC:     managedCluster.AadProfile.EnableAzureRBAC,
			AdminGroupObjectIDs: managedCluster.AadProfile.AdminGroupObjectIDs,
		}
	}

	if existingMC.AadProfile != nil {
		existingMCPropertiesNormalized.AadProfile = &containerservice.ManagedClusterAADProfile{
			Managed:             existingMC.AadProfile.Managed,
			EnableAzureRBAC:     existingMC.AadProfile.EnableAzureRBAC,
			AdminGroupObjectIDs: existingMC.AadProfile.AdminGroupObjectIDs,
		}
	}

	if managedCluster.NetworkProfile != nil {
		propertiesNormalized.NetworkProfile.LoadBalancerProfile = managedCluster.NetworkProfile.LoadBalancerProfile
	}

	if existingMC.NetworkProfile != nil {
		existingMCPropertiesNormalized.NetworkProfile.LoadBalancerProfile = existingMC.NetworkProfile.LoadBalancerProfile
	}

	if managedCluster.APIServerAccessProfile != nil {
		propertiesNormalized.APIServerAccessProfile = &containerservice.ManagedClusterAPIServerAccessProfile{
			AuthorizedIPRanges: managedCluster.APIServerAccessProfile.AuthorizedIPRanges,
		}
	}

	if existingMC.APIServerAccessProfile != nil {
		existingMCPropertiesNormalized.APIServerAccessProfile = &containerservice.ManagedClusterAPIServerAccessProfile{
			AuthorizedIPRanges: existingMC.APIServerAccessProfile.AuthorizedIPRanges,
		}
	}

	if managedCluster.AutoScalerProfile != nil {
		propertiesNormalized.AutoScalerProfile = &containerservice.ManagedClusterPropertiesAutoScalerProfile{
			BalanceSimilarNodeGroups:      managedCluster.AutoScalerProfile.BalanceSimilarNodeGroups,
			Expander:                      managedCluster.AutoScalerProfile.Expander,
			MaxEmptyBulkDelete:            managedCluster.AutoScalerProfile.MaxEmptyBulkDelete,
			MaxGracefulTerminationSec:     managedCluster.AutoScalerProfile.MaxGracefulTerminationSec,
			MaxNodeProvisionTime:          managedCluster.AutoScalerProfile.MaxNodeProvisionTime,
			MaxTotalUnreadyPercentage:     managedCluster.AutoScalerProfile.MaxTotalUnreadyPercentage,
			NewPodScaleUpDelay:            managedCluster.AutoScalerProfile.NewPodScaleUpDelay,
			OkTotalUnreadyCount:           managedCluster.AutoScalerProfile.OkTotalUnreadyCount,
			ScanInterval:                  managedCluster.AutoScalerProfile.ScanInterval,
			ScaleDownDelayAfterAdd:        managedCluster.AutoScalerProfile.ScaleDownDelayAfterAdd,
			ScaleDownDelayAfterDelete:     managedCluster.AutoScalerProfile.ScaleDownDelayAfterDelete,
			ScaleDownDelayAfterFailure:    managedCluster.AutoScalerProfile.ScaleDownDelayAfterFailure,
			ScaleDownUnneededTime:         managedCluster.AutoScalerProfile.ScaleDownUnneededTime,
			ScaleDownUnreadyTime:          managedCluster.AutoScalerProfile.ScaleDownUnreadyTime,
			ScaleDownUtilizationThreshold: managedCluster.AutoScalerProfile.ScaleDownUtilizationThreshold,
			SkipNodesWithLocalStorage:     managedCluster.AutoScalerProfile.SkipNodesWithLocalStorage,
			SkipNodesWithSystemPods:       managedCluster.AutoScalerProfile.SkipNodesWithSystemPods,
		}
	}

	if existingMC.AutoScalerProfile != nil {
		existingMCPropertiesNormalized.AutoScalerProfile = &containerservice.ManagedClusterPropertiesAutoScalerProfile{
			BalanceSimilarNodeGroups:      existingMC.AutoScalerProfile.BalanceSimilarNodeGroups,
			Expander:                      existingMC.AutoScalerProfile.Expander,
			MaxEmptyBulkDelete:            existingMC.AutoScalerProfile.MaxEmptyBulkDelete,
			MaxGracefulTerminationSec:     existingMC.AutoScalerProfile.MaxGracefulTerminationSec,
			MaxNodeProvisionTime:          existingMC.AutoScalerProfile.MaxNodeProvisionTime,
			MaxTotalUnreadyPercentage:     existingMC.AutoScalerProfile.MaxTotalUnreadyPercentage,
			NewPodScaleUpDelay:            existingMC.AutoScalerProfile.NewPodScaleUpDelay,
			OkTotalUnreadyCount:           existingMC.AutoScalerProfile.OkTotalUnreadyCount,
			ScanInterval:                  existingMC.AutoScalerProfile.ScanInterval,
			ScaleDownDelayAfterAdd:        existingMC.AutoScalerProfile.ScaleDownDelayAfterAdd,
			ScaleDownDelayAfterDelete:     existingMC.AutoScalerProfile.ScaleDownDelayAfterDelete,
			ScaleDownDelayAfterFailure:    existingMC.AutoScalerProfile.ScaleDownDelayAfterFailure,
			ScaleDownUnneededTime:         existingMC.AutoScalerProfile.ScaleDownUnneededTime,
			ScaleDownUnreadyTime:          existingMC.AutoScalerProfile.ScaleDownUnreadyTime,
			ScaleDownUtilizationThreshold: existingMC.AutoScalerProfile.ScaleDownUtilizationThreshold,
			SkipNodesWithLocalStorage:     existingMC.AutoScalerProfile.SkipNodesWithLocalStorage,
			SkipNodesWithSystemPods:       existingMC.AutoScalerProfile.SkipNodesWithSystemPods,
		}
	}

	// Once the AKS autoscaler has been updated it will always return values so we need to
	// respect those values even though the settings are now not being explicitly set by CAPZ.
	if existingMC.AutoScalerProfile != nil && managedCluster.AutoScalerProfile == nil {
		existingMCPropertiesNormalized.AutoScalerProfile = nil
		propertiesNormalized.AutoScalerProfile = nil
	}

	clusterNormalized := &containerservice.ManagedCluster{
		ManagedClusterProperties: propertiesNormalized,
	}
	existingMCClusterNormalized := &containerservice.ManagedCluster{
		ManagedClusterProperties: existingMCPropertiesNormalized,
	}

	if managedCluster.Sku != nil {
		clusterNormalized.Sku = managedCluster.Sku
	}
	if existingMC.Sku != nil {
		existingMCClusterNormalized.Sku = existingMC.Sku
	}

	diff := cmp.Diff(clusterNormalized, existingMCClusterNormalized)
	return diff
}

// isAuthIPRangesNilOrEmpty returns true if the managed cluster's APIServerAccessProfile or AuthorizedIPRanges is nil or if AuthorizedIPRanges is empty.
func isAuthIPRangesNilOrEmpty(managedCluster containerservice.ManagedCluster) bool {
	if managedCluster.APIServerAccessProfile == nil || managedCluster.APIServerAccessProfile.AuthorizedIPRanges == nil || reflect.DeepEqual(managedCluster.APIServerAccessProfile.AuthorizedIPRanges, &[]string{}) {
		return true
	}
	return false
}

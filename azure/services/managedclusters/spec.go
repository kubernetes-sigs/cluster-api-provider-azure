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

	asocontainerservicev1preview "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20230202preview"
	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231001"
	asocontainerservicev1hub "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231001/storage"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/agentpools"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/aso"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
	"sigs.k8s.io/cluster-api-provider-azure/util/versions"
	"sigs.k8s.io/cluster-api/util/secret"
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

	// NetworkPluginMode is the mode the network plugin should use.
	NetworkPluginMode *infrav1.NetworkPluginMode

	// NetworkPolicy used for building Kubernetes network. Possible values include: 'azure', 'calico', 'cilium'.
	NetworkPolicy string

	// NetworkDataplane used for building Kubernetes network. Possible values include: 'azure', 'cilium'.
	NetworkDataplane *infrav1.NetworkDataplaneType

	// OutboundType used for building Kubernetes network. Possible values include: 'loadBalancer', 'managedNATGateway', 'userAssignedNATGateway', 'userDefinedRouting'.
	OutboundType *infrav1.ManagedControlPlaneOutboundType

	// SSHPublicKey is a string literal containing an ssh public key. Will autogenerate and discard if not provided.
	SSHPublicKey string

	// GetAllAgentPools is a function that returns the list of agent pool specifications in this cluster.
	GetAllAgentPools func() ([]azure.ASOResourceSpecGetter[genruntime.MetaObject], error)

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

	// AutoScalerProfile is the parameters to be applied to the cluster-autoscaler when enabled.
	AutoScalerProfile *AutoScalerProfile

	// Identity is the AKS control plane Identity configuration
	Identity *infrav1.Identity

	// KubeletUserAssignedIdentity is the user-assigned identity for kubelet to authenticate to ACR.
	KubeletUserAssignedIdentity string

	// HTTPProxyConfig is the HTTP proxy configuration for the cluster.
	HTTPProxyConfig *HTTPProxyConfig

	// OIDCIssuerProfile is the OIDC issuer profile of the Managed Cluster.
	OIDCIssuerProfile *OIDCIssuerProfile

	// DNSPrefix allows the user to customize dns prefix.
	DNSPrefix *string

	// DisableLocalAccounts disables getting static credentials for this cluster when set. Expected to only be used for AAD clusters.
	DisableLocalAccounts *bool

	// AutoUpgradeProfile defines auto upgrade configuration.
	AutoUpgradeProfile *ManagedClusterAutoUpgradeProfile

	// SecurityProfile defines the security profile for the cluster.
	SecurityProfile *ManagedClusterSecurityProfile

	// Patches are extra patches to be applied to the ASO resource.
	Patches []string

	// Preview enables the preview API version.
	Preview bool
}

// ManagedClusterAutoUpgradeProfile auto upgrade profile for a managed cluster.
type ManagedClusterAutoUpgradeProfile struct {
	// UpgradeChannel defines the channel for auto upgrade configuration.
	UpgradeChannel *infrav1.UpgradeChannel
}

// HTTPProxyConfig is the HTTP proxy configuration for the cluster.
type HTTPProxyConfig struct {
	// HTTPProxy is the HTTP proxy server endpoint to use.
	HTTPProxy *string `json:"httpProxy,omitempty"`

	// HTTPSProxy is the HTTPS proxy server endpoint to use.
	HTTPSProxy *string `json:"httpsProxy,omitempty"`

	// NoProxy is the endpoints that should not go through proxy.
	NoProxy []string `json:"noProxy,omitempty"`

	// TrustedCA is the Alternative CA cert to use for connecting to proxy servers.
	TrustedCA *string `json:"trustedCa,omitempty"`
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
	ManagedOutboundIPs *int

	// OutboundIPPrefixes are the desired outbound IP Prefix resources for the cluster load balancer.
	OutboundIPPrefixes []string

	// OutboundIPs are the desired outbound IP resources for the cluster load balancer.
	OutboundIPs []string

	// AllocatedOutboundPorts are the desired number of allocated SNAT ports per VM. Allowed values must be in the range of 0 to 64000 (inclusive). The default value is 0 which results in Azure dynamically allocating ports.
	AllocatedOutboundPorts *int

	// IdleTimeoutInMinutes  are the desired outbound flow idle timeout in minutes. Allowed values must be in the range of 4 to 120 (inclusive). The default value is 30 minutes.
	IdleTimeoutInMinutes *int
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

// OIDCIssuerProfile is the OIDC issuer profile of the Managed Cluster.
type OIDCIssuerProfile struct {
	// Enabled is whether the OIDC issuer is enabled.
	Enabled *bool
}

// ManagedClusterSecurityProfile defines the security profile for the cluster.
type ManagedClusterSecurityProfile struct {
	// AzureKeyVaultKms defines Azure Key Vault key management service settings for the security profile.
	AzureKeyVaultKms *AzureKeyVaultKms

	// Defender defines Microsoft Defender settings for the security profile.
	Defender *ManagedClusterSecurityProfileDefender

	// ImageCleaner settings for the security profile.
	ImageCleaner *ManagedClusterSecurityProfileImageCleaner

	// Workloadidentity enables Kubernetes applications to access Azure cloud resources securely with Azure AD.
	WorkloadIdentity *ManagedClusterSecurityProfileWorkloadIdentity
}

// ManagedClusterSecurityProfileDefender defines Microsoft Defender settings for the security profile.
type ManagedClusterSecurityProfileDefender struct {
	// LogAnalyticsWorkspaceResourceID is the ID of the Log Analytics workspace that has to be associated with Microsoft Defender.
	// When Microsoft Defender is enabled, this field is required and must be a valid workspace resource ID.
	LogAnalyticsWorkspaceResourceID *string

	// SecurityMonitoring profile defines the Microsoft Defender threat detection for Cloud settings for the security profile.
	SecurityMonitoring *ManagedClusterSecurityProfileDefenderSecurityMonitoring
}

// ManagedClusterSecurityProfileDefenderSecurityMonitoring settings for the security profile threat detection.
type ManagedClusterSecurityProfileDefenderSecurityMonitoring struct {
	// Enabled enables Defender threat detection
	Enabled *bool
}

// ManagedClusterSecurityProfileImageCleaner removes unused images from nodes, freeing up disk space and helping to reduce attack surface area.
type ManagedClusterSecurityProfileImageCleaner struct {
	// Enabled enables Image Cleaner on AKS cluster.
	Enabled *bool

	// Image Cleaner scanning interval in hours.
	IntervalHours *int
}

// ManagedClusterSecurityProfileWorkloadIdentity defines Workload identity settings for the security profile.
type ManagedClusterSecurityProfileWorkloadIdentity struct {
	// Enabled enables workload identity.
	Enabled *bool
}

// AzureKeyVaultKms Azure Key Vault key management service settings for the security profile.
type AzureKeyVaultKms struct {
	// Enabled enables Azure Key Vault key management service. The default is false.
	Enabled *bool

	// KeyID defines the Identifier of Azure Key Vault key.
	// When Azure Key Vault key management service is enabled, this field is required and must be a valid key identifier.
	KeyID *string

	// KeyVaultNetworkAccess defines the network access of key vault.
	// The possible values are Public and Private.
	// Public means the key vault allows public access from all networks.
	// Private means the key vault disables public access and enables private link. The default value is Public.
	KeyVaultNetworkAccess *infrav1.KeyVaultNetworkAccessTypes

	// KeyVaultResourceID is the Resource ID of key vault. When keyVaultNetworkAccess is Private, this field is required and must be a valid resource ID.
	KeyVaultResourceID *string
}

// buildAutoScalerProfile builds the AutoScalerProfile for the ManagedClusterProperties.
func (s *ManagedClusterSpec) buildAutoScalerProfile(managedCluster *asocontainerservicev1.ManagedCluster) {
	if s.AutoScalerProfile == nil {
		return
	}

	autoScalerProfile := s.AutoScalerProfile
	mcAutoScalerProfile := &asocontainerservicev1.ManagedClusterProperties_AutoScalerProfile{
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
		mcAutoScalerProfile.Expander = ptr.To(asocontainerservicev1.ManagedClusterProperties_AutoScalerProfile_Expander(*autoScalerProfile.Expander))
	}

	managedCluster.Spec.AutoScalerProfile = mcAutoScalerProfile
}

// buildManagedClusterVersion gets the desired managed k8s version.
// If autoupgrade channels is set to patch, stable or rapid, clusters can be upgraded to higher version by AKS.
// If autoupgrade is triggered, existing kubernetes version will be higher than the user desired kubernetes version.
// CAPZ should honour the upgrade and it should not downgrade to the lower desired version.
func (s *ManagedClusterSpec) buildManagedClusterVersion(managedCluster, existing *asocontainerservicev1.ManagedCluster) {
	if existing == nil || existing.Status.CurrentKubernetesVersion == nil {
		return
	}

	kubernetesVersion := versions.GetHigherK8sVersion(s.Version, *existing.Status.CurrentKubernetesVersion)

	if kubernetesVersion != "" {
		managedCluster.Spec.KubernetesVersion = &kubernetesVersion
	}
}

// ResourceRef implements azure.ASOResourceSpecGetter.
func (s *ManagedClusterSpec) ResourceRef() genruntime.MetaObject {
	if s.Preview {
		return &asocontainerservicev1preview.ManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: s.Name,
			},
		}
	}
	return &asocontainerservicev1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: s.Name,
		},
	}
}

func (s *ManagedClusterSpec) buildNetworkProfile(managedCluster *asocontainerservicev1.ManagedCluster) error {
	networkProfile := &asocontainerservicev1.ContainerServiceNetworkProfile{
		NetworkPlugin:   azure.AliasOrNil[asocontainerservicev1.NetworkPlugin](&s.NetworkPlugin),
		LoadBalancerSku: azure.AliasOrNil[asocontainerservicev1.ContainerServiceNetworkProfile_LoadBalancerSku](&s.LoadBalancerSKU),
		NetworkPolicy:   azure.AliasOrNil[asocontainerservicev1.ContainerServiceNetworkProfile_NetworkPolicy](&s.NetworkPolicy),
	}

	if s.NetworkDataplane != nil {
		networkProfile.NetworkDataplane = ptr.To(asocontainerservicev1.ContainerServiceNetworkProfile_NetworkDataplane(*s.NetworkDataplane))
	}

	if s.NetworkPluginMode != nil {
		networkProfile.NetworkPluginMode = ptr.To(asocontainerservicev1.ContainerServiceNetworkProfile_NetworkPluginMode(*s.NetworkPluginMode))
	}

	if s.PodCIDR != "" {
		networkProfile.PodCidr = &s.PodCIDR
	}

	if s.ServiceCIDR != "" {
		networkProfile.ServiceCidr = &s.ServiceCIDR
		networkProfile.DnsServiceIP = s.DNSServiceIP
		if s.DNSServiceIP == nil {
			ip, _, err := net.ParseCIDR(s.ServiceCIDR)
			if err != nil {
				return fmt.Errorf("failed to parse service cidr: %w", err)
			}
			// HACK: set the last octet of the IP to .10
			// This ensures the dns IP is valid in the service cidr without forcing the user
			// to specify it in both the Capi cluster and the Azure control plane.
			// https://golang.org/src/net/ip.go#L48
			ip[15] = byte(10)
			dnsIP := ip.String()
			networkProfile.DnsServiceIP = &dnsIP
		}
	}

	if s.LoadBalancerProfile != nil {
		networkProfile.LoadBalancerProfile = s.GetLoadBalancerProfile()
	}

	if s.OutboundType != nil {
		networkProfile.OutboundType = ptr.To(asocontainerservicev1.ContainerServiceNetworkProfile_OutboundType(*s.OutboundType))
	}

	managedCluster.Spec.NetworkProfile = networkProfile
	return nil
}

func (s *ManagedClusterSpec) buildOperatorSpec(managedCluster *asocontainerservicev1.ManagedCluster) {
	// OperatorSpec defines how the Secrets generated by ASO should look for the AKS cluster kubeconfigs.
	// There is no prescribed naming convention that must be followed.
	operatorSpec := &asocontainerservicev1.ManagedClusterOperatorSpec{
		Secrets: &asocontainerservicev1.ManagedClusterOperatorSecrets{
			AdminCredentials: &genruntime.SecretDestination{
				Name: adminKubeconfigSecretName(s.ClusterName),
				Key:  secret.KubeconfigDataName,
			},
		},
	}

	if s.AADProfile != nil {
		if ptr.Deref(s.DisableLocalAccounts, false) {
			// admin credentials cannot be fetched when local accounts are disabled
			operatorSpec.Secrets.AdminCredentials = nil
		}
		if s.AADProfile.Managed {
			operatorSpec.Secrets.UserCredentials = &genruntime.SecretDestination{
				Name: userKubeconfigSecretName(s.ClusterName),
				Key:  secret.KubeconfigDataName,
			}
		}
	}

	if s.OIDCIssuerProfile != nil && ptr.Deref(s.OIDCIssuerProfile.Enabled, false) {
		operatorSpec.ConfigMaps = &asocontainerservicev1.ManagedClusterOperatorConfigMaps{
			OIDCIssuerProfile: &genruntime.ConfigMapDestination{
				Name: oidcIssuerURLConfigMapName(s.ClusterName),
				Key:  oidcIssuerProfileURL,
			},
		}
	}
	managedCluster.Spec.OperatorSpec = operatorSpec
}

func (s *ManagedClusterSpec) buildSecurityProfile(managedCluster *asocontainerservicev1.ManagedCluster) {
	if s.SecurityProfile == nil {
		return
	}

	securityProfile := &asocontainerservicev1.ManagedClusterSecurityProfile{}
	if s.SecurityProfile.AzureKeyVaultKms != nil {
		securityProfile.AzureKeyVaultKms = &asocontainerservicev1.AzureKeyVaultKms{
			Enabled: s.SecurityProfile.AzureKeyVaultKms.Enabled,
			KeyId:   s.SecurityProfile.AzureKeyVaultKms.KeyID,
		}
		if s.SecurityProfile.AzureKeyVaultKms.KeyVaultNetworkAccess != nil {
			keyVaultNetworkAccess := string(*s.SecurityProfile.AzureKeyVaultKms.KeyVaultNetworkAccess)
			securityProfile.AzureKeyVaultKms.KeyVaultNetworkAccess = ptr.To(asocontainerservicev1.AzureKeyVaultKms_KeyVaultNetworkAccess(keyVaultNetworkAccess))
		}
		if s.SecurityProfile.AzureKeyVaultKms.KeyVaultResourceID != nil {
			securityProfile.AzureKeyVaultKms.KeyVaultResourceReference = &genruntime.ResourceReference{
				ARMID: *s.SecurityProfile.AzureKeyVaultKms.KeyVaultResourceID,
			}
		}
	}
	if s.SecurityProfile.Defender != nil {
		securityProfile.Defender = &asocontainerservicev1.ManagedClusterSecurityProfileDefender{
			LogAnalyticsWorkspaceResourceReference: &genruntime.ResourceReference{
				ARMID: *s.SecurityProfile.Defender.LogAnalyticsWorkspaceResourceID,
			},
		}
		if s.SecurityProfile.Defender.SecurityMonitoring != nil {
			securityProfile.Defender.SecurityMonitoring = &asocontainerservicev1.ManagedClusterSecurityProfileDefenderSecurityMonitoring{
				Enabled: s.SecurityProfile.Defender.SecurityMonitoring.Enabled,
			}
		}
	}
	if s.SecurityProfile.ImageCleaner != nil {
		securityProfile.ImageCleaner = &asocontainerservicev1.ManagedClusterSecurityProfileImageCleaner{
			Enabled:       s.SecurityProfile.ImageCleaner.Enabled,
			IntervalHours: s.SecurityProfile.ImageCleaner.IntervalHours,
		}
	}
	if s.SecurityProfile.WorkloadIdentity != nil {
		securityProfile.WorkloadIdentity = &asocontainerservicev1.ManagedClusterSecurityProfileWorkloadIdentity{
			Enabled: s.SecurityProfile.WorkloadIdentity.Enabled,
		}
	}
	managedCluster.Spec.SecurityProfile = securityProfile
}

func (s *ManagedClusterSpec) buildLinuxSSHProfile(managedCluster *asocontainerservicev1.ManagedCluster) error {
	var decodedSSHPublicKey []byte
	if s.SSHPublicKey == "" {
		return nil
	}

	decodedSSHPublicKey, err := base64.StdEncoding.DecodeString(s.SSHPublicKey)
	if err != nil {
		return errors.Wrap(err, "failed to decode SSHPublicKey")
	}

	if decodedSSHPublicKey == nil {
		return nil
	}
	managedCluster.Spec.LinuxProfile = &asocontainerservicev1.ContainerServiceLinuxProfile{
		AdminUsername: ptr.To(azure.DefaultAKSUserName),
		Ssh: &asocontainerservicev1.ContainerServiceSshConfiguration{
			PublicKeys: []asocontainerservicev1.ContainerServiceSshPublicKey{
				{
					KeyData: ptr.To(string(decodedSSHPublicKey)),
				},
			},
		},
	}
	return nil
}

func (s *ManagedClusterSpec) buildAPIServerAccessProfile(managedCluster *asocontainerservicev1.ManagedCluster) {
	if s.APIServerAccessProfile == nil {
		return
	}

	apiServerAccessProfile := &asocontainerservicev1.ManagedClusterAPIServerAccessProfile{
		EnablePrivateCluster:           s.APIServerAccessProfile.EnablePrivateCluster,
		PrivateDNSZone:                 s.APIServerAccessProfile.PrivateDNSZone,
		EnablePrivateClusterPublicFQDN: s.APIServerAccessProfile.EnablePrivateClusterPublicFQDN,
	}

	if s.APIServerAccessProfile.AuthorizedIPRanges != nil {
		apiServerAccessProfile.AuthorizedIPRanges = s.APIServerAccessProfile.AuthorizedIPRanges
	}

	managedCluster.Spec.ApiServerAccessProfile = apiServerAccessProfile
}

func (s *ManagedClusterSpec) buildHTTPProxyConfig(managedCluster *asocontainerservicev1.ManagedCluster) {
	if s.HTTPProxyConfig == nil {
		return
	}

	httpProxyConfig := &asocontainerservicev1.ManagedClusterHTTPProxyConfig{
		HttpProxy:  s.HTTPProxyConfig.HTTPProxy,
		HttpsProxy: s.HTTPProxyConfig.HTTPSProxy,
		TrustedCa:  s.HTTPProxyConfig.TrustedCA,
	}

	if s.HTTPProxyConfig.NoProxy != nil {
		httpProxyConfig.NoProxy = s.HTTPProxyConfig.NoProxy
	}
	managedCluster.Spec.HttpProxyConfig = httpProxyConfig
}

func (s *ManagedClusterSpec) buildSKU(managedCluster *asocontainerservicev1.ManagedCluster) {
	if s.SKU == nil {
		return
	}

	tierName := asocontainerservicev1.ManagedClusterSKU_Tier(s.SKU.Tier)
	managedCluster.Spec.Sku = &asocontainerservicev1.ManagedClusterSKU{
		Name: ptr.To(asocontainerservicev1.ManagedClusterSKU_Name("Base")),
		Tier: ptr.To(tierName),
	}
}

func (s *ManagedClusterSpec) buildAddonProfiles(managedCluster *asocontainerservicev1.ManagedCluster) {
	if s.AddonProfiles == nil {
		return
	}

	if managedCluster.Spec.AddonProfiles == nil {
		managedCluster.Spec.AddonProfiles = map[string]asocontainerservicev1.ManagedClusterAddonProfile{}
	}

	for i := range s.AddonProfiles {
		item := s.AddonProfiles[i]
		addonProfile := asocontainerservicev1.ManagedClusterAddonProfile{
			Enabled: &item.Enabled,
		}
		if item.Config != nil {
			addonProfile.Config = item.Config
		}
		managedCluster.Spec.AddonProfiles[item.Name] = addonProfile
	}
}

func (s *ManagedClusterSpec) buildIdentityProfile(managedCluster *asocontainerservicev1.ManagedCluster) {
	if s.KubeletUserAssignedIdentity == "" {
		return
	}

	managedCluster.Spec.IdentityProfile = map[string]asocontainerservicev1.UserAssignedIdentity{
		kubeletIdentityKey: {
			ResourceReference: &genruntime.ResourceReference{
				ARMID: s.KubeletUserAssignedIdentity,
			},
		},
	}
}

func (s *ManagedClusterSpec) buildAADProfile(managedCluster *asocontainerservicev1.ManagedCluster) {
	if s.AADProfile == nil {
		return
	}

	managedCluster.Spec.AadProfile = &asocontainerservicev1.ManagedClusterAADProfile{
		Managed:             &s.AADProfile.Managed,
		EnableAzureRBAC:     &s.AADProfile.EnableAzureRBAC,
		AdminGroupObjectIDs: s.AADProfile.AdminGroupObjectIDs,
	}

	if s.DisableLocalAccounts != nil {
		managedCluster.Spec.DisableLocalAccounts = s.DisableLocalAccounts
	}
}

func convertPreviewToV1(previewCluster *asocontainerservicev1preview.ManagedCluster) (*asocontainerservicev1.ManagedCluster, error) {
	hub := &asocontainerservicev1hub.ManagedCluster{}
	if err := previewCluster.ConvertTo(hub); err != nil {
		return nil, err
	}
	stable := &asocontainerservicev1.ManagedCluster{}
	if err := stable.ConvertFrom(hub); err != nil {
		return nil, err
	}
	return stable.DeepCopy(), nil
}

func convertV1ToPreview(managedCluster *asocontainerservicev1.ManagedCluster) (*asocontainerservicev1preview.ManagedCluster, error) {
	hub := &asocontainerservicev1hub.ManagedCluster{}
	if err := managedCluster.ConvertTo(hub); err != nil {
		return nil, err
	}
	prev := &asocontainerservicev1preview.ManagedCluster{}
	if err := prev.ConvertFrom(hub); err != nil {
		return nil, err
	}
	return prev, nil
}

// Parameters returns the parameters for the managed clusters.
func (s *ManagedClusterSpec) Parameters(ctx context.Context, existingObj genruntime.MetaObject) (params genruntime.MetaObject, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "managedclusters.Service.Parameters")
	defer done()

	// If existing is preview, convert to stable then back to preview at the end of the function.
	var existing *asocontainerservicev1.ManagedCluster
	var existingStatus asocontainerservicev1preview.ManagedCluster_STATUS

	if existingObj != nil {
		if s.Preview {
			existingPreview := existingObj.(*asocontainerservicev1preview.ManagedCluster)
			existingStatus = existingPreview.Status

			existing, err = convertPreviewToV1(existingPreview)
			if err != nil {
				return nil, err
			}
		} else {
			existing = existingObj.(*asocontainerservicev1.ManagedCluster)
		}
	}

	managedCluster := existing
	if managedCluster == nil {
		managedCluster = &asocontainerservicev1.ManagedCluster{
			Spec: asocontainerservicev1.ManagedCluster_Spec{
				Tags: infrav1.Build(infrav1.BuildParams{
					Lifecycle:   infrav1.ResourceLifecycleOwned,
					ClusterName: s.ClusterName,
					Name:        ptr.To(s.Name),
					Role:        ptr.To(infrav1.CommonRole),
					// Additional tags managed separately
				}),
			},
		}
	}

	managedCluster.Spec.AzureName = s.Name
	managedCluster.Spec.Owner = &genruntime.KnownResourceReference{
		Name: s.ResourceGroup,
	}

	if err = s.buildIdentity(managedCluster); err != nil {
		return nil, errors.Wrapf(err, "Identity is not valid: %s", err)
	}
	s.buildIdentityProfile(managedCluster)

	managedCluster.Spec.Location = &s.Location
	managedCluster.Spec.NodeResourceGroup = &s.NodeResourceGroup
	managedCluster.Spec.DnsPrefix = s.DNSPrefix
	managedCluster.Spec.EnableRBAC = ptr.To(true)
	managedCluster.Spec.ServicePrincipalProfile = &asocontainerservicev1.ManagedClusterServicePrincipalProfile{
		ClientId: ptr.To("msi"),
	}
	s.buildManagedClusterVersion(managedCluster, existing)
	s.buildAutoScalerProfile(managedCluster)
	s.buildOperatorSpec(managedCluster)
	s.buildAddonProfiles(managedCluster)
	s.buildAPIServerAccessProfile(managedCluster)
	s.buildSKU(managedCluster)
	if err := s.buildNetworkProfile(managedCluster); err != nil {
		return nil, err
	}
	if err := s.buildLinuxSSHProfile(managedCluster); err != nil {
		return nil, err
	}
	s.buildAADProfile(managedCluster)
	s.buildHTTPProxyConfig(managedCluster)
	s.buildSecurityProfile(managedCluster)

	if s.OIDCIssuerProfile != nil {
		managedCluster.Spec.OidcIssuerProfile = &asocontainerservicev1.ManagedClusterOIDCIssuerProfile{
			Enabled: s.OIDCIssuerProfile.Enabled,
		}
	}

	if s.AutoUpgradeProfile != nil {
		managedCluster.Spec.AutoUpgradeProfile = &asocontainerservicev1.ManagedClusterAutoUpgradeProfile{
			UpgradeChannel: (*asocontainerservicev1.ManagedClusterAutoUpgradeProfile_UpgradeChannel)(s.AutoUpgradeProfile.UpgradeChannel),
		}
	}

	// Only include AgentPoolProfiles during initial cluster creation. Agent pools are managed solely by the
	// AzureManagedMachinePool controller thereafter.
	var prevAgentPoolProfiles []asocontainerservicev1preview.ManagedClusterAgentPoolProfile
	managedCluster.Spec.AgentPoolProfiles = nil
	if managedCluster.Status.AgentPoolProfiles == nil {
		// Add all agent pools to cluster spec that will be submitted to the API
		agentPoolSpecs, err := s.GetAllAgentPools()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get agent pool specs for managed cluster %s", s.Name)
		}

		scheme := runtime.NewScheme()
		if err := asocontainerservicev1.AddToScheme(scheme); err != nil {
			return nil, errors.Wrap(err, "error constructing scheme")
		}
		if err := asocontainerservicev1preview.AddToScheme(scheme); err != nil {
			return nil, errors.Wrap(err, "error constructing scheme")
		}
		for _, agentPoolSpec := range agentPoolSpecs {
			agentPool, err := aso.PatchedParameters(ctx, scheme, agentPoolSpec, nil)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get agent pool parameters for managed cluster %s", s.Name)
			}
			agentPoolSpecTyped := agentPoolSpec.(*agentpools.AgentPoolSpec)
			if s.Preview {
				agentPoolTyped := agentPool.(*asocontainerservicev1preview.ManagedClustersAgentPool)
				agentPoolTyped.Spec.AzureName = agentPoolSpecTyped.AzureName
				profile := converters.AgentPoolToManagedClusterAgentPoolPreviewProfile(agentPoolTyped)
				prevAgentPoolProfiles = append(prevAgentPoolProfiles, profile)
			} else {
				agentPoolTyped := agentPool.(*asocontainerservicev1.ManagedClustersAgentPool)
				agentPoolTyped.Spec.AzureName = agentPoolSpecTyped.AzureName
				profile := converters.AgentPoolToManagedClusterAgentPoolProfile(agentPoolTyped)
				managedCluster.Spec.AgentPoolProfiles = append(managedCluster.Spec.AgentPoolProfiles, profile)
			}
		}
	}

	if s.Preview {
		preview, err := convertV1ToPreview(managedCluster)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			preview.Status = existingStatus
		}
		if prevAgentPoolProfiles != nil {
			preview.Spec.AgentPoolProfiles = prevAgentPoolProfiles
		}
		return preview, nil
	}

	return managedCluster, nil
}

// GetLoadBalancerProfile returns an asocontainerservicev1.ManagedClusterLoadBalancerProfile from the
// information present in ManagedClusterSpec.LoadBalancerProfile.
func (s *ManagedClusterSpec) GetLoadBalancerProfile() (loadBalancerProfile *asocontainerservicev1.ManagedClusterLoadBalancerProfile) {
	loadBalancerProfile = &asocontainerservicev1.ManagedClusterLoadBalancerProfile{
		AllocatedOutboundPorts: s.LoadBalancerProfile.AllocatedOutboundPorts,
		IdleTimeoutInMinutes:   s.LoadBalancerProfile.IdleTimeoutInMinutes,
	}
	if s.LoadBalancerProfile.ManagedOutboundIPs != nil {
		loadBalancerProfile.ManagedOutboundIPs = &asocontainerservicev1.ManagedClusterLoadBalancerProfile_ManagedOutboundIPs{Count: s.LoadBalancerProfile.ManagedOutboundIPs}
	}
	if len(s.LoadBalancerProfile.OutboundIPPrefixes) > 0 {
		loadBalancerProfile.OutboundIPPrefixes = &asocontainerservicev1.ManagedClusterLoadBalancerProfile_OutboundIPPrefixes{
			PublicIPPrefixes: convertToResourceReferences(s.LoadBalancerProfile.OutboundIPPrefixes),
		}
	}
	if len(s.LoadBalancerProfile.OutboundIPs) > 0 {
		loadBalancerProfile.OutboundIPs = &asocontainerservicev1.ManagedClusterLoadBalancerProfile_OutboundIPs{
			PublicIPs: convertToResourceReferences(s.LoadBalancerProfile.OutboundIPs),
		}
	}
	return
}

func convertToResourceReferences(resources []string) []asocontainerservicev1.ResourceReference {
	resourceReferences := make([]asocontainerservicev1.ResourceReference, len(resources))
	for i := range resources {
		resourceReferences[i] = asocontainerservicev1.ResourceReference{
			Reference: &genruntime.ResourceReference{
				ARMID: resources[i],
			},
		}
	}
	return resourceReferences
}

func (s *ManagedClusterSpec) buildIdentity(managedCluster *asocontainerservicev1.ManagedCluster) error {
	if s.Identity == nil || s.Identity.Type == "" {
		managedCluster.Spec.Identity = &asocontainerservicev1.ManagedClusterIdentity{
			Type: ptr.To(asocontainerservicev1.ManagedClusterIdentity_Type_SystemAssigned),
		}
		return nil
	}

	managedClusterIdentity := &asocontainerservicev1.ManagedClusterIdentity{
		Type: ptr.To(asocontainerservicev1.ManagedClusterIdentity_Type(s.Identity.Type)),
	}

	if ptr.Deref(managedClusterIdentity.Type, "") == asocontainerservicev1.ManagedClusterIdentity_Type_UserAssigned {
		if s.Identity.UserAssignedIdentityResourceID == "" {
			return errors.Errorf("Identity is set to \"UserAssigned\" but no UserAssignedIdentityResourceID is present")
		}

		managedClusterIdentity.UserAssignedIdentities = []asocontainerservicev1.UserAssignedIdentityDetails{
			{
				Reference: genruntime.ResourceReference{
					ARMID: s.Identity.UserAssignedIdentityResourceID,
				},
			},
		}
	}
	managedCluster.Spec.Identity = managedClusterIdentity
	return nil
}

func adminKubeconfigSecretName(clusterName string) string {
	return secret.Name(clusterName+"-aso", secret.Kubeconfig)
}

func oidcIssuerURLConfigMapName(clusterName string) string {
	return secret.Name(clusterName+"-aso", "oidc-issuer-profile")
}

func userKubeconfigSecretName(clusterName string) string {
	return secret.Name(clusterName+"-user-aso", secret.Kubeconfig)
}

// WasManaged implements azure.ASOResourceSpecGetter.
func (s *ManagedClusterSpec) WasManaged(resource genruntime.MetaObject) bool {
	// CAPZ has never supported BYO managed clusters.
	return true
}

var _ aso.TagsGetterSetter[genruntime.MetaObject] = (*ManagedClusterSpec)(nil)

// GetAdditionalTags implements aso.TagsGetterSetter.
func (s *ManagedClusterSpec) GetAdditionalTags() infrav1.Tags {
	return s.Tags
}

// GetDesiredTags implements aso.TagsGetterSetter.
func (s *ManagedClusterSpec) GetDesiredTags(resource genruntime.MetaObject) infrav1.Tags {
	if s.Preview {
		return resource.(*asocontainerservicev1preview.ManagedCluster).Spec.Tags
	}
	return resource.(*asocontainerservicev1.ManagedCluster).Spec.Tags
}

// SetTags implements aso.TagsGetterSetter.
func (s *ManagedClusterSpec) SetTags(resource genruntime.MetaObject, tags infrav1.Tags) {
	if s.Preview {
		resource.(*asocontainerservicev1preview.ManagedCluster).Spec.Tags = tags
		return
	}
	resource.(*asocontainerservicev1.ManagedCluster).Spec.Tags = tags
}

var _ aso.Patcher = (*ManagedClusterSpec)(nil)

// ExtraPatches implements aso.Patcher.
func (s *ManagedClusterSpec) ExtraPatches() []string {
	return s.Patches
}

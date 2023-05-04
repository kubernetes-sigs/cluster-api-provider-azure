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

package managedclusters

import (
	"context"
	"fmt"
	"net"
	"time"

	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/feature"
	"sigs.k8s.io/cluster-api-provider-azure/util/kubelogin"

	"github.com/Azure/azure-sdk-for-go/services/preview/containerservice/mgmt/2022-03-02-preview/containerservice"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	infrav1alpha4 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/util/maps"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const serviceName = "managedclusters"

var (
	defaultUser     = "azureuser"
	managedIdentity = "msi"
)

// ManagedClusterScope defines the scope interface for a managed cluster.
type ManagedClusterScope interface {
	azure.ClusterDescriber
	ManagedClusterAnnotations() map[string]string
	ManagedClusterSpec() (azure.ManagedClusterSpec, error)
	GetAllAgentPoolSpecs(ctx context.Context) ([]azure.AgentPoolSpec, error)
	SetControlPlaneEndpoint(clusterv1.APIEndpoint)
	MakeEmptyKubeConfigSecret() corev1.Secret
	GetKubeConfigData() []byte
	SetKubeConfigData([]byte)
	GetManagedControlPlaneCredentialsProvider() *scope.ManagedControlPlaneCredentialsProvider
}

// Service provides operations on azure resources.
type Service struct {
	Scope ManagedClusterScope
	Client
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
		KubernetesVersion:    managedCluster.ManagedClusterProperties.KubernetesVersion,
		NetworkProfile:       &containerservice.NetworkProfile{},
		DisableLocalAccounts: managedCluster.ManagedClusterProperties.DisableLocalAccounts,
	}

	existingMCPropertiesNormalized := &containerservice.ManagedClusterProperties{
		KubernetesVersion:    existingMC.ManagedClusterProperties.KubernetesVersion,
		NetworkProfile:       &containerservice.NetworkProfile{},
		DisableLocalAccounts: existingMC.ManagedClusterProperties.DisableLocalAccounts,
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

	// TODO: Enable this after we start specifying addon profiles through DKC controller.
	//if managedCluster.AddonProfiles != nil {
	//	for k, v := range managedCluster.AddonProfiles {
	//		if propertiesNormalized.AddonProfiles == nil {
	//			propertiesNormalized.AddonProfiles = map[string]*containerservice.ManagedClusterAddonProfile{}
	//		}
	//		propertiesNormalized.AddonProfiles[k] = &containerservice.ManagedClusterAddonProfile{
	//			Enabled: v.Enabled,
	//			Config:  v.Config,
	//		}
	//	}
	//}
	//
	//if existingMC.AddonProfiles != nil {
	//	for k, v := range existingMC.AddonProfiles {
	//		// If existing addon profile is disabled and the desired addon profile is nil or doesn't specify, skip it.
	//		if !*v.Enabled && (propertiesNormalized.AddonProfiles == nil || propertiesNormalized.AddonProfiles[k] == nil) {
	//			continue
	//		}
	//		if existingMCPropertiesNormalized.AddonProfiles == nil {
	//			existingMCPropertiesNormalized.AddonProfiles = map[string]*containerservice.ManagedClusterAddonProfile{}
	//		}
	//		existingMCPropertiesNormalized.AddonProfiles[k] = &containerservice.ManagedClusterAddonProfile{
	//			Enabled: v.Enabled,
	//			Config:  v.Config,
	//		}
	//	}
	//}

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

	diff := cmp.Diff(existingMCClusterNormalized, clusterNormalized)
	return diff
}

// New creates a new service.
func New(scope ManagedClusterScope) *Service {
	return &Service{
		Scope:  scope,
		Client: NewClient(scope),
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return serviceName
}

// Reconcile idempotently creates or updates a managed cluster, if possible.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "managedclusters.Service.Reconcile")
	defer done()

	managedClusterSpec, err := s.Scope.ManagedClusterSpec()
	if err != nil {
		return errors.Wrap(err, "failed to get managed cluster spec")
	}

	isCreate := false
	existingMC, err := s.Client.Get(ctx, managedClusterSpec.ResourceGroupName, managedClusterSpec.Name)
	// Transient or other failure not due to 404
	if err != nil && !azure.ResourceNotFound(err) {
		return azure.WithTransientError(errors.Wrap(err, "failed to fetch existing managed cluster"), 20*time.Second)
	}

	// We are creating this cluster for the first time.
	// Configure the agent pool, rest will be handled by machinepool controller
	// We do this here because AKS will only let us mutate agent pools via managed
	// clusters API at create time, not update.
	if azure.ResourceNotFound(err) {
		isCreate = true
		// Add system agent pool to cluster spec that will be submitted to the API
		managedClusterSpec.AgentPools, err = s.Scope.GetAllAgentPoolSpecs(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to get system agent pool specs for managed cluster %s", s.Scope.ClusterName())
		}
	}

	managedCluster := containerservice.ManagedCluster{
		Identity: &containerservice.ManagedClusterIdentity{
			Type: containerservice.ResourceIdentityTypeSystemAssigned,
		},
		Location: &managedClusterSpec.Location,
		Tags:     *to.StringMapPtr(managedClusterSpec.Tags),
		ManagedClusterProperties: &containerservice.ManagedClusterProperties{
			NodeResourceGroup: &managedClusterSpec.NodeResourceGroupName,
			EnableRBAC:        to.BoolPtr(true),
			DNSPrefix:         &managedClusterSpec.Name,
			KubernetesVersion: &managedClusterSpec.Version,
			ServicePrincipalProfile: &containerservice.ManagedClusterServicePrincipalProfile{
				ClientID: &managedIdentity,
			},
			AgentPoolProfiles: &[]containerservice.ManagedClusterAgentPoolProfile{},
			NetworkProfile: &containerservice.NetworkProfile{
				NetworkPlugin:   containerservice.NetworkPlugin(managedClusterSpec.NetworkPlugin),
				LoadBalancerSku: containerservice.LoadBalancerSku(managedClusterSpec.LoadBalancerSKU),
				NetworkPolicy:   containerservice.NetworkPolicy(managedClusterSpec.NetworkPolicy),
			},
			DisableLocalAccounts: managedClusterSpec.DisableLocalAccounts,
		},
	}

	if managedClusterSpec.IPFamilies != nil {
		var ipFamilies []containerservice.IPFamily
		for _, ipf := range *managedClusterSpec.IPFamilies {
			ipFamilies = append(ipFamilies, containerservice.IPFamily(ipf))
		}
		managedCluster.NetworkProfile.IPFamilies = &ipFamilies
	}

	if managedClusterSpec.PodCIDR != "" {
		managedCluster.NetworkProfile.PodCidr = &managedClusterSpec.PodCIDR
	}

	if managedClusterSpec.ServiceCIDR != "" {
		if managedClusterSpec.DNSServiceIP == nil {
			managedCluster.NetworkProfile.ServiceCidr = &managedClusterSpec.ServiceCIDR
			ip, _, err := net.ParseCIDR(managedClusterSpec.ServiceCIDR)
			if err != nil {
				return fmt.Errorf("failed to parse service cidr: %w", err)
			}
			// HACK: set the last octet of the IP to .10
			// This ensures the dns IP is valid in the service cidr without forcing the user
			// to specify it in both the Capi cluster and the Azure control plane.
			// https://golang.org/src/net/ip.go#L48
			ip[15] = byte(10)
			dnsIP := ip.String()
			managedCluster.NetworkProfile.DNSServiceIP = &dnsIP
		} else {
			managedCluster.NetworkProfile.DNSServiceIP = managedClusterSpec.DNSServiceIP
		}
	}

	for i := range managedClusterSpec.AgentPools {
		pool := managedClusterSpec.AgentPools[i]
		profile := converters.AgentPoolToManagedClusterAgentPoolProfile(pool)

		if pool.KubeletConfig != nil {
			profile.KubeletConfig = (*containerservice.KubeletConfig)(pool.KubeletConfig)
		}

		profile.Tags = pool.AdditionalTags

		*managedCluster.AgentPoolProfiles = append(*managedCluster.AgentPoolProfiles, profile)
	}

	if managedClusterSpec.AADProfile != nil {
		managedCluster.AadProfile = &containerservice.ManagedClusterAADProfile{
			Managed:             &managedClusterSpec.AADProfile.Managed,
			EnableAzureRBAC:     &managedClusterSpec.AADProfile.EnableAzureRBAC,
			AdminGroupObjectIDs: &managedClusterSpec.AADProfile.AdminGroupObjectIDs,
		}
	}

	handleAddonProfiles(managedCluster, managedClusterSpec)

	if managedClusterSpec.SKU != nil {
		tierName := containerservice.ManagedClusterSKUTier(managedClusterSpec.SKU.Tier)
		managedCluster.Sku = &containerservice.ManagedClusterSKU{
			Name: containerservice.ManagedClusterSKUNameBasic,
			Tier: tierName,
		}
	}

	if managedClusterSpec.SSHPublicKey != nil {
		managedCluster.LinuxProfile = &containerservice.LinuxProfile{
			AdminUsername: &defaultUser,
			SSH: &containerservice.SSHConfiguration{
				PublicKeys: &[]containerservice.SSHPublicKey{
					{
						KeyData: managedClusterSpec.SSHPublicKey,
					},
				},
			},
		}
	}

	if managedClusterSpec.LoadBalancerProfile != nil {
		managedCluster.NetworkProfile.LoadBalancerProfile = &containerservice.ManagedClusterLoadBalancerProfile{
			AllocatedOutboundPorts: managedClusterSpec.LoadBalancerProfile.AllocatedOutboundPorts,
			IdleTimeoutInMinutes:   managedClusterSpec.LoadBalancerProfile.IdleTimeoutInMinutes,
		}
		if managedClusterSpec.LoadBalancerProfile.ManagedOutboundIPs != nil {
			managedCluster.NetworkProfile.LoadBalancerProfile.ManagedOutboundIPs = &containerservice.ManagedClusterLoadBalancerProfileManagedOutboundIPs{Count: managedClusterSpec.LoadBalancerProfile.ManagedOutboundIPs}
		}
		if len(managedClusterSpec.LoadBalancerProfile.OutboundIPPrefixes) > 0 {
			managedCluster.NetworkProfile.LoadBalancerProfile.OutboundIPPrefixes = &containerservice.ManagedClusterLoadBalancerProfileOutboundIPPrefixes{
				PublicIPPrefixes: convertToResourceReferences(managedClusterSpec.LoadBalancerProfile.OutboundIPPrefixes),
			}
		}
		if len(managedClusterSpec.LoadBalancerProfile.OutboundIPs) > 0 {
			managedCluster.NetworkProfile.LoadBalancerProfile.OutboundIPs = &containerservice.ManagedClusterLoadBalancerProfileOutboundIPs{
				PublicIPs: convertToResourceReferences(managedClusterSpec.LoadBalancerProfile.OutboundIPs),
			}
		}
	}

	if managedClusterSpec.APIServerAccessProfile != nil {
		managedCluster.APIServerAccessProfile = &containerservice.ManagedClusterAPIServerAccessProfile{
			AuthorizedIPRanges:             &managedClusterSpec.APIServerAccessProfile.AuthorizedIPRanges,
			EnablePrivateCluster:           managedClusterSpec.APIServerAccessProfile.EnablePrivateCluster,
			PrivateDNSZone:                 managedClusterSpec.APIServerAccessProfile.PrivateDNSZone,
			EnablePrivateClusterPublicFQDN: managedClusterSpec.APIServerAccessProfile.EnablePrivateClusterPublicFQDN,
		}
	}

	customHeaders := maps.FilterByKeyPrefix(s.Scope.ManagedClusterAnnotations(), azure.CustomHeaderPrefix)
	// Use the MC fetched from Azure if no update is needed. This is to ensure the read-only fields like Fqdn from the
	// existing MC are used for updating the AzureManagedCluster.
	result := existingMC
	if isCreate {
		result, err = s.Client.CreateOrUpdate(ctx, managedClusterSpec.ResourceGroupName, managedClusterSpec.Name, managedCluster, customHeaders)
		if err != nil {
			return fmt.Errorf("failed to create managed cluster, %w", err)
		}
	} else {
		ps := *existingMC.ManagedClusterProperties.ProvisioningState
		if ps != string(infrav1alpha4.Canceled) && ps != string(infrav1alpha4.Failed) && ps != string(infrav1alpha4.Succeeded) {
			msg := fmt.Sprintf("Unable to update existing managed cluster in non terminal state. Managed cluster must be in one of the following provisioning states: canceled, failed, or succeeded. Actual state: %s", ps)
			klog.V(2).Infof(msg)
			return azure.WithTransientError(errors.New(msg), 20*time.Second)
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

		diff := computeDiffOfNormalizedClusters(managedCluster, existingMC)
		if diff != "" {
			klog.V(2).Infof("Cluster %s: update required (+new -old):\n%s", s.Scope.ClusterName(), diff)
			result, err = s.Client.CreateOrUpdate(ctx, managedClusterSpec.ResourceGroupName, managedClusterSpec.Name, managedCluster, customHeaders)
			if err != nil {
				return fmt.Errorf("failed to update managed cluster, %w", err)
			}
		}
	}

	// Update control plane endpoint.
	if result.ManagedClusterProperties != nil && result.ManagedClusterProperties.Fqdn != nil {
		endpoint := clusterv1.APIEndpoint{
			Host: *result.ManagedClusterProperties.Fqdn,
			Port: 443,
		}
		s.Scope.SetControlPlaneEndpoint(endpoint)
	} else {
		// Fail if cluster api endpoint is not available.
		return fmt.Errorf("failed to get API endpoint for managed cluster")
	}

	// Update kubeconfig data
	// Always fetch credentials in case of rotation
	kubeConfigData, err := s.Client.GetCredentials(ctx, s.Scope.ResourceGroup(), s.Scope.ClusterName())
	if err != nil {
		return errors.Wrap(err, "failed to get credentials for managed cluster")
	}
	klog.V(2).Infof("Successfully fetched kubeconfig data for managed cluster %s", s.Scope.ClusterName())
	// Covert kubelogin data to non-interactive format for use with other controllers.
	if feature.Gates.Enabled(feature.Kubelogin) {
		convertedKubeConfigData, err := kubelogin.ConvertKubeConfig(ctx, s.Scope.ClusterName(), kubeConfigData, s.Scope.GetManagedControlPlaneCredentialsProvider())
		if err != nil {
			return errors.Wrap(err, "failed to convert kubeconfig to non-interactive format")
		}
		klog.V(2).Infof("Successfully converted kubeconfig to non-interactive format for managed cluster %s", s.Scope.ClusterName())
		s.Scope.SetKubeConfigData(convertedKubeConfigData)
	} else {
		s.Scope.SetKubeConfigData(kubeConfigData)
	}

	return nil
}

func handleAddonProfiles(managedCluster containerservice.ManagedCluster, spec azure.ManagedClusterSpec) {
	for i := range spec.AddonProfiles {
		if managedCluster.AddonProfiles == nil {
			managedCluster.AddonProfiles = map[string]*containerservice.ManagedClusterAddonProfile{}
		}
		item := spec.AddonProfiles[i]
		addonProfile := &containerservice.ManagedClusterAddonProfile{
			Enabled: &item.Enabled,
		}
		if item.Config != nil {
			addonProfile.Config = *to.StringMapPtr(item.Config)
		}
		managedCluster.AddonProfiles[item.Name] = addonProfile
	}
}

// Delete deletes the managed cluster.
func (s *Service) Delete(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "managedclusters.Service.Delete")
	defer done()

	klog.V(2).Infof("Deleting managed cluster  %s ", s.Scope.ClusterName())
	err := s.Client.Delete(ctx, s.Scope.ResourceGroup(), s.Scope.ClusterName())
	if err != nil {
		if azure.ResourceNotFound(err) {
			// already deleted
			return nil
		}
		return errors.Wrapf(err, "failed to delete managed cluster %s in resource group %s", s.Scope.ClusterName(), s.Scope.ResourceGroup())
	}

	klog.V(2).Infof("successfully deleted managed cluster %s ", s.Scope.ClusterName())
	return nil
}

// IsManaged returns always returns true as CAPZ does not support BYO managed cluster.
func (s *Service) IsManaged(ctx context.Context) (bool, error) {
	return true, nil
}

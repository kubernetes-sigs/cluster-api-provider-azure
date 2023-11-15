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
	"testing"

	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20230201"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/agentpools"
	"sigs.k8s.io/cluster-api/util/secret"
)

func TestParameters(t *testing.T) {
	t.Run("no existing managed cluster", func(t *testing.T) {
		g := NewGomegaWithT(t)

		spec := &ManagedClusterSpec{
			Name:              "name",
			Namespace:         "namespace",
			ResourceGroup:     "rg",
			NodeResourceGroup: "node rg",
			ClusterName:       "cluster",
			VnetSubnetID:      "vnet subnet id",
			Location:          "location",
			Tags:              map[string]string{"additional": "tags"},
			Version:           "version",
			LoadBalancerSKU:   "lb sku",
			NetworkPlugin:     "network plugin",
			NetworkPluginMode: ptr.To(infrav1.NetworkPluginMode("network plugin mode")),
			NetworkPolicy:     "network policy",
			OutboundType:      ptr.To(infrav1.ManagedControlPlaneOutboundType("outbound type")),
			SSHPublicKey:      base64.StdEncoding.EncodeToString([]byte("ssh")),
			GetAllAgentPools: func() ([]azure.ASOResourceSpecGetter[*asocontainerservicev1.ManagedClustersAgentPool], error) {
				return []azure.ASOResourceSpecGetter[*asocontainerservicev1.ManagedClustersAgentPool]{
					&agentpools.AgentPoolSpec{
						Replicas:  5,
						Mode:      "mode",
						AzureName: "agentpool",
					},
				}, nil
			},
			PodCIDR:      "pod cidr",
			ServiceCIDR:  "0.0.0.0/10",
			DNSServiceIP: nil,
			AddonProfiles: []AddonProfile{
				{
					Name:    "addon name",
					Enabled: true,
					Config:  map[string]string{"addon": "config"},
				},
			},
			AADProfile: &AADProfile{
				Managed: true,
			},
			SKU: &SKU{
				Tier: "sku tier",
			},
			LoadBalancerProfile: &LoadBalancerProfile{
				ManagedOutboundIPs: ptr.To(16),
				OutboundIPPrefixes: []string{"outbound ip prefixes"},
				OutboundIPs:        []string{"outbound ips"},
			},
			APIServerAccessProfile: &APIServerAccessProfile{
				AuthorizedIPRanges: []string{"authorized ip ranges"},
			},
			AutoScalerProfile: &AutoScalerProfile{
				Expander: ptr.To("expander"),
			},
			Identity: &infrav1.Identity{
				Type:                           infrav1.ManagedControlPlaneIdentityType(asocontainerservicev1.ManagedClusterIdentity_Type_UserAssigned),
				UserAssignedIdentityResourceID: "user assigned id id",
			},
			KubeletUserAssignedIdentity: "kubelet id",
			HTTPProxyConfig: &HTTPProxyConfig{
				NoProxy: []string{"noproxy"},
			},
			OIDCIssuerProfile: &OIDCIssuerProfile{
				Enabled: ptr.To(true),
			},
			DNSPrefix:            ptr.To("dns prefix"),
			DisableLocalAccounts: ptr.To(true),
		}

		expected := &asocontainerservicev1.ManagedCluster{
			Spec: asocontainerservicev1.ManagedCluster_Spec{
				AadProfile: &asocontainerservicev1.ManagedClusterAADProfile{
					EnableAzureRBAC: ptr.To(false),
					Managed:         ptr.To(true),
				},
				AddonProfiles: map[string]asocontainerservicev1.ManagedClusterAddonProfile{
					"addon name": {
						Config:  map[string]string{"addon": "config"},
						Enabled: ptr.To(true),
					},
				},
				AgentPoolProfiles: []asocontainerservicev1.ManagedClusterAgentPoolProfile{
					{
						Count:             ptr.To(5),
						EnableAutoScaling: ptr.To(false),
						Mode:              ptr.To(asocontainerservicev1.AgentPoolMode("mode")),
						Name:              ptr.To("agentpool"),
						OsDiskSizeGB:      ptr.To(asocontainerservicev1.ContainerServiceOSDisk(0)),
						Type:              ptr.To(asocontainerservicev1.AgentPoolType_VirtualMachineScaleSets),
					},
				},
				ApiServerAccessProfile: &asocontainerservicev1.ManagedClusterAPIServerAccessProfile{
					AuthorizedIPRanges: []string{"authorized ip ranges"},
				},
				AutoScalerProfile: &asocontainerservicev1.ManagedClusterProperties_AutoScalerProfile{
					Expander: ptr.To(asocontainerservicev1.ManagedClusterProperties_AutoScalerProfile_Expander("expander")),
				},
				AzureName:            "name",
				DisableLocalAccounts: ptr.To(true),
				DnsPrefix:            ptr.To("dns prefix"),
				EnableRBAC:           ptr.To(true),
				HttpProxyConfig: &asocontainerservicev1.ManagedClusterHTTPProxyConfig{
					NoProxy: []string{"noproxy"},
				},
				Identity: &asocontainerservicev1.ManagedClusterIdentity{
					Type: ptr.To(asocontainerservicev1.ManagedClusterIdentity_Type_UserAssigned),
					UserAssignedIdentities: []asocontainerservicev1.UserAssignedIdentityDetails{
						{
							Reference: genruntime.ResourceReference{
								ARMID: "user assigned id id",
							},
						},
					},
				},
				IdentityProfile: map[string]asocontainerservicev1.UserAssignedIdentity{
					kubeletIdentityKey: {
						ResourceReference: &genruntime.ResourceReference{
							ARMID: "kubelet id",
						},
					},
				},
				KubernetesVersion: ptr.To("version"),
				LinuxProfile: &asocontainerservicev1.ContainerServiceLinuxProfile{
					AdminUsername: ptr.To(azure.DefaultAKSUserName),
					Ssh: &asocontainerservicev1.ContainerServiceSshConfiguration{
						PublicKeys: []asocontainerservicev1.ContainerServiceSshPublicKey{
							{
								KeyData: ptr.To("ssh"),
							},
						},
					},
				},
				Location: ptr.To("location"),
				NetworkProfile: &asocontainerservicev1.ContainerServiceNetworkProfile{
					DnsServiceIP: ptr.To("0.0.0.10"),
					LoadBalancerProfile: &asocontainerservicev1.ManagedClusterLoadBalancerProfile{
						ManagedOutboundIPs: &asocontainerservicev1.ManagedClusterLoadBalancerProfile_ManagedOutboundIPs{
							Count: ptr.To(16),
						},
						OutboundIPPrefixes: &asocontainerservicev1.ManagedClusterLoadBalancerProfile_OutboundIPPrefixes{
							PublicIPPrefixes: []asocontainerservicev1.ResourceReference{
								{
									Reference: &genruntime.ResourceReference{
										ARMID: "outbound ip prefixes",
									},
								},
							},
						},
						OutboundIPs: &asocontainerservicev1.ManagedClusterLoadBalancerProfile_OutboundIPs{
							PublicIPs: []asocontainerservicev1.ResourceReference{
								{
									Reference: &genruntime.ResourceReference{
										ARMID: "outbound ips",
									},
								},
							},
						},
					},
					LoadBalancerSku:   ptr.To(asocontainerservicev1.ContainerServiceNetworkProfile_LoadBalancerSku("lb sku")),
					NetworkPlugin:     ptr.To(asocontainerservicev1.ContainerServiceNetworkProfile_NetworkPlugin("network plugin")),
					NetworkPluginMode: ptr.To(asocontainerservicev1.ContainerServiceNetworkProfile_NetworkPluginMode("network plugin mode")),
					NetworkPolicy:     ptr.To(asocontainerservicev1.ContainerServiceNetworkProfile_NetworkPolicy("network policy")),
					OutboundType:      ptr.To(asocontainerservicev1.ContainerServiceNetworkProfile_OutboundType("outbound type")),
					PodCidr:           ptr.To("pod cidr"),
					ServiceCidr:       ptr.To("0.0.0.0/10"),
				},
				NodeResourceGroup: ptr.To("node rg"),
				OidcIssuerProfile: &asocontainerservicev1.ManagedClusterOIDCIssuerProfile{
					Enabled: ptr.To(true),
				},
				OperatorSpec: &asocontainerservicev1.ManagedClusterOperatorSpec{
					Secrets: &asocontainerservicev1.ManagedClusterOperatorSecrets{
						UserCredentials: &genruntime.SecretDestination{
							Name: "cluster-user-aso-kubeconfig",
							Key:  secret.KubeconfigDataName,
						},
					},
				},
				Owner: &genruntime.KnownResourceReference{
					Name: "rg",
				},
				ServicePrincipalProfile: &asocontainerservicev1.ManagedClusterServicePrincipalProfile{
					ClientId: ptr.To("msi"),
				},
				Sku: &asocontainerservicev1.ManagedClusterSKU{
					Name: ptr.To(asocontainerservicev1.ManagedClusterSKU_Name_Base),
					Tier: ptr.To(asocontainerservicev1.ManagedClusterSKU_Tier("sku tier")),
				},
				Tags: map[string]string{
					"Name": "name",
					"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster": "owned",
					"sigs.k8s.io_cluster-api-provider-azure_role":            "common",
				},
			},
		}

		actual, err := spec.Parameters(context.Background(), nil)

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(cmp.Diff(actual, expected)).To(BeEmpty())
	})

	t.Run("with existing managed cluster", func(t *testing.T) {
		g := NewGomegaWithT(t)

		spec := &ManagedClusterSpec{
			DNSPrefix: ptr.To("managed by CAPZ"),
			Tags:      map[string]string{"additional": "tags"},
		}
		existing := &asocontainerservicev1.ManagedCluster{
			Spec: asocontainerservicev1.ManagedCluster_Spec{
				DnsPrefix:               ptr.To("set by the user"),
				EnablePodSecurityPolicy: ptr.To(true), // set by the user
			},
			Status: asocontainerservicev1.ManagedCluster_STATUS{
				AgentPoolProfiles: []asocontainerservicev1.ManagedClusterAgentPoolProfile_STATUS{},
				Tags:              map[string]string{},
			},
		}

		actual, err := spec.Parameters(context.Background(), existing)

		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(actual.Spec.AgentPoolProfiles).To(BeNil())
		g.Expect(actual.Spec.Tags).To(BeNil())
		g.Expect(actual.Spec.DnsPrefix).To(Equal(ptr.To("managed by CAPZ")))
		g.Expect(actual.Spec.EnablePodSecurityPolicy).To(Equal(ptr.To(true)))
	})
}

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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/agentpools"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

func TestParameters(t *testing.T) {
	testcases := []struct {
		name          string
		spec          *ManagedClusterSpec
		existing      interface{}
		expectedError string
		expect        func(g *WithT, result interface{})
	}{
		{
			name: "managedcluster in non-terminal provisioning state",
			existing: armcontainerservice.ManagedCluster{
				Properties: &armcontainerservice.ManagedClusterProperties{
					ProvisioningState: ptr.To("Deleting"),
				},
			},
			spec: &ManagedClusterSpec{
				Name: "test-managedcluster",
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "Unable to update existing managed cluster in non-terminal state. Managed cluster must be in one of the following provisioning states: Canceled, Failed, or Succeeded. Actual state: Deleting. Object will be requeued after 20s",
		},
		{
			name:     "managedcluster does not exist",
			existing: nil,
			spec: &ManagedClusterSpec{
				Name:              "test-managedcluster",
				ResourceGroup:     "test-rg",
				NodeResourceGroup: "test-node-rg",
				ClusterName:       "test-cluster",
				Location:          "test-location",
				Tags: map[string]string{
					"test-tag": "test-value",
				},
				Version:           "v1.22.0",
				LoadBalancerSKU:   "standard",
				SSHPublicKey:      base64.StdEncoding.EncodeToString([]byte("test-ssh-key")),
				NetworkPluginMode: ptr.To(infrav1.NetworkPluginModeOverlay),
				OIDCIssuerProfile: &OIDCIssuerProfile{
					Enabled: ptr.To(true),
				},
				GetAllAgentPools: func() ([]azure.ResourceSpecGetter, error) {
					return []azure.ResourceSpecGetter{
						&agentpools.AgentPoolSpec{
							Name:          "test-agentpool-0",
							Mode:          string(infrav1.NodePoolModeSystem),
							ResourceGroup: "test-rg",
							Replicas:      int32(2),
							AdditionalTags: map[string]string{
								"test-tag": "test-value",
							},
						},
						&agentpools.AgentPoolSpec{
							Name:              "test-agentpool-1",
							Mode:              string(infrav1.NodePoolModeUser),
							ResourceGroup:     "test-rg",
							Replicas:          int32(4),
							Cluster:           "test-managedcluster",
							SKU:               "test_SKU",
							Version:           ptr.To("v1.22.0"),
							VnetSubnetID:      "fake/subnet/id",
							MaxPods:           ptr.To[int32](int32(32)),
							AvailabilityZones: []string{"1", "2"},
							AdditionalTags: map[string]string{
								"test-tag": "test-value",
							},
						},
					}, nil
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcontainerservice.ManagedCluster{}))
				sampleCluster := getSampleManagedCluster()
				g.Expect(gomockinternal.DiffEq(result).Matches(sampleCluster)).To(BeTrue(), cmp.Diff(result, getSampleManagedCluster()))
			},
		},
		{
			name:     "managedcluster does not exist without DNSServiceIP",
			existing: nil,
			spec: &ManagedClusterSpec{
				Name:              "test-managedcluster",
				ResourceGroup:     "test-rg",
				NodeResourceGroup: "test-node-rg",
				ClusterName:       "test-cluster",
				Location:          "test-location",
				Tags: map[string]string{
					"test-tag": "test-value",
				},
				Version:           "v1.22.0",
				LoadBalancerSKU:   "standard",
				SSHPublicKey:      base64.StdEncoding.EncodeToString([]byte("test-ssh-key")),
				NetworkPluginMode: ptr.To(infrav1.NetworkPluginModeOverlay),
				OIDCIssuerProfile: &OIDCIssuerProfile{
					Enabled: ptr.To(true),
				},
				ServiceCIDR: "192.168.200.6/30",
				GetAllAgentPools: func() ([]azure.ResourceSpecGetter, error) {
					return []azure.ResourceSpecGetter{
						&agentpools.AgentPoolSpec{
							Name:          "test-agentpool-0",
							Mode:          string(infrav1.NodePoolModeSystem),
							ResourceGroup: "test-rg",
							Replicas:      int32(2),
							AdditionalTags: map[string]string{
								"test-tag": "test-value",
							},
						},
						&agentpools.AgentPoolSpec{
							Name:              "test-agentpool-1",
							Mode:              string(infrav1.NodePoolModeUser),
							ResourceGroup:     "test-rg",
							Replicas:          int32(4),
							Cluster:           "test-managedcluster",
							SKU:               "test_SKU",
							Version:           ptr.To("v1.22.0"),
							VnetSubnetID:      "fake/subnet/id",
							MaxPods:           ptr.To[int32](int32(32)),
							AvailabilityZones: []string{"1", "2"},
							AdditionalTags: map[string]string{
								"test-tag": "test-value",
							},
						},
					}, nil
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcontainerservice.ManagedCluster{}))
				sampleCluster := getSampleManagedCluster()
				sampleCluster.Properties.NetworkProfile.ServiceCidr = ptr.To("192.168.200.6/30")
				sampleCluster.Properties.NetworkProfile.DNSServiceIP = ptr.To("192.168.200.10")
				g.Expect(gomockinternal.DiffEq(result).Matches(sampleCluster)).To(BeTrue(), cmp.Diff(result, getSampleManagedCluster()))
			},
		},
		{
			name:     "managedcluster does not exist with DNSServiceIP",
			existing: nil,
			spec: &ManagedClusterSpec{
				Name:              "test-managedcluster",
				ResourceGroup:     "test-rg",
				NodeResourceGroup: "test-node-rg",
				ClusterName:       "test-cluster",
				Location:          "test-location",
				Tags: map[string]string{
					"test-tag": "test-value",
				},
				Version:           "v1.22.0",
				LoadBalancerSKU:   "standard",
				SSHPublicKey:      base64.StdEncoding.EncodeToString([]byte("test-ssh-key")),
				NetworkPluginMode: ptr.To(infrav1.NetworkPluginModeOverlay),
				OIDCIssuerProfile: &OIDCIssuerProfile{
					Enabled: ptr.To(true),
				},
				ServiceCIDR:  "192.168.200.6/30",
				DNSServiceIP: ptr.To("192.168.200.6"),
				GetAllAgentPools: func() ([]azure.ResourceSpecGetter, error) {
					return []azure.ResourceSpecGetter{
						&agentpools.AgentPoolSpec{
							Name:          "test-agentpool-0",
							Mode:          string(infrav1.NodePoolModeSystem),
							ResourceGroup: "test-rg",
							Replicas:      int32(2),
							AdditionalTags: map[string]string{
								"test-tag": "test-value",
							},
						},
						&agentpools.AgentPoolSpec{
							Name:              "test-agentpool-1",
							Mode:              string(infrav1.NodePoolModeUser),
							ResourceGroup:     "test-rg",
							Replicas:          int32(4),
							Cluster:           "test-managedcluster",
							SKU:               "test_SKU",
							Version:           ptr.To("v1.22.0"),
							VnetSubnetID:      "fake/subnet/id",
							MaxPods:           ptr.To[int32](int32(32)),
							AvailabilityZones: []string{"1", "2"},
							AdditionalTags: map[string]string{
								"test-tag": "test-value",
							},
						},
					}, nil
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcontainerservice.ManagedCluster{}))
				sampleCluster := getSampleManagedCluster()
				sampleCluster.Properties.NetworkProfile.ServiceCidr = ptr.To("192.168.200.6/30")
				sampleCluster.Properties.NetworkProfile.DNSServiceIP = ptr.To("192.168.200.6")
				g.Expect(gomockinternal.DiffEq(result).Matches(sampleCluster)).To(BeTrue(), cmp.Diff(result, getSampleManagedCluster()))
			},
		},
		{
			name:     "managedcluster exists, no update needed",
			existing: getExistingCluster(),
			spec: &ManagedClusterSpec{
				Name:          "test-managedcluster",
				ResourceGroup: "test-rg",
				Location:      "test-location",
				Tags: map[string]string{
					"test-tag": "test-value",
				},
				Version:         "v1.22.0",
				LoadBalancerSKU: "standard",
				OIDCIssuerProfile: &OIDCIssuerProfile{
					Enabled: ptr.To(true),
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
		},
		{
			name:     "managedcluster exists and an update is needed",
			existing: getExistingCluster(),
			spec: &ManagedClusterSpec{
				Name:          "test-managedcluster",
				ResourceGroup: "test-rg",
				Location:      "test-location",
				Tags: map[string]string{
					"test-tag": "test-value",
				},
				Version:         "v1.22.99",
				LoadBalancerSKU: "standard",
				Identity: &infrav1.Identity{
					Type:                           infrav1.ManagedControlPlaneIdentityTypeUserAssigned,
					UserAssignedIdentityResourceID: "/resource/ID",
				},
				KubeletUserAssignedIdentity: "/resource/ID",
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcontainerservice.ManagedCluster{}))
				g.Expect(result.(armcontainerservice.ManagedCluster).Properties.KubernetesVersion).To(Equal(ptr.To("v1.22.99")))
				g.Expect(result.(armcontainerservice.ManagedCluster).Identity.Type).To(Equal(ptr.To(armcontainerservice.ResourceIdentityType("UserAssigned"))))
				g.Expect(result.(armcontainerservice.ManagedCluster).Identity.UserAssignedIdentities).To(Equal(map[string]*armcontainerservice.ManagedServiceIdentityUserAssignedIdentitiesValue{"/resource/ID": {}}))
				g.Expect(result.(armcontainerservice.ManagedCluster).Properties.IdentityProfile).To(Equal(map[string]*armcontainerservice.UserAssignedIdentity{kubeletIdentityKey: {ResourceID: ptr.To("/resource/ID")}}))
			},
		},
		{
			name:     "delete all tags",
			existing: getExistingCluster(),
			spec: &ManagedClusterSpec{
				Name:            "test-managedcluster",
				ResourceGroup:   "test-rg",
				Location:        "test-location",
				Tags:            nil,
				Version:         "v1.22.0",
				LoadBalancerSKU: "standard",
				OIDCIssuerProfile: &OIDCIssuerProfile{
					Enabled: ptr.To(true),
				},
			},
			expect: func(g *WithT, result interface{}) {
				// Additional tags are handled by azure/services/tags, so a diff
				// here shouldn't trigger an update on the managed cluster resource.
				g.Expect(result).To(BeNil())
			},
		},
		{
			name:     "set Linux profile if SSH key is set",
			existing: nil,
			spec: &ManagedClusterSpec{
				Name:            "test-managedcluster",
				ResourceGroup:   "test-rg",
				Location:        "test-location",
				Tags:            nil,
				Version:         "v1.22.0",
				LoadBalancerSKU: "standard",
				OIDCIssuerProfile: &OIDCIssuerProfile{
					Enabled: ptr.To(true),
				},
				SSHPublicKey: base64.StdEncoding.EncodeToString([]byte("test-ssh-key")),
				GetAllAgentPools: func() ([]azure.ResourceSpecGetter, error) {
					return []azure.ResourceSpecGetter{
						&agentpools.AgentPoolSpec{
							Name:          "test-agentpool-0",
							Mode:          string(infrav1.NodePoolModeSystem),
							ResourceGroup: "test-rg",
							Replicas:      int32(2),
							AdditionalTags: map[string]string{
								"test-tag": "test-value",
							},
						},
					}, nil
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcontainerservice.ManagedCluster{}))
				g.Expect(result.(armcontainerservice.ManagedCluster).Properties.LinuxProfile).To(Not(BeNil()))
				g.Expect(*(result.(armcontainerservice.ManagedCluster).Properties.LinuxProfile.SSH.PublicKeys)[0].KeyData).To(Equal("test-ssh-key"))
			},
		},
		{
			name:     "set HTTPProxyConfig if set",
			existing: nil,
			spec: &ManagedClusterSpec{
				Name:            "test-managedcluster",
				ResourceGroup:   "test-rg",
				Location:        "test-location",
				Tags:            nil,
				Version:         "v1.22.0",
				LoadBalancerSKU: "standard",
				OIDCIssuerProfile: &OIDCIssuerProfile{
					Enabled: ptr.To(true),
				},
				HTTPProxyConfig: &HTTPProxyConfig{
					HTTPProxy:  ptr.To("http://proxy.com"),
					HTTPSProxy: ptr.To("https://proxy.com"),
				},
				GetAllAgentPools: func() ([]azure.ResourceSpecGetter, error) {
					return []azure.ResourceSpecGetter{}, nil
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcontainerservice.ManagedCluster{}))
				g.Expect(result.(armcontainerservice.ManagedCluster).Properties.HTTPProxyConfig).To(Not(BeNil()))
				g.Expect((*result.(armcontainerservice.ManagedCluster).Properties.HTTPProxyConfig.HTTPProxy)).To(Equal("http://proxy.com"))
			},
		},
		{
			name:     "set HTTPProxyConfig if set with no proxy list",
			existing: nil,
			spec: &ManagedClusterSpec{
				Name:            "test-managedcluster",
				ResourceGroup:   "test-rg",
				Location:        "test-location",
				Tags:            nil,
				Version:         "v1.22.0",
				LoadBalancerSKU: "standard",
				OIDCIssuerProfile: &OIDCIssuerProfile{
					Enabled: ptr.To(true),
				},
				HTTPProxyConfig: &HTTPProxyConfig{
					NoProxy: []string{"noproxy1", "noproxy2"},
				},
				GetAllAgentPools: func() ([]azure.ResourceSpecGetter, error) {
					return []azure.ResourceSpecGetter{}, nil
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcontainerservice.ManagedCluster{}))
				g.Expect(result.(armcontainerservice.ManagedCluster).Properties.HTTPProxyConfig).To(Not(BeNil()))
				g.Expect((result.(armcontainerservice.ManagedCluster).Properties.HTTPProxyConfig.NoProxy)).To(Equal([]*string{ptr.To("noproxy1"), ptr.To("noproxy2")}))
			},
		},
		{
			name:     "skip Linux profile if SSH key is not set",
			existing: nil,
			spec: &ManagedClusterSpec{
				Name:            "test-managedcluster",
				ResourceGroup:   "test-rg",
				Location:        "test-location",
				Tags:            nil,
				Version:         "v1.22.0",
				LoadBalancerSKU: "standard",
				OIDCIssuerProfile: &OIDCIssuerProfile{
					Enabled: ptr.To(true),
				},
				SSHPublicKey: "",
				GetAllAgentPools: func() ([]azure.ResourceSpecGetter, error) {
					return []azure.ResourceSpecGetter{
						&agentpools.AgentPoolSpec{
							Name:          "test-agentpool-0",
							Mode:          string(infrav1.NodePoolModeSystem),
							ResourceGroup: "test-rg",
							Replicas:      int32(2),
							AdditionalTags: map[string]string{
								"test-tag": "test-value",
							},
						},
					}, nil
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcontainerservice.ManagedCluster{}))
				g.Expect(result.(armcontainerservice.ManagedCluster).Properties.LinuxProfile).To(BeNil())
			},
		},
		{
			name:     "no update needed if both clusters have no authorized IP ranges",
			existing: getExistingClusterWithAPIServerAccessProfile(),
			spec: &ManagedClusterSpec{
				Name:          "test-managedcluster",
				ResourceGroup: "test-rg",
				Location:      "test-location",
				Tags: map[string]string{
					"test-tag": "test-value",
				},
				Version:         "v1.22.0",
				LoadBalancerSKU: "standard",
				OIDCIssuerProfile: &OIDCIssuerProfile{
					Enabled: ptr.To(true),
				},
				APIServerAccessProfile: &APIServerAccessProfile{
					AuthorizedIPRanges: func() []string {
						var arr []string
						return arr
					}(),
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
		},
		{
			name:     "update authorized IP ranges with empty struct if spec does not have authorized IP ranges but existing cluster has authorized IP ranges",
			existing: getExistingClusterWithAuthorizedIPRanges(),
			spec: &ManagedClusterSpec{
				Name:          "test-managedcluster",
				ResourceGroup: "test-rg",
				Location:      "test-location",
				Tags: map[string]string{
					"test-tag": "test-value",
				},
				Version:         "v1.22.0",
				LoadBalancerSKU: "standard",
				OIDCIssuerProfile: &OIDCIssuerProfile{
					Enabled: ptr.To(true),
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcontainerservice.ManagedCluster{}))
				g.Expect(result.(armcontainerservice.ManagedCluster).Properties.APIServerAccessProfile).To(Not(BeNil()))
				g.Expect(result.(armcontainerservice.ManagedCluster).Properties.APIServerAccessProfile.AuthorizedIPRanges).To(Equal([]*string{}))
			},
		},
		{
			name:     "update authorized IP ranges with authorized IPs spec has authorized IP ranges but existing cluster does not have authorized IP ranges",
			existing: getExistingCluster(),
			spec: &ManagedClusterSpec{
				Name:          "test-managedcluster",
				ResourceGroup: "test-rg",
				Location:      "test-location",
				Tags: map[string]string{
					"test-tag": "test-value",
				},
				Version:         "v1.22.0",
				LoadBalancerSKU: "standard",
				OIDCIssuerProfile: &OIDCIssuerProfile{
					Enabled: ptr.To(true),
				},
				APIServerAccessProfile: &APIServerAccessProfile{
					AuthorizedIPRanges: []string{"192.168.0.1/32, 192.168.0.2/32, 192.168.0.3/32"},
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcontainerservice.ManagedCluster{}))
				g.Expect(result.(armcontainerservice.ManagedCluster).Properties.APIServerAccessProfile).To(Not(BeNil()))
				g.Expect(result.(armcontainerservice.ManagedCluster).Properties.APIServerAccessProfile.AuthorizedIPRanges).To(Equal([]*string{ptr.To("192.168.0.1/32, 192.168.0.2/32, 192.168.0.3/32")}))
			},
		},
		{
			name:     "no update needed when authorized IP ranges when both clusters have the same authorized IP ranges",
			existing: getExistingClusterWithAuthorizedIPRanges(),
			spec: &ManagedClusterSpec{
				Name:          "test-managedcluster",
				ResourceGroup: "test-rg",
				Location:      "test-location",
				Tags: map[string]string{
					"test-tag": "test-value",
				},
				Version:         "v1.22.0",
				LoadBalancerSKU: "standard",
				OIDCIssuerProfile: &OIDCIssuerProfile{
					Enabled: ptr.To(true),
				},
				APIServerAccessProfile: &APIServerAccessProfile{
					AuthorizedIPRanges: []string{"192.168.0.1/32, 192.168.0.2/32, 192.168.0.3/32"},
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
		},
		{
			name:     "managedcluster exists with UserAssigned identity, no update needed",
			existing: getExistingClusterWithUserAssignedIdentity(),
			spec: &ManagedClusterSpec{
				Name:          "test-managedcluster",
				ResourceGroup: "test-rg",
				Location:      "test-location",
				Tags: map[string]string{
					"test-tag": "test-value",
				},
				Version:         "v1.22.0",
				LoadBalancerSKU: "standard",
				OIDCIssuerProfile: &OIDCIssuerProfile{
					Enabled: ptr.To(true),
				},
				Identity: &infrav1.Identity{
					Type:                           infrav1.ManagedControlPlaneIdentityTypeUserAssigned,
					UserAssignedIdentityResourceID: "some id",
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
		},
		{
			name: "setting networkPluginMode from nil to \"overlay\" will update",
			existing: func() armcontainerservice.ManagedCluster {
				c := getExistingCluster()
				c.Properties.NetworkProfile.NetworkPluginMode = nil
				return c
			}(),
			spec: &ManagedClusterSpec{
				Name:          "test-managedcluster",
				ResourceGroup: "test-rg",
				Location:      "test-location",
				Tags: map[string]string{
					"test-tag": "test-value",
				},
				Version:         "v1.22.0",
				LoadBalancerSKU: "standard",
				OIDCIssuerProfile: &OIDCIssuerProfile{
					Enabled: ptr.To(true),
				},
				NetworkPluginMode: ptr.To(infrav1.NetworkPluginModeOverlay),
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcontainerservice.ManagedCluster{}))
				g.Expect(result.(armcontainerservice.ManagedCluster).Properties.NetworkProfile.NetworkPluginMode).NotTo(BeNil())
				g.Expect(*result.(armcontainerservice.ManagedCluster).Properties.NetworkProfile.NetworkPluginMode).To(Equal(armcontainerservice.NetworkPluginModeOverlay))
			},
		},
		{
			name:     "setting networkPluginMode from \"overlay\" to nil doesn't require update",
			existing: getExistingCluster(),
			spec: &ManagedClusterSpec{
				Name:          "test-managedcluster",
				ResourceGroup: "test-rg",
				Location:      "test-location",
				Tags: map[string]string{
					"test-tag": "test-value",
				},
				Version:         "v1.22.0",
				LoadBalancerSKU: "standard",
				OIDCIssuerProfile: &OIDCIssuerProfile{
					Enabled: ptr.To(true),
				},
				NetworkPluginMode: nil,
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
		},
		{
			name:     "update needed when oidc issuer profile enabled changes",
			existing: getExistingCluster(),
			spec: &ManagedClusterSpec{
				Name:          "test-managedcluster",
				ResourceGroup: "test-rg",
				Location:      "test-location",
				Tags: map[string]string{
					"test-tag": "test-value",
				},
				Version:         "v1.22.0",
				LoadBalancerSKU: "standard",
				OIDCIssuerProfile: &OIDCIssuerProfile{
					Enabled: ptr.To(false),
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcontainerservice.ManagedCluster{}))
				g.Expect(*result.(armcontainerservice.ManagedCluster).Properties.OidcIssuerProfile.Enabled).To(BeFalse())
			},
		},
		{
			name:     "do not update addon profile",
			existing: getExistingClusterWithAddonProfile(),
			spec: &ManagedClusterSpec{
				Name:          "test-managedcluster",
				ResourceGroup: "test-rg",
				Location:      "test-location",
				Tags: map[string]string{
					"test-tag": "test-value",
				},
				Version:         "v1.22.0",
				LoadBalancerSKU: "standard",
				OIDCIssuerProfile: &OIDCIssuerProfile{
					Enabled: ptr.To(true),
				},
				AddonProfiles: []AddonProfile{
					{
						Name:    "first-addon-profile",
						Enabled: true,
					},
					{
						Name:    "second-addon-profile",
						Enabled: true,
					},
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
		},
		{
			name:     "update needed when addon profile enabled changes",
			existing: getExistingClusterWithAddonProfile(),
			spec: &ManagedClusterSpec{
				Name:          "test-managedcluster",
				ResourceGroup: "test-rg",
				Location:      "test-location",
				Tags: map[string]string{
					"test-tag": "test-value",
				},
				Version:         "v1.22.0",
				LoadBalancerSKU: "standard",
				OIDCIssuerProfile: &OIDCIssuerProfile{
					Enabled: ptr.To(true),
				},
				AddonProfiles: []AddonProfile{
					{
						Name:    "first-addon-profile",
						Enabled: true,
					},
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcontainerservice.ManagedCluster{}))
				g.Expect(*result.(armcontainerservice.ManagedCluster).Properties.AddonProfiles["first-addon-profile"].Enabled).To(BeTrue())
				g.Expect(*result.(armcontainerservice.ManagedCluster).Properties.AddonProfiles["second-addon-profile"].Enabled).To(BeFalse())
			},
		},
		{
			name:     "update when we delete an addon profile",
			existing: getExistingClusterWithAddonProfile(),
			spec: &ManagedClusterSpec{
				Name:          "test-managedcluster",
				ResourceGroup: "test-rg",
				Location:      "test-location",
				Tags: map[string]string{
					"test-tag": "test-value",
				},
				Version:         "v1.22.0",
				LoadBalancerSKU: "standard",
				OIDCIssuerProfile: &OIDCIssuerProfile{
					Enabled: ptr.To(true),
				},
				AddonProfiles: []AddonProfile{
					{
						Name:    "first-addon-profile",
						Enabled: true,
					},
					{
						Name:    "second-addon-profile",
						Enabled: true,
					},
					{
						Name:    "third-addon-profile",
						Enabled: true,
					},
				},
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcontainerservice.ManagedCluster{}))
				g.Expect(*result.(armcontainerservice.ManagedCluster).Properties.AddonProfiles["third-addon-profile"].Enabled).To(BeTrue())
			},
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			format.MaxLength = 10000
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(context.TODO(), tc.existing)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			tc.expect(g, result)
		})
	}
}

func TestGetIdentity(t *testing.T) {
	testcases := []struct {
		name         string
		identity     *infrav1.Identity
		expectedType *armcontainerservice.ResourceIdentityType
	}{
		{
			name:     "default",
			identity: &infrav1.Identity{},
		},
		{
			name: "user-assigned identity",
			identity: &infrav1.Identity{
				Type:                           infrav1.ManagedControlPlaneIdentityTypeUserAssigned,
				UserAssignedIdentityResourceID: "/subscriptions/fae7cc14-bfba-4471-9435-f945b42a16dd/resourcegroups/my-identities/providers/Microsoft.ManagedIdentity/userAssignedIdentities/my-cluster-user-identity",
			},
			expectedType: ptr.To(armcontainerservice.ResourceIdentityTypeUserAssigned),
		},
		{
			name: "system-assigned identity",
			identity: &infrav1.Identity{
				Type: infrav1.ManagedControlPlaneIdentityTypeSystemAssigned,
			},
			expectedType: ptr.To(armcontainerservice.ResourceIdentityTypeSystemAssigned),
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := getIdentity(tc.identity)
			g.Expect(err).To(BeNil())
			if tc.identity.Type != "" {
				g.Expect(result.Type).To(Equal(tc.expectedType))
				if tc.identity.Type == infrav1.ManagedControlPlaneIdentityTypeUserAssigned {
					g.Expect(result.UserAssignedIdentities).To(Not(BeEmpty()))
					g.Expect(*result.UserAssignedIdentities[tc.identity.UserAssignedIdentityResourceID]).To(Equal(armcontainerservice.ManagedServiceIdentityUserAssignedIdentitiesValue{}))
				} else {
					g.Expect(result.UserAssignedIdentities).To(BeEmpty())
				}
			} else {
				g.Expect(result).To(BeNil())
			}
		})
	}
}

func getExistingClusterWithAPIServerAccessProfile() armcontainerservice.ManagedCluster {
	mc := getExistingCluster()
	mc.Properties.APIServerAccessProfile = &armcontainerservice.ManagedClusterAPIServerAccessProfile{
		EnablePrivateCluster: ptr.To(false),
	}
	return mc
}

func getExistingCluster() armcontainerservice.ManagedCluster {
	mc := getSampleManagedCluster()
	mc.Properties.ProvisioningState = ptr.To("Succeeded")
	mc.ID = ptr.To("test-id")
	return mc
}

func getExistingClusterWithAddonProfile() armcontainerservice.ManagedCluster {
	mc := getSampleManagedCluster()
	mc.Properties.ProvisioningState = ptr.To("Succeeded")
	mc.ID = ptr.To("test-id")
	mc.Properties.AddonProfiles = map[string]*armcontainerservice.ManagedClusterAddonProfile{
		"first-addon-profile": {
			Enabled: ptr.To(true),
		},
		"second-addon-profile": {
			Enabled: ptr.To(true),
		},
	}
	return mc
}

func getExistingClusterWithUserAssignedIdentity() armcontainerservice.ManagedCluster {
	mc := getSampleManagedCluster()
	mc.Properties.ProvisioningState = ptr.To("Succeeded")
	mc.ID = ptr.To("test-id")
	mc.Identity = &armcontainerservice.ManagedClusterIdentity{
		Type: ptr.To(armcontainerservice.ResourceIdentityTypeUserAssigned),
		UserAssignedIdentities: map[string]*armcontainerservice.ManagedServiceIdentityUserAssignedIdentitiesValue{
			"some id": {
				ClientID:    ptr.To("some client id"),
				PrincipalID: ptr.To("some principal id"),
			},
		},
	}
	return mc
}

func getSampleManagedCluster() armcontainerservice.ManagedCluster {
	return armcontainerservice.ManagedCluster{
		Properties: &armcontainerservice.ManagedClusterProperties{
			KubernetesVersion: ptr.To("v1.22.0"),
			AgentPoolProfiles: []*armcontainerservice.ManagedClusterAgentPoolProfile{
				{
					Name:         ptr.To("test-agentpool-0"),
					Mode:         ptr.To(armcontainerservice.AgentPoolMode(infrav1.NodePoolModeSystem)),
					Count:        ptr.To[int32](2),
					Type:         ptr.To(armcontainerservice.AgentPoolTypeVirtualMachineScaleSets),
					OSDiskSizeGB: ptr.To[int32](0),
					Tags: map[string]*string{
						"test-tag": ptr.To("test-value"),
					},
					EnableAutoScaling: ptr.To(false),
				},
				{
					Name:                ptr.To("test-agentpool-1"),
					Mode:                ptr.To(armcontainerservice.AgentPoolMode(infrav1.NodePoolModeUser)),
					Count:               ptr.To[int32](4),
					Type:                ptr.To(armcontainerservice.AgentPoolTypeVirtualMachineScaleSets),
					OSDiskSizeGB:        ptr.To[int32](0),
					VMSize:              ptr.To("test_SKU"),
					OrchestratorVersion: ptr.To("v1.22.0"),
					VnetSubnetID:        ptr.To("fake/subnet/id"),
					MaxPods:             ptr.To[int32](int32(32)),
					AvailabilityZones:   []*string{ptr.To("1"), ptr.To("2")},
					Tags: map[string]*string{
						"test-tag": ptr.To("test-value"),
					},
					EnableAutoScaling: ptr.To(false),
				},
			},
			LinuxProfile: &armcontainerservice.LinuxProfile{
				AdminUsername: ptr.To(azure.DefaultAKSUserName),
				SSH: &armcontainerservice.SSHConfiguration{
					PublicKeys: []*armcontainerservice.SSHPublicKey{
						{
							KeyData: ptr.To("test-ssh-key"),
						},
					},
				},
			},
			ServicePrincipalProfile: &armcontainerservice.ManagedClusterServicePrincipalProfile{ClientID: ptr.To("msi")},
			NodeResourceGroup:       ptr.To("test-node-rg"),
			EnableRBAC:              ptr.To(true),
			NetworkProfile: &armcontainerservice.NetworkProfile{
				LoadBalancerSKU:   ptr.To(armcontainerservice.LoadBalancerSKUStandard),
				NetworkPluginMode: ptr.To(armcontainerservice.NetworkPluginModeOverlay),
			},
			OidcIssuerProfile: &armcontainerservice.ManagedClusterOIDCIssuerProfile{
				Enabled: ptr.To(true),
			},
		},
		Identity: &armcontainerservice.ManagedClusterIdentity{
			Type: ptr.To(armcontainerservice.ResourceIdentityTypeSystemAssigned),
		},
		Location: ptr.To("test-location"),
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			ClusterName: "test-cluster",
			Name:        ptr.To("test-managedcluster"),
			Role:        ptr.To(infrav1.CommonRole),
			Additional: infrav1.Tags{
				"test-tag": "test-value",
			},
		})),
	}
}

func getExistingClusterWithAuthorizedIPRanges() armcontainerservice.ManagedCluster {
	mc := getExistingCluster()
	mc.Properties.APIServerAccessProfile = &armcontainerservice.ManagedClusterAPIServerAccessProfile{
		AuthorizedIPRanges: []*string{ptr.To("192.168.0.1/32, 192.168.0.2/32, 192.168.0.3/32")},
	}
	return mc
}

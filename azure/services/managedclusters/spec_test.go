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
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/agentpools"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	nodePoolModeUser                     = armcontainerservice.AgentPoolMode(infrav1.NodePoolModeUser)
	nodePoolModeSystem                   = armcontainerservice.AgentPoolMode(infrav1.NodePoolModeSystem)
	agentPoolTypeVirtualMachineScaleSets = armcontainerservice.AgentPoolTypeVirtualMachineScaleSets
	loadBalancerSKUStandard              = armcontainerservice.LoadBalancerSKUStandard
	resourceIdentityTypeSystemAssigned   = armcontainerservice.ResourceIdentityTypeSystemAssigned
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
					ProvisioningState: pointer.String("Deleting"),
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
				Version:         "v1.22.0",
				LoadBalancerSKU: "Standard",
				SSHPublicKey:    base64.StdEncoding.EncodeToString([]byte("test-ssh-key")),
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
							Version:           pointer.String("v1.22.0"),
							VnetSubnetID:      "fake/subnet/id",
							MaxPods:           pointer.Int32(int32(32)),
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
				g.Expect(gomockinternal.DiffEq(result).Matches(getSampleManagedCluster())).To(BeTrue(), cmp.Diff(result, getSampleManagedCluster()))
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
				LoadBalancerSKU: "Standard",
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
				LoadBalancerSKU: "Standard",
				Identity: &infrav1.Identity{
					Type:                           infrav1.ManagedControlPlaneIdentityTypeUserAssigned,
					UserAssignedIdentityResourceID: "/resource/ID",
				},
				KubeletUserAssignedIdentity: "/resource/ID",
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armcontainerservice.ManagedCluster{}))
				g.Expect(result.(armcontainerservice.ManagedCluster).Properties.KubernetesVersion).To(Equal(pointer.String("v1.22.99")))
				g.Expect(result.(armcontainerservice.ManagedCluster).Identity.Type).To(Equal(armcontainerservice.ResourceIdentityType("UserAssigned")))
				g.Expect(result.(armcontainerservice.ManagedCluster).Identity.UserAssignedIdentities).To(Equal(map[string]*armcontainerservice.ManagedServiceIdentityUserAssignedIdentitiesValue{"/resource/ID": {}}))
				g.Expect(result.(armcontainerservice.ManagedCluster).Properties.IdentityProfile).To(Equal(map[string]*armcontainerservice.UserAssignedIdentity{kubeletIdentityKey: {ResourceID: pointer.String("/resource/ID")}}))
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
				LoadBalancerSKU: "Standard",
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
				LoadBalancerSKU: "Standard",
				SSHPublicKey:    base64.StdEncoding.EncodeToString([]byte("test-ssh-key")),
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
			name:     "skip Linux profile if SSH key is not set",
			existing: nil,
			spec: &ManagedClusterSpec{
				Name:            "test-managedcluster",
				ResourceGroup:   "test-rg",
				Location:        "test-location",
				Tags:            nil,
				Version:         "v1.22.0",
				LoadBalancerSKU: "Standard",
				SSHPublicKey:    "",
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
				LoadBalancerSKU: "Standard",
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
		expectedType armcontainerservice.ResourceIdentityType
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
			expectedType: armcontainerservice.ResourceIdentityTypeUserAssigned,
		},
		{
			name: "system-assigned identity",
			identity: &infrav1.Identity{
				Type: infrav1.ManagedControlPlaneIdentityTypeSystemAssigned,
			},
			expectedType: armcontainerservice.ResourceIdentityTypeSystemAssigned,
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
		EnablePrivateCluster: pointer.Bool(false),
	}
	return mc
}

func getExistingCluster() armcontainerservice.ManagedCluster {
	mc := getSampleManagedCluster()
	mc.Properties.ProvisioningState = pointer.String("Succeeded")
	mc.ID = pointer.String("test-id")
	return mc
}

func getSampleManagedCluster() armcontainerservice.ManagedCluster {
	return armcontainerservice.ManagedCluster{
		Properties: &armcontainerservice.ManagedClusterProperties{
			KubernetesVersion: pointer.String("v1.22.0"),
			DNSPrefix:         pointer.String("test-managedcluster"),
			AgentPoolProfiles: []*armcontainerservice.ManagedClusterAgentPoolProfile{
				{
					Name:         pointer.String("test-agentpool-0"),
					Mode:         &nodePoolModeSystem,
					Count:        pointer.Int32(2),
					Type:         &agentPoolTypeVirtualMachineScaleSets,
					OSDiskSizeGB: pointer.Int32(0),
					Tags: map[string]*string{
						"test-tag": pointer.String("test-value"),
					},
					EnableAutoScaling: pointer.Bool(false),
				},
				{
					Name:                pointer.String("test-agentpool-1"),
					Mode:                &nodePoolModeUser,
					Count:               pointer.Int32(4),
					Type:                &agentPoolTypeVirtualMachineScaleSets,
					OSDiskSizeGB:        pointer.Int32(0),
					VMSize:              pointer.String("test_SKU"),
					OrchestratorVersion: pointer.String("v1.22.0"),
					VnetSubnetID:        pointer.String("fake/subnet/id"),
					MaxPods:             pointer.Int32(int32(32)),
					AvailabilityZones:   []*string{pointer.String("1"), pointer.String("2")},
					Tags: map[string]*string{
						"test-tag": pointer.String("test-value"),
					},
					EnableAutoScaling: pointer.Bool(false),
				},
			},
			LinuxProfile: &armcontainerservice.LinuxProfile{
				AdminUsername: pointer.String(azure.DefaultAKSUserName),
				SSH: &armcontainerservice.SSHConfiguration{
					PublicKeys: []*armcontainerservice.SSHPublicKey{
						{
							KeyData: pointer.String("test-ssh-key"),
						},
					},
				},
			},
			ServicePrincipalProfile: &armcontainerservice.ManagedClusterServicePrincipalProfile{ClientID: pointer.String("msi")},
			NodeResourceGroup:       pointer.String("test-node-rg"),
			EnableRBAC:              pointer.Bool(true),
			NetworkProfile: &armcontainerservice.NetworkProfile{
				LoadBalancerSKU: &loadBalancerSKUStandard,
			},
		},
		Identity: &armcontainerservice.ManagedClusterIdentity{
			Type: &resourceIdentityTypeSystemAssigned,
		},
		Location: pointer.String("test-location"),
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			ClusterName: "test-cluster",
			Name:        pointer.String("test-managedcluster"),
			Role:        pointer.String(infrav1.CommonRole),
			Additional: infrav1.Tags{
				"test-tag": "test-value",
			},
		})),
	}
}

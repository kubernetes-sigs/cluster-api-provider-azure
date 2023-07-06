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
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2022-03-01/containerservice"
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
			existing: containerservice.ManagedCluster{
				ManagedClusterProperties: &containerservice.ManagedClusterProperties{
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
				g.Expect(result).To(BeAssignableToTypeOf(containerservice.ManagedCluster{}))
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
				g.Expect(result).To(BeAssignableToTypeOf(containerservice.ManagedCluster{}))
				g.Expect(result.(containerservice.ManagedCluster).KubernetesVersion).To(Equal(pointer.String("v1.22.99")))
				g.Expect(result.(containerservice.ManagedCluster).Identity.Type).To(Equal(containerservice.ResourceIdentityType("UserAssigned")))
				g.Expect(result.(containerservice.ManagedCluster).Identity.UserAssignedIdentities).To(Equal(map[string]*containerservice.ManagedClusterIdentityUserAssignedIdentitiesValue{"/resource/ID": {}}))
				g.Expect(result.(containerservice.ManagedCluster).IdentityProfile).To(Equal(map[string]*containerservice.UserAssignedIdentity{kubeletIdentityKey: {ResourceID: pointer.String("/resource/ID")}}))
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
		expectedType containerservice.ResourceIdentityType
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
			expectedType: containerservice.ResourceIdentityTypeUserAssigned,
		},
		{
			name: "system-assigned identity",
			identity: &infrav1.Identity{
				Type: infrav1.ManagedControlPlaneIdentityTypeSystemAssigned,
			},
			expectedType: containerservice.ResourceIdentityTypeSystemAssigned,
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
					g.Expect(*result.UserAssignedIdentities[tc.identity.UserAssignedIdentityResourceID]).To(Equal(containerservice.ManagedClusterIdentityUserAssignedIdentitiesValue{}))
				} else {
					g.Expect(result.UserAssignedIdentities).To(BeEmpty())
				}
			} else {
				g.Expect(result).To(BeNil())
			}
		})
	}
}

func getExistingClusterWithAPIServerAccessProfile() containerservice.ManagedCluster {
	mc := getExistingCluster()
	mc.APIServerAccessProfile = &containerservice.ManagedClusterAPIServerAccessProfile{
		EnablePrivateCluster: pointer.Bool(false),
	}
	return mc
}

func getExistingCluster() containerservice.ManagedCluster {
	mc := getSampleManagedCluster()
	mc.ProvisioningState = pointer.String("Succeeded")
	mc.ID = pointer.String("test-id")
	return mc
}

func getSampleManagedCluster() containerservice.ManagedCluster {
	return containerservice.ManagedCluster{
		ManagedClusterProperties: &containerservice.ManagedClusterProperties{
			KubernetesVersion: pointer.String("v1.22.0"),
			DNSPrefix:         pointer.String("test-managedcluster"),
			AgentPoolProfiles: &[]containerservice.ManagedClusterAgentPoolProfile{
				{
					Name:         pointer.String("test-agentpool-0"),
					Mode:         containerservice.AgentPoolMode(infrav1.NodePoolModeSystem),
					Count:        pointer.Int32(2),
					Type:         containerservice.AgentPoolTypeVirtualMachineScaleSets,
					OsDiskSizeGB: pointer.Int32(0),
					Tags: map[string]*string{
						"test-tag": pointer.String("test-value"),
					},
				},
				{
					Name:                pointer.String("test-agentpool-1"),
					Mode:                containerservice.AgentPoolMode(infrav1.NodePoolModeUser),
					Count:               pointer.Int32(4),
					Type:                containerservice.AgentPoolTypeVirtualMachineScaleSets,
					OsDiskSizeGB:        pointer.Int32(0),
					VMSize:              pointer.String("test_SKU"),
					OrchestratorVersion: pointer.String("v1.22.0"),
					VnetSubnetID:        pointer.String("fake/subnet/id"),
					MaxPods:             pointer.Int32(int32(32)),
					AvailabilityZones:   &[]string{"1", "2"},
					Tags: map[string]*string{
						"test-tag": pointer.String("test-value"),
					},
				},
			},
			LinuxProfile: &containerservice.LinuxProfile{
				AdminUsername: pointer.String(azure.DefaultAKSUserName),
				SSH: &containerservice.SSHConfiguration{
					PublicKeys: &[]containerservice.SSHPublicKey{
						{
							KeyData: pointer.String(""),
						},
					},
				},
			},
			ServicePrincipalProfile: &containerservice.ManagedClusterServicePrincipalProfile{ClientID: pointer.String("msi")},
			NodeResourceGroup:       pointer.String("test-node-rg"),
			EnableRBAC:              pointer.Bool(true),
			NetworkProfile: &containerservice.NetworkProfile{
				LoadBalancerSku: containerservice.LoadBalancerSku("Standard"),
			},
		},
		Identity: &containerservice.ManagedClusterIdentity{
			Type: containerservice.ResourceIdentityTypeSystemAssigned,
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

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
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-05-01/containerservice"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
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
					ProvisioningState: to.StringPtr("Deleting"),
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
				Location:          "test-location",
				Tags: map[string]string{
					"test-tag": "test-value",
				},
				Version:         "v1.22.0",
				LoadBalancerSKU: "Standard",
				GetAllNodePools: func() ([]azure.AKSNodePoolSpec, error) {
					return []azure.AKSNodePoolSpec{
						{
							Name:          "test-nodepool-0",
							Mode:          string(infrav1exp.NodePoolModeSystem),
							ResourceGroup: "test-rg",
							Replicas:      int32(2),
						},
						{
							Name:              "test-nodepool-1",
							Mode:              string(infrav1exp.NodePoolModeUser),
							ResourceGroup:     "test-rg",
							Replicas:          int32(4),
							Cluster:           "test-managedcluster",
							SKU:               "test_SKU",
							Version:           to.StringPtr("v1.22.0"),
							VnetSubnetID:      "fake/subnet/id",
							MaxPods:           to.Int32Ptr(int32(32)),
							AvailabilityZones: []string{"1", "2"},
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
			},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(containerservice.ManagedCluster{}))
				g.Expect(result.(containerservice.ManagedCluster).KubernetesVersion).To(Equal(to.StringPtr("v1.22.99")))
			},
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(tc.existing)
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

func getExistingCluster() containerservice.ManagedCluster {
	mc := getSampleManagedCluster()
	mc.ProvisioningState = to.StringPtr("Succeeded")
	mc.ID = to.StringPtr("test-id")
	return mc
}

func getSampleManagedCluster() containerservice.ManagedCluster {
	return containerservice.ManagedCluster{
		ManagedClusterProperties: &containerservice.ManagedClusterProperties{
			KubernetesVersion: to.StringPtr("v1.22.0"),
			DNSPrefix:         to.StringPtr("test-managedcluster"),
			AgentPoolProfiles: &[]containerservice.ManagedClusterAgentPoolProfile{
				converters.NodePoolToManagedClusterAgentPoolProfile(azure.AKSNodePoolSpec{
					Name:          "test-nodepool-0",
					Mode:          string(infrav1exp.NodePoolModeSystem),
					ResourceGroup: "test-rg",
					Replicas:      int32(2),
				}),
				converters.NodePoolToManagedClusterAgentPoolProfile(azure.AKSNodePoolSpec{
					Name:              "test-nodepool-1",
					Mode:              string(infrav1exp.NodePoolModeUser),
					ResourceGroup:     "test-rg",
					Replicas:          int32(4),
					Cluster:           "test-managedcluster",
					SKU:               "test_SKU",
					Version:           to.StringPtr("v1.22.0"),
					VnetSubnetID:      "fake/subnet/id",
					MaxPods:           to.Int32Ptr(int32(32)),
					AvailabilityZones: []string{"1", "2"},
				}),
			},
			LinuxProfile: &containerservice.LinuxProfile{
				AdminUsername: to.StringPtr(azure.DefaultAKSUserName),
				SSH: &containerservice.SSHConfiguration{
					PublicKeys: &[]containerservice.SSHPublicKey{
						{
							KeyData: to.StringPtr(""),
						},
					},
				},
			},
			ServicePrincipalProfile: &containerservice.ManagedClusterServicePrincipalProfile{ClientID: to.StringPtr("msi")},
			NodeResourceGroup:       to.StringPtr("test-node-rg"),
			EnableRBAC:              to.BoolPtr(true),
			NetworkProfile: &containerservice.NetworkProfile{
				LoadBalancerSku: containerservice.LoadBalancerSku("Standard"),
			},
		},
		Identity: &containerservice.ManagedClusterIdentity{
			Type: containerservice.ResourceIdentityTypeSystemAssigned,
		},
		Location: to.StringPtr("test-location"),
		Tags: map[string]*string{
			"test-tag": to.StringPtr("test-value"),
		},
	}
}

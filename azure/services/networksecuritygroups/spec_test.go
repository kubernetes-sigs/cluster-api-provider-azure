/*
Copyright 2025 The Kubernetes Authors.

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

package networksecuritygroups

import (
	"testing"

	"github.com/Azure/azure-service-operator/v2/api/network/v1api20201101"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

func TestNSGSpec_ResourceRef(t *testing.T) {
	g := NewWithT(t)

	spec := &NSGSpec{
		Name:          "test-nsg",
		Location:      "eastus",
		ClusterName:   "test-cluster",
		ResourceGroup: "test-rg",
	}

	ref := spec.ResourceRef()
	g.Expect(ref).NotTo(BeNil())
	g.Expect(ref.Name).To(Equal(azure.GetNormalizedKubernetesName("test-nsg")))
}

func TestNSGSpec_ResourceName(t *testing.T) {
	g := NewWithT(t)

	spec := &NSGSpec{
		Name: "test-nsg",
	}

	g.Expect(spec.ResourceName()).To(Equal("test-nsg"))
}

func TestNSGSpec_ResourceGroupName(t *testing.T) {
	g := NewWithT(t)

	spec := &NSGSpec{
		ResourceGroup: "test-rg",
	}

	g.Expect(spec.ResourceGroupName()).To(Equal("test-rg"))
}

func TestNSGSpec_Parameters(t *testing.T) {
	testCases := []struct {
		name     string
		spec     *NSGSpec
		existing *v1api20201101.NetworkSecurityGroup
	}{
		{
			name: "new network security group",
			spec: &NSGSpec{
				Name:          "test-nsg",
				Location:      "eastus",
				ClusterName:   "test-cluster",
				ResourceGroup: "test-rg",
				SecurityRules: []infrav1.SecurityRule{
					{
						Name:             "allow_ssh",
						Description:      "Allow SSH",
						Priority:         1000,
						Protocol:         infrav1.SecurityGroupProtocolTCP,
						Direction:        infrav1.SecurityRuleDirectionInbound,
						Source:           ptr.To("*"),
						SourcePorts:      ptr.To("*"),
						Destination:      ptr.To("*"),
						DestinationPorts: ptr.To("22"),
						Action:           infrav1.SecurityRuleActionAllow,
					},
				},
				AdditionalTags: infrav1.Tags{
					"environment": "test",
				},
			},
			existing: nil,
		},
		{
			name: "existing network security group",
			spec: &NSGSpec{
				Name:          "existing-nsg",
				Location:      "westus",
				ClusterName:   "test-cluster",
				ResourceGroup: "test-rg",
				AdditionalTags: infrav1.Tags{
					"updated": "true",
				},
			},
			existing: &v1api20201101.NetworkSecurityGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: "existing-nsg",
				},
				Spec: v1api20201101.NetworkSecurityGroup_Spec{
					Location: ptr.To("eastus"),
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			result, err := tc.spec.Parameters(t.Context(), tc.existing)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result).NotTo(BeNil())

			// Verify basic spec properties
			g.Expect(*result.Spec.Location).To(Equal(tc.spec.Location))
			g.Expect(result.Spec.Owner.Name).To(Equal(tc.spec.ResourceGroup))

			// Verify labels include cluster name
			g.Expect(result.ObjectMeta.Labels).To(HaveKeyWithValue(clusterv1beta1.ClusterNameLabel, tc.spec.ClusterName))

			// Verify additional tags are included in labels
			for k, v := range tc.spec.AdditionalTags {
				g.Expect(result.ObjectMeta.Labels).To(HaveKeyWithValue(k, v))
			}
		})
	}
}

func TestNSGSpec_WasManaged(t *testing.T) {
	g := NewWithT(t)

	spec := &NSGSpec{
		Name:          "test-nsg",
		Location:      "eastus",
		ClusterName:   "test-cluster",
		ResourceGroup: "test-rg",
	}

	// Should always return true for NSGs
	g.Expect(spec.WasManaged(nil)).To(BeTrue())
	g.Expect(spec.WasManaged(&v1api20201101.NetworkSecurityGroup{})).To(BeTrue())
}

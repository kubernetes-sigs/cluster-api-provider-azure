/*
Copyright 2023 The Kubernetes Authors.

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

package virtualnetworks

import (
	"context"
	"testing"

	asonetworkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20201101"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestParameters(t *testing.T) {
	tests := []struct {
		name     string
		spec     VNetSpec
		existing *asonetworkv1.VirtualNetwork
		expected *asonetworkv1.VirtualNetwork
	}{
		{
			name: "new vnet",
			spec: VNetSpec{
				ResourceGroup: "rg",
				Name:          "name",
				Namespace:     "namespace",
				CIDRs:         []string{"cidr"},
				Location:      "location",
				ExtendedLocation: &infrav1.ExtendedLocationSpec{
					Name: "loc-name",
					Type: "loc-type",
				},
				ClusterName:    "cluster",
				AdditionalTags: map[string]string{"my": "tag"},
			},
			expected: &asonetworkv1.VirtualNetwork{
				Spec: asonetworkv1.VirtualNetwork_Spec{
					Tags: map[string]string{
						"my": "tag",
						"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster": "owned",
						"sigs.k8s.io_cluster-api-provider-azure_role":            "common",
						"Name": "name",
					},
					AzureName: "name",
					Owner: &genruntime.KnownResourceReference{
						Name: "rg",
					},
					Location: ptr.To("location"),
					ExtendedLocation: &asonetworkv1.ExtendedLocation{
						Name: ptr.To("loc-name"),
						Type: ptr.To(asonetworkv1.ExtendedLocationType("loc-type")),
					},
					AddressSpace: &asonetworkv1.AddressSpace{
						AddressPrefixes: []string{"cidr"},
					},
				},
			},
		},
		{
			name: "from existing vnet",
			spec: VNetSpec{
				ResourceGroup: "rg",
				Name:          "name",
				Namespace:     "namespace",
				CIDRs:         []string{"cidr"},
				Location:      "location",
				ExtendedLocation: &infrav1.ExtendedLocationSpec{
					Name: "loc-name",
					Type: "loc-type",
				},
				ClusterName:    "cluster",
				AdditionalTags: map[string]string{"my": "tag"},
			},
			existing: &asonetworkv1.VirtualNetwork{
				Spec: asonetworkv1.VirtualNetwork_Spec{
					Tags: map[string]string{
						"tags": "set",
						"by":   "user",
					},
				},
			},
			expected: &asonetworkv1.VirtualNetwork{
				Spec: asonetworkv1.VirtualNetwork_Spec{
					Tags: map[string]string{
						"tags": "set",
						"by":   "user",
					},
					AzureName: "name",
					Owner: &genruntime.KnownResourceReference{
						Name: "rg",
					},
					Location: ptr.To("location"),
					ExtendedLocation: &asonetworkv1.ExtendedLocation{
						Name: ptr.To("loc-name"),
						Type: ptr.To(asonetworkv1.ExtendedLocationType("loc-type")),
					},
					AddressSpace: &asonetworkv1.AddressSpace{
						AddressPrefixes: []string{"cidr"},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			actual, err := test.spec.Parameters(context.Background(), test.existing)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cmp.Diff(test.expected, actual)).To(BeEmpty())
		})
	}
}

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

package privatedns

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/privatedns/mgmt/2018-09-01/privatedns"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

var (
	linkSpec = LinkSpec{
		Name:              "my-link",
		ZoneName:          "my-zone",
		SubscriptionID:    "123",
		VNetResourceGroup: "my-vnet-rg",
		VNetName:          "my-vnet",
		ResourceGroup:     "my-rg",
		ClusterName:       "my-cluster",
		AdditionalTags:    nil,
	}
)

func TestLinkSpec_ResourceName(t *testing.T) {
	g := NewWithT(t)
	g.Expect(linkSpec.ResourceName()).Should(Equal("my-link"))
}

func TestLinkSpec_ResourceGroupName(t *testing.T) {
	g := NewWithT(t)
	g.Expect(linkSpec.ResourceGroupName()).Should(Equal("my-rg"))
}

func TestLinkSpec_OwnerResourceName(t *testing.T) {
	g := NewWithT(t)
	g.Expect(linkSpec.OwnerResourceName()).Should(Equal("my-zone"))
}

func TestLinkSpec_Parameters(t *testing.T) {
	testcases := []struct {
		name          string
		spec          LinkSpec
		existing      interface{}
		expect        func(g *WithT, result interface{})
		expectedError string
	}{
		{
			name:          "new private dns virtual network link",
			expectedError: "",
			spec:          linkSpec,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(Equal(privatedns.VirtualNetworkLink{
					VirtualNetworkLinkProperties: &privatedns.VirtualNetworkLinkProperties{
						VirtualNetwork: &privatedns.SubResource{
							ID: ptr.To("/subscriptions/123/resourceGroups/my-vnet-rg/providers/Microsoft.Network/virtualNetworks/my-vnet"),
						},
						RegistrationEnabled: ptr.To(false),
					},
					Location: ptr.To(azure.Global),
					Tags: map[string]*string{
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
					},
				}))
			},
		},
		{
			name:          "existing managed private virtual network link",
			expectedError: "",
			spec:          linkSpec,
			existing: privatedns.VirtualNetworkLink{Tags: map[string]*string{
				"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
			}},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
		},
		{
			name:          "existing unmanaged private dns zone",
			expectedError: "",
			spec:          linkSpec,
			existing:      privatedns.VirtualNetworkLink{},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
		},
		{
			name:          "type cast error",
			expectedError: "string is not a privatedns.VirtualNetworkLink",
			spec:          linkSpec,
			existing:      "I'm not privatedns.VirtualNetworkLink",
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			result, err := tc.spec.Parameters(context.TODO(), tc.existing)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				tc.expect(g, result)
			}
		})
	}
}

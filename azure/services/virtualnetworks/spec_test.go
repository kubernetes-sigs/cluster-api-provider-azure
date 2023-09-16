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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

var (
	fakeVirtualNetwork = armnetwork.VirtualNetwork{
		ID:   ptr.To("/subscriptions/subscription/resourceGroups/test-group/providers/Microsoft.Network/virtualNetworks/test-vnet"),
		Name: ptr.To("test-vnet"),
		Tags: map[string]*string{
			"foo":       ptr.To("bar"),
			"something": ptr.To("else"),
		},
		Properties: &armnetwork.VirtualNetworkPropertiesFormat{
			AddressSpace: &armnetwork.AddressSpace{
				AddressPrefixes: []*string{ptr.To("fake-cidr")},
			},
			Subnets: []*armnetwork.Subnet{
				{
					Name: ptr.To("test-subnet"),
					Properties: &armnetwork.SubnetPropertiesFormat{
						AddressPrefix: ptr.To("subnet-cidr"),
					},
				},
				{
					Name: ptr.To("test-subnet-2"),
					Properties: &armnetwork.SubnetPropertiesFormat{
						AddressPrefixes: []*string{
							ptr.To("subnet-cidr-1"),
							ptr.To("subnet-cidr-2"),
						},
					},
				},
			},
		},
	}

	fakeVNetSpec1 = VNetSpec{
		Name:        "test-vnet",
		ClusterName: "cluster",
		CIDRs:       []string{"10.0.0.0/8"},
		Location:    "test-location",
		ExtendedLocation: &infrav1.ExtendedLocationSpec{
			Name: "test-extended-location-name",
			Type: "test-extended-location-type",
		},
		AdditionalTags: map[string]string{"foo": "bar"},
	}
	fakeVNetTags = map[string]*string{
		"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster": ptr.To("owned"),
		"sigs.k8s.io_cluster-api-provider-azure_role":            ptr.To("common"),
		"foo":  ptr.To("bar"),
		"Name": ptr.To("test-vnet"),
	}
)

func TestVNetSpec_Parameters(t *testing.T) {
	testCases := []struct {
		name          string
		spec          *VNetSpec
		existing      interface{}
		expect        func(g *WithT, result interface{})
		expectedError string
	}{
		{
			name:     "get result as nil when existing VirtualNetwork is present",
			spec:     &fakeVNetSpec1,
			existing: fakeVirtualNetwork,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
		{
			name:     "get result as nil when existing VirtualNetwork is present with empty data",
			spec:     &fakeVNetSpec1,
			existing: armnetwork.VirtualNetwork{},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
		{
			name:     "get VirtualNetwork when all values are present",
			spec:     &fakeVNetSpec1,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.VirtualNetwork{}))
				g.Expect(result.(armnetwork.VirtualNetwork).Location).To(Equal(ptr.To[string](fakeVNetSpec1.Location)))
				g.Expect(result.(armnetwork.VirtualNetwork).Tags).To(Equal(fakeVNetTags))
				g.Expect(result.(armnetwork.VirtualNetwork).ExtendedLocation.Name).To(Equal(ptr.To[string](fakeVNetSpec1.ExtendedLocation.Name)))
				g.Expect(result.(armnetwork.VirtualNetwork).ExtendedLocation.Type).To(Equal(ptr.To(armnetwork.ExtendedLocationTypes(fakeVNetSpec1.ExtendedLocation.Type))))
				g.Expect(result.(armnetwork.VirtualNetwork).Properties.AddressSpace.AddressPrefixes[0]).To(Equal(ptr.To(fakeVNetSpec1.CIDRs[0])))
			},
			expectedError: "",
		},
	}
	for _, tc := range testCases {
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
			}
			tc.expect(g, result)
		})
	}
}

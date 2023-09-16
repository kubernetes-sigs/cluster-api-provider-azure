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

package bastionhosts

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

var (
	fakeSKU         = armnetwork.BastionHostSKUName("fake-SKU")
	fakeBastionHost = armnetwork.BastionHost{
		Location: &fakeAzureBastionSpec.Location,
		Name:     ptr.To("my-bastion-host"),
		SKU:      &armnetwork.SKU{Name: &fakeSKU},
	}
	fakeAzureBastionSpec1 = AzureBastionSpec{
		Name:            "my-bastion",
		ClusterName:     "cluster",
		Location:        "westus",
		SubnetID:        "my-subnet-id",
		PublicIPID:      "my-public-ip-id",
		Sku:             infrav1.BastionHostSkuName("fake-SKU"),
		EnableTunneling: false,
	}

	fakeBastionHostTags = map[string]*string{
		"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster": ptr.To("owned"),
		"sigs.k8s.io_cluster-api-provider-azure_role":            ptr.To("Bastion"),
		"Name": ptr.To("my-bastion"),
	}
)

func TestAzureBastionSpec_Parameters(t *testing.T) {
	testCases := []struct {
		name          string
		spec          *AzureBastionSpec
		existing      interface{}
		expect        func(g *WithT, result interface{})
		expectedError string
	}{
		{
			name:     "error when existing host is not of BastionHost type",
			spec:     &AzureBastionSpec{},
			existing: struct{}{},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "struct {} is not an armnetwork.BastionHost",
		},
		{
			name:     "get result as nil when existing BastionHost is present",
			spec:     &AzureBastionSpec{},
			existing: fakeBastionHost,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
		{
			name:     "get result as nil when existing BastionHost is present with empty data",
			spec:     &AzureBastionSpec{},
			existing: armnetwork.BastionHost{},
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeNil())
			},
			expectedError: "",
		},
		{
			name:     "get BastionHost when all values are present",
			spec:     &fakeAzureBastionSpec1,
			existing: nil,
			expect: func(g *WithT, result interface{}) {
				g.Expect(result).To(BeAssignableToTypeOf(armnetwork.BastionHost{}))
				g.Expect(result.(armnetwork.BastionHost).Location).To(Equal(ptr.To[string](fakeAzureBastionSpec1.Location)))
				g.Expect(result.(armnetwork.BastionHost).Name).To(Equal(ptr.To[string](fakeAzureBastionSpec1.ResourceName())))
				g.Expect(result.(armnetwork.BastionHost).SKU.Name).To(Equal(ptr.To(armnetwork.BastionHostSKUName(fakeAzureBastionSpec1.Sku))))
				g.Expect(result.(armnetwork.BastionHost).Properties.EnableTunneling).To(Equal(ptr.To(fakeAzureBastionSpec1.EnableTunneling)))
				g.Expect(result.(armnetwork.BastionHost).Tags).To(Equal(fakeBastionHostTags))
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

/*
Copyright 2021 The Kubernetes Authors.

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

package azure

import (
	"testing"

	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestVMSS_HasModelChanges(t *testing.T) {
	cases := []struct {
		Name            string
		Factory         func() (VMSS, VMSS)
		HasModelChanges bool
	}{
		{
			Name: "two empty VMSS",
			Factory: func() (VMSS, VMSS) {
				return VMSS{}, VMSS{}
			},
			HasModelChanges: false,
		},
		{
			Name: "one empty and other with image changes",
			Factory: func() (VMSS, VMSS) {
				return VMSS{}, VMSS{
					Image: infrav1.Image{
						Marketplace: &infrav1.AzureMarketplaceImage{
							Version: "foo",
						},
					},
				}
			},
			HasModelChanges: true,
		},
		{
			Name: "one empty and other with image changes",
			Factory: func() (VMSS, VMSS) {
				return VMSS{}, VMSS{
					Image: infrav1.Image{
						Marketplace: &infrav1.AzureMarketplaceImage{
							Version: "foo",
						},
					},
				}
			},
			HasModelChanges: true,
		},
		{
			Name: "same default VMSS",
			Factory: func() (VMSS, VMSS) {
				l := getDefaultVMSSForModelTesting()
				r := getDefaultVMSSForModelTesting()
				return r, l
			},
			HasModelChanges: false,
		},
		{
			Name: "with different identity",
			Factory: func() (VMSS, VMSS) {
				l := getDefaultVMSSForModelTesting()
				l.Identity = infrav1.VMIdentityNone
				r := getDefaultVMSSForModelTesting()
				return r, l
			},
			HasModelChanges: true,
		},
		{
			Name: "with different Zones",
			Factory: func() (VMSS, VMSS) {
				l := getDefaultVMSSForModelTesting()
				l.Zones = []string{"0"}
				r := getDefaultVMSSForModelTesting()
				return r, l
			},
			HasModelChanges: true,
		},
		{
			Name: "with empty image",
			Factory: func() (VMSS, VMSS) {
				l := getDefaultVMSSForModelTesting()
				l.Image = infrav1.Image{}
				r := getDefaultVMSSForModelTesting()
				return r, l
			},
			HasModelChanges: true,
		},
		{
			Name: "with different image reference ID",
			Factory: func() (VMSS, VMSS) {
				l := getDefaultVMSSForModelTesting()
				l.Image = infrav1.Image{
					ID: to.StringPtr("foo"),
				}
				r := getDefaultVMSSForModelTesting()
				return r, l
			},
			HasModelChanges: true,
		},
		{
			Name: "with different SKU",
			Factory: func() (VMSS, VMSS) {
				l := getDefaultVMSSForModelTesting()
				l.Sku = "reallySmallVM"
				r := getDefaultVMSSForModelTesting()
				return r, l
			},
			HasModelChanges: true,
		},
		{
			Name: "with different Tags",
			Factory: func() (VMSS, VMSS) {
				l := getDefaultVMSSForModelTesting()
				l.Tags = infrav1.Tags{
					"bin": "baz",
				}
				r := getDefaultVMSSForModelTesting()
				return r, l
			},
			HasModelChanges: true,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			l, r := c.Factory()
			g := NewWithT(t)
			g.Expect(l.HasModelChanges(r)).To(Equal(c.HasModelChanges))
		})
	}
}

func getDefaultVMSSForModelTesting() VMSS {
	return VMSS{
		Zones: []string{"0", "1"},
		Image: infrav1.Image{
			Marketplace: &infrav1.AzureMarketplaceImage{
				Version: "foo",
			},
		},
		Sku:      "reallyBigVM",
		Identity: infrav1.VMIdentitySystemAssigned,
		Tags: infrav1.Tags{
			"foo": "baz",
		},
	}
}

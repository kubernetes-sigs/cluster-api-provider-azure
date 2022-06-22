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

package converters

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func Test_ImageToPlan(t *testing.T) {
	cases := []struct {
		name   string
		image  *infrav1.Image
		expect func(*GomegaWithT, *compute.Plan)
	}{
		{
			name: "Should return a plan for a Community Gallery image with plan details",
			image: &infrav1.Image{
				ComputeGallery: &infrav1.AzureComputeGalleryImage{
					Plan: &infrav1.ImagePlan{
						Publisher: "my-publisher",
						Offer:     "my-offer",
						SKU:       "my-sku",
					},
				},
			},
			expect: func(g *GomegaWithT, result *compute.Plan) {
				g.Expect(result).To(Equal(&compute.Plan{
					Name:      to.StringPtr("my-sku"),
					Publisher: to.StringPtr("my-publisher"),
					Product:   to.StringPtr("my-offer"),
				}))
			},
		},
		{
			name: "Should return a plan for a SIG image with plan details",
			image: &infrav1.Image{
				SharedGallery: &infrav1.AzureSharedGalleryImage{
					SubscriptionID: "fake-sub-id",
					ResourceGroup:  "fake-rg",
					Gallery:        "fake-gallery-name",
					Name:           "fake-image-name",
					Version:        "v1.0.0",
					Publisher:      to.StringPtr("my-publisher"),
					Offer:          to.StringPtr("my-offer"),
					SKU:            to.StringPtr("my-sku"),
				},
			},
			expect: func(g *GomegaWithT, result *compute.Plan) {
				g.Expect(result).To(Equal(&compute.Plan{
					Name:      to.StringPtr("my-sku"),
					Publisher: to.StringPtr("my-publisher"),
					Product:   to.StringPtr("my-offer"),
				}))
			},
		},
		{
			name: "Should return nil for a SIG image without plan info",
			image: &infrav1.Image{
				SharedGallery: &infrav1.AzureSharedGalleryImage{
					SubscriptionID: "fake-sub-id",
					ResourceGroup:  "fake-rg",
					Gallery:        "fake-gallery-name",
					Name:           "fake-image-name",
					Version:        "v1.0.0",
				},
			},
			expect: func(g *GomegaWithT, result *compute.Plan) {
				g.Expect(result).To(BeNil())
			},
		},
		{
			name: "Should return nil for a Marketplace first party image",
			image: &infrav1.Image{
				Marketplace: &infrav1.AzureMarketplaceImage{
					ImagePlan: infrav1.ImagePlan{
						Publisher: "my-publisher",
						Offer:     "my-offer",
						SKU:       "my-sku",
					},
					Version:         "v0.5.0",
					ThirdPartyImage: false,
				},
			},
			expect: func(g *GomegaWithT, result *compute.Plan) {
				g.Expect(result).To(BeNil())
			},
		},
		{
			name: "Should return a plan for a Marketplace third party image",
			image: &infrav1.Image{
				Marketplace: &infrav1.AzureMarketplaceImage{
					ImagePlan: infrav1.ImagePlan{
						Publisher: "my-publisher",
						Offer:     "my-offer",
						SKU:       "my-sku",
					},
					Version:         "v0.5.0",
					ThirdPartyImage: true,
				},
			},
			expect: func(g *GomegaWithT, result *compute.Plan) {
				g.Expect(result).To(Equal(&compute.Plan{
					Name:      to.StringPtr("my-sku"),
					Publisher: to.StringPtr("my-publisher"),
					Product:   to.StringPtr("my-offer"),
				}))
			},
		},
		{
			name: "Should return nil for an image ID",
			image: &infrav1.Image{
				ID: to.StringPtr("fake/image/id"),
			},
			expect: func(g *GomegaWithT, result *compute.Plan) {
				g.Expect(result).To(BeNil())
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := NewGomegaWithT(t)
			result := ImageToPlan(c.image)
			c.expect(g, result)
		})
	}
}

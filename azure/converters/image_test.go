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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func Test_ImageToPlan(t *testing.T) {
	cases := []struct {
		name   string
		image  *infrav1.Image
		expect func(*GomegaWithT, *armcompute.Plan)
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
			expect: func(g *GomegaWithT, result *armcompute.Plan) {
				g.Expect(result).To(Equal(&armcompute.Plan{
					Name:      ptr.To("my-sku"),
					Publisher: ptr.To("my-publisher"),
					Product:   ptr.To("my-offer"),
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
					Publisher:      ptr.To("my-publisher"),
					Offer:          ptr.To("my-offer"),
					SKU:            ptr.To("my-sku"),
				},
			},
			expect: func(g *GomegaWithT, result *armcompute.Plan) {
				g.Expect(result).To(Equal(&armcompute.Plan{
					Name:      ptr.To("my-sku"),
					Publisher: ptr.To("my-publisher"),
					Product:   ptr.To("my-offer"),
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
			expect: func(g *GomegaWithT, result *armcompute.Plan) {
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
			expect: func(g *GomegaWithT, result *armcompute.Plan) {
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
			expect: func(g *GomegaWithT, result *armcompute.Plan) {
				g.Expect(result).To(Equal(&armcompute.Plan{
					Name:      ptr.To("my-sku"),
					Publisher: ptr.To("my-publisher"),
					Product:   ptr.To("my-offer"),
				}))
			},
		},
		{
			name: "Should return nil for an image ID",
			image: &infrav1.Image{
				ID: ptr.To("fake/image/id"),
			},
			expect: func(g *GomegaWithT, result *armcompute.Plan) {
				g.Expect(result).To(BeNil())
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := NewGomegaWithT(t)
			result := ImageToPlan(c.image)
			c.expect(g, result)
		})
	}
}

func Test_ComputeImageToSDK(t *testing.T) {
	cases := []struct {
		name   string
		image  *infrav1.Image
		expect func(*GomegaWithT, *armcompute.ImageReference, error)
	}{
		{
			name: "Should return parsed compute gallery image id",
			image: &infrav1.Image{
				ComputeGallery: &infrav1.AzureComputeGalleryImage{
					ResourceGroup:  ptr.To("my-resourcegroup"),
					SubscriptionID: ptr.To("my-subscription-id"),
					Gallery:        "my-gallery",
					Name:           "my-image",
					Version:        "my-version",
				},
			},
			expect: func(g *GomegaWithT, result *armcompute.ImageReference, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(&armcompute.ImageReference{
					ID: ptr.To("/subscriptions/my-subscription-id/resourceGroups/my-resourcegroup/providers/Microsoft.Compute/galleries/my-gallery/images/my-image/versions/my-version"),
				}))
			},
		},
		{
			name: "Should return parsed shared gallery image id",
			image: &infrav1.Image{
				SharedGallery: &infrav1.AzureSharedGalleryImage{
					ResourceGroup:  "my-resourcegroup",
					SubscriptionID: "my-subscription-id",
					Gallery:        "my-gallery",
					Name:           "my-image",
					Version:        "my-version",
				},
			},
			expect: func(g *GomegaWithT, result *armcompute.ImageReference, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(&armcompute.ImageReference{
					ID: ptr.To("/subscriptions/my-subscription-id/resourceGroups/my-resourcegroup/providers/Microsoft.Compute/galleries/my-gallery/images/my-image/versions/my-version"),
				}))
			},
		},
		{
			name: "Should return parsed community gallery image id",
			image: &infrav1.Image{
				ComputeGallery: &infrav1.AzureComputeGalleryImage{
					Gallery: "my-gallery",
					Name:    "my-image",
					Version: "my-version",
				},
			},
			expect: func(g *GomegaWithT, result *armcompute.ImageReference, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(&armcompute.ImageReference{
					CommunityGalleryImageID: ptr.To("/CommunityGalleries/my-gallery/Images/my-image/Versions/my-version"),
				}))
			},
		},
		{
			name: "Should return error if SharedGallery and ComputeGallery are nil",
			image: &infrav1.Image{
				ComputeGallery: nil,
				SharedGallery:  nil,
			},
			expect: func(g *GomegaWithT, result *armcompute.ImageReference, err error) {
				g.Expect(err).Should(HaveOccurred())
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := NewGomegaWithT(t)
			result, err := computeImageToSDK(c.image)
			c.expect(g, result, err)
		})
	}
}

func Test_ImageToSDK(t *testing.T) {
	cases := []struct {
		name   string
		image  *infrav1.Image
		expect func(*GomegaWithT, *armcompute.ImageReference, error)
	}{
		{
			name: "Should return parsed compute gallery image id",
			image: &infrav1.Image{
				ComputeGallery: &infrav1.AzureComputeGalleryImage{
					ResourceGroup:  ptr.To("my-resourcegroup"),
					SubscriptionID: ptr.To("my-subscription-id"),
					Gallery:        "my-gallery",
					Name:           "my-image",
					Version:        "my-version",
				},
			},
			expect: func(g *GomegaWithT, result *armcompute.ImageReference, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(&armcompute.ImageReference{
					ID: ptr.To("/subscriptions/my-subscription-id/resourceGroups/my-resourcegroup/providers/Microsoft.Compute/galleries/my-gallery/images/my-image/versions/my-version"),
				}))
			},
		},
		{
			name: "Should return parsed shared gallery image id",
			image: &infrav1.Image{
				SharedGallery: &infrav1.AzureSharedGalleryImage{
					ResourceGroup:  "my-resourcegroup",
					SubscriptionID: "my-subscription-id",
					Gallery:        "my-gallery",
					Name:           "my-image",
					Version:        "my-version",
				},
			},
			expect: func(g *GomegaWithT, result *armcompute.ImageReference, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(&armcompute.ImageReference{
					ID: ptr.To("/subscriptions/my-subscription-id/resourceGroups/my-resourcegroup/providers/Microsoft.Compute/galleries/my-gallery/images/my-image/versions/my-version"),
				}))
			},
		},
		{
			name: "Should return parsed community gallery image id",
			image: &infrav1.Image{
				ComputeGallery: &infrav1.AzureComputeGalleryImage{
					Gallery: "my-gallery",
					Name:    "my-image",
					Version: "my-version",
				},
			},
			expect: func(g *GomegaWithT, result *armcompute.ImageReference, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(&armcompute.ImageReference{
					CommunityGalleryImageID: ptr.To("/CommunityGalleries/my-gallery/Images/my-image/Versions/my-version"),
				}))
			},
		},
		{
			name: "Should return error if SharedGallery and ComputeGallery are nil",
			image: &infrav1.Image{
				ComputeGallery: nil,
				SharedGallery:  nil,
			},
			expect: func(g *GomegaWithT, result *armcompute.ImageReference, err error) {
				g.Expect(err).Should(HaveOccurred())
			},
		},
		{
			name: "Should return specific image if ID is set",
			image: &infrav1.Image{
				ID: ptr.To("my-image-id"),
			},
			expect: func(g *GomegaWithT, result *armcompute.ImageReference, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(&armcompute.ImageReference{
					ID: ptr.To("my-image-id"),
				}))
			},
		},
		{
			name: "Should return Marketplace image if Marketplace is set",
			image: &infrav1.Image{
				Marketplace: &infrav1.AzureMarketplaceImage{
					ImagePlan: infrav1.ImagePlan{
						Publisher: "my-publisher",
						Offer:     "my-offer",
						SKU:       "my-sku",
					},
					Version: "v0.5.0",
				},
			},
			expect: func(g *GomegaWithT, result *armcompute.ImageReference, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(&armcompute.ImageReference{
					Offer:     ptr.To("my-offer"),
					Publisher: ptr.To("my-publisher"),
					SKU:       ptr.To("my-sku"),
					Version:   ptr.To("v0.5.0"),
				}))
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			g := NewGomegaWithT(t)
			result, err := ImageToSDK(c.image)
			c.expect(g, result, err)
		})
	}
}

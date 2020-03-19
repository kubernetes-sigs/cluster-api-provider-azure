/*
Copyright 2020 The Kubernetes Authors.

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

package v1alpha3

import (
	"testing"

	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestImageRequired(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		Image *Image
	}

	extension := test{}

	errs := ValidateImage(extension.Image, field.NewPath("image"))
	g.Expect(errs).To(HaveLen(1))
	g.Expect(errs[0].Type).To(Equal(field.ErrorTypeRequired))
	g.Expect(errs[0].Field).To(Equal("image"))
	g.Expect(errs[0].Detail).NotTo(BeEmpty())
}

func TestImageTooManyDetails(t *testing.T) {
	g := NewWithT(t)

	image := &Image{
		Marketplace: &AzureMarketplaceImage{
			Offer:     "OFFER",
			Publisher: "PUBLISHER",
			SKU:       "SKU",
			Version:   "1.0.0.",
		},
		SharedGallery: &AzureSharedGalleryImage{
			Gallery:        "GALLERY",
			Name:           "GALLERY1",
			ResourceGroup:  "RG1",
			SubscriptionID: "SUB12",
			Version:        "1.0.0.",
		},
	}

	g.Expect(ValidateImage(image, field.NewPath("image"))).To(HaveLen(1))
}

func TestSharedImageGalleryValid(t *testing.T) {
	g := NewWithT(t)

	testCases := map[string]struct {
		image          *Image
		expectedErrors int
	}{
		"AzureSharedGalleryImage - fully specified": {
			expectedErrors: 0,
			image:          createTestSharedImage("SUB1243", "RG1234", "IMAGENAME", "GALLERY9876", "1.0.0"),
		},
		"AzureSharedGalleryImage - missing subscription": {
			expectedErrors: 1,
			image:          createTestSharedImage("", "RG1234", "IMAGENAME", "GALLERY9876", "1.0.0"),
		},
		"AzureSharedGalleryImage - missing resource group": {
			expectedErrors: 1,
			image:          createTestSharedImage("SUB1234", "", "IMAGENAME", "GALLERY9876", "1.0.0"),
		},
		"AzureSharedGalleryImage - missing image name": {
			expectedErrors: 1,
			image:          createTestSharedImage("SUB1243", "RG1234", "", "GALLERY9876", "1.0.0"),
		},
		"AzureSharedGalleryImage - missing gallery": {
			expectedErrors: 1,
			image:          createTestSharedImage("SUB1243", "RG1234", "IMAGENAME", "", "1.0.0"),
		},
		"AzureSharedGalleryImage - missing version": {
			expectedErrors: 1,
			image:          createTestSharedImage("SUB1243", "RG1234", "IMAGENAME", "GALLERY9876", ""),
		},
	}

	for _, tc := range testCases {
		g.Expect(ValidateImage(tc.image, field.NewPath("image"))).To(HaveLen(tc.expectedErrors))
	}
}

func TestMarketPlaceImageValid(t *testing.T) {
	g := NewWithT(t)

	testCases := map[string]struct {
		image          *Image
		expectedErrors int
	}{
		"AzureMarketplaceImage - fully specified": {
			expectedErrors: 0,
			image:          createTestMarketPlaceImage("PUB1234", "OFFER1234", "SKU1234", "1.0.0"),
		},
		"AzureMarketplaceImage - missing publisher": {
			expectedErrors: 1,
			image:          createTestMarketPlaceImage("", "OFFER1234", "SKU1234", "1.0.0"),
		},
		"AzureMarketplaceImage - missing offer": {
			expectedErrors: 1,
			image:          createTestMarketPlaceImage("PUB1234", "", "SKU1234", "1.0.0"),
		},
		"AzureMarketplaceImage - missing sku": {
			expectedErrors: 1,
			image:          createTestMarketPlaceImage("PUB1234", "OFFER1234", "", "1.0.0"),
		},
		"AzureMarketplaceImage - missing version": {
			expectedErrors: 1,
			image:          createTestMarketPlaceImage("PUB1234", "OFFER1234", "SKU1234", ""),
		},
	}

	for _, tc := range testCases {
		g.Expect(ValidateImage(tc.image, field.NewPath("image"))).To(HaveLen(tc.expectedErrors))
	}
}

func TestImageByIDValid(t *testing.T) {
	g := NewWithT(t)

	testCases := map[string]struct {
		image          *Image
		expectedErrors int
	}{
		"AzureImageByID - with id": {
			expectedErrors: 0,
			image:          createTestImageByID("ID1234"),
		},
		"AzureImageByID - missing ID": {
			expectedErrors: 1,
			image:          createTestImageByID(""),
		},
	}

	for _, tc := range testCases {
		g.Expect(ValidateImage(tc.image, field.NewPath("image"))).To(HaveLen(tc.expectedErrors))
	}
}

func createTestSharedImage(subscriptionID, resourceGroup, name, gallery, version string) *Image {
	return &Image{
		SharedGallery: &AzureSharedGalleryImage{
			SubscriptionID: subscriptionID,
			ResourceGroup:  resourceGroup,
			Name:           name,
			Gallery:        gallery,
			Version:        version,
		},
	}
}

func createTestMarketPlaceImage(publisher, offer, sku, version string) *Image {
	return &Image{
		Marketplace: &AzureMarketplaceImage{
			Publisher: publisher,
			Offer:     offer,
			SKU:       sku,
			Version:   version,
		},
	}
}

func createTestImageByID(imageID string) *Image {
	return &Image{
		ID: &imageID,
	}
}

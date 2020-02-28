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
	"encoding/json"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func TestImageRequired(t *testing.T) {
	type test struct {
		Image *runtime.RawExtension
	}

	extension := test{}

	errs := ValidateImage(extension.Image, field.NewPath("image"))
	if len(errs) != 1 {
		t.Errorf("unexpected number of errors, expected 1 but got %d", len(errs))
	}
	if errs[0].Type.String() != field.ErrorTypeRequired.String() {
		t.Errorf("unexpected field required error but go %s", errs[0].Type.String())
	}
	if errs[0].Field != "image" {
		t.Errorf("unexpected field name, expected image but got %s", errs[0].Field)
	}
	if errs[0].Detail == "" {
		t.Error("expected a non-empty error detail")
	}
}

func TestImageGroupValid(t *testing.T) {
	testCases := map[string]struct {
		image      *runtime.RawExtension
		validgroup bool
	}{
		"correct group name - v1alpha3": {
			validgroup: true,
			image:      &runtime.RawExtension{Raw: []byte(`{ "apiVersion": "infrastructure.cluster.x-k8s.io/v1alpha3", "kind": "AzureImageByID", "id": "12345ABCD" }`)},
		},
		"incorrect group name": {
			validgroup: false,
			image:      &runtime.RawExtension{Raw: []byte(`{ "apiVersion": "infrastructure.wrong.io/v1alpha3", "kind": "AzureImageByID", "id": "12345ABCD" }`)},
		},
	}

	for k, tc := range testCases {
		errs := ValidateImage(tc.image, field.NewPath("image"))

		if tc.validgroup {
			if len(errs) > 0 {
				t.Errorf("test case '%s' failed, expected no errors but got %d errors", k, len(errs))
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("test case '%s' failed, expected errors but got no errors", k)
			}
		}
	}
}

func TestImageKindValid(t *testing.T) {
	testCases := map[string]struct {
		image     *runtime.RawExtension
		validKind bool
	}{
		"correct kind - AzureMarketplaceImage": {
			validKind: true,
			image:     &runtime.RawExtension{Raw: []byte(`{ "apiVersion": "infrastructure.cluster.x-k8s.io/v1alpha3", "kind": "AzureMarketplaceImage","publisher": "pub1", "offer": "offer1", "sku": "sku1", "version": "0.0.1" }`)},
		},
		"correct kind - AzureSharedGalleryImage": {
			validKind: true,
			image:     &runtime.RawExtension{Raw: []byte(`{ "apiVersion": "infrastructure.cluster.x-k8s.io/v1alpha3", "kind": "AzureSharedGalleryImage", "subscriptionID": "sub1", "resourceGroup": "rg1", "gallery": "gal1", "name": "abcd", "version": "0.0.1" }`)},
		},
		"correct kind - AzureImageByID": {
			validKind: true,
			image:     &runtime.RawExtension{Raw: []byte(`{ "apiVersion": "infrastructure.cluster.x-k8s.io/v1alpha3", "kind": "AzureImageByID", "id": "12345ABCD" }`)},
		},
		"incorrect kind": {
			validKind: false,
			image:     &runtime.RawExtension{Raw: []byte(`{ "apiVersion": "infrastructure.wrong.io/v1alpha3", "kind": "AzureImageByID", "id": "12345ABCD" }`)},
		},
	}

	for k, tc := range testCases {
		errs := ValidateImage(tc.image, field.NewPath("image"))

		if tc.validKind {
			if len(errs) > 0 {
				t.Errorf("test case '%s' failed, expected no errors but got %d errors: %#v", k, len(errs), errs)
			}
		} else {
			if len(errs) == 0 {
				t.Errorf("test case '%s' failed, expected errors but got no errors", k)
			}
		}
	}
}

func TestSharedImageGalleryValid(t *testing.T) {
	testCases := map[string]struct {
		image          *AzureSharedGalleryImage
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

	for k, tc := range testCases {
		imageData, err := json.Marshal(tc.image)
		if err != nil {
			t.Errorf("test case '%s' encountered an unexpected error: %#v", k, err)
		}
		rawImage := &runtime.RawExtension{Raw: imageData}

		errs := ValidateImage(rawImage, field.NewPath("image"))

		if len(errs) != tc.expectedErrors {
			t.Errorf("test case '%s' failed, expected %d errors but got %d: %#v", k, tc.expectedErrors, len(errs), errs)
		}
	}
}

func TestMarketPlaceImageValid(t *testing.T) {
	testCases := map[string]struct {
		image          *AzureMarketplaceImage
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

	for k, tc := range testCases {
		imageData, err := json.Marshal(tc.image)
		if err != nil {
			t.Errorf("test case '%s' encountered an unexpected error: %#v", k, err)
		}
		rawImage := &runtime.RawExtension{Raw: imageData}

		errs := ValidateImage(rawImage, field.NewPath("image"))

		if len(errs) != tc.expectedErrors {
			t.Errorf("test case '%s' failed, expected %d errors but got %d: %#v", k, tc.expectedErrors, len(errs), errs)
		}
	}
}

func TestImageByIDValid(t *testing.T) {
	testCases := map[string]struct {
		image          *AzureImageByID
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

	for k, tc := range testCases {
		imageData, err := json.Marshal(tc.image)
		if err != nil {
			t.Errorf("test case '%s' encountered an unexpected error: %#v", k, err)
		}
		rawImage := &runtime.RawExtension{Raw: imageData}

		errs := ValidateImage(rawImage, field.NewPath("image"))

		if len(errs) != tc.expectedErrors {
			t.Errorf("test case '%s' failed, expected %d errors but got %d: %#v", k, tc.expectedErrors, len(errs), errs)
		}
	}
}

func createTestSharedImage(subscriptionID, resourceGroup, name, gallery, version string) *AzureSharedGalleryImage {
	image := &AzureSharedGalleryImage{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AzureSharedGalleryImage",
			APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
		},
		SubscriptionID: subscriptionID,
		ResourceGroup:  resourceGroup,
		Name:           name,
		Gallery:        gallery,
		Version:        version,
	}

	return image
}

func createTestMarketPlaceImage(publisher, offer, sku, version string) *AzureMarketplaceImage {
	image := &AzureMarketplaceImage{
		TypeMeta: metav1.TypeMeta{
			Kind:       AzureMarketplaceImageKind,
			APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
		},
		Publisher: publisher,
		Offer:     offer,
		SKU:       sku,
		Version:   version,
	}

	return image
}

func createTestImageByID(imageID string) *AzureImageByID {
	image := &AzureImageByID{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AzureImageByID",
			APIVersion: "infrastructure.cluster.x-k8s.io/v1alpha3",
		},
		ID: imageID,
	}

	return image
}

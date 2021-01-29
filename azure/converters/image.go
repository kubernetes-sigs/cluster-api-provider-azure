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

package converters

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
)

// ImageToSDK converts a CAPZ Image (as RawExtension) to a Azure SDK Image Reference.
func ImageToSDK(image *infrav1.Image) (*compute.ImageReference, error) {

	if image.ID != nil {
		return specificImageToSDK(image)
	}
	if image.Marketplace != nil {
		return mpImageToSDK(image)
	}
	if image.SharedGallery != nil {
		return sigImageToSDK(image)
	}

	return nil, errors.New("unable to convert image as no options set")

}

func mpImageToSDK(image *infrav1.Image) (*compute.ImageReference, error) {
	return &compute.ImageReference{
		Publisher: &image.Marketplace.Publisher,
		Offer:     &image.Marketplace.Offer,
		Sku:       &image.Marketplace.SKU,
		Version:   &image.Marketplace.Version,
	}, nil

}

func sigImageToSDK(image *infrav1.Image) (*compute.ImageReference, error) {
	imageID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/galleries/%s/images/%s/versions/%s",
		image.SharedGallery.SubscriptionID,
		image.SharedGallery.ResourceGroup,
		image.SharedGallery.Gallery,
		image.SharedGallery.Name,
		image.SharedGallery.Version)

	return &compute.ImageReference{
		ID: &imageID,
	}, nil
}

func specificImageToSDK(image *infrav1.Image) (*compute.ImageReference, error) {
	return &compute.ImageReference{
		ID: image.ID,
	}, nil
}

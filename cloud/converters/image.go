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

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"

	"k8s.io/apimachinery/pkg/runtime"
)

// ImageToSDK converts a CAPZ Image (as RawExtension) to a Azure SDK Image Reference.
func ImageToSDK(rawImage *runtime.RawExtension) (*compute.ImageReference, error) {
	unknown := new(runtime.Unknown)
	if err := infrav1.DecodeRawExtension(rawImage, unknown); err != nil {
		return nil, err
	}

	switch unknown.Kind {
	case infrav1.AzureMarketplaceImageKind:
		return mpImageToSDK(rawImage)
	case infrav1.AzureSharedGalleryImageKind:
		return sigImageToSDK(rawImage)
	case infrav1.AzureImageByIDKind:
		return specificImageToSDK(rawImage)
	default:
		return nil, fmt.Errorf("unknown image kind: %s", unknown.Kind)
	}
}

func mpImageToSDK(rawImage *runtime.RawExtension) (*compute.ImageReference, error) {
	image := &infrav1.AzureMarketplaceImage{}
	if err := infrav1.DecodeRawExtension(rawImage, image); err != nil {
		return nil, err
	}

	return &compute.ImageReference{
		Publisher: &image.Publisher,
		Offer:     &image.Offer,
		Sku:       &image.SKU,
		Version:   &image.Version,
	}, nil

}

func sigImageToSDK(rawImage *runtime.RawExtension) (*compute.ImageReference, error) {
	image := &infrav1.AzureSharedGalleryImage{}
	if err := infrav1.DecodeRawExtension(rawImage, image); err != nil {
		return nil, err
	}

	imageID := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/galleries/%s/images/%s/versions/%s", image.SubscriptionID, image.ResourceGroup, image.Gallery, image.Name, image.Version)

	return &compute.ImageReference{
		ID: &imageID,
	}, nil
}

func specificImageToSDK(rawImage *runtime.RawExtension) (*compute.ImageReference, error) {
	image := &infrav1.AzureImageByID{}
	if err := infrav1.DecodeRawExtension(rawImage, image); err != nil {
		return nil, err
	}

	return &compute.ImageReference{
		ID: &image.ID,
	}, nil
}

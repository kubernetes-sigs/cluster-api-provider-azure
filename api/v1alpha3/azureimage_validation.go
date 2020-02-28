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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var supportedAPIGroup = GroupVersion.Group
var supportedVersion = GroupVersion.Version
var supportedKinds = sets.NewString(
	AzureMarketplaceImageKind,
	AzureSharedGalleryImageKind,
	AzureImageByIDKind)

// ValidateImage validates an image
func ValidateImage(rawImage *runtime.RawExtension, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if rawImage == nil || rawImage.Raw == nil {
		allErrs = append(allErrs, field.Required(fldPath, "an image must be specified"))
		return allErrs
	}

	unknown := new(runtime.Unknown)
	if err := DecodeRawExtension(rawImage, unknown); err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, err))
		return allErrs
	}

	if unknown.GroupVersionKind().Group != supportedAPIGroup {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("apiVersion"), unknown.GroupVersionKind().Group, []string{supportedAPIGroup}))
	}

	if unknown.GroupVersionKind().Version != supportedVersion {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("apiVersion"), unknown.GroupVersionKind().Version, []string{supportedVersion}))
	}

	if !supportedKinds.Has(unknown.Kind) {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("kind"), unknown.Kind, supportedKinds.List()))
	}

	switch unknown.Kind {
	case AzureMarketplaceImageKind:
		allErrs = append(allErrs, validateMarketplaceImage(rawImage, fldPath)...)
	case AzureSharedGalleryImageKind:
		allErrs = append(allErrs, validateSharedGalleryImage(rawImage, fldPath)...)
	case AzureImageByIDKind:
		allErrs = append(allErrs, validateSpecifcImage(rawImage, fldPath)...)
	}

	return allErrs
}

func validateSharedGalleryImage(rawImage *runtime.RawExtension, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	image := &AzureSharedGalleryImage{}
	if err := DecodeRawExtension(rawImage, image); err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, err))
		return allErrs
	}

	if image.SubscriptionID == "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("SubscriptionID"), "", "SubscriptionID cannot be empty when specifying an AzureSharedGalleryImage"))
	}
	if image.ResourceGroup == "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("ResourceGroup"), "", "ResourceGroup cannot be empty when specifying an AzureSharedGalleryImage"))
	}
	if image.Gallery == "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("Gallery"), "", "Gallery cannot be empty when specifying an AzureSharedGalleryImage"))
	}
	if image.Name == "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("Name"), "", "Name cannot be empty when specifying an AzureSharedGalleryImage"))
	}
	if image.Version == "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("Version"), "", "Version cannot be empty when specifying an AzureSharedGalleryImage"))
	}

	return allErrs
}

func validateMarketplaceImage(rawImage *runtime.RawExtension, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	image := &AzureMarketplaceImage{}
	if err := DecodeRawExtension(rawImage, image); err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, err))
		return allErrs
	}

	if image.Publisher == "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("Publisher"), "", "Publisher cannot be empty when specifying an AzureMarketplaceImage"))
	}
	if image.Offer == "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("Offer"), "", "Offer cannot be empty when specifying an AzureMarketplaceImage"))
	}
	if image.SKU == "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("SKU"), "", "SKU cannot be empty when specifying an AzureMarketplaceImage"))
	}
	if image.Version == "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("Version"), "", "Version cannot be empty when specifying an AzureMarketplaceImage"))
	}
	return allErrs
}

func validateSpecifcImage(rawImage *runtime.RawExtension, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	image := &AzureImageByID{}
	if err := DecodeRawExtension(rawImage, image); err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, err))
		return allErrs
	}

	if image.ID == "" {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("ID"), "", "ID cannot be empty when specifying an AzureImageByID"))
	}

	return allErrs
}

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

package v1alpha2

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	infrav1alpha3 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this AzureMachine to the Hub version (v1alpha3).
func (src *AzureMachine) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha3.AzureMachine)

	if err := Convert_v1alpha2_AzureMachine_To_v1alpha3_AzureMachine(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data from annotations
	restored := &infrav1alpha3.AzureMachine{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha3) to this version.
func (dst *AzureMachine) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha3.AzureMachine)
	if err := Convert_v1alpha3_AzureMachine_To_v1alpha2_AzureMachine(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	return nil
}

// ConvertTo converts this AzureMachineList to the Hub version (v1alpha3).
func (src *AzureMachineList) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha3.AzureMachineList)
	return Convert_v1alpha2_AzureMachineList_To_v1alpha3_AzureMachineList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1alpha3) to this version.
func (dst *AzureMachineList) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha3.AzureMachineList)
	return Convert_v1alpha3_AzureMachineList_To_v1alpha2_AzureMachineList(src, dst, nil)
}

func Convert_v1alpha2_AzureMachineSpec_To_v1alpha3_AzureMachineSpec(in *AzureMachineSpec, out *infrav1alpha3.AzureMachineSpec, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha2_AzureMachineSpec_To_v1alpha3_AzureMachineSpec(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha3_AzureMachineSpec_To_v1alpha2_AzureMachineSpec converts from the Hub version (v1alpha3) of the AzureMachineSpec to this version.
func Convert_v1alpha3_AzureMachineSpec_To_v1alpha2_AzureMachineSpec(in *infrav1alpha3.AzureMachineSpec, out *AzureMachineSpec, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha3_AzureMachineSpec_To_v1alpha2_AzureMachineSpec(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha2_AzureMachineStatus_To_v1alpha3_AzureMachineStatus converts this AzureMachineStatus to the Hub version (v1alpha3).
func Convert_v1alpha2_AzureMachineStatus_To_v1alpha3_AzureMachineStatus(in *AzureMachineStatus, out *infrav1alpha3.AzureMachineStatus, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha2_AzureMachineStatus_To_v1alpha3_AzureMachineStatus(in, out, s); err != nil {
		return err
	}

	// Manually convert the Error fields to the Failure fields
	out.FailureMessage = in.ErrorMessage
	out.FailureReason = in.ErrorReason

	return nil
}

// Convert_v1alpha3_AzureMachineStatus_To_v1alpha2_AzureMachineStatus converts from the Hub version (v1alpha3) of the AzureMachineStatus to this version.
func Convert_v1alpha3_AzureMachineStatus_To_v1alpha2_AzureMachineStatus(in *infrav1alpha3.AzureMachineStatus, out *AzureMachineStatus, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha3_AzureMachineStatus_To_v1alpha2_AzureMachineStatus(in, out, s); err != nil {
		return err
	}

	// Manually convert the Failure fields to the Error fields
	out.ErrorMessage = in.FailureMessage
	out.ErrorReason = in.FailureReason

	return nil
}

// Convert_v1alpha2_Image_To_runtime_RawExtension converts from an Image to RawExtension
func Convert_v1alpha2_Image_To_runtime_RawExtension(in *Image, out *runtime.RawExtension, s apiconversion.Scope) error { // nolint
	if isAzureMarketPlaceImage(in) {
		image := infrav1alpha3.AzureMarketplaceImage{
			TypeMeta: metav1.TypeMeta{
				Kind:       infrav1alpha3.AzureMarketplaceImageKind,
				APIVersion: infrav1alpha3.GroupVersion.String(),
			},
			Publisher: *in.Publisher,
			Offer:     *in.Offer,
			SKU:       *in.SKU,
			Version:   *in.Version,
		}
		return convertImageToRawExtension(image, out)

	}
	if isSharedGalleryImage(in) {
		image := infrav1alpha3.AzureSharedGalleryImage{
			TypeMeta: metav1.TypeMeta{
				Kind:       infrav1alpha3.AzureSharedGalleryImageKind,
				APIVersion: infrav1alpha3.GroupVersion.String(),
			},
			SubscriptionID: *in.SubscriptionID,
			ResourceGroup:  *in.ResourceGroup,
			Gallery:        *in.Gallery,
			Name:           *in.Name,
			Version:        *in.Version,
		}
		return convertImageToRawExtension(image, out)
	}
	if isImageByID(in) {
		image := infrav1alpha3.AzureImageByID{
			TypeMeta: metav1.TypeMeta{
				Kind:       infrav1alpha3.AzureImageByIDKind,
				APIVersion: infrav1alpha3.GroupVersion.String(),
			},
			ID: *in.ID,
		}
		return convertImageToRawExtension(image, out)
	}

	return errors.New("cannot determine image type for conversion")
}

// Convert_runtime_RawExtension_To_v1alpha2_Image converts from RawExtension to Image
func Convert_runtime_RawExtension_To_v1alpha2_Image(in *runtime.RawExtension, out *Image, s apiconversion.Scope) error { // nolint

	unknown := new(runtime.Unknown)

	if err := infrav1alpha3.DecodeRawExtension(in, unknown); err != nil {
		return errors.Wrap(err, "unable to decode rawextension to unknown")
	}

	switch unknown.Kind {
	case infrav1alpha3.AzureMarketplaceImageKind:
		return mpImageToImage(in, out)
	case infrav1alpha3.AzureSharedGalleryImageKind:
		return sigImageToImage(in, out)
	case infrav1alpha3.AzureImageByIDKind:
		return specificImageToImage(in, out)
	default:
		return fmt.Errorf("unknown image kind: %s", unknown.Kind)
	}
}

func isAzureMarketPlaceImage(in *Image) bool {
	if in.Publisher == nil || in.Offer == nil || in.SKU == nil || in.Version == nil {
		return false
	}

	if len(*in.Publisher) == 0 || len(*in.Offer) == 0 || len(*in.SKU) == 0 || len(*in.Version) == 0 {
		return false
	}

	return true
}

func isSharedGalleryImage(in *Image) bool {
	if in.SubscriptionID == nil || in.ResourceGroup == nil || in.Gallery == nil || in.Version == nil || in.Name == nil {
		return false
	}

	if len(*in.SubscriptionID) == 0 || len(*in.ResourceGroup) == 0 || len(*in.Gallery) == 0 || len(*in.Version) == 0 || len(*in.Name) == 0 {
		return false
	}

	return true
}

func isImageByID(in *Image) bool {
	return in.ID != nil && len(*in.ID) > 0
}

func convertImageToRawExtension(image interface{}, out *runtime.RawExtension) error {
	imageData, err := json.Marshal(image)
	if err != nil {
		return errors.Wrap(err, "error marshalling image kind to json")
	}

	out.Raw = imageData
	return nil
}

func mpImageToImage(rawImage *runtime.RawExtension, out *Image) error {
	image := &infrav1alpha3.AzureMarketplaceImage{}

	if err := infrav1alpha3.DecodeRawExtension(rawImage, image); err != nil {
		return errors.Wrap(err, "failed decoding image to AzureMarketplaceImage")
	}

	out.Publisher = &image.Publisher
	out.Offer = &image.Offer
	out.SKU = &image.SKU
	out.Version = &image.Version

	return nil
}

func sigImageToImage(rawImage *runtime.RawExtension, out *Image) error {
	image := &infrav1alpha3.AzureSharedGalleryImage{}

	if err := infrav1alpha3.DecodeRawExtension(rawImage, image); err != nil {
		return errors.Wrap(err, "failed decoding image to AzureSharedGalleryImage")
	}

	out.SubscriptionID = &image.SubscriptionID
	out.ResourceGroup = &image.ResourceGroup
	out.Gallery = &image.Gallery
	out.Version = &image.Version
	out.Name = &image.Name

	return nil
}

func specificImageToImage(rawImage *runtime.RawExtension, out *Image) error {
	image := &infrav1alpha3.AzureImageByID{}

	if err := infrav1alpha3.DecodeRawExtension(rawImage, image); err != nil {
		return errors.Wrap(err, "failed decoding image to AzureImageByID")
	}

	out.ID = &image.ID

	return nil
}

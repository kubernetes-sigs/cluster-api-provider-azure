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
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	infrav1alpha3 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	v1alpha3 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
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

	restoreAzureMachineSpec(&restored.Spec, &dst.Spec)

	// Manual conversion for conditions
	dst.SetConditions(restored.GetConditions())

	return nil
}

func restoreAzureMachineSpec(restored, dst *infrav1alpha3.AzureMachineSpec) {
	if restored.Identity != "" {
		dst.Identity = restored.Identity
	}
	if len(restored.UserAssignedIdentities) > 0 {
		dst.UserAssignedIdentities = restored.UserAssignedIdentities
	}
	dst.RoleAssignmentName = restored.RoleAssignmentName
	if restored.AcceleratedNetworking != nil {
		dst.AcceleratedNetworking = restored.AcceleratedNetworking
	}
	dst.FailureDomain = restored.FailureDomain
	dst.EnableIPForwarding = restored.EnableIPForwarding
	if restored.SpotVMOptions != nil {
		dst.SpotVMOptions = restored.SpotVMOptions.DeepCopy()
	}
	if restored.SecurityProfile != nil {
		dst.SecurityProfile = restored.SecurityProfile.DeepCopy()
	}
	if len(restored.DataDisks) != 0 {
		dst.DataDisks = restored.DataDisks
	}
	dst.OSDisk.DiffDiskSettings = restored.OSDisk.DiffDiskSettings
	dst.OSDisk.CachingType = restored.OSDisk.CachingType
	if restored.OSDisk.ManagedDisk.DiskEncryptionSet != nil {
		dst.OSDisk.ManagedDisk.DiskEncryptionSet = restored.OSDisk.ManagedDisk.DiskEncryptionSet.DeepCopy()
	}

	if restored.Image != nil && restored.Image.Marketplace != nil {
		dst.Image.Marketplace.ThirdPartyImage = restored.Image.Marketplace.ThirdPartyImage
	}
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

// Convert_v1alpha2_Image_To_v1alpha3_Image converts from an Images between v1alpha2 and v1alpha3
func Convert_v1alpha2_Image_To_v1alpha3_Image(in *Image, out *infrav1alpha3.Image, s apiconversion.Scope) error { //nolint
	if isImageByID(in) {
		out.ID = in.ID
		return nil
	}
	if isAzureMarketPlaceImage(in) {
		out.Marketplace = &infrav1alpha3.AzureMarketplaceImage{
			Publisher: *in.Publisher,
			Offer:     *in.Offer,
			SKU:       *in.SKU,
			Version:   *in.Version,
		}
		return nil
	}
	if isSharedGalleryImage(in) {
		out.SharedGallery = &infrav1alpha3.AzureSharedGalleryImage{
			SubscriptionID: *in.SubscriptionID,
			ResourceGroup:  *in.ResourceGroup,
			Gallery:        *in.Gallery,
			Name:           *in.Name,
			Version:        *in.Version,
		}
		return nil
	}
	return nil
}

// Convert_v1alpha3_Image_To_v1alpha2_Image converts Images from v1alpha3 to v1alpha2
func Convert_v1alpha3_Image_To_v1alpha2_Image(in *infrav1alpha3.Image, out *Image, s apiconversion.Scope) error { // nolint
	if in.ID != nil {
		out.ID = in.ID
		return nil
	}
	if in.Marketplace != nil {
		out.Publisher = &in.Marketplace.Publisher
		out.Offer = &in.Marketplace.Offer
		out.SKU = &in.Marketplace.SKU
		out.Version = &in.Marketplace.Version
		return nil
	}
	if in.SharedGallery != nil {
		out.SubscriptionID = &in.SharedGallery.SubscriptionID
		out.ResourceGroup = &in.SharedGallery.ResourceGroup
		out.Gallery = &in.SharedGallery.Gallery
		out.Version = &in.SharedGallery.Version
		out.Name = &in.SharedGallery.Name
		return nil
	}
	return nil
}

// Convert_v1alpha3_OSDisk_To_v1alpha2_OSDisk converts between api versions
func Convert_v1alpha3_OSDisk_To_v1alpha2_OSDisk(in *v1alpha3.OSDisk, out *OSDisk, s apiconversion.Scope) error {
	return autoConvert_v1alpha3_OSDisk_To_v1alpha2_OSDisk(in, out, s)
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

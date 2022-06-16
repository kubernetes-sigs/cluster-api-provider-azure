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

package v1alpha3

import (
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	"sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this AzureMachine to the Hub version (v1beta1).
func (src *AzureMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.AzureMachine)
	if err := Convert_v1alpha3_AzureMachine_To_v1beta1_AzureMachine(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data from annotations
	restored := &v1beta1.AzureMachine{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	// Handle special case for conversion of ManagedDisk to pointer.
	if restored.Spec.OSDisk.ManagedDisk == nil && dst.Spec.OSDisk.ManagedDisk != nil {
		if *dst.Spec.OSDisk.ManagedDisk == (v1beta1.ManagedDiskParameters{}) {
			// restore nil value if nothing has changed since conversion
			dst.Spec.OSDisk.ManagedDisk = nil
		}
	}

	if restored.Spec.Image != nil && restored.Spec.Image.SharedGallery != nil {
		dst.Spec.Image.SharedGallery.Offer = restored.Spec.Image.SharedGallery.Offer
		dst.Spec.Image.SharedGallery.Publisher = restored.Spec.Image.SharedGallery.Publisher
		dst.Spec.Image.SharedGallery.SKU = restored.Spec.Image.SharedGallery.SKU
	}

	if dst.Spec.Image != nil && restored.Spec.Image.ComputeGallery != nil {
		dst.Spec.Image.ComputeGallery = restored.Spec.Image.ComputeGallery
	}

	dst.Spec.SubnetName = restored.Spec.SubnetName

	dst.Status.LongRunningOperationStates = restored.Status.LongRunningOperationStates

	return nil
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.AzureMachine)
	if err := Convert_v1beta1_AzureMachine_To_v1alpha3_AzureMachine(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	return utilconversion.MarshalData(src, dst)
}

// ConvertTo converts this AzureMachineList to the Hub version (v1beta1).
func (src *AzureMachineList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.AzureMachineList)
	return Convert_v1alpha3_AzureMachineList_To_v1beta1_AzureMachineList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureMachineList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.AzureMachineList)
	return Convert_v1beta1_AzureMachineList_To_v1alpha3_AzureMachineList(src, dst, nil)
}

func Convert_v1alpha3_AzureMachineSpec_To_v1beta1_AzureMachineSpec(in *AzureMachineSpec, out *v1beta1.AzureMachineSpec, s apiconversion.Scope) error {
	return autoConvert_v1alpha3_AzureMachineSpec_To_v1beta1_AzureMachineSpec(in, out, s)
}

// Convert_v1beta1_AzureMachineSpec_To_v1alpha3_AzureMachineSpec converts from the Hub version (v1beta1) of the AzureMachineSpec to this version.
func Convert_v1beta1_AzureMachineSpec_To_v1alpha3_AzureMachineSpec(in *v1beta1.AzureMachineSpec, out *AzureMachineSpec, s apiconversion.Scope) error {
	return autoConvert_v1beta1_AzureMachineSpec_To_v1alpha3_AzureMachineSpec(in, out, s)
}

// Convert_v1alpha3_AzureMachineStatus_To_v1beta1_AzureMachineStatus converts this AzureMachineStatus to the Hub version (v1beta1).
func Convert_v1alpha3_AzureMachineStatus_To_v1beta1_AzureMachineStatus(in *AzureMachineStatus, out *v1beta1.AzureMachineStatus, s apiconversion.Scope) error {
	return autoConvert_v1alpha3_AzureMachineStatus_To_v1beta1_AzureMachineStatus(in, out, s)
}

// Convert_v1beta1_AzureMachineStatus_To_v1alpha3_AzureMachineStatus converts from the Hub version (v1beta1) of the AzureMachineStatus to this version.
func Convert_v1beta1_AzureMachineStatus_To_v1alpha3_AzureMachineStatus(in *v1beta1.AzureMachineStatus, out *AzureMachineStatus, s apiconversion.Scope) error {
	return autoConvert_v1beta1_AzureMachineStatus_To_v1alpha3_AzureMachineStatus(in, out, s)
}

// Convert_v1alpha3_OSDisk_To_v1beta1_OSDisk converts this OSDisk to the Hub version (v1beta1).
func Convert_v1alpha3_OSDisk_To_v1beta1_OSDisk(in *OSDisk, out *v1beta1.OSDisk, s apiconversion.Scope) error {
	out.OSType = in.OSType
	if in.DiskSizeGB != 0 {
		out.DiskSizeGB = &in.DiskSizeGB
	}
	out.DiffDiskSettings = (*v1beta1.DiffDiskSettings)(in.DiffDiskSettings)
	out.CachingType = in.CachingType
	out.ManagedDisk = &v1beta1.ManagedDiskParameters{}

	return Convert_v1alpha3_ManagedDisk_To_v1beta1_ManagedDiskParameters(&in.ManagedDisk, out.ManagedDisk, s)
}

// Convert_v1beta1_OSDisk_To_v1alpha3_OSDisk converts from the Hub version (v1beta1) of the AzureMachineStatus to this version.
func Convert_v1beta1_OSDisk_To_v1alpha3_OSDisk(in *v1beta1.OSDisk, out *OSDisk, s apiconversion.Scope) error {
	out.OSType = in.OSType
	if in.DiskSizeGB != nil {
		out.DiskSizeGB = *in.DiskSizeGB
	}
	out.DiffDiskSettings = (*DiffDiskSettings)(in.DiffDiskSettings)
	out.CachingType = in.CachingType

	if in.ManagedDisk != nil {
		out.ManagedDisk = ManagedDisk{}
		if err := Convert_v1beta1_ManagedDiskParameters_To_v1alpha3_ManagedDisk(in.ManagedDisk, &out.ManagedDisk, s); err != nil {
			return err
		}
	}

	return nil
}

// Convert_v1alpha3_ManagedDisk_To_v1beta1_ManagedDiskParameters converts this ManagedDisk to the Hub version (v1beta1).
func Convert_v1alpha3_ManagedDisk_To_v1beta1_ManagedDiskParameters(in *ManagedDisk, out *v1beta1.ManagedDiskParameters, s apiconversion.Scope) error {
	out.StorageAccountType = in.StorageAccountType
	out.DiskEncryptionSet = (*v1beta1.DiskEncryptionSetParameters)(in.DiskEncryptionSet)
	return nil
}

// Convert_v1beta1_ManagedDiskParameters_To_v1alpha3_ManagedDisk converts from the Hub version (v1beta1) of the ManagedDiskParameters to this version.
func Convert_v1beta1_ManagedDiskParameters_To_v1alpha3_ManagedDisk(in *v1beta1.ManagedDiskParameters, out *ManagedDisk, s apiconversion.Scope) error {
	out.StorageAccountType = in.StorageAccountType
	out.DiskEncryptionSet = (*DiskEncryptionSetParameters)(in.DiskEncryptionSet)
	return nil
}

func Convert_v1beta1_AzureMarketplaceImage_To_v1alpha3_AzureMarketplaceImage(in *v1beta1.AzureMarketplaceImage, out *AzureMarketplaceImage, s apiconversion.Scope) error {
	out.Offer = in.ImagePlan.Offer
	out.Publisher = in.ImagePlan.Publisher
	out.SKU = in.ImagePlan.SKU

	return autoConvert_v1beta1_AzureMarketplaceImage_To_v1alpha3_AzureMarketplaceImage(in, out, s)
}

func Convert_v1alpha3_AzureMarketplaceImage_To_v1beta1_AzureMarketplaceImage(in *AzureMarketplaceImage, out *v1beta1.AzureMarketplaceImage, s apiconversion.Scope) error {
	out.ImagePlan.Offer = in.Offer
	out.ImagePlan.Publisher = in.Publisher
	out.ImagePlan.SKU = in.SKU

	return autoConvert_v1alpha3_AzureMarketplaceImage_To_v1beta1_AzureMarketplaceImage(in, out, s)
}

func Convert_v1beta1_Image_To_v1alpha3_Image(in *v1beta1.Image, out *Image, s apiconversion.Scope) error {
	return autoConvert_v1beta1_Image_To_v1alpha3_Image(in, out, s)
}

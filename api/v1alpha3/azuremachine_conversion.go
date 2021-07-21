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
	"sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this AzureMachine to the Hub version (v1alpha4).
func (src *AzureMachine) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*v1alpha4.AzureMachine)

	if err := Convert_v1alpha3_AzureMachine_To_v1alpha4_AzureMachine(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data from annotations
	restored := &v1alpha4.AzureMachine{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	// Handle special case for conversion of ManagedDisk to pointer.
	if restored.Spec.OSDisk.ManagedDisk == nil && dst.Spec.OSDisk.ManagedDisk != nil {
		if *dst.Spec.OSDisk.ManagedDisk == (v1alpha4.ManagedDiskParameters{}) {
			// restore nil value if nothing has changed since conversion
			dst.Spec.OSDisk.ManagedDisk = nil
		}
	}

	dst.Spec.SubnetName = restored.Spec.SubnetName

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha4) to this version.
func (dst *AzureMachine) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*v1alpha4.AzureMachine)
	if err := Convert_v1alpha4_AzureMachine_To_v1alpha3_AzureMachine(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	return nil
}

// ConvertTo converts this AzureMachineList to the Hub version (v1alpha4).
func (src *AzureMachineList) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*v1alpha4.AzureMachineList)
	return Convert_v1alpha3_AzureMachineList_To_v1alpha4_AzureMachineList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1alpha4) to this version.
func (dst *AzureMachineList) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*v1alpha4.AzureMachineList)
	return Convert_v1alpha4_AzureMachineList_To_v1alpha3_AzureMachineList(src, dst, nil)
}

func Convert_v1alpha3_AzureMachineSpec_To_v1alpha4_AzureMachineSpec(in *AzureMachineSpec, out *v1alpha4.AzureMachineSpec, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha3_AzureMachineSpec_To_v1alpha4_AzureMachineSpec(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha4_AzureMachineSpec_To_v1alpha3_AzureMachineSpec converts from the Hub version (v1alpha4) of the AzureMachineSpec to this version.
func Convert_v1alpha4_AzureMachineSpec_To_v1alpha3_AzureMachineSpec(in *v1alpha4.AzureMachineSpec, out *AzureMachineSpec, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha4_AzureMachineSpec_To_v1alpha3_AzureMachineSpec(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha3_AzureMachineStatus_To_v1alpha4_AzureMachineStatus converts this AzureMachineStatus to the Hub version (v1alpha4).
func Convert_v1alpha3_AzureMachineStatus_To_v1alpha4_AzureMachineStatus(in *AzureMachineStatus, out *v1alpha4.AzureMachineStatus, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha3_AzureMachineStatus_To_v1alpha4_AzureMachineStatus(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha4_AzureMachineStatus_To_v1alpha3_AzureMachineStatus converts from the Hub version (v1alpha4) of the AzureMachineStatus to this version.
func Convert_v1alpha4_AzureMachineStatus_To_v1alpha3_AzureMachineStatus(in *v1alpha4.AzureMachineStatus, out *AzureMachineStatus, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha4_AzureMachineStatus_To_v1alpha3_AzureMachineStatus(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha3_OSDisk_To_v1alpha4_OSDisk converts this OSDisk to the Hub version (v1alpha4).
func Convert_v1alpha3_OSDisk_To_v1alpha4_OSDisk(in *OSDisk, out *v1alpha4.OSDisk, s apiconversion.Scope) error { // nolint
	out.OSType = in.OSType
	if in.DiskSizeGB != 0 {
		out.DiskSizeGB = &in.DiskSizeGB
	}
	out.DiffDiskSettings = (*v1alpha4.DiffDiskSettings)(in.DiffDiskSettings)
	out.CachingType = in.CachingType
	out.ManagedDisk = &v1alpha4.ManagedDiskParameters{}

	if err := Convert_v1alpha3_ManagedDisk_To_v1alpha4_ManagedDiskParameters(&in.ManagedDisk, out.ManagedDisk, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha4_OSDisk_To_v1alpha3_OSDisk converts from the Hub version (v1alpha4) of the AzureMachineStatus to this version.
func Convert_v1alpha4_OSDisk_To_v1alpha3_OSDisk(in *v1alpha4.OSDisk, out *OSDisk, s apiconversion.Scope) error { // nolint
	out.OSType = in.OSType
	if in.DiskSizeGB != nil {
		out.DiskSizeGB = *in.DiskSizeGB
	}
	out.DiffDiskSettings = (*DiffDiskSettings)(in.DiffDiskSettings)
	out.CachingType = in.CachingType

	if in.ManagedDisk != nil {
		out.ManagedDisk = ManagedDisk{}
		if err := Convert_v1alpha4_ManagedDiskParameters_To_v1alpha3_ManagedDisk(in.ManagedDisk, &out.ManagedDisk, s); err != nil {
			return err
		}
	}

	return nil
}

// Convert_v1alpha3_ManagedDisk_To_v1alpha4_ManagedDiskParameters converts this ManagedDisk to the Hub version (v1alpha4).
func Convert_v1alpha3_ManagedDisk_To_v1alpha4_ManagedDiskParameters(in *ManagedDisk, out *v1alpha4.ManagedDiskParameters, s apiconversion.Scope) error { // nolint
	out.StorageAccountType = in.StorageAccountType
	out.DiskEncryptionSet = (*v1alpha4.DiskEncryptionSetParameters)(in.DiskEncryptionSet)
	return nil
}

// Convert_v1alpha4_ManagedDiskParameters_To_v1alpha3_ManagedDisk converts from the Hub version (v1alpha4) of the ManagedDiskParameters to this version.
func Convert_v1alpha4_ManagedDiskParameters_To_v1alpha3_ManagedDisk(in *v1alpha4.ManagedDiskParameters, out *ManagedDisk, s apiconversion.Scope) error { // nolint
	out.StorageAccountType = in.StorageAccountType
	out.DiskEncryptionSet = (*DiskEncryptionSetParameters)(in.DiskEncryptionSet)
	return nil
}

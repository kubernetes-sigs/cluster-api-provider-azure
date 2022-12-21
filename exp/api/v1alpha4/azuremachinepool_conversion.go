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

package v1alpha4

import (
	unsafe "unsafe"

	"k8s.io/apimachinery/pkg/api/resource"
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	infrav1alpha4 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this AzureMachinePool to the Hub version (v1beta1).
func (src *AzureMachinePool) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1exp.AzureMachinePool)
	if err := Convert_v1alpha4_AzureMachinePool_To_v1beta1_AzureMachinePool(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data.
	restored := &infrav1exp.AzureMachinePool{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	if restored.Spec.Template.NetworkInterfaces != nil {
		dst.Spec.Template.NetworkInterfaces = restored.Spec.Template.NetworkInterfaces
	}

	if restored.Spec.Template.Image != nil && restored.Spec.Template.Image.ComputeGallery != nil {
		dst.Spec.Template.Image.ComputeGallery = restored.Spec.Template.Image.ComputeGallery
	}

	if restored.Status.Image != nil && restored.Status.Image.ComputeGallery != nil {
		dst.Status.Image.ComputeGallery = restored.Status.Image.ComputeGallery
	}

	if len(restored.Spec.Template.VMExtensions) > 0 {
		dst.Spec.Template.VMExtensions = restored.Spec.Template.VMExtensions
	}

	if restored.Spec.Template.SpotVMOptions != nil && restored.Spec.Template.SpotVMOptions.EvictionPolicy != nil {
		dst.Spec.Template.SpotVMOptions.EvictionPolicy = restored.Spec.Template.SpotVMOptions.EvictionPolicy
	}

	if restored.Spec.Template.Diagnostics != nil {
		dst.Spec.Template.Diagnostics = restored.Spec.Template.Diagnostics
	}

	for i, r := range restored.Status.LongRunningOperationStates {
		if r.Name == dst.Status.LongRunningOperationStates[i].Name {
			dst.Status.LongRunningOperationStates[i].ServiceName = r.ServiceName
		}
	}

	// Restore orchestration mode
	dst.Spec.OrchestrationMode = restored.Spec.OrchestrationMode

	if restored.Spec.SystemAssignedIdentityRole != nil {
		dst.Spec.SystemAssignedIdentityRole = restored.Spec.SystemAssignedIdentityRole
	}

	return nil
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureMachinePool) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1exp.AzureMachinePool)
	if err := Convert_v1beta1_AzureMachinePool_To_v1alpha4_AzureMachinePool(src, dst, nil); err != nil {
		return err
	}

	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	return utilconversion.MarshalData(src, dst)
}

// Convert_v1beta1_AzureMachinePoolSpec_To_v1alpha4_AzureMachinePoolSpec converts a v1beta1 AzureMachinePool.Spec to a v1alpha4 AzureMachinePool.Spec.
func Convert_v1beta1_AzureMachinePoolSpec_To_v1alpha4_AzureMachinePoolSpec(in *infrav1exp.AzureMachinePoolSpec, out *AzureMachinePoolSpec, s apiconversion.Scope) error {
	return autoConvert_v1beta1_AzureMachinePoolSpec_To_v1alpha4_AzureMachinePoolSpec(in, out, s)
}

// ConvertTo converts this AzureMachinePool to the Hub version (v1beta1).
func (src *AzureMachinePoolList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1exp.AzureMachinePoolList)
	return Convert_v1alpha4_AzureMachinePoolList_To_v1beta1_AzureMachinePoolList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureMachinePoolList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1exp.AzureMachinePoolList)
	return Convert_v1beta1_AzureMachinePoolList_To_v1alpha4_AzureMachinePoolList(src, dst, nil)
}

// Convert_v1beta1_AzureMachinePoolMachineTemplate_To_v1alpha4_AzureMachinePoolMachineTemplate converts an Azure Machine Pool Machine Template from v1beta1 to v1alpha4.
func Convert_v1beta1_AzureMachinePoolMachineTemplate_To_v1alpha4_AzureMachinePoolMachineTemplate(in *infrav1exp.AzureMachinePoolMachineTemplate, out *AzureMachinePoolMachineTemplate, s apiconversion.Scope) error {
	return autoConvert_v1beta1_AzureMachinePoolMachineTemplate_To_v1alpha4_AzureMachinePoolMachineTemplate(in, out, s)
}

// Convert_v1beta1_SpotVMOptions_To_v1alpha4_SpotVMOptions converts a SpotVMOptions from v1beta1 to v1alpha4.
func Convert_v1beta1_SpotVMOptions_To_v1alpha4_SpotVMOptions(in *infrav1.SpotVMOptions, out *infrav1alpha4.SpotVMOptions, s apiconversion.Scope) error {
	out.MaxPrice = (*resource.Quantity)(unsafe.Pointer(in.MaxPrice))
	return nil
}

// Convert_v1alpha4_SpotVMOptions_To_v1beta1_SpotVMOptions converts a SpotVMOptions from v1alpha4 to v1beta1.
func Convert_v1alpha4_SpotVMOptions_To_v1beta1_SpotVMOptions(in *infrav1alpha4.SpotVMOptions, out *infrav1.SpotVMOptions, s apiconversion.Scope) error {
	out.MaxPrice = (*resource.Quantity)(unsafe.Pointer(in.MaxPrice))
	return nil
}

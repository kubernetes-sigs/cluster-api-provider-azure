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
	convert "k8s.io/apimachinery/pkg/conversion"
	infrav1alpha3 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this AzureMachinePool to the Hub version (v1beta1).
func (src *AzureMachinePool) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1exp.AzureMachinePool)
	if err := Convert_v1alpha3_AzureMachinePool_To_v1beta1_AzureMachinePool(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data.
	restored := &infrav1exp.AzureMachinePool{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	// Handle special case for conversion of ManagedDisk to pointer.
	if restored.Spec.Template.OSDisk.ManagedDisk == nil && dst.Spec.Template.OSDisk.ManagedDisk != nil {
		if *dst.Spec.Template.OSDisk.ManagedDisk == (infrav1.ManagedDiskParameters{}) {
			// restore nil value if nothing has changed since conversion
			dst.Spec.Template.OSDisk.ManagedDisk = nil
		}
	}

	dst.Spec.Template.SubnetName = restored.Spec.Template.SubnetName

	dst.Spec.Strategy.Type = restored.Spec.Strategy.Type
	if restored.Spec.Strategy.RollingUpdate != nil {
		if dst.Spec.Strategy.RollingUpdate == nil {
			dst.Spec.Strategy.RollingUpdate = &infrav1exp.MachineRollingUpdateDeployment{}
		}

		dst.Spec.Strategy.RollingUpdate.DeletePolicy = restored.Spec.Strategy.RollingUpdate.DeletePolicy
	}

	if restored.Spec.NodeDrainTimeout != nil {
		dst.Spec.NodeDrainTimeout = restored.Spec.NodeDrainTimeout
	}

	if restored.Status.Image != nil {
		dst.Status.Image = restored.Status.Image
	}

	if restored.Spec.Template.Image != nil && restored.Spec.Template.Image.SharedGallery != nil {
		dst.Spec.Template.Image.SharedGallery.Offer = restored.Spec.Template.Image.SharedGallery.Offer
		dst.Spec.Template.Image.SharedGallery.Publisher = restored.Spec.Template.Image.SharedGallery.Publisher
		dst.Spec.Template.Image.SharedGallery.SKU = restored.Spec.Template.Image.SharedGallery.SKU
	}

	if dst.Spec.Template.Image != nil && restored.Spec.Template.Image.ComputeGallery != nil {
		dst.Spec.Template.Image.ComputeGallery = restored.Spec.Template.Image.ComputeGallery
	}

	if len(dst.Annotations) == 0 {
		dst.Annotations = nil
	}

	for i, r := range restored.Status.LongRunningOperationStates {
		if r.Name == dst.Status.LongRunningOperationStates[i].Name {
			dst.Status.LongRunningOperationStates[i].ServiceName = r.ServiceName
		}
	}

	return nil
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureMachinePool) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1exp.AzureMachinePool)
	if err := Convert_v1beta1_AzureMachinePool_To_v1alpha3_AzureMachinePool(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	return utilconversion.MarshalData(src, dst)
}

func Convert_v1beta1_AzureMachinePoolMachineTemplate_To_v1alpha3_AzureMachinePoolMachineTemplate(in *infrav1exp.AzureMachinePoolMachineTemplate, out *AzureMachinePoolMachineTemplate, s convert.Scope) error {
	return autoConvert_v1beta1_AzureMachinePoolMachineTemplate_To_v1alpha3_AzureMachinePoolMachineTemplate(in, out, s)
}

func Convert_v1beta1_AzureMachinePoolSpec_To_v1alpha3_AzureMachinePoolSpec(in *infrav1exp.AzureMachinePoolSpec, out *AzureMachinePoolSpec, s convert.Scope) error {
	return autoConvert_v1beta1_AzureMachinePoolSpec_To_v1alpha3_AzureMachinePoolSpec(in, out, s)
}

func Convert_v1beta1_AzureMachinePoolStatus_To_v1alpha3_AzureMachinePoolStatus(in *infrav1exp.AzureMachinePoolStatus, out *AzureMachinePoolStatus, s convert.Scope) error {
	if len(in.LongRunningOperationStates) > 0 {
		if out.LongRunningOperationState == nil {
			out.LongRunningOperationState = &infrav1alpha3.Future{}
		}
		if err := infrav1alpha3.Convert_v1beta1_Future_To_v1alpha3_Future(&in.LongRunningOperationStates[0], out.LongRunningOperationState, s); err != nil {
			return err
		}
	}
	return autoConvert_v1beta1_AzureMachinePoolStatus_To_v1alpha3_AzureMachinePoolStatus(in, out, s)
}

func Convert_v1alpha3_AzureMachinePoolStatus_To_v1beta1_AzureMachinePoolStatus(in *AzureMachinePoolStatus, out *infrav1exp.AzureMachinePoolStatus, s convert.Scope) error {
	if in.LongRunningOperationState != nil {
		f := infrav1.Future{}
		if err := infrav1alpha3.Convert_v1alpha3_Future_To_v1beta1_Future(in.LongRunningOperationState, &f, s); err != nil {
			return err
		}
		out.LongRunningOperationStates = []infrav1.Future{f}
	}
	return autoConvert_v1alpha3_AzureMachinePoolStatus_To_v1beta1_AzureMachinePoolStatus(in, out, s)
}

// ConvertTo converts this AzureMachinePoolList to the Hub version (v1beta1).
func (src *AzureMachinePoolList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1exp.AzureMachinePoolList)
	return Convert_v1alpha3_AzureMachinePoolList_To_v1beta1_AzureMachinePoolList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureMachinePoolList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1exp.AzureMachinePoolList)
	return Convert_v1beta1_AzureMachinePoolList_To_v1alpha3_AzureMachinePoolList(src, dst, nil)
}

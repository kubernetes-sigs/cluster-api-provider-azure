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

	if restored.Spec.Template.Image != nil && restored.Spec.Template.Image.ComputeGallery != nil {
		dst.Spec.Template.Image.ComputeGallery = restored.Spec.Template.Image.ComputeGallery
	}

	if restored.Status.Image != nil && restored.Status.Image.ComputeGallery != nil {
		dst.Status.Image.ComputeGallery = restored.Status.Image.ComputeGallery
	}

	return nil
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureMachinePool) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1exp.AzureMachinePool)
	if err := Convert_v1beta1_AzureMachinePool_To_v1alpha4_AzureMachinePool(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	return utilconversion.MarshalData(src, dst)
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

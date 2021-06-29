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
	apimachineryconversion "k8s.io/apimachinery/pkg/conversion"
	infrav1alpha4 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this AzureMachineTemplate to the Hub version (v1alpha4).
func (src *AzureMachineTemplate) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha4.AzureMachineTemplate)

	if err := Convert_v1alpha3_AzureMachineTemplate_To_v1alpha4_AzureMachineTemplate(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data from annotations
	restored := &infrav1alpha4.AzureMachineTemplate{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	// Handle special case for conversion of ManagedDisk to pointer.
	if restored.Spec.Template.Spec.OSDisk.ManagedDisk == nil && dst.Spec.Template.Spec.OSDisk.ManagedDisk != nil {
		if *dst.Spec.Template.Spec.OSDisk.ManagedDisk == (infrav1alpha4.ManagedDiskParameters{}) {
			// restore nil value if nothing has changed since conversion
			dst.Spec.Template.Spec.OSDisk.ManagedDisk = nil
		}
	}

	if restored.Spec.Template.Spec.Image.SharedGallery != nil {
		dst.Spec.Template.Spec.Image.SharedGallery.Offer = restored.Spec.Template.Spec.Image.SharedGallery.Offer
		dst.Spec.Template.Spec.Image.SharedGallery.Publisher = restored.Spec.Template.Spec.Image.SharedGallery.Publisher
		dst.Spec.Template.Spec.Image.SharedGallery.SKU = restored.Spec.Template.Spec.Image.SharedGallery.SKU
	}

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha4) to this version.
func (dst *AzureMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha4.AzureMachineTemplate)
	if err := Convert_v1alpha4_AzureMachineTemplate_To_v1alpha3_AzureMachineTemplate(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	return nil
}

// ConvertTo converts this AzureMachineTemplateList to the Hub version (v1alpha4).
func (src *AzureMachineTemplateList) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha4.AzureMachineTemplateList)
	return Convert_v1alpha3_AzureMachineTemplateList_To_v1alpha4_AzureMachineTemplateList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1alpha4) to this version.
func (dst *AzureMachineTemplateList) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha4.AzureMachineTemplateList)
	return Convert_v1alpha4_AzureMachineTemplateList_To_v1alpha3_AzureMachineTemplateList(src, dst, nil)
}

func Convert_v1alpha4_AzureSharedGalleryImage_To_v1alpha3_AzureSharedGalleryImage(in *infrav1alpha4.AzureSharedGalleryImage, out *AzureSharedGalleryImage, s apimachineryconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha4_AzureSharedGalleryImage_To_v1alpha3_AzureSharedGalleryImage(in, out, s); err != nil {
		return err
	}

	return nil
}

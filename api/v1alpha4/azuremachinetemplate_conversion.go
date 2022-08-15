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
	apimachineryconversion "k8s.io/apimachinery/pkg/conversion"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this AzureMachineTemplate to the Hub version (v1beta1).
func (src *AzureMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.AzureMachineTemplate)
	if err := Convert_v1alpha4_AzureMachineTemplate_To_v1beta1_AzureMachineTemplate(src, dst, nil); err != nil {
		return err
	}

	// Restore missing fields from annotations
	restored := &infrav1.AzureMachineTemplate{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	if dst.Spec.Template.Spec.Image != nil && restored.Spec.Template.Spec.Image.ComputeGallery != nil {
		dst.Spec.Template.Spec.Image.ComputeGallery = restored.Spec.Template.Spec.Image.ComputeGallery
	}

	if restored.Spec.Template.Spec.AdditionalCapabilities != nil {
		dst.Spec.Template.Spec.AdditionalCapabilities = restored.Spec.Template.Spec.AdditionalCapabilities
	}

	dst.Spec.Template.ObjectMeta = restored.Spec.Template.ObjectMeta

	return nil
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.AzureMachineTemplate)
	if err := Convert_v1beta1_AzureMachineTemplate_To_v1alpha4_AzureMachineTemplate(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	return utilconversion.MarshalData(src, dst)
}

// ConvertTo converts this AzureMachineTemplateList to the Hub version (v1beta1).
func (src *AzureMachineTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.AzureMachineTemplateList)
	return Convert_v1alpha4_AzureMachineTemplateList_To_v1beta1_AzureMachineTemplateList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureMachineTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.AzureMachineTemplateList)
	return Convert_v1beta1_AzureMachineTemplateList_To_v1alpha4_AzureMachineTemplateList(src, dst, nil)
}

func Convert_v1beta1_AzureMachineTemplateResource_To_v1alpha4_AzureMachineTemplateResource(in *infrav1.AzureMachineTemplateResource, out *AzureMachineTemplateResource, s apimachineryconversion.Scope) error {
	return autoConvert_v1beta1_AzureMachineTemplateResource_To_v1alpha4_AzureMachineTemplateResource(in, out, s)
}

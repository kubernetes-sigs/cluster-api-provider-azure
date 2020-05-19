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
	infrav1alpha3 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this AzureMachineTemplate to the Hub version (v1alpha3).
func (src *AzureMachineTemplate) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha3.AzureMachineTemplate)
	if err := Convert_v1alpha2_AzureMachineTemplate_To_v1alpha3_AzureMachineTemplate(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data from annotations
	restored := &infrav1alpha3.AzureMachineTemplate{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}
	if restored.Spec.Template.Spec.Identity != "" {
		dst.Spec.Template.Spec.Identity = restored.Spec.Template.Spec.Identity
	}
	if len(restored.Spec.Template.Spec.UserAssignedIdentities) > 0 {
		dst.Spec.Template.Spec.UserAssignedIdentities = restored.Spec.Template.Spec.UserAssignedIdentities
	}
	dst.Spec.Template.Spec.FailureDomain = restored.Spec.Template.Spec.FailureDomain

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha3) to this version.
func (dst *AzureMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha3.AzureMachineTemplate)
	if err := Convert_v1alpha3_AzureMachineTemplate_To_v1alpha2_AzureMachineTemplate(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	return nil
}

// ConvertTo converts this AzureMachineTemplateList to the Hub version (v1alpha3).
func (src *AzureMachineTemplateList) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha3.AzureMachineTemplateList)
	return Convert_v1alpha2_AzureMachineTemplateList_To_v1alpha3_AzureMachineTemplateList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1alpha3) to this version.
func (dst *AzureMachineTemplateList) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha3.AzureMachineTemplateList)
	return Convert_v1alpha3_AzureMachineTemplateList_To_v1alpha2_AzureMachineTemplateList(src, dst, nil)
}

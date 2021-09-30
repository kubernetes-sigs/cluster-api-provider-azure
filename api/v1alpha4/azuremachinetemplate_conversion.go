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
	infrav1beta1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this AzureMachineTemplate to the Hub version (v1beta1).
func (src *AzureMachineTemplate) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1beta1.AzureMachineTemplate)

	return Convert_v1alpha4_AzureMachineTemplate_To_v1beta1_AzureMachineTemplate(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1beta1.AzureMachineTemplate)
	return Convert_v1beta1_AzureMachineTemplate_To_v1alpha4_AzureMachineTemplate(src, dst, nil)
}

// ConvertTo converts this AzureMachineTemplateList to the Hub version (v1beta1).
func (src *AzureMachineTemplateList) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1beta1.AzureMachineTemplateList)
	return Convert_v1alpha4_AzureMachineTemplateList_To_v1beta1_AzureMachineTemplateList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureMachineTemplateList) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1beta1.AzureMachineTemplateList)
	return Convert_v1beta1_AzureMachineTemplateList_To_v1alpha4_AzureMachineTemplateList(src, dst, nil)
}

/*
Copyright 2025 The Kubernetes Authors.

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

package v1alpha1

import (
"sigs.k8s.io/controller-runtime/pkg/conversion"

infrav1beta2 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta2"
)

// ConvertTo converts this AzureASOManagedMachinePoolTemplate to the Hub version (v1beta2).
func (src *AzureASOManagedMachinePoolTemplate) ConvertTo(dstRaw conversion.Hub) error {
dst := dstRaw.(*infrav1beta2.AzureASOManagedMachinePoolTemplate)
return Convert_v1alpha1_AzureASOManagedMachinePoolTemplate_To_v1beta2_AzureASOManagedMachinePoolTemplate(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta2) to this version (v1alpha1).
func (dst *AzureASOManagedMachinePoolTemplate) ConvertFrom(srcRaw conversion.Hub) error {
src := srcRaw.(*infrav1beta2.AzureASOManagedMachinePoolTemplate)
return Convert_v1beta2_AzureASOManagedMachinePoolTemplate_To_v1alpha1_AzureASOManagedMachinePoolTemplate(src, dst, nil)
}

// ConvertTo converts this AzureASOManagedMachinePoolTemplateList to the Hub version.
func (src *AzureASOManagedMachinePoolTemplateList) ConvertTo(dstRaw conversion.Hub) error {
dst := dstRaw.(*infrav1beta2.AzureASOManagedMachinePoolTemplateList)
return Convert_v1alpha1_AzureASOManagedMachinePoolTemplateList_To_v1beta2_AzureASOManagedMachinePoolTemplateList(src, dst, nil)
}

// ConvertFrom converts from the Hub version to this version.
func (dst *AzureASOManagedMachinePoolTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
src := srcRaw.(*infrav1beta2.AzureASOManagedMachinePoolTemplateList)
return Convert_v1beta2_AzureASOManagedMachinePoolTemplateList_To_v1alpha1_AzureASOManagedMachinePoolTemplateList(src, dst, nil)
}

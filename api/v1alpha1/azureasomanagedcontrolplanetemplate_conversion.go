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

// ConvertTo converts this AzureASOManagedControlPlaneTemplate to the Hub version (v1beta2).
func (src *AzureASOManagedControlPlaneTemplate) ConvertTo(dstRaw conversion.Hub) error {
dst := dstRaw.(*infrav1beta2.AzureASOManagedControlPlaneTemplate)
return Convert_v1alpha1_AzureASOManagedControlPlaneTemplate_To_v1beta2_AzureASOManagedControlPlaneTemplate(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta2) to this version (v1alpha1).
func (dst *AzureASOManagedControlPlaneTemplate) ConvertFrom(srcRaw conversion.Hub) error {
src := srcRaw.(*infrav1beta2.AzureASOManagedControlPlaneTemplate)
return Convert_v1beta2_AzureASOManagedControlPlaneTemplate_To_v1alpha1_AzureASOManagedControlPlaneTemplate(src, dst, nil)
}

// ConvertTo converts this AzureASOManagedControlPlaneTemplateList to the Hub version.
func (src *AzureASOManagedControlPlaneTemplateList) ConvertTo(dstRaw conversion.Hub) error {
dst := dstRaw.(*infrav1beta2.AzureASOManagedControlPlaneTemplateList)
return Convert_v1alpha1_AzureASOManagedControlPlaneTemplateList_To_v1beta2_AzureASOManagedControlPlaneTemplateList(src, dst, nil)
}

// ConvertFrom converts from the Hub version to this version.
func (dst *AzureASOManagedControlPlaneTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
src := srcRaw.(*infrav1beta2.AzureASOManagedControlPlaneTemplateList)
return Convert_v1beta2_AzureASOManagedControlPlaneTemplateList_To_v1alpha1_AzureASOManagedControlPlaneTemplateList(src, dst, nil)
}

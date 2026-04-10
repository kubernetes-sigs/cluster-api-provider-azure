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

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta2"
)

// ConvertTo converts this AzureASOManagedClusterTemplate to the Hub version (v1beta2).
func (src *AzureASOManagedClusterTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.AzureASOManagedClusterTemplate)
	return Convert_v1alpha1_AzureASOManagedClusterTemplate_To_v1beta2_AzureASOManagedClusterTemplate(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta2) to this version (v1alpha1).
func (dst *AzureASOManagedClusterTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.AzureASOManagedClusterTemplate)
	return Convert_v1beta2_AzureASOManagedClusterTemplate_To_v1alpha1_AzureASOManagedClusterTemplate(src, dst, nil)
}

// ConvertTo converts this AzureASOManagedClusterTemplateList to the Hub version.
func (src *AzureASOManagedClusterTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.AzureASOManagedClusterTemplateList)
	return Convert_v1alpha1_AzureASOManagedClusterTemplateList_To_v1beta2_AzureASOManagedClusterTemplateList(src, dst, nil)
}

// ConvertFrom converts from the Hub version to this version.
func (dst *AzureASOManagedClusterTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.AzureASOManagedClusterTemplateList)
	return Convert_v1beta2_AzureASOManagedClusterTemplateList_To_v1alpha1_AzureASOManagedClusterTemplateList(src, dst, nil)
}

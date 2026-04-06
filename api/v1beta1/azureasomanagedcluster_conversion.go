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

package v1beta1

import (
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta2"
)

// ConvertTo converts this AzureASOManagedCluster to the Hub version (v1beta2).
func (src *AzureASOManagedCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.AzureASOManagedCluster)
	return Convert_v1beta1_AzureASOManagedCluster_To_v1beta2_AzureASOManagedCluster(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta2) to this version (v1beta1).
func (dst *AzureASOManagedCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta2.AzureASOManagedCluster)
	return Convert_v1beta2_AzureASOManagedCluster_To_v1beta1_AzureASOManagedCluster(src, dst, nil)
}

// ConvertTo converts this AzureASOManagedClusterList to the Hub version.
func (src *AzureASOManagedClusterList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.AzureASOManagedClusterList)
	return Convert_v1beta1_AzureASOManagedClusterList_To_v1beta2_AzureASOManagedClusterList(src, dst, nil)
}

// ConvertFrom converts from the Hub version to this version.
func (dst *AzureASOManagedClusterList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta2.AzureASOManagedClusterList)
	return Convert_v1beta2_AzureASOManagedClusterList_To_v1beta1_AzureASOManagedClusterList(src, dst, nil)
}

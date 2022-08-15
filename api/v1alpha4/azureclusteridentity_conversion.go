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
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this AzureCluster to the Hub version (v1beta1).
func (src *AzureClusterIdentity) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.AzureClusterIdentity)
	return Convert_v1alpha4_AzureClusterIdentity_To_v1beta1_AzureClusterIdentity(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureClusterIdentity) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.AzureClusterIdentity)
	return Convert_v1beta1_AzureClusterIdentity_To_v1alpha4_AzureClusterIdentity(src, dst, nil)
}

// ConvertTo converts this AzureCluster to the Hub version (v1beta1).
func (src *AzureClusterIdentityList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.AzureClusterIdentityList)
	return Convert_v1alpha4_AzureClusterIdentityList_To_v1beta1_AzureClusterIdentityList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureClusterIdentityList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.AzureClusterIdentityList)
	return Convert_v1beta1_AzureClusterIdentityList_To_v1alpha4_AzureClusterIdentityList(src, dst, nil)
}

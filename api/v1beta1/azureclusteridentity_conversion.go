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

func (src *AzureClusterIdentity) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.AzureClusterIdentity)
	if err := Convert_v1beta1_AzureClusterIdentity_To_v1beta2_AzureClusterIdentity(src, dst, nil); err != nil {
		return err
	}
	// Preserve v1beta1 conditions in deprecated for round-trip fidelity.
	if len(src.Status.Conditions) > 0 {
		if dst.Status.Deprecated == nil {
			dst.Status.Deprecated = &infrav1beta2.AzureClusterIdentityDeprecatedStatus{}
		}
		if dst.Status.Deprecated.V1Beta1 == nil {
			dst.Status.Deprecated.V1Beta1 = &infrav1beta2.AzureClusterIdentityV1Beta1DeprecatedStatus{}
		}
		dst.Status.Deprecated.V1Beta1.Conditions = src.Status.Conditions
	}
	return nil
}

func (dst *AzureClusterIdentity) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta2.AzureClusterIdentity)
	return Convert_v1beta2_AzureClusterIdentity_To_v1beta1_AzureClusterIdentity(src, dst, nil)
}

func (src *AzureClusterIdentityList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.AzureClusterIdentityList)
	return Convert_v1beta1_AzureClusterIdentityList_To_v1beta2_AzureClusterIdentityList(src, dst, nil)
}

func (dst *AzureClusterIdentityList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta2.AzureClusterIdentityList)
	return Convert_v1beta2_AzureClusterIdentityList_To_v1beta1_AzureClusterIdentityList(src, dst, nil)
}

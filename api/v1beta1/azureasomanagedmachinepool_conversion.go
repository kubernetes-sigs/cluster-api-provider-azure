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

// ConvertTo converts this AzureASOManagedMachinePool to the Hub version (v1beta2).
func (src *AzureASOManagedMachinePool) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.AzureASOManagedMachinePool)
	return Convert_v1beta1_AzureASOManagedMachinePool_To_v1beta2_AzureASOManagedMachinePool(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta2) to this version (v1beta1).
func (dst *AzureASOManagedMachinePool) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta2.AzureASOManagedMachinePool)
	return Convert_v1beta2_AzureASOManagedMachinePool_To_v1beta1_AzureASOManagedMachinePool(src, dst, nil)
}

// ConvertTo converts this AzureASOManagedMachinePoolList to the Hub version.
func (src *AzureASOManagedMachinePoolList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.AzureASOManagedMachinePoolList)
	return Convert_v1beta1_AzureASOManagedMachinePoolList_To_v1beta2_AzureASOManagedMachinePoolList(src, dst, nil)
}

// ConvertFrom converts from the Hub version to this version.
func (dst *AzureASOManagedMachinePoolList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta2.AzureASOManagedMachinePoolList)
	return Convert_v1beta2_AzureASOManagedMachinePoolList_To_v1beta1_AzureASOManagedMachinePoolList(src, dst, nil)
}

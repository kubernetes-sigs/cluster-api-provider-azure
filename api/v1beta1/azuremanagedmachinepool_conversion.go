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

// ConvertTo converts this AzureManagedMachinePool to the Hub version (v1beta2).
func (src *AzureManagedMachinePool) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.AzureManagedMachinePool)
	return Convert_v1beta1_AzureManagedMachinePool_To_v1beta2_AzureManagedMachinePool(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta2) to this version (v1beta1).
func (dst *AzureManagedMachinePool) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta2.AzureManagedMachinePool)
	return Convert_v1beta2_AzureManagedMachinePool_To_v1beta1_AzureManagedMachinePool(src, dst, nil)
}

// ConvertTo converts this AzureManagedMachinePoolList to the Hub version.
func (src *AzureManagedMachinePoolList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1beta2.AzureManagedMachinePoolList)
	return Convert_v1beta1_AzureManagedMachinePoolList_To_v1beta2_AzureManagedMachinePoolList(src, dst, nil)
}

// ConvertFrom converts from the Hub version to this version.
func (dst *AzureManagedMachinePoolList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1beta2.AzureManagedMachinePoolList)
	return Convert_v1beta2_AzureManagedMachinePoolList_To_v1beta1_AzureManagedMachinePoolList(src, dst, nil)
}

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
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this AzureMachinePoolMachine to the Hub version (v1beta1).
func (src *AzureMachinePoolMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1exp.AzureMachinePoolMachine)
	return Convert_v1alpha4_AzureMachinePoolMachine_To_v1beta1_AzureMachinePoolMachine(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureMachinePoolMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1exp.AzureMachinePoolMachine)
	return Convert_v1beta1_AzureMachinePoolMachine_To_v1alpha4_AzureMachinePoolMachine(src, dst, nil)
}

// ConvertTo converts this AzureMachinePoolMachineList to the Hub version (v1beta1).
func (src *AzureMachinePoolMachineList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1exp.AzureMachinePoolMachineList)
	return Convert_v1alpha4_AzureMachinePoolMachineList_To_v1beta1_AzureMachinePoolMachineList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureMachinePoolMachineList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1exp.AzureMachinePoolMachineList)
	return Convert_v1beta1_AzureMachinePoolMachineList_To_v1alpha4_AzureMachinePoolMachineList(src, dst, nil)
}

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
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta2"
)

// ConvertTo converts this AzureMachine to the Hub version (v1beta2).
func (src *AzureMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.AzureMachine)
	if err := Convert_v1beta1_AzureMachine_To_v1beta2_AzureMachine(src, dst, nil); err != nil {
		return err
	}

	// Restore hub-only status fields that don't roundtrip through v1beta1.
	restored := &infrav1.AzureMachine{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil {
		return err
	} else if ok {
		dst.Status.Conditions = restored.Status.Conditions
		dst.Status.Initialization = restored.Status.Initialization
		dst.Status.Deprecated = restored.Status.Deprecated
	}

	return nil
}

// ConvertFrom converts from the Hub version (v1beta2) to this version (v1beta1).
func (dst *AzureMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.AzureMachine)
	if err := Convert_v1beta2_AzureMachine_To_v1beta1_AzureMachine(src, dst, nil); err != nil {
		return err
	}

	// Preserve hub-only fields in an annotation for lossless roundtrips.
	return utilconversion.MarshalData(src, dst)
}

// ConvertTo converts this AzureMachineList to the Hub version.
func (src *AzureMachineList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.AzureMachineList)
	return Convert_v1beta1_AzureMachineList_To_v1beta2_AzureMachineList(src, dst, nil)
}

// ConvertFrom converts from the Hub version to this version.
func (dst *AzureMachineList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.AzureMachineList)
	return Convert_v1beta2_AzureMachineList_To_v1beta1_AzureMachineList(src, dst, nil)
}

/*
Copyright 2020 The Kubernetes Authors.

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

package v1alpha2

import (
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	infrav1alpha3 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this AzureMachine to the Hub version (v1alpha3).
func (src *AzureMachine) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha3.AzureMachine)

	if err := Convert_v1alpha2_AzureMachine_To_v1alpha3_AzureMachine(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data from annotations
	restored := &infrav1alpha3.AzureMachine{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha3) to this version.
func (dst *AzureMachine) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha3.AzureMachine)
	if err := Convert_v1alpha3_AzureMachine_To_v1alpha2_AzureMachine(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	return nil
}

// ConvertTo converts this AzureMachineList to the Hub version (v1alpha3).
func (src *AzureMachineList) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha3.AzureMachineList)
	return Convert_v1alpha2_AzureMachineList_To_v1alpha3_AzureMachineList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1alpha3) to this version.
func (dst *AzureMachineList) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha3.AzureMachineList)
	return Convert_v1alpha3_AzureMachineList_To_v1alpha2_AzureMachineList(src, dst, nil)
}

func Convert_v1alpha2_AzureMachineSpec_To_v1alpha3_AzureMachineSpec(in *AzureMachineSpec, out *infrav1alpha3.AzureMachineSpec, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha2_AzureMachineSpec_To_v1alpha3_AzureMachineSpec(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha3_AzureMachineSpec_To_v1alpha2_AzureMachineSpec converts from the Hub version (v1alpha3) of the AzureMachineSpec to this version.
func Convert_v1alpha3_AzureMachineSpec_To_v1alpha2_AzureMachineSpec(in *infrav1alpha3.AzureMachineSpec, out *AzureMachineSpec, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha3_AzureMachineSpec_To_v1alpha2_AzureMachineSpec(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha2_AzureMachineStatus_To_v1alpha3_AzureMachineStatus converts this AzureMachineStatus to the Hub version (v1alpha3).
func Convert_v1alpha2_AzureMachineStatus_To_v1alpha3_AzureMachineStatus(in *AzureMachineStatus, out *infrav1alpha3.AzureMachineStatus, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha2_AzureMachineStatus_To_v1alpha3_AzureMachineStatus(in, out, s); err != nil {
		return err
	}

	// Manually convert the Error fields to the Failure fields
	out.FailureMessage = in.ErrorMessage
	out.FailureReason = in.ErrorReason

	return nil
}

// Convert_v1alpha3_AzureMachineStatus_To_v1alpha2_AzureMachineStatus converts from the Hub version (v1alpha3) of the AzureMachineStatus to this version.
func Convert_v1alpha3_AzureMachineStatus_To_v1alpha2_AzureMachineStatus(in *infrav1alpha3.AzureMachineStatus, out *AzureMachineStatus, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha3_AzureMachineStatus_To_v1alpha2_AzureMachineStatus(in, out, s); err != nil {
		return err
	}

	// Manually convert the Failure fields to the Error fields
	out.ErrorMessage = in.FailureMessage
	out.ErrorReason = in.FailureReason

	return nil
}

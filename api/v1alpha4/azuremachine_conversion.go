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
	machineryConversion "k8s.io/apimachinery/pkg/conversion"
	"sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this AzureMachine to the Hub version (v1beta1).
func (src *AzureMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.AzureMachine)

	if err := autoConvert_v1alpha4_AzureMachine_To_v1beta1_AzureMachine(src, dst, nil); err != nil {
		return err
	}

	restored := &v1beta1.AzureMachine{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	if restored.Spec.NetworkInterfaces != nil {
		dst.Spec.NetworkInterfaces = restored.Spec.NetworkInterfaces
	}
	return nil
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.AzureMachine)
	if err := Convert_v1beta1_AzureMachine_To_v1alpha4_AzureMachine(src, dst, nil); err != nil {
		return err
	}
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}
	return nil
}

// ConvertTo converts this AzureMachineList to the Hub version (v1beta1).
func (src *AzureMachineList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta1.AzureMachineList)
	return Convert_v1alpha4_AzureMachineList_To_v1beta1_AzureMachineList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureMachineList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta1.AzureMachineList)
	return Convert_v1beta1_AzureMachineList_To_v1alpha4_AzureMachineList(src, dst, nil)
}

func Convert_v1beta1_AzureMachineSpec_To_v1alpha4_AzureMachineSpec(in *v1beta1.AzureMachineSpec, out *AzureMachineSpec, s machineryConversion.Scope) error {
	return autoConvert_v1beta1_AzureMachineSpec_To_v1alpha4_AzureMachineSpec(in, out, s)
}

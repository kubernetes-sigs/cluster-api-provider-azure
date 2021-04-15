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

package v1alpha3

import (
	infrav1alpha4 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	expv1alpha4 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this AzureMachinePool to the Hub version (v1alpha4).
func (src *AzureMachinePool) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*expv1alpha4.AzureMachinePool)

	if err := Convert_v1alpha3_AzureMachinePool_To_v1alpha4_AzureMachinePool(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data.
	restored := &expv1alpha4.AzureMachinePool{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	// Handle special case for conversion of ManagedDisk to pointer.
	if restored.Spec.Template.OSDisk.ManagedDisk == nil && dst.Spec.Template.OSDisk.ManagedDisk != nil {
		if *dst.Spec.Template.OSDisk.ManagedDisk == (infrav1alpha4.ManagedDiskParameters{}) {
			// restore nil value if nothing has changed since conversion
			dst.Spec.Template.OSDisk.ManagedDisk = nil
		}
	}

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha4) to this version.
func (dst *AzureMachinePool) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*expv1alpha4.AzureMachinePool)

	if err := Convert_v1alpha4_AzureMachinePool_To_v1alpha3_AzureMachinePool(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	return nil
}

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

// ConvertTo converts this AzureCluster to the Hub version (v1alpha3).
func (src *AzureCluster) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha3.AzureCluster)

	if err := Convert_v1alpha2_AzureCluster_To_v1alpha3_AzureCluster(src, dst, nil); err != nil {
		return err
	}

	// Manually convert Status.APIEndpoints to Spec.ControlPlaneEndpoint.
	if len(src.Status.APIEndpoints) > 0 {
		endpoint := src.Status.APIEndpoints[0]
		dst.Spec.ControlPlaneEndpoint.Host = endpoint.Host
		dst.Spec.ControlPlaneEndpoint.Port = int32(endpoint.Port)
	}

	// Manually restore data.
	restored := &infrav1alpha3.AzureCluster{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	dst.Status.FailureDomains = restored.Status.FailureDomains

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha3) to this version.
func (dst *AzureCluster) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha3.AzureCluster)

	if err := Convert_v1alpha3_AzureCluster_To_v1alpha2_AzureCluster(src, dst, nil); err != nil {
		return err
	}

	// Manually convert Spec.ControlPlaneEndpoint to Status.APIEndpoints.
	if !src.Spec.ControlPlaneEndpoint.IsZero() {
		dst.Status.APIEndpoints = []APIEndpoint{
			{
				Host: src.Spec.ControlPlaneEndpoint.Host,
				Port: int(src.Spec.ControlPlaneEndpoint.Port),
			},
		}
	}

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	return nil
}

// ConvertTo converts this AzureClusterList to the Hub version (v1alpha3).
func (src *AzureClusterList) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha3.AzureClusterList)
	return Convert_v1alpha2_AzureClusterList_To_v1alpha3_AzureClusterList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1alpha3) to this version.
func (dst *AzureClusterList) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha3.AzureClusterList)
	return Convert_v1alpha3_AzureClusterList_To_v1alpha2_AzureClusterList(src, dst, nil)
}

// Convert_v1alpha2_AzureClusterStatus_To_v1alpha3_AzureClusterStatus converts AzureCluster.Status from v1alpha2 to v1alpha3.
func Convert_v1alpha2_AzureClusterStatus_To_v1alpha3_AzureClusterStatus(in *AzureClusterStatus, out *infrav1alpha3.AzureClusterStatus, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha2_AzureClusterStatus_To_v1alpha3_AzureClusterStatus(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha2_AzureClusterSpec_To_v1alpha3_AzureClusterSpec.
func Convert_v1alpha2_AzureClusterSpec_To_v1alpha3_AzureClusterSpec(in *AzureClusterSpec, out *infrav1alpha3.AzureClusterSpec, s apiconversion.Scope) error { //nolint
	if err := autoConvert_v1alpha2_AzureClusterSpec_To_v1alpha3_AzureClusterSpec(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha3_AzureClusterSpec_To_v1alpha2_AzureClusterSpec converts from the Hub version (v1alpha3) of the AzureClusterSpec to this version.
func Convert_v1alpha3_AzureClusterSpec_To_v1alpha2_AzureClusterSpec(in *infrav1alpha3.AzureClusterSpec, out *AzureClusterSpec, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha3_AzureClusterSpec_To_v1alpha2_AzureClusterSpec(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha3_AzureClusterStatus_To_v1alpha2_AzureClusterStatus.
func Convert_v1alpha3_AzureClusterStatus_To_v1alpha2_AzureClusterStatus(in *infrav1alpha3.AzureClusterStatus, out *AzureClusterStatus, s apiconversion.Scope) error { //nolint
	if err := autoConvert_v1alpha3_AzureClusterStatus_To_v1alpha2_AzureClusterStatus(in, out, s); err != nil {
		return err
	}

	return nil
}

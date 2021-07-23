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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	infrav1alpha4 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

const (
	AzureClusterKind = "AzureCluster"
)

// ConvertTo converts this AzureCluster to the Hub version (v1alpha4).
func (src *AzureClusterIdentity) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha4.AzureClusterIdentity)
	if err := Convert_v1alpha3_AzureClusterIdentity_To_v1alpha4_AzureClusterIdentity(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data.
	restored := &infrav1alpha4.AzureClusterIdentity{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	if len(src.Spec.AllowedNamespaces) > 0 {
		dst.Spec.AllowedNamespaces = &infrav1alpha4.AllowedNamespaces{}
		for _, ns := range src.Spec.AllowedNamespaces {
			dst.Spec.AllowedNamespaces.NamespaceList = append(dst.Spec.AllowedNamespaces.NamespaceList, ns)
		}
		dst.Spec.AllowedNamespaces.Selector = restored.Spec.AllowedNamespaces.Selector
	}

	// removing ownerReference for AzureCluster as ownerReference is not required from v1alpha4 onwards.
	var restoredOwnerReferences []v1.OwnerReference
	for _, ownerRef :=  range dst.OwnerReferences {
		if ownerRef.Kind != AzureClusterKind {
			restoredOwnerReferences = append(restoredOwnerReferences, ownerRef)
		}
	}
	dst.OwnerReferences = restoredOwnerReferences

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha4) to this version.
func (dst *AzureClusterIdentity) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha4.AzureClusterIdentity)
	if err := Convert_v1alpha4_AzureClusterIdentity_To_v1alpha3_AzureClusterIdentity(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	if src.Spec.AllowedNamespaces != nil {
		for _, ns := range src.Spec.AllowedNamespaces.NamespaceList {
			dst.Spec.AllowedNamespaces = append(dst.Spec.AllowedNamespaces, ns)
		}
	}

	return nil
}

// Convert_v1alpha3_AzureClusterIdentitySpec_To_v1alpha4_AzureClusterIdentitySpec.
func Convert_v1alpha3_AzureClusterIdentitySpec_To_v1alpha4_AzureClusterIdentitySpec(in *AzureClusterIdentitySpec, out *infrav1alpha4.AzureClusterIdentitySpec, s apiconversion.Scope) error { //nolint
	if err := autoConvert_v1alpha3_AzureClusterIdentitySpec_To_v1alpha4_AzureClusterIdentitySpec(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha4_AzureClusterIdentitySpec_To_v1alpha3_AzureClusterIdentitySpec
func Convert_v1alpha4_AzureClusterIdentitySpec_To_v1alpha3_AzureClusterIdentitySpec(in *infrav1alpha4.AzureClusterIdentitySpec, out *AzureClusterIdentitySpec, s apiconversion.Scope) error { //nolint
	if err := autoConvert_v1alpha4_AzureClusterIdentitySpec_To_v1alpha3_AzureClusterIdentitySpec(in, out, s); err != nil {
		return err
	}

	return nil
}

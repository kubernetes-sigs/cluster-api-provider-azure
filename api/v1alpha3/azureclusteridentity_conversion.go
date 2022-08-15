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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

const (
	AzureClusterKind = "AzureCluster"
)

// ConvertTo converts this AzureCluster to the Hub version (v1beta1).
func (src *AzureClusterIdentity) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.AzureClusterIdentity)
	if err := Convert_v1alpha3_AzureClusterIdentity_To_v1beta1_AzureClusterIdentity(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data.
	restored := &infrav1.AzureClusterIdentity{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	if len(dst.Annotations) == 0 {
		dst.Annotations = nil
	}

	if len(src.Spec.AllowedNamespaces) > 0 {
		dst.Spec.AllowedNamespaces = &infrav1.AllowedNamespaces{}
		dst.Spec.AllowedNamespaces.NamespaceList = append(dst.Spec.AllowedNamespaces.NamespaceList, src.Spec.AllowedNamespaces...)
	}

	if restored.Spec.AllowedNamespaces != nil && restored.Spec.AllowedNamespaces.Selector != nil {
		if dst.Spec.AllowedNamespaces == nil {
			dst.Spec.AllowedNamespaces = &infrav1.AllowedNamespaces{}
		}
		dst.Spec.AllowedNamespaces.Selector = restored.Spec.AllowedNamespaces.Selector
	}

	// removing ownerReference for AzureCluster as ownerReference is not required from v1alpha4/v1beta1 onwards.
	var restoredOwnerReferences []metav1.OwnerReference
	for _, ownerRef := range dst.OwnerReferences {
		if ownerRef.Kind != AzureClusterKind {
			restoredOwnerReferences = append(restoredOwnerReferences, ownerRef)
		}
	}
	dst.OwnerReferences = restoredOwnerReferences

	return nil
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureClusterIdentity) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.AzureClusterIdentity)
	if err := Convert_v1beta1_AzureClusterIdentity_To_v1alpha3_AzureClusterIdentity(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	if src.Spec.AllowedNamespaces != nil {
		dst.Spec.AllowedNamespaces = append(dst.Spec.AllowedNamespaces, src.Spec.AllowedNamespaces.NamespaceList...)
	}

	return nil
}

// Convert_v1alpha3_AzureClusterIdentitySpec_To_v1beta1_AzureClusterIdentitySpec.
func Convert_v1alpha3_AzureClusterIdentitySpec_To_v1beta1_AzureClusterIdentitySpec(in *AzureClusterIdentitySpec, out *infrav1.AzureClusterIdentitySpec, s apiconversion.Scope) error {
	return autoConvert_v1alpha3_AzureClusterIdentitySpec_To_v1beta1_AzureClusterIdentitySpec(in, out, s)
}

// Convert_v1beta1_AzureClusterIdentitySpec_To_v1alpha3_AzureClusterIdentitySpec.
func Convert_v1beta1_AzureClusterIdentitySpec_To_v1alpha3_AzureClusterIdentitySpec(in *infrav1.AzureClusterIdentitySpec, out *AzureClusterIdentitySpec, s apiconversion.Scope) error {
	return autoConvert_v1beta1_AzureClusterIdentitySpec_To_v1alpha3_AzureClusterIdentitySpec(in, out, s)
}

// ConvertTo converts this AzureClusterIdentityList to the Hub version (v1beta1).
func (src *AzureClusterIdentityList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.AzureClusterIdentityList)
	return Convert_v1alpha3_AzureClusterIdentityList_To_v1beta1_AzureClusterIdentityList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureClusterIdentityList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.AzureClusterIdentityList)
	return Convert_v1beta1_AzureClusterIdentityList_To_v1alpha3_AzureClusterIdentityList(src, dst, nil)
}

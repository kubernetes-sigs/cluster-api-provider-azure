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
	unsafe "unsafe"

	apiconversion "k8s.io/apimachinery/pkg/conversion"
	infrav1alpha3 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

const (
	subscriptionIDAnnotation = "azurecluster.infrastructure.cluster.x-k8s.io/subscriptionID"
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

	if subscriptionID, ok := src.Annotations[subscriptionIDAnnotation]; ok {
		dst.Spec.SubscriptionID = subscriptionID
		delete(dst.Annotations, subscriptionIDAnnotation)
		if len(dst.Annotations) == 0 {
			dst.Annotations = nil
		}
	}

	// Manually restore data.
	restored := &infrav1alpha3.AzureCluster{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	dst.Status.FailureDomains = restored.Status.FailureDomains
	dst.Spec.NetworkSpec.Vnet.CIDRBlocks = restored.Spec.NetworkSpec.Vnet.CIDRBlocks

	for _, restoredSubnet := range restored.Spec.NetworkSpec.Subnets {
		if restoredSubnet != nil {
			for _, dstSubnet := range dst.Spec.NetworkSpec.Subnets {
				if dstSubnet != nil && dstSubnet.Name == restoredSubnet.Name {
					dstSubnet.RouteTable = restoredSubnet.RouteTable
					dstSubnet.CIDRBlocks = restoredSubnet.CIDRBlocks
					dstSubnet.SecurityGroup.IngressRules = restoredSubnet.SecurityGroup.IngressRules
				}
			}
		}
	}

	// Manually convert conditions
	dst.SetConditions(restored.GetConditions())

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

	// Preserve Spec.SubscriptionID in annotation `azurecluster.infrastructure.cluster.x-k8s.io/subscriptionID`
	if src.Spec.SubscriptionID != "" {
		if dst.Annotations == nil {
			dst.Annotations = make(map[string]string)
		}
		dst.Annotations[subscriptionIDAnnotation] = src.Spec.SubscriptionID
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

// Convert_v1alpha2_Network_To_v1alpha3_Network.
func Convert_v1alpha2_Network_To_v1alpha3_Network(in *Network, out *infrav1alpha3.Network, s apiconversion.Scope) error { //nolint
	return autoConvert_v1alpha2_Network_To_v1alpha3_Network(in, out, s)
}

// Convert_v1alpha2_NetworkSpec_To_v1alpha3_NetworkSpec.
func Convert_v1alpha2_NetworkSpec_To_v1alpha3_NetworkSpec(in *NetworkSpec, out *infrav1alpha3.NetworkSpec, s apiconversion.Scope) error { //nolint
	if err := Convert_v1alpha2_VnetSpec_To_v1alpha3_VnetSpec(&in.Vnet, &out.Vnet, s); err != nil {
		return err
	}

	out.Subnets = make(infrav1alpha3.Subnets, len(in.Subnets))
	for i := range in.Subnets {
		if in.Subnets[i] != nil {
			out.Subnets[i] = &infrav1alpha3.SubnetSpec{}
			if err := Convert_v1alpha2_SubnetSpec_To_v1alpha3_SubnetSpec(in.Subnets[i], out.Subnets[i], s); err != nil {
				return err
			}
		}
	}

	return nil
}

// Convert_v1alpha3_NetworkSpec_To_v1alpha2_NetworkSpec.
func Convert_v1alpha3_NetworkSpec_To_v1alpha2_NetworkSpec(in *infrav1alpha3.NetworkSpec, out *NetworkSpec, s apiconversion.Scope) error { //nolint
	if err := Convert_v1alpha3_VnetSpec_To_v1alpha2_VnetSpec(&in.Vnet, &out.Vnet, s); err != nil {
		return err
	}

	out.Subnets = make(Subnets, len(in.Subnets))
	for i := range in.Subnets {
		if in.Subnets[i] != nil {
			out.Subnets[i] = &SubnetSpec{}
			if err := Convert_v1alpha3_SubnetSpec_To_v1alpha2_SubnetSpec(in.Subnets[i], out.Subnets[i], s); err != nil {
				return err
			}
		}
	}

	return nil
}

// Convert_v1alpha3_VnetSpec_To_v1alpha2_VnetSpec.
func Convert_v1alpha3_VnetSpec_To_v1alpha2_VnetSpec(in *infrav1alpha3.VnetSpec, out *VnetSpec, s apiconversion.Scope) error { //nolint
	return autoConvert_v1alpha3_VnetSpec_To_v1alpha2_VnetSpec(in, out, s)
}

// Convert_v1alpha2_SubnetSpec_To_v1alpha3_SubnetSpec.
func Convert_v1alpha2_SubnetSpec_To_v1alpha3_SubnetSpec(in *SubnetSpec, out *infrav1alpha3.SubnetSpec, s apiconversion.Scope) error { //nolint
	return autoConvert_v1alpha2_SubnetSpec_To_v1alpha3_SubnetSpec(in, out, s)
}

// Convert_v1alpha3_SubnetSpec_To_v1alpha2_SubnetSpec.
func Convert_v1alpha3_SubnetSpec_To_v1alpha2_SubnetSpec(in *infrav1alpha3.SubnetSpec, out *SubnetSpec, s apiconversion.Scope) error { //nolint
	return autoConvert_v1alpha3_SubnetSpec_To_v1alpha2_SubnetSpec(in, out, s)
}

func Convert_v1alpha3_SecurityGroup_To_v1alpha2_SecurityGroup(in *infrav1alpha3.SecurityGroup, out *SecurityGroup, s apiconversion.Scope) error {
	out.ID = in.ID
	out.Name = in.Name

	out.IngressRules = make(IngressRules, len(in.IngressRules))
	for i := range in.IngressRules {
		if in.IngressRules[i] != nil {
			out.IngressRules[i] = &IngressRule{}
			if err := Convert_v1alpha3_IngressRule_To_v1alpha2_IngressRule(in.IngressRules[i], out.IngressRules[i], s); err != nil {
				return err
			}
		}
	}

	out.Tags = *(*Tags)(unsafe.Pointer(&in.Tags))
	return nil
}

func Convert_v1alpha2_SecurityGroup_To_v1alpha3_SecurityGroup(in *SecurityGroup, out *infrav1alpha3.SecurityGroup, s apiconversion.Scope) error {
	out.ID = in.ID
	out.Name = in.Name

	out.IngressRules = make(infrav1alpha3.IngressRules, len(in.IngressRules))
	for i := range in.IngressRules {
		if in.IngressRules[i] != nil {
			out.IngressRules[i] = &infrav1alpha3.IngressRule{}
			if err := Convert_v1alpha2_IngressRule_To_v1alpha3_IngressRule(in.IngressRules[i], out.IngressRules[i], s); err != nil {
				return err
			}
		}
	}

	out.Tags = *(*infrav1alpha3.Tags)(unsafe.Pointer(&in.Tags))
	return nil
}

// Convert_v1alpha2_IngressRule_To_v1alpha3_IngressRule
func Convert_v1alpha2_IngressRule_To_v1alpha3_IngressRule(in *IngressRule, out *infrav1alpha3.IngressRule, s apiconversion.Scope) error {
	return autoConvert_v1alpha2_IngressRule_To_v1alpha3_IngressRule(in, out, s)
}

// Convert_v1alpha3_IngressRule_To_v1alpha2_IngressRule
func Convert_v1alpha3_IngressRule_To_v1alpha2_IngressRule(in *infrav1alpha3.IngressRule, out *IngressRule, s apiconversion.Scope) error {
	return autoConvert_v1alpha3_IngressRule_To_v1alpha2_IngressRule(in, out, s)
}

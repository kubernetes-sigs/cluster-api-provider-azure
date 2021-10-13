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
	apiconversion "k8s.io/apimachinery/pkg/conversion"
	"k8s.io/utils/pointer"
	infrav1beta1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

const (
	azureEnvironmentAnnotation = "azurecluster.infrastructure.cluster.x-k8s.io/azureEnvironment"
)

// ConvertTo converts this AzureCluster to the Hub version (v1beta1).
func (src *AzureCluster) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1beta1.AzureCluster)
	if err := Convert_v1alpha3_AzureCluster_To_v1beta1_AzureCluster(src, dst, nil); err != nil {
		return err
	}

	if azureEnvironment, ok := src.Annotations[azureEnvironmentAnnotation]; ok {
		dst.Spec.AzureEnvironment = azureEnvironment
		delete(dst.Annotations, azureEnvironmentAnnotation)
		if len(dst.Annotations) == 0 {
			dst.Annotations = nil
		}
	}
	// Manually restore data.
	restored := &infrav1beta1.AzureCluster{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	dst.Spec.NetworkSpec.PrivateDNSZoneName = restored.Spec.NetworkSpec.PrivateDNSZoneName

	dst.Spec.NetworkSpec.APIServerLB.FrontendIPsCount = restored.Spec.NetworkSpec.APIServerLB.FrontendIPsCount
	dst.Spec.NetworkSpec.APIServerLB.IdleTimeoutInMinutes = restored.Spec.NetworkSpec.APIServerLB.IdleTimeoutInMinutes
	dst.Spec.CloudProviderConfigOverrides = restored.Spec.CloudProviderConfigOverrides
	dst.Spec.BastionSpec = restored.Spec.BastionSpec

	// set default control plane outbound lb for private v1alpha3 clusters
	if src.Spec.NetworkSpec.APIServerLB.Type == Internal && restored.Spec.NetworkSpec.ControlPlaneOutboundLB == nil {
		dst.Spec.NetworkSpec.ControlPlaneOutboundLB = &infrav1beta1.LoadBalancerSpec{
			FrontendIPsCount: pointer.Int32Ptr(1),
		}
	} else {
		dst.Spec.NetworkSpec.ControlPlaneOutboundLB = restored.Spec.NetworkSpec.ControlPlaneOutboundLB
	}

	// set default node plane outbound lb for all v1alpha3 clusters
	if restored.Spec.NetworkSpec.NodeOutboundLB == nil {
		dst.Spec.NetworkSpec.NodeOutboundLB = &infrav1beta1.LoadBalancerSpec{
			FrontendIPsCount: pointer.Int32Ptr(1),
		}
	} else {
		dst.Spec.NetworkSpec.NodeOutboundLB = restored.Spec.NetworkSpec.NodeOutboundLB
	}

	// Here we manually restore outbound security rules. Since v1alpha3 only supports ingress ("Inbound") rules, all v1alpha4/v1beta1 outbound rules are dropped when an AzureCluster
	// is converted to v1alpha3. We loop through all security group rules. For all previously existing outbound rules we restore the full rule.
	for _, restoredSubnet := range restored.Spec.NetworkSpec.Subnets {
		for i, dstSubnet := range dst.Spec.NetworkSpec.Subnets {
			if dstSubnet.Name == restoredSubnet.Name {
				var restoredOutboundRules []infrav1beta1.SecurityRule
				for _, restoredSecurityRule := range restoredSubnet.SecurityGroup.SecurityRules {
					if restoredSecurityRule.Direction != infrav1beta1.SecurityRuleDirectionInbound {
						// For non-inbound rules which are only supported starting in v1alpha4/v1beta1, we restore the entire rule.
						restoredOutboundRules = append(restoredOutboundRules, restoredSecurityRule)
					}
				}
				dst.Spec.NetworkSpec.Subnets[i].SecurityGroup.SecurityRules = append(dst.Spec.NetworkSpec.Subnets[i].SecurityGroup.SecurityRules, restoredOutboundRules...)
				dst.Spec.NetworkSpec.Subnets[i].NatGateway = restoredSubnet.NatGateway

				break
			}
		}
	}

	dst.Status.LongRunningOperationStates = restored.Status.LongRunningOperationStates

	// Restore list of virtual network peerings
	dst.Spec.NetworkSpec.Vnet.Peerings = restored.Spec.NetworkSpec.Vnet.Peerings

	return nil
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureCluster) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1beta1.AzureCluster)
	if err := Convert_v1beta1_AzureCluster_To_v1alpha3_AzureCluster(src, dst, nil); err != nil {
		return err
	}

	// Preserve Spec.AzureEnvironment in annotation `azurecluster.infrastructure.cluster.x-k8s.io/azureEnvironment`
	if src.Spec.AzureEnvironment != "" {
		if dst.Annotations == nil {
			dst.Annotations = make(map[string]string)
		}
		dst.Annotations[azureEnvironmentAnnotation] = src.Spec.AzureEnvironment
	}
	// Preserve Hub data on down-conversion.
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	return nil
}

// ConvertTo converts this AzureClusterList to the Hub version (v1beta1).
func (src *AzureClusterList) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1beta1.AzureClusterList)
	return Convert_v1alpha3_AzureClusterList_To_v1beta1_AzureClusterList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureClusterList) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1beta1.AzureClusterList)
	return Convert_v1beta1_AzureClusterList_To_v1alpha3_AzureClusterList(src, dst, nil)
}

// Convert_v1alpha3_AzureClusterStatus_To_v1beta1_AzureClusterStatus converts AzureCluster.Status from v1alpha3 to v1beta1.
func Convert_v1alpha3_AzureClusterStatus_To_v1beta1_AzureClusterStatus(in *AzureClusterStatus, out *infrav1beta1.AzureClusterStatus, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha3_AzureClusterStatus_To_v1beta1_AzureClusterStatus(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha3_AzureClusterSpec_To_v1beta1_AzureClusterSpec.
func Convert_v1alpha3_AzureClusterSpec_To_v1beta1_AzureClusterSpec(in *AzureClusterSpec, out *infrav1beta1.AzureClusterSpec, s apiconversion.Scope) error { //nolint
	if err := autoConvert_v1alpha3_AzureClusterSpec_To_v1beta1_AzureClusterSpec(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1beta1_AzureClusterSpec_To_v1alpha3_AzureClusterSpec converts from the Hub version (v1beta1) of the AzureClusterSpec to this version.
func Convert_v1beta1_AzureClusterSpec_To_v1alpha3_AzureClusterSpec(in *infrav1beta1.AzureClusterSpec, out *AzureClusterSpec, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1beta1_AzureClusterSpec_To_v1alpha3_AzureClusterSpec(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1beta1_AzureClusterStatus_To_v1alpha3_AzureClusterStatus.
func Convert_v1beta1_AzureClusterStatus_To_v1alpha3_AzureClusterStatus(in *infrav1beta1.AzureClusterStatus, out *AzureClusterStatus, s apiconversion.Scope) error { //nolint
	if err := autoConvert_v1beta1_AzureClusterStatus_To_v1alpha3_AzureClusterStatus(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha3_NetworkSpec_To_v1beta1_NetworkSpec.
func Convert_v1alpha3_NetworkSpec_To_v1beta1_NetworkSpec(in *NetworkSpec, out *infrav1beta1.NetworkSpec, s apiconversion.Scope) error { //nolint
	if err := Convert_v1alpha3_VnetSpec_To_v1beta1_VnetSpec(&in.Vnet, &out.Vnet, s); err != nil {
		return err
	}

	out.Subnets = make(infrav1beta1.Subnets, len(in.Subnets))
	for i := range in.Subnets {
		out.Subnets[i] = infrav1beta1.SubnetSpec{}
		if err := Convert_v1alpha3_SubnetSpec_To_v1beta1_SubnetSpec(&in.Subnets[i], &out.Subnets[i], s); err != nil {
			return err
		}
	}

	if err := autoConvert_v1alpha3_LoadBalancerSpec_To_v1beta1_LoadBalancerSpec(&in.APIServerLB, &out.APIServerLB, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1beta1_NetworkSpec_To_v1alpha3_NetworkSpec.
func Convert_v1beta1_NetworkSpec_To_v1alpha3_NetworkSpec(in *infrav1beta1.NetworkSpec, out *NetworkSpec, s apiconversion.Scope) error { //nolint
	if err := Convert_v1beta1_VnetSpec_To_v1alpha3_VnetSpec(&in.Vnet, &out.Vnet, s); err != nil {
		return err
	}

	out.Subnets = make(Subnets, len(in.Subnets))
	for i := range in.Subnets {
		out.Subnets[i] = SubnetSpec{}
		if err := Convert_v1beta1_SubnetSpec_To_v1alpha3_SubnetSpec(&in.Subnets[i], &out.Subnets[i], s); err != nil {
			return err
		}
	}

	if err := autoConvert_v1beta1_LoadBalancerSpec_To_v1alpha3_LoadBalancerSpec(&in.APIServerLB, &out.APIServerLB, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1beta1_VnetSpec_To_v1alpha3_VnetSpec.
func Convert_v1beta1_VnetSpec_To_v1alpha3_VnetSpec(in *infrav1beta1.VnetSpec, out *VnetSpec, s apiconversion.Scope) error { //nolint
	return autoConvert_v1beta1_VnetSpec_To_v1alpha3_VnetSpec(in, out, s)
}

// Convert_v1alpha3_SubnetSpec_To_v1beta1_SubnetSpec.
func Convert_v1alpha3_SubnetSpec_To_v1beta1_SubnetSpec(in *SubnetSpec, out *infrav1beta1.SubnetSpec, s apiconversion.Scope) error { //nolint
	return autoConvert_v1alpha3_SubnetSpec_To_v1beta1_SubnetSpec(in, out, s)
}

// Convert_v1beta1_SubnetSpec_To_v1alpha3_SubnetSpec.
func Convert_v1beta1_SubnetSpec_To_v1alpha3_SubnetSpec(in *infrav1beta1.SubnetSpec, out *SubnetSpec, s apiconversion.Scope) error { //nolint
	return autoConvert_v1beta1_SubnetSpec_To_v1alpha3_SubnetSpec(in, out, s)
}

func Convert_v1beta1_SecurityGroup_To_v1alpha3_SecurityGroup(in *infrav1beta1.SecurityGroup, out *SecurityGroup, s apiconversion.Scope) error {
	out.ID = in.ID
	out.Name = in.Name

	out.IngressRules = make(IngressRules, 0)
	for _, rule := range in.SecurityRules {
		if rule.Direction == infrav1beta1.SecurityRuleDirectionInbound { // only inbound rules are supported in v1alpha3.
			ingressRule := IngressRule{}
			if err := Convert_v1beta1_SecurityRule_To_v1alpha3_IngressRule(&rule, &ingressRule, s); err != nil {
				return err
			}
			out.IngressRules = append(out.IngressRules, ingressRule)
		}
	}

	out.Tags = *(*Tags)(&in.Tags)
	return nil
}

func Convert_v1alpha3_SecurityGroup_To_v1beta1_SecurityGroup(in *SecurityGroup, out *infrav1beta1.SecurityGroup, s apiconversion.Scope) error { //nolint
	out.ID = in.ID
	out.Name = in.Name

	out.SecurityRules = make(infrav1beta1.SecurityRules, len(in.IngressRules))
	for i := range in.IngressRules {
		out.SecurityRules[i] = infrav1beta1.SecurityRule{}
		if err := Convert_v1alpha3_IngressRule_To_v1beta1_SecurityRule(&in.IngressRules[i], &out.SecurityRules[i], s); err != nil {
			return err
		}
	}

	out.Tags = *(*infrav1beta1.Tags)(&in.Tags)
	return nil
}

// Convert_v1alpha3_IngressRule_To_v1beta1_SecurityRule
func Convert_v1alpha3_IngressRule_To_v1beta1_SecurityRule(in *IngressRule, out *infrav1beta1.SecurityRule, _ apiconversion.Scope) error { //nolint
	out.Name = in.Name
	out.Description = in.Description
	out.Protocol = infrav1beta1.SecurityGroupProtocol(in.Protocol)
	out.Priority = in.Priority
	out.SourcePorts = in.SourcePorts
	out.DestinationPorts = in.DestinationPorts
	out.Source = in.Source
	out.Destination = in.Destination
	out.Direction = infrav1beta1.SecurityRuleDirectionInbound // all v1alpha3 rules are inbound.
	return nil
}

// Convert_v1beta1_SecurityRule_To_v1alpha3_IngressRule
func Convert_v1beta1_SecurityRule_To_v1alpha3_IngressRule(in *infrav1beta1.SecurityRule, out *IngressRule, _ apiconversion.Scope) error { //nolint
	out.Name = in.Name
	out.Description = in.Description
	out.Protocol = SecurityGroupProtocol(in.Protocol)
	out.Priority = in.Priority
	out.SourcePorts = in.SourcePorts
	out.DestinationPorts = in.DestinationPorts
	out.Source = in.Source
	out.Destination = in.Destination
	return nil
}

// Convert_v1alpha3_VnetSpec_To_v1beta1_VnetSpec is an autogenerated conversion function.
func Convert_v1alpha3_VnetSpec_To_v1beta1_VnetSpec(in *VnetSpec, out *infrav1beta1.VnetSpec, s apiconversion.Scope) error {
	return autoConvert_v1alpha3_VnetSpec_To_v1beta1_VnetSpec(in, out, s)
}

// Convert_v1beta1_LoadBalancerSpec_To_v1alpha3_LoadBalancerSpec is an autogenerated conversion function.
func Convert_v1beta1_LoadBalancerSpec_To_v1alpha3_LoadBalancerSpec(in *infrav1beta1.LoadBalancerSpec, out *LoadBalancerSpec, s apiconversion.Scope) error {
	return autoConvert_v1beta1_LoadBalancerSpec_To_v1alpha3_LoadBalancerSpec(in, out, s)
}

// Convert_v1alpha3_Future_To_v1beta1_Future is an autogenerated conversion function.
func Convert_v1alpha3_Future_To_v1beta1_Future(in *Future, out *infrav1beta1.Future, s apiconversion.Scope) error {
	out.Data = in.FutureData
	return autoConvert_v1alpha3_Future_To_v1beta1_Future(in, out, s)
}

// Convert_v1beta1_Future_To_v1alpha3_Future is an autogenerated conversion function.
func Convert_v1beta1_Future_To_v1alpha3_Future(in *infrav1beta1.Future, out *Future, s apiconversion.Scope) error {
	out.FutureData = in.Data
	return autoConvert_v1beta1_Future_To_v1alpha3_Future(in, out, s)
}

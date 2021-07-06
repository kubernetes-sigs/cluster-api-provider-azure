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
	infrav1alpha4 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	apiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	apiv1alpha4 "sigs.k8s.io/cluster-api/api/v1alpha4"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

const (
	azureEnvironmentAnnotation = "azurecluster.infrastructure.cluster.x-k8s.io/azureEnvironment"
)

// ConvertTo converts this AzureCluster to the Hub version (v1alpha4).
func (src *AzureCluster) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha4.AzureCluster)
	if err := Convert_v1alpha3_AzureCluster_To_v1alpha4_AzureCluster(src, dst, nil); err != nil {
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
	restored := &infrav1alpha4.AzureCluster{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	dst.Spec.NetworkSpec.PrivateDNSZoneName = restored.Spec.NetworkSpec.PrivateDNSZoneName

	dst.Spec.NetworkSpec.APIServerLB.FrontendIPsCount = restored.Spec.NetworkSpec.APIServerLB.FrontendIPsCount
	dst.Spec.NetworkSpec.APIServerLB.IdleTimeoutInMinutes = restored.Spec.NetworkSpec.APIServerLB.IdleTimeoutInMinutes
	dst.Spec.NetworkSpec.NodeOutboundLB = restored.Spec.NetworkSpec.NodeOutboundLB
	dst.Spec.NetworkSpec.ControlPlaneOutboundLB = restored.Spec.NetworkSpec.ControlPlaneOutboundLB
	dst.Spec.CloudProviderConfigOverrides = restored.Spec.CloudProviderConfigOverrides
	dst.Spec.BastionSpec = restored.Spec.BastionSpec

	// Here we manually restore outbound security rules. Since v1alpha3 only supports ingress ("Inbound") rules, all v1alpha4 outbound rules are dropped when an AzureCluster
	// is converted to v1alpha3. We loop through all security group rules. For all previously existing outbound rules we restore the full rule.
	for _, restoredSubnet := range restored.Spec.NetworkSpec.Subnets {
		for i, dstSubnet := range dst.Spec.NetworkSpec.Subnets {
			if dstSubnet.Name == restoredSubnet.Name {
				var restoredOutboundRules []infrav1alpha4.SecurityRule
				for _, restoredSecurityRule := range restoredSubnet.SecurityGroup.SecurityRules {
					if restoredSecurityRule.Direction != infrav1alpha4.SecurityRuleDirectionInbound {
						// For non-inbound rules which are only supported starting in v1alpha4, we restore the entire rule.
						restoredOutboundRules = append(restoredOutboundRules, restoredSecurityRule)
					}
				}
				dst.Spec.NetworkSpec.Subnets[i].SecurityGroup.SecurityRules = append(dst.Spec.NetworkSpec.Subnets[i].SecurityGroup.SecurityRules, restoredOutboundRules...)
				dst.Spec.NetworkSpec.Subnets[i].NatGateway = restoredSubnet.NatGateway

				break
			}
		}
	}

	return nil
}

// ConvertFrom converts from the Hub version (v1alpha4) to this version.
func (dst *AzureCluster) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha4.AzureCluster)
	if err := Convert_v1alpha4_AzureCluster_To_v1alpha3_AzureCluster(src, dst, nil); err != nil {
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

// ConvertTo converts this AzureClusterList to the Hub version (v1alpha4).
func (src *AzureClusterList) ConvertTo(dstRaw conversion.Hub) error { // nolint
	dst := dstRaw.(*infrav1alpha4.AzureClusterList)
	return Convert_v1alpha3_AzureClusterList_To_v1alpha4_AzureClusterList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1alpha4) to this version.
func (dst *AzureClusterList) ConvertFrom(srcRaw conversion.Hub) error { // nolint
	src := srcRaw.(*infrav1alpha4.AzureClusterList)
	return Convert_v1alpha4_AzureClusterList_To_v1alpha3_AzureClusterList(src, dst, nil)
}

// Convert_v1alpha3_AzureClusterStatus_To_v1alpha4_AzureClusterStatus converts AzureCluster.Status from v1alpha3 to v1alpha4.
func Convert_v1alpha3_AzureClusterStatus_To_v1alpha4_AzureClusterStatus(in *AzureClusterStatus, out *infrav1alpha4.AzureClusterStatus, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha3_AzureClusterStatus_To_v1alpha4_AzureClusterStatus(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha3_AzureClusterSpec_To_v1alpha4_AzureClusterSpec.
func Convert_v1alpha3_AzureClusterSpec_To_v1alpha4_AzureClusterSpec(in *AzureClusterSpec, out *infrav1alpha4.AzureClusterSpec, s apiconversion.Scope) error { //nolint
	if err := autoConvert_v1alpha3_AzureClusterSpec_To_v1alpha4_AzureClusterSpec(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha4_AzureClusterSpec_To_v1alpha3_AzureClusterSpec converts from the Hub version (v1alpha4) of the AzureClusterSpec to this version.
func Convert_v1alpha4_AzureClusterSpec_To_v1alpha3_AzureClusterSpec(in *infrav1alpha4.AzureClusterSpec, out *AzureClusterSpec, s apiconversion.Scope) error { // nolint
	if err := autoConvert_v1alpha4_AzureClusterSpec_To_v1alpha3_AzureClusterSpec(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha4_AzureClusterStatus_To_v1alpha3_AzureClusterStatus.
func Convert_v1alpha4_AzureClusterStatus_To_v1alpha3_AzureClusterStatus(in *infrav1alpha4.AzureClusterStatus, out *AzureClusterStatus, s apiconversion.Scope) error { //nolint
	if err := autoConvert_v1alpha4_AzureClusterStatus_To_v1alpha3_AzureClusterStatus(in, out, s); err != nil {
		return err
	}

	return nil
}

// Convert_v1alpha3_NetworkSpec_To_v1alpha4_NetworkSpec.
func Convert_v1alpha3_NetworkSpec_To_v1alpha4_NetworkSpec(in *NetworkSpec, out *infrav1alpha4.NetworkSpec, s apiconversion.Scope) error { //nolint
	if err := Convert_v1alpha3_VnetSpec_To_v1alpha4_VnetSpec(&in.Vnet, &out.Vnet, s); err != nil {
		return err
	}

	out.Subnets = make(infrav1alpha4.Subnets, len(in.Subnets))
	for i := range in.Subnets {
		out.Subnets[i] = infrav1alpha4.SubnetSpec{}
		if err := Convert_v1alpha3_SubnetSpec_To_v1alpha4_SubnetSpec(&in.Subnets[i], &out.Subnets[i], s); err != nil {
			return err
		}
	}

	if err := autoConvert_v1alpha3_LoadBalancerSpec_To_v1alpha4_LoadBalancerSpec(&in.APIServerLB, &out.APIServerLB, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1alpha4_NetworkSpec_To_v1alpha3_NetworkSpec.
func Convert_v1alpha4_NetworkSpec_To_v1alpha3_NetworkSpec(in *infrav1alpha4.NetworkSpec, out *NetworkSpec, s apiconversion.Scope) error { //nolint
	if err := Convert_v1alpha4_VnetSpec_To_v1alpha3_VnetSpec(&in.Vnet, &out.Vnet, s); err != nil {
		return err
	}

	out.Subnets = make(Subnets, len(in.Subnets))
	for i := range in.Subnets {
		out.Subnets[i] = SubnetSpec{}
		if err := Convert_v1alpha4_SubnetSpec_To_v1alpha3_SubnetSpec(&in.Subnets[i], &out.Subnets[i], s); err != nil {
			return err
		}
	}

	if err := autoConvert_v1alpha4_LoadBalancerSpec_To_v1alpha3_LoadBalancerSpec(&in.APIServerLB, &out.APIServerLB, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1alpha4_VnetSpec_To_v1alpha3_VnetSpec.
func Convert_v1alpha4_VnetSpec_To_v1alpha3_VnetSpec(in *infrav1alpha4.VnetSpec, out *VnetSpec, s apiconversion.Scope) error { //nolint
	return autoConvert_v1alpha4_VnetSpec_To_v1alpha3_VnetSpec(in, out, s)
}

// Convert_v1alpha3_SubnetSpec_To_v1alpha4_SubnetSpec.
func Convert_v1alpha3_SubnetSpec_To_v1alpha4_SubnetSpec(in *SubnetSpec, out *infrav1alpha4.SubnetSpec, s apiconversion.Scope) error { //nolint
	return autoConvert_v1alpha3_SubnetSpec_To_v1alpha4_SubnetSpec(in, out, s)
}

// Convert_v1alpha4_SubnetSpec_To_v1alpha3_SubnetSpec.
func Convert_v1alpha4_SubnetSpec_To_v1alpha3_SubnetSpec(in *infrav1alpha4.SubnetSpec, out *SubnetSpec, s apiconversion.Scope) error { //nolint
	return autoConvert_v1alpha4_SubnetSpec_To_v1alpha3_SubnetSpec(in, out, s)
}

func Convert_v1alpha4_SecurityGroup_To_v1alpha3_SecurityGroup(in *infrav1alpha4.SecurityGroup, out *SecurityGroup, s apiconversion.Scope) error {
	out.ID = in.ID
	out.Name = in.Name

	out.IngressRules = make(IngressRules, 0)
	for _, rule := range in.SecurityRules {
		if rule.Direction == infrav1alpha4.SecurityRuleDirectionInbound { // only inbound rules are supported in v1alpha3.
			ingressRule := IngressRule{}
			if err := Convert_v1alpha4_SecurityRule_To_v1alpha3_IngressRule(&rule, &ingressRule, s); err != nil {
				return err
			}
			out.IngressRules = append(out.IngressRules, ingressRule)
		}
	}

	out.Tags = *(*Tags)(&in.Tags)
	return nil
}

func Convert_v1alpha3_SecurityGroup_To_v1alpha4_SecurityGroup(in *SecurityGroup, out *infrav1alpha4.SecurityGroup, s apiconversion.Scope) error { //nolint
	out.ID = in.ID
	out.Name = in.Name

	out.SecurityRules = make(infrav1alpha4.SecurityRules, len(in.IngressRules))
	for i := range in.IngressRules {
		out.SecurityRules[i] = infrav1alpha4.SecurityRule{}
		if err := Convert_v1alpha3_IngressRule_To_v1alpha4_SecurityRule(&in.IngressRules[i], &out.SecurityRules[i], s); err != nil {
			return err
		}
	}

	out.Tags = *(*infrav1alpha4.Tags)(&in.Tags)
	return nil
}

// Convert_v1alpha3_IngressRule_To_v1alpha4_SecurityRule
func Convert_v1alpha3_IngressRule_To_v1alpha4_SecurityRule(in *IngressRule, out *infrav1alpha4.SecurityRule, _ apiconversion.Scope) error { //nolint
	out.Name = in.Name
	out.Description = in.Description
	out.Protocol = infrav1alpha4.SecurityGroupProtocol(in.Protocol)
	out.Priority = in.Priority
	out.SourcePorts = in.SourcePorts
	out.DestinationPorts = in.DestinationPorts
	out.Source = in.Source
	out.Destination = in.Destination
	out.Direction = infrav1alpha4.SecurityRuleDirectionInbound // all v1alpha3 rules are inbound.
	return nil
}

// Convert_v1alpha4_SecurityRule_To_v1alpha3_IngressRule
func Convert_v1alpha4_SecurityRule_To_v1alpha3_IngressRule(in *infrav1alpha4.SecurityRule, out *IngressRule, _ apiconversion.Scope) error { //nolint
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

// Convert_v1alpha3_APIEndpoint_To_v1alpha4_APIEndpoint is an autogenerated conversion function.
func Convert_v1alpha3_APIEndpoint_To_v1alpha4_APIEndpoint(in *apiv1alpha3.APIEndpoint, out *apiv1alpha4.APIEndpoint, s apiconversion.Scope) error {
	return apiv1alpha3.Convert_v1alpha3_APIEndpoint_To_v1alpha4_APIEndpoint(in, out, s)
}

// Convert_v1alpha4_APIEndpoint_To_v1alpha3_APIEndpoint is an autogenerated conversion function.
func Convert_v1alpha4_APIEndpoint_To_v1alpha3_APIEndpoint(in *apiv1alpha4.APIEndpoint, out *apiv1alpha3.APIEndpoint, s apiconversion.Scope) error {
	return apiv1alpha3.Convert_v1alpha4_APIEndpoint_To_v1alpha3_APIEndpoint(in, out, s)
}

// Convert_v1alpha3_VnetSpec_To_v1alpha4_VnetSpec is an autogenerated conversion function.
func Convert_v1alpha3_VnetSpec_To_v1alpha4_VnetSpec(in *VnetSpec, out *infrav1alpha4.VnetSpec, s apiconversion.Scope) error {
	return autoConvert_v1alpha3_VnetSpec_To_v1alpha4_VnetSpec(in, out, s)
}

// Convert_v1alpha4_LoadBalancerSpec_To_v1alpha3_LoadBalancerSpec is an autogenerated conversion function.
func Convert_v1alpha4_LoadBalancerSpec_To_v1alpha3_LoadBalancerSpec(in *infrav1alpha4.LoadBalancerSpec, out *LoadBalancerSpec, s apiconversion.Scope) error {
	return autoConvert_v1alpha4_LoadBalancerSpec_To_v1alpha3_LoadBalancerSpec(in, out, s)
}

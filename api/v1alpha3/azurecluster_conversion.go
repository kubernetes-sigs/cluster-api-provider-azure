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
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

const (
	azureEnvironmentAnnotation = "azurecluster.infrastructure.cluster.x-k8s.io/azureEnvironment"
)

// ConvertTo converts this AzureCluster to the Hub version (v1beta1).
func (src *AzureCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.AzureCluster)
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

	// set default control plane outbound lb for private v1alpha3 clusters.
	if src.Spec.NetworkSpec.APIServerLB.Type == Internal {
		dst.Spec.NetworkSpec.ControlPlaneOutboundLB = &infrav1.LoadBalancerSpec{
			FrontendIPsCount: pointer.Int32Ptr(1),
		}
		// We also need to set the defaults here because "get" won't set defaults, and hence there is no mismatch when a client
		// gets a v1alpha3 cluster.
		dst.SetControlPlaneOutboundLBDefaults()
	}

	// set default node plane outbound lb for all v1alpha3 clusters.
	dst.Spec.NetworkSpec.NodeOutboundLB = &infrav1.LoadBalancerSpec{
		FrontendIPsCount: pointer.Int32Ptr(1),
	}
	// We also need to set the defaults here because "get" won't set defaults, and hence there is no mismatch when a client
	// gets a v1alpha3 cluster.
	dst.SetNodeOutboundLBDefaults()

	// Manually restore data.
	restored := &infrav1.AzureCluster{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	// override outbound lb if it's present in restored.
	dst.Spec.NetworkSpec.ControlPlaneOutboundLB = restored.Spec.NetworkSpec.ControlPlaneOutboundLB
	dst.Spec.NetworkSpec.NodeOutboundLB = restored.Spec.NetworkSpec.NodeOutboundLB

	dst.Spec.NetworkSpec.PrivateDNSZoneName = restored.Spec.NetworkSpec.PrivateDNSZoneName

	dst.Spec.NetworkSpec.APIServerLB.FrontendIPsCount = restored.Spec.NetworkSpec.APIServerLB.FrontendIPsCount
	dst.Spec.NetworkSpec.APIServerLB.IdleTimeoutInMinutes = restored.Spec.NetworkSpec.APIServerLB.IdleTimeoutInMinutes

	for _, restoredFrontendIP := range restored.Spec.NetworkSpec.APIServerLB.FrontendIPs {
		for i, dstFrontendIP := range dst.Spec.NetworkSpec.APIServerLB.FrontendIPs {
			if restoredFrontendIP.Name == dstFrontendIP.Name && restoredFrontendIP.PublicIP != nil {
				dst.Spec.NetworkSpec.APIServerLB.FrontendIPs[i].PublicIP.IPTags = restoredFrontendIP.PublicIP.IPTags
			}
		}
	}

	dst.Spec.CloudProviderConfigOverrides = restored.Spec.CloudProviderConfigOverrides
	dst.Spec.BastionSpec = restored.Spec.BastionSpec

	// Here we manually restore outbound security rules. Since v1alpha3 only supports ingress ("Inbound") rules, all v1alpha4/v1beta1 outbound rules are dropped when an AzureCluster
	// is converted to v1alpha3. We loop through all security group rules. For all previously existing outbound rules we restore the full rule.
	for _, restoredSubnet := range restored.Spec.NetworkSpec.Subnets {
		for i, dstSubnet := range dst.Spec.NetworkSpec.Subnets {
			if dstSubnet.Name == restoredSubnet.Name {
				var restoredOutboundRules []infrav1.SecurityRule
				for _, restoredSecurityRule := range restoredSubnet.SecurityGroup.SecurityRules {
					if restoredSecurityRule.Direction != infrav1.SecurityRuleDirectionInbound {
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
func (dst *AzureCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.AzureCluster)
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
	return utilconversion.MarshalData(src, dst)
}

// ConvertTo converts this AzureClusterList to the Hub version (v1beta1).
func (src *AzureClusterList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.AzureClusterList)
	return Convert_v1alpha3_AzureClusterList_To_v1beta1_AzureClusterList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureClusterList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.AzureClusterList)
	return Convert_v1beta1_AzureClusterList_To_v1alpha3_AzureClusterList(src, dst, nil)
}

// Convert_v1alpha3_AzureClusterStatus_To_v1beta1_AzureClusterStatus converts AzureCluster.Status from v1alpha3 to v1beta1.
func Convert_v1alpha3_AzureClusterStatus_To_v1beta1_AzureClusterStatus(in *AzureClusterStatus, out *infrav1.AzureClusterStatus, s apiconversion.Scope) error {
	return autoConvert_v1alpha3_AzureClusterStatus_To_v1beta1_AzureClusterStatus(in, out, s)
}

// Convert_v1alpha3_AzureClusterSpec_To_v1beta1_AzureClusterSpec.
func Convert_v1alpha3_AzureClusterSpec_To_v1beta1_AzureClusterSpec(in *AzureClusterSpec, out *infrav1.AzureClusterSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha3_AzureClusterSpec_To_v1beta1_AzureClusterSpec(in, out, s); err != nil {
		return err
	}

	// copy AzureClusterClassSpec fields
	out.SubscriptionID = in.SubscriptionID
	out.Location = in.Location
	out.AdditionalTags = *(*infrav1.Tags)(&in.AdditionalTags)
	out.IdentityRef = in.IdentityRef

	return nil
}

// Convert_v1beta1_AzureClusterSpec_To_v1alpha3_AzureClusterSpec converts from the Hub version (v1beta1) of the AzureClusterSpec to this version.
func Convert_v1beta1_AzureClusterSpec_To_v1alpha3_AzureClusterSpec(in *infrav1.AzureClusterSpec, out *AzureClusterSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_AzureClusterSpec_To_v1alpha3_AzureClusterSpec(in, out, s); err != nil {
		return err
	}

	// copy AzureClusterClassSpec fields
	out.SubscriptionID = in.SubscriptionID
	out.Location = in.Location
	out.AdditionalTags = Tags(in.AdditionalTags)
	out.IdentityRef = in.IdentityRef

	return nil
}

// Convert_v1beta1_AzureClusterStatus_To_v1alpha3_AzureClusterStatus.
func Convert_v1beta1_AzureClusterStatus_To_v1alpha3_AzureClusterStatus(in *infrav1.AzureClusterStatus, out *AzureClusterStatus, s apiconversion.Scope) error {
	return autoConvert_v1beta1_AzureClusterStatus_To_v1alpha3_AzureClusterStatus(in, out, s)
}

// Convert_v1alpha3_NetworkSpec_To_v1beta1_NetworkSpec.
func Convert_v1alpha3_NetworkSpec_To_v1beta1_NetworkSpec(in *NetworkSpec, out *infrav1.NetworkSpec, s apiconversion.Scope) error {
	if err := Convert_v1alpha3_VnetSpec_To_v1beta1_VnetSpec(&in.Vnet, &out.Vnet, s); err != nil {
		return err
	}

	out.Subnets = make(infrav1.Subnets, len(in.Subnets))
	for i := range in.Subnets {
		out.Subnets[i] = infrav1.SubnetSpec{}
		if err := Convert_v1alpha3_SubnetSpec_To_v1beta1_SubnetSpec(&in.Subnets[i], &out.Subnets[i], s); err != nil {
			return err
		}
	}

	return Convert_v1alpha3_LoadBalancerSpec_To_v1beta1_LoadBalancerSpec(&in.APIServerLB, &out.APIServerLB, s)
}

// Convert_v1beta1_NetworkSpec_To_v1alpha3_NetworkSpec.
func Convert_v1beta1_NetworkSpec_To_v1alpha3_NetworkSpec(in *infrav1.NetworkSpec, out *NetworkSpec, s apiconversion.Scope) error {
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

	return Convert_v1beta1_LoadBalancerSpec_To_v1alpha3_LoadBalancerSpec(&in.APIServerLB, &out.APIServerLB, s)
}

// Convert_v1beta1_VnetSpec_To_v1alpha3_VnetSpec.
func Convert_v1beta1_VnetSpec_To_v1alpha3_VnetSpec(in *infrav1.VnetSpec, out *VnetSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_VnetSpec_To_v1alpha3_VnetSpec(in, out, s); err != nil {
		return err
	}

	// copy VnetClassSpec fields
	out.CIDRBlocks = in.CIDRBlocks
	out.Tags = Tags(in.Tags)

	return nil
}

// Convert_v1alpha3_SubnetSpec_To_v1beta1_SubnetSpec.
func Convert_v1alpha3_SubnetSpec_To_v1beta1_SubnetSpec(in *SubnetSpec, out *infrav1.SubnetSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha3_SubnetSpec_To_v1beta1_SubnetSpec(in, out, s); err != nil {
		return err
	}

	// Convert SubnetClassSpec fields
	out.Role = infrav1.SubnetRole(in.Role)
	out.CIDRBlocks = in.CIDRBlocks

	return nil
}

// Convert_v1beta1_SubnetSpec_To_v1alpha3_SubnetSpec.
func Convert_v1beta1_SubnetSpec_To_v1alpha3_SubnetSpec(in *infrav1.SubnetSpec, out *SubnetSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_SubnetSpec_To_v1alpha3_SubnetSpec(in, out, s); err != nil {
		return err
	}

	// Convert SubnetClassSpec fields
	out.Role = SubnetRole(in.Role)
	out.CIDRBlocks = in.CIDRBlocks

	return nil
}

func Convert_v1beta1_SecurityGroup_To_v1alpha3_SecurityGroup(in *infrav1.SecurityGroup, out *SecurityGroup, s apiconversion.Scope) error {
	out.ID = in.ID
	out.Name = in.Name

	out.IngressRules = make(IngressRules, 0)
	for _, rule := range in.SecurityRules {
		rule := rule
		if rule.Direction == infrav1.SecurityRuleDirectionInbound { // only inbound rules are supported in v1alpha3.
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

func Convert_v1alpha3_SecurityGroup_To_v1beta1_SecurityGroup(in *SecurityGroup, out *infrav1.SecurityGroup, s apiconversion.Scope) error {
	out.ID = in.ID
	out.Name = in.Name

	out.SecurityRules = make(infrav1.SecurityRules, len(in.IngressRules))
	for i := range in.IngressRules {
		out.SecurityRules[i] = infrav1.SecurityRule{}
		if err := Convert_v1alpha3_IngressRule_To_v1beta1_SecurityRule(&in.IngressRules[i], &out.SecurityRules[i], s); err != nil {
			return err
		}
	}

	out.Tags = *(*infrav1.Tags)(&in.Tags)
	return nil
}

// Convert_v1alpha3_IngressRule_To_v1beta1_SecurityRule.
func Convert_v1alpha3_IngressRule_To_v1beta1_SecurityRule(in *IngressRule, out *infrav1.SecurityRule, _ apiconversion.Scope) error {
	out.Name = in.Name
	out.Description = in.Description
	out.Protocol = infrav1.SecurityGroupProtocol(in.Protocol)
	out.Priority = in.Priority
	out.SourcePorts = in.SourcePorts
	out.DestinationPorts = in.DestinationPorts
	out.Source = in.Source
	out.Destination = in.Destination
	out.Direction = infrav1.SecurityRuleDirectionInbound // all v1alpha3 rules are inbound.
	return nil
}

// Convert_v1beta1_SecurityRule_To_v1alpha3_IngressRule.
func Convert_v1beta1_SecurityRule_To_v1alpha3_IngressRule(in *infrav1.SecurityRule, out *IngressRule, _ apiconversion.Scope) error {
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
func Convert_v1alpha3_VnetSpec_To_v1beta1_VnetSpec(in *VnetSpec, out *infrav1.VnetSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha3_VnetSpec_To_v1beta1_VnetSpec(in, out, s); err != nil {
		return err
	}

	// copy VnetClassSpec fields
	out.CIDRBlocks = in.CIDRBlocks
	out.Tags = *(*infrav1.Tags)(&in.Tags)

	return nil
}

// Convert_v1beta1_LoadBalancerSpec_To_v1alpha3_LoadBalancerSpec is an autogenerated conversion function.
func Convert_v1beta1_LoadBalancerSpec_To_v1alpha3_LoadBalancerSpec(in *infrav1.LoadBalancerSpec, out *LoadBalancerSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_LoadBalancerSpec_To_v1alpha3_LoadBalancerSpec(in, out, s); err != nil {
		return err
	}

	// Convert LoadBalancerClassSpec fields
	out.SKU = SKU(in.SKU)
	if in.FrontendIPs != nil {
		in, out := &in.FrontendIPs, &out.FrontendIPs
		*out = make([]FrontendIP, len(*in))
		for i := range *in {
			if err := Convert_v1beta1_FrontendIP_To_v1alpha3_FrontendIP(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.FrontendIPs = nil
	}
	out.Type = LBType(in.Type)

	return nil
}

func Convert_v1alpha3_LoadBalancerSpec_To_v1beta1_LoadBalancerSpec(in *LoadBalancerSpec, out *infrav1.LoadBalancerSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha3_LoadBalancerSpec_To_v1beta1_LoadBalancerSpec(in, out, s); err != nil {
		return err
	}

	// Convert LoadBalancerClassSpec fields
	out.SKU = infrav1.SKU(in.SKU)
	if in.FrontendIPs != nil {
		in, out := &in.FrontendIPs, &out.FrontendIPs
		*out = make([]infrav1.FrontendIP, len(*in))
		for i := range *in {
			if err := Convert_v1alpha3_FrontendIP_To_v1beta1_FrontendIP(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.FrontendIPs = nil
	}
	out.Type = infrav1.LBType(in.Type)

	return nil
}

// Convert_v1alpha3_Future_To_v1beta1_Future is an autogenerated conversion function.
func Convert_v1alpha3_Future_To_v1beta1_Future(in *Future, out *infrav1.Future, s apiconversion.Scope) error {
	out.Data = in.FutureData
	return autoConvert_v1alpha3_Future_To_v1beta1_Future(in, out, s)
}

// Convert_v1beta1_Future_To_v1alpha3_Future is an autogenerated conversion function.
func Convert_v1beta1_Future_To_v1alpha3_Future(in *infrav1.Future, out *Future, s apiconversion.Scope) error {
	out.FutureData = in.Data
	return autoConvert_v1beta1_Future_To_v1alpha3_Future(in, out, s)
}

// Convert_v1alpha3_FrontendIP_To_v1beta1_FrontendIP is an autogenerated conversion function.
func Convert_v1alpha3_FrontendIP_To_v1beta1_FrontendIP(in *FrontendIP, out *infrav1.FrontendIP, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha3_FrontendIP_To_v1beta1_FrontendIP(in, out, s); err != nil {
		return err
	}

	// Convert FrontendIPClass fields
	out.PrivateIPAddress = in.PrivateIPAddress

	return nil
}

// Convert_v1beta1_FrontendIP_To_v1alpha3_FrontendIP is an autogenerated conversion function.
func Convert_v1beta1_FrontendIP_To_v1alpha3_FrontendIP(in *infrav1.FrontendIP, out *FrontendIP, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_FrontendIP_To_v1alpha3_FrontendIP(in, out, s); err != nil {
		return err
	}

	// Convert FrontendIPClass fields
	out.PrivateIPAddress = in.PrivateIPAddress

	return nil
}

// Convert_v1beta1_PublicIPSpec_To_v1alpha3_PublicIPSpec is an autogenerated conversion function.
func Convert_v1beta1_PublicIPSpec_To_v1alpha3_PublicIPSpec(in *infrav1.PublicIPSpec, out *PublicIPSpec, s apiconversion.Scope) error {
	return autoConvert_v1beta1_PublicIPSpec_To_v1alpha3_PublicIPSpec(in, out, s)
}

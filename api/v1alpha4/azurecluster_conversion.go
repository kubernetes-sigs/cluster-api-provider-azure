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
	"unsafe"

	apiconversion "k8s.io/apimachinery/pkg/conversion"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// ConvertTo converts this AzureCluster to the Hub version (v1beta1).
func (src *AzureCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.AzureCluster)
	if err := Convert_v1alpha4_AzureCluster_To_v1beta1_AzureCluster(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data.
	restored := &infrav1.AzureCluster{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	// Restore list of virtual network peerings
	dst.Spec.NetworkSpec.Vnet.Peerings = restored.Spec.NetworkSpec.Vnet.Peerings

	// Restore API Server LB IP tags.
	for _, restoredFrontendIP := range restored.Spec.NetworkSpec.APIServerLB.FrontendIPs {
		for i, dstFrontendIP := range dst.Spec.NetworkSpec.APIServerLB.FrontendIPs {
			if restoredFrontendIP.Name == dstFrontendIP.Name && restoredFrontendIP.PublicIP != nil {
				dst.Spec.NetworkSpec.APIServerLB.FrontendIPs[i].PublicIP.IPTags = restoredFrontendIP.PublicIP.IPTags
			}
		}
	}

	// Restore outbound LB IP tags.
	if restored.Spec.NetworkSpec.ControlPlaneOutboundLB != nil {
		for _, restoredFrontendIP := range restored.Spec.NetworkSpec.ControlPlaneOutboundLB.FrontendIPs {
			for i, dstFrontendIP := range dst.Spec.NetworkSpec.ControlPlaneOutboundLB.FrontendIPs {
				if restoredFrontendIP.Name == dstFrontendIP.Name && restoredFrontendIP.PublicIP != nil {
					dst.Spec.NetworkSpec.ControlPlaneOutboundLB.FrontendIPs[i].PublicIP.IPTags = restoredFrontendIP.PublicIP.IPTags
				}
			}
		}
	}
	if restored.Spec.NetworkSpec.NodeOutboundLB != nil {
		for _, restoredFrontendIP := range restored.Spec.NetworkSpec.NodeOutboundLB.FrontendIPs {
			for i, dstFrontendIP := range dst.Spec.NetworkSpec.NodeOutboundLB.FrontendIPs {
				if restoredFrontendIP.Name == dstFrontendIP.Name && restoredFrontendIP.PublicIP != nil {
					dst.Spec.NetworkSpec.NodeOutboundLB.FrontendIPs[i].PublicIP.IPTags = restoredFrontendIP.PublicIP.IPTags
				}
			}
		}
	}

	// Restore NAT Gateway IP tags.
	for _, restoredSubnet := range restored.Spec.NetworkSpec.Subnets {
		for i, dstSubnet := range dst.Spec.NetworkSpec.Subnets {
			if dstSubnet.Name == restoredSubnet.Name {
				dst.Spec.NetworkSpec.Subnets[i].NatGateway.NatGatewayIP.IPTags = restoredSubnet.NatGateway.NatGatewayIP.IPTags
			}
		}
	}

	// Restore Azure Bastion IP tags.
	if restored.Spec.BastionSpec.AzureBastion != nil && dst.Spec.BastionSpec.AzureBastion != nil {
		if restored.Spec.BastionSpec.AzureBastion.PublicIP.Name == dst.Spec.BastionSpec.AzureBastion.PublicIP.Name {
			dst.Spec.BastionSpec.AzureBastion.PublicIP.IPTags = restored.Spec.BastionSpec.AzureBastion.PublicIP.IPTags
		}
		if restored.Spec.BastionSpec.AzureBastion.Subnet.NatGateway.NatGatewayIP.Name == dst.Spec.BastionSpec.AzureBastion.Subnet.NatGateway.NatGatewayIP.Name {
			dst.Spec.BastionSpec.AzureBastion.Subnet.NatGateway.NatGatewayIP.IPTags = restored.Spec.BastionSpec.AzureBastion.Subnet.NatGateway.NatGatewayIP.IPTags
		}
	}

	return nil
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.AzureCluster)
	if err := Convert_v1beta1_AzureCluster_To_v1alpha4_AzureCluster(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion.
	return utilconversion.MarshalData(src, dst)
}

// ConvertTo converts this AzureClusterList to the Hub version (v1beta1).
func (src *AzureClusterList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.AzureClusterList)
	return Convert_v1alpha4_AzureClusterList_To_v1beta1_AzureClusterList(src, dst, nil)
}

// ConvertFrom converts from the Hub version (v1beta1) to this version.
func (dst *AzureClusterList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.AzureClusterList)
	return Convert_v1beta1_AzureClusterList_To_v1alpha4_AzureClusterList(src, dst, nil)
}

// Convert_v1beta1_VnetSpec_To_v1alpha4_VnetSpec.
func Convert_v1beta1_VnetSpec_To_v1alpha4_VnetSpec(in *infrav1.VnetSpec, out *VnetSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_VnetSpec_To_v1alpha4_VnetSpec(in, out, s); err != nil {
		return err
	}

	// Convert VnetClassSpec fields
	out.CIDRBlocks = in.CIDRBlocks
	out.Tags = *(*Tags)(&in.Tags)

	return nil
}

// Convert_v1alpha4_VnetSpec_To_v1beta1_VnetSpec is an autogenerated conversion function.
func Convert_v1alpha4_VnetSpec_To_v1beta1_VnetSpec(in *VnetSpec, out *infrav1.VnetSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha4_VnetSpec_To_v1beta1_VnetSpec(in, out, s); err != nil {
		return err
	}

	// Convert VnetClassSpec fields
	out.CIDRBlocks = in.CIDRBlocks
	out.Tags = *(*infrav1.Tags)(&in.Tags)

	return nil
}

// Convert_v1alpha4_AzureClusterSpec_To_v1beta1_AzureClusterSpec is an autogenerated conversion function.
func Convert_v1alpha4_AzureClusterSpec_To_v1beta1_AzureClusterSpec(in *AzureClusterSpec, out *infrav1.AzureClusterSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha4_AzureClusterSpec_To_v1beta1_AzureClusterSpec(in, out, s); err != nil {
		return err
	}

	// Convert AzureClusterClassSpec fields
	out.SubscriptionID = in.SubscriptionID
	out.Location = in.Location
	out.AdditionalTags = *(*infrav1.Tags)(&in.AdditionalTags)
	out.IdentityRef = in.IdentityRef
	out.AzureEnvironment = in.AzureEnvironment
	out.CloudProviderConfigOverrides = (*infrav1.CloudProviderConfigOverrides)(unsafe.Pointer(in.CloudProviderConfigOverrides))

	return nil
}

// Convert_v1beta1_AzureClusterSpec_To_v1alpha4_AzureClusterSpec is an autogenerated conversion function.
func Convert_v1beta1_AzureClusterSpec_To_v1alpha4_AzureClusterSpec(in *infrav1.AzureClusterSpec, out *AzureClusterSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_AzureClusterSpec_To_v1alpha4_AzureClusterSpec(in, out, s); err != nil {
		return err
	}

	// Convert AzureClusterClassSpec fields
	out.SubscriptionID = in.SubscriptionID
	out.Location = in.Location
	out.AdditionalTags = Tags(in.AdditionalTags)
	out.IdentityRef = in.IdentityRef
	out.AzureEnvironment = in.AzureEnvironment
	out.CloudProviderConfigOverrides = (*CloudProviderConfigOverrides)(unsafe.Pointer(in.CloudProviderConfigOverrides))

	return nil
}

// Convert_v1alpha4_FrontendIP_To_v1beta1_FrontendIP is an autogenerated conversion function.
func Convert_v1alpha4_FrontendIP_To_v1beta1_FrontendIP(in *FrontendIP, out *infrav1.FrontendIP, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha4_FrontendIP_To_v1beta1_FrontendIP(in, out, s); err != nil {
		return err
	}

	// Convert FrontendIPClass fields
	out.PrivateIPAddress = in.PrivateIPAddress

	return nil
}

// Convert_v1beta1_FrontendIP_To_v1alpha4_FrontendIP is an autogenerated conversion function.
func Convert_v1beta1_FrontendIP_To_v1alpha4_FrontendIP(in *infrav1.FrontendIP, out *FrontendIP, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_FrontendIP_To_v1alpha4_FrontendIP(in, out, s); err != nil {
		return err
	}

	// Convert FrontendIPClass fields
	out.PrivateIPAddress = in.PrivateIPAddress

	return nil
}

// Convert_v1alpha4_LoadBalancerSpec_To_v1beta1_LoadBalancerSpec is an autogenerated conversion function.
func Convert_v1alpha4_LoadBalancerSpec_To_v1beta1_LoadBalancerSpec(in *LoadBalancerSpec, out *infrav1.LoadBalancerSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha4_LoadBalancerSpec_To_v1beta1_LoadBalancerSpec(in, out, s); err != nil {
		return err
	}

	// Convert LoadBalancerClassSpec fields
	out.SKU = infrav1.SKU(in.SKU)
	if in.FrontendIPs != nil {
		in, out := &in.FrontendIPs, &out.FrontendIPs
		*out = make([]infrav1.FrontendIP, len(*in))
		for i := range *in {
			if err := Convert_v1alpha4_FrontendIP_To_v1beta1_FrontendIP(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.FrontendIPs = nil
	}
	out.Type = infrav1.LBType(in.Type)
	out.FrontendIPsCount = in.FrontendIPsCount
	out.IdleTimeoutInMinutes = in.IdleTimeoutInMinutes

	return nil
}

// Convert_v1beta1_LoadBalancerSpec_To_v1alpha4_LoadBalancerSpec is an autogenerated conversion function.
func Convert_v1beta1_LoadBalancerSpec_To_v1alpha4_LoadBalancerSpec(in *infrav1.LoadBalancerSpec, out *LoadBalancerSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_LoadBalancerSpec_To_v1alpha4_LoadBalancerSpec(in, out, s); err != nil {
		return err
	}

	// Convert LoadBalancerClassSpec fields
	out.SKU = SKU(in.SKU)
	if in.FrontendIPs != nil {
		in, out := &in.FrontendIPs, &out.FrontendIPs
		*out = make([]FrontendIP, len(*in))
		for i := range *in {
			if err := Convert_v1beta1_FrontendIP_To_v1alpha4_FrontendIP(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.FrontendIPs = nil
	}
	out.Type = LBType(in.Type)
	out.FrontendIPsCount = in.FrontendIPsCount
	out.IdleTimeoutInMinutes = in.IdleTimeoutInMinutes

	return nil
}

// Convert_v1alpha4_NetworkSpec_To_v1beta1_NetworkSpec is an autogenerated conversion function.
func Convert_v1alpha4_NetworkSpec_To_v1beta1_NetworkSpec(in *NetworkSpec, out *infrav1.NetworkSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha4_NetworkSpec_To_v1beta1_NetworkSpec(in, out, s); err != nil {
		return err
	}

	// Convert NetworkClassSpec fields
	out.PrivateDNSZoneName = in.PrivateDNSZoneName

	return nil
}

// Convert_v1beta1_NetworkSpec_To_v1alpha4_NetworkSpec is an autogenerated conversion function.
func Convert_v1beta1_NetworkSpec_To_v1alpha4_NetworkSpec(in *infrav1.NetworkSpec, out *NetworkSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_NetworkSpec_To_v1alpha4_NetworkSpec(in, out, s); err != nil {
		return err
	}

	// Convert NetworkClassSpec fields
	out.PrivateDNSZoneName = in.PrivateDNSZoneName

	return nil
}

// Convert_v1alpha4_SubnetSpec_To_v1beta1_SubnetSpec is an autogenerated conversion function.
func Convert_v1alpha4_SubnetSpec_To_v1beta1_SubnetSpec(in *SubnetSpec, out *infrav1.SubnetSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha4_SubnetSpec_To_v1beta1_SubnetSpec(in, out, s); err != nil {
		return err
	}

	// Convert SubnetClassSpec fields
	out.Role = infrav1.SubnetRole(in.Role)
	out.CIDRBlocks = in.CIDRBlocks

	return nil
}

// Convert_v1beta1_SubnetSpec_To_v1alpha4_SubnetSpec is an autogenerated conversion function.
func Convert_v1beta1_SubnetSpec_To_v1alpha4_SubnetSpec(in *infrav1.SubnetSpec, out *SubnetSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_SubnetSpec_To_v1alpha4_SubnetSpec(in, out, s); err != nil {
		return err
	}

	// Convert SubnetClassSpec fields
	out.Role = SubnetRole(in.Role)
	out.CIDRBlocks = in.CIDRBlocks

	return nil
}

// Convert_v1alpha4_SecurityGroup_To_v1beta1_SecurityGroup is an autogenerated conversion function.
func Convert_v1alpha4_SecurityGroup_To_v1beta1_SecurityGroup(in *SecurityGroup, out *infrav1.SecurityGroup, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha4_SecurityGroup_To_v1beta1_SecurityGroup(in, out, s); err != nil {
		return err
	}

	// Convert SecurityGroupClass fields
	out.SecurityRules = *(*infrav1.SecurityRules)(unsafe.Pointer(&in.SecurityRules))
	out.Tags = *(*infrav1.Tags)(&in.Tags)

	return nil
}

// Convert_v1beta1_SecurityGroup_To_v1alpha4_SecurityGroup is an autogenerated conversion function.
func Convert_v1beta1_SecurityGroup_To_v1alpha4_SecurityGroup(in *infrav1.SecurityGroup, out *SecurityGroup, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_SecurityGroup_To_v1alpha4_SecurityGroup(in, out, s); err != nil {
		return err
	}

	// Convert SecurityGroupClass fields
	out.SecurityRules = *(*SecurityRules)(unsafe.Pointer(&in.SecurityRules))
	out.Tags = *(*Tags)(&in.Tags)

	return nil
}

func Convert_v1alpha4_NatGateway_To_v1beta1_NatGateway(in *NatGateway, out *infrav1.NatGateway, s apiconversion.Scope) error {
	if err := autoConvert_v1alpha4_NatGateway_To_v1beta1_NatGateway(in, out, s); err != nil {
		return err
	}

	// Convert Name field
	out.Name = in.Name
	return nil
}

func Convert_v1beta1_NatGateway_To_v1alpha4_NatGateway(in *infrav1.NatGateway, out *NatGateway, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_NatGateway_To_v1alpha4_NatGateway(in, out, s); err != nil {
		return err
	}

	// Convert Name field
	out.Name = in.Name
	return nil
}

// Convert_v1beta1_PublicIPSpec_To_v1alpha4_PublicIPSpec is an autogenerated conversion function.
func Convert_v1beta1_PublicIPSpec_To_v1alpha4_PublicIPSpec(in *infrav1.PublicIPSpec, out *PublicIPSpec, s apiconversion.Scope) error {
	return autoConvert_v1beta1_PublicIPSpec_To_v1alpha4_PublicIPSpec(in, out, s)
}

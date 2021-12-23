//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright The Kubernetes Authors.

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha4

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	apiv1alpha4 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	cluster_apiapiv1alpha4 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/errors"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AADProfile) DeepCopyInto(out *AADProfile) {
	*out = *in
	if in.AdminGroupObjectIDs != nil {
		in, out := &in.AdminGroupObjectIDs, &out.AdminGroupObjectIDs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AADProfile.
func (in *AADProfile) DeepCopy() *AADProfile {
	if in == nil {
		return nil
	}
	out := new(AADProfile)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *APIServerAccessProfile) DeepCopyInto(out *APIServerAccessProfile) {
	*out = *in
	if in.AuthorizedIPRanges != nil {
		in, out := &in.AuthorizedIPRanges, &out.AuthorizedIPRanges
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.EnablePrivateCluster != nil {
		in, out := &in.EnablePrivateCluster, &out.EnablePrivateCluster
		*out = new(bool)
		**out = **in
	}
	if in.PrivateDNSZone != nil {
		in, out := &in.PrivateDNSZone, &out.PrivateDNSZone
		*out = new(string)
		**out = **in
	}
	if in.EnablePrivateClusterPublicFQDN != nil {
		in, out := &in.EnablePrivateClusterPublicFQDN, &out.EnablePrivateClusterPublicFQDN
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new APIServerAccessProfile.
func (in *APIServerAccessProfile) DeepCopy() *APIServerAccessProfile {
	if in == nil {
		return nil
	}
	out := new(APIServerAccessProfile)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureMachinePool) DeepCopyInto(out *AzureMachinePool) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureMachinePool.
func (in *AzureMachinePool) DeepCopy() *AzureMachinePool {
	if in == nil {
		return nil
	}
	out := new(AzureMachinePool)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AzureMachinePool) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureMachinePoolDeploymentStrategy) DeepCopyInto(out *AzureMachinePoolDeploymentStrategy) {
	*out = *in
	if in.RollingUpdate != nil {
		in, out := &in.RollingUpdate, &out.RollingUpdate
		*out = new(MachineRollingUpdateDeployment)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureMachinePoolDeploymentStrategy.
func (in *AzureMachinePoolDeploymentStrategy) DeepCopy() *AzureMachinePoolDeploymentStrategy {
	if in == nil {
		return nil
	}
	out := new(AzureMachinePoolDeploymentStrategy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureMachinePoolInstanceStatus) DeepCopyInto(out *AzureMachinePoolInstanceStatus) {
	*out = *in
	if in.ProvisioningState != nil {
		in, out := &in.ProvisioningState, &out.ProvisioningState
		*out = new(apiv1alpha4.ProvisioningState)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureMachinePoolInstanceStatus.
func (in *AzureMachinePoolInstanceStatus) DeepCopy() *AzureMachinePoolInstanceStatus {
	if in == nil {
		return nil
	}
	out := new(AzureMachinePoolInstanceStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureMachinePoolList) DeepCopyInto(out *AzureMachinePoolList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AzureMachinePool, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureMachinePoolList.
func (in *AzureMachinePoolList) DeepCopy() *AzureMachinePoolList {
	if in == nil {
		return nil
	}
	out := new(AzureMachinePoolList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AzureMachinePoolList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureMachinePoolMachine) DeepCopyInto(out *AzureMachinePoolMachine) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureMachinePoolMachine.
func (in *AzureMachinePoolMachine) DeepCopy() *AzureMachinePoolMachine {
	if in == nil {
		return nil
	}
	out := new(AzureMachinePoolMachine)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AzureMachinePoolMachine) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureMachinePoolMachineList) DeepCopyInto(out *AzureMachinePoolMachineList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AzureMachinePoolMachine, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureMachinePoolMachineList.
func (in *AzureMachinePoolMachineList) DeepCopy() *AzureMachinePoolMachineList {
	if in == nil {
		return nil
	}
	out := new(AzureMachinePoolMachineList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AzureMachinePoolMachineList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureMachinePoolMachineSpec) DeepCopyInto(out *AzureMachinePoolMachineSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureMachinePoolMachineSpec.
func (in *AzureMachinePoolMachineSpec) DeepCopy() *AzureMachinePoolMachineSpec {
	if in == nil {
		return nil
	}
	out := new(AzureMachinePoolMachineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureMachinePoolMachineStatus) DeepCopyInto(out *AzureMachinePoolMachineStatus) {
	*out = *in
	if in.NodeRef != nil {
		in, out := &in.NodeRef, &out.NodeRef
		*out = new(corev1.ObjectReference)
		**out = **in
	}
	if in.ProvisioningState != nil {
		in, out := &in.ProvisioningState, &out.ProvisioningState
		*out = new(apiv1alpha4.ProvisioningState)
		**out = **in
	}
	if in.FailureReason != nil {
		in, out := &in.FailureReason, &out.FailureReason
		*out = new(errors.MachineStatusError)
		**out = **in
	}
	if in.FailureMessage != nil {
		in, out := &in.FailureMessage, &out.FailureMessage
		*out = new(string)
		**out = **in
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make(cluster_apiapiv1alpha4.Conditions, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.LongRunningOperationStates != nil {
		in, out := &in.LongRunningOperationStates, &out.LongRunningOperationStates
		*out = make(apiv1alpha4.Futures, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureMachinePoolMachineStatus.
func (in *AzureMachinePoolMachineStatus) DeepCopy() *AzureMachinePoolMachineStatus {
	if in == nil {
		return nil
	}
	out := new(AzureMachinePoolMachineStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureMachinePoolMachineTemplate) DeepCopyInto(out *AzureMachinePoolMachineTemplate) {
	*out = *in
	if in.Image != nil {
		in, out := &in.Image, &out.Image
		*out = new(apiv1alpha4.Image)
		(*in).DeepCopyInto(*out)
	}
	in.OSDisk.DeepCopyInto(&out.OSDisk)
	if in.DataDisks != nil {
		in, out := &in.DataDisks, &out.DataDisks
		*out = make([]apiv1alpha4.DataDisk, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.AcceleratedNetworking != nil {
		in, out := &in.AcceleratedNetworking, &out.AcceleratedNetworking
		*out = new(bool)
		**out = **in
	}
	if in.TerminateNotificationTimeout != nil {
		in, out := &in.TerminateNotificationTimeout, &out.TerminateNotificationTimeout
		*out = new(int)
		**out = **in
	}
	if in.SecurityProfile != nil {
		in, out := &in.SecurityProfile, &out.SecurityProfile
		*out = new(apiv1alpha4.SecurityProfile)
		(*in).DeepCopyInto(*out)
	}
	if in.SpotVMOptions != nil {
		in, out := &in.SpotVMOptions, &out.SpotVMOptions
		*out = new(apiv1alpha4.SpotVMOptions)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureMachinePoolMachineTemplate.
func (in *AzureMachinePoolMachineTemplate) DeepCopy() *AzureMachinePoolMachineTemplate {
	if in == nil {
		return nil
	}
	out := new(AzureMachinePoolMachineTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureMachinePoolSpec) DeepCopyInto(out *AzureMachinePoolSpec) {
	*out = *in
	in.Template.DeepCopyInto(&out.Template)
	if in.AdditionalTags != nil {
		in, out := &in.AdditionalTags, &out.AdditionalTags
		*out = make(apiv1alpha4.Tags, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.ProviderIDList != nil {
		in, out := &in.ProviderIDList, &out.ProviderIDList
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.UserAssignedIdentities != nil {
		in, out := &in.UserAssignedIdentities, &out.UserAssignedIdentities
		*out = make([]apiv1alpha4.UserAssignedIdentity, len(*in))
		copy(*out, *in)
	}
	in.Strategy.DeepCopyInto(&out.Strategy)
	if in.NodeDrainTimeout != nil {
		in, out := &in.NodeDrainTimeout, &out.NodeDrainTimeout
		*out = new(v1.Duration)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureMachinePoolSpec.
func (in *AzureMachinePoolSpec) DeepCopy() *AzureMachinePoolSpec {
	if in == nil {
		return nil
	}
	out := new(AzureMachinePoolSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureMachinePoolStatus) DeepCopyInto(out *AzureMachinePoolStatus) {
	*out = *in
	if in.Instances != nil {
		in, out := &in.Instances, &out.Instances
		*out = make([]*AzureMachinePoolInstanceStatus, len(*in))
		for i := range *in {
			if (*in)[i] != nil {
				in, out := &(*in)[i], &(*out)[i]
				*out = new(AzureMachinePoolInstanceStatus)
				(*in).DeepCopyInto(*out)
			}
		}
	}
	if in.Image != nil {
		in, out := &in.Image, &out.Image
		*out = new(apiv1alpha4.Image)
		(*in).DeepCopyInto(*out)
	}
	if in.ProvisioningState != nil {
		in, out := &in.ProvisioningState, &out.ProvisioningState
		*out = new(apiv1alpha4.ProvisioningState)
		**out = **in
	}
	if in.FailureReason != nil {
		in, out := &in.FailureReason, &out.FailureReason
		*out = new(errors.MachineStatusError)
		**out = **in
	}
	if in.FailureMessage != nil {
		in, out := &in.FailureMessage, &out.FailureMessage
		*out = new(string)
		**out = **in
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make(cluster_apiapiv1alpha4.Conditions, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.LongRunningOperationStates != nil {
		in, out := &in.LongRunningOperationStates, &out.LongRunningOperationStates
		*out = make(apiv1alpha4.Futures, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureMachinePoolStatus.
func (in *AzureMachinePoolStatus) DeepCopy() *AzureMachinePoolStatus {
	if in == nil {
		return nil
	}
	out := new(AzureMachinePoolStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureManagedCluster) DeepCopyInto(out *AzureManagedCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = in.Spec
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureManagedCluster.
func (in *AzureManagedCluster) DeepCopy() *AzureManagedCluster {
	if in == nil {
		return nil
	}
	out := new(AzureManagedCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AzureManagedCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureManagedClusterList) DeepCopyInto(out *AzureManagedClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AzureManagedCluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureManagedClusterList.
func (in *AzureManagedClusterList) DeepCopy() *AzureManagedClusterList {
	if in == nil {
		return nil
	}
	out := new(AzureManagedClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AzureManagedClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureManagedClusterSpec) DeepCopyInto(out *AzureManagedClusterSpec) {
	*out = *in
	out.ControlPlaneEndpoint = in.ControlPlaneEndpoint
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureManagedClusterSpec.
func (in *AzureManagedClusterSpec) DeepCopy() *AzureManagedClusterSpec {
	if in == nil {
		return nil
	}
	out := new(AzureManagedClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureManagedClusterStatus) DeepCopyInto(out *AzureManagedClusterStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureManagedClusterStatus.
func (in *AzureManagedClusterStatus) DeepCopy() *AzureManagedClusterStatus {
	if in == nil {
		return nil
	}
	out := new(AzureManagedClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureManagedControlPlane) DeepCopyInto(out *AzureManagedControlPlane) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureManagedControlPlane.
func (in *AzureManagedControlPlane) DeepCopy() *AzureManagedControlPlane {
	if in == nil {
		return nil
	}
	out := new(AzureManagedControlPlane)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AzureManagedControlPlane) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureManagedControlPlaneList) DeepCopyInto(out *AzureManagedControlPlaneList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AzureManagedControlPlane, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureManagedControlPlaneList.
func (in *AzureManagedControlPlaneList) DeepCopy() *AzureManagedControlPlaneList {
	if in == nil {
		return nil
	}
	out := new(AzureManagedControlPlaneList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AzureManagedControlPlaneList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureManagedControlPlaneSpec) DeepCopyInto(out *AzureManagedControlPlaneSpec) {
	*out = *in
	out.VirtualNetwork = in.VirtualNetwork
	out.ControlPlaneEndpoint = in.ControlPlaneEndpoint
	if in.AdditionalTags != nil {
		in, out := &in.AdditionalTags, &out.AdditionalTags
		*out = make(apiv1alpha4.Tags, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.NetworkPlugin != nil {
		in, out := &in.NetworkPlugin, &out.NetworkPlugin
		*out = new(string)
		**out = **in
	}
	if in.NetworkPolicy != nil {
		in, out := &in.NetworkPolicy, &out.NetworkPolicy
		*out = new(string)
		**out = **in
	}
	if in.SSHPublicKey != nil {
		in, out := &in.SSHPublicKey, &out.SSHPublicKey
		*out = new(string)
		**out = **in
	}
	if in.DNSServiceIP != nil {
		in, out := &in.DNSServiceIP, &out.DNSServiceIP
		*out = new(string)
		**out = **in
	}
	if in.LoadBalancerSKU != nil {
		in, out := &in.LoadBalancerSKU, &out.LoadBalancerSKU
		*out = new(string)
		**out = **in
	}
	if in.IdentityRef != nil {
		in, out := &in.IdentityRef, &out.IdentityRef
		*out = new(corev1.ObjectReference)
		**out = **in
	}
	if in.AADProfile != nil {
		in, out := &in.AADProfile, &out.AADProfile
		*out = new(AADProfile)
		(*in).DeepCopyInto(*out)
	}
	if in.SKU != nil {
		in, out := &in.SKU, &out.SKU
		*out = new(SKU)
		**out = **in
	}
	if in.LoadBalancerProfile != nil {
		in, out := &in.LoadBalancerProfile, &out.LoadBalancerProfile
		*out = new(LoadBalancerProfile)
		(*in).DeepCopyInto(*out)
	}
	if in.APIServerAccessProfile != nil {
		in, out := &in.APIServerAccessProfile, &out.APIServerAccessProfile
		*out = new(APIServerAccessProfile)
		(*in).DeepCopyInto(*out)
	}
	if in.DisableLocalAccounts != nil {
		in, out := &in.DisableLocalAccounts, &out.DisableLocalAccounts
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureManagedControlPlaneSpec.
func (in *AzureManagedControlPlaneSpec) DeepCopy() *AzureManagedControlPlaneSpec {
	if in == nil {
		return nil
	}
	out := new(AzureManagedControlPlaneSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureManagedControlPlaneStatus) DeepCopyInto(out *AzureManagedControlPlaneStatus) {
	*out = *in
	if in.LongRunningOperationStates != nil {
		in, out := &in.LongRunningOperationStates, &out.LongRunningOperationStates
		*out = make(apiv1alpha4.Futures, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureManagedControlPlaneStatus.
func (in *AzureManagedControlPlaneStatus) DeepCopy() *AzureManagedControlPlaneStatus {
	if in == nil {
		return nil
	}
	out := new(AzureManagedControlPlaneStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureManagedMachinePool) DeepCopyInto(out *AzureManagedMachinePool) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureManagedMachinePool.
func (in *AzureManagedMachinePool) DeepCopy() *AzureManagedMachinePool {
	if in == nil {
		return nil
	}
	out := new(AzureManagedMachinePool)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AzureManagedMachinePool) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureManagedMachinePoolList) DeepCopyInto(out *AzureManagedMachinePoolList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AzureManagedMachinePool, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureManagedMachinePoolList.
func (in *AzureManagedMachinePoolList) DeepCopy() *AzureManagedMachinePoolList {
	if in == nil {
		return nil
	}
	out := new(AzureManagedMachinePoolList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AzureManagedMachinePoolList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureManagedMachinePoolSpec) DeepCopyInto(out *AzureManagedMachinePoolSpec) {
	*out = *in
	if in.Name != nil {
		in, out := &in.Name, &out.Name
		*out = new(string)
		**out = **in
	}
	if in.OSDiskSizeGB != nil {
		in, out := &in.OSDiskSizeGB, &out.OSDiskSizeGB
		*out = new(int32)
		**out = **in
	}
	if in.ProviderIDList != nil {
		in, out := &in.ProviderIDList, &out.ProviderIDList
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureManagedMachinePoolSpec.
func (in *AzureManagedMachinePoolSpec) DeepCopy() *AzureManagedMachinePoolSpec {
	if in == nil {
		return nil
	}
	out := new(AzureManagedMachinePoolSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AzureManagedMachinePoolStatus) DeepCopyInto(out *AzureManagedMachinePoolStatus) {
	*out = *in
	if in.ErrorReason != nil {
		in, out := &in.ErrorReason, &out.ErrorReason
		*out = new(errors.MachineStatusError)
		**out = **in
	}
	if in.ErrorMessage != nil {
		in, out := &in.ErrorMessage, &out.ErrorMessage
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AzureManagedMachinePoolStatus.
func (in *AzureManagedMachinePoolStatus) DeepCopy() *AzureManagedMachinePoolStatus {
	if in == nil {
		return nil
	}
	out := new(AzureManagedMachinePoolStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *LoadBalancerProfile) DeepCopyInto(out *LoadBalancerProfile) {
	*out = *in
	if in.ManagedOutboundIPs != nil {
		in, out := &in.ManagedOutboundIPs, &out.ManagedOutboundIPs
		*out = new(int32)
		**out = **in
	}
	if in.OutboundIPPrefixes != nil {
		in, out := &in.OutboundIPPrefixes, &out.OutboundIPPrefixes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.OutboundIPs != nil {
		in, out := &in.OutboundIPs, &out.OutboundIPs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.AllocatedOutboundPorts != nil {
		in, out := &in.AllocatedOutboundPorts, &out.AllocatedOutboundPorts
		*out = new(int32)
		**out = **in
	}
	if in.IdleTimeoutInMinutes != nil {
		in, out := &in.IdleTimeoutInMinutes, &out.IdleTimeoutInMinutes
		*out = new(int32)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new LoadBalancerProfile.
func (in *LoadBalancerProfile) DeepCopy() *LoadBalancerProfile {
	if in == nil {
		return nil
	}
	out := new(LoadBalancerProfile)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MachineRollingUpdateDeployment) DeepCopyInto(out *MachineRollingUpdateDeployment) {
	*out = *in
	if in.MaxUnavailable != nil {
		in, out := &in.MaxUnavailable, &out.MaxUnavailable
		*out = new(intstr.IntOrString)
		**out = **in
	}
	if in.MaxSurge != nil {
		in, out := &in.MaxSurge, &out.MaxSurge
		*out = new(intstr.IntOrString)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MachineRollingUpdateDeployment.
func (in *MachineRollingUpdateDeployment) DeepCopy() *MachineRollingUpdateDeployment {
	if in == nil {
		return nil
	}
	out := new(MachineRollingUpdateDeployment)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ManagedControlPlaneSubnet) DeepCopyInto(out *ManagedControlPlaneSubnet) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ManagedControlPlaneSubnet.
func (in *ManagedControlPlaneSubnet) DeepCopy() *ManagedControlPlaneSubnet {
	if in == nil {
		return nil
	}
	out := new(ManagedControlPlaneSubnet)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ManagedControlPlaneVirtualNetwork) DeepCopyInto(out *ManagedControlPlaneVirtualNetwork) {
	*out = *in
	out.Subnet = in.Subnet
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ManagedControlPlaneVirtualNetwork.
func (in *ManagedControlPlaneVirtualNetwork) DeepCopy() *ManagedControlPlaneVirtualNetwork {
	if in == nil {
		return nil
	}
	out := new(ManagedControlPlaneVirtualNetwork)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SKU) DeepCopyInto(out *SKU) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SKU.
func (in *SKU) DeepCopy() *SKU {
	if in == nil {
		return nil
	}
	out := new(SKU)
	in.DeepCopyInto(out)
	return out
}

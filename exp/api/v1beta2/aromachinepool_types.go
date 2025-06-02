/*
Copyright 2025 The Kubernetes Authors.

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

package v1beta2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

// AROMachinePoolSpec defines the desired state of AROMachinePool.
type AROMachinePoolSpec struct {
	// NodePoolName specifies the name of the nodepool in ARO
	// must be a valid DNS-1035 label, so it must consist of lower case alphanumeric and have a max length of 15 characters.
	//
	// +immutable
	// +kubebuilder:validation:XValidation:rule="self == oldSelf", message="nodepoolName is immutable"
	// +kubebuilder:validation:MaxLength:=15
	// +kubebuilder:validation:Pattern:=`^[a-z]([-a-z0-9]*[a-z0-9])?$`
	NodePoolName string `json:"nodePoolName"`

	// Version specifies the OpenShift version of the nodes associated with this machinepool.
	// AROControlPlane version is used if not set.
	//
	// +optional
	Version string `json:"version,omitempty"`

	// AROPlatformProfileMachinePool represents the NodePool Azure platform configuration.
	Platform AROPlatformProfileMachinePool `json:"platform,omitempty"`

	// Labels specifies labels for the Kubernetes node objects
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Taints specifies the taints to apply to the nodes of the machine pool
	// +optional
	Taints []AROTaint `json:"taints,omitempty"`

	// AdditionalTags are user-defined tags to be added on the underlying EC2 instances associated with this machine pool.
	// +immutable
	// +optional
	AdditionalTags infrav1.Tags `json:"additionalTags,omitempty"`

	// AutoRepair specifies whether health checks should be enabled for machines
	// in the NodePool. The default is true.
	// +kubebuilder:default=true
	// +optional
	AutoRepair bool `json:"autoRepair,omitempty"`

	// Autoscaling specifies auto scaling behaviour for this MachinePool.
	// required if Replicas is not configured
	// +optional
	Autoscaling *AROMachinePoolAutoScaling `json:"autoscaling,omitempty"`
}

// AROPlatformProfileMachinePool represents the Azure platform configuration.
type AROPlatformProfileMachinePool struct {
	// Azure subnet id
	Subnet string `json:"subnet,omitempty"`

	// Subnet Ref name that is used to create the VirtualNetworksSubnet CR. The SubnetRef must be in the same namespace as the AroMachinePool and cannot be set with Subnet.
	SubnetRef string `json:"subnetRef,omitempty"`

	// VMSize sets the VM disk volume size to the node.
	VMSize string `json:"vmSize,omitempty"`

	// DiskSizeGiB sets the disk volume size for the machine pool, in Gib.
	DiskSizeGiB int32 `json:"diskSizeGiB,omitempty"`

	// DiskStorageAccountType represents supported Azure storage account types.
	// Available values are Premium_LRS, StandardSSD_LRS and Standard_LRS.
	DiskStorageAccountType string `json:"diskStorageAccountType,omitempty"`

	// AvailabilityZone specifying the availability zone where instances of this machine pool should run.
	AvailabilityZone string `json:"availabilityZone,omitempty"`
}

// AROTaint represents a taint to be applied to a node.
type AROTaint struct {
	// The taint key to be applied to a node.
	//
	// +kubebuilder:validation:Required
	Key string `json:"key"`
	// The taint value corresponding to the taint key.
	//
	// +kubebuilder:validation:Pattern:=`^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$`
	// +optional
	Value string `json:"value,omitempty"`
	// The effect of the taint on pods that do not tolerate the taint.
	// Valid effects are NoSchedule, PreferNoSchedule and NoExecute.
	//
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=NoSchedule;PreferNoSchedule;NoExecute
	Effect corev1.TaintEffect `json:"effect"`
}

// AROMachinePoolAutoScaling specifies scaling options.
type AROMachinePoolAutoScaling struct {
	// +kubebuilder:validation:Minimum=1
	MinReplicas int `json:"minReplicas,omitempty"`
	// +kubebuilder:validation:Minimum=1
	MaxReplicas int `json:"maxReplicas,omitempty"`
}

// AROMachinePoolStatus defines the observed state of AROMachinePool.
type AROMachinePoolStatus struct {
	// Ready denotes that the AROMachinePool nodepool has joined
	// the cluster
	// +kubebuilder:default=false
	Ready bool `json:"ready"`
	// Replicas is the most recently observed number of replicas.
	// +optional
	Replicas int32 `json:"replicas"`
	// Conditions defines current service state of the managed machine pool
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the state and will be set to a descriptive error message.
	//
	// This field should not be set for transitive errors that a controller
	// faces that are expected to be fixed automatically over
	// time (like service outages), but instead indicate that something is
	// fundamentally wrong with the spec or the configuration of
	// the controller, and that manual intervention is required.
	//
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// ID is the ID given by ARO.
	ID string `json:"id,omitempty"`

	// Available upgrades for the ARO MachinePool.
	AvailableUpgrades []string `json:"availableUpgrades,omitempty"`

	// ProvisioningState represents the asynchronous provisioning state of an ARM resource.
	// Allowed values are; Succeeded, Failed, Canceled, Accepted, Deleting, Provisioning and Updating.
	ProvisioningState string `json:"provisioningState,omitempty"`

	// ARO-HCP OpenShift semantic version, for example "4.20.0".
	Version string `json:"version"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=aromachinepools,scope=Namespaced,categories=cluster-api,shortName=aromp
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="MachinePool ready status"
// +kubebuilder:printcolumn:name="Replicas",type="integer",JSONPath=".status.replicas",description="Number of replicas"
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=aromachinepools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=aromachinepools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=aromachinepools/finalizers,verbs=update

// AROMachinePool is the Schema for the aromachinepools API.
type AROMachinePool struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AROMachinePoolSpec   `json:"spec,omitempty"`
	Status AROMachinePoolStatus `json:"status,omitempty"`
}

const (
	// AROMachinePoolKind is the kind for AROMachinePool.
	AROMachinePoolKind = "AROMachinePool"

	// AROMachinePoolFinalizer is the finalizer added to AROControlPlanes.
	AROMachinePoolFinalizer = "aromachinepool/finalizer"
)

// +kubebuilder:object:root=true

// AROMachinePoolList contains a list of AROMachinePools.
type AROMachinePoolList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AROMachinePool `json:"items"`
}

// GetConditions returns the observations of the operational state of the AROMachinePool resource.
func (r *AROMachinePool) GetConditions() clusterv1.Conditions {
	return r.Status.Conditions
}

// SetConditions sets the underlying service state of the AROMachinePool to the predescribed clusterv1.Conditions.
func (r *AROMachinePool) SetConditions(conditions clusterv1.Conditions) {
	r.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&AROMachinePool{}, &AROMachinePoolList{})
}

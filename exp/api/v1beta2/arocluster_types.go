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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// AROClusterSpec defines the desired state of AROCluster.
type AROClusterSpec struct {
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`
}

// AROClusterStatus defines the observed state of AROCluster.
type AROClusterStatus struct {
	// Ready is when the AROControlPlane has a API server URL.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// FailureDomains specifies a list fo available availability zones that can be used
	// +optional
	FailureDomains clusterv1.FailureDomains `json:"failureDomains,omitempty"`

	// Conditions define the current service state of the AROCluster.
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// initialization provides observations of the AROCluster initialization process.
	// NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Machine provisioning.
	// +optional
	Initialization *AROClusterInitializationStatus `json:"initialization,omitempty"`
}

// AROClusterInitializationStatus provides observations of the AROCluster initialization process.
type AROClusterInitializationStatus struct {
	// provision is true when the AROCluster provider reports that the infra cluster is provisioned;
	// A infra cluster is considered provisioned when it has valid endpoint.
	// NOTE: this field is part of the Cluster API contract, and it is used to orchestrate initial Machine provisioning.
	// +optional
	Provisioned bool `json:"provisioned,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=aroclusters,scope=Namespaced,categories=cluster-api,shortName=aroc
// +kubebuilder:storageversion
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".metadata.labels.cluster\\.x-k8s\\.io/cluster-name",description="Cluster to which this AroManagedControl belongs"
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.ready",description="Control plane infrastructure is ready for worker nodes"
// +kubebuilder:printcolumn:name="Provisioned",type="boolean",JSONPath=".status.initialization.provisioned",description="Control plane infrastructure is provisioned"
// +kubebuilder:printcolumn:name="Endpoint",type="string",JSONPath=".spec.controlPlaneEndpoint.host",description="API Endpoint",priority=1
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=aroclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=aroclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=aroclusters/finalizers,verbs=update

// AROCluster is the Schema for the AROClusters API.
type AROCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AROClusterSpec   `json:"spec,omitempty"`
	Status AROClusterStatus `json:"status,omitempty"`
}

// GetConditions returns the conditions for the AROCluster.
func (ac *AROCluster) GetConditions() clusterv1.Conditions {
	return ac.Status.Conditions
}

// SetConditions sets the conditions for the AROCluster.
func (ac *AROCluster) SetConditions(conditions clusterv1.Conditions) {
	ac.Status.Conditions = conditions
}

const (
	// AROClusterKind is the kind for AROCluster.
	AROClusterKind = "AROCluster"

	// AROClusterFinalizer is the finalizer added to AROControlPlanes.
	AROClusterFinalizer = "arocluster/finalizer"
)

// +kubebuilder:object:root=true

// AROClusterList contains a list of AROCluster.
type AROClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AROCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AROCluster{}, &AROClusterList{})
}

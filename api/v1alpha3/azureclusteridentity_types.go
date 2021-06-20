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

package v1alpha3

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

// AzureClusterIdentitySpec defines the parameters that are used to create an AzureIdentity.
type AzureClusterIdentitySpec struct {
	// UserAssignedMSI or Service Principal
	Type IdentityType `json:"type"`
	// User assigned MSI resource id.
	// +optional
	ResourceID string `json:"resourceID,omitempty"`
	// Both User Assigned MSI and SP can use this field.
	ClientID string `json:"clientID"`
	// ClientSecret is a secret reference which should contain either a Service Principal password or certificate secret.
	// +optional
	ClientSecret corev1.SecretReference `json:"clientSecret,omitempty"`
	// Service principal primary tenant id.
	TenantID string `json:"tenantID"`
	// AllowedNamespaces is an array of namespaces that AzureClusters can
	// use this Identity from.
	//
	// An empty list (default) indicates that AzureClusters can use this
	// Identity from any namespace. This field is intentionally not a
	// pointer because the nil behavior (no namespaces) is undesirable here.
	// +optional
	AllowedNamespaces []string `json:"allowedNamespaces"`
}

// AzureClusterIdentityStatus defines the observed state of AzureClusterIdentity.
type AzureClusterIdentityStatus struct {
	// Conditions defines current service state of the AzureClusterIdentity.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=azureclusteridentities,scope=Namespaced,categories=cluster-api
// +kubebuilder:subresource:status

// AzureClusterIdentity is the Schema for the azureclustersidentities API.
type AzureClusterIdentity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AzureClusterIdentitySpec   `json:"spec,omitempty"`
	Status AzureClusterIdentityStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AzureClusterIdentityList contains a list of AzureClusterIdentities.
type AzureClusterIdentityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AzureClusterIdentity `json:"items"`
}

// GetConditions returns the list of conditions for an AzureClusterIdentity API object.
func (c *AzureClusterIdentity) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions will set the given conditions on an AzureClusterIdentity object.
func (c *AzureClusterIdentity) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

// ClusterNamespaceAllowed indicates if the cluster namespace is allowed.
func (c *AzureClusterIdentity) ClusterNamespaceAllowed(namespace string) bool {
	if len(c.Spec.AllowedNamespaces) == 0 {
		return true
	}
	for _, v := range c.Spec.AllowedNamespaces {
		if v == namespace {
			return true
		}
	}

	return false
}

func init() {
	SchemeBuilder.Register(&AzureClusterIdentity{}, &AzureClusterIdentityList{})
}

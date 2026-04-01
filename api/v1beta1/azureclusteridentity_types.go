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

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
)

// AllowedNamespaces defines the namespaces the clusters are allowed to use the identity from
// NamespaceList takes precedence over the Selector.
type AllowedNamespaces struct {
	// An empty list indicates that AzureCluster cannot use the identity from any namespace.
	//
	// +optional
	NamespaceList []string `json:"list"`
	// Selector is a selector of namespaces that AzureCluster can
	// use this Identity from. This is a standard Kubernetes LabelSelector,
	// a label query over a set of resources. The result of matchLabels and
	// matchExpressions are ANDed.
	//
	// A nil or empty selector indicates that AzureCluster cannot use this
	// AzureClusterIdentity from any namespace.
	// +optional
	Selector *metav1.LabelSelector `json:"selector"`
}

// AzureClusterIdentitySpec defines the parameters that are used to create an AzureIdentity.
type AzureClusterIdentitySpec struct {
	// Type is the type of Azure Identity used.
	// ServicePrincipal, ServicePrincipalCertificate, UserAssignedMSI, ManualServicePrincipal, UserAssignedIdentityCredential, or WorkloadIdentity.
	Type IdentityType `json:"type"`
	// ResourceID is the Azure resource ID for the User Assigned MSI resource.
	// Only applicable when type is UserAssignedMSI.
	//
	// Deprecated: This field no longer has any effect.
	//
	// +optional
	ResourceID string `json:"resourceID,omitempty"`
	// ClientID is the service principal client ID.
	// Both User Assigned MSI and SP can use this field.
	ClientID string `json:"clientID"`
	// ClientSecret is a secret reference which should contain either a Service Principal password or certificate secret.
	// +optional
	ClientSecret corev1.SecretReference `json:"clientSecret,omitempty"`
	// CertPath is the path where certificates exist. When set, it takes precedence over ClientSecret for types that use certs like ServicePrincipalCertificate.
	// +optional
	CertPath string `json:"certPath,omitempty"`
	// UserAssignedIdentityCredentialsPath is the path where an existing JSON file exists containing the JSON format of
	// a UserAssignedIdentityCredentials struct.
	// See the msi-dataplane for more details on UserAssignedIdentityCredentials - https://github.com/Azure/msi-dataplane/blob/main/pkg/dataplane/internal/client/models.go#L125
	// +optional
	UserAssignedIdentityCredentialsPath string `json:"userAssignedIdentityCredentialsPath,omitempty"`
	// UserAssignedIdentityCredentialsCloudType is used with UserAssignedIdentityCredentialsPath to specify the Cloud
	// type. Can only be one of the following values: public, china, or usgovernment
	// If a value is not specified, defaults to public
	// +optional
	UserAssignedIdentityCredentialsCloudType string `json:"userAssignedIdentityCredentialsCloudType,omitempty"`
	// TenantID is the service principal primary tenant id.
	TenantID string `json:"tenantID"`
	// AllowedNamespaces is used to identify the namespaces the clusters are allowed to use the identity from.
	// Namespaces can be selected either using an array of namespaces or with label selector.
	// An empty allowedNamespaces object indicates that AzureClusters can use this identity from any namespace.
	// If this object is nil, no namespaces will be allowed (default behaviour, if this field is not provided)
	// A namespace should be either in the NamespaceList or match with Selector to use the identity.
	//
	// +optional
	AllowedNamespaces *AllowedNamespaces `json:"allowedNamespaces"`
}

// AzureClusterIdentityStatus defines the observed state of AzureClusterIdentity.
type AzureClusterIdentityStatus struct {
	// Conditions defines current service state of the AzureClusterIdentity.
	// +optional
	Conditions clusterv1beta1.Conditions `json:"conditions,omitempty"`

	// v1beta2 groups all the fields that will be added or modified in AzureClusterIdentity's status with the v1beta2 version of the Cluster API contract.
	// +optional
	V1Beta2 *AzureClusterIdentityV1Beta2Status `json:"v1beta2,omitempty"`
}

// AzureClusterIdentityV1Beta2Status groups all the fields that will be added or modified in AzureClusterIdentity with the v1beta2 version of the Cluster API contract.
type AzureClusterIdentityV1Beta2Status struct {
	// conditions represents the observations of an AzureClusterIdentity's current state.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=32
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type",description="Type of AzureClusterIdentity"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of this AzureClusterIdentity"
// +kubebuilder:resource:path=azureclusteridentities,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion
// +kubebuilder:subresource:status

// AzureClusterIdentity is the Schema for the azureclustersidentities API.
type AzureClusterIdentity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AzureClusterIdentitySpec   `json:"spec,omitempty"`
	Status AzureClusterIdentityStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AzureClusterIdentityList contains a list of AzureClusterIdentity.
type AzureClusterIdentityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AzureClusterIdentity `json:"items"`
}

// GetConditions returns the list of conditions for an AzureClusterIdentity API object.
func (c *AzureClusterIdentity) GetConditions() clusterv1beta1.Conditions {
	return c.Status.Conditions
}

// SetConditions will set the given conditions on an AzureClusterIdentity object.
func (c *AzureClusterIdentity) SetConditions(conditions clusterv1beta1.Conditions) {
	c.Status.Conditions = conditions
}

// GetV1Beta2Conditions returns the v1beta2 conditions for an AzureClusterIdentity API object.
// Note: GetV1Beta2Conditions will be renamed to GetConditions in a later stage of the transition to the v1beta2 Cluster API contract.
func (c *AzureClusterIdentity) GetV1Beta2Conditions() []metav1.Condition {
	if c.Status.V1Beta2 == nil {
		return nil
	}
	return c.Status.V1Beta2.Conditions
}

// SetV1Beta2Conditions sets the v1beta2 conditions on an AzureClusterIdentity object.
// Note: SetV1Beta2Conditions will be renamed to SetConditions in a later stage of the transition to the v1beta2 Cluster API contract.
func (c *AzureClusterIdentity) SetV1Beta2Conditions(conditions []metav1.Condition) {
	if c.Status.V1Beta2 == nil {
		c.Status.V1Beta2 = &AzureClusterIdentityV1Beta2Status{}
	}
	c.Status.V1Beta2.Conditions = conditions
}

func init() {
	objectTypes = append(objectTypes, &AzureClusterIdentity{}, &AzureClusterIdentityList{})
}

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// AzureMarketplaceImageKind is the kind name for an Azure Marketplace Image
	AzureMarketplaceImageKind = "AzureMarketplaceImage"

	// AzureSharedGalleryImageKind is the kind name for an Azure shared image gallery image
	AzureSharedGalleryImageKind = "AzureSharedGalleryImage"

	// AzureImageByIDKind is the kind name for an image with a specific id
	AzureImageByIDKind = "AzureImageByID"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=azureimages,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion

// AzureMarketplaceImage defines an image in the Azure marketplace to use for VM creation
type AzureMarketplaceImage struct {
	metav1.TypeMeta `json:",inline"`

	// Publisher is the name of the organization that created the image
	// +kubebuilder:validation:MinLength=1
	Publisher string `json:"publisher"`
	// Offer specifies the name of a group of related images created by the publisher.
	// For example, UbuntuServer, WindowsServer
	// +kubebuilder:validation:MinLength=1
	Offer string `json:"offer"`
	// SKU specifies an instance of an offer, such as a major release of a distribution.
	// For example, 18.04-LTS, 2019-Datacenter
	// +kubebuilder:validation:MinLength=1
	SKU string `json:"sku"`
	// Version specifies the version of an image sku. The allowed formats
	// are Major.Minor.Build or 'latest'. Major, Minor, and Build are decimal numbers.
	// Specify 'latest' to use the latest version of an image available at deploy time.
	// Even if you use 'latest', the VM image will not automatically update after deploy
	// time even if a new version becomes available.
	// +kubebuilder:validation:MinLength=1
	Version string `json:"version"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=azureimages,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion

// AzureSharedGalleryImage defines an image in a Shared Image Gallery to use for VM creation
type AzureSharedGalleryImage struct {
	metav1.TypeMeta `json:",inline"`

	// SubscriptionID is the identifier of the subscription that contains the shared image gallery
	// +kubebuilder:validation:MinLength=1
	SubscriptionID string `json:"subscriptionID"`
	// ResourceGroup specifies the resource group containing the shared image gallery
	// +kubebuilder:validation:MinLength=1
	ResourceGroup string `json:"resourceGroup"`
	// Gallery specifies the name of the shared image gallery that contains the image
	// +kubebuilder:validation:MinLength=1
	Gallery string `json:"gallery"`
	// Name is the name of the image
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Version specifies the version of the marketplace image. The allowed formats
	// are Major.Minor.Build or 'latest'. Major, Minor, and Build are decimal numbers.
	// Specify 'latest' to use the latest version of an image available at deploy time.
	// Even if you use 'latest', the VM image will not automatically update after deploy
	// time even if a new version becomes available.
	// +kubebuilder:validation:MinLength=1
	Version string `json:"version"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=azureimages,scope=Namespaced,categories=cluster-api
// +kubebuilder:storageversion

// AzureImageByID defines a specific image via by resource id to use for VM creation
type AzureImageByID struct {
	metav1.TypeMeta `json:",inline"`

	// ID specifies the resource identifier of the image
	// +kubebuilder:validation:MinLength=1
	ID string `json:"id"`
}

func init() {
	SchemeBuilder.Register(
		&AzureMarketplaceImage{},
		&AzureSharedGalleryImage{},
		&AzureImageByID{})
}

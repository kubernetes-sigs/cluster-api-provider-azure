/*
Copyright 2019 The Kubernetes Authors.

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

package virtualmachines

import (
	"context"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
)

// VMScope defines the scope interface for a virtual machines service.
type VMScope interface {
	logr.Logger
	azure.ClusterDescriber
	VMSpecs() []azure.VMSpec
	GetBootstrapData(ctx context.Context) (string, error)
	GetVMImage() (*infrav1.Image, error)
	SetAnnotation(string, string)
	SetProviderID(string)
	SetAddresses([]corev1.NodeAddress)
	SetVMState(infrav1.VMState)
}

// Service provides operations on azure resources
type Service struct {
	Scope VMScope
	Client
	InterfacesClient networkinterfaces.Client
	PublicIPsClient  publicips.Client
	ResourceSKUCache *resourceskus.Cache
}

// NewService creates a new service.
func NewService(scope VMScope, skuCache *resourceskus.Cache) *Service {
	return &Service{
		Scope:            scope,
		Client:           NewClient(scope),
		InterfacesClient: networkinterfaces.NewClient(scope),
		PublicIPsClient:  publicips.NewClient(scope),
		ResourceSKUCache: skuCache,
	}
}

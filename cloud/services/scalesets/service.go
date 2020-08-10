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
package scalesets

import (
	"context"
	"github.com/go-logr/logr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
)

// ScaleSetScope defines the scope interface for a scale sets service.
type ScaleSetScope interface {
	azure.ClusterDescriber
	logr.Logger
	ScaleSetSpec() azure.ScaleSetSpec
	GetBootstrapData(ctx context.Context) (string, error)
	GetVMImage() (*infrav1.Image, error)
	SetAnnotation(string, string)
	SetProviderID(string)
	SetProvisioningState(infrav1.VMState)
}

// Service provides operations on azure resources
type Service struct {
	Scope ScaleSetScope
	Client
	ResourceSKUCache *resourceskus.Cache
}

// NewService creates a new service.
func NewService(scope ScaleSetScope, skuCache *resourceskus.Cache) *Service {
	return &Service{
		Client:           NewClient(scope),
		Scope:            scope,
		ResourceSKUCache: skuCache,
	}
}

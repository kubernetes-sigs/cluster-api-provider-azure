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

package vaults

import (
	"context"

	"github.com/Azure/azure-service-operator/v2/api/keyvault/v1api20230701"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/conditionalaso"
	"sigs.k8s.io/cluster-api-provider-azure/util/slice"
)

const (
	serviceName = "vaults"
	// VaultReadyCondition defines the condition type for vault readiness.
	VaultReadyCondition clusterv1.ConditionType = "VaultReady"
)

// KeyVaultScope defines the scope interface for a key vault service.
type KeyVaultScope interface {
	conditionalaso.Scope
	VaultSpecs() []azure.ASOResourceSpecGetter[*v1api20230701.Vault]
}

// Service provides operations on Azure Key Vault resources.
type Service struct {
	Scope KeyVaultScope
}

// New creates a new service.
func New(scope KeyVaultScope) *conditionalaso.Service[*v1api20230701.Vault, KeyVaultScope] {
	svc := conditionalaso.NewService[*v1api20230701.Vault, KeyVaultScope](serviceName, scope)
	svc.ListFunc = list
	svc.Specs = scope.VaultSpecs()
	svc.ConditionType = VaultReadyCondition
	return svc
}

func list(ctx context.Context, client client.Client, opts ...client.ListOption) ([]*v1api20230701.Vault, error) {
	list := &v1api20230701.VaultList{}
	err := client.List(ctx, list, opts...)
	return slice.ToPtrs(list.Items), err
}

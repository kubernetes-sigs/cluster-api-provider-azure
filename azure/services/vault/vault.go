/*
Copyright 2022 The Kubernetes Authors.

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

package vault

import (
	"context"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "vault"

// Scope defines the scope interface for vault service.
type Scope interface {
	azure.Authorizer
	async.FutureScope
	VaultSpec() azure.ResourceSpecGetter
}

// Service provides operations on Azure resources.
type Service struct {
	Scope Scope
	async.Reconciler
}

// Name returns the service name.
func (s *Service) Name() string {
	return serviceName
}

// IsManaged returns true if the vault lifecycles is managed.
func (s *Service) IsManaged(ctx context.Context) (bool, error) {
	return true, nil
}

// New creates a new service.
func New(scope Scope) *Service {
	client := newClient(scope)
	return &Service{
		Scope:      scope,
		Reconciler: async.New(scope, client, client),
	}
}

// Reconcile gets/creates/updates vault.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "vault.Service.Reconcile")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	spec := s.Scope.VaultSpec()
	if spec == nil {
		return nil
	}

	_, err := s.CreateResource(ctx, spec, serviceName)
	s.Scope.UpdatePutStatus(infrav1.VaultReadyCondition, serviceName, err)

	return err
}

// Delete deletes the vault.
func (s *Service) Delete(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "vault.Service.Delete")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	spec := s.Scope.VaultSpec()
	if spec == nil {
		return nil
	}

	err := s.DeleteResource(ctx, spec, serviceName)
	if err != nil && azure.ResourceNotFound(err) {
		s.Scope.UpdateDeleteStatus(infrav1.VaultReadyCondition, serviceName, nil)
		return nil
	}

	s.Scope.UpdateDeleteStatus(infrav1.VaultReadyCondition, serviceName, err)

	return err
}

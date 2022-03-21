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

package secrets

import (
	"context"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "secrets"

// SecretScope defines the scope interface for secrets service.
type SecretScope interface {
	azure.Authorizer
	async.FutureScope
	SecretSpecs() []azure.ResourceSpecGetter
	VMState() infrav1.ProvisioningState
}

// Service provides operations on Azure resources.
type Service struct {
	Scope SecretScope
	async.Reconciler
}

// Name returns the service name.
func (s *Service) Name() string {
	return serviceName
}

// IsManaged returns true if the secrets' lifecycles are managed.
func (s *Service) IsManaged(ctx context.Context) (bool, error) {
	return true, nil
}

// New creates a new service.
func New(scope SecretScope) *Service {
	client := newClient(scope)
	return &Service{
		Scope:      scope,
		Reconciler: async.New(scope, client, client),
	}
}

// Reconcile gets/creates/updates secrets.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "secrets.Service.Reconcile")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	specs := s.Scope.SecretSpecs()
	if len(specs) == 0 {
		log.Info("secrets already created, skipping creation")
		return nil
	}

	if s.Scope.VMState() != "" {
		log.Info("VM provisioning has begun, skipping secret reconciliation")
		return nil
	}

	// We go through the list of SecretSpecs to create each one, independently of the result of the previous one.
	// If multiple errors occur, we return the most pressing one
	// order of precedence is: error deleting -> deleting in progress -> deleted (no error)
	var result error
	for _, spec := range specs {
		if _, err := s.CreateResource(ctx, spec, serviceName); err != nil {
			if !azure.IsOperationNotDoneError(err) || result == nil {
				result = err
			}
		}
	}
	s.Scope.UpdatePutStatus(infrav1.SecretReadyCondition, serviceName, result)

	return result
}

// Delete deletes azure resources for this service.
// no-op for secrets.
func (s *Service) Delete(ctx context.Context) error {
	return nil
}

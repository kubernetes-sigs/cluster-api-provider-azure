/*
Copyright 2023 The Kubernetes Authors.

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

package privatelinks

import (
	"context"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// ServiceName is the name of this service.
const ServiceName = "privatelinks"

// PrivateLinkScope defines the scope interface for a private link.
type PrivateLinkScope interface {
	azure.ClusterScoper
	azure.AsyncStatusUpdater
	PrivateLinkSpecs() []azure.ResourceSpecGetter
}

// Service provides operations on Azure resources.
type Service struct {
	Scope PrivateLinkScope
	async.Reconciler
}

// New creates a new service.
func New(scope PrivateLinkScope) *Service {
	client := newClient(scope)
	return &Service{
		Scope:      scope,
		Reconciler: async.New(scope, client, client),
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return ServiceName
}

// Reconcile idempotently creates or updates a private link.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "privatelinks.Service.Reconcile")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	specs := s.Scope.PrivateLinkSpecs()
	if len(specs) == 0 {
		return nil
	}

	var resultingErr error
	for _, privateLinkSpec := range specs {
		_, err := s.CreateOrUpdateResource(ctx, privateLinkSpec, ServiceName)
		if err != nil {
			if !azure.IsOperationNotDoneError(err) || resultingErr == nil {
				resultingErr = err
			}
		}
	}

	s.Scope.UpdatePutStatus(infrav1.PrivateLinksReadyCondition, ServiceName, resultingErr)
	return resultingErr
}

// Delete reconciles the private link deletion.
func (s *Service) Delete(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "privatelinks.Service.Delete")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	specs := s.Scope.PrivateLinkSpecs()
	if len(specs) == 0 {
		return nil
	}

	// We go through the list of PrivateLinkSpecs to delete each one, independently of the resultingErr of the previous one.
	// If multiple errors occur, we return the most pressing one.
	//  Order of precedence (highest -> lowest) is: error that is not an operationNotDoneError (i.e. error creating) -> operationNotDoneError (i.e. creating in progress) -> no error (i.e. created)
	var resultingErr error
	for _, privateLinkSpec := range specs {
		if err := s.DeleteResource(ctx, privateLinkSpec, ServiceName); err != nil {
			if !azure.IsOperationNotDoneError(err) || resultingErr == nil {
				resultingErr = err
			}
		}
	}
	s.Scope.UpdateDeleteStatus(infrav1.PrivateLinksReadyCondition, ServiceName, resultingErr)
	return resultingErr
}

// IsManaged returns always returns true as CAPZ does not support BYO private links.
func (s *Service) IsManaged(ctx context.Context) (bool, error) {
	return true, nil
}

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

package securitygroups

import (
	"context"

	"github.com/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "securitygroups"

// NSGScope defines the scope interface for a security groups service.
type NSGScope interface {
	azure.Authorizer
	azure.AsyncStatusUpdater
	NSGSpecs() []azure.ResourceSpecGetter
	IsVnetManaged(context.Context) (bool, error)
}

// Service provides operations on Azure resources.
type Service struct {
	Scope NSGScope
	client
}

// New creates a new service.
func New(scope NSGScope) *Service {
	return &Service{
		Scope:  scope,
		client: newClient(scope),
	}
}

// Reconcile gets/creates/updates network security groups.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "securitygroups.Service.Reconcile")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	// Only create the NSGs if their lifecycle is managed by this controller.
	// NSGs are managed if and only if the vnet is managed.
	managed, err := s.Scope.IsVnetManaged(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to determine if network security groups are managed")
	} else if !managed {
		log.Info("Skipping network security groups reconcile in custom VNet mode")
		return nil
	}

	// We go through the list of NSGSpecs to reconcile each one, independently of the result of the previous one.
	// If multiple erros occur, we return the most pressing one
	// order of precedence is: error creating -> creating in progress -> created (no error)
	var resErr error

	for _, nsgSpec := range s.Scope.NSGSpecs() {
		if _, err := async.CreateResource(ctx, s.Scope, s.client, nsgSpec, serviceName); err != nil {
			if !azure.IsOperationNotDoneError(err) || resErr == nil {
				resErr = err
			}
		}
	}

	s.Scope.UpdatePutStatus(infrav1.SecurityGroupsReadyCondition, serviceName, resErr)
	return resErr
}

// Delete deletes network security groups.
func (s *Service) Delete(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "securitygroups.Service.Delete")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	// Only delete the NSG if its lifecycle is managed by this controller.
	// NSGs are managed if and only if the vnet is managed.
	managed, err := s.Scope.IsVnetManaged(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to determine if network security groups are managed")
	} else if !managed {
		log.V(4).Info("Skipping network security groups delete in custom VNet mode")
		return nil
	}

	var result error

	// We go through the list of NSGSpecs to delete each one, independently of the result of the previous one.
	// If multiple erros occur, we return the most pressing one
	// order of precedence is: error deleting -> deleting in progress -> deleted (no error)
	for _, nsgSpec := range s.Scope.NSGSpecs() {
		if err := async.DeleteResource(ctx, s.Scope, s.client, nsgSpec, serviceName); err != nil {
			if !azure.IsOperationNotDoneError(err) || result == nil {
				result = err
			}
		}
	}

	s.Scope.UpdateDeleteStatus(infrav1.SecurityGroupsReadyCondition, serviceName, result)
	return result
}

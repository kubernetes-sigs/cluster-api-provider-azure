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
	IsVnetManaged() bool
}

// Service provides operations on Azure resources.
type Service struct {
	Scope NSGScope
	async.Reconciler
}

// New creates a new service.
func New(scope NSGScope) *Service {
	client := newClient(scope)
	return &Service{
		Scope:      scope,
		Reconciler: async.New(scope, client, client),
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return serviceName
}

// Reconcile gets/creates/updates network security groups.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "securitygroups.Service.Reconcile")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	// Only create the NSGs if their lifecycle is managed by this controller.
	if managed, err := s.IsManaged(ctx); err == nil && !managed {
		log.V(4).Info("Skipping network security groups reconcile in custom VNet mode")
		return nil
	} else if err != nil {
		return errors.Wrap(err, "failed to check if security groups are managed")
	}

	specs := s.Scope.NSGSpecs()
	if len(specs) == 0 {
		return nil
	}

	var resErr error

	// We go through the list of security groups to reconcile each one, independently of the result of the previous one.
	// If multiple errors occur, we return the most pressing one.
	//  Order of precedence (highest -> lowest) is: error that is not an operationNotDoneError (i.e. error creating) -> operationNotDoneError (i.e. creating in progress) -> no error (i.e. created)
	for _, nsgSpec := range specs {
		if _, err := s.CreateResource(ctx, nsgSpec, serviceName); err != nil {
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
	_, log, done := tele.StartSpanWithLogger(ctx, "securitygroups.Service.Delete")
	defer done()

	log.V(4).Info("Network security group will be deleted when its parent Virtual Network is deleted")
	return nil
}

// IsManaged returns true if the security groups' lifecycles are managed.
func (s *Service) IsManaged(ctx context.Context) (bool, error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "securitygroups.Service.IsManaged")
	defer done()

	return s.Scope.IsVnetManaged(), nil
}

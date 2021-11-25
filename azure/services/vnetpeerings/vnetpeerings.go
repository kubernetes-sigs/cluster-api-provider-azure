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

package vnetpeerings

import (
	"context"

	"github.com/go-logr/logr"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "vnetpeerings"

// VnetPeeringScope defines the scope interface for a subnet service.
type VnetPeeringScope interface {
	logr.Logger
	azure.Authorizer
	azure.AsyncStatusUpdater
	VnetPeeringSpecs() []azure.ResourceSpecGetter
}

// Service provides operations on Azure resources.
type Service struct {
	Scope VnetPeeringScope
	Client
}

// New creates a new service.
func New(scope VnetPeeringScope) *Service {
	return &Service{
		Scope:  scope,
		Client: NewClient(scope),
	}
}

// Reconcile gets/creates/updates a peering.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "vnetpeerings.Service.Reconcile")
	defer span.End()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	// We go through the list of VnetPeeringSpecs to reconcile each one, independently of the result of the previous one.
	// If multiple errors occur, we return the most pressing one
	// order of precedence is: error creating -> creating in progress -> created (no error)
	var result error
	for _, peeringSpec := range s.Scope.VnetPeeringSpecs() {
		if _, err := async.CreateResource(ctx, s.Scope, s.Client, peeringSpec, serviceName); err != nil {
			if !azure.IsOperationNotDoneError(err) || result == nil {
				result = err
			}
		}
	}

	s.Scope.UpdatePutStatus(infrav1.VnetPeeringReadyCondition, serviceName, result)
	return result
}

// Delete deletes the peering with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "vnetpeerings.Service.Delete")
	defer span.End()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	var result error

	// We go through the list of VnetPeeringSpecs to delete each one, independently of the result of the previous one.
	// If multiple errors occur, we return the most pressing one
	// order of precedence is: error deleting -> deleting in progress -> deleted (no error)
	for _, peeringSpec := range s.Scope.VnetPeeringSpecs() {
		if err := async.DeleteResource(ctx, s.Scope, s.Client, peeringSpec, serviceName); err != nil {
			if !azure.IsOperationNotDoneError(err) || result == nil {
				result = err
			}
		}
	}
	s.Scope.UpdateDeleteStatus(infrav1.VnetPeeringReadyCondition, serviceName, result)
	return result
}

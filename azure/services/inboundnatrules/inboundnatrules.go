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

package inboundnatrules

import (
	"context"

	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "inboundnatrules"

// InboundNatScope defines the scope interface for an inbound NAT service.
type InboundNatScope interface {
	azure.ClusterDescriber
	azure.AsyncStatusUpdater
	APIServerLBName() string
	InboundNatSpecs(map[int32]struct{}) []azure.ResourceSpecGetter
}

// Service provides operations on Azure resources.
type Service struct {
	Scope InboundNatScope
	client
	async.Reconciler
}

// New creates a new service.
func New(scope InboundNatScope) *Service {
	client := newClient(scope)
	return &Service{
		Scope:      scope,
		client:     client,
		Reconciler: async.New(scope, client, client),
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return serviceName
}

// Reconcile gets/creates/updates an inbound NAT rule.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "inboundnatrules.Service.Reconcile")
	defer done()

	// Externally managed clusters might not have an LB
	if s.Scope.APIServerLBName() == "" {
		log.V(4).Info("Skipping InboundNatRule reconciliation as the cluster has no LB configured")
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	existingRules, err := s.client.List(ctx, s.Scope.ResourceGroup(), s.Scope.APIServerLBName())
	if err != nil {
		result := errors.Wrapf(err, "failed to get existing NAT rules")
		s.Scope.UpdatePutStatus(infrav1.InboundNATRulesReadyCondition, serviceName, result)
		return result
	}

	portsInUse := make(map[int32]struct{})
	for _, rule := range existingRules {
		portsInUse[*rule.InboundNatRulePropertiesFormat.FrontendPort] = struct{}{} // Mark frontend port as in use
	}

	specs := s.Scope.InboundNatSpecs(portsInUse)
	if len(specs) == 0 {
		return nil
	}

	// We go through the list of InboundNatSpecs to reconcile each one, independently of the result of the previous one.
	// If multiple errors occur, we return the most pressing one.
	//  Order of precedence (highest -> lowest) is: error that is not an operationNotDoneError (i.e. error creating) -> operationNotDoneError (i.e. creating in progress) -> no error (i.e. created)
	var result error
	for _, natRule := range specs {
		// If we are creating multiple inbound NAT rules, we could have a collision in finding an available frontend port since the newly created rule takes an available port, and we do not update portsInUse in the specs.
		// It doesn't matter in this case since we only create one rule per machine, but for multiple rules, we could end up restarting the Reconcile function each time to get the updated available ports.
		// TODO: We can update the available ports and recompute the specs each time, or alternatively, we could deterministically calculate the ports we plan on using to avoid collisions, i.e. rule #1 uses the first available port, rule #2 uses the second available port, etc.
		if _, err := s.CreateResource(ctx, natRule, serviceName); err != nil {
			if !azure.IsOperationNotDoneError(err) || result == nil {
				result = err
			}
		}
	}

	s.Scope.UpdatePutStatus(infrav1.InboundNATRulesReadyCondition, serviceName, result)

	return result
}

// Delete deletes the inbound NAT rule with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "inboundnatrules.Service.Delete")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	specs := s.Scope.InboundNatSpecs(make(map[int32]struct{}))
	if len(specs) == 0 {
		return nil
	}

	// We go through the list of InboundNatSpecs to delete each one, independently of the result of the previous one.
	// If multiple errors occur, we return the most pressing one.
	//  Order of precedence (highest -> lowest) is: error that is not an operationNotDoneError (i.e. error deleting) -> operationNotDoneError (i.e. deleting in progress) -> no error (i.e. deleted)
	var result error
	for _, natRule := range specs {
		if err := s.DeleteResource(ctx, natRule, serviceName); err != nil {
			if !azure.IsOperationNotDoneError(err) || result == nil {
				result = err
			}
		}
	}

	s.Scope.UpdateDeleteStatus(infrav1.InboundNATRulesReadyCondition, serviceName, result)
	return result
}

// IsManaged returns always returns true as CAPZ does not support BYO inbound NAT rules.
func (s *Service) IsManaged(ctx context.Context) (bool, error) {
	return true, nil
}

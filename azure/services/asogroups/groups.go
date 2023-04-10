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

package asogroups

import (
	"context"

	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/aso"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceName is the name of this service.
const ServiceName = "group"

// Service provides operations on Azure resources.
type Service struct {
	Scope GroupScope
	aso.Reconciler
}

// GroupScope defines the scope interface for a group service.
type GroupScope interface {
	azure.AsyncStatusUpdater
	ASOGroupSpec() azure.ASOResourceSpecGetter
	GetClient() client.Client
}

// New creates a new service.
func New(scope GroupScope) *Service {
	return &Service{
		Scope:      scope,
		Reconciler: aso.New(scope.GetClient()),
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return ServiceName
}

// Reconcile idempotently creates or updates a resource group.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "groups.Service.Reconcile")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	groupSpec := s.Scope.ASOGroupSpec()
	if groupSpec == nil {
		return nil
	}

	_, err := s.CreateOrUpdateResource(ctx, groupSpec, ServiceName)
	s.Scope.UpdatePutStatus(infrav1.ResourceGroupReadyCondition, ServiceName, err)
	return err
}

// Delete deletes the resource group if it is managed by capz.
func (s *Service) Delete(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "groups.Service.Delete")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	groupSpec := s.Scope.ASOGroupSpec()
	if groupSpec == nil {
		return nil
	}

	// check that the resource group is not BYO.
	managed, err := s.IsManaged(ctx)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// already deleted or doesn't exist, cleanup status and return.
			s.Scope.UpdateDeleteStatus(infrav1.ResourceGroupReadyCondition, ServiceName, nil)
			return nil
		}
		return errors.Wrap(err, "could not get resource group management state")
	}
	if !managed {
		log.V(2).Info("Skipping resource group deletion in unmanaged mode")
		return nil
	}

	err = s.DeleteResource(ctx, groupSpec, ServiceName)
	s.Scope.UpdateDeleteStatus(infrav1.ResourceGroupReadyCondition, ServiceName, err)
	return err
}

// IsManaged returns true if the resource group has an owned tag with the cluster name as value,
// meaning that the resource group's lifecycle is managed.
func (s *Service) IsManaged(ctx context.Context) (bool, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "groups.Service.IsManaged")
	defer done()

	spec := s.Scope.ASOGroupSpec()
	group := &asoresourcesv1.ResourceGroup{}
	err := s.Scope.GetClient().Get(ctx, client.ObjectKeyFromObject(spec.ResourceRef()), group)
	if err != nil {
		log.Error(err, "error getting resource group")
		return false, err
	}

	return true, nil // not yet implemented
}

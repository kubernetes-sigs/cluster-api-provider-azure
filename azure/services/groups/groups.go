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

package groups

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/asyncpoller"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// ServiceName is the name of this service.
const ServiceName = "group"

// Service provides operations on Azure resources.
type Service struct {
	Scope GroupScope
	asyncpoller.Reconciler
	client
}

// GroupScope defines the scope interface for a group service.
type GroupScope interface {
	azure.Authorizer
	azure.AsyncStatusUpdater
	GroupSpec() azure.ResourceSpecGetter
	ClusterName() string
}

// New creates a new service.
func New(scope GroupScope) (*Service, error) {
	client, err := newClient(scope)
	if err != nil {
		return nil, err
	}
	return &Service{
		Scope:  scope,
		client: client,
		Reconciler: asyncpoller.New[armresources.ResourceGroupsClientCreateOrUpdateResponse,
			armresources.ResourceGroupsClientDeleteResponse](scope, client, client),
	}, nil
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

	groupSpec := s.Scope.GroupSpec()
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

	groupSpec := s.Scope.GroupSpec()
	if groupSpec == nil {
		return nil
	}

	// check that the resource group is not BYO.
	managed, err := s.IsManaged(ctx)
	if err != nil {
		if azure.ResourceNotFound(err) {
			// already deleted or doesn't exist, cleanup status and return.
			s.Scope.DeleteLongRunningOperationState(groupSpec.ResourceName(), ServiceName, infrav1.DeleteFuture)
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
	ctx, _, done := tele.StartSpanWithLogger(ctx, "groups.Service.IsManaged")
	defer done()

	groupSpec := s.Scope.GroupSpec()
	groupIface, err := s.client.Get(ctx, groupSpec)
	if err != nil {
		return false, err
	}
	group, ok := groupIface.(armresources.ResourceGroup)
	if !ok {
		return false, errors.Errorf("%T is not a armresources.ResourceGroup", groupIface)
	}

	tags := converters.MapToTags(group.Tags)
	return tags.HasOwned(s.Scope.ClusterName()), nil
}

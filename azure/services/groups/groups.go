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

package groups

import (
	"context"

	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	asoannotations "github.com/Azure/azure-service-operator/v2/pkg/common/annotations"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/aso"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ServiceName is the name of this service.
const ServiceName = "group"

// Service provides operations on Azure resources.
type Service struct {
	Scope GroupScope
	*aso.Service[ASOType, GroupScope]
}

// GroupScope defines the scope interface for a group service.
type GroupScope interface {
	azure.AsyncStatusUpdater
	aso.Scope
	GroupSpec() azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]
}

// New creates a new service.
func New(scope GroupScope) *Service {
	svc := aso.NewService[ASOType](ServiceName, scope)
	svc.Specs = []azure.ASOResourceSpecGetter[ASOType]{scope.GroupSpec()}
	svc.PostReconcileHook = postReconcileHook
	svc.PostDeleteHook = postDeleteHook
	return &Service{
		Scope:   scope,
		Service: svc,
	}
}

func postReconcileHook(scope GroupScope, err error) error {
	scope.UpdatePutStatus(infrav1.ResourceGroupReadyCondition, ServiceName, err)
	return err
}

func postDeleteHook(scope GroupScope, err error) error {
	scope.UpdateDeleteStatus(infrav1.ResourceGroupReadyCondition, ServiceName, err)
	return err
}

// IsManaged returns true if the ASO ResourceGroup was created by CAPZ,
// meaning that the resource group's lifecycle is managed.
func (s *Service) IsManaged(ctx context.Context) (bool, error) {
	return aso.IsManaged(ctx, s.Scope.GetClient(), s.Specs[0], s.Scope.ClusterName())
}

// ShouldDeleteIndividualResources returns false if the resource group is
// managed and reconciled by ASO, meaning that we can rely on a single resource
// group delete operation as opposed to deleting every individual resource.
func (s *Service) ShouldDeleteIndividualResources(ctx context.Context) bool {
	// Since this is a best effort attempt to speed up delete, we don't fail the delete if we can't get the RG status.
	// Instead, take the long way and delete all resources one by one.
	managed, err := s.IsManaged(ctx)
	if err != nil || !managed {
		return true
	}

	// For ASO, "managed" only tells us that we're allowed to delete the ASO
	// resource. We also need to check that deleting the ASO resource will really
	// delete the underlying resource group by checking the ASO reconcile-policy.
	group := s.Specs[0].ResourceRef()
	err = s.Scope.GetClient().Get(ctx, client.ObjectKeyFromObject(group), group)
	return err != nil || group.GetAnnotations()[asoannotations.ReconcilePolicy] != string(asoannotations.ReconcilePolicyManage)
}

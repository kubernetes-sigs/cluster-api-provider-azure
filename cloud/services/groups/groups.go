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

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
)

// Reconcile gets/creates/updates a resource group.
func (s *Service) Reconcile(ctx context.Context) error {
	if _, err := s.Client.Get(ctx, s.Scope.ResourceGroup()); err == nil {
		// resource group already exists, skip creation
		return nil
	}
	s.Scope.V(2).Info("creating resource group", "resource group", s.Scope.ResourceGroup())
	group := resources.Group{
		Location: to.StringPtr(s.Scope.Location()),
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: s.Scope.ClusterName(),
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Name:        to.StringPtr(s.Scope.ResourceGroup()),
			Role:        to.StringPtr(infrav1.CommonRole),
			Additional:  s.Scope.AdditionalTags(),
		})),
	}

	_, err := s.Client.CreateOrUpdate(ctx, s.Scope.ResourceGroup(), group)
	if err != nil {
		return errors.Wrapf(err, "failed to create resource group %s", s.Scope.ResourceGroup())
	}

	s.Scope.V(2).Info("successfully created resource group", "resource group", s.Scope.ResourceGroup())
	return nil
}

// Delete deletes the resource group with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	managed, err := s.IsGroupManaged(ctx)
	if err != nil {
		return errors.Wrap(err, "could not get resource group management state")
	}

	if !managed {
		s.Scope.V(2).Info("Should not delete resource group in unmanaged mode")
		return azure.ErrNotOwned
	}

	s.Scope.V(2).Info("deleting resource group", "resource group", s.Scope.ResourceGroup())
	err = s.Client.Delete(ctx, s.Scope.ResourceGroup())
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "failed to delete resource group %s", s.Scope.ResourceGroup())
	}

	s.Scope.V(2).Info("successfully deleted resource group", "resource group", s.Scope.ResourceGroup())
	return nil
}

// IsGroupManaged returns true if the resource group has an owned tag with the cluster name as value,
// meaning that the resource group's lifecycle is managed.
func (s *Service) IsGroupManaged(ctx context.Context) (bool, error) {
	group, err := s.Client.Get(ctx, s.Scope.ResourceGroup())
	if err != nil {
		return false, err
	}
	tags := converters.MapToTags(group.Tags)
	return tags.HasOwned(s.Scope.ClusterName()), nil
}

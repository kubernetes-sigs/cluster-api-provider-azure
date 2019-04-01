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

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/converters"
)

// Reconcile gets/creates/updates a resource group.
func (s *Service) Reconcile(ctx context.Context, _ v1alpha1.ResourceSpec) error {
	klog.V(2).Infof("reconciling resource group %s", s.Scope.ResourceGroup().Name)
	rg, err := s.get(s.Scope.Context)
	if err != nil {
		switch {
		case s.Scope.ResourceGroup().IsProvided():
			return errors.Wrapf(err, "failed to reconcile resource group %s: an unmanaged resource group was specified, but cannot be found", s.Scope.ResourceGroup().Name)
		case !s.Scope.ResourceGroup().IsProvided():
			rg, err = s.createOrUpdate(s.Scope.Context)
			if err != nil {
				return errors.Wrapf(err, "failed to reconcile resource group %s", s.Scope.ResourceGroup().Name)
			}
		default:
			return errors.Wrapf(err, "failed to reconcile resource group %s", s.Scope.ResourceGroup().Name)
		}
	}

	rg.DeepCopyInto(s.Scope.ResourceGroup())
	klog.V(2).Infof("successfully reconciled resource group %s", s.Scope.ResourceGroup().Name)
	return nil
}

// get provides information about a resource group.
func (s *Service) get(ctx context.Context) (rg *v1alpha1.ResourceGroup, err error) {
	klog.V(2).Infof("checking for resource group %s", s.Scope.ResourceGroup().Name)
	existingRG, err := s.Client.Get(ctx, s.Scope.ResourceGroup().Name)
	if err != nil && azure.ResourceNotFound(err) {
		return nil, errors.Wrapf(err, "resource group %s not found", s.Scope.ResourceGroup().Name)
	} else if err != nil {
		return rg, err
	}

	klog.V(2).Infof("successfully retrieved resource group %s", s.Scope.ResourceGroup().Name)
	return converters.SDKToResourceGroup(existingRG, s.Scope.ResourceGroup().Managed), nil
}

// createOrUpdate creates or updates a resource group.
func (s *Service) createOrUpdate(ctx context.Context) (*v1alpha1.ResourceGroup, error) {
	klog.V(2).Infof("creating/updating resource group %s", s.Scope.ResourceGroup().Name)
	rg, err := s.Client.CreateOrUpdate(
		ctx,
		s.Scope.ResourceGroup().Name,
		resources.Group{
			Location: to.StringPtr(s.Scope.ClusterConfig.Location),
		},
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create/update resource group %s", s.Scope.ResourceGroup().Name)
	}

	klog.V(2).Infof("successfully created/updated resource group %s", s.Scope.ResourceGroup().Name)
	return converters.SDKToResourceGroup(rg, s.Scope.ResourceGroup().Managed), nil
}

// Delete deletes the resource group with the provided name.
func (s *Service) Delete(ctx context.Context, _ v1alpha1.ResourceSpec) error {
	klog.V(2).Infof("deleting resource group %s", s.Scope.ResourceGroup().Name)
	if s.Scope.ResourceGroup().IsProvided() {
		klog.V(2).Infof("resource group %s is unmanaged; skipping deletion", s.Scope.ResourceGroup().Name)
		return nil
	}

	future, err := s.Client.Delete(ctx, s.Scope.ResourceGroup().Name)
	if err != nil && azure.ResourceNotFound(err) {
		return errors.Wrapf(err, "resource group %s may have already been deleted", s.Scope.ResourceGroup().Name)
	} else if err != nil {
		return errors.Wrapf(err, "failed to delete resource group %s", s.Scope.ResourceGroup().Name)
	}

	err = future.WaitForCompletionRef(ctx, s.Client.Client)
	if err != nil {
		return errors.Wrap(err, "cannot delete, future response")
	}

	_, err = future.Result(s.Client)

	klog.V(2).Infof("successfully deleted resource group %s", s.Scope.ResourceGroup().Name)
	return err
}

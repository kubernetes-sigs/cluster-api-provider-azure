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

package publicips

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "publicips"

// PublicIPScope defines the scope interface for a public IP service.
type PublicIPScope interface {
	logr.Logger
	azure.ClusterDescriber
	azure.AsyncStatusUpdater
	PublicIPSpecs() []PublicIPSpec
}

// Service provides operations on Azure resources.
type Service struct {
	Scope PublicIPScope
	Client
}

// New creates a new service.
func New(scope PublicIPScope) *Service {
	return &Service{
		Scope:  scope,
		Client: NewClient(scope),
	}
}

// Reconcile gets/creates/updates a public ip.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "publicips.Service.Reconcile")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	// We go through the list of public ip specs to reconcile each one, independently of the result of the previous one.
	// If multiple errors occur, we return the most pressing one
	// order of precedence is: error creating -> creating in progress -> created (no error)
	var result error

	for _, ipSpec := range s.Scope.PublicIPSpecs() {
		if err := async.CreateResource(ctx, s.Scope, s.Client, ipSpec, serviceName); err != nil {
			if !azure.IsOperationNotDoneError(err) || result == nil {
				result = err
			}
		}
	}

	s.Scope.UpdatePutStatus(infrav1.PublicIPsReadyCondition, serviceName, result)
	return result
}

// TODO(karuppiah7890): Make the delete use async package and delete public IPs asynchronously
// and not block till the operation is over. Write tests of course
// Delete deletes the public IP with the provided scope.
func (s *Service) Delete(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "publicips.Service.Delete")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	// We go through the list of public ip specs to delete each one, independently of the result of the previous one.
	// If multiple errors occur, we return the most pressing one
	// order of precedence is: error deleting -> deleting in progress -> deleted (no error)
	var result error

	for _, ip := range s.Scope.PublicIPSpecs() {
		managed, err := s.isIPManaged(ctx, ip.Name)
		if err != nil {
			if azure.ResourceNotFound(err) {
				// public ip already deleted or doesn't exist
				continue
			}

			result = errors.Wrap(err, "could not get management state of test-group/my-publicip public ip")
		}

		if !managed {
			s.Scope.V(2).Info("Skipping IP deletion for unmanaged public IP", "public ip", ip.Name)
			continue
		}

		if err = async.DeleteResource(ctx, s.Scope, s.Client, ip, serviceName); err != nil {
			if !azure.IsOperationNotDoneError(err) || result == nil {
				result = err
			}
		}
	}

	s.Scope.UpdateDeleteStatus(infrav1.PublicIPsReadyCondition, serviceName, result)
	return result
}

// isIPManaged returns true if the IP has an owned tag with the cluster name as value,
// meaning that the IP's lifecycle is managed.
func (s *Service) isIPManaged(ctx context.Context, ipName string) (bool, error) {
	ip, err := s.Client.Get(ctx, s.Scope.ResourceGroup(), ipName)
	if err != nil {
		return false, err
	}
	tags := converters.MapToTags(ip.Tags)
	return tags.HasOwned(s.Scope.ClusterName()), nil
}

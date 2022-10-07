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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-08-01/network"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "publicips"

// PublicIPScope defines the scope interface for a public IP service.
type PublicIPScope interface {
	azure.Authorizer
	azure.AsyncStatusUpdater
	azure.ClusterDescriber
	PublicIPSpecs() []azure.ResourceSpecGetter
}

// Service provides operations on Azure resources.
type Service struct {
	Scope PublicIPScope
	async.Reconciler
	async.Getter
}

// New creates a new service.
func New(scope PublicIPScope) *Service {
	client := NewClient(scope)
	return &Service{
		Scope:      scope,
		Getter:     client,
		Reconciler: async.New(scope, client, client),
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return serviceName
}

// Reconcile idempotently creates or updates a public IP.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "publicips.Service.Reconcile")
	defer done()

	specs := s.Scope.PublicIPSpecs()
	if len(specs) == 0 {
		return nil
	}

	// We go through the list of PublicIPSpecs to reconcile each one, independently of the result of the previous one.
	// If multiple errors occur, we return the most pressing one.
	//  Order of precedence (highest -> lowest) is: error that is not an operationNotDoneError (i.e. error creating) -> operationNotDoneError (i.e. creating in progress) -> no error (i.e. created)
	var result error
	for _, publicIPSpec := range specs {
		if _, err := s.CreateOrUpdateResource(ctx, publicIPSpec, serviceName); err != nil {
			if !azure.IsOperationNotDoneError(err) || result == nil {
				result = err
			}
		}
	}

	s.Scope.UpdatePutStatus(infrav1.PublicIPsReadyCondition, serviceName, result)
	return result
}

// Delete deletes the public IP with the provided scope.
func (s *Service) Delete(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "publicips.Service.Delete")
	defer done()

	specs := s.Scope.PublicIPSpecs()
	if len(specs) == 0 {
		return nil
	}

	hasManagedPublicIPs := false

	// We go through the list of VnetPeeringSpecs to delete each one, independently of the result of the previous one.
	// If multiple errors occur, we return the most pressing one.
	//  Order of precedence (highest -> lowest) is: error that is not an operationNotDoneError (i.e. error deleting) -> operationNotDoneError (i.e. deleting in progress) -> no error (i.e. deleted)
	var result error
	for _, publicIPSpec := range specs {
		managed, err := s.isIPManaged(ctx, publicIPSpec)
		if err != nil && !azure.ResourceNotFound(err) {
			return errors.Wrap(err, "could not get public IP management state")
		}

		if !managed {
			log.V(2).Info("Skipping IP deletion for unmanaged public IP", "public ip", publicIPSpec.ResourceName())
			continue
		}

		log.V(2).Info("deleting public IP", "public ip", publicIPSpec.ResourceName())
		hasManagedPublicIPs = true
		if err := s.DeleteResource(ctx, publicIPSpec, serviceName); err != nil {
			if !azure.IsOperationNotDoneError(err) || result == nil {
				result = err
			}
		}

		log.V(2).Info("deleted public IP", "public ip", publicIPSpec.ResourceName())
	}

	if hasManagedPublicIPs {
		s.Scope.UpdateDeleteStatus(infrav1.PublicIPsReadyCondition, serviceName, result)
	}

	return result
}

// isIPManaged returns true if the IP has an owned tag with the cluster name as value,
// meaning that the IP's lifecycle is managed.
func (s *Service) isIPManaged(ctx context.Context, spec azure.ResourceSpecGetter) (bool, error) {
	result, err := s.Get(ctx, spec)
	if err != nil {
		return false, err
	}

	publicIP, ok := result.(network.PublicIPAddress)
	if !ok {
		return false, errors.Errorf("%T is not a network.PublicIPAddress", publicIP)
	}

	tags := converters.MapToTags(publicIP.Tags)
	return tags.HasOwned(s.Scope.ClusterName()), nil
}

// IsManaged returns always returns true as public IPs are managed on a one-by-one basis.
func (s *Service) IsManaged(ctx context.Context) (bool, error) {
	return true, nil
}

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

package subnets

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-02-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "subnets"

// SubnetScope defines the scope interface for a subnet service.
type SubnetScope interface {
	azure.Authorizer
	azure.AsyncStatusUpdater
	UpdateSubnetID(string, string)
	UpdateSubnetCIDRs(string, []string)
	IsVnetManaged() bool
	SubnetSpecs() []azure.ResourceSpecGetter
}

// Service provides operations on Azure resources.
type Service struct {
	Scope SubnetScope
	async.Reconciler
}

// New creates a new service.
func New(scope SubnetScope) *Service {
	Client := NewClient(scope)
	return &Service{
		Scope:      scope,
		Reconciler: async.New(scope, Client, Client),
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return serviceName
}

// Reconcile gets/creates/updates a subnet.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "subnets.Service.Reconcile")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	specs := s.Scope.SubnetSpecs()
	if len(specs) == 0 {
		return nil
	}

	// We go through the list of SubnetSpecs to reconcile each one, independently of the result of the previous one.
	// If multiple errors occur, we return the most pressing one.
	//  Order of precedence (highest -> lowest) is: error that is not an operationNotDoneError (i.e. error creating) -> operationNotDoneError (i.e. creating in progress) -> no error (i.e. created)
	var resultErr error
	for _, subnetSpec := range specs {
		result, err := s.CreateResource(ctx, subnetSpec, serviceName)
		if err != nil {
			if !azure.IsOperationNotDoneError(err) || resultErr == nil {
				resultErr = err
			}
		} else {
			subnet, ok := result.(network.Subnet)
			if !ok {
				return errors.Errorf("%T is not a network.Subnet", result)
			}
			s.Scope.UpdateSubnetID(subnetSpec.ResourceName(), to.String(subnet.ID))
			s.Scope.UpdateSubnetCIDRs(subnetSpec.ResourceName(), converters.GetSubnetAddresses(subnet))
		}
	}

	if s.Scope.IsVnetManaged() {
		s.Scope.UpdatePutStatus(infrav1.SubnetsReadyCondition, serviceName, resultErr)
	}

	return resultErr
}

// Delete takes no action.
// We don't need to explicitly delete a subnet;
// We can rely upon the Virtual Network to delete its subnet(s).
func (s *Service) Delete(ctx context.Context) error {
	_, log, done := tele.StartSpanWithLogger(ctx, "subnets.Service.Delete")
	defer done()

	log.V(4).Info("Subnet will be deleted when its parent Virtual Network is deleted")
	return nil
}

// IsManaged always returns false for subnets.
// This is semantically imprecise, as we aren't actually able to determine
// whether or not a subnet was created by capz or if it existed previously,
// but we must implement `IsManaged` given the ServiceReconciler type constraints.
func (s *Service) IsManaged(ctx context.Context) (bool, error) {
	return false, nil
}

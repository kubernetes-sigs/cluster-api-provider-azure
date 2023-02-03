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
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "subnets"

// SubnetScope defines the scope interface for a subnet service.
type SubnetScope interface {
	azure.ClusterScoper
	azure.AsyncStatusUpdater
	UpdateSubnetID(string, string)
	UpdateSubnetCIDRs(string, []string)
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
			var addresses []string
			if subnet.SubnetPropertiesFormat != nil && subnet.SubnetPropertiesFormat.AddressPrefix != nil {
				addresses = []string{to.String(subnet.SubnetPropertiesFormat.AddressPrefix)}
			} else if subnet.SubnetPropertiesFormat != nil && subnet.SubnetPropertiesFormat.AddressPrefixes != nil {
				addresses = to.StringSlice(subnet.SubnetPropertiesFormat.AddressPrefixes)
			}

			s.Scope.UpdateSubnetID(subnetSpec.ResourceName(), to.String(subnet.ID))
			s.Scope.UpdateSubnetCIDRs(subnetSpec.ResourceName(), addresses)
		}
	}

	if s.Scope.IsVnetManaged() {
		s.Scope.UpdatePutStatus(infrav1.SubnetsReadyCondition, serviceName, resultErr)
	}

	return resultErr
}

// Delete deletes the subnet with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "subnets.Service.Delete")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	if managed, err := s.IsManaged(ctx); err == nil && !managed {
		log.Info("Skipping subnets deletion in custom vnet mode")
		return nil
	} else if err != nil {
		return errors.Wrap(err, "failed to check if subnets are managed")
	}

	specs := s.Scope.SubnetSpecs()
	if len(specs) == 0 {
		return nil
	}

	// We go through the list of SubnetSpecs to delete each one, independently of the result of the previous one.
	// If multiple errors occur, we return the most pressing one.
	//  Order of precedence (highest -> lowest) is: error that is not an operationNotDoneError (i.e. error deleting) -> operationNotDoneError (i.e. deleting in progress) -> no error (i.e. deleted)
	var result error
	for _, subnetSpec := range specs {
		if err := s.DeleteResource(ctx, subnetSpec, serviceName); err != nil {
			if !azure.IsOperationNotDoneError(err) || result == nil {
				result = err
			}
		}
	}

	s.Scope.UpdateDeleteStatus(infrav1.SubnetsReadyCondition, serviceName, result)
	return result
}

// IsManaged returns true if the route tables' lifecycles are managed.
func (s *Service) IsManaged(ctx context.Context) (bool, error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "subnets.Service.IsManaged")
	defer done()

	return s.Scope.IsVnetManaged(), nil
}

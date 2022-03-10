/*
Copyright 2021 The Kubernetes Authors.

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

package scalesetvms

import (
	"context"
	"time"

	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "scalesetvms"

type (
	// ScaleSetVMScope defines the scope interface for a scale sets service.
	ScaleSetVMScope interface {
		azure.ClusterDescriber
		azure.AsyncStatusUpdater
		InstanceID() string
		ScaleSetName() string
		SetVMSSVM(vmssvm *azure.VMSSVM)
	}

	// Service provides operations on Azure resources.
	Service struct {
		Client client
		Scope  ScaleSetVMScope
	}
)

// NewService creates a new service.
func NewService(scope ScaleSetVMScope) *Service {
	return &Service{
		Client: newClient(scope),
		Scope:  scope,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return serviceName
}

// Reconcile idempotently gets, creates, and updates a scale set.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scalesetvms.Service.Reconcile")
	defer done()

	var (
		resourceGroup = s.Scope.ResourceGroup()
		vmssName      = s.Scope.ScaleSetName()
		instanceID    = s.Scope.InstanceID()
	)

	// fetch the latest data about the instance -- model mutations are handled by the AzureMachinePoolReconciler
	instance, err := s.Client.Get(ctx, resourceGroup, vmssName, instanceID)
	if err != nil {
		if azure.ResourceNotFound(err) {
			return azure.WithTransientError(errors.New("instance does not exist yet"), 30*time.Second)
		}
		return errors.Wrap(err, "failed getting instance")
	}

	s.Scope.SetVMSSVM(converters.SDKToVMSSVM(instance))
	return nil
}

// Delete deletes a scaleset instance asynchronously returning a future which encapsulates the long-running operation.
func (s *Service) Delete(ctx context.Context) error {
	var (
		resourceGroup = s.Scope.ResourceGroup()
		vmssName      = s.Scope.ScaleSetName()
		instanceID    = s.Scope.InstanceID()
	)

	ctx, log, done := tele.StartSpanWithLogger(
		ctx,
		"scalesetvms.Service.Delete",
		tele.KVP("resourceGroup", resourceGroup),
		tele.KVP("scaleset", vmssName),
		tele.KVP("instanceID", instanceID),
	)
	defer done()

	defer func() {
		if instance, err := s.Client.Get(ctx, resourceGroup, vmssName, instanceID); err == nil && instance.VirtualMachineScaleSetVMProperties != nil {
			log.V(4).Info("updating vmss vm state", "state", instance.ProvisioningState)
			s.Scope.SetVMSSVM(converters.SDKToVMSSVM(instance))
		}
	}()

	log.V(4).Info("entering delete")
	future := s.Scope.GetLongRunningOperationState(instanceID, serviceName)
	if future != nil {
		if future.Type != infrav1.DeleteFuture {
			return azure.WithTransientError(errors.New("attempting to delete, non-delete operation in progress"), 30*time.Second)
		}

		log.V(4).Info("checking if the instance is done deleting")
		if _, err := s.Client.GetResultIfDone(ctx, future); err != nil {
			// fetch instance to update status
			return errors.Wrap(err, "failed to get result of long running operation")
		}

		// there was no error in fetching the result, the future has been completed
		log.V(4).Info("successfully deleted the instance")
		s.Scope.DeleteLongRunningOperationState(instanceID, serviceName)
		return nil
	}

	// since the future was nil, there is no ongoing activity; start deleting the instance
	future, err := s.Client.DeleteAsync(ctx, resourceGroup, vmssName, instanceID)
	if err != nil {
		if azure.ResourceNotFound(err) {
			// already deleted
			return nil
		}
		return errors.Wrapf(err, "failed to delete instance %s/%s", vmssName, instanceID)
	}

	s.Scope.SetLongRunningOperationState(future)

	log.V(4).Info("checking if the instance is done deleting")
	if _, err := s.Client.GetResultIfDone(ctx, future); err != nil {
		// fetch instance to update status
		return errors.Wrap(err, "failed to get result of long running operation")
	}

	s.Scope.DeleteLongRunningOperationState(instanceID, serviceName)
	return nil
}

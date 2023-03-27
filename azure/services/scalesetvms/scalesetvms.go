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
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/virtualmachines"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "scalesetvms"

type (
	// ScaleSetVMScope defines the scope interface for a scale sets service.
	ScaleSetVMScope interface {
		azure.ClusterDescriber
		azure.AsyncStatusUpdater
		InstanceID() string
		ProviderID() string
		ScaleSetName() string
		OrchestrationMode() infrav1.OrchestrationModeType
		SetVMSSVM(vmssvm *azure.VMSSVM)
	}

	// Service provides operations on Azure resources.
	Service struct {
		Client   client
		VMClient virtualmachines.Client
		Scope    ScaleSetVMScope
	}
)

// NewService creates a new service.
func NewService(scope ScaleSetVMScope) *Service {
	return &Service{
		Client:   newClient(scope),
		VMClient: virtualmachines.NewClient(scope),
		Scope:    scope,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return serviceName
}

// Reconcile idempotently gets, creates, and updates a scale set.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "scalesetvms.Service.Reconcile")
	defer done()

	var (
		resourceGroup = s.Scope.ResourceGroup()
		vmssName      = s.Scope.ScaleSetName()
		instanceID    = s.Scope.InstanceID()
		providerID    = s.Scope.ProviderID()
		isFlex        = s.Scope.OrchestrationMode() == infrav1.FlexibleOrchestrationMode
	)

	// Fetch the latest instance or VM data. AzureMachinePoolReconciler handles model mutations.
	if isFlex {
		resourceID := strings.TrimPrefix(providerID, azure.ProviderIDPrefix)
		log.V(4).Info("VMSS is flex", "vmssName", vmssName, "providerID", providerID, "resourceID", resourceID)
		// Using VMSS Flex, so fetch by resource ID.
		vm, err := s.VMClient.GetByID(ctx, resourceID)
		if err != nil {
			if azure.ResourceNotFound(err) {
				return azure.WithTransientError(errors.New("vm does not exist yet"), 30*time.Second)
			}
			return errors.Wrap(err, "failed getting vm")
		}
		s.Scope.SetVMSSVM(converters.SDKVMToVMSSVM(vm, infrav1.FlexibleOrchestrationMode))
		return nil
	}

	log.V(4).Info("VMSS is uniform", "vmssName", vmssName, "providerID", providerID, "instanceID", instanceID)
	// Using VMSS Uniform, so fetch by instance ID.
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
		providerID    = s.Scope.ProviderID()
		isFlex        = s.Scope.OrchestrationMode() == infrav1.FlexibleOrchestrationMode
	)

	ctx, log, done := tele.StartSpanWithLogger(
		ctx,
		"scalesetvms.Service.Delete",
		tele.KVP("resourceGroup", resourceGroup),
		tele.KVP("scaleset", vmssName),
		tele.KVP("instanceID", instanceID),
	)
	defer done()

	if isFlex {
		return s.deleteVMSSFlexVM(ctx, strings.TrimPrefix(providerID, azure.ProviderIDPrefix))
	}
	return s.deleteVMSSUniformInstance(ctx, resourceGroup, vmssName, instanceID, log)
}

func (s *Service) deleteVMSSFlexVM(ctx context.Context, resourceID string) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "scalesetvms.Service.deleteVMSSFlexVM")
	defer done()

	defer func() {
		if vm, err := s.VMClient.GetByID(ctx, resourceID); err == nil && vm.VirtualMachineProperties != nil {
			log.V(4).Info("vmss vm delete in progress", "state", vm.ProvisioningState)
			s.Scope.SetVMSSVM(converters.SDKVMToVMSSVM(vm, s.Scope.OrchestrationMode()))
		}
	}()

	parsed, err := arm.ParseResourceID(resourceID)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to parse resource id %q", resourceID))
	}
	resourceGroup, resourceName := parsed.ResourceGroupName, parsed.Name

	log.V(4).Info("entering delete")
	future := s.Scope.GetLongRunningOperationState(resourceName, serviceName, infrav1.DeleteFuture)
	if future != nil {
		if future.Type != infrav1.DeleteFuture {
			return azure.WithTransientError(errors.New("attempting to delete, non-delete operation in progress"), 30*time.Second)
		}

		log.V(4).Info("checking if the vm is done deleting")
		if _, err := s.VMClient.GetResultIfDone(ctx, future); err != nil {
			// fetch vm to update status
			return errors.Wrap(err, "failed to get result of long running operation")
		}

		// there was no error in fetching the result, the future has been completed
		log.V(4).Info("successfully deleted the vm")
		s.Scope.DeleteLongRunningOperationState(resourceName, serviceName, infrav1.DeleteFuture)
		return nil
	}
	// since the future was nil, there is no ongoing activity; start deleting the vm
	log.V(4).Info("vmss delete vm future is nil") // This is always true

	vmGetter := &VMSSFlexVMGetter{
		Name:          resourceName,
		ResourceGroup: resourceGroup,
	}

	sdkFuture, err := s.VMClient.DeleteAsync(ctx, vmGetter)
	if err != nil {
		if azure.ResourceNotFound(err) {
			// already deleted
			return nil
		}
		return errors.Wrapf(err, "failed to delete vm %s/%s", resourceGroup, resourceName)
	}

	if sdkFuture != nil {
		future, err = converters.SDKToFuture(sdkFuture, infrav1.DeleteFuture, serviceName, vmGetter.ResourceName(), vmGetter.ResourceGroupName())
		if err != nil {
			return errors.Wrapf(err, "failed to convert SDK to Future %s/%s", resourceGroup, resourceName)
		}
		s.Scope.SetLongRunningOperationState(future)
		return nil
	}

	s.Scope.DeleteLongRunningOperationState(resourceName, serviceName, infrav1.DeleteFuture)
	return nil
}

func (s *Service) deleteVMSSUniformInstance(ctx context.Context, resourceGroup string, vmssName string, instanceID string, log logr.Logger) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "scalesetvms.Service.deleteVMSSUniformInstance")
	defer done()

	defer func() {
		if instance, err := s.Client.Get(ctx, resourceGroup, vmssName, instanceID); err == nil && instance.VirtualMachineScaleSetVMProperties != nil {
			log.V(4).Info("updating vmss vm state", "state", instance.ProvisioningState)
			s.Scope.SetVMSSVM(converters.SDKToVMSSVM(instance))
		}
	}()

	log.V(4).Info("entering delete")
	future := s.Scope.GetLongRunningOperationState(instanceID, serviceName, infrav1.DeleteFuture)
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
		s.Scope.DeleteLongRunningOperationState(instanceID, serviceName, infrav1.DeleteFuture)
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

	s.Scope.DeleteLongRunningOperationState(instanceID, serviceName, infrav1.DeleteFuture)
	return nil
}

// VMSSFlexVMGetter gets the information required to create, update, or delete an Azure resource.
type VMSSFlexVMGetter struct {
	Name          string
	ResourceGroup string
}

// ResourceName returns the name of the resource.
func (vm *VMSSFlexVMGetter) ResourceName() string {
	return vm.Name
}

// OwnerResourceName returns the name of the resource that owns this Azure subresource.
func (vm *VMSSFlexVMGetter) OwnerResourceName() string {
	return ""
}

// ResourceGroupName returns the name of the resource group the resource is in.
func (vm *VMSSFlexVMGetter) ResourceGroupName() string {
	return vm.ResourceGroup
}

// Parameters takes the existing resource and returns the desired parameters of the resource.
// If the resource does not exist, or we do not care about existing parameters to update the resource, existing should be `nil`.
// If no update is needed on the resource, Parameters should return `nil`.
// NOTE: Not yet implemented, see kubernetes-sigs/cluster-api-provider-azure#2720.
func (vm *VMSSFlexVMGetter) Parameters(ctx context.Context, existing interface{}) (params interface{}, err error) {
	return nil, nil
}

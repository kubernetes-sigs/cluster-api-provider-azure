/*
Copyright 2020 The Kubernetes Authors.

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

package v1alpha3

import (
	"errors"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
)

// log is for logging in this package.
var azuremachinepoollog = logf.Log.WithName("azuremachinepool-resource")

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (amp *AzureMachinePool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(amp).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-exp-infrastructure-cluster-x-k8s-io-v1alpha3-azuremachinepool,mutating=true,failurePolicy=fail,groups=exp.infrastructure.cluster.x-k8s.io,resources=azuremachinepools,verbs=create;update,versions=v1alpha3,name=azuremachinepool.kb.io,sideEffects=None

var _ webhook.Defaulter = &AzureMachinePool{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (amp *AzureMachinePool) Default() {
	azuremachinepoollog.Info("default", "name", amp.Name)

	err := amp.SetDefaultSSHPublicKey()
	if err != nil {
		azuremachinepoollog.Error(err, "SetDefaultSshPublicKey failed")
	}
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-exp-infrastructure-cluster-x-k8s-io-v1alpha3-azuremachinepool,mutating=false,failurePolicy=fail,groups=exp.infrastructure.cluster.x-k8s.io,resources=azuremachinepools,versions=v1alpha3,name=azuremachinepool.kb.io,sideEffects=None

var _ webhook.Validator = &AzureMachinePool{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (amp *AzureMachinePool) ValidateCreate() error {
	azuremachinepoollog.Info("validate create", "name", amp.Name)
	return amp.Validate(nil)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (amp *AzureMachinePool) ValidateUpdate(old runtime.Object) error {
	azuremachinepoollog.Info("validate update", "name", amp.Name)
	return amp.Validate(old)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (amp *AzureMachinePool) ValidateDelete() error {
	azuremachinepoollog.Info("validate delete", "name", amp.Name)
	return nil
}

// Validate the Azure Machine Pool and return an aggregate error
func (amp *AzureMachinePool) Validate(old runtime.Object) error {
	validators := []func() error{
		amp.ValidateImage,
		amp.ValidateTerminateNotificationTimeout,
		amp.ValidateSSHKey,
		amp.ValidateUserAssignedIdentity,
		amp.ValidateSystemAssignedIdentity(old),
	}

	var errs []error
	for _, validator := range validators {
		if err := validator(); err != nil {
			errs = append(errs, err)
		}
	}

	return kerrors.NewAggregate(errs)
}

// ValidateImage of an AzureMachinePool
func (amp *AzureMachinePool) ValidateImage() error {
	if amp.Spec.Template.Image != nil {
		image := amp.Spec.Template.Image
		if errs := infrav1.ValidateImage(image, field.NewPath("image")); len(errs) > 0 {
			agg := kerrors.NewAggregate(errs.ToAggregate().Errors())
			azuremachinepoollog.Info("Invalid image: %s", agg.Error())
			return agg
		}
	}

	return nil
}

// ValidateTerminateNotificationTimeout termination notification timeout to be between 5 and 15
func (amp *AzureMachinePool) ValidateTerminateNotificationTimeout() error {
	if amp.Spec.Template.TerminateNotificationTimeout == nil {
		return nil
	}
	if *amp.Spec.Template.TerminateNotificationTimeout < 5 {
		return errors.New("Minimum timeout 5 is allowed for TerminateNotificationTimeout")
	}

	if *amp.Spec.Template.TerminateNotificationTimeout > 15 {
		return errors.New("Maximum timeout 15 is allowed for TerminateNotificationTimeout")
	}

	return nil
}

// ValidateSSHKey validates an SSHKey
func (amp *AzureMachinePool) ValidateSSHKey() error {
	if amp.Spec.Template.SSHPublicKey != "" {
		sshKey := amp.Spec.Template.SSHPublicKey
		if errs := infrav1.ValidateSSHKey(sshKey, field.NewPath("sshKey")); len(errs) > 0 {
			agg := kerrors.NewAggregate(errs.ToAggregate().Errors())
			azuremachinepoollog.Info("Invalid sshKey: %s", agg.Error())
			return agg
		}
	}

	return nil
}

// ValidateUserAssignedIdentity validates the user-assigned identities list
func (amp *AzureMachinePool) ValidateUserAssignedIdentity() error {
	fldPath := field.NewPath("UserAssignedIdentities")
	if errs := infrav1.ValidateUserAssignedIdentity(amp.Spec.Identity, amp.Spec.UserAssignedIdentities, fldPath); len(errs) > 0 {
		return kerrors.NewAggregate(errs.ToAggregate().Errors())
	}

	return nil
}

// ValidateSystemAssignedIdentity validates system-assigned identity role
func (amp *AzureMachinePool) ValidateSystemAssignedIdentity(old runtime.Object) func() error {
	return func() error {
		var oldRole string
		if old != nil {
			oldMachinePool, ok := old.(*AzureMachinePool)
			if !ok {
				return fmt.Errorf("unexpected type for old azure machine pool object. Expected: %q, Got: %q",
					"AzureMachinePool", reflect.TypeOf(old))
			}
			oldRole = oldMachinePool.Spec.RoleAssignmentName
		}

		fldPath := field.NewPath("roleAssignmentName")
		if errs := infrav1.ValidateSystemAssignedIdentity(amp.Spec.Identity, oldRole, amp.Spec.RoleAssignmentName, fldPath); len(errs) > 0 {
			return kerrors.NewAggregate(errs.ToAggregate().Errors())
		}

		return nil
	}
}

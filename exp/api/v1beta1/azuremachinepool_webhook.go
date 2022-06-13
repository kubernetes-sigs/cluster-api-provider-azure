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

package v1beta1

import (
	"errors"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/feature"
	capifeature "sigs.k8s.io/cluster-api/feature"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (amp *AzureMachinePool) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(amp).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-azuremachinepool,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=azuremachinepools,verbs=create;update,versions=v1beta1,name=default.azuremachinepool.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Defaulter = &AzureMachinePool{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (amp *AzureMachinePool) Default() {
	if err := amp.SetDefaultSSHPublicKey(); err != nil {
		ctrl.Log.WithName("AzureMachinePoolLogger").Error(err, "SetDefaultSshPublicKey failed")
	}
	amp.SetIdentityDefaults()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-azuremachinepool,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=azuremachinepools,versions=v1beta1,name=validation.azuremachinepool.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Validator = &AzureMachinePool{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (amp *AzureMachinePool) ValidateCreate() error {
	return amp.Validate(nil)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (amp *AzureMachinePool) ValidateUpdate(old runtime.Object) error {
	return amp.Validate(old)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (amp *AzureMachinePool) ValidateDelete() error {
	return nil
}

// Validate the Azure Machine Pool and return an aggregate error.
func (amp *AzureMachinePool) Validate(old runtime.Object) error {
	// NOTE: AzureMachinePool is behind MachinePool feature gate flag; the web hook
	// must prevent creating new objects new case the feature flag is disabled.
	if !feature.Gates.Enabled(capifeature.MachinePool) {
		return field.Forbidden(
			field.NewPath("spec"),
			"can be set only if the MachinePool feature flag is enabled",
		)
	}

	validators := []func() error{
		amp.ValidateImage,
		amp.ValidateTerminateNotificationTimeout,
		amp.ValidateSSHKey,
		amp.ValidateUserAssignedIdentity,
		amp.ValidateStrategy(),
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

// ValidateImage of an AzureMachinePool.
func (amp *AzureMachinePool) ValidateImage() error {
	if amp.Spec.Template.Image != nil {
		image := amp.Spec.Template.Image
		if errs := infrav1.ValidateImage(image, field.NewPath("image")); len(errs) > 0 {
			agg := kerrors.NewAggregate(errs.ToAggregate().Errors())
			return agg
		}
	}

	return nil
}

// ValidateTerminateNotificationTimeout termination notification timeout to be between 5 and 15.
func (amp *AzureMachinePool) ValidateTerminateNotificationTimeout() error {
	if amp.Spec.Template.TerminateNotificationTimeout == nil {
		return nil
	}
	if *amp.Spec.Template.TerminateNotificationTimeout < 5 {
		return errors.New("minimum timeout 5 is allowed for TerminateNotificationTimeout")
	}

	if *amp.Spec.Template.TerminateNotificationTimeout > 15 {
		return errors.New("maximum timeout 15 is allowed for TerminateNotificationTimeout")
	}

	return nil
}

// ValidateSSHKey validates an SSHKey.
func (amp *AzureMachinePool) ValidateSSHKey() error {
	if amp.Spec.Template.SSHPublicKey != "" {
		sshKey := amp.Spec.Template.SSHPublicKey
		if errs := infrav1.ValidateSSHKey(sshKey, field.NewPath("sshKey")); len(errs) > 0 {
			agg := kerrors.NewAggregate(errs.ToAggregate().Errors())
			return agg
		}
	}

	return nil
}

// ValidateUserAssignedIdentity validates the user-assigned identities list.
func (amp *AzureMachinePool) ValidateUserAssignedIdentity() error {
	fldPath := field.NewPath("UserAssignedIdentities")
	if errs := infrav1.ValidateUserAssignedIdentity(amp.Spec.Identity, amp.Spec.UserAssignedIdentities, fldPath); len(errs) > 0 {
		return kerrors.NewAggregate(errs.ToAggregate().Errors())
	}

	return nil
}

// ValidateStrategy validates the strategy.
func (amp *AzureMachinePool) ValidateStrategy() func() error {
	return func() error {
		if amp.Spec.Strategy.Type == RollingUpdateAzureMachinePoolDeploymentStrategyType && amp.Spec.Strategy.RollingUpdate != nil {
			rollingUpdateStrategy := amp.Spec.Strategy.RollingUpdate
			maxSurge := rollingUpdateStrategy.MaxSurge
			maxUnavailable := rollingUpdateStrategy.MaxUnavailable
			if maxSurge.Type == intstr.Int && maxSurge.IntVal == 0 &&
				maxUnavailable.Type == intstr.Int && maxUnavailable.IntVal == 0 {
				return errors.New("rolling update strategy MaxUnavailable must not be 0 if MaxSurge is 0")
			}
		}

		return nil
	}
}

// ValidateSystemAssignedIdentity validates system-assigned identity role.
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

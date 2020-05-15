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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var machinelog = logf.Log.WithName("azuremachine-resource")

// SetupWebhookWithManager will setup and register the webhook with the controller mnager
func (m *AzureMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha3-azuremachine,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=azuremachines,versions=v1alpha3,name=validation.azuremachine.infrastructure.cluster.x-k8s.io,sideEffects=None

var _ webhook.Validator = &AzureMachine{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (m *AzureMachine) ValidateCreate() error {
	machinelog.Info("validate create", "name", m.Name)

	if errs := ValidateImage(m.Spec.Image, field.NewPath("image")); len(errs) > 0 {
		return apierrors.NewInvalid(
			GroupVersion.WithKind("AzureMachine").GroupKind(),
			m.Name, errs)
	}

	if errs := ValidateSSHKey(m.Spec.SSHPublicKey, field.NewPath("sshPublicKey")); len(errs) > 0 {
		return apierrors.NewInvalid(
			GroupVersion.WithKind("AzureMachine").GroupKind(),
			m.Name, errs)
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (m *AzureMachine) ValidateUpdate(old runtime.Object) error {
	machinelog.Info("validate update", "name", m.Name)

	if errs := ValidateSSHKey(m.Spec.SSHPublicKey, field.NewPath("sshPublicKey")); len(errs) > 0 {
		return apierrors.NewInvalid(
			GroupVersion.WithKind("AzureMachine").GroupKind(),
			m.Name, errs)
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (m *AzureMachine) ValidateDelete() error {
	machinelog.Info("validate delete", "name", m.Name)

	return nil
}

// Default implements webhookutil.defaulter so a webhook will be registered for the type
func (m *AzureMachine) Default() {
	machinelog.Info("default", "name", m.Name)

	err := m.SetDefaultSSHPublicKey()
	if err != nil {
		machinelog.Error(err, "SetDefaultSshPublicKey failed")
	}
}

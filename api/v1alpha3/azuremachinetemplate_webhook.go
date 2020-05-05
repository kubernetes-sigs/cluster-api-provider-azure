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
var _ = logf.Log.WithName("azuremachinetemplate-resource")

func (r *AzureMachineTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha3-azuremachinetemplate,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=azuremachinetemplate,versions=v1alpha3,name=validation.azuremachinetemplate.infrastructure.cluster.x-k8s.io

var _ webhook.Validator = &AzureMachineTemplate{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *AzureMachineTemplate) ValidateCreate() error {
	machinelog.Info("validate create", "name", r.Name)

	if errs := ValidateImage(r.Spec.Template.Spec.Image, field.NewPath("image")); len(errs) > 0 {
		return apierrors.NewInvalid(
			GroupVersion.WithKind("AzureMachineTemplate").GroupKind(),
			r.Name, errs)
	}

	if errs := ValidateSSHKey(r.Spec.Template.Spec.SSHPublicKey, field.NewPath("sshPublicKey")); len(errs) > 0 {
		return apierrors.NewInvalid(
			GroupVersion.WithKind("AzureMachineTemplate").GroupKind(),
			r.Name, errs)
	}

	if errs := ValidateDataDisks(r.Spec.Template.Spec.DataDisks, field.NewPath("dataDisks")); len(errs) > 0 {
		return apierrors.NewInvalid(
			GroupVersion.WithKind("AzureMachineTemplate").GroupKind(),
			r.Name, errs)
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *AzureMachineTemplate) ValidateUpdate(old runtime.Object) error {
	machinelog.Info("validate update", "name", r.Name)

	if errs := ValidateSSHKey(r.Spec.Template.Spec.SSHPublicKey, field.NewPath("sshPublicKey")); len(errs) > 0 {
		return apierrors.NewInvalid(
			GroupVersion.WithKind("AzureMachineTemplate").GroupKind(),
			r.Name, errs)
	}

	if errs := ValidateDataDisks(r.Spec.Template.Spec.DataDisks, field.NewPath("dataDisks")); len(errs) > 0 {
		return apierrors.NewInvalid(
			GroupVersion.WithKind("AzureMachineTemplate").GroupKind(),
			r.Name, errs)
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *AzureMachineTemplate) ValidateDelete() error {
	machinelog.Info("validate delete", "name", r.Name)

	return nil
}

// Default implements webhookutil.defaulter so a webhook will be registered for the type
func (r *AzureMachineTemplate) Default() {
	machinelog.Info("default", "name", r.Name)

	// err := r.SetDefaultSSHPublicKey()
	// if err != nil {
	// 	machinelog.Error(err, "SetDefaultSshPublicKey failed")
	// }

	// err = r.SetDefaultsDataDisks()
	// if err != nil {
	// 	machinelog.Error(err, "SetDefaultDataDisks failed")
	// }
}

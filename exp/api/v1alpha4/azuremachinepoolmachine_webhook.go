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

package v1alpha4

import (
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var azuremachinepoolmachinelog = logf.Log.WithName("azuremachinepoolmachine-resource")

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (ampm *AzureMachinePoolMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(ampm).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha4-azuremachinepoolmachine,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=azuremachinepoolmachines,versions=v1alpha4,name=azuremachinepoolmachine.kb.io,sideEffects=None,admissionReviewVersions=v1beta1

var _ webhook.Validator = &AzureMachinePoolMachine{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (ampm *AzureMachinePoolMachine) ValidateCreate() error {
	azuremachinepoolmachinelog.Info("validate create", "name", ampm.Name)
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (ampm *AzureMachinePoolMachine) ValidateUpdate(old runtime.Object) error {
	azuremachinepoolmachinelog.Info("validate update", "name", ampm.Name)
	oldMachine, ok := old.(*AzureMachinePoolMachine)
	if !ok {
		return errors.New("expected and AzureMachinePoolMachine")
	}

	if oldMachine.Spec.ProviderID != "" && ampm.Spec.ProviderID != oldMachine.Spec.ProviderID {
		return errors.New("providerID is immutable")
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (ampm *AzureMachinePoolMachine) ValidateDelete() error {
	azuremachinepoolmachinelog.Info("validate delete", "name", ampm.Name)
	return nil
}

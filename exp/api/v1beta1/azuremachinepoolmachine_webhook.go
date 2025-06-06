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
	"context"
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (ampm *AzureMachinePoolMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	w := new(azureMachinePoolMachineWebhook)
	return ctrl.NewWebhookManagedBy(mgr).
		For(ampm).
		WithValidator(w).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-azuremachinepoolmachine,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=azuremachinepoolmachines,versions=v1beta1,name=azuremachinepoolmachine.kb.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

type azureMachinePoolMachineWebhook struct{}

var _ webhook.CustomValidator = &azureMachinePoolMachineWebhook{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (*azureMachinePoolMachineWebhook) ValidateCreate(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (*azureMachinePoolMachineWebhook) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	ampm, ok := newObj.(*AzureMachinePoolMachine)
	if !ok {
		return nil, fmt.Errorf("expected an AzureMachinePoolMachine object but got %T", ampm)
	}

	oldMachine, ok := oldObj.(*AzureMachinePoolMachine)
	if !ok {
		return nil, errors.New("expected and AzureMachinePoolMachine")
	}

	if oldMachine.Spec.ProviderID != "" && ampm.Spec.ProviderID != oldMachine.Spec.ProviderID {
		return nil, errors.New("providerID is immutable")
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (*azureMachinePoolMachineWebhook) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

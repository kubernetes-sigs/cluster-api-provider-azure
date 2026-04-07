/*
Copyright The Kubernetes Authors.

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

package webhooks

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta2"
	webhookutils "sigs.k8s.io/cluster-api-provider-azure/util/webhook"
)

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (w *AzureClusterIdentityWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&infrav1.AzureClusterIdentity{}).
		WithValidator(w).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-azureclusteridentity,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=azureclusteridentities,versions=v1beta1,name=validation.azureclusteridentity.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

// AzureClusterIdentityWebhook implements a validating webhook for AzureClusterIdentity.
type AzureClusterIdentityWebhook struct{}

var _ webhook.CustomValidator = &AzureClusterIdentityWebhook{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (*AzureClusterIdentityWebhook) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	c, ok := obj.(*infrav1.AzureClusterIdentity)
	if !ok {
		return nil, fmt.Errorf("expected an AzureClusterIdentity object but got %T", c)
	}

	return validateAzureClusterIdentity(c)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (*AzureClusterIdentityWebhook) ValidateUpdate(_ context.Context, oldRaw, newObj runtime.Object) (admission.Warnings, error) {
	c, ok := newObj.(*infrav1.AzureClusterIdentity)
	if !ok {
		return nil, fmt.Errorf("expected an AzureClusterIdentity object but got %T", c)
	}

	var allErrs field.ErrorList
	old := oldRaw.(*infrav1.AzureClusterIdentity)
	if err := webhookutils.ValidateImmutable(
		field.NewPath("Spec", "Type"),
		old.Spec.Type,
		c.Spec.Type); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return validateAzureClusterIdentity(c)
	}
	return nil, apierrors.NewInvalid(infrav1.GroupVersion.WithKind(infrav1.AzureClusterIdentityKind).GroupKind(), c.Name, allErrs)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (*AzureClusterIdentityWebhook) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

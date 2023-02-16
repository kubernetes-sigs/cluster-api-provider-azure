/*
Copyright 2023 The Kubernetes Authors.

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
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/cluster-api-provider-azure/feature"
	capifeature "sigs.k8s.io/cluster-api/feature"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// AzureManagedControlPlaneTemplateImmutableMsg is the message used for errors on fields that are immutable.
const AzureManagedControlPlaneTemplateImmutableMsg = "AzureManagedControlPlaneTemplate spec.template.spec field is immutable. Please create new resource instead. ref doc: https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-class/change-clusterclass.html"

// SetupAzureManagedControlPlaneTemplateWithManager will set up the webhook to be managed by the specified manager.
func SetupAzureManagedControlPlaneTemplateWithManager(mgr ctrl.Manager) error {
	mcpw := &azureManagedControlPlaneTemplateWebhook{Client: mgr.GetClient()}
	return ctrl.NewWebhookManagedBy(mgr).
		For(&AzureManagedControlPlaneTemplate{}).
		WithDefaulter(mcpw).
		WithValidator(mcpw).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-azuremanagedcontrolplanetemplate,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedcontrolplanetemplates,versions=v1beta1,name=validation.azuremanagedcontrolplanetemplate.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-azuremanagedcontrolplanetemplate,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedcontrolplanetemplates,versions=v1beta1,name=default.azuremanagedcontrolplanetemplate.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

type azureManagedControlPlaneTemplateWebhook struct {
	Client client.Client
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (mcpw *azureManagedControlPlaneTemplateWebhook) Default(ctx context.Context, obj runtime.Object) error {
	mcp, ok := obj.(*AzureManagedControlPlaneTemplate)
	if !ok {
		return apierrors.NewBadRequest("expected an AzureManagedControlPlaneTemplate")
	}
	mcp.setDefaults()
	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (mcpw *azureManagedControlPlaneTemplateWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	mcp, ok := obj.(*AzureManagedControlPlaneTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest("expected an AzureManagedControlPlaneTemplate")
	}
	// NOTE: AzureManagedControlPlane relies upon MachinePools, which is behind a feature gate flag.
	// The webhook must prevent creating new objects in case the feature flag is disabled.
	if !feature.Gates.Enabled(capifeature.MachinePool) {
		return nil, field.Forbidden(
			field.NewPath("spec"),
			"can be set only if the Cluster API 'MachinePool' feature flag is enabled",
		)
	}

	return nil, mcp.validateManagedControlPlaneTemplate(mcpw.Client)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (mcpw *azureManagedControlPlaneTemplateWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	var allErrs field.ErrorList
	old, ok := oldObj.(*AzureManagedControlPlaneTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest("expected an AzureManagedControlPlaneTemplate")
	}
	mcp, ok := newObj.(*AzureManagedControlPlaneTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest("expected an AzureManagedControlPlaneTemplate")
	}
	if !reflect.DeepEqual(mcp.Spec.Template.Spec, old.Spec.Template.Spec) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("AzureManagedControlPlaneTemplate", "spec", "template", "spec"), mcp, AzureManagedControlPlaneTemplateImmutableMsg),
		)
	}

	if len(allErrs) == 0 {
		return nil, nil
	}
	return nil, apierrors.NewInvalid(GroupVersion.WithKind("AzureManagedControlPlaneTemplate").GroupKind(), mcp.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (mcpw *azureManagedControlPlaneTemplateWebhook) ValidateDelete(ctx context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

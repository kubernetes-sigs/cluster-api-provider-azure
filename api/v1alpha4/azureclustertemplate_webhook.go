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
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// AzureClusterTemplateImmutableMsg ...
const AzureClusterTemplateImmutableMsg = "AzureClusterTemplate spec.template.spec field is immutable. Please create new resource instead. ref doc: https://cluster-api.sigs.k8s.io/tasks/change-cluster-template.html"

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (r *AzureClusterTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha4-azureclustertemplate,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=azureclustertemplates,versions=v1alpha4,name=validation.azureclustertemplate.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1alpha4-azureclustertemplate,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=azureclustertemplates,versions=v1alpha4,name=default.azureclustertemplate.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1beta1

var _ webhook.Defaulter = &AzureClusterTemplate{}

// Default implements webhookutil.defaulter so a webhook will be registered for the type.
func (r *AzureClusterTemplate) Default() {
	r.Spec.Template.Spec.setDefaults(r.Name)
}

var _ webhook.Validator = &AzureClusterTemplate{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *AzureClusterTemplate) ValidateCreate() error {
	allErrs := r.Spec.Template.Spec.validateClusterSpec(nil)
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(
		schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "AzureClusterTemplate"},
		r.Name, allErrs,
	)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *AzureClusterTemplate) ValidateUpdate(oldRaw runtime.Object) error {
	var allErrs field.ErrorList
	old := oldRaw.(*AzureClusterTemplate)
	if !reflect.DeepEqual(r.Spec.Template.Spec, old.Spec.Template.Spec) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("AzureClusterTemplate", "spec", "template", "spec"), r, AzureClusterTemplateImmutableMsg),
		)
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("AzureClusterTemplate").GroupKind(), r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *AzureClusterTemplate) ValidateDelete() error {
	return nil
}

/*
Copyright 2022 The Kubernetes Authors.

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
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// AzureClusterTemplateImmutableMsg is the message used for errors on fields that are immutable.
const AzureClusterTemplateImmutableMsg = "AzureClusterTemplate spec.template.spec field is immutable. Please create new resource instead. ref doc: https://cluster-api.sigs.k8s.io/tasks/experimental-features/cluster-class/change-clusterclass.html"

// SetupWebhookWithManager will set up the webhook to be managed by the specified manager.
func (c *AzureClusterTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-azureclustertemplate,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=azureclustertemplates,versions=v1beta1,name=validation.azureclustertemplate.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-azureclustertemplate,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=azureclustertemplates,versions=v1beta1,name=default.azureclustertemplate.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Defaulter = &AzureClusterTemplate{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (c *AzureClusterTemplate) Default() {
	c.setDefaults()
}

var _ webhook.Validator = &AzureClusterTemplate{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (c *AzureClusterTemplate) ValidateCreate() error {
	return c.validateClusterTemplate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (c *AzureClusterTemplate) ValidateUpdate(oldRaw runtime.Object) error {
	var allErrs field.ErrorList
	old := oldRaw.(*AzureClusterTemplate)
	if !reflect.DeepEqual(c.Spec.Template.Spec, old.Spec.Template.Spec) {
		// The equality failure could be because of default mismatch after default use NATGayeway for outbound traffic.
		// the new object `c` will have run through the default webhooks but the old object `old` would not have so.
		// This means the old object will have default NodeOutboundLB, but the new one will not have default NodeOutboundLB.
		// This will result in object inequality. To workaround this, we set the old object defaults here so that the old object also gets
		// the new defaults.

		// We need to set the NodeOutboundLB if there has an existing one
		if old.Spec.Template.Spec.NetworkSpec.NodeOutboundLB != nil {
			old.Spec.Template.Spec.NetworkSpec.NodeOutboundLB = c.Spec.Template.Spec.NetworkSpec.NodeOutboundLB
			old.Default()
			c.Default()
		}

		// if it's still not equal, return error.
		if !reflect.DeepEqual(c.Spec.Template.Spec, old.Spec.Template.Spec) {
			allErrs = append(allErrs,
				field.Invalid(field.NewPath("AzureClusterTemplate", "spec", "template", "spec"), c, AzureClusterTemplateImmutableMsg),
			)
		}
	}

	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(GroupVersion.WithKind("AzureClusterTemplate").GroupKind(), c.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (c *AzureClusterTemplate) ValidateDelete() error {
	return nil
}

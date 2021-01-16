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
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var clusterlog = logf.Log.WithName("azurecluster-resource")

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (c *AzureCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(c).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha3-azurecluster,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=azureclusters,versions=v1alpha3,name=validation.azurecluster.infrastructure.cluster.x-k8s.io,sideEffects=None
// +kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1alpha3-azurecluster,mutating=true,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=azureclusters,versions=v1alpha3,name=default.azurecluster.infrastructure.cluster.x-k8s.io,sideEffects=None

var _ webhook.Validator = &AzureCluster{}
var _ webhook.Defaulter = &AzureCluster{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (c *AzureCluster) Default() {
	clusterlog.Info("default", "name", c.Name)

	c.setDefaults()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (c *AzureCluster) ValidateCreate() error {
	clusterlog.Info("validate create", "name", c.Name)

	return c.validateCluster(nil)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (c *AzureCluster) ValidateUpdate(oldRaw runtime.Object) error {
	clusterlog.Info("validate update", "name", c.Name)
	var allErrs field.ErrorList
	old := oldRaw.(*AzureCluster)

	if !reflect.DeepEqual(c.Spec.ResourceGroup, old.Spec.ResourceGroup) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "ResourceGroup"),
				c.Spec.ResourceGroup, "field is immutable"),
		)
	}

	if !reflect.DeepEqual(c.Spec.SubscriptionID, old.Spec.SubscriptionID) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "SubscriptionID"),
				c.Spec.SubscriptionID, "field is immutable"),
		)
	}

	if !reflect.DeepEqual(c.Spec.Location, old.Spec.Location) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("spec", "Location"),
				c.Spec.Location, "field is immutable"),
		)
	}

	if len(allErrs) == 0 {
		return c.validateCluster(old)
	}

	return apierrors.NewInvalid(GroupVersion.WithKind("AzureCluster").GroupKind(), c.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (c *AzureCluster) ValidateDelete() error {
	clusterlog.Info("validate delete", "name", c.Name)

	return nil
}

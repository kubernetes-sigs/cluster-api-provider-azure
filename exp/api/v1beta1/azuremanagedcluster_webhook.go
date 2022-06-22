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
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/feature"
	"sigs.k8s.io/cluster-api-provider-azure/util/maps"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (r *AzureManagedCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-azuremanagedcluster,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedclusters,versions=v1beta1,name=validation.azuremanagedclusters.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Validator = &AzureManagedCluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *AzureManagedCluster) ValidateCreate() error {
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *AzureManagedCluster) ValidateUpdate(oldRaw runtime.Object) error {
	// NOTE: AzureManagedCluster is behind AKS feature gate flag; the web hook
	// must prevent creating new objects new case the feature flag is disabled.
	if !feature.Gates.Enabled(feature.AKS) {
		return field.Forbidden(
			field.NewPath("spec"),
			"can be set only if the AKS feature flag is enabled",
		)
	}

	old := oldRaw.(*AzureManagedCluster)
	var allErrs field.ErrorList

	// custom headers are immutable
	oldCustomHeaders := maps.FilterByKeyPrefix(old.ObjectMeta.Annotations, azure.CustomHeaderPrefix)
	newCustomHeaders := maps.FilterByKeyPrefix(r.ObjectMeta.Annotations, azure.CustomHeaderPrefix)
	if !reflect.DeepEqual(oldCustomHeaders, newCustomHeaders) {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("metadata", "annotations"),
				r.ObjectMeta.Annotations,
				fmt.Sprintf("annotations with '%s' prefix are immutable", azure.CustomHeaderPrefix)))
	}

	if len(allErrs) != 0 {
		return apierrors.NewInvalid(GroupVersion.WithKind("AzureManagedCluster").GroupKind(), r.Name, allErrs)
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *AzureManagedCluster) ValidateDelete() error {
	return nil
}

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
	"fmt"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var _ = logf.Log.WithName("azurecluster-resource")

func (r *AzureCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1alpha3-azurecluster,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,groups=infrastructure.cluster.x-k8s.io,resources=azurecluster,versions=v1alpha3,name=validation.azurecluster.infrastructure.cluster.x-k8s.io

var _ webhook.Validator = &AzureCluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *AzureCluster) ValidateCreate() error {
	machinelog.Info("validate create", "name", r.Name)

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *AzureCluster) ValidateUpdate(old runtime.Object) error {
	machinelog.Info("validate update", "name", r.Name)
	azureCluster, ok := old.(*AzureCluster)
	if !ok {
		return fmt.Errorf("update object is not a AzureCluster type")
	}

	if azureCluster.Spec.Location != r.Spec.Location {
		allErrs := field.ErrorList{}
		allErrs = append(allErrs, field.Invalid(field.NewPath("location"), r.Spec.Location, "AzureCluster Location is not mutable"))
		return apierrors.NewInvalid(
			GroupVersion.WithKind("AzureCluster").GroupKind(),
			r.Spec.Location, allErrs)
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *AzureCluster) ValidateDelete() error {
	machinelog.Info("validate delete", "name", r.Name)

	return nil
}

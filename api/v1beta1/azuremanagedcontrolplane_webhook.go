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
	"net"
	"reflect"
	"regexp"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var kubeSemver = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)([-0-9a-zA-Z_\.+]*)?$`)

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (m *AzureManagedControlPlane) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-azuremanagedcontrolplane,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedcontrolplanes,verbs=create;update,versions=v1beta1,name=default.azuremanagedcontrolplanes.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (m *AzureManagedControlPlane) Default(_ client.Client) {
	m.setDefaultSku()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-azuremanagedcontrolplane,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedcontrolplanes,versions=v1beta1,name=validation.azuremanagedcontrolplanes.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (m *AzureManagedControlPlane) ValidateCreate(client client.Client) error {
	return m.Validate(client)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (m *AzureManagedControlPlane) ValidateUpdate(oldRaw runtime.Object, client client.Client) error {
	var allErrs field.ErrorList
	old := oldRaw.(*AzureManagedControlPlane)

	if errs := m.validateAPIServerAccessProfileUpdate(old); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	if len(allErrs) == 0 {
		return m.Validate(client)
	}

	return apierrors.NewInvalid(GroupVersion.WithKind("AzureManagedControlPlane").GroupKind(), m.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (m *AzureManagedControlPlane) ValidateDelete(_ client.Client) error {
	return nil
}

// Validate the Azure Machine Pool and return an aggregate error.
func (m *AzureManagedControlPlane) Validate(cli client.Client) error {
	validators := []func(client client.Client) error{
		m.validateAPIServerAccessProfile,
	}

	var errs []error
	for _, validator := range validators {
		if err := validator(cli); err != nil {
			errs = append(errs, err)
		}
	}

	return kerrors.NewAggregate(errs)
}

// validateAPIServerAccessProfile validates an APIServerAccessProfile.
func (m *AzureManagedControlPlane) validateAPIServerAccessProfile(_ client.Client) error {
	if m.Spec.APIServerAccessProfile != nil {
		var allErrs field.ErrorList
		for _, ipRange := range m.Spec.APIServerAccessProfile.AuthorizedIPRanges {
			if _, _, err := net.ParseCIDR(ipRange); err != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath("Spec", "APIServerAccessProfile", "AuthorizedIPRanges"), ipRange, "invalid CIDR format"))
			}
		}
		if len(allErrs) > 0 {
			return kerrors.NewAggregate(allErrs.ToAggregate().Errors())
		}
	}
	return nil
}

// validateAPIServerAccessProfileUpdate validates update to APIServerAccessProfile.
func (m *AzureManagedControlPlane) validateAPIServerAccessProfileUpdate(old *AzureManagedControlPlane) field.ErrorList {
	var allErrs field.ErrorList

	newAPIServerAccessProfileNormalized := &APIServerAccessProfile{}
	oldAPIServerAccessProfileNormalized := &APIServerAccessProfile{}
	if m.Spec.APIServerAccessProfile != nil {
		newAPIServerAccessProfileNormalized = &APIServerAccessProfile{
			EnablePrivateCluster:           m.Spec.APIServerAccessProfile.EnablePrivateCluster,
			PrivateDNSZone:                 m.Spec.APIServerAccessProfile.PrivateDNSZone,
			EnablePrivateClusterPublicFQDN: m.Spec.APIServerAccessProfile.EnablePrivateClusterPublicFQDN,
		}
	}
	if old.Spec.APIServerAccessProfile != nil {
		oldAPIServerAccessProfileNormalized = &APIServerAccessProfile{
			EnablePrivateCluster:           old.Spec.APIServerAccessProfile.EnablePrivateCluster,
			PrivateDNSZone:                 old.Spec.APIServerAccessProfile.PrivateDNSZone,
			EnablePrivateClusterPublicFQDN: old.Spec.APIServerAccessProfile.EnablePrivateClusterPublicFQDN,
		}
	}

	if !reflect.DeepEqual(newAPIServerAccessProfileNormalized, oldAPIServerAccessProfileNormalized) {
		allErrs = append(allErrs,
			field.Invalid(field.NewPath("Spec", "APIServerAccessProfile"),
				m.Spec.APIServerAccessProfile, "fields (except for AuthorizedIPRanges) are immutable"),
		)
	}

	return allErrs
}

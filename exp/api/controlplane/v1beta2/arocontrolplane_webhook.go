/*
Copyright 2025 The Kubernetes Authors.

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

package v1beta2

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// SetupAROControlPlaneWebhookWithManager sets up and registers the webhook with the manager.
func SetupAROControlPlaneWebhookWithManager(mgr ctrl.Manager) error {
	mw := &aroControlPlaneWebhook{Client: mgr.GetClient()}
	return ctrl.NewWebhookManagedBy(mgr).
		For(&AROControlPlane{}).
		WithDefaulter(mw).
		WithValidator(mw).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-controlplane-cluster-x-k8s-io-v1beta2-arocontrolplane,mutating=true,failurePolicy=fail,groups=controlplane.cluster.x-k8s.io,resources=arocontrolplanes,verbs=create;update,versions=v1beta2,name=default.arocontrolplanes.controlplane.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta2

// aroControlPlaneWebhook implements a validating and defaulting webhook for AROControlPlane.
type aroControlPlaneWebhook struct {
	Client client.Client
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (mw *aroControlPlaneWebhook) Default(_ context.Context, obj runtime.Object) error {
	_, ok := obj.(*AROControlPlane)
	if !ok {
		return apierrors.NewBadRequest("expected an AROControlPlane")
	}

	// No defaults to set in resources-only mode
	return nil
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-controlplane-cluster-x-k8s-io-v1beta2-arocontrolplane,mutating=false,failurePolicy=fail,groups=controlplane.cluster.x-k8s.io,resources=arocontrolplanes,versions=v1beta2,name=validation.arocontrolplanes.controlplane.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta2

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (mw *aroControlPlaneWebhook) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	m, ok := obj.(*AROControlPlane)
	if !ok {
		return nil, apierrors.NewBadRequest("expected an AROControlPlane")
	}

	return nil, m.Validate(mw.Client)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (mw *aroControlPlaneWebhook) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (admission.Warnings, error) {
	m, ok := newObj.(*AROControlPlane)
	if !ok {
		return nil, apierrors.NewBadRequest("expected an AROControlPlane")
	}

	// ASO handles immutability for resources-mode fields
	// Only validate the current state
	return nil, m.Validate(mw.Client)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (mw *aroControlPlaneWebhook) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// Validate the Azure Managed Control Plane and return an aggregate error.
func (m *AROControlPlane) Validate(cli client.Client) error {
	// Validate that resources mode is used
	if len(m.Spec.Resources) == 0 {
		allErrs := field.ErrorList{
			field.Required(
				field.NewPath("spec", "resources"),
				"resources mode is required; field-based configuration is no longer supported"),
		}
		return allErrs.ToAggregate()
	}

	// Validate identity, resources, and name
	var allErrs field.ErrorList
	validators := []func(client client.Client) field.ErrorList{
		m.validateIdentity,
		m.validateResources,
	}
	for _, validator := range validators {
		if err := validator(cli); err != nil {
			allErrs = append(allErrs, err...)
		}
	}

	allErrs = append(allErrs, validateName(m.Name, field.NewPath("name"))...)

	return allErrs.ToAggregate()
}

func validateName(name string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	if lName := strings.ToLower(name); strings.Contains(lName, "microsoft") ||
		strings.Contains(lName, "windows") {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("Name"), name,
			"cluster name is invalid because 'MICROSOFT' and 'WINDOWS' can't be used as either a whole word or a substring in the name"))
	}

	return allErrs
}

// validateIdentity validates an Identity.
func (m *AROControlPlane) validateIdentity(_ client.Client) field.ErrorList {
	var allErrs field.ErrorList

	if m.Spec.IdentityRef != nil {
		if m.Spec.IdentityRef.Name == "" {
			allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "identityRef", "name"), m.Spec.IdentityRef.Name, "cannot be empty"))
		}
	}

	if len(allErrs) > 0 {
		return allErrs
	}

	return nil
}

// validateResources validates the resources field configuration.
func (m *AROControlPlane) validateResources(_ client.Client) field.ErrorList {
	var allErrs field.ErrorList

	if len(m.Spec.Resources) == 0 {
		return allErrs // Resources is optional
	}

	basePath := field.NewPath("spec", "resources")

	// When using resources mode, deprecated fields are ignored
	// The controller will ignore these fields when Resources is set

	// Validate that each resource can be unmarshaled
	for i := range m.Spec.Resources {
		resourcePath := basePath.Index(i)
		raw := &m.Spec.Resources[i]

		if raw.Raw == nil {
			allErrs = append(allErrs, field.Required(resourcePath, "resource cannot be empty"))
			continue
		}

		// Basic validation: check that it's valid JSON
		var obj map[string]interface{}
		if err := json.Unmarshal(raw.Raw, &obj); err != nil {
			allErrs = append(allErrs, field.Invalid(resourcePath, string(raw.Raw), fmt.Sprintf("resource must be valid JSON: %v", err)))
			continue
		}

		// Validate that required fields exist
		apiVersion, ok := obj["apiVersion"].(string)
		if !ok || apiVersion == "" {
			allErrs = append(allErrs, field.Required(resourcePath.Child("apiVersion"), "resource must have apiVersion"))
		}

		kind, ok := obj["kind"].(string)
		if !ok || kind == "" {
			allErrs = append(allErrs, field.Required(resourcePath.Child("kind"), "resource must have kind"))
		}

		// Check if metadata exists
		metadata, ok := obj["metadata"].(map[string]interface{})
		if !ok {
			allErrs = append(allErrs, field.Required(resourcePath.Child("metadata"), "resource must have metadata"))
			continue
		}

		// Validate name exists in metadata
		name, ok := metadata["name"].(string)
		if !ok || name == "" {
			allErrs = append(allErrs, field.Required(resourcePath.Child("metadata", "name"), "resource must have metadata.name"))
		}
	}

	return allErrs
}

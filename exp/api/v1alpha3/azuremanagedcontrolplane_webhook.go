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
	"net"
	"regexp"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var azuremanagedcontrolplanelog = logf.Log.WithName("azuremanagedcontrolplane-resource")

var kubeSemver = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)([-0-9a-zA-Z_\.+]*)?$`)

func (r *AzureManagedControlPlane) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-exp-infrastructure-cluster-x-k8s-io-v1alpha3-azuremanagedcontrolplane,mutating=true,failurePolicy=fail,groups=exp.infrastructure.cluster.x-k8s.io,resources=azuremanagedcontrolplanes,verbs=create;update,versions=v1alpha3,name=azuremanagedcontrolplane.kb.io

var _ webhook.Defaulter = &AzureManagedControlPlane{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *AzureManagedControlPlane) Default() {
	azuremanagedcontrolplanelog.Info("default", "name", r.Name)

	if r.Spec.NetworkPlugin == nil {
		networkPlugin := "azure"
		r.Spec.NetworkPlugin = &networkPlugin
	}
	if r.Spec.LoadBalancerSKU == nil {
		loadBalancerSKU := "Standard"
		r.Spec.LoadBalancerSKU = &loadBalancerSKU
	}
	if r.Spec.NetworkPolicy == nil {
		NetworkPolicy := "calico"
		r.Spec.NetworkPolicy = &NetworkPolicy
	}

	if r.Spec.Version != "" && !strings.HasPrefix(r.Spec.Version, "v") {
		normalizedVersion := "v" + r.Spec.Version
		r.Spec.Version = normalizedVersion
	}
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-exp-infrastructure-cluster-x-k8s-io-v1alpha3-azuremanagedcontrolplane,mutating=false,failurePolicy=fail,groups=exp.infrastructure.cluster.x-k8s.io,resources=azuremanagedcontrolplanes,versions=v1alpha3,name=azuremanagedcontrolplane.kb.io

var _ webhook.Validator = &AzureManagedControlPlane{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *AzureManagedControlPlane) ValidateCreate() error {
	azuremanagedcontrolplanelog.Info("validate create", "name", r.Name)
	return r.Validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *AzureManagedControlPlane) ValidateUpdate(old runtime.Object) error {
	azuremanagedcontrolplanelog.Info("validate update", "name", r.Name)

	return r.Validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *AzureManagedControlPlane) ValidateDelete() error {
	azuremanagedcontrolplanelog.Info("validate delete", "name", r.Name)

	return nil
}

// Validate the Azure Managed ControlPlane and return an aggregate error
func (r *AzureManagedControlPlane) Validate() error {
	var allErrs field.ErrorList
	err := r.validateDNSServiceIP()
	if err != nil {
		allErrs = append(allErrs, err)
	}

	if !kubeSemver.MatchString(r.Spec.Version) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "version"), r.Spec.Version, "must be a valid semantic version"))
	}

	if len(allErrs) > 0 {
		return apierrors.NewInvalid(GroupVersion.WithKind("AzureManagedControlPlane").GroupKind(), r.Name, allErrs)
	}

	return nil
}

// validate DNSServiceIP
func (r *AzureManagedControlPlane) validateDNSServiceIP() *field.Error {
	if r.Spec.DNSServiceIP != nil {
		if net.ParseIP(*r.Spec.DNSServiceIP) == nil {
			return field.Invalid(field.NewPath("spec", "DNSServiceIO"), *r.Spec.DNSServiceIP, "DNSServiceIP must be a valid IP")
		}
	}
	return nil
}

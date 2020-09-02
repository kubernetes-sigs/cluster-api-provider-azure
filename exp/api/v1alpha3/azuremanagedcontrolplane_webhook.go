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
	"errors"
	"net"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// log is for logging in this package.
var azuremanagedcontrolplanelog = logf.Log.WithName("azuremanagedcontrolplane-resource")

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
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-exp-infrastructure-cluster-x-k8s-io-v1alpha3-azuremanagedcontrolplane,mutating=false,failurePolicy=fail,groups=exp.infrastructure.cluster.x-k8s.io,resources=azuremanagedcontrolplanes,versions=v1alpha3,name=azuremanagedcontrolplane.kb.io

var _ webhook.Validator = &AzureManagedControlPlane{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *AzureManagedControlPlane) ValidateCreate() error {
	azuremanagedcontrolplanelog.Info("validate create", "name", r.Name)

	err := r.validateDNSServiceIP()
	if err != nil {
		return err
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *AzureManagedControlPlane) ValidateUpdate(old runtime.Object) error {
	azuremanagedcontrolplanelog.Info("validate update", "name", r.Name)

	err := r.validateDNSServiceIP()
	if err != nil {
		return err
	}
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *AzureManagedControlPlane) ValidateDelete() error {
	azuremanagedcontrolplanelog.Info("validate delete", "name", r.Name)

	return nil
}

// validate DNSServiceIP
func (r *AzureManagedControlPlane) validateDNSServiceIP() error {
	if r.Spec.DNSServiceIP != nil {
		if net.ParseIP(*r.Spec.DNSServiceIP) == nil {
			return errors.New("DNSServiceIP must be a valid IP")
		}
	}
	return nil
}

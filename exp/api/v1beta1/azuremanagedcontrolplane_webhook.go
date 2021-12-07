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

package v1beta1

import (
	"errors"
	"net"
	"reflect"
	"regexp"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var kubeSemver = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)([-0-9a-zA-Z_\.+]*)?$`)

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (r *AzureManagedControlPlane) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-azuremanagedcontrolplane,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedcontrolplanes,verbs=create;update,versions=v1beta1,name=default.azuremanagedcontrolplanes.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Defaulter = &AzureManagedControlPlane{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *AzureManagedControlPlane) Default() {
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

	err := r.setDefaultSSHPublicKey()
	if err != nil {
		ctrl.Log.WithName("AzureManagedControlPlaneWebHookLogger").Error(err, "SetDefaultSshPublicKey failed")
	}

	r.setDefaultNodeResourceGroupName()
	r.setDefaultVirtualNetwork()
	r.setDefaultSubnet()
	r.setDefaultSku()
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-azuremanagedcontrolplane,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedcontrolplanes,versions=v1beta1,name=validation.azuremanagedcontrolplanes.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Validator = &AzureManagedControlPlane{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *AzureManagedControlPlane) ValidateCreate() error {
	return r.Validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *AzureManagedControlPlane) ValidateUpdate(oldRaw runtime.Object) error {
	var allErrs field.ErrorList
	old := oldRaw.(*AzureManagedControlPlane)

	if r.Spec.SubscriptionID != old.Spec.SubscriptionID {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "SubscriptionID"),
				r.Spec.SubscriptionID,
				"field is immutable"))
	}

	if r.Spec.ResourceGroupName != old.Spec.ResourceGroupName {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "ResourceGroupName"),
				r.Spec.ResourceGroupName,
				"field is immutable"))
	}

	if r.Spec.NodeResourceGroupName != old.Spec.NodeResourceGroupName {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "NodeResourceGroupName"),
				r.Spec.NodeResourceGroupName,
				"field is immutable"))
	}

	if r.Spec.Location != old.Spec.Location {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "Location"),
				r.Spec.Location,
				"field is immutable"))
	}

	if old.Spec.SSHPublicKey != "" {
		// Prevent SSH key modification if it was already set to some value
		if r.Spec.SSHPublicKey != old.Spec.SSHPublicKey {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "SSHPublicKey"),
					r.Spec.SSHPublicKey,
					"field is immutable"))
		}
	}

	if old.Spec.DNSServiceIP != nil {
		// Prevent DNSServiceIP modification if it was already set to some value
		if r.Spec.DNSServiceIP == nil {
			// unsetting the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "DNSServiceIP"),
					r.Spec.DNSServiceIP,
					"field is immutable, unsetting is not allowed"))
		} else if *r.Spec.DNSServiceIP != *old.Spec.DNSServiceIP {
			// changing the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "DNSServiceIP"),
					*r.Spec.DNSServiceIP,
					"field is immutable"))
		}
	}

	if old.Spec.NetworkPlugin != nil {
		// Prevent NetworkPlugin modification if it was already set to some value
		if r.Spec.NetworkPlugin == nil {
			// unsetting the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "NetworkPlugin"),
					r.Spec.NetworkPlugin,
					"field is immutable, unsetting is not allowed"))
		} else if *r.Spec.NetworkPlugin != *old.Spec.NetworkPlugin {
			// changing the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "NetworkPlugin"),
					*r.Spec.NetworkPlugin,
					"field is immutable"))
		}
	}

	if old.Spec.NetworkPolicy != nil {
		// Prevent NetworkPolicy modification if it was already set to some value
		if r.Spec.NetworkPolicy == nil {
			// unsetting the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "NetworkPolicy"),
					r.Spec.NetworkPolicy,
					"field is immutable, unsetting is not allowed"))
		} else if *r.Spec.NetworkPolicy != *old.Spec.NetworkPolicy {
			// changing the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "NetworkPolicy"),
					*r.Spec.NetworkPolicy,
					"field is immutable"))
		}
	}

	if old.Spec.LoadBalancerSKU != nil {
		// Prevent LoadBalancerSKU modification if it was already set to some value
		if r.Spec.LoadBalancerSKU == nil {
			// unsetting the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "LoadBalancerSKU"),
					r.Spec.LoadBalancerSKU,
					"field is immutable, unsetting is not allowed"))
		} else if *r.Spec.LoadBalancerSKU != *old.Spec.LoadBalancerSKU {
			// changing the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "LoadBalancerSKU"),
					*r.Spec.LoadBalancerSKU,
					"field is immutable"))
		}
	}

	if old.Spec.AADProfile != nil {
		if r.Spec.AADProfile == nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "AADProfile"),
					r.Spec.AADProfile,
					"field cannot be nil, cannot disable AADProfile"))
		} else {
			if !r.Spec.AADProfile.Managed && old.Spec.AADProfile.Managed {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("Spec", "AADProfile.Managed"),
						r.Spec.AADProfile.Managed,
						"cannot set AADProfile.Managed to false"))
			}
			if len(r.Spec.AADProfile.AdminGroupObjectIDs) == 0 {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("Spec", "AADProfile.AdminGroupObjectIDs"),
						r.Spec.AADProfile.AdminGroupObjectIDs,
						"length of AADProfile.AdminGroupObjectIDs cannot be zero"))
			}
		}
	}

	if errs := r.validateAPIServerAccessProfileUpdate(old); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	if len(allErrs) == 0 {
		return r.Validate()
	}

	return apierrors.NewInvalid(GroupVersion.WithKind("AzureManagedControlPlane").GroupKind(), r.Name, allErrs)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *AzureManagedControlPlane) ValidateDelete() error {
	return nil
}

// Validate the Azure Machine Pool and return an aggregate error.
func (r *AzureManagedControlPlane) Validate() error {
	validators := []func() error{
		r.validateVersion,
		r.validateDNSServiceIP,
		r.validateSSHKey,
		r.validateLoadBalancerProfile,
		r.validateAPIServerAccessProfile,
	}

	var errs []error
	for _, validator := range validators {
		if err := validator(); err != nil {
			errs = append(errs, err)
		}
	}

	return kerrors.NewAggregate(errs)
}

// validate DNSServiceIP.
func (r *AzureManagedControlPlane) validateDNSServiceIP() error {
	if r.Spec.DNSServiceIP != nil {
		if net.ParseIP(*r.Spec.DNSServiceIP) == nil {
			return errors.New("DNSServiceIP must be a valid IP")
		}
	}

	return nil
}

func (r *AzureManagedControlPlane) validateVersion() error {
	if !kubeSemver.MatchString(r.Spec.Version) {
		return errors.New("must be a valid semantic version")
	}

	return nil
}

// ValidateSSHKey validates an SSHKey.
func (r *AzureManagedControlPlane) validateSSHKey() error {
	if r.Spec.SSHPublicKey != "" {
		sshKey := r.Spec.SSHPublicKey
		if errs := infrav1.ValidateSSHKey(sshKey, field.NewPath("sshKey")); len(errs) > 0 {
			return kerrors.NewAggregate(errs.ToAggregate().Errors())
		}
	}

	return nil
}

// ValidateLoadBalancerProfile validates a LoadBalancerProfile.
func (r *AzureManagedControlPlane) validateLoadBalancerProfile() error {
	if r.Spec.LoadBalancerProfile != nil {
		var errs []error
		var allErrs field.ErrorList
		numOutboundIPTypes := 0

		if r.Spec.LoadBalancerProfile.ManagedOutboundIPs != nil {
			if *r.Spec.LoadBalancerProfile.ManagedOutboundIPs < 1 || *r.Spec.LoadBalancerProfile.ManagedOutboundIPs > 100 {
				allErrs = append(allErrs, field.Invalid(field.NewPath("Spec", "LoadBalancerProfile", "ManagedOutboundIPs"), *r.Spec.LoadBalancerProfile.ManagedOutboundIPs, "value should be in between 1 and 100"))
			}
		}

		if r.Spec.LoadBalancerProfile.AllocatedOutboundPorts != nil {
			if *r.Spec.LoadBalancerProfile.AllocatedOutboundPorts < 0 || *r.Spec.LoadBalancerProfile.AllocatedOutboundPorts > 64000 {
				allErrs = append(allErrs, field.Invalid(field.NewPath("Spec", "LoadBalancerProfile", "AllocatedOutboundPorts"), *r.Spec.LoadBalancerProfile.AllocatedOutboundPorts, "value should be in between 0 and 64000"))
			}
		}

		if r.Spec.LoadBalancerProfile.IdleTimeoutInMinutes != nil {
			if *r.Spec.LoadBalancerProfile.IdleTimeoutInMinutes < 4 || *r.Spec.LoadBalancerProfile.IdleTimeoutInMinutes > 120 {
				allErrs = append(allErrs, field.Invalid(field.NewPath("Spec", "LoadBalancerProfile", "IdleTimeoutInMinutes"), *r.Spec.LoadBalancerProfile.IdleTimeoutInMinutes, "value should be in between 4 and 120"))
			}
		}

		if r.Spec.LoadBalancerProfile.ManagedOutboundIPs != nil {
			numOutboundIPTypes++
		}
		if len(r.Spec.LoadBalancerProfile.OutboundIPPrefixes) > 0 {
			numOutboundIPTypes++
		}
		if len(r.Spec.LoadBalancerProfile.OutboundIPs) > 0 {
			numOutboundIPTypes++
		}
		if numOutboundIPTypes > 1 {
			errs = append(errs, errors.New("Load balancer profile must specify at most one of ManagedOutboundIPs, OutboundIPPrefixes and OutboundIPs"))
		}

		if len(allErrs) > 0 {
			agg := kerrors.NewAggregate(allErrs.ToAggregate().Errors())
			errs = append(errs, agg)
		}

		return kerrors.NewAggregate(errs)
	}

	return nil
}

// validateAPIServerAccessProfile validates an APIServerAccessProfile.
func (r *AzureManagedControlPlane) validateAPIServerAccessProfile() error {
	if r.Spec.APIServerAccessProfile != nil {
		var allErrs field.ErrorList
		for _, ipRange := range r.Spec.APIServerAccessProfile.AuthorizedIPRanges {
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
func (r *AzureManagedControlPlane) validateAPIServerAccessProfileUpdate(old *AzureManagedControlPlane) field.ErrorList {
	var allErrs field.ErrorList

	newAPIServerAccessProfileNormalized := &APIServerAccessProfile{}
	oldAPIServerAccessProfileNormalized := &APIServerAccessProfile{}
	if r.Spec.APIServerAccessProfile != nil {
		newAPIServerAccessProfileNormalized = &APIServerAccessProfile{
			EnablePrivateCluster:           r.Spec.APIServerAccessProfile.EnablePrivateCluster,
			PrivateDNSZone:                 r.Spec.APIServerAccessProfile.PrivateDNSZone,
			EnablePrivateClusterPublicFQDN: r.Spec.APIServerAccessProfile.EnablePrivateClusterPublicFQDN,
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
				r.Spec.APIServerAccessProfile, "fields (except for AuthorizedIPRanges) are immutable"),
		)
	}

	return allErrs
}

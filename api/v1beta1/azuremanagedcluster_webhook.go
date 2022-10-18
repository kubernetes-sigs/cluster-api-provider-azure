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
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/cluster-api-provider-azure/feature"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/maps"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (m *AzureManagedCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta1-azuremanagedcontrolplane,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedcontrolplanes,verbs=create;update,versions=v1beta1,name=default.azuremanagedcontrolplanes.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (m *AzureManagedCluster) Default(_ client.Client) {
	if m.Spec.NetworkPlugin == nil {
		networkPlugin := "azure"
		m.Spec.NetworkPlugin = &networkPlugin
	}
	if m.Spec.LoadBalancerSKU == nil {
		loadBalancerSKU := "Standard"
		m.Spec.LoadBalancerSKU = &loadBalancerSKU
	}
	if m.Spec.NetworkPolicy == nil {
		NetworkPolicy := "calico"
		m.Spec.NetworkPolicy = &NetworkPolicy
	}

	if m.Spec.Version != "" && !strings.HasPrefix(m.Spec.Version, "v") {
		normalizedVersion := "v" + m.Spec.Version
		m.Spec.Version = normalizedVersion
	}

	if err := m.setDefaultSSHPublicKey(); err != nil {
		ctrl.Log.WithName("AzureManagedClusterWebHookLogger").Error(err, "setDefaultSSHPublicKey failed")
	}

	m.setDefaultNodeResourceGroupName()
	m.setDefaultVirtualNetwork()
	m.setDefaultSubnet()
}

// +kubebuilder:webhook:verbs=update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-azuremanagedcluster,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=azuremanagedclusters,versions=v1beta1,name=validation.azuremanagedclusters.infrastructure.cluster.x-k8s.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

var _ webhook.Validator = &AzureManagedCluster{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (m *AzureManagedCluster) ValidateCreate() error {
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (m *AzureManagedCluster) ValidateUpdate(oldRaw runtime.Object) error {
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
	oldCustomHeaders := maps.FilterByKeyPrefix(old.ObjectMeta.Annotations, azureutil.CustomHeaderPrefix)
	newCustomHeaders := maps.FilterByKeyPrefix(m.ObjectMeta.Annotations, azureutil.CustomHeaderPrefix)
	if !reflect.DeepEqual(oldCustomHeaders, newCustomHeaders) {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("metadata", "annotations"),
				m.ObjectMeta.Annotations,
				fmt.Sprintf("annotations with '%s' prefix are immutable", azureutil.CustomHeaderPrefix)))
	}

	if m.Name != old.Name {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Name"),
				m.Name,
				"field is immutable"))
	}

	if m.Spec.SubscriptionID != old.Spec.SubscriptionID {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "SubscriptionID"),
				m.Spec.SubscriptionID,
				"field is immutable"))
	}

	if m.Spec.ResourceGroupName != old.Spec.ResourceGroupName {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "ResourceGroupName"),
				m.Spec.ResourceGroupName,
				"field is immutable"))
	}

	if m.Spec.NodeResourceGroupName != old.Spec.NodeResourceGroupName {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "NodeResourceGroupName"),
				m.Spec.NodeResourceGroupName,
				"field is immutable"))
	}

	if m.Spec.Location != old.Spec.Location {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "Location"),
				m.Spec.Location,
				"field is immutable"))
	}

	if old.Spec.SSHPublicKey != "" {
		// Prevent SSH key modification if it was already set to some value
		if m.Spec.SSHPublicKey != old.Spec.SSHPublicKey {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "SSHPublicKey"),
					m.Spec.SSHPublicKey,
					"field is immutable"))
		}
	}

	if old.Spec.DNSServiceIP != nil {
		// Prevent DNSServiceIP modification if it was already set to some value
		if m.Spec.DNSServiceIP == nil {
			// unsetting the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "DNSServiceIP"),
					m.Spec.DNSServiceIP,
					"field is immutable, unsetting is not allowed"))
		} else if *m.Spec.DNSServiceIP != *old.Spec.DNSServiceIP {
			// changing the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "DNSServiceIP"),
					*m.Spec.DNSServiceIP,
					"field is immutable"))
		}
	}

	if old.Spec.NetworkPlugin != nil {
		// Prevent NetworkPlugin modification if it was already set to some value
		if m.Spec.NetworkPlugin == nil {
			// unsetting the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "NetworkPlugin"),
					m.Spec.NetworkPlugin,
					"field is immutable, unsetting is not allowed"))
		} else if *m.Spec.NetworkPlugin != *old.Spec.NetworkPlugin {
			// changing the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "NetworkPlugin"),
					*m.Spec.NetworkPlugin,
					"field is immutable"))
		}
	}

	if old.Spec.NetworkPolicy != nil {
		// Prevent NetworkPolicy modification if it was already set to some value
		if m.Spec.NetworkPolicy == nil {
			// unsetting the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "NetworkPolicy"),
					m.Spec.NetworkPolicy,
					"field is immutable, unsetting is not allowed"))
		} else if *m.Spec.NetworkPolicy != *old.Spec.NetworkPolicy {
			// changing the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "NetworkPolicy"),
					*m.Spec.NetworkPolicy,
					"field is immutable"))
		}
	}

	if old.Spec.LoadBalancerSKU != nil {
		// Prevent LoadBalancerSKU modification if it was already set to some value
		if m.Spec.LoadBalancerSKU == nil {
			// unsetting the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "LoadBalancerSKU"),
					m.Spec.LoadBalancerSKU,
					"field is immutable, unsetting is not allowed"))
		} else if *m.Spec.LoadBalancerSKU != *old.Spec.LoadBalancerSKU {
			// changing the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "LoadBalancerSKU"),
					*m.Spec.LoadBalancerSKU,
					"field is immutable"))
		}
	}

	if old.Spec.AADProfile != nil {
		if m.Spec.AADProfile == nil {
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "AADProfile"),
					m.Spec.AADProfile,
					"field cannot be nil, cannot disable AADProfile"))
		} else {
			if !m.Spec.AADProfile.Managed && old.Spec.AADProfile.Managed {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("Spec", "AADProfile.Managed"),
						m.Spec.AADProfile.Managed,
						"cannot set AADProfile.Managed to false"))
			}
			if len(m.Spec.AADProfile.AdminGroupObjectIDs) == 0 {
				allErrs = append(allErrs,
					field.Invalid(
						field.NewPath("Spec", "AADProfile.AdminGroupObjectIDs"),
						m.Spec.AADProfile.AdminGroupObjectIDs,
						"length of AADProfile.AdminGroupObjectIDs cannot be zero"))
			}
		}
	}

	if errs := m.validateVirtualNetworkUpdate(old); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	if len(allErrs) != 0 {
		return apierrors.NewInvalid(GroupVersion.WithKind("AzureManagedCluster").GroupKind(), m.Name, allErrs)
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (m *AzureManagedCluster) ValidateDelete() error {
	return nil
}

// validateVirtualNetworkUpdate validates update to APIServerAccessProfile.
func (m *AzureManagedCluster) validateVirtualNetworkUpdate(old *AzureManagedCluster) field.ErrorList {
	var allErrs field.ErrorList
	if old.Spec.VirtualNetwork.Name != m.Spec.VirtualNetwork.Name {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "VirtualNetwork.Name"),
				m.Spec.VirtualNetwork.Name,
				"Virtual Network Name is immutable"))
	}

	if old.Spec.VirtualNetwork.CIDRBlock != m.Spec.VirtualNetwork.CIDRBlock {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "VirtualNetwork.CIDRBlock"),
				m.Spec.VirtualNetwork.CIDRBlock,
				"Virtual Network CIDRBlock is immutable"))
	}

	if old.Spec.VirtualNetwork.Subnet.Name != m.Spec.VirtualNetwork.Subnet.Name {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "VirtualNetwork.Subnet.Name"),
				m.Spec.VirtualNetwork.Subnet.Name,
				"Subnet Name is immutable"))
	}

	if old.Spec.VirtualNetwork.Subnet.CIDRBlock != m.Spec.VirtualNetwork.Subnet.CIDRBlock {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "VirtualNetwork.Subnet.CIDRBlock"),
				m.Spec.VirtualNetwork.Subnet.CIDRBlock,
				"Subnet CIDRBlock is immutable"))
	}

	if old.Spec.VirtualNetwork.ResourceGroup != m.Spec.VirtualNetwork.ResourceGroup {
		allErrs = append(allErrs,
			field.Invalid(
				field.NewPath("Spec", "VirtualNetwork.ResourceGroup"),
				m.Spec.VirtualNetwork.ResourceGroup,
				"Virtual Network Resource Group is immutable"))
	}
	return allErrs
}

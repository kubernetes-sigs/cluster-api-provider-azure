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
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"regexp"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
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
		ctrl.Log.WithName("AzureManagedControlPlaneWebHookLogger").Error(err, "setDefaultSSHPublicKey failed")
	}

	m.setDefaultNodeResourceGroupName()
	m.setDefaultVirtualNetwork()
	m.setDefaultSubnet()
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

	if old.Spec.DisableLocalAccounts != nil {
		// Prevent DisableLocalAccounts modification if it was already set to some value
		if m.Spec.DisableLocalAccounts == nil {
			// unsetting the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "DisableLocalAccounts"),
					m.Spec.DisableLocalAccounts,
					"field is immutable, unsetting is not allowed"))
		} else if *m.Spec.DisableLocalAccounts != *old.Spec.DisableLocalAccounts {
			// changing the field is not allowed
			allErrs = append(allErrs,
				field.Invalid(
					field.NewPath("Spec", "DisableLocalAccounts"),
					*m.Spec.DisableLocalAccounts,
					"field is immutable"))
		}
	}

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
		m.validateName,
		m.validateVersion,
		m.validateDNSServiceIP,
		m.validateSSHKey,
		m.validateLoadBalancerProfile,
		m.validateAPIServerAccessProfile,
		m.validateManagedClusterNetwork,
	}

	var errs []error
	for _, validator := range validators {
		if err := validator(cli); err != nil {
			errs = append(errs, err)
		}
	}

	return kerrors.NewAggregate(errs)
}

// validateDNSServiceIP validates the DNSServiceIP.
func (m *AzureManagedControlPlane) validateDNSServiceIP(_ client.Client) error {
	if m.Spec.DNSServiceIP != nil {
		if net.ParseIP(*m.Spec.DNSServiceIP) == nil {
			return errors.New("DNSServiceIP must be a valid IP")
		}
	}

	return nil
}

// validateVersion validates the Kubernetes version.
func (m *AzureManagedControlPlane) validateVersion(_ client.Client) error {
	if !kubeSemver.MatchString(m.Spec.Version) {
		return errors.New("must be a valid semantic version")
	}

	return nil
}

// validateSSHKey validates an SSHKey.
func (m *AzureManagedControlPlane) validateSSHKey(_ client.Client) error {
	if m.Spec.SSHPublicKey != "" {
		sshKey := m.Spec.SSHPublicKey
		if errs := infrav1.ValidateSSHKey(sshKey, field.NewPath("sshKey")); len(errs) > 0 {
			return kerrors.NewAggregate(errs.ToAggregate().Errors())
		}
	}

	return nil
}

// validateLoadBalancerProfile validates a LoadBalancerProfile.
func (m *AzureManagedControlPlane) validateLoadBalancerProfile(_ client.Client) error {
	if m.Spec.LoadBalancerProfile != nil {
		var errs []error
		var allErrs field.ErrorList
		numOutboundIPTypes := 0

		if m.Spec.LoadBalancerProfile.ManagedOutboundIPs != nil {
			if *m.Spec.LoadBalancerProfile.ManagedOutboundIPs < 1 || *m.Spec.LoadBalancerProfile.ManagedOutboundIPs > 100 {
				allErrs = append(allErrs, field.Invalid(field.NewPath("Spec", "LoadBalancerProfile", "ManagedOutboundIPs"), *m.Spec.LoadBalancerProfile.ManagedOutboundIPs, "value should be in between 1 and 100"))
			}
		}

		if m.Spec.LoadBalancerProfile.AllocatedOutboundPorts != nil {
			if *m.Spec.LoadBalancerProfile.AllocatedOutboundPorts < 0 || *m.Spec.LoadBalancerProfile.AllocatedOutboundPorts > 64000 {
				allErrs = append(allErrs, field.Invalid(field.NewPath("Spec", "LoadBalancerProfile", "AllocatedOutboundPorts"), *m.Spec.LoadBalancerProfile.AllocatedOutboundPorts, "value should be in between 0 and 64000"))
			}
		}

		if m.Spec.LoadBalancerProfile.IdleTimeoutInMinutes != nil {
			if *m.Spec.LoadBalancerProfile.IdleTimeoutInMinutes < 4 || *m.Spec.LoadBalancerProfile.IdleTimeoutInMinutes > 120 {
				allErrs = append(allErrs, field.Invalid(field.NewPath("Spec", "LoadBalancerProfile", "IdleTimeoutInMinutes"), *m.Spec.LoadBalancerProfile.IdleTimeoutInMinutes, "value should be in between 4 and 120"))
			}
		}

		if m.Spec.LoadBalancerProfile.ManagedOutboundIPs != nil {
			numOutboundIPTypes++
		}
		if len(m.Spec.LoadBalancerProfile.OutboundIPPrefixes) > 0 {
			numOutboundIPTypes++
		}
		if len(m.Spec.LoadBalancerProfile.OutboundIPs) > 0 {
			numOutboundIPTypes++
		}
		if numOutboundIPTypes > 1 {
			errs = append(errs, errors.New("load balancer profile must specify at most one of ManagedOutboundIPs, OutboundIPPrefixes and OutboundIPs"))
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

// validateManagedClusterNetwork validates the Cluster network values.
func (m *AzureManagedControlPlane) validateManagedClusterNetwork(cli client.Client) error {
	ctx := context.Background()

	// Fetch the Cluster.
	clusterName, ok := m.Labels[clusterv1.ClusterLabelName]
	if !ok {
		return nil
	}

	ownerCluster := &clusterv1.Cluster{}
	key := client.ObjectKey{
		Namespace: m.Namespace,
		Name:      clusterName,
	}

	if err := cli.Get(ctx, key, ownerCluster); err != nil {
		return err
	}

	var (
		allErrs     field.ErrorList
		serviceCIDR string
	)

	if clusterNetwork := ownerCluster.Spec.ClusterNetwork; clusterNetwork != nil {
		if clusterNetwork.Services != nil {
			// A user may provide zero or one CIDR blocks. If they provide an empty array,
			// we ignore it and use the default. AKS doesn't support > 1 Service/Pod CIDR.
			if len(clusterNetwork.Services.CIDRBlocks) > 1 {
				allErrs = append(allErrs, field.TooMany(field.NewPath("Cluster", "Spec", "ClusterNetwork", "Services", "CIDRBlocks"), len(clusterNetwork.Services.CIDRBlocks), 1))
			}
			if len(clusterNetwork.Services.CIDRBlocks) == 1 {
				serviceCIDR = clusterNetwork.Services.CIDRBlocks[0]
			}
		}
		if clusterNetwork.Pods != nil {
			// A user may provide zero or one CIDR blocks. If they provide an empty array,
			// we ignore it and use the default. AKS doesn't support > 1 Service/Pod CIDR.
			if len(clusterNetwork.Pods.CIDRBlocks) > 1 {
				allErrs = append(allErrs, field.TooMany(field.NewPath("Cluster", "Spec", "ClusterNetwork", "Pods", "CIDRBlocks"), len(clusterNetwork.Pods.CIDRBlocks), 1))
			}
		}
	}

	if m.Spec.DNSServiceIP != nil {
		if serviceCIDR == "" {
			allErrs = append(allErrs, field.Required(field.NewPath("Cluster", "Spec", "ClusterNetwork", "Services", "CIDRBlocks"), "service CIDR must be specified if specifying DNSServiceIP"))
		}
		_, cidr, err := net.ParseCIDR(serviceCIDR)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(field.NewPath("Cluster", "Spec", "ClusterNetwork", "Services", "CIDRBlocks"), serviceCIDR, fmt.Sprintf("failed to parse cluster service cidr: %v", err)))
		}
		ip := net.ParseIP(*m.Spec.DNSServiceIP)
		if !cidr.Contains(ip) {
			allErrs = append(allErrs, field.Invalid(field.NewPath("Cluster", "Spec", "ClusterNetwork", "Services", "CIDRBlocks"), serviceCIDR, "DNSServiceIP must reside within the associated cluster serviceCIDR"))
		}
	}

	if len(allErrs) > 0 {
		return kerrors.NewAggregate(allErrs.ToAggregate().Errors())
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

func (m *AzureManagedControlPlane) validateName(_ client.Client) error {
	if lName := strings.ToLower(m.Name); strings.Contains(lName, "microsoft") ||
		strings.Contains(lName, "windows") {
		return field.Invalid(field.NewPath("Name"), m.Name,
			"cluster name is invalid because 'MICROSOFT' and 'WINDOWS' can't be used as either a whole word or a substring in the name")
	}

	return nil
}

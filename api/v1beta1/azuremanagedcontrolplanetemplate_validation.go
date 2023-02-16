/*
Copyright 2023 The Kubernetes Authors.

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
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Validate the Azure Managed Control Plane Template and return an aggregate error.
func (mcp *AzureManagedControlPlaneTemplate) validateManagedControlPlaneTemplate(cli client.Client) error {
	var allErrs field.ErrorList

	allErrs = append(allErrs, validateDNSServiceIP(
		mcp.Spec.Template.Spec.DNSServiceIP,
		field.NewPath("spec").Child("template").Child("spec").Child("DNSServiceIP"))...)

	allErrs = append(allErrs, validateVersion(
		mcp.Spec.Template.Spec.Version,
		field.NewPath("spec").Child("template").Child("spec").Child("Version"))...)

	allErrs = append(allErrs, validateLoadBalancerProfile(
		mcp.Spec.Template.Spec.LoadBalancerProfile,
		field.NewPath("spec").Child("template").Child("spec").Child("LoadBalancerProfile"))...)

	allErrs = append(allErrs, validateManagedClusterNetwork(
		cli,
		mcp.Labels,
		mcp.Namespace,
		mcp.Spec.Template.Spec.DNSServiceIP,
		mcp.Spec.Template.Spec.VirtualNetwork.Subnet,
		field.NewPath("spec").Child("template").Child("spec"))...)

	allErrs = append(allErrs, validateName(mcp.Name, field.NewPath("Name"))...)

	allErrs = append(allErrs, validateAutoScalerProfile(mcp.Spec.Template.Spec.AutoScalerProfile, field.NewPath("spec").Child("template").Child("spec").Child("AutoScalerProfile"))...)

	return allErrs.ToAggregate()
}

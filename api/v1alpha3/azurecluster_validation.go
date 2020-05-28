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
	"regexp"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	// obtained from https://docs.microsoft.com/en-us/rest/api/resources/resourcegroups/createorupdate#uri-parameters
	resourceGroupRegex = `^[-\w\._\(\)]+$`
	// described in https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/resource-name-rules
	subnetRegex = `^[-\w\._]+$`
	ipv4Regex   = `^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`
)

// validateCluster validates a cluster
func (c *AzureCluster) validateCluster() field.ErrorList {
	return c.validateClusterSpec()
}

// validateClusterSpec validates a ClusterSpec
func (c *AzureCluster) validateClusterSpec() field.ErrorList {
	return validateNetworkSpec(
		c.Spec.NetworkSpec,
		field.NewPath("spec").Child("networkSpec"))
}

// validateNetworkSpec validates a NetworkSpec
func validateNetworkSpec(networkSpec NetworkSpec, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	// If the user specifies a resourceGroup for vnet, it means
	// that she intends to use a pre-existing vnet. In this case,
	// we need to verify the information she provides
	if networkSpec.Vnet.ResourceGroup != "" {
		if err := validateResourceGroup(networkSpec.Vnet.ResourceGroup,
			fldPath.Child("vnet").Child("resourceGroup")); err != nil {
			allErrs = append(allErrs, err)
		}
		allErrs = append(allErrs, validateSubnets(networkSpec.Subnets, fldPath.Child("subnets"))...)
	}
	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
}

// validateResourceGroup validates a ResourceGroup
func validateResourceGroup(resourceGroup string, fldPath *field.Path) *field.Error {
	if success, _ := regexp.MatchString(resourceGroupRegex, resourceGroup); !success {
		return field.Invalid(fldPath, resourceGroup,
			fmt.Sprintf("resourceGroup doesn't match regex %s", resourceGroupRegex))
	}
	return nil
}

// validateSubnets validates a list of Subnets
func validateSubnets(subnets Subnets, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	subnetNames := make(map[string]bool, len(subnets))
	requiredSubnetRoles := map[string]bool{
		"control-plane": false,
		"node":          false,
	}

	for i, subnet := range subnets {
		if err := validateSubnetName(subnet.Name, fldPath.Index(i).Child("name")); err != nil {
			allErrs = append(allErrs, err)
		}
		if _, ok := subnetNames[subnet.Name]; ok {
			allErrs = append(allErrs, field.Duplicate(fldPath, subnet.Name))
		}
		subnetNames[subnet.Name] = true
		if subnet.InternalLBIPAddress != "" {
			if err := validateInternalLBIPAddress(subnet.InternalLBIPAddress,
				fldPath.Index(i).Child("internalLBIPAddress")); err != nil {
				allErrs = append(allErrs, err)
			}
		}
		for role := range requiredSubnetRoles {
			if role == string(subnet.Role) {
				requiredSubnetRoles[role] = true
			}
		}
		if subnet.SecurityGroup.IngressRules != nil {
			for _, ingressRule := range subnet.SecurityGroup.IngressRules {
				if err := validateIngressRule(
					ingressRule,
					fldPath.Index(i).Child("securityGroup").Child("ingressRules").Index(i),
				); err != nil {
					allErrs = append(allErrs, err)
				}
			}
		}
	}
	for k, v := range requiredSubnetRoles {
		if v == false {
			allErrs = append(allErrs, field.Required(fldPath,
				fmt.Sprintf("required role %s not included in provided subnets", k)))
		}
	}
	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
}

// validateSubnetName validates the Name of a Subnet
func validateSubnetName(name string, fldPath *field.Path) *field.Error {
	if success, _ := regexp.Match(subnetRegex, []byte(name)); !success {
		return field.Invalid(fldPath, name,
			fmt.Sprintf("name of subnet doesn't match regex %s", subnetRegex))
	}
	return nil
}

// validateInternalLBIPAddress validates a InternalLBIPAddress
func validateInternalLBIPAddress(address string, fldPath *field.Path) *field.Error {
	if success, _ := regexp.Match(ipv4Regex, []byte(address)); !success {
		return field.Invalid(fldPath, address,
			fmt.Sprintf("internalLBIPAddress doesn't match regex %s", ipv4Regex))
	}
	return nil
}

// validateIngressRule validates an IngressRule
func validateIngressRule(ingressRule *IngressRule, fldPath *field.Path) *field.Error {
	if ingressRule.Priority < 100 || ingressRule.Priority > 4096 {
		return field.Invalid(fldPath, ingressRule.Priority,
			fmt.Sprintf("ingress priorities should be between 100 and 4096"))
	}
	return nil
}

func validateControlPlaneIP(old, new *PublicIPSpec, fldPath *field.Path) *field.Error {
	if old == nil && new != nil {
		return field.Invalid(fldPath, new, fmt.Sprintf("setting control plane endpoint after cluster creation is not allowed"))
	}
	if old != nil && new == nil {
		return field.Invalid(fldPath, new, fmt.Sprintf("removing control plane endpoint after cluster creation is not allowed"))
	}
	if old != nil && new != nil && old.Name != new.Name {
		return field.Invalid(fldPath, new, fmt.Sprintf("changing control plane endpoint after cluster creation is not allowed"))
	}
	if new != nil && new.Name == "" {
		return field.Invalid(fldPath, new, fmt.Sprintf("control plane endpoint IP name must be non-empty"))
	}
	return nil
}

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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
func (c *AzureCluster) validateCluster() error {
	var allErrs field.ErrorList
	allErrs = append(allErrs, c.validateClusterSpec()...)
	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "AzureCluster"},
		c.Name, allErrs)
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
	if networkSpec.Vnet.ResourceGroup != "" {
		if err := validateResourceGroup(networkSpec.Vnet.ResourceGroup,
			fldPath.Child("vnet").Child("resourceGroup")); err != nil {
			allErrs = append(allErrs, err)
		}
	}
	allErrs = append(allErrs, validateSubnets(networkSpec.Subnets, fldPath.Child("subnets"))...)
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
	for i, subnet := range subnets {
		if err := validateSubnetName(subnet.Name, fldPath.Index(i).Child("name")); err != nil {
			allErrs = append(allErrs, err)
		}
		if subnet.InternalLBIPAddress != "" {
			if err := validateInternalLBIPAddress(subnet.InternalLBIPAddress,
				fldPath.Index(i).Child("internalLBIPAddress")); err != nil {
				allErrs = append(allErrs, err)
			}
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

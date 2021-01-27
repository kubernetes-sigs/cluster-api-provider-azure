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
	"net"
	"regexp"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

const (
	// can't use: \/"'[]:|<>+=;,.?*@&, Can't start with underscore. Can't end with period or hyphen.
	// not using . in the name to avoid issues when the name is part of DNS name
	clusterNameRegex = `^[a-z][a-z0-9-]{0,44}[a-z0-9]$`
	// max length of 44 to allow for cluster name to be used as a prefix for VMs and other resources that
	// have limitations as outlined here https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/resource-name-rules
	clusterNameMaxLength = 44
	// obtained from https://docs.microsoft.com/en-us/rest/api/resources/resourcegroups/createorupdate#uri-parameters
	resourceGroupRegex = `^[-\w\._\(\)]+$`
	// described in https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/resource-name-rules
	subnetRegex       = `^[-\w\._]+$`
	loadBalancerRegex = `^[-\w\._]+$`
)

// validateCluster validates a cluster
func (c *AzureCluster) validateCluster(old *AzureCluster) error {
	var allErrs field.ErrorList
	allErrs = append(allErrs, c.validateClusterName()...)
	allErrs = append(allErrs, c.validateClusterSpec(old)...)
	if len(allErrs) == 0 {
		return nil
	}

	return apierrors.NewInvalid(
		schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "AzureCluster"},
		c.Name, allErrs)
}

// validateClusterSpec validates a ClusterSpec
func (c *AzureCluster) validateClusterSpec(old *AzureCluster) field.ErrorList {
	var oldNetworkSpec NetworkSpec
	if old != nil {
		oldNetworkSpec = old.Spec.NetworkSpec
	}
	return validateNetworkSpec(
		c.Spec.NetworkSpec,
		oldNetworkSpec,
		field.NewPath("spec").Child("networkSpec"))
}

// validateClusterName validates ClusterName
func (c *AzureCluster) validateClusterName() field.ErrorList {
	var allErrs field.ErrorList
	if len(c.Name) > clusterNameMaxLength {
		allErrs = append(allErrs, field.Invalid(field.NewPath("metadata").Child("Name"), c.Name,
			fmt.Sprintf("Cluster Name longer than allowed length of %d characters", clusterNameMaxLength)))
	}
	if success, _ := regexp.MatchString(clusterNameRegex, c.Name); !success {
		allErrs = append(allErrs, field.Invalid(field.NewPath("metadata").Child("Name"), c.Name,
			fmt.Sprintf("Cluster Name doesn't match regex %s, can contain only lowercase alphanumeric characters and '-', must start/end with an alphanumeric character",
				clusterNameRegex)))
	}
	if len(allErrs) == 0 {
		return nil
	}
	return allErrs
}

// validateNetworkSpec validates a NetworkSpec
func validateNetworkSpec(networkSpec NetworkSpec, old NetworkSpec, fldPath *field.Path) field.ErrorList {
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
	var cidrBlocks []string
	if subnet := networkSpec.GetControlPlaneSubnet(); subnet != nil {
		cidrBlocks = subnet.CIDRBlocks
	}
	allErrs = append(allErrs, validateAPIServerLB(networkSpec.APIServerLB, old.APIServerLB, cidrBlocks, fldPath.Child("apiServerLB"))...)
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
		if !v {
			allErrs = append(allErrs, field.Required(fldPath,
				fmt.Sprintf("required role %s not included in provided subnets", k)))
		}
	}
	return allErrs
}

// validateSubnetName validates the Name of a Subnet.
func validateSubnetName(name string, fldPath *field.Path) *field.Error {
	if success, _ := regexp.Match(subnetRegex, []byte(name)); !success {
		return field.Invalid(fldPath, name,
			fmt.Sprintf("name of subnet doesn't match regex %s", subnetRegex))
	}
	return nil
}

// validateLoadBalancerName validates the Name of a Load Balancer.
func validateLoadBalancerName(name string, fldPath *field.Path) *field.Error {
	if success, _ := regexp.Match(loadBalancerRegex, []byte(name)); !success {
		return field.Invalid(fldPath, name,
			fmt.Sprintf("name of load balancer doesn't match regex %s", loadBalancerRegex))
	}
	return nil
}

// validateInternalLBIPAddress validates a InternalLBIPAddress.
func validateInternalLBIPAddress(address string, cidrs []string, fldPath *field.Path) *field.Error {
	ip := net.ParseIP(address)
	if ip == nil {
		return field.Invalid(fldPath, address,
			"Internal LB IP address isn't a valid IPv4 or IPv6 address")
	}
	for _, cidr := range cidrs {
		_, subnet, _ := net.ParseCIDR(cidr)
		if subnet.Contains(ip) {
			return nil
		}
	}
	return field.Invalid(fldPath, address,
		fmt.Sprintf("Internal LB IP address needs to be in control plane subnet range (%s)", cidrs))
}

// validateIngressRule validates an IngressRule
func validateIngressRule(ingressRule *IngressRule, fldPath *field.Path) *field.Error {
	if ingressRule.Priority < 100 || ingressRule.Priority > 4096 {
		return field.Invalid(fldPath, ingressRule.Priority, "ingress priorities should be between 100 and 4096")
	}

	return nil
}

func validateAPIServerLB(lb LoadBalancerSpec, old LoadBalancerSpec, cidrs []string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList
	// SKU should be Standard and is immutable.
	if lb.SKU != SKUStandard {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("sku"), lb.SKU, []string{string(SKUStandard)}))
	}
	if old.SKU != "" && old.SKU != lb.SKU {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("sku"), lb.SKU, "API Server load balancer SKU should not be modified after AzureCluster creation."))
	}

	// Type should be Public or Internal.
	if lb.Type != Internal && lb.Type != Public {
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("type"), lb.Type,
			[]string{string(Public), string(Internal)}))
	}
	if old.Type != "" && old.Type != lb.Type {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("type"), lb.Type, "API Server load balancer type should not be modified after AzureCluster creation."))
	}

	// Name should be valid.
	if err := validateLoadBalancerName(lb.Name, fldPath.Child("name")); err != nil {
		allErrs = append(allErrs, err)
	}
	if old.Name != "" && old.Name != lb.Name {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), lb.Type, "API Server load balancer name should not be modified after AzureCluster creation."))
	}

	// There should only be one IP config.
	if len(lb.FrontendIPs) != 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("frontendIPConfigs"), lb.FrontendIPs,
			"API Server Load balancer should have 1 Frontend IP configuration"))
	} else {
		// if Internal, IP config should not have a public IP.
		if lb.Type == Internal {
			if lb.FrontendIPs[0].PublicIP != nil {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("frontendIPConfigs").Index(0).Child("publicIP"),
					"Internal Load Balancers cannot have a Public IP"))
			}
			if lb.FrontendIPs[0].PrivateIPAddress != "" {
				if err := validateInternalLBIPAddress(lb.FrontendIPs[0].PrivateIPAddress, cidrs,
					fldPath.Child("frontendIPConfigs").Index(0).Child("privateIP")); err != nil {
					allErrs = append(allErrs, err)
				}
				if len(old.FrontendIPs) != 0 && old.FrontendIPs[0].PrivateIPAddress != lb.FrontendIPs[0].PrivateIPAddress {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), lb.Type, "API Server load balancer private IP should not be modified after AzureCluster creation."))
				}
			}
		}

		// if Public, IP config should not have a private IP.
		if lb.Type == Public {
			if lb.FrontendIPs[0].PrivateIPAddress != "" {
				allErrs = append(allErrs, field.Forbidden(fldPath.Child("frontendIPConfigs").Index(0).Child("privateIP"),
					"Public Load Balancers cannot have a Private IP"))
			}
		}
	}

	return allErrs
}

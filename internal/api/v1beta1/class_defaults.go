/*
Copyright The Kubernetes Authors.

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
	. "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

// AzureClusterClassSpecSetDefaults sets default values for AzureClusterClassSpec.
func AzureClusterClassSpecSetDefaults(acc *AzureClusterClassSpec) {
	if acc.AzureEnvironment == "" {
		acc.AzureEnvironment = DefaultAzureCloud
	}
}

// VnetClassSpecSetDefaults sets default values for VnetClassSpec.
func VnetClassSpecSetDefaults(vc *VnetClassSpec) {
	if len(vc.CIDRBlocks) == 0 {
		vc.CIDRBlocks = []string{DefaultVnetCIDR}
	}
}

// SubnetClassSpecSetDefaults sets default values for SubnetClassSpec.
func SubnetClassSpecSetDefaults(sc *SubnetClassSpec, cidr string) {
	if len(sc.CIDRBlocks) == 0 {
		sc.CIDRBlocks = []string{cidr}
	}
}

// SecurityGroupClassSetDefaults sets default values for SecurityGroupClass.
func SecurityGroupClassSetDefaults(sgc *SecurityGroupClass) {
	for i := range sgc.SecurityRules {
		if sgc.SecurityRules[i].Direction == "" {
			sgc.SecurityRules[i].Direction = SecurityRuleDirectionInbound
		}
	}
}

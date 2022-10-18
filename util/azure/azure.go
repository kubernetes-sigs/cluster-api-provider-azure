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

package azure

import (
	"fmt"
	"regexp"
	"strings"
)

var azureResourceGroupNameRE = regexp.MustCompile(`.*/subscriptions/(?:.*)/resourceGroups/(.+)/providers/(?:.*)`)

const (
	// AzureSystemNodeLabelPrefix is a standard node label prefix for Azure features, e.g., kubernetes.azure.com/scalesetpriority.
	AzureSystemNodeLabelPrefix = "kubernetes.azure.com"
	// CustomHeaderPrefix is the prefix of annotations that enable additional cluster / node pool features.
	// Whatever follows the prefix will be passed as a header to cluster/node pool creation/update requests.
	// E.g. add `"infrastructure.cluster.x-k8s.io/custom-header-UseGPUDedicatedVHD": "true"` annotation to
	// AzureManagedMachinePool CR to enable creating GPU nodes by the node pool.
	CustomHeaderPrefix = "infrastructure.cluster.x-k8s.io/custom-header-"
)

// ConvertResourceGroupNameToLower converts the resource group name in the resource ID to be lowered.
// Inspired by https://github.com/kubernetes-sigs/cloud-provider-azure/blob/88c9b89611e7c1fcbd39266928cce8406eb0e728/pkg/provider/azure_wrap.go#L409
func ConvertResourceGroupNameToLower(resourceID string) (string, error) {
	matches := azureResourceGroupNameRE.FindStringSubmatch(resourceID)
	if len(matches) != 2 {
		return "", fmt.Errorf("%q isn't in Azure resource ID format %q", resourceID, azureResourceGroupNameRE.String())
	}

	resourceGroup := matches[1]
	return strings.Replace(resourceID, resourceGroup, strings.ToLower(resourceGroup), 1), nil
}

// IsAzureSystemNodeLabelKey is a helper function that determines whether a node label key is an Azure "system" label.
func IsAzureSystemNodeLabelKey(labelKey string) bool {
	return strings.HasPrefix(labelKey, AzureSystemNodeLabelPrefix)
}

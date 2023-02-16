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
	"strings"

	"k8s.io/utils/ptr"
)

func (mcp *AzureManagedControlPlaneTemplate) setDefaults() {
	if mcp.Spec.Template.Spec.NetworkPlugin == nil {
		networkPlugin := CloudProviderName
		mcp.Spec.Template.Spec.NetworkPlugin = &networkPlugin
	}
	if mcp.Spec.Template.Spec.LoadBalancerSKU == nil {
		loadBalancerSKU := "Standard"
		mcp.Spec.Template.Spec.LoadBalancerSKU = &loadBalancerSKU
	}

	if mcp.Spec.Template.Spec.Version != "" && !strings.HasPrefix(mcp.Spec.Template.Spec.Version, "v") {
		normalizedVersion := "v" + mcp.Spec.Template.Spec.Version
		mcp.Spec.Template.Spec.Version = normalizedVersion
	}

	mcp.setDefaultVirtualNetwork()
	mcp.setDefaultSubnet()
	mcp.setDefaultSku()
	mcp.setDefaultAutoScalerProfile()
}

// setDefaultVirtualNetwork sets the default VirtualNetwork for an AzureManagedControlPlaneTemplate.
func (mcp *AzureManagedControlPlaneTemplate) setDefaultVirtualNetwork() {
	if mcp.Spec.Template.Spec.VirtualNetwork.Name == "" {
		mcp.Spec.Template.Spec.VirtualNetwork.Name = mcp.Name
	}
	if mcp.Spec.Template.Spec.VirtualNetwork.CIDRBlock == "" {
		mcp.Spec.Template.Spec.VirtualNetwork.CIDRBlock = defaultAKSVnetCIDR
	}
}

// setDefaultSubnet sets the default Subnet for an AzureManagedControlPlaneTemplate.
func (mcp *AzureManagedControlPlaneTemplate) setDefaultSubnet() {
	if mcp.Spec.Template.Spec.VirtualNetwork.Subnet.Name == "" {
		mcp.Spec.Template.Spec.VirtualNetwork.Subnet.Name = mcp.Name
	}
	if mcp.Spec.Template.Spec.VirtualNetwork.Subnet.CIDRBlock == "" {
		mcp.Spec.Template.Spec.VirtualNetwork.Subnet.CIDRBlock = defaultAKSNodeSubnetCIDR
	}
}

func (mcp *AzureManagedControlPlaneTemplate) setDefaultSku() {
	if mcp.Spec.Template.Spec.SKU == nil {
		mcp.Spec.Template.Spec.SKU = &AKSSku{
			Tier: FreeManagedControlPlaneTier,
		}
	}
}

func (mcp *AzureManagedControlPlaneTemplate) setDefaultAutoScalerProfile() {
	if mcp.Spec.Template.Spec.AutoScalerProfile == nil {
		return
	}

	// Default values are from https://learn.microsoft.com/en-us/azure/aks/cluster-autoscaler#using-the-autoscaler-profile
	// If any values are set, they all need to be set.
	if mcp.Spec.Template.Spec.AutoScalerProfile.BalanceSimilarNodeGroups == nil {
		mcp.Spec.Template.Spec.AutoScalerProfile.BalanceSimilarNodeGroups = (*BalanceSimilarNodeGroups)(ptr.To(string(BalanceSimilarNodeGroupsFalse)))
	}
	if mcp.Spec.Template.Spec.AutoScalerProfile.Expander == nil {
		mcp.Spec.Template.Spec.AutoScalerProfile.Expander = (*Expander)(ptr.To(string(ExpanderRandom)))
	}
	if mcp.Spec.Template.Spec.AutoScalerProfile.MaxEmptyBulkDelete == nil {
		mcp.Spec.Template.Spec.AutoScalerProfile.MaxEmptyBulkDelete = ptr.To("10")
	}
	if mcp.Spec.Template.Spec.AutoScalerProfile.MaxGracefulTerminationSec == nil {
		mcp.Spec.Template.Spec.AutoScalerProfile.MaxGracefulTerminationSec = ptr.To("600")
	}
	if mcp.Spec.Template.Spec.AutoScalerProfile.MaxNodeProvisionTime == nil {
		mcp.Spec.Template.Spec.AutoScalerProfile.MaxNodeProvisionTime = ptr.To("15m")
	}
	if mcp.Spec.Template.Spec.AutoScalerProfile.MaxTotalUnreadyPercentage == nil {
		mcp.Spec.Template.Spec.AutoScalerProfile.MaxTotalUnreadyPercentage = ptr.To("45")
	}
	if mcp.Spec.Template.Spec.AutoScalerProfile.NewPodScaleUpDelay == nil {
		mcp.Spec.Template.Spec.AutoScalerProfile.NewPodScaleUpDelay = ptr.To("0s")
	}
	if mcp.Spec.Template.Spec.AutoScalerProfile.OkTotalUnreadyCount == nil {
		mcp.Spec.Template.Spec.AutoScalerProfile.OkTotalUnreadyCount = ptr.To("3")
	}
	if mcp.Spec.Template.Spec.AutoScalerProfile.ScanInterval == nil {
		mcp.Spec.Template.Spec.AutoScalerProfile.ScanInterval = ptr.To("10s")
	}
	if mcp.Spec.Template.Spec.AutoScalerProfile.ScaleDownDelayAfterAdd == nil {
		mcp.Spec.Template.Spec.AutoScalerProfile.ScaleDownDelayAfterAdd = ptr.To("10m")
	}
	if mcp.Spec.Template.Spec.AutoScalerProfile.ScaleDownDelayAfterDelete == nil {
		// Default is the same as the ScanInterval so default to that same value if it isn't set
		mcp.Spec.Template.Spec.AutoScalerProfile.ScaleDownDelayAfterDelete = mcp.Spec.Template.Spec.AutoScalerProfile.ScanInterval
	}
	if mcp.Spec.Template.Spec.AutoScalerProfile.ScaleDownDelayAfterFailure == nil {
		mcp.Spec.Template.Spec.AutoScalerProfile.ScaleDownDelayAfterFailure = ptr.To("3m")
	}
	if mcp.Spec.Template.Spec.AutoScalerProfile.ScaleDownUnneededTime == nil {
		mcp.Spec.Template.Spec.AutoScalerProfile.ScaleDownUnneededTime = ptr.To("10m")
	}
	if mcp.Spec.Template.Spec.AutoScalerProfile.ScaleDownUnreadyTime == nil {
		mcp.Spec.Template.Spec.AutoScalerProfile.ScaleDownUnreadyTime = ptr.To("20m")
	}
	if mcp.Spec.Template.Spec.AutoScalerProfile.ScaleDownUtilizationThreshold == nil {
		mcp.Spec.Template.Spec.AutoScalerProfile.ScaleDownUtilizationThreshold = ptr.To("0.5")
	}
	if mcp.Spec.Template.Spec.AutoScalerProfile.SkipNodesWithLocalStorage == nil {
		mcp.Spec.Template.Spec.AutoScalerProfile.SkipNodesWithLocalStorage = (*SkipNodesWithLocalStorage)(ptr.To(string(SkipNodesWithLocalStorageFalse)))
	}
	if mcp.Spec.Template.Spec.AutoScalerProfile.SkipNodesWithSystemPods == nil {
		mcp.Spec.Template.Spec.AutoScalerProfile.SkipNodesWithSystemPods = (*SkipNodesWithSystemPods)(ptr.To(string(SkipNodesWithSystemPodsTrue)))
	}
}

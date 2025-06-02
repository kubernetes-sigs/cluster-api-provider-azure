/*
Copyright 2019 The Kubernetes Authors.

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

package networksecuritygroups

import (
	"context"

	"github.com/Azure/azure-service-operator/v2/api/network/v1api20201101"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
)

// ResourceRef implements azure.ASOResourceSpecGetter.
func (s *NSGSpec) ResourceRef() *v1api20201101.NetworkSecurityGroup {
	return &v1api20201101.NetworkSecurityGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name: azure.GetNormalizedKubernetesName(s.Name),
		},
	}
}

// NSGSpec defines the specification for a security group.
type NSGSpec struct {
	Name                     string
	SecurityRules            infrav1.SecurityRules
	Location                 string
	ClusterName              string
	ResourceGroup            string
	AdditionalTags           infrav1.Tags
	LastAppliedSecurityRules map[string]interface{}
}

// ResourceName returns the name of the security group.
func (s *NSGSpec) ResourceName() string {
	return s.Name
}

// ResourceGroupName returns the name of the resource group.
func (s *NSGSpec) ResourceGroupName() string {
	return s.ResourceGroup
}

// Parameters returns the parameters for the security group.
func (s *NSGSpec) Parameters(_ context.Context, existing *v1api20201101.NetworkSecurityGroup) (*v1api20201101.NetworkSecurityGroup, error) {
	params := existing
	if params == nil {
		params = &v1api20201101.NetworkSecurityGroup{}
	}
	tags := converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
		ClusterName: s.ClusterName,
		Lifecycle:   infrav1.ResourceLifecycleOwned,
		Name:        ptr.To(s.Name),
		Additional:  s.AdditionalTags,
	}))
	params.Spec = v1api20201101.NetworkSecurityGroup_Spec{
		Location: ptr.To(s.Location),
		Owner: &genruntime.KnownResourceReference{
			Name: s.ResourceGroupName(),
		},
	}
	// Set metadata
	if params.ObjectMeta.Labels == nil {
		params.ObjectMeta.Labels = make(map[string]string)
	}
	// Add additional tags
	for k, v := range tags {
		if v != nil {
			params.ObjectMeta.Labels[k] = *v
		}
	}
	params.ObjectMeta.Labels[clusterv1.ClusterNameLabel] = s.ClusterName

	return params, nil
}

// WasManaged implements azure.ASOResourceSpecGetter.
func (s *NSGSpec) WasManaged(_ *v1api20201101.NetworkSecurityGroup) bool {
	// Network security groups are always managed by CAPZ
	return true
}

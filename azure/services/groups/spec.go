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

package groups

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
)

// GroupSpec defines the specification for a Resource Group.
type GroupSpec struct {
	Name           string
	Location       string
	ClusterName    string
	AdditionalTags infrav1.Tags
}

// ResourceName returns the name of the group.
func (s *GroupSpec) ResourceName() string {
	return s.Name
}

// ResourceGroupName returns the name of the group.
// Note that it is the same as the resource name in this case.
func (s *GroupSpec) ResourceGroupName() string {
	return s.Name
}

// OwnerResourceName is a no-op for groups.
func (s *GroupSpec) OwnerResourceName() string {
	return "" // not applicable
}

// Parameters returns the parameters for the group.
func (s *GroupSpec) Parameters(ctx context.Context, existing interface{}) (params interface{}, err error) {
	if existing != nil {
		// rg already exists, nothing to update.
		// Note that rg tags are updated separately using tags service.
		return nil, nil
	}
	return resources.Group{
		Location: ptr.To(s.Location),
		// User defined additional tags are created with the resource group and updated using tags service.
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: s.ClusterName,
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Name:        ptr.To(s.Name),
			Role:        ptr.To(infrav1.CommonRole),
			Additional:  s.AdditionalTags,
		})),
	}, nil
}

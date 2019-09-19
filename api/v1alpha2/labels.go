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

package v1alpha2

import (
	"fmt"
	"reflect"
	"strings"
)

// Labels defines a map of tags.
type Labels map[string]string

// Equals returns true if the tags are equal.
func (t Labels) Equals(other Labels) bool {
	return reflect.DeepEqual(t, other)
}

// HasOwned returns true if the tags contains a tag that marks the resource as owned by the cluster from the perspective of this management tooling.
func (t Labels) HasOwned(cluster string) bool {
	value, ok := t[ClusterTagKey(cluster)]
	return ok && ResourceLifecycle(value) == ResourceLifecycleOwned
}

// // HasOwned returns true if the tags contains a tag that marks the resource as owned by the cluster from the perspective of the in-tree cloud provider.
// func (t Labels) HasAzureCloudProviderOwned(cluster string) bool {
// 	value, ok := t[ClusterAzureCloudProviderTagKey(cluster)]
// 	return ok && ResourceLifecycle(value) == ResourceLifecycleOwned
// }

// GetRole returns the Cluster API role for the tagged resource
func (t Labels) GetRole() string {
	return t[NameAzureClusterAPIRole]
}

// ToComputeFilter returns the string representation of the labels as a filter
// to be used in google compute sdk calls.
func (t Labels) ToComputeFilter() string {
	var builder strings.Builder
	for k, v := range t {
		builder.WriteString(fmt.Sprintf("(labels.%s = %q) ", k, v))
	}
	return builder.String()
}

// Difference returns the difference between this map of tags and the other map of tags.
// Items are considered equals if key and value are equals.
func (t Labels) Difference(other Labels) Labels {
	res := make(Labels, len(t))

	for key, value := range t {
		if otherValue, ok := other[key]; ok && value == otherValue {
			continue
		}
		res[key] = value
	}

	return res
}

// AddLabels adds (and overwrites) the current labels with the ones passed in.
func (t Labels) AddLabels(other Labels) Labels {
	for key, value := range other {
		t[key] = value
	}
	return t
}

// ResourceLifecycle configures the lifecycle of a resource
type ResourceLifecycle string

const (
	// ResourceLifecycleOwned is the value we use when tagging resources to indicate
	// that the resource is considered owned and managed by the cluster,
	// and in particular that the lifecycle is tied to the lifecycle of the cluster.
	ResourceLifecycleOwned = ResourceLifecycle("owned")

	// ResourceLifecycleShared is the value we use when tagging resources to indicate
	// that the resource is shared between multiple clusters, and should not be destroyed
	// if the cluster is destroyed.
	ResourceLifecycleShared = ResourceLifecycle("shared")

	// NameAzureProviderPrefix is the tag prefix we use to differentiate
	// cluster-api-provider-azure owned components from other tooling that
	// uses NameKubernetesClusterPrefix
	NameAzureProviderPrefix = "capg-"

	// NameAzureProviderOwned is the tag name we use to differentiate
	// cluster-api-provider-azure owned components from other tooling that
	// uses NameKubernetesClusterPrefix
	NameAzureProviderOwned = NameAzureProviderPrefix + "cluster"

	// NameAzureClusterAPIRole is the tag name we use to mark roles for resources
	// dedicated to this cluster api provider implementation.
	NameAzureClusterAPIRole = NameAzureProviderPrefix + "role"

	// APIServerRoleTagValue describes the value for the apiserver role
	APIServerRoleTagValue = "apiserver"

	// CommonRoleTagValue describes the value for the common role
	CommonRoleTagValue = "common"

	// PublicRoleTagValue describes the value for the public role
	PublicRoleTagValue = "public"

	// PrivateRoleTagValue describes the value for the private role
	PrivateRoleTagValue = "private"
)

// ClusterTagKey generates the key for resources associated with a cluster.
func ClusterTagKey(name string) string {
	return fmt.Sprintf("%s%s", NameAzureProviderOwned, name)
}

// ClusterAzureCloudProviderTagKey generates the key for resources associated a cluster's Azure cloud provider.
// func ClusterAzureCloudProviderTagKey(name string) string {
// return fmt.Sprintf("%s%s", NameKubernetesAzureCloudProviderPrefix, name)
// }

// BuildParams is used to build tags around an azure resource.
type BuildParams struct {
	// Lifecycle determines the resource lifecycle.
	Lifecycle ResourceLifecycle

	// ClusterName is the cluster associated with the resource.
	ClusterName string

	// ResourceID is the unique identifier of the resource to be tagged.
	ResourceID string

	// Role is the role associated to the resource.
	// +optional
	Role *string

	// Any additional tags to be added to the resource.
	// +optional
	Additional Labels
}

// Build builds tags including the cluster tag and returns them in map form.
func Build(params BuildParams) Labels {
	tags := make(Labels)
	for k, v := range params.Additional {
		tags[strings.ToLower(k)] = strings.ToLower(v)
	}

	tags[ClusterTagKey(params.ClusterName)] = string(params.Lifecycle)
	if params.Role != nil {
		tags[NameAzureClusterAPIRole] = strings.ToLower(*params.Role)
	}

	return tags
}

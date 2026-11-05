/*
Copyright 2025 The Kubernetes Authors.

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

package mutators

import (
	"context"
	"fmt"
	"time"

	asoredhatopenshiftv2026 "github.com/Azure/azure-service-operator/v2/api/redhatopenshift/v20260901preview"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const (
	tagCreatedBy = "createdBy"
	tagCreatedAt = "createdAt"

	tagCreatedByValue = "cluster-api-provider-azure"
)

// SetResourceTags sets default CAPZ tags on HcpOpenShiftCluster and HcpOpenShiftClustersNodePool resources.
// Default tags are only added if not already present, so user-defined tags take precedence.
// The createdAt timestamp uses the owning CAPI object's creation time to remain stable across reconciliations.
func SetResourceTags(clusterName string, createdAt time.Time) ResourcesMutator {
	return func(ctx context.Context, us []*unstructured.Unstructured) error {
		_, log, done := tele.StartSpanWithLogger(ctx, "mutators.SetResourceTags")
		defer done()

		for i, u := range us {
			if u.GroupVersionKind().Group != asoredhatopenshiftv2026.GroupVersion.Group {
				continue
			}
			kind := u.GroupVersionKind().Kind
			if kind != scope.HcpClusterKindName && kind != scope.HcpNodePoolKindName {
				continue
			}

			resourcePath := fmt.Sprintf("spec.resources[%d]", i)
			if err := setDefaultTags(u, clusterName, createdAt, resourcePath, log); err != nil {
				return err
			}
		}

		return nil
	}
}

func setDefaultTags(resource *unstructured.Unstructured, clusterName string, createdAt time.Time, resourcePath string, log logr.Logger) error {
	tagsPath := []string{"spec", "tags"}

	existingTags, _, err := unstructured.NestedStringMap(resource.UnstructuredContent(), tagsPath...)
	if err != nil {
		return err
	}
	if existingTags == nil {
		existingTags = make(map[string]string)
	}

	defaultTags := map[string]string{
		tagCreatedBy:                       tagCreatedByValue,
		tagCreatedAt:                       createdAt.UTC().Format(time.RFC3339),
		infrav1.ClusterTagKey(clusterName): string(infrav1.ResourceLifecycleOwned),
	}

	mutated := false
	for key, val := range defaultTags {
		if _, exists := existingTags[key]; !exists {
			existingTags[key] = val
			logMutation(log, mutation{
				location: resourcePath + ".spec.tags." + key,
				val:      val,
				reason:   "because CAPZ default tags should be present on managed resources",
			})
			mutated = true
		}
	}

	if !mutated {
		return nil
	}

	tagsInterface := make(map[string]interface{}, len(existingTags))
	for k, v := range existingTags {
		tagsInterface[k] = v
	}

	return unstructured.SetNestedField(resource.UnstructuredContent(), tagsInterface, tagsPath...)
}

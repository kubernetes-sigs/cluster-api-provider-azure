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
	"testing"
	"time"

	asoredhatopenshiftv2026 "github.com/Azure/azure-service-operator/v2/api/redhatopenshift/v20260901preview"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
)

func hcpClusterUnstructured(tags map[string]interface{}) *unstructured.Unstructured {
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": asoredhatopenshiftv2026.GroupVersion.String(),
			"kind":       scope.HcpClusterKindName,
			"metadata": map[string]interface{}{
				"name": "test-hcp-cluster",
			},
			"spec": map[string]interface{}{},
		},
	}
	if tags != nil {
		u.Object["spec"].(map[string]interface{})["tags"] = tags
	}
	return u
}

func nodePoolUnstructured(tags map[string]interface{}) *unstructured.Unstructured {
	u := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": asoredhatopenshiftv2026.GroupVersion.String(),
			"kind":       scope.HcpNodePoolKindName,
			"metadata": map[string]interface{}{
				"name": "test-nodepool",
			},
			"spec": map[string]interface{}{},
		},
	}
	if tags != nil {
		u.Object["spec"].(map[string]interface{})["tags"] = tags
	}
	return u
}

func TestSetResourceTags(t *testing.T) {
	createdAt := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	clusterName := "my-cluster"
	ownershipTagKey := infrav1.ClusterTagKey(clusterName)

	tests := []struct {
		name         string
		resources    []*unstructured.Unstructured
		validateTags func(*WithT, []*unstructured.Unstructured)
	}{
		{
			name: "sets all default tags on HcpOpenShiftCluster",
			resources: []*unstructured.Unstructured{
				hcpClusterUnstructured(nil),
			},
			validateTags: func(g *WithT, us []*unstructured.Unstructured) {
				tags, found, err := unstructured.NestedStringMap(us[0].UnstructuredContent(), "spec", "tags")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(tags).To(HaveKeyWithValue(tagCreatedBy, tagCreatedByValue))
				g.Expect(tags).To(HaveKeyWithValue(tagCreatedAt, "2026-07-30T12:00:00Z"))
				g.Expect(tags).To(HaveKeyWithValue(ownershipTagKey, string(infrav1.ResourceLifecycleOwned)))
			},
		},
		{
			name: "sets all default tags on HcpOpenShiftClustersNodePool",
			resources: []*unstructured.Unstructured{
				nodePoolUnstructured(nil),
			},
			validateTags: func(g *WithT, us []*unstructured.Unstructured) {
				tags, found, err := unstructured.NestedStringMap(us[0].UnstructuredContent(), "spec", "tags")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(tags).To(HaveKeyWithValue(tagCreatedBy, tagCreatedByValue))
				g.Expect(tags).To(HaveKeyWithValue(tagCreatedAt, "2026-07-30T12:00:00Z"))
				g.Expect(tags).To(HaveKeyWithValue(ownershipTagKey, string(infrav1.ResourceLifecycleOwned)))
			},
		},
		{
			name: "preserves user-defined tags",
			resources: []*unstructured.Unstructured{
				hcpClusterUnstructured(map[string]interface{}{
					"environment": "production",
					"team":        "sre",
				}),
			},
			validateTags: func(g *WithT, us []*unstructured.Unstructured) {
				tags, found, err := unstructured.NestedStringMap(us[0].UnstructuredContent(), "spec", "tags")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(tags).To(HaveKeyWithValue("environment", "production"))
				g.Expect(tags).To(HaveKeyWithValue("team", "sre"))
				g.Expect(tags).To(HaveKeyWithValue(tagCreatedBy, tagCreatedByValue))
				g.Expect(tags).To(HaveKeyWithValue(tagCreatedAt, "2026-07-30T12:00:00Z"))
				g.Expect(tags).To(HaveKeyWithValue(ownershipTagKey, string(infrav1.ResourceLifecycleOwned)))
			},
		},
		{
			name: "user-defined tags take precedence over defaults",
			resources: []*unstructured.Unstructured{
				hcpClusterUnstructured(map[string]interface{}{
					tagCreatedBy: "my-custom-tool",
				}),
			},
			validateTags: func(g *WithT, us []*unstructured.Unstructured) {
				tags, found, err := unstructured.NestedStringMap(us[0].UnstructuredContent(), "spec", "tags")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(tags).To(HaveKeyWithValue(tagCreatedBy, "my-custom-tool"))
				g.Expect(tags).To(HaveKeyWithValue(tagCreatedAt, "2026-07-30T12:00:00Z"))
				g.Expect(tags).To(HaveKeyWithValue(ownershipTagKey, string(infrav1.ResourceLifecycleOwned)))
			},
		},
		{
			name: "createdAt is immutable when already set",
			resources: []*unstructured.Unstructured{
				hcpClusterUnstructured(map[string]interface{}{
					tagCreatedAt: "2025-01-01T00:00:00Z",
				}),
			},
			validateTags: func(g *WithT, us []*unstructured.Unstructured) {
				tags, found, err := unstructured.NestedStringMap(us[0].UnstructuredContent(), "spec", "tags")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(tags).To(HaveKeyWithValue(tagCreatedAt, "2025-01-01T00:00:00Z"))
			},
		},
		{
			name: "skips non-ARO resources",
			resources: []*unstructured.Unstructured{
				{
					Object: map[string]interface{}{
						"apiVersion": "containerservice.azure.com/v1api20231001",
						"kind":       "ManagedCluster",
						"metadata": map[string]interface{}{
							"name": "aks-cluster",
						},
						"spec": map[string]interface{}{},
					},
				},
			},
			validateTags: func(g *WithT, us []*unstructured.Unstructured) {
				_, found, err := unstructured.NestedStringMap(us[0].UnstructuredContent(), "spec", "tags")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(found).To(BeFalse())
			},
		},
		{
			name: "handles multiple resources",
			resources: []*unstructured.Unstructured{
				hcpClusterUnstructured(nil),
				nodePoolUnstructured(nil),
			},
			validateTags: func(g *WithT, us []*unstructured.Unstructured) {
				for _, u := range us {
					tags, found, err := unstructured.NestedStringMap(u.UnstructuredContent(), "spec", "tags")
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(found).To(BeTrue())
					g.Expect(tags).To(HaveKeyWithValue(tagCreatedBy, tagCreatedByValue))
					g.Expect(tags).To(HaveKeyWithValue(tagCreatedAt, "2026-07-30T12:00:00Z"))
					g.Expect(tags).To(HaveKeyWithValue(ownershipTagKey, string(infrav1.ResourceLifecycleOwned)))
				}
			},
		},
		{
			name:      "handles empty resources list",
			resources: []*unstructured.Unstructured{},
			validateTags: func(g *WithT, us []*unstructured.Unstructured) {
				g.Expect(us).To(BeEmpty())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			mutator := SetResourceTags(clusterName, createdAt)
			err := mutator(t.Context(), test.resources)
			g.Expect(err).NotTo(HaveOccurred())

			test.validateTags(g, test.resources)
		})
	}
}

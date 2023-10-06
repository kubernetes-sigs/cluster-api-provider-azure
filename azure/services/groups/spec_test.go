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

package groups

import (
	"context"
	"testing"

	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestParameters(t *testing.T) {
	tests := []struct {
		name     string
		spec     *GroupSpec
		existing *asoresourcesv1.ResourceGroup
		expected *asoresourcesv1.ResourceGroup
	}{
		{
			name: "no existing group",
			spec: &GroupSpec{
				Name:           "name",
				Location:       "location",
				ClusterName:    "cluster",
				AdditionalTags: infrav1.Tags{"some": "tags"},
				Namespace:      "namespace",
				Owner: metav1.OwnerReference{
					Kind: "kind",
				},
			},
			existing: nil,
			expected: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "kind",
						},
					},
				},
				Spec: asoresourcesv1.ResourceGroup_Spec{
					Location: ptr.To("location"),
					Tags: map[string]string{
						"some": "tags",
						"sigs.k8s.io_cluster-api-provider-azure_cluster_cluster": "owned",
						"sigs.k8s.io_cluster-api-provider-azure_role":            "common",
						"Name": "name",
					},
				},
			},
		},
		{
			name: "existing group",
			existing: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "a unique name"},
			},
			expected: &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "a unique name"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			actual, err := test.spec.Parameters(context.Background(), test.existing)
			g.Expect(err).NotTo(HaveOccurred())
			if test.expected == nil {
				g.Expect(actual).To(BeNil())
			} else {
				g.Expect(actual).To(Equal(test.expected))
			}
		})
	}
}

func TestWasManaged(t *testing.T) {
	clusterName := "cluster"

	tests := []struct {
		name     string
		object   *asoresourcesv1.ResourceGroup
		expected bool
	}{
		{
			name:     "no owned label",
			object:   &asoresourcesv1.ResourceGroup{},
			expected: false,
		},
		{
			name: "wrong owned label value",
			object: &asoresourcesv1.ResourceGroup{
				Status: asoresourcesv1.ResourceGroup_STATUS{
					Tags: infrav1.Build(infrav1.BuildParams{
						ClusterName: clusterName,
						Lifecycle:   infrav1.ResourceLifecycle("not owned"),
					}),
				},
			},
			expected: false,
		},
		{
			name: "with owned label",
			object: &asoresourcesv1.ResourceGroup{
				Status: asoresourcesv1.ResourceGroup_STATUS{
					Tags: infrav1.Build(infrav1.BuildParams{
						ClusterName: clusterName,
						Lifecycle:   infrav1.ResourceLifecycleOwned,
					}),
				},
			},
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			s := &GroupSpec{
				ClusterName: clusterName,
			}

			g.Expect(s.WasManaged(test.object)).To(Equal(test.expected))
		})
	}
}

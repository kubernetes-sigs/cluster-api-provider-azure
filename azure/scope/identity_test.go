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

package scope

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAllowedNamespaces(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	_ = infrav1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name             string
		identity         *infrav1.AzureClusterIdentity
		clusterNamespace string
		expected         bool
	}{
		{
			name: "allow any cluster namespace when empty",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					AllowedNamespaces: &infrav1.AllowedNamespaces{},
				},
			},
			clusterNamespace: "default",
			expected:         true,
		},
		{
			name: "no namespaces allowed when list is empty",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					AllowedNamespaces: &infrav1.AllowedNamespaces{
						NamespaceList: []string{},
					},
				},
			},
			clusterNamespace: "default",
			expected:         false,
		},
		{
			name: "allow cluster with namespace in list",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					AllowedNamespaces: &infrav1.AllowedNamespaces{
						NamespaceList: []string{"namespace24", "namespace32"},
					},
				},
			},
			clusterNamespace: "namespace24",
			expected:         true,
		},
		{
			name: "don't allow cluster with namespace not in list",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					AllowedNamespaces: &infrav1.AllowedNamespaces{
						NamespaceList: []string{"namespace24", "namespace32"},
					},
				},
			},
			clusterNamespace: "namespace8",
			expected:         false,
		},
		{
			name: "allow cluster when namespace has selector with matching label",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					AllowedNamespaces: &infrav1.AllowedNamespaces{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"c": "d"},
						},
					},
				},
			},
			clusterNamespace: "namespace8",
			expected:         true,
		},
		{
			name: "don't allow cluster when namespace has selector with different label",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					AllowedNamespaces: &infrav1.AllowedNamespaces{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"x": "y"},
						},
					},
				},
			},
			clusterNamespace: "namespace8",
			expected:         false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			fakeNamespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "namespace8",
					Labels: map[string]string{"c": "d"},
				},
			}
			initObjects := []runtime.Object{tc.identity, fakeNamespace}
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

			actual := IsClusterNamespaceAllowed(context.TODO(), fakeClient, tc.identity.Spec.AllowedNamespaces, tc.clusterNamespace)
			g.Expect(actual).To(Equal(tc.expected))
		})
	}
}

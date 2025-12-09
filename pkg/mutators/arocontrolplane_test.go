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

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controlv1 "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
)

func aroClusterUnstructured(_ *WithT, spec map[string]interface{}) *unstructured.Unstructured {
	aroCluster := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": infrav1exp.GroupVersion.String(),
			"kind":       "AROCluster",
			"metadata": map[string]interface{}{
				"name": "test-cluster",
			},
		},
	}

	if spec != nil {
		aroCluster.Object["spec"] = spec
	}

	return aroCluster
}

func TestSetAROClusterDefaults(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = controlv1.AddToScheme(scheme)

	testCases := []struct {
		name                   string
		aroControlPlane        *controlv1.AROControlPlane
		cluster                *clusterv1.Cluster
		resources              []*unstructured.Unstructured
		expectedError          error
		expectMutationError    bool
		validateMutatedCluster func(*WithT, *unstructured.Unstructured)
	}{
		{
			name: "no ARO cluster",
			aroControlPlane: &controlv1.AROControlPlane{
				Spec: controlv1.AROControlPlaneSpec{
					Version: "v1.25.0",
				},
			},
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
			},
			resources:     []*unstructured.Unstructured{},
			expectedError: ErrNoAROClusterDefined,
		},
		{
			name: "successful mutation",
			aroControlPlane: &controlv1.AROControlPlane{
				Spec: controlv1.AROControlPlaneSpec{
					Version: "v1.25.0",
				},
			},
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Pods: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"10.244.0.0/16"},
						},
						Services: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"10.96.0.0/12"},
						},
					},
				},
			},
			resources: []*unstructured.Unstructured{
				aroClusterUnstructured(g, map[string]interface{}{}),
			},
			validateMutatedCluster: func(g *WithT, cluster *unstructured.Unstructured) {
				// Check kubernetes version
				k8sVersion, found, err := unstructured.NestedString(cluster.Object, "spec", "kubernetesVersion")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(k8sVersion).To(Equal("1.25.0"))

				// Check service CIDR
				serviceCIDR, found, err := unstructured.NestedString(cluster.Object, "spec", "networkProfile", "serviceCidr")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(serviceCIDR).To(Equal("10.96.0.0/12"))

				// Check pod CIDR
				podCIDR, found, err := unstructured.NestedString(cluster.Object, "spec", "networkProfile", "podCidr")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(podCIDR).To(Equal("10.244.0.0/16"))

				// Check credentials
				secrets, found, err := unstructured.NestedMap(cluster.Object, "spec", "operatorSpec", "secrets")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(secrets).To(HaveKey("adminCredentials"))
			},
		},
		{
			name: "version conflict",
			aroControlPlane: &controlv1.AROControlPlane{
				Spec: controlv1.AROControlPlaneSpec{
					Version: "v1.25.0",
				},
			},
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
			},
			resources: []*unstructured.Unstructured{
				aroClusterUnstructured(g, map[string]interface{}{
					"kubernetesVersion": "1.24.0",
				}),
			},
			expectMutationError: true,
		},
		{
			name: "service CIDR conflict",
			aroControlPlane: &controlv1.AROControlPlane{
				Spec: controlv1.AROControlPlaneSpec{
					Version: "v1.25.0",
				},
			},
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Services: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"10.96.0.0/12"},
						},
					},
				},
			},
			resources: []*unstructured.Unstructured{
				aroClusterUnstructured(g, map[string]interface{}{
					"networkProfile": map[string]interface{}{
						"serviceCidr": "10.100.0.0/16",
					},
				}),
			},
			expectMutationError: true,
		},
		{
			name: "pod CIDR conflict",
			aroControlPlane: &controlv1.AROControlPlane{
				Spec: controlv1.AROControlPlaneSpec{
					Version: "v1.25.0",
				},
			},
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Pods: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"10.244.0.0/16"},
						},
					},
				},
			},
			resources: []*unstructured.Unstructured{
				aroClusterUnstructured(g, map[string]interface{}{
					"networkProfile": map[string]interface{}{
						"podCidr": "10.200.0.0/16",
					},
				}),
			},
			expectMutationError: true,
		},
		{
			name: "existing user credentials",
			aroControlPlane: &controlv1.AROControlPlane{
				Spec: controlv1.AROControlPlaneSpec{
					Version: "v1.25.0",
				},
			},
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
			},
			resources: []*unstructured.Unstructured{
				aroClusterUnstructured(g, map[string]interface{}{
					"operatorSpec": map[string]interface{}{
						"secrets": map[string]interface{}{
							"userCredentials": map[string]interface{}{
								"name": "existing-secret",
								"key":  "kubeconfig",
							},
						},
					},
				}),
			},
			validateMutatedCluster: func(g *WithT, cluster *unstructured.Unstructured) {
				// Should preserve existing user credentials
				secrets, found, err := unstructured.NestedMap(cluster.Object, "spec", "operatorSpec", "secrets")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(secrets).To(HaveKey("userCredentials"))
				g.Expect(secrets).NotTo(HaveKey("adminCredentials"))
			},
		},
		{
			name: "existing admin credentials",
			aroControlPlane: &controlv1.AROControlPlane{
				Spec: controlv1.AROControlPlaneSpec{
					Version: "v1.25.0",
				},
			},
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
			},
			resources: []*unstructured.Unstructured{
				aroClusterUnstructured(g, map[string]interface{}{
					"operatorSpec": map[string]interface{}{
						"secrets": map[string]interface{}{
							"adminCredentials": map[string]interface{}{
								"name": "existing-secret",
								"key":  "kubeconfig",
							},
						},
					},
				}),
			},
			validateMutatedCluster: func(g *WithT, cluster *unstructured.Unstructured) {
				// Should preserve existing admin credentials
				secrets, found, err := unstructured.NestedMap(cluster.Object, "spec", "operatorSpec", "secrets")
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(found).To(BeTrue())
				g.Expect(secrets).To(HaveKey("adminCredentials"))
				g.Expect(secrets).NotTo(HaveKey("userCredentials"))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
			mutator := SetAROClusterDefaults(fakeClient, tc.aroControlPlane, tc.cluster)

			err := mutator(t.Context(), tc.resources)

			if tc.expectedError != nil {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(Equal(reconcile.TerminalError(tc.expectedError)))
				return
			}

			if tc.expectMutationError {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(BeAssignableToTypeOf(Incompatible{}))
				return
			}

			g.Expect(err).NotTo(HaveOccurred())

			if tc.validateMutatedCluster != nil && len(tc.resources) > 0 {
				tc.validateMutatedCluster(g, tc.resources[0])
			}
		})
	}
}

func TestSetAROClusterKubernetesVersion(t *testing.T) {
	g := NewWithT(t)

	testCases := []struct {
		name               string
		aroControlPlane    *controlv1.AROControlPlane
		aroCluster         *unstructured.Unstructured
		expectError        bool
		expectedK8sVersion string
	}{
		{
			name: "empty version in control plane",
			aroControlPlane: &controlv1.AROControlPlane{
				Spec: controlv1.AROControlPlaneSpec{
					Version: "",
				},
			},
			aroCluster:  aroClusterUnstructured(g, map[string]interface{}{}),
			expectError: false,
		},
		{
			name: "version with v prefix",
			aroControlPlane: &controlv1.AROControlPlane{
				Spec: controlv1.AROControlPlaneSpec{
					Version: "v1.25.0",
				},
			},
			aroCluster:         aroClusterUnstructured(g, map[string]interface{}{}),
			expectError:        false,
			expectedK8sVersion: "1.25.0",
		},
		{
			name: "version without v prefix",
			aroControlPlane: &controlv1.AROControlPlane{
				Spec: controlv1.AROControlPlaneSpec{
					Version: "1.25.0",
				},
			},
			aroCluster:         aroClusterUnstructured(g, map[string]interface{}{}),
			expectError:        false,
			expectedK8sVersion: "1.25.0",
		},
		{
			name: "conflicting version",
			aroControlPlane: &controlv1.AROControlPlane{
				Spec: controlv1.AROControlPlaneSpec{
					Version: "v1.25.0",
				},
			},
			aroCluster: aroClusterUnstructured(g, map[string]interface{}{
				"kubernetesVersion": "1.24.0",
			}),
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			err := setAROClusterKubernetesVersion(t.Context(), tc.aroControlPlane, "spec.resources[0]", tc.aroCluster)

			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(BeAssignableToTypeOf(Incompatible{}))
			} else {
				g.Expect(err).NotTo(HaveOccurred())

				if tc.expectedK8sVersion != "" {
					k8sVersion, found, err := unstructured.NestedString(tc.aroCluster.Object, "spec", "kubernetesVersion")
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(found).To(BeTrue())
					g.Expect(k8sVersion).To(Equal(tc.expectedK8sVersion))
				}
			}
		})
	}
}

func TestSetAROClusterServiceCIDR(t *testing.T) {
	g := NewWithT(t)

	testCases := []struct {
		name                string
		cluster             *clusterv1.Cluster
		aroCluster          *unstructured.Unstructured
		expectError         bool
		expectedServiceCIDR string
	}{
		{
			name: "no cluster network",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: nil,
				},
			},
			aroCluster:  aroClusterUnstructured(g, map[string]interface{}{}),
			expectError: false,
		},
		{
			name: "no services config",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Services: nil,
					},
				},
			},
			aroCluster:  aroClusterUnstructured(g, map[string]interface{}{}),
			expectError: false,
		},
		{
			name: "empty CIDR blocks",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Services: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{},
						},
					},
				},
			},
			aroCluster:  aroClusterUnstructured(g, map[string]interface{}{}),
			expectError: false,
		},
		{
			name: "successful service CIDR setting",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Services: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"10.96.0.0/12"},
						},
					},
				},
			},
			aroCluster:          aroClusterUnstructured(g, map[string]interface{}{}),
			expectError:         false,
			expectedServiceCIDR: "10.96.0.0/12",
		},
		{
			name: "conflicting service CIDR",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Services: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"10.96.0.0/12"},
						},
					},
				},
			},
			aroCluster: aroClusterUnstructured(g, map[string]interface{}{
				"networkProfile": map[string]interface{}{
					"serviceCidr": "10.100.0.0/16",
				},
			}),
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			err := setAROClusterServiceCIDR(t.Context(), tc.cluster, "spec.resources[0]", tc.aroCluster)

			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(BeAssignableToTypeOf(Incompatible{}))
			} else {
				g.Expect(err).NotTo(HaveOccurred())

				if tc.expectedServiceCIDR != "" {
					serviceCIDR, found, err := unstructured.NestedString(tc.aroCluster.Object, "spec", "networkProfile", "serviceCidr")
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(found).To(BeTrue())
					g.Expect(serviceCIDR).To(Equal(tc.expectedServiceCIDR))
				}
			}
		})
	}
}

func TestSetAROClusterPodCIDR(t *testing.T) {
	g := NewWithT(t)

	testCases := []struct {
		name            string
		cluster         *clusterv1.Cluster
		aroCluster      *unstructured.Unstructured
		expectError     bool
		expectedPodCIDR string
	}{
		{
			name: "no cluster network",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: nil,
				},
			},
			aroCluster:  aroClusterUnstructured(g, map[string]interface{}{}),
			expectError: false,
		},
		{
			name: "successful pod CIDR setting",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Pods: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"10.244.0.0/16"},
						},
					},
				},
			},
			aroCluster:      aroClusterUnstructured(g, map[string]interface{}{}),
			expectError:     false,
			expectedPodCIDR: "10.244.0.0/16",
		},
		{
			name: "conflicting pod CIDR",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Pods: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"10.244.0.0/16"},
						},
					},
				},
			},
			aroCluster: aroClusterUnstructured(g, map[string]interface{}{
				"networkProfile": map[string]interface{}{
					"podCidr": "10.200.0.0/16",
				},
			}),
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			err := setAROClusterPodCIDR(t.Context(), tc.cluster, "spec.resources[0]", tc.aroCluster)

			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(BeAssignableToTypeOf(Incompatible{}))
			} else {
				g.Expect(err).NotTo(HaveOccurred())

				if tc.expectedPodCIDR != "" {
					podCIDR, found, err := unstructured.NestedString(tc.aroCluster.Object, "spec", "networkProfile", "podCidr")
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(found).To(BeTrue())
					g.Expect(podCIDR).To(Equal(tc.expectedPodCIDR))
				}
			}
		})
	}
}

func TestSetAROClusterCredentials(t *testing.T) {
	g := NewWithT(t)

	testCases := []struct {
		name              string
		cluster           *clusterv1.Cluster
		aroCluster        *unstructured.Unstructured
		expectError       bool
		expectCredentials bool
		credentialsType   string
	}{
		{
			name: "no existing credentials",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
			},
			aroCluster:        aroClusterUnstructured(g, map[string]interface{}{}),
			expectError:       false,
			expectCredentials: true,
			credentialsType:   "adminCredentials",
		},
		{
			name: "existing user credentials",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
			},
			aroCluster: aroClusterUnstructured(g, map[string]interface{}{
				"operatorSpec": map[string]interface{}{
					"secrets": map[string]interface{}{
						"userCredentials": map[string]interface{}{
							"name": "existing-secret",
							"key":  "kubeconfig",
						},
					},
				},
			}),
			expectError: false,
		},
		{
			name: "existing admin credentials",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster", Namespace: "default"},
			},
			aroCluster: aroClusterUnstructured(g, map[string]interface{}{
				"operatorSpec": map[string]interface{}{
					"secrets": map[string]interface{}{
						"adminCredentials": map[string]interface{}{
							"name": "existing-secret",
							"key":  "kubeconfig",
						},
					},
				},
			}),
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			err := setAROClusterCredentials(t.Context(), tc.cluster, "spec.resources[0]", tc.aroCluster)

			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())

				if tc.expectCredentials {
					secrets, found, err := unstructured.NestedMap(tc.aroCluster.Object, "spec", "operatorSpec", "secrets")
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(found).To(BeTrue())
					g.Expect(secrets).To(HaveKey(tc.credentialsType))

					adminCreds := secrets[tc.credentialsType].(map[string]interface{})
					expectedSecretName := tc.cluster.Name + "-" + string(secret.Kubeconfig)
					g.Expect(adminCreds["name"]).To(Equal(expectedSecretName))
					g.Expect(adminCreds["key"]).To(Equal(secret.KubeconfigDataName))
				}
			}
		})
	}
}

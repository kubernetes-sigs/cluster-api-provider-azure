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

package controllers

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	cplane "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
	infrav2 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
)

const (
	testAROClusterName = "test-arocluster"
)

func TestAROClusterReconciler_Reconcile(t *testing.T) {
	testcases := []struct {
		name                  string
		request               reconcile.Request
		expectedResult        ctrl.Result
		expectedError         bool
		aroClusterExists      bool
		clusterExists         bool
		aroControlPlaneExists bool
		pausedCluster         bool
		pausedAROCluster      bool
		deletionTimestamp     bool
		controlPlaneAPIURL    string
		controlPlaneEndpoint  *clusterv1.APIEndpoint
		expectedReady         bool
		expectedProvisioned   bool
		validateConditions    func(*WithT, infrav2.AROCluster)
	}{
		{
			name:             "arocluster not found",
			request:          reconcile.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: "nonexistent"}},
			expectedResult:   ctrl.Result{},
			expectedError:    false,
			aroClusterExists: false,
		},
		{
			name:                  "successful reconciliation without control plane",
			request:               reconcile.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: testAROClusterName}},
			expectedResult:        ctrl.Result{Requeue: true}, // Finalizer gets added
			expectedError:         false,
			aroClusterExists:      true,
			clusterExists:         true,
			aroControlPlaneExists: true, // Need control plane to avoid terminal error
			controlPlaneAPIURL:    "",   // But no API URL set
			expectedReady:         false,
			expectedProvisioned:   false,
		},
		{
			name:                  "successful reconciliation with control plane API URL",
			request:               reconcile.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: testAROClusterName}},
			expectedResult:        ctrl.Result{Requeue: true}, // Finalizer gets added first time
			expectedError:         false,
			aroClusterExists:      true,
			clusterExists:         true,
			aroControlPlaneExists: true,
			controlPlaneAPIURL:    "https://test-api.example.com:443",
			controlPlaneEndpoint: &clusterv1.APIEndpoint{
				Host: "test-api.example.com",
				Port: 443,
			},
			expectedReady:       false, // Still false on first reconcile when finalizer is added
			expectedProvisioned: false,
		},
		{
			name:             "paused cluster",
			request:          reconcile.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: testAROClusterName}},
			expectedResult:   ctrl.Result{},
			expectedError:    false,
			aroClusterExists: true,
			clusterExists:    true,
			pausedCluster:    true,
			expectedReady:    false,
		},
		{
			name:             "paused arocluster",
			request:          reconcile.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: testAROClusterName}},
			expectedResult:   ctrl.Result{},
			expectedError:    false,
			aroClusterExists: true,
			clusterExists:    true,
			pausedAROCluster: true,
			expectedReady:    false,
		},
		{
			name:                  "arocluster being deleted",
			request:               reconcile.Request{NamespacedName: types.NamespacedName{Namespace: testNamespace, Name: testAROClusterName}},
			expectedResult:        ctrl.Result{},
			expectedError:         false,
			aroClusterExists:      true,
			clusterExists:         true,
			aroControlPlaneExists: true, // Need control plane for proper cluster setup
			deletionTimestamp:     true,
			expectedReady:         false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			scheme := runtime.NewScheme()
			_ = clusterv1.AddToScheme(scheme)
			_ = infrav2.AddToScheme(scheme)
			_ = cplane.AddToScheme(scheme)
			_ = corev1.AddToScheme(scheme)

			var objects []client.Object

			// Create test objects
			var cluster *clusterv1.Cluster
			var aroCluster *infrav2.AROCluster
			var aroControlPlane *cplane.AROControlPlane

			if tc.clusterExists {
				cluster = createCluster(testClusterName, testNamespace, tc.pausedCluster)
				// Ensure cluster has proper ControlPlaneRef
				cluster.Spec.ControlPlaneRef = &corev1.ObjectReference{
					APIVersion: cplane.GroupVersion.String(),
					Kind:       "AROControlPlane",
					Name:       testControlPlaneName,
					Namespace:  testNamespace,
				}
				objects = append(objects, cluster)
			}

			if tc.aroClusterExists {
				aroCluster = &infrav2.AROCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:            testAROClusterName,
						Namespace:       testNamespace,
						ResourceVersion: "999",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: clusterv1.GroupVersion.String(),
								Kind:       "Cluster",
								Name:       testClusterName,
								UID:        "test-uid",
							},
						},
					},
					Spec: infrav2.AROClusterSpec{},
				}

				if tc.pausedAROCluster {
					aroCluster.Annotations = map[string]string{
						clusterv1.PausedAnnotation: "true",
					}
				}

				if tc.deletionTimestamp {
					now := metav1.Now()
					aroCluster.DeletionTimestamp = &now
					aroCluster.Finalizers = []string{infrav2.AROClusterFinalizer}
				} else {
					// Add finalizer for normal cases
					aroCluster.Finalizers = []string{} // Start without finalizers
				}

				if tc.controlPlaneEndpoint != nil {
					aroCluster.Spec.ControlPlaneEndpoint = *tc.controlPlaneEndpoint
				}

				objects = append(objects, aroCluster)
			}

			if tc.aroControlPlaneExists {
				aroControlPlane = createControlPlane(testControlPlaneName, testNamespace, nil, false)
				aroControlPlane.Status.APIURL = tc.controlPlaneAPIURL
				if tc.controlPlaneAPIURL != "" {
					aroControlPlane.Status.Ready = true
				}
				objects = append(objects, aroControlPlane)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				WithStatusSubresource(&infrav2.AROCluster{}).
				Build()

			reconciler := &AROClusterReconciler{
				Client:           fakeClient,
				WatchFilterValue: "",
			}

			result, err := reconciler.Reconcile(t.Context(), tc.request)

			if tc.expectedError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}

			g.Expect(result).To(Equal(tc.expectedResult))

			// Check AROCluster state if it exists and not being deleted
			if tc.aroClusterExists && !tc.deletionTimestamp {
				var updatedAROCluster infrav2.AROCluster
				err := fakeClient.Get(t.Context(), types.NamespacedName{
					Namespace: testNamespace,
					Name:      testAROClusterName,
				}, &updatedAROCluster)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(updatedAROCluster.Status.Ready).To(Equal(tc.expectedReady))

				if updatedAROCluster.Status.Initialization != nil {
					g.Expect(updatedAROCluster.Status.Initialization.Provisioned).To(Equal(tc.expectedProvisioned))
				} else if tc.expectedProvisioned {
					g.Fail("Expected initialization status to be set")
				}

				if tc.validateConditions != nil {
					tc.validateConditions(g, updatedAROCluster)
				}
			}
		})
	}
}

func TestAROClusterReconciler_reconcileNormal(t *testing.T) {
	testcases := []struct {
		name                 string
		cluster              *clusterv1.Cluster
		aroCluster           *infrav2.AROCluster
		aroControlPlane      *cplane.AROControlPlane
		controlPlaneAPIURL   string
		expectedReady        bool
		expectedProvisioned  bool
		expectedEndpointHost string
		expectedEndpointPort int32
		expectedError        bool
		validateConditions   func(*WithT, infrav2.AROCluster)
	}{
		{
			name: "cluster without control plane",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testClusterName,
					Namespace: testNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					ControlPlaneRef: &corev1.ObjectReference{
						APIVersion: "invalid.api.group/v1",
						Kind:       "SomeOtherControlPlane",
						Name:       testControlPlaneName,
						Namespace:  testNamespace,
					},
				},
			},
			aroCluster: &infrav2.AROCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAROClusterName,
					Namespace: testNamespace,
				},
			},
			expectedError: true,
		},
		{
			name: "cluster with aro control plane but no API URL",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testClusterName,
					Namespace: testNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					ControlPlaneRef: &corev1.ObjectReference{
						APIVersion: cplane.GroupVersion.String(),
						Kind:       "AROControlPlane",
						Name:       testControlPlaneName,
						Namespace:  testNamespace,
					},
				},
			},
			aroCluster: &infrav2.AROCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAROClusterName,
					Namespace: testNamespace,
				},
			},
			aroControlPlane: &cplane.AROControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testControlPlaneName,
					Namespace: testNamespace,
				},
				Status: cplane.AROControlPlaneStatus{
					APIURL: "", // No API URL set
				},
			},
			expectedReady:       false,
			expectedProvisioned: false,
		},
		{
			name: "cluster with control plane and API URL",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testClusterName,
					Namespace: testNamespace,
				},
				Spec: clusterv1.ClusterSpec{
					ControlPlaneRef: &corev1.ObjectReference{
						APIVersion: cplane.GroupVersion.String(),
						Kind:       "AROControlPlane",
						Name:       testControlPlaneName,
						Namespace:  testNamespace,
					},
				},
			},
			aroCluster: &infrav2.AROCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAROClusterName,
					Namespace: testNamespace,
				},
			},
			aroControlPlane: &cplane.AROControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testControlPlaneName,
					Namespace: testNamespace,
				},
				Status: cplane.AROControlPlaneStatus{
					APIURL: "https://test-api.example.com:443",
				},
			},
			controlPlaneAPIURL:   "https://test-api.example.com:443",
			expectedReady:        false, // Still false on first call when finalizer is added
			expectedProvisioned:  false,
			expectedEndpointHost: "", // Empty on first call due to finalizer requeue
			expectedEndpointPort: 0,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			scheme := runtime.NewScheme()
			_ = clusterv1.AddToScheme(scheme)
			_ = infrav2.AddToScheme(scheme)
			_ = cplane.AddToScheme(scheme)

			var objects []client.Object
			objects = append(objects, tc.cluster, tc.aroCluster)
			if tc.aroControlPlane != nil {
				objects = append(objects, tc.aroControlPlane)
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				WithStatusSubresource(&infrav2.AROCluster{}).
				Build()

			reconciler := &AROClusterReconciler{
				Client:           fakeClient,
				WatchFilterValue: "",
			}

			result, err := reconciler.reconcileNormal(t.Context(), tc.aroCluster, tc.cluster)

			if tc.expectedError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(ctrl.Result{Requeue: true})) // Finalizer gets added
			}

			// Check AROCluster state
			var updatedAROCluster infrav2.AROCluster
			err = fakeClient.Get(t.Context(), types.NamespacedName{
				Namespace: tc.aroCluster.Namespace,
				Name:      tc.aroCluster.Name,
			}, &updatedAROCluster)
			g.Expect(err).NotTo(HaveOccurred())

			if !tc.expectedError {
				g.Expect(updatedAROCluster.Status.Ready).To(Equal(tc.expectedReady))

				if tc.expectedProvisioned {
					g.Expect(updatedAROCluster.Status.Initialization).NotTo(BeNil())
					g.Expect(updatedAROCluster.Status.Initialization.Provisioned).To(BeTrue())
				}

				if tc.expectedEndpointHost != "" {
					g.Expect(updatedAROCluster.Spec.ControlPlaneEndpoint.Host).To(Equal(tc.expectedEndpointHost))
					g.Expect(updatedAROCluster.Spec.ControlPlaneEndpoint.Port).To(Equal(tc.expectedEndpointPort))
				}

				if tc.validateConditions != nil {
					tc.validateConditions(g, updatedAROCluster)
				}
			}
		})
	}
}

func TestAROClusterReconciler_reconcilePaused(t *testing.T) {
	testcases := []struct {
		name       string
		aroCluster *infrav2.AROCluster
	}{
		{
			name: "paused arocluster",
			aroCluster: &infrav2.AROCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      testAROClusterName,
					Namespace: testNamespace,
					Annotations: map[string]string{
						clusterv1.PausedAnnotation: "true",
					},
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			scheme := runtime.NewScheme()
			_ = infrav2.AddToScheme(scheme)

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.aroCluster).
				WithStatusSubresource(&infrav2.AROCluster{}).
				Build()

			reconciler := &AROClusterReconciler{
				Client:           fakeClient,
				WatchFilterValue: "",
			}

			result, err := reconciler.reconcilePaused(t.Context(), tc.aroCluster)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result).To(Equal(ctrl.Result{}))

			// Check that Ready status is false
			var updatedAROCluster infrav2.AROCluster
			err = fakeClient.Get(t.Context(), types.NamespacedName{
				Namespace: tc.aroCluster.Namespace,
				Name:      tc.aroCluster.Name,
			}, &updatedAROCluster)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(updatedAROCluster.Status.Ready).To(BeFalse())
		})
	}
}

func TestAROClusterReconciler_reconcileDelete(t *testing.T) {
	testcases := []struct {
		name           string
		aroCluster     *infrav2.AROCluster
		expectedResult ctrl.Result
	}{
		{
			name: "arocluster with finalizer",
			aroCluster: &infrav2.AROCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:              testAROClusterName,
					Namespace:         testNamespace,
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
					Finalizers:        []string{infrav2.AROClusterFinalizer},
				},
			},
			expectedResult: ctrl.Result{},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			scheme := runtime.NewScheme()
			_ = infrav2.AddToScheme(scheme)

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.aroCluster).
				WithStatusSubresource(&infrav2.AROCluster{}).
				Build()

			reconciler := &AROClusterReconciler{
				Client:           fakeClient,
				WatchFilterValue: "",
			}

			result, err := reconciler.reconcileDelete(t.Context(), tc.aroCluster)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result).To(Equal(tc.expectedResult))

			// Check that Ready status is false
			var updatedAROCluster infrav2.AROCluster
			err = fakeClient.Get(t.Context(), types.NamespacedName{
				Namespace: tc.aroCluster.Namespace,
				Name:      tc.aroCluster.Name,
			}, &updatedAROCluster)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(updatedAROCluster.Status.Ready).To(BeFalse())
		})
	}
}

func TestMatchesAROControlPlaneAPIGroup(t *testing.T) {
	testcases := []struct {
		name       string
		apiVersion string
		expected   bool
	}{
		{
			name:       "valid AROControlPlane API version",
			apiVersion: cplane.GroupVersion.String(),
			expected:   true,
		},
		{
			name:       "different API group",
			apiVersion: "cluster.x-k8s.io/v1beta1",
			expected:   false,
		},
		{
			name:       "empty API version",
			apiVersion: "",
			expected:   false,
		},
		{
			name:       "invalid API version format",
			apiVersion: "invalid",
			expected:   false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			result := matchesAROControlPlaneAPIGroup(tc.apiVersion)
			g.Expect(result).To(Equal(tc.expected))
		})
	}
}

func TestAROClusterReconciler_NotFound(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = infrav2.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	reconciler := &AROClusterReconciler{
		Client:           fakeClient,
		WatchFilterValue: "",
	}

	result, err := reconciler.Reconcile(t.Context(), reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: testNamespace,
			Name:      "nonexistent",
		},
	})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(Equal(ctrl.Result{}))
}

func TestAROClusterReconciler_GetOwnerClusterError(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	_ = infrav2.AddToScheme(scheme)
	_ = clusterv1.AddToScheme(scheme)

	aroCluster := &infrav2.AROCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testAROClusterName,
			Namespace: testNamespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
					Name:       "nonexistent-cluster",
					UID:        "test-uid",
				},
			},
		},
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(aroCluster).
		WithStatusSubresource(&infrav2.AROCluster{}).
		Build()

	reconciler := &AROClusterReconciler{
		Client:           fakeClient,
		WatchFilterValue: "",
	}

	result, err := reconciler.Reconcile(t.Context(), reconcile.Request{
		NamespacedName: types.NamespacedName{
			Namespace: testNamespace,
			Name:      testAROClusterName,
		},
	})

	g.Expect(err).To(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	g.Expect(result).To(Equal(ctrl.Result{}))
}

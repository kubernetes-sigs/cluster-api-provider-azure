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

package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type TestClusterReconcileInput struct {
	createAzureClusterService func(*scope.ClusterScope) (*azureClusterService, error)
	azureClusterOptions       func(ac *infrav1.AzureCluster)
	clusterScopeFailureReason capierrors.ClusterStatusError
	cache                     *scope.ClusterCache
	expectedResult            reconcile.Result
	expectedErr               string
	ready                     bool
}

const (
	location = "westus2"
)

var _ = Describe("AzureClusterReconciler", func() {
	BeforeEach(func() {})
	AfterEach(func() {})

	Context("Reconcile an AzureCluster", func() {
		It("should not error with minimal set up", func() {
			reconciler := NewAzureClusterReconciler(testEnv, testEnv.GetEventRecorderFor("azurecluster-reconciler"), reconciler.DefaultLoopTimeout, "")
			By("Calling reconcile")
			name := test.RandomName("foo", 10)
			instance := &infrav1.AzureCluster{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
			result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: client.ObjectKey{
					Namespace: instance.Namespace,
					Name:      instance.Name,
				},
			})

			Expect(err).To(BeNil())
			Expect(result.RequeueAfter).To(BeZero())
		})
	})
})

func TestAzureClusterReconcile(t *testing.T) {
	g := NewWithT(t)
	scheme, err := newScheme()
	g.Expect(err).NotTo(HaveOccurred())

	defaultCluster := getFakeCluster()
	defaultAzureCluster := getFakeAzureCluster()

	cases := map[string]struct {
		objects []runtime.Object
		fail    bool
		err     string
		event   string
	}{
		"should reconcile normally": {
			objects: []runtime.Object{
				defaultCluster,
				defaultAzureCluster,
			},
		},
		"should raise event if the azure cluster is not found": {
			objects: []runtime.Object{
				defaultCluster,
			},
			event: "AzureClusterObjectNotFound",
		},
		"should raise event if cluster is not found": {
			objects: []runtime.Object{
				getFakeAzureCluster(func(ac *infrav1.AzureCluster) {
					ac.OwnerReferences = nil
				}),
				defaultCluster,
			},
			event: "OwnerRefNotSet",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(tc.objects...).
				WithStatusSubresource(
					&infrav1.AzureCluster{},
				).
				Build()

			reconciler := &AzureClusterReconciler{
				Client:   client,
				Recorder: record.NewFakeRecorder(128),
			}

			_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "default",
					Name:      "my-azure-cluster",
				},
			})
			if tc.event != "" {
				g.Expect(reconciler.Recorder.(*record.FakeRecorder).Events).To(Receive(ContainSubstring(tc.event)))
			}
			if tc.fail {
				g.Expect(err).To(MatchError(tc.err))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureClusterReconcileNormal(t *testing.T) {
	cases := map[string]TestClusterReconcileInput{
		"should reconcile normally": {
			createAzureClusterService: func(cs *scope.ClusterScope) (*azureClusterService, error) {
				return getDefaultAzureClusterService(func(acs *azureClusterService) {
					acs.skuCache = resourceskus.NewStaticCache([]armcompute.ResourceSKU{}, cs.Location())
					acs.scope = cs
				}), nil
			},
			cache: &scope.ClusterCache{},
			ready: true,
		},
		"should fail if azure cluster service creator fails": {
			createAzureClusterService: func(*scope.ClusterScope) (*azureClusterService, error) {
				return nil, errors.New("failed to create azure cluster service")
			},
			cache:       &scope.ClusterCache{},
			expectedErr: "failed to create azure cluster service",
		},
		"should reconcile if terminal error is received": {
			createAzureClusterService: func(cs *scope.ClusterScope) (*azureClusterService, error) {
				return getDefaultAzureClusterService(func(acs *azureClusterService) {
					acs.skuCache = resourceskus.NewStaticCache([]armcompute.ResourceSKU{}, cs.Location())
					acs.scope = cs
				}), nil
			},
			clusterScopeFailureReason: capierrors.CreateClusterError,
			cache:                     &scope.ClusterCache{},
		},
		"should requeue if transient error is received": {
			createAzureClusterService: func(cs *scope.ClusterScope) (*azureClusterService, error) {
				return getDefaultAzureClusterService(func(acs *azureClusterService) {
					acs.skuCache = resourceskus.NewStaticCache([]armcompute.ResourceSKU{}, cs.Location())
					acs.scope = cs
					acs.Reconcile = func(ctx context.Context) error {
						return azure.WithTransientError(errors.New("failed to reconcile AzureCluster"), 10*time.Second)
					}
				}), nil
			},
			cache:          &scope.ClusterCache{},
			expectedResult: reconcile.Result{RequeueAfter: 10 * time.Second},
		},
		"should return error for general failures": {
			createAzureClusterService: func(cs *scope.ClusterScope) (*azureClusterService, error) {
				return getDefaultAzureClusterService(func(acs *azureClusterService) {
					acs.skuCache = resourceskus.NewStaticCache([]armcompute.ResourceSKU{}, cs.Location())
					acs.scope = cs
					acs.Reconcile = func(context.Context) error {
						return errors.New("foo error")
					}
					acs.Pause = func(context.Context) error {
						return errors.New("foo error")
					}
					acs.Delete = func(context.Context) error {
						return errors.New("foo error")
					}
				}), nil
			},
			cache:       &scope.ClusterCache{},
			expectedErr: "failed to reconcile cluster services",
		},
	}

	for name, c := range cases {
		tc := c
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			reconciler, clusterScope, err := getClusterReconcileInputs(tc)
			g.Expect(err).NotTo(HaveOccurred())

			result, err := reconciler.reconcileNormal(context.Background(), clusterScope)
			g.Expect(result).To(Equal(tc.expectedResult))

			if tc.ready {
				g.Expect(clusterScope.AzureCluster.Status.Ready).To(BeTrue())
			}
			if tc.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureClusterReconcilePaused(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()

	sb := runtime.NewSchemeBuilder(
		clusterv1.AddToScheme,
		infrav1.AddToScheme,
		asoresourcesv1.AddToScheme,
	)
	s := runtime.NewScheme()
	g.Expect(sb.AddToScheme(s)).To(Succeed())
	c := fake.NewClientBuilder().
		WithScheme(s).
		Build()

	recorder := record.NewFakeRecorder(1)

	reconciler := NewAzureClusterReconciler(c, recorder, reconciler.DefaultLoopTimeout, "")
	name := test.RandomName("paused", 10)
	namespace := "default"

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: clusterv1.ClusterSpec{
			Paused: true,
		},
	}
	g.Expect(c.Create(ctx, cluster)).To(Succeed())

	instance := &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind:       "Cluster",
					APIVersion: clusterv1.GroupVersion.String(),
					Name:       cluster.Name,
					UID:        cluster.UID,
				},
			},
		},
		Spec: infrav1.AzureClusterSpec{
			AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
				SubscriptionID: "something",
			},
			ResourceGroup: name,
		},
	}
	g.Expect(c.Create(ctx, instance)).To(Succeed())

	rg := &asoresourcesv1.ResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	g.Expect(c.Create(ctx, rg)).To(Succeed())

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{
			Namespace: instance.Namespace,
			Name:      instance.Name,
		},
	})

	g.Expect(err).To(BeNil())
	g.Expect(result.RequeueAfter).To(BeZero())

	g.Eventually(recorder.Events).Should(Receive(Equal("Normal ClusterPaused AzureCluster or linked Cluster is marked as paused. Won't reconcile normally")))
}

func TestAzureClusterReconcileDelete(t *testing.T) {
	cases := map[string]TestClusterReconcileInput{
		"should delete successfully": {
			createAzureClusterService: func(cs *scope.ClusterScope) (*azureClusterService, error) {
				return getDefaultAzureClusterService(func(acs *azureClusterService) {
					acs.skuCache = resourceskus.NewStaticCache([]armcompute.ResourceSKU{}, cs.Location())
					acs.scope = cs
				}), nil
			},
			cache: &scope.ClusterCache{},
		},
		"should fail if failed to create azure cluster service": {
			createAzureClusterService: func(cs *scope.ClusterScope) (*azureClusterService, error) {
				return nil, errors.New("failed to create AzureClusterService")
			},
			cache:       &scope.ClusterCache{},
			expectedErr: "failed to create AzureClusterService",
		},
		"should requeue if transient error is received": {
			createAzureClusterService: func(cs *scope.ClusterScope) (*azureClusterService, error) {
				return getDefaultAzureClusterService(func(acs *azureClusterService) {
					acs.skuCache = resourceskus.NewStaticCache([]armcompute.ResourceSKU{}, cs.Location())
					acs.scope = cs
					acs.Reconcile = func(ctx context.Context) error {
						return azure.WithTransientError(errors.New("failed to reconcile AzureCluster"), 10*time.Second)
					}
				}), nil
			},
			cache:          &scope.ClusterCache{},
			expectedResult: reconcile.Result{},
		},
		"should fail to delete for non-transient errors": {
			createAzureClusterService: func(cs *scope.ClusterScope) (*azureClusterService, error) {
				return getDefaultAzureClusterService(func(acs *azureClusterService) {
					acs.skuCache = resourceskus.NewStaticCache([]armcompute.ResourceSKU{}, cs.Location())
					acs.scope = cs
					acs.Reconcile = func(context.Context) error {
						return errors.New("foo error")
					}
					acs.Pause = func(context.Context) error {
						return errors.New("foo error")
					}
					acs.Delete = func(context.Context) error {
						return errors.New("foo error")
					}
				}), nil
			},
			cache:       &scope.ClusterCache{},
			expectedErr: "error deleting AzureCluster",
		},
	}

	for name, c := range cases {
		tc := c
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)

			reconciler, clusterScope, err := getClusterReconcileInputs(tc)
			g.Expect(err).NotTo(HaveOccurred())

			result, err := reconciler.reconcileDelete(context.Background(), clusterScope)
			g.Expect(result).To(Equal(tc.expectedResult))

			if tc.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func getDefaultAzureClusterService(changes ...func(*azureClusterService)) *azureClusterService {
	input := &azureClusterService{
		services: []azure.ServiceReconciler{},
		Reconcile: func(ctx context.Context) error {
			return nil
		},
		Delete: func(ctx context.Context) error {
			return nil
		},
		Pause: func(ctx context.Context) error {
			return nil
		},
	}

	for _, change := range changes {
		change(input)
	}

	return input
}

func getClusterReconcileInputs(tc TestClusterReconcileInput) (*AzureClusterReconciler, *scope.ClusterScope, error) {
	scheme, err := newScheme()
	if err != nil {
		return nil, nil, err
	}

	cluster := getFakeCluster()

	var azureCluster *infrav1.AzureCluster
	if tc.azureClusterOptions != nil {
		azureCluster = getFakeAzureCluster(tc.azureClusterOptions, func(ac *infrav1.AzureCluster) {
			ac.Spec.Location = location
		})
	} else {
		azureCluster = getFakeAzureCluster(func(ac *infrav1.AzureCluster) {
			ac.Spec.Location = location
		})
	}

	objects := []runtime.Object{
		cluster,
		azureCluster,
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objects...).
		WithStatusSubresource(
			&infrav1.AzureCluster{},
		).
		Build()

	reconciler := &AzureClusterReconciler{
		Client:                    client,
		Recorder:                  record.NewFakeRecorder(128),
		createAzureClusterService: tc.createAzureClusterService,
	}

	clusterScope, err := scope.NewClusterScope(context.Background(), scope.ClusterScopeParams{
		Client:       client,
		Cluster:      cluster,
		AzureCluster: azureCluster,
		Cache:        tc.cache,
	})
	if err != nil {
		return nil, nil, err
	}

	return reconciler, clusterScope, nil
}

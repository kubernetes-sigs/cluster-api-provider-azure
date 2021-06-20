/*
Copyright 2020 The Kubernetes Authors.

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
	"os"
	"testing"
	"time"

	"github.com/Azure/go-autorest/autorest/azure/auth"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mocks"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
	gomock2 "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha4"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAzureMachinePoolMachineReconciler_Reconcile(t *testing.T) {
	cases := []struct {
		Name   string
		Setup  func(cb *fake.ClientBuilder, reconciler *mocks.MockReconcilerMockRecorder)
		Verify func(g *WithT, result ctrl.Result, err error)
	}{
		{
			Name: "should successfully reconcile",
			Setup: func(cb *fake.ClientBuilder, reconciler *mocks.MockReconcilerMockRecorder) {
				cluster, azCluster, mp, amp, ampm := getAReadyMachinePoolMachineCluster()
				reconciler.Reconcile(gomock2.AContext()).Return(nil)
				cb.WithObjects(cluster, azCluster, mp, amp, ampm)
			},
			Verify: func(g *WithT, result ctrl.Result, err error) {
				g.Expect(err).ToNot(HaveOccurred())
			},
		},
		{
			Name: "should successfully delete",
			Setup: func(cb *fake.ClientBuilder, reconciler *mocks.MockReconcilerMockRecorder) {
				cluster, azCluster, mp, amp, ampm := getAReadyMachinePoolMachineCluster()
				ampm.DeletionTimestamp = &metav1.Time{
					Time: time.Now(),
				}
				reconciler.Delete(gomock2.AContext()).Return(nil)
				cb.WithObjects(cluster, azCluster, mp, amp, ampm)
			},
			Verify: func(g *WithT, result ctrl.Result, err error) {
				g.Expect(err).ToNot(HaveOccurred())
			},
		},
	}

	os.Setenv(auth.ClientID, "fooClient")
	os.Setenv(auth.ClientSecret, "fooSecret")
	os.Setenv(auth.TenantID, "fooTenant")

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			var (
				g          = NewWithT(t)
				mockCtrl   = gomock.NewController(t)
				reconciler = mocks.NewMockReconciler(mockCtrl)
				scheme     = func() *runtime.Scheme {
					s := runtime.NewScheme()
					for _, addTo := range []func(s *runtime.Scheme) error{
						clusterv1.AddToScheme,
						clusterv1exp.AddToScheme,
						infrav1.AddToScheme,
						infrav1exp.AddToScheme,
					} {
						g.Expect(addTo(s)).To(Succeed())
					}

					return s
				}()
				cb = fake.NewClientBuilder().WithScheme(scheme)
			)
			defer mockCtrl.Finish()

			c.Setup(cb, reconciler.EXPECT())
			controller := NewAzureMachinePoolMachineController(cb.Build(), klogr.New(), nil, 30*time.Second, "foo")
			controller.reconcilerFactory = func(_ *scope.MachinePoolMachineScope) azure.Reconciler {
				return reconciler
			}
			res, err := controller.Reconcile(context.TODO(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "ampm1",
					Namespace: "default",
				},
			})
			c.Verify(g, res, err)
		})
	}
}

func getAReadyMachinePoolMachineCluster() (*clusterv1.Cluster, *infrav1.AzureCluster, *clusterv1exp.MachinePool, *infrav1exp.AzureMachinePool, *infrav1exp.AzureMachinePoolMachine) {
	azCluster := &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "azCluster1",
			Namespace: "default",
		},
		Spec: infrav1.AzureClusterSpec{
			SubscriptionID: "subID",
		},
	}

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster1",
			Namespace: "default",
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &corev1.ObjectReference{
				Name: azCluster.Name,
			},
		},
		Status: clusterv1.ClusterStatus{
			InfrastructureReady: true,
		},
	}

	mp := &clusterv1exp.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mp1",
			Namespace: "default",
			Labels: map[string]string{
				"cluster.x-k8s.io/cluster-name": cluster.Name,
			},
		},
	}

	amp := &infrav1exp.AzureMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "amp1",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       mp.Name,
					Kind:       "MachinePool",
					APIVersion: clusterv1exp.GroupVersion.String(),
				},
			},
		},
	}

	ampm := &infrav1exp.AzureMachinePoolMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ampm1",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       amp.Name,
					Kind:       "AzureMachinePool",
					APIVersion: infrav1exp.GroupVersion.String(),
				},
			},
		},
	}

	return cluster, azCluster, mp, amp, ampm
}

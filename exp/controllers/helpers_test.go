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

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	kubeadmv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/mock_log"
)

var (
	clusterName = "my-cluster"
)

var _ = Describe("BootstrapConfigToInfrastructureMapFunc", func() {
	It("should map bootstrap config to machine pool", func() {
		ctx := context.Background()
		scheme := runtime.NewScheme()
		Expect(kubeadmv1.AddToScheme(scheme)).Should(Succeed())
		Expect(expv1.AddToScheme(scheme)).Should(Succeed())
		Expect(clusterv1.AddToScheme(scheme)).Should(Succeed())
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
		mapFn := BootstrapConfigToInfrastructureMapFunc(fakeClient, ctrl.Log)
		bootstrapConfig := kubeadmv1.KubeadmConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "bootstrap-test",
				Namespace: "default",
			},
		}
		Expect(fakeClient.Create(ctx, &bootstrapConfig)).Should(Succeed())

		By("doing nothing if the config has no owners")
		Expect(mapFn(ctx, &bootstrapConfig)).Should(Equal([]ctrl.Request{}))

		By("doing nothing if the config has no MachinePool owner")
		bootstrapConfig.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: "cluster.x-k8s.io/v1beta1",
				Name:       "machine-pool-test",
				Kind:       "NotAMachinePool",
				UID:        types.UID("foobar"),
				Controller: ptr.To(true),
			},
		}
		Expect(fakeClient.Update(ctx, &bootstrapConfig)).Should(Succeed())
		Expect(mapFn(ctx, &bootstrapConfig)).Should(Equal([]ctrl.Request{}))

		By("doing nothing if the MachinePool is not found")
		bootstrapConfig.OwnerReferences = []metav1.OwnerReference{
			{
				APIVersion: "cluster.x-k8s.io/v1beta1",
				Name:       "machine-pool-test",
				Kind:       "MachinePool",
				UID:        types.UID("foobar"),
				Controller: ptr.To(true),
			},
		}
		Expect(fakeClient.Update(ctx, &bootstrapConfig)).Should(Succeed())
		Expect(mapFn(ctx, &bootstrapConfig)).Should(Equal([]ctrl.Request{}))

		By("doing nothing if the MachinePool has no BootstrapConfigRef")
		machinePool := expv1.MachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "machine-pool-test",
				Namespace: "default",
			},
			Spec: expv1.MachinePoolSpec{
				ClusterName: "test-cluster",
				Template: clusterv1.MachineTemplateSpec{
					Spec: clusterv1.MachineSpec{
						ClusterName: "test-cluster",
					},
				},
			},
		}
		Expect(fakeClient.Create(ctx, &machinePool)).Should(Succeed())
		Expect(mapFn(ctx, &bootstrapConfig)).Should(Equal([]ctrl.Request{}))

		By("doing nothing if the MachinePool has a different BootstrapConfigRef Kind")
		machinePool.Spec.Template.Spec.Bootstrap = clusterv1.Bootstrap{
			ConfigRef: &corev1.ObjectReference{
				APIVersion: "bootstrap.cluster.x-k8s.io/v1beta1",
				Kind:       "OtherBootstrapConfig",
				Name:       bootstrapConfig.Name,
				Namespace:  bootstrapConfig.Namespace,
			},
		}
		Expect(fakeClient.Update(ctx, &machinePool)).Should(Succeed())
		Expect(mapFn(ctx, &bootstrapConfig)).Should(Equal([]ctrl.Request{}))

		By("doing nothing if the MachinePool has a different BootstrapConfigRef Name")
		machinePool.Spec.Template.Spec.Bootstrap.ConfigRef.Kind = bootstrapConfig.GetObjectKind().GroupVersionKind().Kind
		machinePool.Spec.Template.Spec.Bootstrap.ConfigRef.Name = "other-bootstrap-config"
		Expect(fakeClient.Update(ctx, &machinePool)).Should(Succeed())
		Expect(mapFn(ctx, &bootstrapConfig)).Should(Equal([]ctrl.Request{}))

		By("enqueueing AzureMachinePool")
		machinePool.Spec.Template.Spec.Bootstrap.ConfigRef.Name = bootstrapConfig.Name
		Expect(fakeClient.Update(ctx, &machinePool)).Should(Succeed())
		Expect(mapFn(ctx, &bootstrapConfig)).Should(Equal([]ctrl.Request{
			{
				NamespacedName: client.ObjectKey{Namespace: machinePool.Namespace, Name: machinePool.Name},
			},
		}))
	})
})

func TestAzureClusterToAzureMachinePoolsMapper(t *testing.T) {
	g := NewWithT(t)
	scheme := newScheme(g)
	initObjects := []runtime.Object{
		newCluster(clusterName),
		// Create two Machines with an infrastructure ref and one without.
		newMachinePoolWithInfrastructureRef(clusterName, "my-machine-0"),
		newMachinePoolWithInfrastructureRef(clusterName, "my-machine-1"),
		newMachinePool(clusterName, "my-machine-2"),
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

	sink := mock_log.NewMockLogSink(gomock.NewController(t))
	sink.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
	sink.EXPECT().Enabled(4).Return(true)
	sink.EXPECT().WithValues("AzureCluster", "my-cluster", "Namespace", "default").Return(sink)
	sink.EXPECT().Info(4, "gk does not match", "gk", gomock.Any(), "infraGK", gomock.Any())
	mapper, err := AzureClusterToAzureMachinePoolsMapper(t.Context(), fakeClient, scheme, logr.New(sink))
	g.Expect(err).NotTo(HaveOccurred())

	requests := mapper(t.Context(), &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       clusterName,
					Kind:       "Cluster",
					APIVersion: clusterv1.GroupVersion.String(),
				},
			},
		},
	})
	g.Expect(requests).To(HaveLen(2))
}

func Test_MachinePoolToInfrastructureMapFunc(t *testing.T) {
	cases := []struct {
		Name             string
		Setup            func(logMock *mock_log.MockLogSink)
		MapObjectFactory func(*GomegaWithT) client.Object
		Expect           func(*GomegaWithT, []reconcile.Request)
	}{
		{
			Name: "MachinePoolToAzureMachinePool",
			MapObjectFactory: func(g *GomegaWithT) client.Object {
				return newMachinePoolWithInfrastructureRef("azureCluster", "machinePool")
			},
			Setup: func(logMock *mock_log.MockLogSink) {
				logMock.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
			},
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(HaveLen(1))
				g.Expect(reqs[0]).To(Equal(reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "azuremachinePool",
						Namespace: "default",
					},
				}))
			},
		},
		{
			Name: "MachinePoolWithoutMatchingInfraRef",
			MapObjectFactory: func(g *GomegaWithT) client.Object {
				return newMachinePool("azureCluster", "machinePool")
			},
			Setup: func(logMock *mock_log.MockLogSink) {
				ampGK := infrav1exp.GroupVersion.WithKind(infrav1.AzureMachinePoolKind).GroupKind()
				logMock.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
				logMock.EXPECT().Enabled(4).Return(true)
				logMock.EXPECT().Info(4, "gk does not match", "gk", ampGK, "infraGK", gomock.Any())
			},
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(BeEmpty())
			},
		},
		{
			Name: "NotAMachinePool",
			MapObjectFactory: func(g *GomegaWithT) client.Object {
				return newCluster("azureCluster")
			},
			Setup: func(logMock *mock_log.MockLogSink) {
				logMock.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
				logMock.EXPECT().Enabled(4).Return(true)
				logMock.EXPECT().Info(4, "attempt to map incorrect type", "type", "*v1beta1.Cluster")
			},
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(BeEmpty())
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			sink := mock_log.NewMockLogSink(mockCtrl)
			if c.Setup != nil {
				c.Setup(sink)
			}
			f := MachinePoolToInfrastructureMapFunc(infrav1exp.GroupVersion.WithKind(infrav1.AzureMachinePoolKind), logr.New(sink))
			reqs := f(t.Context(), c.MapObjectFactory(g))
			c.Expect(g, reqs)
		})
	}
}

func Test_azureClusterToAzureMachinePoolsFunc(t *testing.T) {
	cases := []struct {
		Name             string
		Setup            func(*testing.T, *GomegaWithT) (*mock_log.MockLogSink, *gomock.Controller, client.Client)
		MapObjectFactory func(*GomegaWithT) client.Object
		Expect           func(*GomegaWithT, []reconcile.Request)
	}{
		{
			Name: "NotAnAzureCluster",
			MapObjectFactory: func(g *GomegaWithT) client.Object {
				return newMachinePool("fakeCluster", "bar")
			},
			Setup: func(t *testing.T, g *GomegaWithT) (*mock_log.MockLogSink, *gomock.Controller, client.Client) {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				sink := mock_log.NewMockLogSink(mockCtrl)
				fakeClient := fake.NewClientBuilder().WithScheme(newScheme(g)).Build()
				sink.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
				sink.EXPECT().Error(gomockinternal.ErrStrEq("expected a AzureCluster but got a *v1beta1.MachinePool"), "failed to get AzureCluster")

				return sink, mockCtrl, fakeClient
			},
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(BeEmpty())
			},
		},
		{
			Name: "AzureClusterDoesNotExist",
			MapObjectFactory: func(g *GomegaWithT) client.Object {
				return newAzureCluster("foo")
			},
			Setup: func(t *testing.T, g *GomegaWithT) (*mock_log.MockLogSink, *gomock.Controller, client.Client) {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				sink := mock_log.NewMockLogSink(mockCtrl)
				fakeClient := fake.NewClientBuilder().WithScheme(newScheme(g)).Build()
				sink.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
				sink.EXPECT().Enabled(4).Return(true)
				sink.EXPECT().WithValues("AzureCluster", "azurefoo", "Namespace", "default").Return(sink)
				sink.EXPECT().Info(4, "owning cluster not found")
				return sink, mockCtrl, fakeClient
			},
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(BeEmpty())
			},
		},
		{
			Name: "AzureClusterExistsButDoesNotHaveMachinePools",
			MapObjectFactory: func(g *GomegaWithT) client.Object {
				return newAzureCluster("foo")
			},
			Setup: func(t *testing.T, g *GomegaWithT) (*mock_log.MockLogSink, *gomock.Controller, client.Client) {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				sink := mock_log.NewMockLogSink(mockCtrl)
				logWithValues := mock_log.NewMockLogSink(mockCtrl)
				const clusterName = "foo"
				initObj := []runtime.Object{
					newCluster(clusterName),
					newAzureCluster(clusterName),
				}
				fakeClient := fake.NewClientBuilder().WithScheme(newScheme(g)).WithRuntimeObjects(initObj...).Build()
				sink.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
				sink.EXPECT().WithValues("AzureCluster", "azurefoo", "Namespace", "default").Return(logWithValues)
				return sink, mockCtrl, fakeClient
			},
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(BeEmpty())
			},
		},
		{
			Name: "AzureClusterExistsWithMachinePoolsButNoInfraRefs",
			MapObjectFactory: func(g *GomegaWithT) client.Object {
				return newAzureCluster("foo")
			},
			Setup: func(t *testing.T, g *GomegaWithT) (*mock_log.MockLogSink, *gomock.Controller, client.Client) {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				sink := mock_log.NewMockLogSink(mockCtrl)
				logWithValues := mock_log.NewMockLogSink(mockCtrl)
				const clusterName = "foo"
				initObj := []runtime.Object{
					newCluster(clusterName),
					newAzureCluster(clusterName),
					newMachinePool(clusterName, "pool1"),
					newMachinePool(clusterName, "pool2"),
				}
				fakeClient := fake.NewClientBuilder().WithScheme(newScheme(g)).WithRuntimeObjects(initObj...).Build()
				sink.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
				sink.EXPECT().WithValues("AzureCluster", "azurefoo", "Namespace", "default").Return(logWithValues)
				return sink, mockCtrl, fakeClient
			},
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(BeEmpty())
			},
		},
		{
			Name: "AzureClusterExistsWithMachinePoolsWithOneInfraRefs",
			MapObjectFactory: func(g *GomegaWithT) client.Object {
				return newAzureCluster("foo")
			},
			Setup: func(t *testing.T, g *GomegaWithT) (*mock_log.MockLogSink, *gomock.Controller, client.Client) {
				t.Helper()
				mockCtrl := gomock.NewController(t)
				sink := mock_log.NewMockLogSink(mockCtrl)
				logWithValues := mock_log.NewMockLogSink(mockCtrl)
				const clusterName = "foo"
				initObj := []runtime.Object{
					newCluster(clusterName),
					newAzureCluster(clusterName),
					newMachinePool(clusterName, "pool1"),
					newAzureMachinePool(clusterName, "azurepool2"),
					newMachinePoolWithInfrastructureRef(clusterName, "pool2"),
				}
				fakeClient := fake.NewClientBuilder().WithScheme(newScheme(g)).WithRuntimeObjects(initObj...).Build()
				sink.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
				sink.EXPECT().WithValues("AzureCluster", "azurefoo", "Namespace", "default").Return(logWithValues)
				return sink, mockCtrl, fakeClient
			},
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(HaveLen(1))
				g.Expect(reqs[0]).To(Equal(reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "azurepool2",
						Namespace: "default",
					},
				}))
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := NewGomegaWithT(t)
			sink, mockctrl, fakeClient := c.Setup(t, g)
			defer mockctrl.Finish()

			f := AzureClusterToAzureMachinePoolsFunc(t.Context(), fakeClient, logr.New(sink))
			reqs := f(t.Context(), c.MapObjectFactory(g))
			c.Expect(g, reqs)
		})
	}
}

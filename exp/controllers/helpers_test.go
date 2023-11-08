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

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-logr/logr"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/mock_log"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	cpName      = "my-managed-cp"
	clusterName = "my-cluster"
)

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
	mapper, err := AzureClusterToAzureMachinePoolsMapper(context.Background(), fakeClient, scheme, logr.New(sink))
	g.Expect(err).NotTo(HaveOccurred())

	requests := mapper(&infrav1.AzureCluster{
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

func TestAzureManagedClusterToAzureManagedMachinePoolsMapper(t *testing.T) {
	g := NewWithT(t)
	scheme := newScheme(g)
	initObjects := []runtime.Object{
		newCluster(clusterName),
		// Create two Machines with an infrastructure ref and one without.
		newManagedMachinePoolInfraReference(clusterName, "my-mmp-0"),
		newManagedMachinePoolInfraReference(clusterName, "my-mmp-1"),
		newManagedMachinePoolInfraReference(clusterName, "my-mmp-2"),
		newMachinePool(clusterName, "my-machine-2"),
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

	sink := mock_log.NewMockLogSink(gomock.NewController(t))
	sink.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
	sink.EXPECT().Enabled(4).Return(true)
	sink.EXPECT().WithValues("AzureManagedCluster", "my-cluster", "Namespace", "default").Return(sink)
	sink.EXPECT().Info(4, "gk does not match", "gk", gomock.Any(), "infraGK", gomock.Any())
	mapper, err := AzureManagedClusterToAzureManagedMachinePoolsMapper(context.Background(), fakeClient, scheme, logr.New(sink))
	g.Expect(err).NotTo(HaveOccurred())

	requests := mapper(&infrav1exp.AzureManagedCluster{
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
	g.Expect(requests).To(ConsistOf([]reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      "azuremy-mmp-0",
				Namespace: "default",
			},
		},
		{
			NamespacedName: types.NamespacedName{
				Name:      "azuremy-mmp-1",
				Namespace: "default",
			},
		},
		{
			NamespacedName: types.NamespacedName{
				Name:      "azuremy-mmp-2",
				Namespace: "default",
			},
		},
	}))
}

func TestAzureManagedControlPlaneToAzureManagedMachinePoolsMapper(t *testing.T) {
	g := NewWithT(t)
	scheme := newScheme(g)
	cluster := newCluster("my-cluster")
	cluster.Spec.ControlPlaneRef = &corev1.ObjectReference{
		APIVersion: infrav1exp.GroupVersion.String(),
		Kind:       "AzureManagedControlPlane",
		Name:       cpName,
		Namespace:  cluster.Namespace,
	}
	initObjects := []runtime.Object{
		cluster,
		newAzureManagedControlPlane(cpName),
		// Create two Machines with an infrastructure ref and one without.
		newManagedMachinePoolInfraReference(clusterName, "my-mmp-0"),
		newManagedMachinePoolInfraReference(clusterName, "my-mmp-1"),
		newManagedMachinePoolInfraReference(clusterName, "my-mmp-2"),
		newMachinePool(clusterName, "my-machine-2"),
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

	sink := mock_log.NewMockLogSink(gomock.NewController(t))
	sink.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
	sink.EXPECT().Enabled(4).Return(true)
	sink.EXPECT().WithValues("AzureManagedControlPlane", cpName, "Namespace", cluster.Namespace).Return(sink)
	sink.EXPECT().Info(4, "gk does not match", "gk", gomock.Any(), "infraGK", gomock.Any())
	mapper, err := AzureManagedControlPlaneToAzureManagedMachinePoolsMapper(context.Background(), fakeClient, scheme, logr.New(sink))
	g.Expect(err).NotTo(HaveOccurred())

	requests := mapper(&infrav1exp.AzureManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cpName,
			Namespace: cluster.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       cluster.Name,
					Kind:       "Cluster",
					APIVersion: clusterv1.GroupVersion.String(),
				},
			},
		},
	})
	g.Expect(requests).To(ConsistOf([]reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      "azuremy-mmp-0",
				Namespace: "default",
			},
		},
		{
			NamespacedName: types.NamespacedName{
				Name:      "azuremy-mmp-1",
				Namespace: "default",
			},
		},
		{
			NamespacedName: types.NamespacedName{
				Name:      "azuremy-mmp-2",
				Namespace: "default",
			},
		},
	}))
}

func TestMachinePoolToAzureManagedControlPlaneMapFuncSuccess(t *testing.T) {
	g := NewWithT(t)
	scheme := newScheme(g)
	cluster := newCluster(clusterName)
	controlPlane := newAzureManagedControlPlane(cpName)
	cluster.Spec.ControlPlaneRef = &corev1.ObjectReference{
		APIVersion: infrav1exp.GroupVersion.String(),
		Kind:       "AzureManagedControlPlane",
		Name:       cpName,
		Namespace:  cluster.Namespace,
	}

	// controlPlane.Spec.DefaultPoolRef.Name = "azuremy-mmp-0"
	managedMachinePool0 := newManagedMachinePoolInfraReference(clusterName, "my-mmp-0")
	azureManagedMachinePool0 := newAzureManagedMachinePool(clusterName, "azuremy-mmp-0", "System")
	managedMachinePool0.Spec.ClusterName = clusterName

	managedMachinePool1 := newManagedMachinePoolInfraReference(clusterName, "my-mmp-1")
	azureManagedMachinePool1 := newAzureManagedMachinePool(clusterName, "azuremy-mmp-1", "User")
	managedMachinePool1.Spec.ClusterName = clusterName

	initObjects := []runtime.Object{
		cluster,
		controlPlane,
		managedMachinePool0,
		azureManagedMachinePool0,
		// Create two Machines with an infrastructure ref and one without.
		managedMachinePool1,
		azureManagedMachinePool1,
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

	sink := mock_log.NewMockLogSink(gomock.NewController(t))
	sink.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
	mapper := MachinePoolToAzureManagedControlPlaneMapFunc(context.Background(), fakeClient, infrav1exp.GroupVersion.WithKind("AzureManagedControlPlane"), logr.New(sink))

	// system pool should trigger
	requests := mapper(newManagedMachinePoolInfraReference(clusterName, "my-mmp-0"))
	g.Expect(requests).To(ConsistOf([]reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      "my-managed-cp",
				Namespace: "default",
			},
		},
	}))

	// any other pool should not trigger
	requests = mapper(newManagedMachinePoolInfraReference(clusterName, "my-mmp-1"))
	g.Expect(requests).To(BeNil())
}

func TestMachinePoolToAzureManagedControlPlaneMapFuncFailure(t *testing.T) {
	g := NewWithT(t)
	scheme := newScheme(g)
	cluster := newCluster(clusterName)
	cluster.Spec.ControlPlaneRef = &corev1.ObjectReference{
		APIVersion: infrav1exp.GroupVersion.String(),
		Kind:       "AzureManagedControlPlane",
		Name:       cpName,
		Namespace:  cluster.Namespace,
	}
	managedMachinePool := newManagedMachinePoolInfraReference(clusterName, "my-mmp-0")
	managedMachinePool.Spec.ClusterName = clusterName
	initObjects := []runtime.Object{
		cluster,
		managedMachinePool,
		// Create two Machines with an infrastructure ref and one without.
		newManagedMachinePoolInfraReference(clusterName, "my-mmp-1"),
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

	sink := mock_log.NewMockLogSink(gomock.NewController(t))
	sink.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
	sink.EXPECT().Error(gomock.Any(), "failed to fetch default pool reference")
	sink.EXPECT().Error(gomock.Any(), "failed to fetch default pool reference") // twice because we are testing two calls

	mapper := MachinePoolToAzureManagedControlPlaneMapFunc(context.Background(), fakeClient, infrav1exp.GroupVersion.WithKind("AzureManagedControlPlane"), logr.New(sink))

	// default pool should trigger if owned cluster could not be fetched
	requests := mapper(newManagedMachinePoolInfraReference(clusterName, "my-mmp-0"))
	g.Expect(requests).To(ConsistOf([]reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      "my-managed-cp",
				Namespace: "default",
			},
		},
	}))

	// any other pool should also trigger if owned cluster could not be fetched
	requests = mapper(newManagedMachinePoolInfraReference(clusterName, "my-mmp-1"))
	g.Expect(requests).To(ConsistOf([]reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      "my-managed-cp",
				Namespace: "default",
			},
		},
	}))
}

func TestAzureManagedClusterToAzureManagedControlPlaneMapper(t *testing.T) {
	g := NewWithT(t)
	scheme := newScheme(g)
	cluster := newCluster("my-cluster")
	cluster.Spec.ControlPlaneRef = &corev1.ObjectReference{
		APIVersion: infrav1exp.GroupVersion.String(),
		Kind:       "AzureManagedControlPlane",
		Name:       cpName,
		Namespace:  cluster.Namespace,
	}

	initObjects := []runtime.Object{
		cluster,
		newAzureManagedControlPlane(cpName),
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

	sink := mock_log.NewMockLogSink(gomock.NewController(t))
	sink.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
	sink.EXPECT().WithValues("AzureManagedCluster", "az-"+cluster.Name, "Namespace", "default")

	mapper, err := AzureManagedClusterToAzureManagedControlPlaneMapper(context.Background(), fakeClient, logr.New(sink))
	g.Expect(err).NotTo(HaveOccurred())
	requests := mapper(&infrav1exp.AzureManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "az-" + cluster.Name,
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       cluster.Name,
					Kind:       "Cluster",
					APIVersion: clusterv1.GroupVersion.String(),
				},
			},
		},
	})
	g.Expect(requests).To(HaveLen(1))
	g.Expect(requests).To(Equal([]reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      cpName,
				Namespace: cluster.Namespace,
			},
		},
	}))
}

func TestClusterToAzureManagedControlPlaneMapper(t *testing.T) {
	g := NewWithT(t)
	cluster := newCluster("my-cluster")
	cluster.Spec.ControlPlaneRef = &corev1.ObjectReference{
		APIVersion: infrav1exp.GroupVersion.String(),
		Kind:       "AzureManagedControlPlane",
		Name:       cpName,
		Namespace:  cluster.Namespace,
	}

	sink := mock_log.NewMockLogSink(gomock.NewController(t))
	sink.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
	sink.EXPECT().WithValues("Cluster", cluster.Name, "Namespace", "default")

	mapper := ClusterToAzureManagedControlPlaneMapper(logr.New(sink))
	requests := mapper(cluster)
	g.Expect(requests).To(HaveLen(1))
	g.Expect(requests).To(Equal([]reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      cpName,
				Namespace: cluster.Namespace,
			},
		},
	}))
}

func TestAzureManagedControlPlaneToAzureManagedClusterMapper(t *testing.T) {
	g := NewWithT(t)
	scheme := newScheme(g)
	cluster := newCluster("my-cluster")
	azManagedCluster := &infrav1exp.AzureManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "az-" + cluster.Name,
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       cluster.Name,
					Kind:       "Cluster",
					APIVersion: clusterv1.GroupVersion.String(),
				},
			},
		},
	}

	cluster.Spec.ControlPlaneRef = &corev1.ObjectReference{
		APIVersion: infrav1exp.GroupVersion.String(),
		Kind:       "AzureManagedControlPlane",
		Name:       cpName,
		Namespace:  cluster.Namespace,
	}
	cluster.Spec.InfrastructureRef = &corev1.ObjectReference{
		APIVersion: infrav1exp.GroupVersion.String(),
		Kind:       "AzureManagedCluster",
		Name:       azManagedCluster.Name,
		Namespace:  azManagedCluster.Namespace,
	}

	initObjects := []runtime.Object{
		cluster,
		newAzureManagedControlPlane(cpName),
		azManagedCluster,
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

	sink := mock_log.NewMockLogSink(gomock.NewController(t))
	sink.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
	sink.EXPECT().WithValues("AzureManagedControlPlane", cpName, "Namespace", cluster.Namespace)

	mapper, err := AzureManagedControlPlaneToAzureManagedClusterMapper(context.Background(), fakeClient, logr.New(sink))
	g.Expect(err).NotTo(HaveOccurred())
	requests := mapper(&infrav1exp.AzureManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cpName,
			Namespace: cluster.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       cluster.Name,
					Kind:       "Cluster",
					APIVersion: clusterv1.GroupVersion.String(),
				},
			},
		},
	})
	g.Expect(requests).To(HaveLen(1))
	g.Expect(requests).To(Equal([]reconcile.Request{
		{
			NamespacedName: types.NamespacedName{
				Name:      azManagedCluster.Name,
				Namespace: azManagedCluster.Namespace,
			},
		},
	}))
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
				ampGK := infrav1exp.GroupVersion.WithKind("AzureMachinePool").GroupKind()
				logMock.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
				logMock.EXPECT().Enabled(4).Return(true)
				logMock.EXPECT().Info(4, "gk does not match", "gk", ampGK, "infraGK", gomock.Any())
			},
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(HaveLen(0))
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
				g.Expect(reqs).To(HaveLen(0))
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			sink := mock_log.NewMockLogSink(mockCtrl)
			if c.Setup != nil {
				c.Setup(sink)
			}
			f := MachinePoolToInfrastructureMapFunc(infrav1exp.GroupVersion.WithKind("AzureMachinePool"), logr.New(sink))
			reqs := f(c.MapObjectFactory(g))
			c.Expect(g, reqs)
		})
	}
}

func Test_ManagedMachinePoolToInfrastructureMapFunc(t *testing.T) {
	cases := []struct {
		Name             string
		Setup            func(logMock *mock_log.MockLogSink)
		MapObjectFactory func(*GomegaWithT) client.Object
		Expect           func(*GomegaWithT, []reconcile.Request)
	}{
		{
			Name: "MachinePoolToAzureManagedMachinePool",
			MapObjectFactory: func(g *GomegaWithT) client.Object {
				return newManagedMachinePoolWithInfrastructureRef("azureManagedCluster", "ManagedMachinePool")
			},
			Setup: func(logMock *mock_log.MockLogSink) {
				logMock.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
			},
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(HaveLen(1))
				g.Expect(reqs[0]).To(Equal(reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "azureManagedMachinePool",
						Namespace: "default",
					},
				}))
			},
		},
		{
			Name: "MachinePoolWithoutMatchingInfraRef",
			MapObjectFactory: func(g *GomegaWithT) client.Object {
				return newMachinePool("azureManagedCluster", "machinePool")
			},
			Setup: func(logMock *mock_log.MockLogSink) {
				ampGK := infrav1exp.GroupVersion.WithKind("AzureManagedMachinePool").GroupKind()
				logMock.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
				logMock.EXPECT().Enabled(4).Return(true)
				logMock.EXPECT().Info(4, "gk does not match", "gk", ampGK, "infraGK", gomock.Any())
			},
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(HaveLen(0))
			},
		},
		{
			Name: "NotAMachinePool",
			MapObjectFactory: func(g *GomegaWithT) client.Object {
				return newCluster("azureManagedCluster")
			},
			Setup: func(logMock *mock_log.MockLogSink) {
				logMock.EXPECT().Init(logr.RuntimeInfo{CallDepth: 1})
				logMock.EXPECT().Enabled(4).Return(true)
				logMock.EXPECT().Info(4, "attempt to map incorrect type", "type", "*v1beta1.Cluster")
			},
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(HaveLen(0))
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			g := NewWithT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			sink := mock_log.NewMockLogSink(mockCtrl)
			if c.Setup != nil {
				c.Setup(sink)
			}
			f := MachinePoolToInfrastructureMapFunc(infrav1exp.GroupVersion.WithKind("AzureManagedMachinePool"), logr.New(sink))
			reqs := f(c.MapObjectFactory(g))
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
				g.Expect(reqs).To(HaveLen(0))
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
				g.Expect(reqs).To(HaveLen(0))
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
				g.Expect(reqs).To(HaveLen(0))
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
				g.Expect(reqs).To(HaveLen(0))
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
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := NewGomegaWithT(t)
			sink, mockctrl, fakeClient := c.Setup(t, g)
			defer mockctrl.Finish()

			f := AzureClusterToAzureMachinePoolsFunc(context.Background(), fakeClient, logr.New(sink))
			reqs := f(c.MapObjectFactory(g))
			c.Expect(g, reqs)
		})
	}
}

func newAzureManagedControlPlane(cpName string) *infrav1exp.AzureManagedControlPlane {
	return &infrav1exp.AzureManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cpName,
			Namespace: "default",
		},
	}
}

func newManagedMachinePoolInfraReference(clusterName, poolName string) *clusterv1exp.MachinePool {
	m := newMachinePool(clusterName, poolName)
	m.Spec.ClusterName = clusterName
	m.Spec.Template.Spec.InfrastructureRef = corev1.ObjectReference{
		Kind:       "AzureManagedMachinePool",
		Namespace:  m.Namespace,
		Name:       "azure" + poolName,
		APIVersion: infrav1exp.GroupVersion.String(),
	}
	return m
}

func newAzureManagedMachinePool(clusterName, poolName, mode string) *infrav1exp.AzureManagedMachinePool {
	return &infrav1exp.AzureManagedMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				clusterv1.ClusterLabelName: clusterName,
			},
			Name:      poolName,
			Namespace: "default",
		},
		Spec: infrav1exp.AzureManagedMachinePoolSpec{
			Mode:         mode,
			SKU:          "Standard_D2s_v3",
			OSDiskSizeGB: to.Int32Ptr(512),
		},
	}
}

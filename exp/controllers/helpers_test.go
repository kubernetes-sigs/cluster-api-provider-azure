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
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/mock_log"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
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

	requests := mapper(context.Background(), &infrav1.AzureCluster{
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
			MapObjectFactory: func(_ *GomegaWithT) client.Object {
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
			MapObjectFactory: func(_ *GomegaWithT) client.Object {
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
			MapObjectFactory: func(_ *GomegaWithT) client.Object {
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
			reqs := f(context.TODO(), c.MapObjectFactory(g))
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
			MapObjectFactory: func(_ *GomegaWithT) client.Object {
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
			MapObjectFactory: func(_ *GomegaWithT) client.Object {
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
			MapObjectFactory: func(_ *GomegaWithT) client.Object {
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
			MapObjectFactory: func(_ *GomegaWithT) client.Object {
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
			MapObjectFactory: func(_ *GomegaWithT) client.Object {
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

			f := AzureClusterToAzureMachinePoolsFunc(context.Background(), fakeClient, logr.New(sink))
			reqs := f(context.TODO(), c.MapObjectFactory(g))
			c.Expect(g, reqs)
		})
	}
}

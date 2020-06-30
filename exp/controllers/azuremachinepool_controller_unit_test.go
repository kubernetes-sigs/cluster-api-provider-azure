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
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/scalesets/mock_scalesets"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/mock_log"
)

func Test_machinePoolToInfrastructureMapFunc(t *testing.T) {
	cases := []struct {
		Name             string
		Setup            func(logMock *mock_log.MockLogger)
		MapObjectFactory func(*gomega.GomegaWithT) handler.MapObject
		Expect           func(*gomega.GomegaWithT, []reconcile.Request)
	}{
		{
			Name: "MachinePoolToAzureMachinePool",
			MapObjectFactory: func(g *gomega.GomegaWithT) handler.MapObject {
				return handler.MapObject{
					Object: newMachinePoolWithInfrastructureRef("azureCluster", "machinePool"),
				}
			},
			Expect: func(g *gomega.GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(gomega.HaveLen(1))
				g.Expect(reqs[0]).To(gomega.Equal(reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "azuremachinePool",
						Namespace: "default",
					},
				}))
			},
		},
		{
			Name: "MachinePoolWithoutMatchingInfraRef",
			MapObjectFactory: func(g *gomega.GomegaWithT) handler.MapObject {
				return handler.MapObject{
					Object: newMachinePool("azureCluster", "machinePool"),
				}
			},
			Setup: func(logMock *mock_log.MockLogger) {
				ampGK := infrav1exp.GroupVersion.WithKind("AzureMachinePool").GroupKind()
				logMock.EXPECT().Info("gk does not match", "gk", ampGK, "infraGK", gomock.Any())
			},
			Expect: func(g *gomega.GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(gomega.HaveLen(0))
			},
		},
		{
			Name: "NotAMachinePool",
			MapObjectFactory: func(g *gomega.GomegaWithT) handler.MapObject {
				return handler.MapObject{
					Object: newCluster("azureCluster"),
				}
			},
			Setup: func(logMock *mock_log.MockLogger) {
				logMock.EXPECT().Info("attempt to map incorrect type", "type", "*v1alpha3.Cluster")
			},
			Expect: func(g *gomega.GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(gomega.HaveLen(0))
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewGomegaWithT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			log := mock_log.NewMockLogger(mockCtrl)
			if c.Setup != nil {
				c.Setup(log)
			}
			f := machinePoolToInfrastructureMapFunc(infrav1exp.GroupVersion.WithKind("AzureMachinePool"), log)
			reqs := f(c.MapObjectFactory(g))
			c.Expect(g, reqs)
		})
	}
}

func Test_azureClusterToAzureMachinePoolsFunc(t *testing.T) {
	cases := []struct {
		Name             string
		Setup            func(*gomega.GomegaWithT, *testing.T) (*mock_log.MockLogger, *gomock.Controller, client.Client)
		MapObjectFactory func(*gomega.GomegaWithT) handler.MapObject
		Expect           func(*gomega.GomegaWithT, []reconcile.Request)
	}{
		{
			Name: "NotAnAzureCluster",
			MapObjectFactory: func(g *gomega.GomegaWithT) handler.MapObject {
				return handler.MapObject{
					Object: newMachinePool("foo", "bar"),
				}
			},
			Setup: func(g *gomega.GomegaWithT, t *testing.T) (*mock_log.MockLogger, *gomock.Controller, client.Client) {
				mockCtrl := gomock.NewController(t)
				log := mock_log.NewMockLogger(mockCtrl)
				kClient := fake.NewFakeClientWithScheme(newScheme(g))
				log.EXPECT().Error(matchers.ErrStrEq("expected a AzureCluster but got a *v1alpha3.MachinePool"), "failed to get AzureCluster")

				return log, mockCtrl, kClient
			},
			Expect: func(g *gomega.GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(gomega.HaveLen(0))
			},
		},
		{
			Name: "AzureClusterDoesNotExist",
			MapObjectFactory: func(g *gomega.GomegaWithT) handler.MapObject {
				return handler.MapObject{
					Object: newAzureCluster("foo"),
				}
			},
			Setup: func(g *gomega.GomegaWithT, t *testing.T) (*mock_log.MockLogger, *gomock.Controller, client.Client) {
				mockCtrl := gomock.NewController(t)
				log := mock_log.NewMockLogger(mockCtrl)
				logWithValues := mock_log.NewMockLogger(mockCtrl)
				kClient := fake.NewFakeClientWithScheme(newScheme(g))
				log.EXPECT().WithValues("AzureCluster", "azurefoo", "Namespace", "default").Return(logWithValues)
				logWithValues.EXPECT().Info("owning cluster not found")
				return log, mockCtrl, kClient
			},
			Expect: func(g *gomega.GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(gomega.HaveLen(0))
			},
		},
		{
			Name: "AzureClusterExistsButDoesNotHaveMachinePools",
			MapObjectFactory: func(g *gomega.GomegaWithT) handler.MapObject {
				return handler.MapObject{
					Object: newAzureCluster("foo"),
				}
			},
			Setup: func(g *gomega.GomegaWithT, t *testing.T) (*mock_log.MockLogger, *gomock.Controller, client.Client) {
				mockCtrl := gomock.NewController(t)
				log := mock_log.NewMockLogger(mockCtrl)
				logWithValues := mock_log.NewMockLogger(mockCtrl)
				const clusterName = "foo"
				initObj := []runtime.Object{
					newCluster(clusterName),
					newAzureCluster(clusterName),
				}
				kClient := fake.NewFakeClientWithScheme(newScheme(g), initObj...)
				log.EXPECT().WithValues("AzureCluster", "azurefoo", "Namespace", "default").Return(logWithValues)
				return log, mockCtrl, kClient
			},
			Expect: func(g *gomega.GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(gomega.HaveLen(0))
			},
		},
		{
			Name: "AzureClusterExistsWithMachinePoolsButNoInfraRefs",
			MapObjectFactory: func(g *gomega.GomegaWithT) handler.MapObject {
				return handler.MapObject{
					Object: newAzureCluster("foo"),
				}
			},
			Setup: func(g *gomega.GomegaWithT, t *testing.T) (*mock_log.MockLogger, *gomock.Controller, client.Client) {
				mockCtrl := gomock.NewController(t)
				log := mock_log.NewMockLogger(mockCtrl)
				logWithValues := mock_log.NewMockLogger(mockCtrl)
				const clusterName = "foo"
				initObj := []runtime.Object{
					newCluster(clusterName),
					newAzureCluster(clusterName),
					newMachinePool(clusterName, "pool1"),
					newMachinePool(clusterName, "pool2"),
				}
				kClient := fake.NewFakeClientWithScheme(newScheme(g), initObj...)
				log.EXPECT().WithValues("AzureCluster", "azurefoo", "Namespace", "default").Return(logWithValues)
				return log, mockCtrl, kClient
			},
			Expect: func(g *gomega.GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(gomega.HaveLen(0))
			},
		},
		{
			Name: "AzureClusterExistsWithMachinePoolsWithOneInfraRefs",
			MapObjectFactory: func(g *gomega.GomegaWithT) handler.MapObject {
				return handler.MapObject{
					Object: newAzureCluster("foo"),
				}
			},
			Setup: func(g *gomega.GomegaWithT, t *testing.T) (*mock_log.MockLogger, *gomock.Controller, client.Client) {
				mockCtrl := gomock.NewController(t)
				log := mock_log.NewMockLogger(mockCtrl)
				logWithValues := mock_log.NewMockLogger(mockCtrl)
				const clusterName = "foo"
				initObj := []runtime.Object{
					newCluster(clusterName),
					newAzureCluster(clusterName),
					newMachinePool(clusterName, "pool1"),
					newMachinePoolWithInfrastructureRef(clusterName, "pool2"),
				}
				kClient := fake.NewFakeClientWithScheme(newScheme(g), initObj...)
				log.EXPECT().WithValues("AzureCluster", "azurefoo", "Namespace", "default").Return(logWithValues)
				return log, mockCtrl, kClient
			},
			Expect: func(g *gomega.GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(gomega.HaveLen(1))
				g.Expect(reqs[0]).To(gomega.Equal(reconcile.Request{
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
			g := gomega.NewGomegaWithT(t)
			log, mockctrl, kClient := c.Setup(g, t)
			defer mockctrl.Finish()

			f := azureClusterToAzureMachinePoolsFunc(kClient, log)
			reqs := f(c.MapObjectFactory(g))
			c.Expect(g, reqs)
		})
	}
}

func Test_newAzureMachinePoolService(t *testing.T) {
	cluster := newAzureCluster("foo")
	cluster.Spec.ResourceGroup = "resourceGroup"
	cs := &scope.ClusterScope{
		AzureCluster: cluster,
	}

	mps := &scope.MachinePoolScope{
		ClusterScope: cs,
		AzureMachinePool: &infrav1exp.AzureMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name: "poolName",
			},
		},
	}

	subject := newAzureMachinePoolService(mps, cs)
	mockCtrl := gomock.NewController(t)
	svcMock := mock_scalesets.NewMockClient(mockCtrl)
	svcMock.EXPECT().Delete(gomock.Any(), "resourceGroup", "poolName").Return(nil)
	defer mockCtrl.Finish()
	subject.virtualMachinesScaleSetSvc.Client = svcMock
	g := gomega.NewWithT(t)
	g.Expect(subject.Delete(context.Background())).ToNot(gomega.HaveOccurred())
}

func newScheme(g *gomega.GomegaWithT) *runtime.Scheme {
	scheme := runtime.NewScheme()
	for _, f := range []func(*runtime.Scheme) error{
		clusterv1.AddToScheme,
		clusterv1exp.AddToScheme,
		infrav1.AddToScheme,
		infrav1exp.AddToScheme,
	} {
		g.Expect(f(scheme)).ToNot(gomega.HaveOccurred())
	}
	return scheme
}

func newMachinePool(clusterName, poolName string) *clusterv1exp.MachinePool {
	return &clusterv1exp.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				clusterv1.ClusterLabelName: clusterName,
			},
			Name:      poolName,
			Namespace: "default",
		},
	}
}

func newMachinePoolWithInfrastructureRef(clusterName, poolName string) *clusterv1exp.MachinePool {
	m := newMachinePool(clusterName, poolName)
	m.Spec.Template.Spec.InfrastructureRef = v1.ObjectReference{
		Kind:       "AzureMachinePool",
		Namespace:  m.Namespace,
		Name:       "azure" + poolName,
		APIVersion: infrav1exp.GroupVersion.String(),
	}
	return m
}

func newAzureCluster(clusterName string) *infrav1.AzureCluster {
	return &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "azure" + clusterName,
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
					Name:       clusterName,
				},
			},
		},
	}
}

func newCluster(name string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
	}
}

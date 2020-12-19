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
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/mock_log"
)

func TestAzureClusterToAzureMachinePoolsMapper(t *testing.T) {
	g := NewWithT(t)
	scheme := newScheme(g)
	clusterName := "my-cluster"
	initObjects := []runtime.Object{
		newCluster(clusterName),
		// Create two Machines with an infrastructure ref and one without.
		newMachinePoolWithInfrastructureRef(clusterName, "my-machine-0"),
		newMachinePoolWithInfrastructureRef(clusterName, "my-machine-1"),
		newMachinePool(clusterName, "my-machine-2"),
	}
	fakeClient := fake.NewFakeClientWithScheme(scheme, initObjects...)

	log := mock_log.NewMockLogger(gomock.NewController(t))
	log.EXPECT().WithValues("AzureCluster", "my-cluster", "Namespace", "default").Return(log)
	log.EXPECT().Info("gk does not match", "gk", gomock.Any(), "infraGK", gomock.Any())
	mapper, err := AzureClusterToAzureMachinePoolsMapper(fakeClient, scheme, log)
	g.Expect(err).NotTo(HaveOccurred())

	requests := mapper.Map(handler.MapObject{
		Object: &infrav1.AzureCluster{
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
		},
	})
	g.Expect(requests).To(HaveLen(2))
}

func TestAzureManagedClusterToAzureManagedMachinePoolsMapper(t *testing.T) {
	g := NewWithT(t)
	scheme := newScheme(g)
	clusterName := "my-cluster"
	initObjects := []runtime.Object{
		newCluster(clusterName),
		// Create two Machines with an infrastructure ref and one without.
		newManagedMachinePoolInfraReference(clusterName, "my-mmp-0"),
		newManagedMachinePoolInfraReference(clusterName, "my-mmp-1"),
		newManagedMachinePoolInfraReference(clusterName, "my-mmp-2"),
		newMachinePool(clusterName, "my-machine-2"),
	}
	fakeClient := fake.NewFakeClientWithScheme(scheme, initObjects...)

	log := mock_log.NewMockLogger(gomock.NewController(t))
	log.EXPECT().WithValues("AzureCluster", "my-cluster", "Namespace", "default").Return(log)
	log.EXPECT().Info("gk does not match", "gk", gomock.Any(), "infraGK", gomock.Any())
	mapper, err := AzureManagedClusterToAzureManagedMachinePoolsMapper(fakeClient, scheme, log)
	g.Expect(err).NotTo(HaveOccurred())

	requests := mapper.Map(handler.MapObject{
		Object: &infrav1exp.AzureManagedCluster{
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

func TestAzureManagedClusterToAzureManagedControlPlaneMapper(t *testing.T) {
	g := NewWithT(t)
	scheme := newScheme(g)
	cpName := "my-managed-cp"
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
	fakeClient := fake.NewFakeClientWithScheme(scheme, initObjects...)

	log := mock_log.NewMockLogger(gomock.NewController(t))
	log.EXPECT().WithValues("AzureCluster", "az-"+cluster.Name, "Namespace", "default")

	mapper, err := AzureManagedClusterToAzureManagedControlPlaneMapper(fakeClient, log)
	g.Expect(err).NotTo(HaveOccurred())
	requests := mapper.Map(handler.MapObject{
		Object: &infrav1exp.AzureManagedCluster{
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

func Test_MachinePoolToInfrastructureMapFunc(t *testing.T) {
	cases := []struct {
		Name             string
		Setup            func(logMock *mock_log.MockLogger)
		MapObjectFactory func(*GomegaWithT) handler.MapObject
		Expect           func(*GomegaWithT, []reconcile.Request)
	}{
		{
			Name: "MachinePoolToAzureMachinePool",
			MapObjectFactory: func(g *GomegaWithT) handler.MapObject {
				return handler.MapObject{
					Object: newMachinePoolWithInfrastructureRef("azureCluster", "machinePool"),
				}
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
			MapObjectFactory: func(g *GomegaWithT) handler.MapObject {
				return handler.MapObject{
					Object: newMachinePool("azureCluster", "machinePool"),
				}
			},
			Setup: func(logMock *mock_log.MockLogger) {
				ampGK := infrav1exp.GroupVersion.WithKind("AzureMachinePool").GroupKind()
				logMock.EXPECT().Info("gk does not match", "gk", ampGK, "infraGK", gomock.Any())
			},
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(HaveLen(0))
			},
		},
		{
			Name: "NotAMachinePool",
			MapObjectFactory: func(g *GomegaWithT) handler.MapObject {
				return handler.MapObject{
					Object: newCluster("azureCluster"),
				}
			},
			Setup: func(logMock *mock_log.MockLogger) {
				logMock.EXPECT().Info("attempt to map incorrect type", "type", "*v1alpha3.Cluster")
			},
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(HaveLen(0))
			},
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			log := mock_log.NewMockLogger(mockCtrl)
			if c.Setup != nil {
				c.Setup(log)
			}
			f := MachinePoolToInfrastructureMapFunc(infrav1exp.GroupVersion.WithKind("AzureMachinePool"), log)
			reqs := f(c.MapObjectFactory(g))
			c.Expect(g, reqs)
		})
	}
}

func Test_azureClusterToAzureMachinePoolsFunc(t *testing.T) {
	cases := []struct {
		Name             string
		Setup            func(*GomegaWithT, *testing.T) (*mock_log.MockLogger, *gomock.Controller, client.Client)
		MapObjectFactory func(*GomegaWithT) handler.MapObject
		Expect           func(*GomegaWithT, []reconcile.Request)
	}{
		{
			Name: "NotAnAzureCluster",
			MapObjectFactory: func(g *GomegaWithT) handler.MapObject {
				return handler.MapObject{
					Object: newMachinePool("foo", "bar"),
				}
			},
			Setup: func(g *GomegaWithT, t *testing.T) (*mock_log.MockLogger, *gomock.Controller, client.Client) {
				mockCtrl := gomock.NewController(t)
				log := mock_log.NewMockLogger(mockCtrl)
				kClient := fake.NewFakeClientWithScheme(newScheme(g))
				log.EXPECT().Error(gomockinternal.ErrStrEq("expected a AzureCluster but got a *v1alpha3.MachinePool"), "failed to get AzureCluster")

				return log, mockCtrl, kClient
			},
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(HaveLen(0))
			},
		},
		{
			Name: "AzureClusterDoesNotExist",
			MapObjectFactory: func(g *GomegaWithT) handler.MapObject {
				return handler.MapObject{
					Object: newAzureCluster("foo"),
				}
			},
			Setup: func(g *GomegaWithT, t *testing.T) (*mock_log.MockLogger, *gomock.Controller, client.Client) {
				mockCtrl := gomock.NewController(t)
				log := mock_log.NewMockLogger(mockCtrl)
				logWithValues := mock_log.NewMockLogger(mockCtrl)
				kClient := fake.NewFakeClientWithScheme(newScheme(g))
				log.EXPECT().WithValues("AzureCluster", "azurefoo", "Namespace", "default").Return(logWithValues)
				logWithValues.EXPECT().Info("owning cluster not found")
				return log, mockCtrl, kClient
			},
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(HaveLen(0))
			},
		},
		{
			Name: "AzureClusterExistsButDoesNotHaveMachinePools",
			MapObjectFactory: func(g *GomegaWithT) handler.MapObject {
				return handler.MapObject{
					Object: newAzureCluster("foo"),
				}
			},
			Setup: func(g *GomegaWithT, t *testing.T) (*mock_log.MockLogger, *gomock.Controller, client.Client) {
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
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(HaveLen(0))
			},
		},
		{
			Name: "AzureClusterExistsWithMachinePoolsButNoInfraRefs",
			MapObjectFactory: func(g *GomegaWithT) handler.MapObject {
				return handler.MapObject{
					Object: newAzureCluster("foo"),
				}
			},
			Setup: func(g *GomegaWithT, t *testing.T) (*mock_log.MockLogger, *gomock.Controller, client.Client) {
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
			Expect: func(g *GomegaWithT, reqs []reconcile.Request) {
				g.Expect(reqs).To(HaveLen(0))
			},
		},
		{
			Name: "AzureClusterExistsWithMachinePoolsWithOneInfraRefs",
			MapObjectFactory: func(g *GomegaWithT) handler.MapObject {
				return handler.MapObject{
					Object: newAzureCluster("foo"),
				}
			},
			Setup: func(g *GomegaWithT, t *testing.T) (*mock_log.MockLogger, *gomock.Controller, client.Client) {
				mockCtrl := gomock.NewController(t)
				log := mock_log.NewMockLogger(mockCtrl)
				logWithValues := mock_log.NewMockLogger(mockCtrl)
				const clusterName = "foo"
				initObj := []runtime.Object{
					newCluster(clusterName),
					newAzureCluster(clusterName),
					newMachinePool(clusterName, "pool1"),
					newAzureMachinePool(clusterName, "azurepool2"),
					newMachinePoolWithInfrastructureRef(clusterName, "pool2"),
				}
				kClient := fake.NewFakeClientWithScheme(newScheme(g), initObj...)
				log.EXPECT().WithValues("AzureCluster", "azurefoo", "Namespace", "default").Return(logWithValues)
				return log, mockCtrl, kClient
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
			log, mockctrl, kClient := c.Setup(g, t)
			defer mockctrl.Finish()

			f := AzureClusterToAzureMachinePoolsFunc(kClient, log)
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
	m.Spec.Template.Spec.InfrastructureRef = corev1.ObjectReference{
		Kind:       "AzureManagedMachinePool",
		Namespace:  m.Namespace,
		Name:       "azure" + poolName,
		APIVersion: infrav1exp.GroupVersion.String(),
	}
	return m
}

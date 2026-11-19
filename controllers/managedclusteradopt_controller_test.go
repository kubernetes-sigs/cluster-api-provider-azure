/*
Copyright 2024 The Kubernetes Authors.

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
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20250801"
	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestManagedClusterAdoptReconcile(t *testing.T) {
	ctx := t.Context()
	g := NewWithT(t)
	req := ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      "fake-mc",
			Namespace: "fake-ns",
		},
	}
	managedCluster := &asocontainerservicev1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-mc",
			Namespace: "fake-ns",
			Annotations: map[string]string{
				adoptAnnotation: adoptAnnotationValue,
			},
		},
		Spec: asocontainerservicev1.ManagedCluster_Spec{
			Owner: &genruntime.KnownResourceReference{
				Name: "fake-mc",
			},
		},
	}

	rg := &asoresourcesv1.ResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-mc",
			Namespace: "fake-ns",
		},
	}

	s := runtime.NewScheme()
	err := asocontainerservicev1.AddToScheme(s)
	g.Expect(err).ToNot(HaveOccurred())
	err = clusterv1.AddToScheme(s)
	g.Expect(err).ToNot(HaveOccurred())
	err = asoresourcesv1.AddToScheme(s)
	g.Expect(err).ToNot(HaveOccurred())
	err = infrav1.AddToScheme(s)
	g.Expect(err).ToNot(HaveOccurred())
	client := fake.NewClientBuilder().WithScheme(s).WithObjects(managedCluster, rg).Build()
	rec := ManagedClusterAdoptReconciler{
		Client: client,
	}
	_, err = rec.Reconcile(ctx, req)
	g.Expect(err).ToNot(HaveOccurred())
	mcp := &infrav1.AzureASOManagedControlPlane{}
	err = rec.Get(ctx, types.NamespacedName{Name: managedCluster.Name, Namespace: managedCluster.Namespace}, mcp)
	g.Expect(err).ToNot(HaveOccurred())
	asomc := &infrav1.AzureASOManagedCluster{}
	err = rec.Get(ctx, types.NamespacedName{Name: managedCluster.Name, Namespace: managedCluster.Namespace}, asomc)
	g.Expect(err).ToNot(HaveOccurred())
}

func TestAdoptControllersDoNotStopManagerWithoutASOCRDs(t *testing.T) {
	g := NewWithT(t)

	s := runtime.NewScheme()
	g.Expect(asocontainerservicev1.AddToScheme(s)).To(Succeed())

	// An empty RESTMapper behaves like a cluster where ASO has not installed its CRDs yet.
	mgr, err := ctrl.NewManager(&rest.Config{Host: "https://127.0.0.1:1"}, ctrl.Options{
		Scheme:                 s,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
		MapperProvider: func(*rest.Config, *http.Client) (meta.RESTMapper, error) {
			return meta.NewDefaultRESTMapper(nil), nil
		},
		Controller: config.Controller{SkipNameValidation: ptr.To(true)},
	})
	g.Expect(err).NotTo(HaveOccurred())

	options := controller.Options{CacheSyncTimeout: 100 * time.Millisecond}
	g.Expect((&ManagedClusterAdoptReconciler{Client: mgr.GetClient()}).SetupWithManager(t.Context(), mgr, options)).To(Succeed())
	g.Expect((&AgentPoolAdoptReconciler{Client: mgr.GetClient()}).SetupWithManager(t.Context(), mgr, options)).To(Succeed())

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	g.Expect(mgr.Start(ctx)).To(Succeed())
	g.Expect(ctx.Err()).To(MatchError(context.DeadlineExceeded))
}

type toggleRESTMapper struct {
	meta.RESTMapper
	served atomic.Bool
}

func (m *toggleRESTMapper) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	if !m.served.Load() {
		return nil, &meta.NoKindMatchError{GroupKind: gk, SearchedVersions: versions}
	}
	return &meta.RESTMapping{}, nil
}

func TestWaitForKind(t *testing.T) {
	g := NewWithT(t)

	mapper := &toggleRESTMapper{}
	var setupCalls atomic.Int32
	runnable := waitForKind(mapper, asocontainerservicev1.GroupVersion.WithKind("ManagedCluster"), 10*time.Millisecond, func() error {
		setupCalls.Add(1)
		return nil
	})

	done := make(chan error, 1)
	go func() { done <- runnable.Start(t.Context()) }()

	g.Consistently(setupCalls.Load, 100*time.Millisecond).Should(BeZero())

	mapper.served.Store(true)
	g.Eventually(done).Should(Receive(Not(HaveOccurred())))
	g.Expect(setupCalls.Load()).To(Equal(int32(1)))
}

func TestWaitForKindStopsWithContext(t *testing.T) {
	g := NewWithT(t)

	runnable := waitForKind(&toggleRESTMapper{}, asocontainerservicev1.GroupVersion.WithKind("ManagedCluster"), 10*time.Millisecond, func() error {
		t.Error("setup must not run while the kind is not served")
		return nil
	})

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	g.Expect(runnable.Start(ctx)).To(Succeed())
}

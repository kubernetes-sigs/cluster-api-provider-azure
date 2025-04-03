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
	"testing"

	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231001"
	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"sigs.k8s.io/cluster-api-provider-azure/api/v1alpha1"
)

func TestManagedClusterAdoptReconcile(t *testing.T) {
	ctx := context.Background()
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
	err = v1alpha1.AddToScheme(s)
	g.Expect(err).ToNot(HaveOccurred())
	client := fake.NewClientBuilder().WithScheme(s).WithObjects(managedCluster, rg).Build()
	rec := ManagedClusterAdoptReconciler{
		Client: client,
	}
	_, err = rec.Reconcile(ctx, req)
	g.Expect(err).ToNot(HaveOccurred())
	mcp := &v1alpha1.AzureASOManagedControlPlane{}
	err = rec.Get(ctx, types.NamespacedName{Name: managedCluster.Name, Namespace: managedCluster.Namespace}, mcp)
	g.Expect(err).ToNot(HaveOccurred())
	asomc := &v1alpha1.AzureASOManagedCluster{}
	err = rec.Get(ctx, types.NamespacedName{Name: managedCluster.Name, Namespace: managedCluster.Namespace}, asomc)
	g.Expect(err).ToNot(HaveOccurred())
}

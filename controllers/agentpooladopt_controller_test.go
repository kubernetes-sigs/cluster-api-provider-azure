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
	"context"
	"testing"

	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231001"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1alpha "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha1"
)

func TestAgentPoolAdoptController(t *testing.T) {
	g := NewWithT(t)
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "fake-agent-pool", Namespace: "fake-ns"}}
	ctx := context.Background()
	scheme, err := newScheme()
	g.Expect(err).ToNot(HaveOccurred())

	agentPool := &asocontainerservicev1.ManagedClustersAgentPool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-agent-pool",
			Namespace: "fake-ns",
			Annotations: map[string]string{
				adoptAnnotation: adoptAnnotationValue,
			},
		},
		Spec: asocontainerservicev1.ManagedClustersAgentPool_Spec{
			Count: ptr.To(1),
			Owner: &genruntime.KnownResourceReference{
				Name: "fake-managed-cluster",
			},
		},
	}
	mc := &asocontainerservicev1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-managed-cluster",
			Namespace: "fake-ns",
			OwnerReferences: []metav1.OwnerReference{
				{
					Kind:       infrav1alpha.AzureASOManagedControlPlaneKind,
					APIVersion: infrav1alpha.GroupVersion.Identifier(),
					Name:       "fake-managed-cluster",
				},
			},
		},
	}
	asoManagedControlPlane := &infrav1alpha.AzureASOManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-managed-cluster",
			Namespace: "fake-ns",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "cluster-name",
			},
		},
	}

	err = asocontainerservicev1.AddToScheme(scheme)
	g.Expect(err).ToNot(HaveOccurred())
	err = infrav1alpha.AddToScheme(scheme)
	g.Expect(err).ToNot(HaveOccurred())
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(agentPool, mc, asoManagedControlPlane).WithStatusSubresource(mc, agentPool, asoManagedControlPlane).Build()
	aprec := &AgentPoolAdoptReconciler{
		Client: client,
	}
	_, err = aprec.Reconcile(ctx, req)
	g.Expect(err).ToNot(HaveOccurred())
	mp := &expv1.MachinePool{}
	err = aprec.Get(ctx, types.NamespacedName{Name: agentPool.Name, Namespace: "fake-ns"}, mp)
	g.Expect(err).ToNot(HaveOccurred())
	asoMP := &infrav1alpha.AzureASOManagedMachinePool{}
	err = aprec.Get(ctx, types.NamespacedName{Name: agentPool.Name, Namespace: "fake-ns"}, asoMP)
	g.Expect(err).ToNot(HaveOccurred())
}

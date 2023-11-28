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

	aadpodv1 "github.com/Azure/aad-pod-identity/pkg/apis/aadpodidentity/v1"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestUnclonedMachinesPredicate(t *testing.T) {
	cases := map[string]struct {
		expected    bool
		labels      map[string]string
		annotations map[string]string
	}{
		"uncloned worker node should return true": {
			expected:    true,
			labels:      nil,
			annotations: nil,
		},
		"uncloned control plane node should return true": {
			expected: true,
			labels: map[string]string{
				clusterv1.MachineControlPlaneLabel: "",
			},
			annotations: nil,
		},
		"cloned node should return false": {
			expected: false,
			labels:   nil,
			annotations: map[string]string{
				clusterv1.TemplateClonedFromGroupKindAnnotation: infrav1.GroupVersion.WithKind("AzureMachineTemplate").GroupKind().String(),
			},
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			machine := &infrav1.AzureMachine{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      tc.labels,
					Annotations: tc.annotations,
				},
			}
			e := event.GenericEvent{
				Object: machine,
			}
			filter := filterUnclonedMachinesPredicate{}
			if filter.Generic(e) != tc.expected {
				t.Errorf("expected: %t, got %t", tc.expected, filter.Generic(e))
			}
		})
	}
}

func TestAzureJSONMachineReconciler(t *testing.T) {
	scheme, err := newScheme()
	if err != nil {
		t.Error(err)
	}

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-cluster",
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &corev1.ObjectReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
				Kind:       infrav1.AzureClusterKind,
				Name:       "my-azure-cluster",
			},
		},
	}

	azureCluster := &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-azure-cluster",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "cluster.x-k8s.io/v1beta1",
					Kind:       "Cluster",
					Name:       "my-cluster",
				},
			},
		},
		Spec: infrav1.AzureClusterSpec{
			AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
				SubscriptionID: "123",
			},
			NetworkSpec: infrav1.NetworkSpec{
				Subnets: infrav1.Subnets{
					{
						SubnetClassSpec: infrav1.SubnetClassSpec{
							Name: "node",
							Role: infrav1.SubnetNode,
						},
					},
				},
			},
		},
	}

	azureMachine := &infrav1.AzureMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-machine",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "my-cluster",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "cluster.x-k8s.io/v1beta1",
					Kind:       "Cluster",
					Name:       "my-cluster",
				},
			},
		},
	}

	cases := map[string]struct {
		objects []runtime.Object
		fail    bool
		err     string
	}{
		"should reconcile normally": {
			objects: []runtime.Object{
				cluster,
				azureCluster,
				azureMachine,
			},
		},
		"missing azure cluster should return error": {
			objects: []runtime.Object{
				cluster,
				azureMachine,
			},
			fail: true,
			err:  "azureclusters.infrastructure.cluster.x-k8s.io \"my-azure-cluster\" not found",
		},
		"infra ref is nil": {
			objects: []runtime.Object{
				&clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-cluster",
					},
					Spec: clusterv1.ClusterSpec{
						InfrastructureRef: nil,
					},
				},
				azureCluster,
				azureMachine,
			},
			fail: false,
		},
		"infra ref is not an azure cluster": {
			objects: []runtime.Object{
				&clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-cluster",
					},
					Spec: clusterv1.ClusterSpec{
						InfrastructureRef: &corev1.ObjectReference{
							APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
							Kind:       "FooCluster",
							Name:       "my-foo-cluster",
						},
					},
				},
				azureCluster,
				azureMachine,
			},
			fail: false,
		},
	}

	os.Setenv(auth.ClientID, "fooClient")
	os.Setenv(auth.ClientSecret, "fooSecret")
	os.Setenv(auth.TenantID, "fooTenant")

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tc.objects...).Build()

			reconciler := &AzureJSONMachineReconciler{
				Client:   client,
				Recorder: record.NewFakeRecorder(128),
			}

			_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "",
					Name:      "my-machine",
				},
			})
			if tc.fail {
				if diff := cmp.Diff(tc.err, err.Error()); diff != "" {
					t.Error(diff)
				}
			} else {
				if err != nil {
					t.Errorf("expected success, but got error: %s", err.Error())
				}
			}
		})
	}
}

func newScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	schemeFn := []func(*runtime.Scheme) error{
		clientgoscheme.AddToScheme,
		infrav1.AddToScheme,
		clusterv1.AddToScheme,
		infrav1exp.AddToScheme,
		aadpodv1.AddToScheme,
		expv1.AddToScheme,
	}
	for _, fn := range schemeFn {
		fn := fn
		if err := fn(scheme); err != nil {
			return nil, err
		}
	}
	return scheme, nil
}

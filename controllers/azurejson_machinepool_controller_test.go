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
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/identities"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/identities/mock_identities"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAzureJSONPoolReconciler(t *testing.T) {
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

	machinePool := &expv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-machine-pool",
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

	azureMachinePool := &infrav1exp.AzureMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-azure-machine-pool",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "cluster.x-k8s.io/v1beta1",
					Kind:       "Cluster",
					Name:       "my-cluster",
				},
				{
					APIVersion: "cluster.x-k8s.io/v1beta1",
					Kind:       "MachinePool",
					Name:       "my-machine-pool",
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
				machinePool,
				azureMachinePool,
			},
		},
		"missing azure cluster should return error": {
			objects: []runtime.Object{
				cluster,
				machinePool,
				azureMachinePool,
			},
			fail: true,
			err:  "failed to create cluster scope for cluster /my-cluster: azureclusters.infrastructure.cluster.x-k8s.io \"my-azure-cluster\" not found",
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
				machinePool,
				azureMachinePool,
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
				machinePool,
				azureMachinePool,
			},
			fail: true,
			err:  "failed to create cluster scope for cluster /my-cluster: unsupported infrastructure type \"FooCluster\", should be AzureCluster or AzureManagedCluster",
		},
	}

	t.Setenv(auth.ClientID, "fooClient")
	t.Setenv(auth.ClientSecret, "fooSecret")
	t.Setenv(auth.TenantID, "fooTenant")

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tc.objects...).Build()

			reconciler := &AzureJSONMachinePoolReconciler{
				Client:   client,
				Recorder: record.NewFakeRecorder(128),
			}

			_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "",
					Name:      "my-azure-machine-pool",
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

func TestAzureJSONPoolReconcilerUserAssignedIdentities(t *testing.T) {
	g := NewWithT(t)
	ctrlr := gomock.NewController(t)
	defer ctrlr.Finish()
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "fake-machine-pool", Namespace: "fake-ns"}}
	ctx := context.Background()
	scheme, err := newScheme()
	g.Expect(err).ToNot(HaveOccurred())

	azureMP := &infrav1exp.AzureMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-machine-pool",
			Namespace: "fake-ns",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "fake-cluster",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: fmt.Sprintf("%s/%s", expv1.GroupVersion.Group, expv1.GroupVersion.Version),
					Kind:       "MachinePool",
					Name:       "fake-other-machine-pool",
					Controller: to.Ptr(true),
				},
			},
		},
		Spec: infrav1exp.AzureMachinePoolSpec{
			UserAssignedIdentities: []infrav1.UserAssignedIdentity{
				{
					ProviderID: "fake-id",
				},
			},
		},
	}

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-cluster",
			Namespace: "fake-ns",
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &corev1.ObjectReference{
				Kind:      "AzureCluster",
				Name:      "fake-azure-cluster",
				Namespace: "fake-ns",
			},
		},
	}

	ownerMP := &expv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-other-machine-pool",
			Namespace: "fake-ns",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "fake-cluster",
			},
		},
	}

	azureCluster := &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-azure-cluster",
			Namespace: "fake-ns",
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
	apiVersion, kind := infrav1.GroupVersion.WithKind("AzureMachinePool").ToAPIVersionAndKind()

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      azureMP.Name,
			Namespace: "fake-ns",
			Labels: map[string]string{
				"fake-cluster": string(infrav1.ResourceLifecycleOwned),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: apiVersion,
					Kind:       kind,
					Name:       azureMP.GetName(),
					Controller: ptr.To(true),
				},
			},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(azureMP, ownerMP, cluster, azureCluster, sec).Build()
	rec := AzureJSONMachinePoolReconciler{
		Client:           client,
		Recorder:         record.NewFakeRecorder(42),
		ReconcileTimeout: reconciler.DefaultLoopTimeout,
	}
	id := "fake-id"
	getClient = func(auth azure.Authorizer) (identities.Client, error) {
		mockClient := mock_identities.NewMockClient(ctrlr)
		mockClient.EXPECT().GetClientID(gomock.Any(), gomock.Any()).Return(id, nil)
		return mockClient, nil
	}

	_, err = rec.Reconcile(ctx, req)
	g.Expect(err).ToNot(HaveOccurred())
}

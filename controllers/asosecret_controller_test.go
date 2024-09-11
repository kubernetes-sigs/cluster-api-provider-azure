/*
Copyright 2023 The Kubernetes Authors.

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

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestASOSecretReconcile(t *testing.T) {
	os.Setenv("AZURE_CLIENT_ID", "fooClient")             //nolint:tenv // we want to use os.Setenv here instead of t.Setenv
	os.Setenv("AZURE_CLIENT_SECRET", "fooSecret")         //nolint:tenv // we want to use os.Setenv here instead of t.Setenv
	os.Setenv("AZURE_TENANT_ID", "fooTenant")             //nolint:tenv // we want to use os.Setenv here instead of t.Setenv
	os.Setenv("AZURE_SUBSCRIPTION_ID", "fooSubscription") //nolint:tenv // we want to use os.Setenv here instead of t.Setenv

	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = clientgoscheme.AddToScheme(scheme)

	defaultCluster := getASOCluster()
	defaultAzureCluster := getASOAzureCluster()
	defaultAzureManagedControlPlane := getASOAzureManagedControlPlane()
	defaultASOSecret := getASOSecret(defaultAzureCluster)
	defaultClusterIdentityType := infrav1.ServicePrincipal

	cases := map[string]struct {
		clusterName string
		objects     []runtime.Object
		err         string
		event       string
		asoSecret   *corev1.Secret
	}{
		"should not fail if the azure cluster is not found": {
			clusterName: defaultAzureCluster.Name,
			objects: []runtime.Object{
				getASOCluster(func(c *clusterv1.Cluster) {
					c.Spec.InfrastructureRef.Name = defaultAzureCluster.Name
					c.Spec.InfrastructureRef.Kind = defaultAzureCluster.Kind
				}),
			},
		},
		"should not fail for AzureCluster without ownerRef set yet": {
			clusterName: defaultAzureCluster.Name,
			objects: []runtime.Object{
				getASOAzureCluster(func(c *infrav1.AzureCluster) {
					c.ObjectMeta.OwnerReferences = nil
				}),
				defaultCluster,
			},
		},
		"should reconcile normally for AzureCluster with IdentityRef configured": {
			clusterName: defaultAzureCluster.Name,
			objects: []runtime.Object{
				getASOAzureCluster(func(c *infrav1.AzureCluster) {
					c.Spec.IdentityRef = &corev1.ObjectReference{
						Name:      "my-azure-cluster-identity",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentity(func(identity *infrav1.AzureClusterIdentity) {
					identity.Spec.Type = defaultClusterIdentityType
					identity.Spec.ClientSecret = corev1.SecretReference{
						Name:      "fooSecret",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentitySecret(),
				defaultCluster,
			},
			asoSecret: getASOSecret(defaultAzureCluster, func(s *corev1.Secret) {
				s.Data = map[string][]byte{
					"AZURE_SUBSCRIPTION_ID": []byte("123"),
					"AZURE_TENANT_ID":       []byte("fooTenant"),
					"AZURE_CLIENT_ID":       []byte("fooClient"),
					"AZURE_CLIENT_SECRET":   []byte("fooSecret"),
				}
			}),
		},
		"should reconcile normally for AzureManagedControlPlane with IdentityRef configured": {
			clusterName: defaultAzureManagedControlPlane.Name,
			objects: []runtime.Object{
				getASOAzureManagedControlPlane(func(c *infrav1.AzureManagedControlPlane) {
					c.Spec.IdentityRef = &corev1.ObjectReference{
						Name:      "my-azure-cluster-identity",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentity(func(identity *infrav1.AzureClusterIdentity) {
					identity.Spec.Type = defaultClusterIdentityType
					identity.Spec.ClientSecret = corev1.SecretReference{
						Name:      "fooSecret",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentitySecret(),
				defaultCluster,
			},
			asoSecret: getASOSecret(defaultAzureManagedControlPlane, func(s *corev1.Secret) {
				s.Data = map[string][]byte{
					"AZURE_SUBSCRIPTION_ID": []byte("fooSubscription"),
					"AZURE_TENANT_ID":       []byte("fooTenant"),
					"AZURE_CLIENT_ID":       []byte("fooClient"),
					"AZURE_CLIENT_SECRET":   []byte("fooSecret"),
				}
			}),
		},
		"should reconcile normally for AzureCluster with an IdentityRef of type WorkloadIdentity": {
			clusterName: defaultAzureCluster.Name,
			objects: []runtime.Object{
				getASOAzureCluster(func(c *infrav1.AzureCluster) {
					c.Spec.IdentityRef = &corev1.ObjectReference{
						Name:      "my-azure-cluster-identity",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentity(func(identity *infrav1.AzureClusterIdentity) {
					identity.Spec.Type = "WorkloadIdentity"
				}),
				defaultCluster,
			},
			asoSecret: getASOSecret(defaultAzureCluster, func(s *corev1.Secret) {
				s.Data = map[string][]byte{
					"AZURE_SUBSCRIPTION_ID": []byte("123"),
					"AZURE_TENANT_ID":       []byte("fooTenant"),
					"AZURE_CLIENT_ID":       []byte("fooClient"),
					"AUTH_MODE":             []byte("workloadidentity"),
				}
			}),
		},
		"should reconcile normally for AzureManagedControlPlane with an IdentityRef of type WorkloadIdentity": {
			clusterName: defaultAzureManagedControlPlane.Name,
			objects: []runtime.Object{
				getASOAzureManagedControlPlane(func(c *infrav1.AzureManagedControlPlane) {
					c.Spec.IdentityRef = &corev1.ObjectReference{
						Name:      "my-azure-cluster-identity",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentity(func(identity *infrav1.AzureClusterIdentity) {
					identity.Spec.Type = infrav1.WorkloadIdentity
				}),
				defaultCluster,
			},
			asoSecret: getASOSecret(defaultAzureManagedControlPlane, func(s *corev1.Secret) {
				s.Data = map[string][]byte{
					"AZURE_SUBSCRIPTION_ID": []byte("fooSubscription"),
					"AZURE_TENANT_ID":       []byte("fooTenant"),
					"AZURE_CLIENT_ID":       []byte("fooClient"),
					"AUTH_MODE":             []byte("workloadidentity"),
				}
			}),
		},
		"should reconcile normally for AzureCluster with an IdentityRef of type UserAssignedMSI": {
			clusterName: defaultAzureCluster.Name,
			objects: []runtime.Object{
				getASOAzureCluster(func(c *infrav1.AzureCluster) {
					c.Spec.IdentityRef = &corev1.ObjectReference{
						Name:      "my-azure-cluster-identity",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentity(func(identity *infrav1.AzureClusterIdentity) {
					identity.Spec.Type = infrav1.UserAssignedMSI
				}),
				defaultCluster,
			},
			asoSecret: getASOSecret(defaultAzureCluster, func(s *corev1.Secret) {
				s.Data = map[string][]byte{
					"AZURE_SUBSCRIPTION_ID": []byte("123"),
					"AZURE_TENANT_ID":       []byte("fooTenant"),
					"AZURE_CLIENT_ID":       []byte("fooClient"),
					"AUTH_MODE":             []byte("podidentity"),
				}
			}),
		},
		"should reconcile normally for AzureManagedControlPlane with an IdentityRef of type UserAssignedMSI": {
			clusterName: defaultAzureManagedControlPlane.Name,
			objects: []runtime.Object{
				getASOAzureManagedControlPlane(func(c *infrav1.AzureManagedControlPlane) {
					c.Spec.IdentityRef = &corev1.ObjectReference{
						Name:      "my-azure-cluster-identity",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentity(func(identity *infrav1.AzureClusterIdentity) {
					identity.Spec.Type = infrav1.UserAssignedMSI
				}),
				defaultCluster,
			},
			asoSecret: getASOSecret(defaultAzureManagedControlPlane, func(s *corev1.Secret) {
				s.Data = map[string][]byte{
					"AZURE_SUBSCRIPTION_ID": []byte("fooSubscription"),
					"AZURE_TENANT_ID":       []byte("fooTenant"),
					"AZURE_CLIENT_ID":       []byte("fooClient"),
					"AUTH_MODE":             []byte("podidentity"),
				}
			}),
		},
		"should fail if IdentityRef secret doesn't exist": {
			clusterName: defaultAzureManagedControlPlane.Name,
			objects: []runtime.Object{
				getASOAzureManagedControlPlane(func(c *infrav1.AzureManagedControlPlane) {
					c.Spec.IdentityRef = &corev1.ObjectReference{
						Name:      "my-azure-cluster-identity",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentity(func(identity *infrav1.AzureClusterIdentity) {
					identity.Spec.Type = defaultClusterIdentityType
					identity.Spec.ClientSecret = corev1.SecretReference{
						Name:      "fooSecret",
						Namespace: "default",
					}
				}),
				defaultCluster,
			},
			err: "secrets \"fooSecret\" not found",
		},
		"should return if cluster does not exist": {
			clusterName: defaultAzureCluster.Name,
			objects: []runtime.Object{
				defaultAzureCluster,
			},
			err: "failed to get Cluster/my-cluster: clusters.cluster.x-k8s.io \"my-cluster\" not found",
		},
		"should return if cluster is paused": {
			clusterName: defaultAzureCluster.Name,
			objects: []runtime.Object{
				getASOCluster(func(c *clusterv1.Cluster) {
					c.Spec.Paused = true
				}),
				getASOAzureCluster(func(c *infrav1.AzureCluster) {
					c.Spec.IdentityRef = &corev1.ObjectReference{
						Name:      "my-azure-cluster-identity",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentity(func(identity *infrav1.AzureClusterIdentity) {
					identity.Spec.Type = defaultClusterIdentityType
					identity.Spec.ClientSecret = corev1.SecretReference{
						Name:      "fooSecret",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentitySecret(),
			},
			event: "AzureCluster or linked Cluster is marked as paused. Won't reconcile",
		},
		"should return if azureCluster is not yet available": {
			clusterName: defaultAzureCluster.Name,
			objects: []runtime.Object{
				defaultCluster,
			},
			event: "AzureClusterObjectNotFound AzureCluster object default/my-azure-cluster not found",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tc.objects...).Build()

			reconciler := &ASOSecretReconciler{
				Client:   clientBuilder,
				Recorder: record.NewFakeRecorder(128),
			}

			_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "default",
					Name:      tc.clusterName,
				},
			})

			existingASOSecret := &corev1.Secret{}
			asoSecretErr := clientBuilder.Get(context.Background(), types.NamespacedName{
				Namespace: defaultASOSecret.Namespace,
				Name:      defaultASOSecret.Name,
			}, existingASOSecret)

			if tc.asoSecret != nil {
				g.Expect(asoSecretErr).NotTo(HaveOccurred())
				g.Expect(tc.asoSecret.Data).To(BeEquivalentTo(existingASOSecret.Data))
			} else {
				g.Expect(asoSecretErr).To(HaveOccurred())
			}

			if tc.err != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tc.err)))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			if tc.event != "" {
				g.Expect(reconciler.Recorder.(*record.FakeRecorder).Events).To(Receive(ContainSubstring(tc.event)))
			}
		})
	}
}

func getASOCluster(changes ...func(*clusterv1.Cluster)) *clusterv1.Cluster {
	input := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster",
			Namespace: "default",
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &corev1.ObjectReference{
				APIVersion: infrav1.GroupVersion.String(),
			},
		},
		Status: clusterv1.ClusterStatus{
			InfrastructureReady: true,
		},
	}

	for _, change := range changes {
		change(input)
	}

	return input
}

func getASOAzureCluster(changes ...func(*infrav1.AzureCluster)) *infrav1.AzureCluster {
	input := &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-azure-cluster",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
					Name:       "my-cluster",
				},
			},
		},
		Spec: infrav1.AzureClusterSpec{
			AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
				SubscriptionID: "123",
			},
		},
	}
	for _, change := range changes {
		change(input)
	}

	return input
}

func getASOAzureManagedControlPlane(changes ...func(*infrav1.AzureManagedControlPlane)) *infrav1.AzureManagedControlPlane {
	input := &infrav1.AzureManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-azure-managed-control-plane",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       "my-cluster",
					Kind:       "Cluster",
					APIVersion: clusterv1.GroupVersion.String(),
				},
			},
		},
		Spec: infrav1.AzureManagedControlPlaneSpec{},
		Status: infrav1.AzureManagedControlPlaneStatus{
			Ready:       true,
			Initialized: true,
		},
	}
	for _, change := range changes {
		change(input)
	}

	return input
}

func getASOAzureClusterIdentity(changes ...func(identity *infrav1.AzureClusterIdentity)) *infrav1.AzureClusterIdentity {
	input := &infrav1.AzureClusterIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-azure-cluster-identity",
			Namespace: "default",
		},
		Spec: infrav1.AzureClusterIdentitySpec{
			ClientID: "fooClient",
			TenantID: "fooTenant",
		},
	}

	for _, change := range changes {
		change(input)
	}

	return input
}

func getASOAzureClusterIdentitySecret(changes ...func(secret *corev1.Secret)) *corev1.Secret {
	input := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fooSecret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"clientSecret": []byte("fooSecret"),
		},
	}

	for _, change := range changes {
		change(input)
	}

	return input
}

func getASOSecret(cluster client.Object, changes ...func(secret *corev1.Secret)) *corev1.Secret {
	input := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster-aso-secret",
			Namespace: "default",
			Labels: map[string]string{
				"my-cluster": "owned",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: cluster.GetObjectKind().GroupVersionKind().GroupVersion().String(),
					Kind:       cluster.GetObjectKind().GroupVersionKind().Kind,
					Name:       cluster.GetName(),
					UID:        cluster.GetUID(),
					Controller: ptr.To(true),
				},
			},
		},
		Data: map[string][]byte{
			"AZURE_SUBSCRIPTION_ID": []byte("fooSubscription"),
		},
	}

	for _, change := range changes {
		change(input)
	}

	return input
}

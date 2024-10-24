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
	"encoding/json"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231001"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/secret"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1alpha "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha1"
)

func TestAzureASOManagedControlPlaneReconcile(t *testing.T) {
	ctx := context.Background()

	s := runtime.NewScheme()
	sb := runtime.NewSchemeBuilder(
		infrav1alpha.AddToScheme,
		clusterv1.AddToScheme,
		asocontainerservicev1.AddToScheme,
		corev1.AddToScheme,
	)
	NewGomegaWithT(t).Expect(sb.AddToScheme(s)).To(Succeed())
	fakeClientBuilder := func() *fakeclient.ClientBuilder {
		return fakeclient.NewClientBuilder().
			WithScheme(s).
			WithStatusSubresource(&infrav1alpha.AzureASOManagedControlPlane{})
	}

	t.Run("AzureASOManagedControlPlane does not exist", func(t *testing.T) {
		g := NewGomegaWithT(t)

		c := fakeClientBuilder().
			Build()
		r := &AzureASOManagedControlPlaneReconciler{
			Client: c,
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "doesn't", Name: "exist"}})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{}))
	})

	t.Run("Cluster does not exist", func(t *testing.T) {
		g := NewGomegaWithT(t)

		asoManagedControlPlane := &infrav1alpha.AzureASOManagedControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "amcp",
				Namespace: "ns",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.Identifier(),
						Kind:       "Cluster",
						Name:       "cluster",
					},
				},
			},
		}
		c := fakeClientBuilder().
			WithObjects(asoManagedControlPlane).
			Build()
		r := &AzureASOManagedControlPlaneReconciler{
			Client: c,
		}
		_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedControlPlane)})
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("adds a finalizer and block-move annotation", func(t *testing.T) {
		g := NewGomegaWithT(t)

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster",
				Namespace: "ns",
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					APIVersion: infrav1alpha.GroupVersion.Identifier(),
					Kind:       infrav1alpha.AzureASOManagedClusterKind,
				},
			},
		}
		asoManagedControlPlane := &infrav1alpha.AzureASOManagedControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "amcp",
				Namespace: cluster.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.Identifier(),
						Kind:       "Cluster",
						Name:       cluster.Name,
					},
				},
			},
		}
		c := fakeClientBuilder().
			WithObjects(cluster, asoManagedControlPlane).
			Build()
		r := &AzureASOManagedControlPlaneReconciler{
			Client: c,
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedControlPlane)})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{Requeue: true}))

		g.Expect(c.Get(ctx, client.ObjectKeyFromObject(asoManagedControlPlane), asoManagedControlPlane)).To(Succeed())
		g.Expect(asoManagedControlPlane.GetFinalizers()).To(ContainElement(infrav1alpha.AzureASOManagedControlPlaneFinalizer))
		g.Expect(asoManagedControlPlane.GetAnnotations()).To(HaveKey(clusterctlv1.BlockMoveAnnotation))
	})

	t.Run("reconciles resources that are not ready", func(t *testing.T) {
		g := NewGomegaWithT(t)

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster",
				Namespace: "ns",
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					APIVersion: infrav1alpha.GroupVersion.Identifier(),
					Kind:       infrav1alpha.AzureASOManagedClusterKind,
				},
			},
		}
		asoManagedControlPlane := &infrav1alpha.AzureASOManagedControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "amcp",
				Namespace: cluster.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.Identifier(),
						Kind:       "Cluster",
						Name:       cluster.Name,
					},
				},
				Finalizers: []string{
					infrav1alpha.AzureASOManagedControlPlaneFinalizer,
				},
				Annotations: map[string]string{
					clusterctlv1.BlockMoveAnnotation: "true",
				},
			},
			Spec: infrav1alpha.AzureASOManagedControlPlaneSpec{
				AzureASOManagedControlPlaneTemplateResourceSpec: infrav1alpha.AzureASOManagedControlPlaneTemplateResourceSpec{
					Resources: []runtime.RawExtension{
						{
							Raw: mcJSON(g, &asocontainerservicev1.ManagedCluster{
								ObjectMeta: metav1.ObjectMeta{
									Name: "mc",
								},
							}),
						},
					},
				},
			},
			Status: infrav1alpha.AzureASOManagedControlPlaneStatus{
				Ready: true,
			},
		}
		c := fakeClientBuilder().
			WithObjects(cluster, asoManagedControlPlane).
			Build()
		r := &AzureASOManagedControlPlaneReconciler{
			Client: c,
			newResourceReconciler: func(asoManagedControlPlane *infrav1alpha.AzureASOManagedControlPlane, _ []*unstructured.Unstructured) resourceReconciler {
				return &fakeResourceReconciler{
					owner: asoManagedControlPlane,
					reconcileFunc: func(ctx context.Context, o client.Object) error {
						asoManagedControlPlane.SetResourceStatuses([]infrav1alpha.ResourceStatus{
							{Ready: true},
							{Ready: false},
							{Ready: true},
						})
						return nil
					},
				}
			},
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedControlPlane)})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{}))

		g.Expect(c.Get(ctx, client.ObjectKeyFromObject(asoManagedControlPlane), asoManagedControlPlane)).To(Succeed())
		g.Expect(asoManagedControlPlane.Status.Ready).To(BeFalse())
	})

	t.Run("successfully reconciles normally", func(t *testing.T) {
		g := NewGomegaWithT(t)

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster",
				Namespace: "ns",
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					APIVersion: infrav1alpha.GroupVersion.Identifier(),
					Kind:       infrav1alpha.AzureASOManagedClusterKind,
				},
			},
		}
		kubeconfig := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secret.Name(cluster.Name, secret.Kubeconfig),
				Namespace: cluster.Namespace,
			},
			Data: map[string][]byte{
				"some other key": []byte("some data"),
			},
		}
		managedCluster := &asocontainerservicev1.ManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mc",
				Namespace: cluster.Namespace,
			},
			Spec: asocontainerservicev1.ManagedCluster_Spec{
				OperatorSpec: &asocontainerservicev1.ManagedClusterOperatorSpec{
					Secrets: &asocontainerservicev1.ManagedClusterOperatorSecrets{
						UserCredentials: &genruntime.SecretDestination{
							Name: secret.Name(cluster.Name, secret.Kubeconfig),
							Key:  "some other key",
						},
					},
				},
			},
			Status: asocontainerservicev1.ManagedCluster_STATUS{
				Fqdn:                     ptr.To("endpoint"),
				CurrentKubernetesVersion: ptr.To("Current"),
			},
		}
		asoManagedControlPlane := &infrav1alpha.AzureASOManagedControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "amcp",
				Namespace: cluster.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.Identifier(),
						Kind:       "Cluster",
						Name:       cluster.Name,
					},
				},
				Finalizers: []string{
					infrav1alpha.AzureASOManagedControlPlaneFinalizer,
				},
				Annotations: map[string]string{
					clusterctlv1.BlockMoveAnnotation: "true",
				},
			},
			Spec: infrav1alpha.AzureASOManagedControlPlaneSpec{
				AzureASOManagedControlPlaneTemplateResourceSpec: infrav1alpha.AzureASOManagedControlPlaneTemplateResourceSpec{
					Resources: []runtime.RawExtension{
						{
							Raw: mcJSON(g, &asocontainerservicev1.ManagedCluster{
								ObjectMeta: metav1.ObjectMeta{
									Name: managedCluster.Name,
								},
							}),
						},
					},
				},
			},
			Status: infrav1alpha.AzureASOManagedControlPlaneStatus{
				Ready: false,
			},
		}
		c := fakeClientBuilder().
			WithObjects(cluster, asoManagedControlPlane, managedCluster, kubeconfig).
			Build()
		kubeConfigPatched := false
		r := &AzureASOManagedControlPlaneReconciler{
			Client: &FakeClient{
				Client: c,
				patchFunc: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
					kubeconfig := obj.(*corev1.Secret)
					g.Expect(kubeconfig.Data[secret.KubeconfigDataName]).NotTo(BeEmpty())
					kubeConfigPatched = true
					return nil
				},
			},
			newResourceReconciler: func(_ *infrav1alpha.AzureASOManagedControlPlane, _ []*unstructured.Unstructured) resourceReconciler {
				return &fakeResourceReconciler{
					reconcileFunc: func(ctx context.Context, o client.Object) error {
						return nil
					},
				}
			},
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedControlPlane)})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{}))

		g.Expect(c.Get(ctx, client.ObjectKeyFromObject(asoManagedControlPlane), asoManagedControlPlane)).To(Succeed())
		g.Expect(asoManagedControlPlane.Status.ControlPlaneEndpoint.Host).To(Equal("endpoint"))
		g.Expect(asoManagedControlPlane.Status.Version).To(Equal("vCurrent"))
		g.Expect(kubeConfigPatched).To(BeTrue())
		g.Expect(asoManagedControlPlane.Status.Ready).To(BeTrue())
	})

	t.Run("successfully reconciles a kubeconfig with a token", func(t *testing.T) {
		g := NewGomegaWithT(t)

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster",
				Namespace: "ns",
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					APIVersion: infrav1alpha.GroupVersion.Identifier(),
					Kind:       infrav1alpha.AzureASOManagedClusterKind,
				},
			},
		}
		kubeconfig := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secret.Name(cluster.Name, secret.Kubeconfig) + "-user",
				Namespace: cluster.Namespace,
			},
			Data: map[string][]byte{
				"some other key": func() []byte {
					kubeconfig := &clientcmdapi.Config{
						AuthInfos: map[string]*clientcmdapi.AuthInfo{
							"some-user": {
								Exec: &clientcmdapi.ExecConfig{},
							},
						},
					}
					kubeconfigData, err := clientcmd.Write(*kubeconfig)
					g.Expect(err).NotTo(HaveOccurred())
					return kubeconfigData
				}(),
			},
		}
		managedCluster := &asocontainerservicev1.ManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mc",
				Namespace: cluster.Namespace,
			},
			Spec: asocontainerservicev1.ManagedCluster_Spec{
				OperatorSpec: &asocontainerservicev1.ManagedClusterOperatorSpec{
					Secrets: &asocontainerservicev1.ManagedClusterOperatorSecrets{
						UserCredentials: &genruntime.SecretDestination{
							Name: secret.Name(cluster.Name, secret.Kubeconfig) + "-user",
							Key:  "some other key",
						},
					},
				},
			},
			Status: asocontainerservicev1.ManagedCluster_STATUS{
				Fqdn: ptr.To("endpoint"),
				AadProfile: &asocontainerservicev1.ManagedClusterAADProfile_STATUS{
					Managed: ptr.To(true),
				},
				DisableLocalAccounts: ptr.To(true),
			},
		}
		asoManagedControlPlane := &infrav1alpha.AzureASOManagedControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "amcp",
				Namespace: cluster.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.Identifier(),
						Kind:       "Cluster",
						Name:       cluster.Name,
					},
				},
				Finalizers: []string{
					infrav1alpha.AzureASOManagedControlPlaneFinalizer,
				},
				Annotations: map[string]string{
					clusterctlv1.BlockMoveAnnotation: "true",
				},
			},
			Spec: infrav1alpha.AzureASOManagedControlPlaneSpec{
				AzureASOManagedControlPlaneTemplateResourceSpec: infrav1alpha.AzureASOManagedControlPlaneTemplateResourceSpec{
					Resources: []runtime.RawExtension{
						{
							Raw: mcJSON(g, &asocontainerservicev1.ManagedCluster{
								ObjectMeta: metav1.ObjectMeta{
									Name: managedCluster.Name,
								},
							}),
						},
					},
				},
			},
			Status: infrav1alpha.AzureASOManagedControlPlaneStatus{
				Ready: false,
			},
		}
		c := fakeClientBuilder().
			WithObjects(cluster, asoManagedControlPlane, managedCluster, kubeconfig).
			Build()
		kubeConfigPatched := false
		r := &AzureASOManagedControlPlaneReconciler{
			Client: &FakeClient{
				Client: c,
				patchFunc: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
					kubeconfigSecret := obj.(*corev1.Secret)
					g.Expect(kubeconfigSecret.Data[secret.KubeconfigDataName]).NotTo(BeEmpty())
					kubeConfigPatched = true

					kubeconfig, err := clientcmd.Load(kubeconfigSecret.Data[secret.KubeconfigDataName])
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(kubeconfig.AuthInfos).To(HaveEach(Satisfy(func(user *clientcmdapi.AuthInfo) bool {
						return user.Exec == nil &&
							user.Token == "token"
					})))

					return nil
				},
			},
			newResourceReconciler: func(_ *infrav1alpha.AzureASOManagedControlPlane, _ []*unstructured.Unstructured) resourceReconciler {
				return &fakeResourceReconciler{
					reconcileFunc: func(ctx context.Context, o client.Object) error {
						return nil
					},
				}
			},
			CredentialCache: fakeASOCredentialCache(func(_ context.Context, _ client.Object) (azcore.TokenCredential, error) {
				return fakeTokenCredential{token: "token", expiresOn: time.Now().Add(1 * time.Hour)}, nil
			}),
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedControlPlane)})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Requeue).To(BeFalse())
		g.Expect(result.RequeueAfter).NotTo(BeZero())

		g.Expect(c.Get(ctx, client.ObjectKeyFromObject(asoManagedControlPlane), asoManagedControlPlane)).To(Succeed())
		g.Expect(kubeConfigPatched).To(BeTrue())
		g.Expect(asoManagedControlPlane.Status.Ready).To(BeTrue())
	})

	t.Run("successfully reconciles a kubeconfig with a token that has expired", func(t *testing.T) {
		g := NewGomegaWithT(t)

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster",
				Namespace: "ns",
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					APIVersion: infrav1alpha.GroupVersion.Identifier(),
					Kind:       infrav1alpha.AzureASOManagedClusterKind,
				},
			},
		}
		kubeconfig := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secret.Name(cluster.Name, secret.Kubeconfig) + "-user",
				Namespace: cluster.Namespace,
			},
			Data: map[string][]byte{
				"some other key": func() []byte {
					kubeconfig := &clientcmdapi.Config{
						AuthInfos: map[string]*clientcmdapi.AuthInfo{
							"some-user": {
								Exec: &clientcmdapi.ExecConfig{},
							},
						},
					}
					kubeconfigData, err := clientcmd.Write(*kubeconfig)
					g.Expect(err).NotTo(HaveOccurred())
					return kubeconfigData
				}(),
			},
		}
		managedCluster := &asocontainerservicev1.ManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mc",
				Namespace: cluster.Namespace,
			},
			Spec: asocontainerservicev1.ManagedCluster_Spec{
				OperatorSpec: &asocontainerservicev1.ManagedClusterOperatorSpec{
					Secrets: &asocontainerservicev1.ManagedClusterOperatorSecrets{
						UserCredentials: &genruntime.SecretDestination{
							Name: secret.Name(cluster.Name, secret.Kubeconfig) + "-user",
							Key:  "some other key",
						},
					},
				},
			},
			Status: asocontainerservicev1.ManagedCluster_STATUS{
				Fqdn: ptr.To("endpoint"),
				AadProfile: &asocontainerservicev1.ManagedClusterAADProfile_STATUS{
					Managed: ptr.To(true),
				},
				DisableLocalAccounts: ptr.To(true),
			},
		}
		asoManagedControlPlane := &infrav1alpha.AzureASOManagedControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "amcp",
				Namespace: cluster.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.Identifier(),
						Kind:       "Cluster",
						Name:       cluster.Name,
					},
				},
				Finalizers: []string{
					infrav1alpha.AzureASOManagedControlPlaneFinalizer,
				},
				Annotations: map[string]string{
					clusterctlv1.BlockMoveAnnotation: "true",
				},
			},
			Spec: infrav1alpha.AzureASOManagedControlPlaneSpec{
				AzureASOManagedControlPlaneTemplateResourceSpec: infrav1alpha.AzureASOManagedControlPlaneTemplateResourceSpec{
					Resources: []runtime.RawExtension{
						{
							Raw: mcJSON(g, &asocontainerservicev1.ManagedCluster{
								ObjectMeta: metav1.ObjectMeta{
									Name: managedCluster.Name,
								},
							}),
						},
					},
				},
			},
			Status: infrav1alpha.AzureASOManagedControlPlaneStatus{
				Ready: true,
			},
		}
		c := fakeClientBuilder().
			WithObjects(cluster, asoManagedControlPlane, managedCluster, kubeconfig).
			Build()
		kubeConfigPatched := false
		r := &AzureASOManagedControlPlaneReconciler{
			Client: &FakeClient{
				Client: c,
				patchFunc: func(_ context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) error {
					kubeconfigSecret := obj.(*corev1.Secret)
					g.Expect(kubeconfigSecret.Data[secret.KubeconfigDataName]).NotTo(BeEmpty())
					kubeConfigPatched = true

					kubeconfig, err := clientcmd.Load(kubeconfigSecret.Data[secret.KubeconfigDataName])
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(kubeconfig.AuthInfos).To(HaveEach(Satisfy(func(user *clientcmdapi.AuthInfo) bool {
						return user.Exec == nil &&
							user.Token == "token"
					})))

					return nil
				},
			},
			newResourceReconciler: func(_ *infrav1alpha.AzureASOManagedControlPlane, _ []*unstructured.Unstructured) resourceReconciler {
				return &fakeResourceReconciler{
					reconcileFunc: func(ctx context.Context, o client.Object) error {
						return nil
					},
				}
			},
			CredentialCache: fakeASOCredentialCache(func(_ context.Context, _ client.Object) (azcore.TokenCredential, error) {
				return fakeTokenCredential{token: "token", expiresOn: time.Now()}, nil
			}),
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedControlPlane)})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{Requeue: true}))

		g.Expect(c.Get(ctx, client.ObjectKeyFromObject(asoManagedControlPlane), asoManagedControlPlane)).To(Succeed())
		g.Expect(kubeConfigPatched).To(BeTrue())
		g.Expect(asoManagedControlPlane.Status.Ready).To(BeFalse())
	})

	t.Run("successfully reconciles pause", func(t *testing.T) {
		g := NewGomegaWithT(t)

		cluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster",
				Namespace: "ns",
			},
			Spec: clusterv1.ClusterSpec{
				Paused: true,
			},
		}
		asoManagedControlPlane := &infrav1alpha.AzureASOManagedControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "amcp",
				Namespace: cluster.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.Identifier(),
						Kind:       "Cluster",
						Name:       cluster.Name,
					},
				},
				Annotations: map[string]string{
					clusterctlv1.BlockMoveAnnotation: "true",
				},
			},
		}
		c := fakeClientBuilder().
			WithObjects(cluster, asoManagedControlPlane).
			Build()
		r := &AzureASOManagedControlPlaneReconciler{
			Client: c,
			newResourceReconciler: func(_ *infrav1alpha.AzureASOManagedControlPlane, _ []*unstructured.Unstructured) resourceReconciler {
				return &fakeResourceReconciler{
					pauseFunc: func(_ context.Context, _ client.Object) error {
						return nil
					},
				}
			},
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedControlPlane)})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{}))

		g.Expect(c.Get(ctx, client.ObjectKeyFromObject(asoManagedControlPlane), asoManagedControlPlane)).To(Succeed())
		g.Expect(asoManagedControlPlane.GetAnnotations()).NotTo(HaveKey(clusterctlv1.BlockMoveAnnotation))
	})

	t.Run("successfully reconciles delete", func(t *testing.T) {
		g := NewGomegaWithT(t)

		asoManagedControlPlane := &infrav1alpha.AzureASOManagedControlPlane{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "amcp",
				Namespace: "ns",
				Finalizers: []string{
					infrav1alpha.AzureASOManagedControlPlaneFinalizer,
				},
				DeletionTimestamp: &metav1.Time{Time: time.Date(1, 0, 0, 0, 0, 0, 0, time.UTC)},
			},
		}
		c := fakeClientBuilder().
			WithObjects(asoManagedControlPlane).
			Build()
		r := &AzureASOManagedControlPlaneReconciler{
			Client: c,
			newResourceReconciler: func(_ *infrav1alpha.AzureASOManagedControlPlane, _ []*unstructured.Unstructured) resourceReconciler {
				return &fakeResourceReconciler{
					deleteFunc: func(ctx context.Context, o client.Object) error {
						return nil
					},
				}
			},
		}
		result, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(asoManagedControlPlane)})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{}))

		err = c.Get(ctx, client.ObjectKeyFromObject(asoManagedControlPlane), asoManagedControlPlane)
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})
}

func TestGetControlPlaneEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		managedCluster *asocontainerservicev1.ManagedCluster
		expected       clusterv1.APIEndpoint
	}{
		{
			name:           "empty",
			managedCluster: &asocontainerservicev1.ManagedCluster{},
			expected:       clusterv1.APIEndpoint{},
		},
		{
			name: "public fqdn",
			managedCluster: &asocontainerservicev1.ManagedCluster{
				Status: asocontainerservicev1.ManagedCluster_STATUS{
					Fqdn: ptr.To("fqdn"),
				},
			},
			expected: clusterv1.APIEndpoint{
				Host: "fqdn",
				Port: 443,
			},
		},
		{
			name: "private fqdn",
			managedCluster: &asocontainerservicev1.ManagedCluster{
				Status: asocontainerservicev1.ManagedCluster_STATUS{
					PrivateFQDN: ptr.To("fqdn"),
				},
			},
			expected: clusterv1.APIEndpoint{
				Host: "fqdn",
				Port: 443,
			},
		},
		{
			name: "public and private fqdn",
			managedCluster: &asocontainerservicev1.ManagedCluster{
				Status: asocontainerservicev1.ManagedCluster_STATUS{
					PrivateFQDN: ptr.To("private"),
					Fqdn:        ptr.To("public"),
				},
			},
			expected: clusterv1.APIEndpoint{
				Host: "private",
				Port: 443,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			g.Expect(getControlPlaneEndpoint(test.managedCluster)).To(Equal(test.expected))
		})
	}
}

func mcJSON(g Gomega, mc *asocontainerservicev1.ManagedCluster) []byte {
	mc.SetGroupVersionKind(asocontainerservicev1.GroupVersion.WithKind("ManagedCluster"))
	j, err := json.Marshal(mc)
	g.Expect(err).NotTo(HaveOccurred())
	return j
}

type fakeASOCredentialCache func(context.Context, client.Object) (azcore.TokenCredential, error)

func (t fakeASOCredentialCache) authTokenForASOResource(ctx context.Context, obj client.Object) (azcore.TokenCredential, error) {
	return t(ctx, obj)
}

type fakeTokenCredential struct {
	token     string
	expiresOn time.Time
}

func (t fakeTokenCredential) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: t.token, ExpiresOn: t.expiresOn}, nil
}

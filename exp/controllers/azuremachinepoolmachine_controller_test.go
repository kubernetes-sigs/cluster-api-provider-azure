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
	"time"

	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	gomock2 "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	reconcilerutils "sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
)

func TestAzureMachinePoolMachineReconciler_Reconcile(t *testing.T) {
	cases := []struct {
		Name   string
		Setup  func(cb *fake.ClientBuilder, reconciler *mock_azure.MockReconcilerMockRecorder)
		Verify func(g *WithT, c client.Client, result ctrl.Result, err error)
	}{
		{
			Name: "should successfully reconcile",
			Setup: func(cb *fake.ClientBuilder, reconciler *mock_azure.MockReconcilerMockRecorder) {
				objects := getReadyMachinePoolMachineClusterObjects(false, nil)
				reconciler.Reconcile(gomock2.AContext()).Return(nil)
				cb.WithObjects(objects...)
			},
			Verify: func(g *WithT, c client.Client, result ctrl.Result, err error) {
				g.Expect(err).NotTo(HaveOccurred())
			},
		},
		{
			Name: "should not set failure properties if VMSS VM has state Failed",
			Setup: func(cb *fake.ClientBuilder, reconciler *mock_azure.MockReconcilerMockRecorder) {
				objects := getReadyMachinePoolMachineClusterObjects(false, ptr.To(infrav1.Failed))
				reconciler.Reconcile(gomock2.AContext()).Return(nil)
				cb.WithObjects(objects...)
			},
			Verify: func(g *WithT, c client.Client, result ctrl.Result, err error) {
				g.Expect(err).NotTo(HaveOccurred())

				ampm := &infrav1exp.AzureMachinePoolMachine{}
				err = c.Get(context.Background(), types.NamespacedName{
					Name:      "ampm1",
					Namespace: "default",
				}, ampm)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ampm.Status.FailureReason).To(BeNil())
				g.Expect(ampm.Status.FailureMessage).To(BeNil())
			},
		},
		{
			Name: "should successfully delete",
			Setup: func(cb *fake.ClientBuilder, reconciler *mock_azure.MockReconcilerMockRecorder) {
				objects := getReadyMachinePoolMachineClusterObjects(true, nil)
				reconciler.Delete(gomock2.AContext()).Return(nil)
				cb.WithObjects(objects...)
			},
			Verify: func(g *WithT, c client.Client, result ctrl.Result, err error) {
				g.Expect(err).NotTo(HaveOccurred())
			},
		},
		{
			Name: "should delete Machine if VMSS VM has state Deleting",
			Setup: func(cb *fake.ClientBuilder, reconciler *mock_azure.MockReconcilerMockRecorder) {
				objects := getReadyMachinePoolMachineClusterObjects(false, ptr.To(infrav1.Deleting))
				reconciler.Reconcile(gomock2.AContext()).Return(nil)
				cb.WithObjects(objects...)
			},
			Verify: func(g *WithT, c client.Client, result ctrl.Result, err error) {
				g.Expect(err).NotTo(HaveOccurred())

				machine := &clusterv1.Machine{}
				err = c.Get(context.Background(), types.NamespacedName{
					Name:      "ma1",
					Namespace: "default",
				}, machine)
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).Should(ContainSubstring("not found"))
			},
		},
		{
			Name: "should remove finalizer if Machine is not found and AzureMachinePool has deletionTimestamp set",
			Setup: func(cb *fake.ClientBuilder, reconciler *mock_azure.MockReconcilerMockRecorder) {
				objects := getDeletingMachinePoolObjects()
				cb.WithObjects(objects...)
			},
			Verify: func(g *WithT, c client.Client, result ctrl.Result, err error) {
				g.Expect(err).NotTo(HaveOccurred())

				ampm := &infrav1exp.AzureMachinePoolMachine{}
				err = c.Get(context.Background(), types.NamespacedName{
					Name:      "ampm1",
					Namespace: "default",
				}, ampm)
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).Should(ContainSubstring("not found"))
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			var (
				g          = NewWithT(t)
				mockCtrl   = gomock.NewController(t)
				reconciler = mock_azure.NewMockReconciler(mockCtrl)
				scheme     = func() *runtime.Scheme {
					s := runtime.NewScheme()
					for _, addTo := range []func(s *runtime.Scheme) error{
						clusterv1.AddToScheme,
						expv1.AddToScheme,
						infrav1.AddToScheme,
						infrav1exp.AddToScheme,
						corev1.AddToScheme,
					} {
						g.Expect(addTo(s)).To(Succeed())
					}

					return s
				}()
				cb = fake.NewClientBuilder().WithScheme(scheme)
			)
			defer mockCtrl.Finish()

			c.Setup(cb, reconciler.EXPECT())
			cl := cb.Build()
			controller := NewAzureMachinePoolMachineController(cl, record.NewFakeRecorder(1), reconcilerutils.Timeouts{}, "foo", azure.NewCredentialCache())
			controller.reconcilerFactory = func(_ *scope.MachinePoolMachineScope) (azure.Reconciler, error) {
				return reconciler, nil
			}
			res, err := controller.Reconcile(context.TODO(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      "ampm1",
					Namespace: "default",
				},
			})
			c.Verify(g, cl, res, err)
		})
	}
}

func getReadyMachinePoolMachineClusterObjects(ampmIsDeleting bool, ampmProvisioningState *infrav1.ProvisioningState) []client.Object {
	azCluster := &infrav1.AzureCluster{
		TypeMeta: metav1.TypeMeta{
			Kind: "AzureCluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "azCluster1",
			Namespace: "default",
		},
		Spec: infrav1.AzureClusterSpec{
			AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
				SubscriptionID: "subID",
				IdentityRef: &corev1.ObjectReference{
					Name:      "fake-identity",
					Namespace: "default",
					Kind:      "AzureClusterIdentity",
				},
			},
		},
	}

	cluster := &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind: "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster1",
			Namespace: "default",
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &corev1.ObjectReference{
				Name:      azCluster.Name,
				Namespace: "default",
				Kind:      "AzureCluster",
			},
		},
		Status: clusterv1.ClusterStatus{
			InfrastructureReady: true,
		},
	}

	mp := &expv1.MachinePool{
		TypeMeta: metav1.TypeMeta{
			Kind: "MachinePool",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mp1",
			Namespace: "default",
			Labels: map[string]string{
				"cluster.x-k8s.io/cluster-name": cluster.Name,
			},
		},
	}

	amp := &infrav1exp.AzureMachinePool{
		TypeMeta: metav1.TypeMeta{
			Kind: "AzureMachinePool",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "amp1",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       mp.Name,
					Kind:       "MachinePool",
					APIVersion: expv1.GroupVersion.String(),
				},
			},
		},
	}

	ma := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ma1",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       mp.Name,
					Kind:       "MachinePool",
					APIVersion: expv1.GroupVersion.String(),
				},
			},
			Labels: map[string]string{
				"cluster.x-k8s.io/cluster-name": cluster.Name,
			},
		},
	}

	ampm := &infrav1exp.AzureMachinePoolMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "ampm1",
			Namespace:  "default",
			Finalizers: []string{"test"},
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: cluster.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       amp.Name,
					Kind:       infrav1.AzureMachinePoolKind,
					APIVersion: infrav1exp.GroupVersion.String(),
				},
				{
					Name:       ma.Name,
					Kind:       "Machine",
					APIVersion: clusterv1.GroupVersion.String(),
				},
			},
		},
	}

	fakeIdentity := &infrav1.AzureClusterIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-identity",
			Namespace: "default",
		},
		Spec: infrav1.AzureClusterIdentitySpec{
			Type: infrav1.ServicePrincipal,
			ClientSecret: corev1.SecretReference{
				Name:      "fooSecret",
				Namespace: "default",
			},
			TenantID: "fake-tenantid",
		},
	}

	fakeSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fooSecret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"clientSecret": []byte("fooSecret"),
		},
	}

	if ampmIsDeleting {
		ampm.DeletionTimestamp = &metav1.Time{
			Time: time.Now(),
		}
	}
	if ampmProvisioningState != nil {
		ampm.Status = infrav1exp.AzureMachinePoolMachineStatus{
			ProvisioningState: ampmProvisioningState,
		}
	}

	return []client.Object{cluster, azCluster, mp, amp, ma, ampm, fakeIdentity, fakeSecret}
}

func getDeletingMachinePoolObjects() []client.Object {
	azCluster := &infrav1.AzureCluster{
		TypeMeta: metav1.TypeMeta{
			Kind: "AzureCluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "azCluster1",
			Namespace: "default",
		},
		Spec: infrav1.AzureClusterSpec{
			AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
				SubscriptionID: "subID",
				IdentityRef: &corev1.ObjectReference{
					Name:      "fake-identity",
					Namespace: "default",
					Kind:      "AzureClusterIdentity",
				},
			},
		},
	}

	cluster := &clusterv1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind: "Cluster",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster1",
			Namespace: "default",
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &corev1.ObjectReference{
				Name:      azCluster.Name,
				Namespace: "default",
				Kind:      "AzureCluster",
			},
		},
		Status: clusterv1.ClusterStatus{
			InfrastructureReady: true,
		},
	}

	mp := &expv1.MachinePool{
		TypeMeta: metav1.TypeMeta{
			Kind: "MachinePool",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "mp1",
			Namespace:  "default",
			Finalizers: []string{"test"},
			DeletionTimestamp: &metav1.Time{
				Time: time.Now(),
			},
			Labels: map[string]string{
				"cluster.x-k8s.io/cluster-name": cluster.Name,
			},
		},
	}

	amp := &infrav1exp.AzureMachinePool{
		TypeMeta: metav1.TypeMeta{
			Kind: "AzureMachinePool",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "amp1",
			Namespace:  "default",
			Finalizers: []string{"test"},
			DeletionTimestamp: &metav1.Time{
				Time: time.Now(),
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       mp.Name,
					Kind:       "MachinePool",
					APIVersion: expv1.GroupVersion.String(),
				},
			},
		},
	}

	ampm := &infrav1exp.AzureMachinePoolMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ampm1",
			Namespace: "default",
			DeletionTimestamp: &metav1.Time{
				Time: time.Now(),
			},
			Finalizers: []string{infrav1exp.AzureMachinePoolMachineFinalizer},
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: cluster.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       amp.Name,
					Kind:       infrav1.AzureMachinePoolKind,
					APIVersion: infrav1exp.GroupVersion.String(),
				},
			},
		},
	}

	fakeIdentity := &infrav1.AzureClusterIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-identity",
			Namespace: "default",
		},
		Spec: infrav1.AzureClusterIdentitySpec{
			Type: infrav1.ServicePrincipal,
			ClientSecret: corev1.SecretReference{
				Name:      "fooSecret",
				Namespace: "default",
			},
			TenantID: "fake-tenantid",
		},
	}

	fakeSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fooSecret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"clientSecret": []byte("fooSecret"),
		},
	}

	return []client.Object{cluster, azCluster, mp, amp, ampm, fakeIdentity, fakeSecret}
}

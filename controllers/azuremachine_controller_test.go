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
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type TestReconcileInput struct {
	createAzureMachineService func(*scope.MachineScope) (*azureMachineService, error)
	azureMachineOptions       func(am *infrav1.AzureMachine)
	expectedErr               string
	machineScopeFailureReason capierrors.MachineStatusError
	ready                     bool
	cache                     *scope.MachineCache
	skuCache                  scope.SKUCacher
	expectedResult            reconcile.Result
}

func TestAzureMachineReconcile(t *testing.T) {
	g := NewWithT(t)
	scheme, err := newScheme()
	g.Expect(err).NotTo(HaveOccurred())

	defaultCluster := getFakeCluster()
	defaultAzureCluster := getFakeAzureCluster()
	defaultAzureMachine := getFakeAzureMachine()
	defaultMachine := getFakeMachine(defaultAzureMachine)

	cases := map[string]struct {
		objects []runtime.Object
		fail    bool
		err     string
		event   string
	}{
		"should reconcile normally": {
			objects: []runtime.Object{
				defaultCluster,
				defaultAzureCluster,
				defaultAzureMachine,
				defaultMachine,
			},
		},
		"should not fail if the azure machine is not found": {
			objects: []runtime.Object{
				defaultCluster,
				defaultAzureCluster,
				defaultMachine,
			},
		},
		"should fail if machine is not found": {
			objects: []runtime.Object{
				defaultCluster,
				defaultAzureCluster,
				defaultAzureMachine,
			},
			fail: true,
			err:  "machines.cluster.x-k8s.io \"my-machine\" not found",
		},
		"should return if azureMachine has not yet set ownerref": {
			objects: []runtime.Object{
				defaultCluster,
				defaultAzureCluster,
				getFakeAzureMachine(func(am *infrav1.AzureMachine) {
					am.OwnerReferences = nil
				}),
				defaultMachine,
			},
			event: "Machine controller dependency not yet met",
		},
		"should return if cluster does not exist": {
			objects: []runtime.Object{
				defaultAzureCluster,
				defaultAzureMachine,
				defaultMachine,
			},
			event: "Unable to get cluster from metadata",
		},
		"should return if azureCluster does not yet available": {
			objects: []runtime.Object{
				defaultCluster,
				defaultAzureMachine,
				defaultMachine,
			},
			event: "AzureCluster unavailable",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(tc.objects...).
				WithStatusSubresource(
					&infrav1.AzureMachine{},
				).
				Build()

			reconciler := &AzureMachineReconciler{
				Client:   client,
				Recorder: record.NewFakeRecorder(128),
			}

			_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "default",
					Name:      "my-machine",
				},
			})
			if tc.event != "" {
				g.Expect(reconciler.Recorder.(*record.FakeRecorder).Events).To(Receive(ContainSubstring(tc.event)))
			}
			if tc.fail {
				g.Expect(err).To(MatchError(tc.err))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

type fakeSKUCacher struct{}

func (f fakeSKUCacher) Get(context.Context, string, resourceskus.ResourceType) (resourceskus.SKU, error) {
	return resourceskus.SKU{}, errors.New("not implemented")
}

func TestAzureMachineReconcileNormal(t *testing.T) {
	cases := map[string]TestReconcileInput{
		"should reconcile normally": {
			createAzureMachineService: getFakeAzureMachineService,
			cache:                     &scope.MachineCache{},
			ready:                     true,
		},
		"should skip reconciliation if error state is detected on azure machine": {
			azureMachineOptions: func(am *infrav1.AzureMachine) {
				updateError := capierrors.UpdateMachineError
				am.Status.FailureReason = &updateError
			},
			createAzureMachineService: getFakeAzureMachineService,
		},
		"should fail if failed to initialize machine cache": {
			createAzureMachineService: getFakeAzureMachineService,
			cache:                     nil,
			skuCache:                  fakeSKUCacher{},
			expectedErr:               "failed to init machine scope cache",
		},
		"should fail if identities are not ready": {
			azureMachineOptions: func(am *infrav1.AzureMachine) {
				am.Status.Conditions = clusterv1.Conditions{
					{
						Type:   infrav1.VMIdentitiesReadyCondition,
						Reason: infrav1.UserAssignedIdentityMissingReason,
						Status: corev1.ConditionFalse,
					},
				}
			},
			createAzureMachineService: getFakeAzureMachineService,
			cache:                     &scope.MachineCache{},
			expectedErr:               "VM identities are not ready",
		},
		"should fail if azure machine service creator fails": {
			createAzureMachineService: func(*scope.MachineScope) (*azureMachineService, error) {
				return nil, errors.New("failed to create azure machine service")
			},
			cache:       &scope.MachineCache{},
			expectedErr: "failed to create azure machine service",
		},
		"should fail if VM is deleted": {
			createAzureMachineService: getFakeAzureMachineServiceWithVMDeleted,
			machineScopeFailureReason: capierrors.UpdateMachineError,
			cache:                     &scope.MachineCache{},
			expectedErr:               "failed to reconcile AzureMachine",
		},
		"should reconcile if terminal error is received": {
			createAzureMachineService: getFakeAzureMachineServiceWithTerminalError,
			machineScopeFailureReason: capierrors.CreateMachineError,
			cache:                     &scope.MachineCache{},
		},
		"should requeue if transient error is received": {
			createAzureMachineService: getFakeAzureMachineServiceWithTransientError,
			cache:                     &scope.MachineCache{},
			expectedResult:            reconcile.Result{RequeueAfter: 10 * time.Second},
		},
		"should return error for general failures": {
			createAzureMachineService: getFakeAzureMachineServiceWithGeneralError,
			cache:                     &scope.MachineCache{},
			expectedErr:               "failed to reconcile AzureMachine",
		},
	}

	for name, c := range cases {
		tc := c
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)

			reconciler, machineScope, clusterScope, err := getReconcileInputs(tc)
			g.Expect(err).NotTo(HaveOccurred())

			result, err := reconciler.reconcileNormal(context.Background(), machineScope, clusterScope)
			g.Expect(result).To(Equal(tc.expectedResult))

			if tc.ready {
				g.Expect(machineScope.AzureMachine.Status.Ready).To(BeTrue())
			}
			if tc.machineScopeFailureReason != "" {
				g.Expect(machineScope.AzureMachine.Status.FailureReason).ToNot(BeNil())
				g.Expect(*machineScope.AzureMachine.Status.FailureReason).To(Equal(tc.machineScopeFailureReason))
			}
			if tc.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureMachineReconcilePause(t *testing.T) {
	cases := map[string]TestReconcileInput{
		"should pause successfully": {
			createAzureMachineService: getFakeAzureMachineService,
			cache:                     &scope.MachineCache{},
		},
		"should fail if failed to create azure machine service": {
			createAzureMachineService: getFakeAzureMachineServiceWithFailure,
			cache:                     &scope.MachineCache{},
			expectedErr:               "failed to create AzureMachineService",
		},
		"should fail to pause for errors": {
			createAzureMachineService: getFakeAzureMachineServiceWithGeneralError,
			cache:                     &scope.MachineCache{},
			expectedErr:               "failed to pause azure machine service",
		},
	}

	for name, c := range cases {
		tc := c
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)

			reconciler, machineScope, _, err := getReconcileInputs(tc)
			g.Expect(err).NotTo(HaveOccurred())

			result, err := reconciler.reconcilePause(context.Background(), machineScope)
			g.Expect(result).To(Equal(tc.expectedResult))

			if tc.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureMachineReconcileDelete(t *testing.T) {
	cases := map[string]TestReconcileInput{
		"should delete successfully": {
			createAzureMachineService: getFakeAzureMachineService,
			cache:                     &scope.MachineCache{},
		},
		"should fail if failed to create azure machine service": {
			createAzureMachineService: getFakeAzureMachineServiceWithFailure,
			cache:                     &scope.MachineCache{},
			expectedErr:               "failed to create AzureMachineService",
		},
		"should requeue if transient error is received": {
			createAzureMachineService: getFakeAzureMachineServiceWithTransientError,
			cache:                     &scope.MachineCache{},
			expectedResult:            reconcile.Result{RequeueAfter: 10 * time.Second},
		},
		"should fail to delete for non-transient errors": {
			createAzureMachineService: getFakeAzureMachineServiceWithGeneralError,
			cache:                     &scope.MachineCache{},
			expectedErr:               "error deleting AzureMachine",
		},
	}

	for name, c := range cases {
		tc := c
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)

			reconciler, machineScope, clusterScope, err := getReconcileInputs(tc)
			g.Expect(err).NotTo(HaveOccurred())

			result, err := reconciler.reconcileDelete(context.Background(), machineScope, clusterScope)
			g.Expect(result).To(Equal(tc.expectedResult))

			if tc.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func getReconcileInputs(tc TestReconcileInput) (*AzureMachineReconciler, *scope.MachineScope, *scope.ClusterScope, error) {
	scheme, err := newScheme()
	if err != nil {
		return nil, nil, nil, err
	}

	var azureMachine *infrav1.AzureMachine
	if tc.azureMachineOptions != nil {
		azureMachine = getFakeAzureMachine(tc.azureMachineOptions)
	} else {
		azureMachine = getFakeAzureMachine()
	}

	cluster := getFakeCluster()
	azureCluster := getFakeAzureCluster(func(ac *infrav1.AzureCluster) {
		ac.Spec.Location = "westus2"
	})
	machine := getFakeMachine(azureMachine, func(m *clusterv1.Machine) {
		m.Spec.Bootstrap = clusterv1.Bootstrap{
			DataSecretName: ptr.To("fooSecret"),
		}
	})

	objects := []runtime.Object{
		cluster,
		azureCluster,
		machine,
		azureMachine,
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "fooSecret",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"value": []byte("foo"),
			},
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objects...).
		WithStatusSubresource(
			&infrav1.AzureMachine{},
		).
		Build()

	reconciler := &AzureMachineReconciler{
		Client:                    client,
		Recorder:                  record.NewFakeRecorder(128),
		createAzureMachineService: tc.createAzureMachineService,
	}

	clusterScope, err := scope.NewClusterScope(context.Background(), scope.ClusterScopeParams{
		Client:       client,
		Cluster:      cluster,
		AzureCluster: azureCluster,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
		Client:       client,
		Machine:      machine,
		AzureMachine: azureMachine,
		ClusterScope: clusterScope,
		Cache:        tc.cache,
		SKUCache:     tc.skuCache,
	})
	if err != nil {
		return nil, nil, nil, err
	}

	return reconciler, machineScope, clusterScope, nil
}

func getFakeAzureMachineService(machineScope *scope.MachineScope) (*azureMachineService, error) {
	cache, err := resourceskus.GetCache(machineScope, machineScope.Location())
	if err != nil {
		return nil, errors.Wrap(err, "failed creating a NewCache")
	}

	return getDefaultAzureMachineService(machineScope, cache), nil
}

func getFakeAzureMachineServiceWithFailure(_ *scope.MachineScope) (*azureMachineService, error) {
	return nil, errors.New("failed to create AzureMachineService")
}

func getFakeAzureMachineServiceWithVMDeleted(machineScope *scope.MachineScope) (*azureMachineService, error) {
	cache, err := resourceskus.GetCache(machineScope, machineScope.Location())
	if err != nil {
		return nil, errors.Wrap(err, "failed creating a NewCache")
	}

	ams := getDefaultAzureMachineService(machineScope, cache)
	ams.Reconcile = func(context.Context) error {
		return azure.VMDeletedError{}
	}

	return ams, nil
}

func getFakeAzureMachineServiceWithTerminalError(machineScope *scope.MachineScope) (*azureMachineService, error) {
	cache, err := resourceskus.GetCache(machineScope, machineScope.Location())
	if err != nil {
		return nil, errors.Wrap(err, "failed creating a NewCache")
	}

	ams := getDefaultAzureMachineService(machineScope, cache)
	ams.Reconcile = func(context.Context) error {
		return azure.WithTerminalError(errors.New("failed to reconcile AzureMachine"))
	}

	return ams, nil
}

func getFakeAzureMachineServiceWithTransientError(machineScope *scope.MachineScope) (*azureMachineService, error) {
	cache, err := resourceskus.GetCache(machineScope, machineScope.Location())
	if err != nil {
		return nil, errors.Wrap(err, "failed creating a NewCache")
	}

	ams := getDefaultAzureMachineService(machineScope, cache)
	ams.Reconcile = func(context.Context) error {
		return azure.WithTransientError(errors.New("failed to reconcile AzureMachine"), 10*time.Second)
	}
	ams.Delete = func(context.Context) error {
		return azure.WithTransientError(errors.New("failed to reconcile AzureMachine"), 10*time.Second)
	}

	return ams, nil
}

func getFakeAzureMachineServiceWithGeneralError(machineScope *scope.MachineScope) (*azureMachineService, error) {
	cache, err := resourceskus.GetCache(machineScope, machineScope.Location())
	if err != nil {
		return nil, errors.Wrap(err, "failed creating a NewCache")
	}

	ams := getDefaultAzureMachineService(machineScope, cache)
	ams.Reconcile = func(context.Context) error {
		return errors.New("foo error")
	}
	ams.Pause = func(context.Context) error {
		return errors.New("foo error")
	}
	ams.Delete = func(context.Context) error {
		return errors.New("foo error")
	}

	return ams, nil
}

func getDefaultAzureMachineService(machineScope *scope.MachineScope, cache *resourceskus.Cache) *azureMachineService {
	return &azureMachineService{
		scope:    machineScope,
		services: []azure.ServiceReconciler{},
		skuCache: cache,
		Reconcile: func(context.Context) error {
			return nil
		},
		Pause: func(context.Context) error {
			return nil
		},
		Delete: func(context.Context) error {
			return nil
		},
	}
}

func getFakeCluster(changes ...func(*clusterv1.Cluster)) *clusterv1.Cluster {
	input := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster",
			Namespace: "default",
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &corev1.ObjectReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
				Kind:       "AzureCluster",
				Name:       "my-azure-cluster",
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

func getFakeAzureCluster(changes ...func(*infrav1.AzureCluster)) *infrav1.AzureCluster {
	input := &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-azure-cluster",
			Namespace: "default",
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
	for _, change := range changes {
		change(input)
	}

	return input
}

func getFakeAzureMachine(changes ...func(*infrav1.AzureMachine)) *infrav1.AzureMachine {
	input := &infrav1.AzureMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-machine",
			Namespace: "default",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "my-cluster",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "cluster.x-k8s.io/v1beta1",
					Kind:       "Machine",
					Name:       "my-machine",
				},
			},
		},
		Spec: infrav1.AzureMachineSpec{
			VMSize: "Standard_D2s_v3",
		},
	}
	for _, change := range changes {
		change(input)
	}

	return input
}

func getFakeMachine(azureMachine *infrav1.AzureMachine, changes ...func(*clusterv1.Machine)) *clusterv1.Machine {
	input := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-machine",
			Namespace: "default",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "my-cluster",
			},
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "cluster.x-k8s.io/v1beta1",
			Kind:       "Machine",
		},
		Spec: clusterv1.MachineSpec{
			InfrastructureRef: corev1.ObjectReference{
				APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
				Kind:       "AzureMachine",
				Name:       azureMachine.Name,
				Namespace:  azureMachine.Namespace,
			},
			Version: ptr.To("v1.22.0"),
		},
	}
	for _, change := range changes {
		change(input)
	}

	return input
}

func TestConditions(t *testing.T) {
	g := NewWithT(t)
	scheme := setupScheme(g)

	testcases := []struct {
		name               string
		clusterStatus      clusterv1.ClusterStatus
		machine            *clusterv1.Machine
		azureMachine       *infrav1.AzureMachine
		expectedConditions []clusterv1.Condition
	}{
		{
			name: "cluster infrastructure is not ready yet",
			clusterStatus: clusterv1.ClusterStatus{
				InfrastructureReady: false,
			},
			machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "my-cluster",
					},
					Name: "my-machine",
				},
			},
			azureMachine: &infrav1.AzureMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "azure-test1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "test1",
						},
					},
				},
			},
			expectedConditions: []clusterv1.Condition{{
				Type:     "VMRunning",
				Status:   corev1.ConditionFalse,
				Severity: clusterv1.ConditionSeverityInfo,
				Reason:   "WaitingForClusterInfrastructure",
			}},
		},
		{
			name: "bootstrap data secret reference is not yet available",
			clusterStatus: clusterv1.ClusterStatus{
				InfrastructureReady: true,
			},
			machine: &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "my-cluster",
					},
					Name: "my-machine",
				},
			},
			azureMachine: &infrav1.AzureMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "azure-test1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "test1",
						},
					},
				},
			},
			expectedConditions: []clusterv1.Condition{{
				Type:     "VMRunning",
				Status:   corev1.ConditionFalse,
				Severity: clusterv1.ConditionSeverityInfo,
				Reason:   "WaitingForBootstrapData",
			}},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
				},
				Status: tc.clusterStatus,
			}
			azureCluster := &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						SubscriptionID: "123",
					},
				},
			}
			initObjects := []runtime.Object{
				cluster,
				tc.machine,
				azureCluster,
				tc.azureMachine,
			}
			client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()
			recorder := record.NewFakeRecorder(10)

			reconciler := NewAzureMachineReconciler(client, recorder, reconciler.DefaultLoopTimeout, "")

			clusterScope, err := scope.NewClusterScope(context.TODO(), scope.ClusterScopeParams{
				Client:       client,
				Cluster:      cluster,
				AzureCluster: azureCluster,
			})
			g.Expect(err).NotTo(HaveOccurred())

			machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
				Client:       client,
				ClusterScope: clusterScope,
				Machine:      tc.machine,
				AzureMachine: tc.azureMachine,
				Cache:        &scope.MachineCache{},
			})
			g.Expect(err).NotTo(HaveOccurred())

			_, err = reconciler.reconcileNormal(context.TODO(), machineScope, clusterScope)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(len(machineScope.AzureMachine.GetConditions())).To(Equal(len(tc.expectedConditions)))
			for i, c := range machineScope.AzureMachine.GetConditions() {
				g.Expect(conditionsMatch(c, tc.expectedConditions[i])).To(BeTrue())
			}
		})
	}
}

func conditionsMatch(i, j clusterv1.Condition) bool {
	return i.Type == j.Type &&
		i.Status == j.Status &&
		i.Reason == j.Reason &&
		i.Severity == j.Severity
}

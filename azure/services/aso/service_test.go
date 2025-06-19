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

package aso

import (
	"context"
	"testing"

	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/aso/mock_aso"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	reconcilerutils "sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
)

const (
	serviceName   = "test"
	conditionType = clusterv1.ConditionType("Test")
)

func TestServiceReconcile(t *testing.T) {
	t.Run("no specs", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)
		postReconcileErr := errors.New("PostReconcileHook error")
		scope := mock_aso.NewMockScope(mockCtrl)
		scope.EXPECT().UpdatePutStatus(conditionType, serviceName, postReconcileErr)
		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		scope.EXPECT().DefaultedAzureServiceReconcileTimeout().Return(reconcilerutils.DefaultAzureServiceReconcileTimeout)

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         nil,
			name:          serviceName,
			ConditionType: conditionType,
			PostReconcileHook: func(_ context.Context, _ *mock_aso.MockScope, _ error) error {
				return postReconcileErr
			},
		}

		err := s.Reconcile(t.Context())
		g.Expect(err).To(MatchError(postReconcileErr))
	})

	t.Run("CreateOrUpdateResource returns error", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)

		scope := mock_aso.NewMockScope(mockCtrl)
		specs := []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
		}

		reconcileErr := errors.New("CreateOrUpdateResource error")
		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		reconciler.EXPECT().CreateOrUpdateResource(gomockinternal.AContext(), specs[0], serviceName).Return(nil, reconcileErr)
		scope.EXPECT().UpdatePutStatus(conditionType, serviceName, reconcileErr)
		scope.EXPECT().DefaultedAzureServiceReconcileTimeout().Return(reconcilerutils.DefaultAzureServiceReconcileTimeout)

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Reconcile(t.Context())
		g.Expect(err).To(MatchError(reconcileErr))
	})

	t.Run("CreateOrUpdateResource succeeds for all resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)

		scope := mock_aso.NewMockScope(mockCtrl)
		specs := []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
		}

		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		reconciler.EXPECT().CreateOrUpdateResource(gomockinternal.AContext(), specs[0], serviceName).Return(nil, nil)
		reconciler.EXPECT().CreateOrUpdateResource(gomockinternal.AContext(), specs[1], serviceName).Return(nil, nil)
		reconciler.EXPECT().CreateOrUpdateResource(gomockinternal.AContext(), specs[2], serviceName).Return(nil, nil)
		scope.EXPECT().UpdatePutStatus(conditionType, serviceName, nil)
		scope.EXPECT().DefaultedAzureServiceReconcileTimeout().Return(reconcilerutils.DefaultAzureServiceReconcileTimeout)

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Reconcile(t.Context())
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("CreateOrUpdateResource returns not done", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)

		scope := mock_aso.NewMockScope(mockCtrl)
		specs := []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
		}

		reconcileErr := azure.NewOperationNotDoneError(&infrav1.Future{})
		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		reconciler.EXPECT().CreateOrUpdateResource(gomockinternal.AContext(), specs[0], serviceName).Return(nil, nil)
		reconciler.EXPECT().CreateOrUpdateResource(gomockinternal.AContext(), specs[1], serviceName).Return(nil, reconcileErr)
		reconciler.EXPECT().CreateOrUpdateResource(gomockinternal.AContext(), specs[2], serviceName).Return(nil, nil)
		scope.EXPECT().UpdatePutStatus(conditionType, serviceName, reconcileErr)
		scope.EXPECT().DefaultedAzureServiceReconcileTimeout().Return(reconcilerutils.DefaultAzureServiceReconcileTimeout)

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Reconcile(t.Context())
		g.Expect(azure.IsOperationNotDoneError(err)).To(BeTrue())
	})

	t.Run("CreateOrUpdateResource returns not done and another error", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)

		scope := mock_aso.NewMockScope(mockCtrl)
		specs := []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
		}

		reconcileErr := errors.New("non-not done error")
		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		reconciler.EXPECT().CreateOrUpdateResource(gomockinternal.AContext(), specs[0], serviceName).Return(nil, azure.NewOperationNotDoneError(&infrav1.Future{}))
		reconciler.EXPECT().CreateOrUpdateResource(gomockinternal.AContext(), specs[1], serviceName).Return(nil, reconcileErr)
		reconciler.EXPECT().CreateOrUpdateResource(gomockinternal.AContext(), specs[2], serviceName).Return(nil, azure.NewOperationNotDoneError(&infrav1.Future{}))
		scope.EXPECT().UpdatePutStatus(conditionType, serviceName, reconcileErr)
		scope.EXPECT().DefaultedAzureServiceReconcileTimeout().Return(reconcilerutils.DefaultAzureServiceReconcileTimeout)

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Reconcile(t.Context())
		g.Expect(err).To(MatchError(reconcileErr))
	})

	t.Run("CreateOrUpdateResource returns error and runs PostCreateOrUpdateResourceHook and PostReconcileHook", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)

		scope := mock_aso.NewMockScope(mockCtrl)
		specs := []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
		}

		reconcileErr := errors.New("CreateOrUpdateResource error")
		postResourceErr := errors.New("PostCreateOrUpdateResource error")
		postReconcileErr := errors.New("PostReconcile error")
		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		reconciler.EXPECT().CreateOrUpdateResource(gomockinternal.AContext(), specs[0], serviceName).Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a very special name",
			},
		}, reconcileErr)
		scope.EXPECT().UpdatePutStatus(conditionType, serviceName, postReconcileErr)
		scope.EXPECT().DefaultedAzureServiceReconcileTimeout().Return(reconcilerutils.DefaultAzureServiceReconcileTimeout)

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
			PostCreateOrUpdateResourceHook: func(_ context.Context, scopeParam *mock_aso.MockScope, result *asoresourcesv1.ResourceGroup, err error) error {
				g.Expect(scopeParam).To(BeIdenticalTo(scope))
				g.Expect(result.Name).To(Equal("a very special name"))
				g.Expect(err).To(MatchError(reconcileErr))
				return postResourceErr
			},
			PostReconcileHook: func(_ context.Context, scopeParam *mock_aso.MockScope, err error) error {
				g.Expect(scopeParam).To(BeIdenticalTo(scope))
				g.Expect(err).To(MatchError(postResourceErr))
				return postReconcileErr
			},
		}

		err := s.Reconcile(t.Context())
		g.Expect(err).To(MatchError(postReconcileErr))
	})

	t.Run("stale resources are deleted", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)

		scope := mock_aso.NewMockScope(mockCtrl)
		rg0 := &asoresourcesv1.ResourceGroup{ObjectMeta: metav1.ObjectMeta{Name: "spec0"}}
		rg1 := &asoresourcesv1.ResourceGroup{ObjectMeta: metav1.ObjectMeta{Name: "spec1"}}
		specs := []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
			mockSpecExpectingResourceRef(mockCtrl, rg0),
			mockSpecExpectingResourceRef(mockCtrl, rg1),
		}

		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		reconciler.EXPECT().CreateOrUpdateResource(gomockinternal.AContext(), specs[0], serviceName).Return(rg0, azure.NewOperationNotDoneError(&infrav1.Future{}))
		reconciler.EXPECT().CreateOrUpdateResource(gomockinternal.AContext(), specs[1], serviceName).Return(rg1, nil)
		scope.EXPECT().DefaultedAzureServiceReconcileTimeout().Return(reconcilerutils.DefaultAzureServiceReconcileTimeout)

		sch := runtime.NewScheme()
		g.Expect(asoresourcesv1.AddToScheme(sch)).To(Succeed())
		ctrlClient := fake.NewClientBuilder().
			WithScheme(sch).
			Build()

		deleteErr := errors.New("delete error")

		deleteMe := &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "delete-me",
				Namespace: "namespace",
			},
		}
		deleteMeToo := &asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "delete-me-too",
				Namespace: "namespace",
			},
		}

		scope.EXPECT().GetClient().Return(ctrlClient).AnyTimes()
		scope.EXPECT().ASOOwner().Return(&asoresourcesv1.ResourceGroup{}).AnyTimes()
		list := func(_ context.Context, _ client.Client, _ ...client.ListOption) ([]*asoresourcesv1.ResourceGroup, error) {
			return []*asoresourcesv1.ResourceGroup{
				deleteMe,
				rg0,
				deleteMeToo,
				rg1,
			}, nil
		}
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), deleteMe, serviceName).Return(deleteErr)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), deleteMeToo, serviceName).Return(azure.NewOperationNotDoneError(&infrav1.Future{}))
		scope.EXPECT().UpdatePutStatus(conditionType, serviceName, deleteErr)

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			ListFunc:      list,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Reconcile(t.Context())
		g.Expect(err).To(MatchError(deleteErr))
	})
}

func TestServiceDelete(t *testing.T) {
	t.Run("no specs", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)
		scope := mock_aso.NewMockScope(mockCtrl)
		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		scope.EXPECT().DefaultedAzureServiceReconcileTimeout().Return(reconcilerutils.DefaultAzureServiceReconcileTimeout)

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         nil,
			name:          serviceName,
			ConditionType: conditionType,
			PostDeleteHook: func(_ context.Context, _ *mock_aso.MockScope, _ error) error {
				return errors.New("hook should not be called")
			},
		}

		err := s.Delete(t.Context())
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("DeleteResource returns error", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)

		scope := mock_aso.NewMockScope(mockCtrl)
		specs := []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
			mockSpecExpectingResourceRef(mockCtrl, &asoresourcesv1.ResourceGroup{}),
		}

		deleteErr := errors.New("DeleteResource error")
		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[0].ResourceRef(), serviceName).Return(deleteErr)
		scope.EXPECT().UpdateDeleteStatus(conditionType, serviceName, deleteErr)
		scope.EXPECT().DefaultedAzureServiceReconcileTimeout().Return(reconcilerutils.DefaultAzureServiceReconcileTimeout)

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Delete(t.Context())
		g.Expect(err).To(MatchError(deleteErr))
	})

	t.Run("DeleteResource succeeds for all resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)

		scope := mock_aso.NewMockScope(mockCtrl)
		specs := []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
			mockSpecExpectingResourceRef(mockCtrl, &asoresourcesv1.ResourceGroup{}),
			mockSpecExpectingResourceRef(mockCtrl, &asoresourcesv1.ResourceGroup{}),
			mockSpecExpectingResourceRef(mockCtrl, &asoresourcesv1.ResourceGroup{}),
		}

		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[0].ResourceRef(), serviceName).Return(nil)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[1].ResourceRef(), serviceName).Return(nil)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[2].ResourceRef(), serviceName).Return(nil)
		scope.EXPECT().UpdateDeleteStatus(conditionType, serviceName, nil)
		scope.EXPECT().DefaultedAzureServiceReconcileTimeout().Return(reconcilerutils.DefaultAzureServiceReconcileTimeout)

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Delete(t.Context())
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("DeleteResource returns not done", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)

		scope := mock_aso.NewMockScope(mockCtrl)
		specs := []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
			mockSpecExpectingResourceRef(mockCtrl, &asoresourcesv1.ResourceGroup{}),
			mockSpecExpectingResourceRef(mockCtrl, &asoresourcesv1.ResourceGroup{}),
			mockSpecExpectingResourceRef(mockCtrl, &asoresourcesv1.ResourceGroup{}),
		}

		deleteErr := azure.NewOperationNotDoneError(&infrav1.Future{})
		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[0].ResourceRef(), serviceName).Return(nil)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[1].ResourceRef(), serviceName).Return(deleteErr)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[2].ResourceRef(), serviceName).Return(nil)
		scope.EXPECT().UpdateDeleteStatus(conditionType, serviceName, deleteErr)
		scope.EXPECT().DefaultedAzureServiceReconcileTimeout().Return(reconcilerutils.DefaultAzureServiceReconcileTimeout)

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Delete(t.Context())
		g.Expect(azure.IsOperationNotDoneError(err)).To(BeTrue())
	})

	t.Run("DeleteResource returns not done and another error", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)

		scope := mock_aso.NewMockScope(mockCtrl)
		specs := []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
			mockSpecExpectingResourceRef(mockCtrl, &asoresourcesv1.ResourceGroup{}),
			mockSpecExpectingResourceRef(mockCtrl, &asoresourcesv1.ResourceGroup{}),
			mockSpecExpectingResourceRef(mockCtrl, &asoresourcesv1.ResourceGroup{}),
		}

		deleteErr := errors.New("non-not done error")
		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[0].ResourceRef(), serviceName).Return(azure.NewOperationNotDoneError(&infrav1.Future{}))
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[1].ResourceRef(), serviceName).Return(deleteErr)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[2].ResourceRef(), serviceName).Return(azure.NewOperationNotDoneError(&infrav1.Future{}))
		scope.EXPECT().UpdateDeleteStatus(conditionType, serviceName, deleteErr)
		scope.EXPECT().DefaultedAzureServiceReconcileTimeout().Return(reconcilerutils.DefaultAzureServiceReconcileTimeout)

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Delete(t.Context())
		g.Expect(err).To(MatchError(deleteErr))
	})

	t.Run("DeleteResource returns error and runs PostDeleteHook", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)

		scope := mock_aso.NewMockScope(mockCtrl)
		specs := []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
			mockSpecExpectingResourceRef(mockCtrl, &asoresourcesv1.ResourceGroup{}),
		}

		deleteErr := errors.New("DeleteResource error")
		postErr := errors.New("PostDeleteHook error")
		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[0].ResourceRef(), serviceName).Return(deleteErr)
		scope.EXPECT().UpdateDeleteStatus(conditionType, serviceName, postErr)
		scope.EXPECT().DefaultedAzureServiceReconcileTimeout().Return(reconcilerutils.DefaultAzureServiceReconcileTimeout)

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
			PostDeleteHook: func(_ context.Context, scopeParam *mock_aso.MockScope, err error) error {
				g.Expect(scopeParam).To(BeIdenticalTo(scope))
				g.Expect(err).To(MatchError(deleteErr))
				return postErr
			},
		}

		err := s.Delete(t.Context())
		g.Expect(err).To(MatchError(postErr))
	})
}

func TestServicePause(t *testing.T) {
	t.Run("PauseResource succeeds for all resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)

		scope := mock_aso.NewMockScope(mockCtrl)
		specs := []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
			mockSpecExpectingResourceRef(mockCtrl, &asoresourcesv1.ResourceGroup{}),
			mockSpecExpectingResourceRef(mockCtrl, &asoresourcesv1.ResourceGroup{}),
			mockSpecExpectingResourceRef(mockCtrl, &asoresourcesv1.ResourceGroup{}),
		}

		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		reconciler.EXPECT().PauseResource(gomockinternal.AContext(), specs[0].ResourceRef(), serviceName).Return(nil)
		reconciler.EXPECT().PauseResource(gomockinternal.AContext(), specs[1].ResourceRef(), serviceName).Return(nil)
		reconciler.EXPECT().PauseResource(gomockinternal.AContext(), specs[2].ResourceRef(), serviceName).Return(nil)

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Pause(t.Context())
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("PauseResource fails for one resource", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)

		scope := mock_aso.NewMockScope(mockCtrl)
		specs := []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
			mockSpecExpectingResourceRef(mockCtrl, &asoresourcesv1.ResourceGroup{}),
			mockSpecExpectingResourceRef(mockCtrl, &asoresourcesv1.ResourceGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "namespace",
				},
			}),
			mockSpecExpectingResourceRef(mockCtrl, &asoresourcesv1.ResourceGroup{}),
		}
		scope.EXPECT().ASOOwner().Return(&asoresourcesv1.ResourceGroup{})

		pauseErr := errors.New("Pause error")
		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		reconciler.EXPECT().PauseResource(gomockinternal.AContext(), specs[0].ResourceRef(), serviceName).Return(nil)
		reconciler.EXPECT().PauseResource(gomockinternal.AContext(), specs[1].ResourceRef(), serviceName).Return(pauseErr)

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Pause(t.Context())
		g.Expect(err).To(MatchError(pauseErr))
	})
}

func mockSpecExpectingResourceRef(ctrl *gomock.Controller, resource *asoresourcesv1.ResourceGroup) azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup] {
	spec := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](ctrl)
	spec.EXPECT().ResourceRef().Return(resource).AnyTimes()
	return spec
}

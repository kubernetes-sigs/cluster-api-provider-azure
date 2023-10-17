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
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/aso/mock_aso"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	serviceName   = "test"
	conditionType = clusterv1.ConditionType("Test")
)

func TestServiceReconcile(t *testing.T) {
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

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Reconcile(context.Background())
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

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Reconcile(context.Background())
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

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Reconcile(context.Background())
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

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Reconcile(context.Background())
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

		err := s.Reconcile(context.Background())
		g.Expect(err).To(MatchError(postReconcileErr))
	})
}

func TestServiceDelete(t *testing.T) {
	t.Run("DeleteResource returns error", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)

		scope := mock_aso.NewMockScope(mockCtrl)
		specs := []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
		}

		deleteErr := errors.New("DeleteResource error")
		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[0], serviceName).Return(deleteErr)
		scope.EXPECT().UpdateDeleteStatus(conditionType, serviceName, deleteErr)

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Delete(context.Background())
		g.Expect(err).To(MatchError("DeleteResource error"))
	})

	t.Run("DeleteResource succeeds for all resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)

		scope := mock_aso.NewMockScope(mockCtrl)
		specs := []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
		}

		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[0], serviceName).Return(nil)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[1], serviceName).Return(nil)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[2], serviceName).Return(nil)
		scope.EXPECT().UpdateDeleteStatus(conditionType, serviceName, nil)

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Delete(context.Background())
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("DeleteResource returns not done", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)

		scope := mock_aso.NewMockScope(mockCtrl)
		specs := []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
		}

		deleteErr := azure.NewOperationNotDoneError(&infrav1.Future{})
		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[0], serviceName).Return(nil)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[1], serviceName).Return(deleteErr)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[2], serviceName).Return(nil)
		scope.EXPECT().UpdateDeleteStatus(conditionType, serviceName, deleteErr)

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Delete(context.Background())
		g.Expect(azure.IsOperationNotDoneError(err)).To(BeTrue())
	})

	t.Run("DeleteResource returns not done and another error", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)

		scope := mock_aso.NewMockScope(mockCtrl)
		specs := []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
		}

		deleteErr := errors.New("non-not done error")
		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[0], serviceName).Return(azure.NewOperationNotDoneError(&infrav1.Future{}))
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[1], serviceName).Return(deleteErr)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[2], serviceName).Return(azure.NewOperationNotDoneError(&infrav1.Future{}))
		scope.EXPECT().UpdateDeleteStatus(conditionType, serviceName, deleteErr)

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Delete(context.Background())
		g.Expect(err).To(MatchError("non-not done error"))
	})

	t.Run("DeleteResource returns error and runs PostDeleteHook", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)

		scope := mock_aso.NewMockScope(mockCtrl)
		specs := []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
		}

		deleteErr := errors.New("DeleteResource error")
		postErr := errors.New("PostDeleteHook error")
		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		reconciler.EXPECT().DeleteResource(gomockinternal.AContext(), specs[0], serviceName).Return(deleteErr)
		scope.EXPECT().UpdateDeleteStatus(conditionType, serviceName, postErr)

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

		err := s.Delete(context.Background())
		g.Expect(err).To(MatchError(postErr))
	})
}

func TestServicePause(t *testing.T) {
	t.Run("PauseResource succeeds for all resources", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)

		scope := mock_aso.NewMockScope(mockCtrl)
		specs := []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
		}

		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		reconciler.EXPECT().PauseResource(gomockinternal.AContext(), specs[0], serviceName).Return(nil)
		reconciler.EXPECT().PauseResource(gomockinternal.AContext(), specs[1], serviceName).Return(nil)
		reconciler.EXPECT().PauseResource(gomockinternal.AContext(), specs[2], serviceName).Return(nil)

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Pause(context.Background())
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("PauseResource fails for one resource", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mockCtrl := gomock.NewController(t)

		scope := mock_aso.NewMockScope(mockCtrl)
		failSpec := mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl)
		failSpec.EXPECT().ResourceRef().Return(&asoresourcesv1.ResourceGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "name",
				Namespace: "namespace",
			},
		})
		specs := []azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
			failSpec,
			mock_azure.NewMockASOResourceSpecGetter[*asoresourcesv1.ResourceGroup](mockCtrl),
		}

		reconciler := mock_aso.NewMockReconciler[*asoresourcesv1.ResourceGroup](mockCtrl)
		reconciler.EXPECT().PauseResource(gomockinternal.AContext(), specs[0], serviceName).Return(nil)
		reconciler.EXPECT().PauseResource(gomockinternal.AContext(), specs[1], serviceName).Return(errors.New("Pause error"))

		s := &Service[*asoresourcesv1.ResourceGroup, *mock_aso.MockScope]{
			Reconciler:    reconciler,
			Scope:         scope,
			Specs:         specs,
			name:          serviceName,
			ConditionType: conditionType,
		}

		err := s.Pause(context.Background())
		g.Expect(err).To(MatchError(ContainSubstring("Pause error")))
	})
}

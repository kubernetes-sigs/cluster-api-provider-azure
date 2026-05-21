/*
Copyright The Kubernetes Authors.

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

package azure_test

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_reconciler"
)

var (
	errFoo            = errors.New("foo error")
	errBar            = errors.New("bar error")
	opNotDoneErr      = azure.NewOperationNotDoneError(&infrav1.Future{Type: "create", ResourceGroup: "rg", Name: "res"})
	testConditionType = clusterv1beta1.ConditionType("TestCondition")
)

func TestReconcileAll(t *testing.T) {
	tests := []struct {
		name      string
		specs     []azure.ResourceSpecGetter
		setupMock func(reconciler *mock_reconciler.MockResourceReconciler, updater *mock_azure.MockAsyncStatusUpdater)
		expectErr error
	}{
		{
			name:  "no specs returns nil without status update",
			specs: nil,
			setupMock: func(_ *mock_reconciler.MockResourceReconciler, _ *mock_azure.MockAsyncStatusUpdater) {
			},
			expectErr: nil,
		},
		{
			name:  "all specs succeed",
			specs: []azure.ResourceSpecGetter{&mock_azure.MockResourceSpecGetter{}},
			setupMock: func(reconciler *mock_reconciler.MockResourceReconciler, updater *mock_azure.MockAsyncStatusUpdater) {
				reconciler.EXPECT().CreateOrUpdateResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
				updater.EXPECT().UpdatePutStatus(testConditionType, "test-service", nil)
			},
			expectErr: nil,
		},
		{
			name:  "one spec returns OperationNotDoneError",
			specs: []azure.ResourceSpecGetter{&mock_azure.MockResourceSpecGetter{}},
			setupMock: func(reconciler *mock_reconciler.MockResourceReconciler, updater *mock_azure.MockAsyncStatusUpdater) {
				reconciler.EXPECT().CreateOrUpdateResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, opNotDoneErr)
				updater.EXPECT().UpdatePutStatus(testConditionType, "test-service", opNotDoneErr)
			},
			expectErr: opNotDoneErr,
		},
		{
			name:  "one spec returns real error",
			specs: []azure.ResourceSpecGetter{&mock_azure.MockResourceSpecGetter{}},
			setupMock: func(reconciler *mock_reconciler.MockResourceReconciler, updater *mock_azure.MockAsyncStatusUpdater) {
				reconciler.EXPECT().CreateOrUpdateResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errFoo)
				updater.EXPECT().UpdatePutStatus(testConditionType, "test-service", errFoo)
			},
			expectErr: errFoo,
		},
		{
			name:  "real error takes precedence over OperationNotDoneError",
			specs: []azure.ResourceSpecGetter{&mock_azure.MockResourceSpecGetter{}, &mock_azure.MockResourceSpecGetter{}},
			setupMock: func(reconciler *mock_reconciler.MockResourceReconciler, updater *mock_azure.MockAsyncStatusUpdater) {
				first := reconciler.EXPECT().CreateOrUpdateResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, opNotDoneErr)
				reconciler.EXPECT().CreateOrUpdateResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errFoo).After(first)
				updater.EXPECT().UpdatePutStatus(testConditionType, "test-service", errFoo)
			},
			expectErr: errFoo,
		},
		{
			name:  "first real error wins over later OperationNotDoneError",
			specs: []azure.ResourceSpecGetter{&mock_azure.MockResourceSpecGetter{}, &mock_azure.MockResourceSpecGetter{}},
			setupMock: func(reconciler *mock_reconciler.MockResourceReconciler, updater *mock_azure.MockAsyncStatusUpdater) {
				first := reconciler.EXPECT().CreateOrUpdateResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errFoo)
				reconciler.EXPECT().CreateOrUpdateResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, opNotDoneErr).After(first)
				updater.EXPECT().UpdatePutStatus(testConditionType, "test-service", errFoo)
			},
			expectErr: errFoo,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			reconciler := mock_reconciler.NewMockResourceReconciler(mockCtrl)
			updater := mock_azure.NewMockAsyncStatusUpdater(mockCtrl)
			tc.setupMock(reconciler, updater)

			err := azure.ReconcileAll(t.Context(), reconciler, updater, tc.specs, "test-service", testConditionType)

			if tc.expectErr != nil {
				g.Expect(err).To(MatchError(tc.expectErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteAll(t *testing.T) {
	tests := []struct {
		name      string
		specs     []azure.ResourceSpecGetter
		setupMock func(reconciler *mock_reconciler.MockResourceReconciler, updater *mock_azure.MockAsyncStatusUpdater)
		expectErr error
	}{
		{
			name:  "no specs returns nil without status update",
			specs: nil,
			setupMock: func(_ *mock_reconciler.MockResourceReconciler, _ *mock_azure.MockAsyncStatusUpdater) {
			},
			expectErr: nil,
		},
		{
			name:  "all specs succeed",
			specs: []azure.ResourceSpecGetter{&mock_azure.MockResourceSpecGetter{}},
			setupMock: func(reconciler *mock_reconciler.MockResourceReconciler, updater *mock_azure.MockAsyncStatusUpdater) {
				reconciler.EXPECT().DeleteResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				updater.EXPECT().UpdateDeleteStatus(testConditionType, "test-service", nil)
			},
			expectErr: nil,
		},
		{
			name:  "one spec returns OperationNotDoneError",
			specs: []azure.ResourceSpecGetter{&mock_azure.MockResourceSpecGetter{}},
			setupMock: func(reconciler *mock_reconciler.MockResourceReconciler, updater *mock_azure.MockAsyncStatusUpdater) {
				reconciler.EXPECT().DeleteResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(opNotDoneErr)
				updater.EXPECT().UpdateDeleteStatus(testConditionType, "test-service", opNotDoneErr)
			},
			expectErr: opNotDoneErr,
		},
		{
			name:  "one spec returns real error",
			specs: []azure.ResourceSpecGetter{&mock_azure.MockResourceSpecGetter{}},
			setupMock: func(reconciler *mock_reconciler.MockResourceReconciler, updater *mock_azure.MockAsyncStatusUpdater) {
				reconciler.EXPECT().DeleteResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(errBar)
				updater.EXPECT().UpdateDeleteStatus(testConditionType, "test-service", errBar)
			},
			expectErr: errBar,
		},
		{
			name:  "real error takes precedence over OperationNotDoneError",
			specs: []azure.ResourceSpecGetter{&mock_azure.MockResourceSpecGetter{}, &mock_azure.MockResourceSpecGetter{}},
			setupMock: func(reconciler *mock_reconciler.MockResourceReconciler, updater *mock_azure.MockAsyncStatusUpdater) {
				first := reconciler.EXPECT().DeleteResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(opNotDoneErr)
				reconciler.EXPECT().DeleteResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(errBar).After(first)
				updater.EXPECT().UpdateDeleteStatus(testConditionType, "test-service", errBar)
			},
			expectErr: errBar,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			reconciler := mock_reconciler.NewMockResourceReconciler(mockCtrl)
			updater := mock_azure.NewMockAsyncStatusUpdater(mockCtrl)
			tc.setupMock(reconciler, updater)

			err := azure.DeleteAll(t.Context(), reconciler, updater, tc.specs, "test-service", testConditionType)

			if tc.expectErr != nil {
				g.Expect(err).To(MatchError(tc.expectErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

/*
Copyright 2022 The Kubernetes Authors.

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

package vault

import (
	"context"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/vault/mock_vault"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	fakeVault = Spec{
		Name:          "my-vault",
		ResourceGroup: "my-rg",
		Location:      "eastus",
		ClusterName:   "my-cluster",
		TenantID:      "my-tenant",
	}
	notDoneError  = azure.NewOperationNotDoneError(&infrav1.Future{})
	notFoundError = autorest.DetailedError{StatusCode: 404}
)

func TestReconcileVault(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_vault.MockScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "no vault",
			expectedError: "",
			expect: func(s *mock_vault.MockScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VaultSpec().Return(nil)
			},
		},
		{
			name:          "create vault succeeds, should return no error",
			expectedError: "",
			expect: func(s *mock_vault.MockScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VaultSpec().Return(&fakeVault)
				r.CreateResource(gomockinternal.AContext(), &fakeVault, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.VaultReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "returns not done error when vault creation is long running",
			expectedError: notDoneError.Error(),
			expect: func(s *mock_vault.MockScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VaultSpec().Return(&fakeVault)
				r.CreateResource(gomockinternal.AContext(), &fakeVault, serviceName).Return(nil, notDoneError)
				s.UpdatePutStatus(infrav1.VaultReadyCondition, serviceName, notDoneError)
			},
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			scopeMock := mock_vault.NewMockScope(mockCtrl)
			reconcilerMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), reconcilerMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: reconcilerMock,
			}

			err := s.Reconcile(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteVault(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_vault.MockScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "no vault",
			expectedError: "",
			expect: func(s *mock_vault.MockScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VaultSpec().Return(nil)
			},
		},
		{
			name:          "delete vault succeeds, should return no error",
			expectedError: "",
			expect: func(s *mock_vault.MockScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VaultSpec().Return(&fakeVault)
				r.DeleteResource(gomockinternal.AContext(), &fakeVault, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.VaultReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "returns error when vault deletion fails",
			expectedError: notDoneError.Error(),
			expect: func(s *mock_vault.MockScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VaultSpec().Return(&fakeVault)
				r.DeleteResource(gomockinternal.AContext(), &fakeVault, serviceName).Return(notDoneError)
				s.UpdateDeleteStatus(infrav1.VaultReadyCondition, serviceName, notDoneError)
			},
		},
		{
			name:          "return without error if vault is not found",
			expectedError: "",
			expect: func(s *mock_vault.MockScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VaultSpec().Return(&fakeVault)
				r.DeleteResource(gomockinternal.AContext(), &fakeVault, serviceName).Return(notFoundError)
				s.UpdateDeleteStatus(infrav1.VaultReadyCondition, serviceName, nil)
			},
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			scopeMock := mock_vault.NewMockScope(mockCtrl)
			reconcilerMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), reconcilerMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: reconcilerMock,
			}

			err := s.Delete(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

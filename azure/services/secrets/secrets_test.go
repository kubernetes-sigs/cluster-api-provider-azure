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

package secrets

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/secrets/mock_secrets"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	fakeSecret1 = SecretSpec{
		Name:      "secret-1",
		VaultName: "my-cluster-vault",
		Value:     "secret val",
	}
	fakeSecret2 = SecretSpec{
		Name:      "secret-2",
		VaultName: "my-cluster-vault",
		Value:     "secret val 2",
	}
	errFake      = errors.New("this is an error")
	notDoneError = azure.NewOperationNotDoneError(&infrav1.Future{})
)

func TestReconcileSecrets(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_secrets.MockSecretScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "no secrets",
			expectedError: "",
			expect: func(s *mock_secrets.MockSecretScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.SecretSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "skip reconcile if vm is provisioning has started",
			expectedError: "",
			expect: func(s *mock_secrets.MockSecretScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.SecretSpecs().Return([]azure.ResourceSpecGetter{fakeSecret1, fakeSecret2})
				s.VMState().Return(infrav1.Creating)
			},
		},
		{
			name:          "create multiple secrets succeeds, should return no error",
			expectedError: "",
			expect: func(s *mock_secrets.MockSecretScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.SecretSpecs().Return([]azure.ResourceSpecGetter{&fakeSecret1, &fakeSecret2})
				s.VMState()
				r.CreateResource(gomockinternal.AContext(), &fakeSecret1, serviceName).Return(nil, nil)
				r.CreateResource(gomockinternal.AContext(), &fakeSecret2, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.SecretReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "returns not done error when secret creation is long running",
			expectedError: notDoneError.Error(),
			expect: func(s *mock_secrets.MockSecretScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.SecretSpecs().Return([]azure.ResourceSpecGetter{&fakeSecret1, &fakeSecret2})
				s.VMState()
				r.CreateResource(gomockinternal.AContext(), &fakeSecret1, serviceName).Return(nil, nil)
				r.CreateResource(gomockinternal.AContext(), &fakeSecret2, serviceName).Return(nil, notDoneError)
				s.UpdatePutStatus(infrav1.SecretReadyCondition, serviceName, notDoneError)
			},
		},
		{
			name:          "returns error with more severity when there are multiple errors",
			expectedError: errFake.Error(),
			expect: func(s *mock_secrets.MockSecretScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.SecretSpecs().Return([]azure.ResourceSpecGetter{&fakeSecret1, &fakeSecret2})
				s.VMState()
				r.CreateResource(gomockinternal.AContext(), &fakeSecret1, serviceName).Return(nil, errFake)
				r.CreateResource(gomockinternal.AContext(), &fakeSecret2, serviceName).Return(nil, notDoneError)
				s.UpdatePutStatus(infrav1.SecretReadyCondition, serviceName, errFake)
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

			scopeMock := mock_secrets.NewMockSecretScope(mockCtrl)
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

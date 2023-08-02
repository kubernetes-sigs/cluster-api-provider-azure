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

package bastionhosts

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/client-go/kubernetes/scheme"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	mock_bastionhosts "sigs.k8s.io/cluster-api-provider-azure/azure/services/bastionhosts/mocks_bastionhosts"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var (
	fakeSubnetID         = "my-subnet-id"
	fakePublicIPID       = "my-public-ip-id"
	fakeAzureBastionSpec = AzureBastionSpec{
		Name:        "my-bastion",
		Location:    "westus",
		ClusterName: "my-cluster",
		SubnetID:    fakeSubnetID,
		PublicIPID:  fakePublicIPID,
	}
	internalError = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error")
)

func init() {
	_ = clusterv1.AddToScheme(scheme.Scheme)
}

func TestReconcileBastionHosts(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_bastionhosts.MockBastionScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "bastion successfully created",
			expectedError: "",
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AzureBastionSpec().Return(&fakeAzureBastionSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeAzureBastionSpec, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.BastionHostReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "no bastion spec found",
			expectedError: "",
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AzureBastionSpec().Return(nil)
			},
		},
		{
			name:          "fail to create a bastion",
			expectedError: internalError.Error(),
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AzureBastionSpec().Return(&fakeAzureBastionSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeAzureBastionSpec, serviceName).Return(nil, internalError)
				s.UpdatePutStatus(infrav1.BastionHostReadyCondition, serviceName, internalError)
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
			scopeMock := mock_bastionhosts.NewMockBastionScope(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: asyncMock,
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

func TestDeleteBastionHost(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_bastionhosts.MockBastionScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "successfully delete an existing bastion host",
			expectedError: "",
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AzureBastionSpec().Return(&fakeAzureBastionSpec)
				r.DeleteResource(gomockinternal.AContext(), &fakeAzureBastionSpec, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.BastionHostReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "bastion host deletion fails",
			expectedError: internalError.Error(),
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AzureBastionSpec().Return(&fakeAzureBastionSpec)
				r.DeleteResource(gomockinternal.AContext(), &fakeAzureBastionSpec, serviceName).Return(internalError)
				s.UpdateDeleteStatus(infrav1.BastionHostReadyCondition, serviceName, internalError)
			},
		},
		{
			name:          "no bastion spec found",
			expectedError: "",
			expect: func(s *mock_bastionhosts.MockBastionScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.AzureBastionSpec().Return(nil)
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
			scopeMock := mock_bastionhosts.NewMockBastionScope(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: asyncMock,
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

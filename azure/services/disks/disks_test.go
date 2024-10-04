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

package disks

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/disks/mock_disks"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
)

var (
	diskSpec1 = DiskSpec{
		Name:          "my-disk-1",
		ResourceGroup: "my-group",
	}

	diskSpec2 = DiskSpec{
		Name:          "my-disk-2",
		ResourceGroup: "my-group",
	}

	fakeDiskSpecs = []azure.ResourceSpecGetter{
		&diskSpec1,
		&diskSpec2,
	}

	internalError = &azcore.ResponseError{
		RawResponse: &http.Response{
			Body:       io.NopCloser(strings.NewReader("#: Internal Server Error: StatusCode=500")),
			StatusCode: http.StatusInternalServerError,
		},
	}
)

func TestDeleteDisk(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_disks.MockDiskScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no disk specs are found",
			expectedError: "",
			expect: func(s *mock_disks.MockDiskScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.DiskSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "delete the disk",
			expectedError: "",
			expect: func(s *mock_disks.MockDiskScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DiskSpecs().Return(fakeDiskSpecs)
				gomock.InOrder(
					s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout),
					r.DeleteResource(gomockinternal.AContext(), &diskSpec1, serviceName).Return(nil),
					r.DeleteResource(gomockinternal.AContext(), &diskSpec2, serviceName).Return(nil),
					s.UpdateDeleteStatus(infrav1.DisksReadyCondition, serviceName, nil),
				)
			},
		},
		{
			name:          "disk already deleted",
			expectedError: "",
			expect: func(s *mock_disks.MockDiskScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DiskSpecs().Return(fakeDiskSpecs)
				gomock.InOrder(
					s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout),
					r.DeleteResource(gomockinternal.AContext(), &diskSpec1, serviceName).Return(nil),
					r.DeleteResource(gomockinternal.AContext(), &diskSpec2, serviceName).Return(nil),
					s.UpdateDeleteStatus(infrav1.DisksReadyCondition, serviceName, nil),
				)
			},
		},
		{
			name:          "error while trying to delete the disk",
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_disks.MockDiskScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DiskSpecs().Return(fakeDiskSpecs)
				gomock.InOrder(
					s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout),
					r.DeleteResource(gomockinternal.AContext(), &diskSpec1, serviceName).Return(internalError),
					r.DeleteResource(gomockinternal.AContext(), &diskSpec2, serviceName).Return(nil),
					s.UpdateDeleteStatus(infrav1.DisksReadyCondition, serviceName, internalError),
				)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_disks.NewMockDiskScope(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: asyncMock,
			}

			err := s.Delete(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

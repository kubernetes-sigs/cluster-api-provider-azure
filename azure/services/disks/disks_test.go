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
	"fmt"
	"net/http"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/disks/mock_disks"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
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

	internalError = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error")
	notFoundError = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not Found")
)

func TestDeleteDisk(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_disks.MockDiskScopeMockRecorder, m *mock_disks.MockclientMockRecorder)
	}{
		{
			name:          "delete the disk",
			expectedError: "",
			expect: func(s *mock_disks.MockDiskScopeMockRecorder, m *mock_disks.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.DiskSpecs().Return(fakeDiskSpecs)
				gomock.InOrder(
					s.GetLongRunningOperationState("my-disk-1", serviceName),
					m.DeleteAsync(gomockinternal.AContext(), &diskSpec1).Return(nil, nil),
					s.GetLongRunningOperationState("my-disk-2", serviceName),
					m.DeleteAsync(gomockinternal.AContext(), &diskSpec2).Return(nil, nil),
					s.UpdateDeleteStatus(infrav1.DisksReadyCondition, serviceName, nil),
				)
			},
		},
		{
			name:          "disk already deleted",
			expectedError: "",
			expect: func(s *mock_disks.MockDiskScopeMockRecorder, m *mock_disks.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.DiskSpecs().Return(fakeDiskSpecs)
				gomock.InOrder(
					s.GetLongRunningOperationState("my-disk-1", serviceName),
					m.DeleteAsync(gomockinternal.AContext(), &diskSpec1).Return(nil, notFoundError),
					s.GetLongRunningOperationState("my-disk-2", serviceName),
					m.DeleteAsync(gomockinternal.AContext(), &diskSpec2).Return(nil, nil),
					s.UpdateDeleteStatus(infrav1.DisksReadyCondition, serviceName, nil),
				)
			},
		},
		{
			name:          "error while trying to delete the disk",
			expectedError: "failed to delete resource my-group/my-disk-1 (service: disks): #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_disks.MockDiskScopeMockRecorder, m *mock_disks.MockclientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.DiskSpecs().Return(fakeDiskSpecs)
				gomock.InOrder(
					s.GetLongRunningOperationState("my-disk-1", serviceName),
					m.DeleteAsync(gomockinternal.AContext(), &diskSpec1).Return(nil, internalError),
					s.GetLongRunningOperationState("my-disk-2", serviceName),
					m.DeleteAsync(gomockinternal.AContext(), &diskSpec2).Return(nil, nil),
					s.UpdateDeleteStatus(infrav1.DisksReadyCondition, serviceName, gomockinternal.ErrStrEq(fmt.Sprintf("failed to delete resource my-group/my-disk-1 (service: disks): %s", internalError.Error()))),
				)
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
			scopeMock := mock_disks.NewMockDiskScope(mockCtrl)
			clientMock := mock_disks.NewMockclient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				client: clientMock,
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

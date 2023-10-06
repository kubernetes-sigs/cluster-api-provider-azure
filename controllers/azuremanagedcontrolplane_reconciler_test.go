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

package controllers

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"go.uber.org/mock/gomock"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

func TestAzureManagedControlPlaneServicePause(t *testing.T) {
	type pausingServiceReconciler struct {
		*mock_azure.MockServiceReconciler
		*mock_azure.MockPauser
	}

	cases := map[string]struct {
		expectedError string
		expect        func(one pausingServiceReconciler, two pausingServiceReconciler, three pausingServiceReconciler)
	}{
		"all services are paused in order": {
			expectedError: "",
			expect: func(one pausingServiceReconciler, two pausingServiceReconciler, three pausingServiceReconciler) {
				gomock.InOrder(
					one.MockPauser.EXPECT().Pause(gomockinternal.AContext()).Return(nil),
					two.MockPauser.EXPECT().Pause(gomockinternal.AContext()).Return(nil),
					three.MockPauser.EXPECT().Pause(gomockinternal.AContext()).Return(nil))
			},
		},
		"service pause fails": {
			expectedError: "failed to pause AzureManagedControlPlane service two: some error happened",
			expect: func(one pausingServiceReconciler, two pausingServiceReconciler, _ pausingServiceReconciler) {
				gomock.InOrder(
					one.MockPauser.EXPECT().Pause(gomockinternal.AContext()).Return(nil),
					two.MockPauser.EXPECT().Pause(gomockinternal.AContext()).Return(errors.New("some error happened")),
					two.MockServiceReconciler.EXPECT().Name().Return("two"))
			},
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)

			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			newPausingServiceReconciler := func() pausingServiceReconciler {
				return pausingServiceReconciler{
					mock_azure.NewMockServiceReconciler(mockCtrl),
					mock_azure.NewMockPauser(mockCtrl),
				}
			}
			svcOneMock := newPausingServiceReconciler()
			svcTwoMock := newPausingServiceReconciler()
			svcThreeMock := newPausingServiceReconciler()

			tc.expect(svcOneMock, svcTwoMock, svcThreeMock)

			s := &azureManagedControlPlaneService{
				services: []azure.ServiceReconciler{
					svcOneMock,
					svcTwoMock,
					svcThreeMock,
				},
			}

			err := s.Pause(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

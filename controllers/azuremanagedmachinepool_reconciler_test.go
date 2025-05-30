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

package controllers

import (
	"testing"

	"github.com/onsi/gomega"
	"github.com/pkg/errors"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

func TestIsAgentPoolVMSSNotFoundError(t *testing.T) {
	cases := []struct {
		Name     string
		Err      error
		Expected bool
	}{
		{
			Name:     "WithANotFoundError",
			Err:      NewAgentPoolVMSSNotFoundError("foo", "baz"),
			Expected: true,
		},
		{
			Name:     "WithAWrappedNotFoundError",
			Err:      errors.Wrap(NewAgentPoolVMSSNotFoundError("foo", "baz"), "boom"),
			Expected: true,
		},
		{
			Name:     "NotTheRightKindOfError",
			Err:      errors.New("foo"),
			Expected: false,
		},
		{
			Name:     "NilError",
			Err:      nil,
			Expected: false,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			g.Expect(errors.Is(c.Err, NewAgentPoolVMSSNotFoundError("foo", "baz"))).To(gomega.Equal(c.Expected))
		})
	}
}

func TestAzureManagedMachinePoolServicePause(t *testing.T) {
	type pausingServiceReconciler struct {
		*mock_azure.MockServiceReconciler
		*mock_azure.MockPauser
	}

	cases := map[string]struct {
		expectedError string
		expect        func(svc pausingServiceReconciler)
	}{
		"service paused": {
			expectedError: "",
			expect: func(svc pausingServiceReconciler) {
				gomock.InOrder(
					svc.MockPauser.EXPECT().Pause(gomockinternal.AContext()).Return(nil),
				)
			},
		},
		"service pause fails": {
			expectedError: "failed to pause machine pool ammp: some error happened",
			expect: func(svc pausingServiceReconciler) {
				gomock.InOrder(
					svc.MockPauser.EXPECT().Pause(gomockinternal.AContext()).Return(errors.New("some error happened")),
				)
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := gomega.NewWithT(t)

			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			newPausingServiceReconciler := func() pausingServiceReconciler {
				return pausingServiceReconciler{
					mock_azure.NewMockServiceReconciler(mockCtrl),
					mock_azure.NewMockPauser(mockCtrl),
				}
			}
			svcMock := newPausingServiceReconciler()

			tc.expect(svcMock)

			s := &azureManagedMachinePoolService{
				agentPoolsSvc: svcMock,
				scope: &scope.ManagedMachinePoolScope{
					InfraMachinePool: &infrav1.AzureManagedMachinePool{
						ObjectMeta: metav1.ObjectMeta{
							Name: "ammp",
						},
					},
				},
			}

			err := s.Pause(t.Context())
			if tc.expectedError != "" {
				g.Expect(err).To(gomega.HaveOccurred())
				g.Expect(err).To(gomega.MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(gomega.HaveOccurred())
			}
		})
	}
}

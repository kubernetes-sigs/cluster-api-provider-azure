/*
Copyright 2019 The Kubernetes Authors.

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
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/groups"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/vnetpeerings"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

func TestAzureClusterServiceReconcile(t *testing.T) {
	cases := map[string]struct {
		expectedError string
		expect        func(one *mock_azure.MockServiceReconcilerMockRecorder, two *mock_azure.MockServiceReconcilerMockRecorder, three *mock_azure.MockServiceReconcilerMockRecorder)
	}{
		"all services are reconciled in order": {
			expectedError: "",
			expect: func(one *mock_azure.MockServiceReconcilerMockRecorder, two *mock_azure.MockServiceReconcilerMockRecorder, three *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					one.Reconcile(gomockinternal.AContext()).Return(nil),
					two.Reconcile(gomockinternal.AContext()).Return(nil),
					three.Reconcile(gomockinternal.AContext()).Return(nil))
			},
		},
		"service reconcile fails": {
			expectedError: "failed to reconcile AzureCluster service two: some error happened",
			expect: func(one *mock_azure.MockServiceReconcilerMockRecorder, two *mock_azure.MockServiceReconcilerMockRecorder, three *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					one.Reconcile(gomockinternal.AContext()).Return(nil),
					two.Reconcile(gomockinternal.AContext()).Return(errors.New("some error happened")),
					two.Name().Return("two"))
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
			svcOneMock := mock_azure.NewMockServiceReconciler(mockCtrl)
			svcTwoMock := mock_azure.NewMockServiceReconciler(mockCtrl)
			svcThreeMock := mock_azure.NewMockServiceReconciler(mockCtrl)

			tc.expect(svcOneMock.EXPECT(), svcTwoMock.EXPECT(), svcThreeMock.EXPECT())

			s := &azureClusterService{
				scope: &scope.ClusterScope{
					Cluster:      &clusterv1.Cluster{},
					AzureCluster: &infrav1.AzureCluster{},
				},
				services: []azure.ServiceReconciler{
					svcOneMock,
					svcTwoMock,
					svcThreeMock,
				},
				skuCache: resourceskus.NewStaticCache([]compute.ResourceSku{}, ""),
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

func TestAzureClusterServicePause(t *testing.T) {
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
			expectedError: "failed to pause AzureCluster service two: some error happened",
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

			s := &azureClusterService{
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

func TestAzureClusterServiceDelete(t *testing.T) {
	cases := map[string]struct {
		expectedError string
		expect        func(grp *mock_azure.MockServiceReconcilerMockRecorder, vpr *mock_azure.MockServiceReconcilerMockRecorder, one *mock_azure.MockServiceReconcilerMockRecorder, two *mock_azure.MockServiceReconcilerMockRecorder, three *mock_azure.MockServiceReconcilerMockRecorder)
	}{
		"Resource Group is deleted successfully": {
			expectedError: "",
			expect: func(grp *mock_azure.MockServiceReconcilerMockRecorder, vpr *mock_azure.MockServiceReconcilerMockRecorder, one *mock_azure.MockServiceReconcilerMockRecorder, two *mock_azure.MockServiceReconcilerMockRecorder, three *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					grp.Name().Return(groups.ServiceName),
					grp.IsManaged(gomockinternal.AContext()).Return(true, nil),
					grp.Name().Return(groups.ServiceName),
					vpr.Name().Return(vnetpeerings.ServiceName),
					vpr.Delete(gomockinternal.AContext()).Return(nil),
					grp.Delete(gomockinternal.AContext()).Return(nil))
			},
		},
		"Error when checking if resource group is managed": {
			expectedError: "failed to determine if the AzureCluster resource group is managed: an error happened",
			expect: func(grp *mock_azure.MockServiceReconcilerMockRecorder, vpr *mock_azure.MockServiceReconcilerMockRecorder, one *mock_azure.MockServiceReconcilerMockRecorder, two *mock_azure.MockServiceReconcilerMockRecorder, three *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					grp.Name().Return(groups.ServiceName),
					grp.IsManaged(gomockinternal.AContext()).Return(false, errors.New("an error happened")))
			},
		},
		"Resource Group delete fails": {
			expectedError: "failed to delete resource group: internal error",
			expect: func(grp *mock_azure.MockServiceReconcilerMockRecorder, vpr *mock_azure.MockServiceReconcilerMockRecorder, one *mock_azure.MockServiceReconcilerMockRecorder, two *mock_azure.MockServiceReconcilerMockRecorder, three *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					grp.Name().Return(groups.ServiceName),
					grp.IsManaged(gomockinternal.AContext()).Return(true, nil),
					grp.Name().Return(groups.ServiceName),
					vpr.Name().Return(vnetpeerings.ServiceName),
					vpr.Delete(gomockinternal.AContext()).Return(nil),
					grp.Delete(gomockinternal.AContext()).Return(errors.New("internal error")))
			},
		},
		"Resource Group not owned by cluster": {
			expectedError: "",
			expect: func(grp *mock_azure.MockServiceReconcilerMockRecorder, vpr *mock_azure.MockServiceReconcilerMockRecorder, one *mock_azure.MockServiceReconcilerMockRecorder, two *mock_azure.MockServiceReconcilerMockRecorder, three *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					grp.Name().Return(groups.ServiceName),
					grp.IsManaged(gomockinternal.AContext()).Return(false, nil),
					three.Delete(gomockinternal.AContext()).Return(nil),
					two.Delete(gomockinternal.AContext()).Return(nil),
					one.Delete(gomockinternal.AContext()).Return(nil),
					vpr.Delete(gomockinternal.AContext()).Return(nil),
					grp.Delete(gomockinternal.AContext()).Return(nil))
			},
		},
		"service delete fails": {
			expectedError: "failed to delete AzureCluster service two: some error happened",
			expect: func(grp *mock_azure.MockServiceReconcilerMockRecorder, vpr *mock_azure.MockServiceReconcilerMockRecorder, one *mock_azure.MockServiceReconcilerMockRecorder, two *mock_azure.MockServiceReconcilerMockRecorder, three *mock_azure.MockServiceReconcilerMockRecorder) {
				gomock.InOrder(
					grp.Name().Return(groups.ServiceName),
					grp.IsManaged(gomockinternal.AContext()).Return(false, nil),
					three.Delete(gomockinternal.AContext()).Return(nil),
					two.Delete(gomockinternal.AContext()).Return(errors.New("some error happened")),
					two.Name().Return("two"))
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
			groupsMock := mock_azure.NewMockServiceReconciler(mockCtrl)
			vnetpeeringsMock := mock_azure.NewMockServiceReconciler(mockCtrl)
			svcOneMock := mock_azure.NewMockServiceReconciler(mockCtrl)
			svcTwoMock := mock_azure.NewMockServiceReconciler(mockCtrl)
			svcThreeMock := mock_azure.NewMockServiceReconciler(mockCtrl)

			tc.expect(groupsMock.EXPECT(), vnetpeeringsMock.EXPECT(), svcOneMock.EXPECT(), svcTwoMock.EXPECT(), svcThreeMock.EXPECT())

			s := &azureClusterService{
				scope: &scope.ClusterScope{
					AzureCluster: &infrav1.AzureCluster{},
				},
				services: []azure.ServiceReconciler{
					groupsMock,
					vnetpeeringsMock,
					svcOneMock,
					svcTwoMock,
					svcThreeMock,
				},
				skuCache: resourceskus.NewStaticCache([]compute.ResourceSku{}, ""),
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

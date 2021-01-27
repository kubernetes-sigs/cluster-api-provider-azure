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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/mocks"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

type expect func(grp *mocks.MockServiceMockRecorder, vnet *mocks.MockServiceMockRecorder, sg *mocks.MockServiceMockRecorder, rt *mocks.MockServiceMockRecorder, sn *mocks.MockServiceMockRecorder, pip *mocks.MockServiceMockRecorder, lb *mocks.MockServiceMockRecorder, dns *mocks.MockServiceMockRecorder)

func TestAzureClusterReconcilerDelete(t *testing.T) {
	cases := map[string]struct {
		expectedError string
		expect        expect
	}{
		"Resource Group is deleted successfully": {
			expectedError: "",
			expect: func(grp *mocks.MockServiceMockRecorder, vnet *mocks.MockServiceMockRecorder, sg *mocks.MockServiceMockRecorder, rt *mocks.MockServiceMockRecorder, sn *mocks.MockServiceMockRecorder, pip *mocks.MockServiceMockRecorder, lb *mocks.MockServiceMockRecorder, dns *mocks.MockServiceMockRecorder) {
				gomock.InOrder(
					grp.Delete(gomockinternal.AContext()).Return(nil))
			},
		},
		"Resource Group delete fails": {
			expectedError: "failed to delete resource group: internal error",
			expect: func(grp *mocks.MockServiceMockRecorder, vnet *mocks.MockServiceMockRecorder, sg *mocks.MockServiceMockRecorder, rt *mocks.MockServiceMockRecorder, sn *mocks.MockServiceMockRecorder, pip *mocks.MockServiceMockRecorder, lb *mocks.MockServiceMockRecorder, dns *mocks.MockServiceMockRecorder) {
				gomock.InOrder(
					grp.Delete(gomockinternal.AContext()).Return(errors.New("internal error")))
			},
		},
		"Resource Group not owned by cluster": {
			expectedError: "",
			expect: func(grp *mocks.MockServiceMockRecorder, vnet *mocks.MockServiceMockRecorder, sg *mocks.MockServiceMockRecorder, rt *mocks.MockServiceMockRecorder, sn *mocks.MockServiceMockRecorder, pip *mocks.MockServiceMockRecorder, lb *mocks.MockServiceMockRecorder, dns *mocks.MockServiceMockRecorder) {
				gomock.InOrder(
					grp.Delete(gomockinternal.AContext()).Return(azure.ErrNotOwned),
					dns.Delete(gomockinternal.AContext()),
					lb.Delete(gomockinternal.AContext()),
					pip.Delete(gomockinternal.AContext()),
					sn.Delete(gomockinternal.AContext()),
					rt.Delete(gomockinternal.AContext()),
					sg.Delete(gomockinternal.AContext()),
					vnet.Delete(gomockinternal.AContext()),
				)
			},
		},
		"Load Balancer delete fails": {
			expectedError: "failed to delete load balancer: some error happened",
			expect: func(grp *mocks.MockServiceMockRecorder, vnet *mocks.MockServiceMockRecorder, sg *mocks.MockServiceMockRecorder, rt *mocks.MockServiceMockRecorder, sn *mocks.MockServiceMockRecorder, pip *mocks.MockServiceMockRecorder, lb *mocks.MockServiceMockRecorder, dns *mocks.MockServiceMockRecorder) {
				gomock.InOrder(
					grp.Delete(gomockinternal.AContext()).Return(azure.ErrNotOwned),
					dns.Delete(gomockinternal.AContext()),
					lb.Delete(gomockinternal.AContext()).Return(errors.New("some error happened")),
				)
			},
		},
		"Route table delete fails": {
			expectedError: "failed to delete route table: some error happened",
			expect: func(grp *mocks.MockServiceMockRecorder, vnet *mocks.MockServiceMockRecorder, sg *mocks.MockServiceMockRecorder, rt *mocks.MockServiceMockRecorder, sn *mocks.MockServiceMockRecorder, pip *mocks.MockServiceMockRecorder, lb *mocks.MockServiceMockRecorder, dns *mocks.MockServiceMockRecorder) {
				gomock.InOrder(
					grp.Delete(gomockinternal.AContext()).Return(azure.ErrNotOwned),
					dns.Delete(gomockinternal.AContext()),
					lb.Delete(gomockinternal.AContext()),
					pip.Delete(gomockinternal.AContext()),
					sn.Delete(gomockinternal.AContext()),
					rt.Delete(gomockinternal.AContext()).Return(errors.New("some error happened")),
				)
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
			groupsMock := mocks.NewMockService(mockCtrl)
			vnetMock := mocks.NewMockService(mockCtrl)
			sgMock := mocks.NewMockService(mockCtrl)
			rtMock := mocks.NewMockService(mockCtrl)
			subnetsMock := mocks.NewMockService(mockCtrl)
			publicIPMock := mocks.NewMockService(mockCtrl)
			lbMock := mocks.NewMockService(mockCtrl)
			dnsMock := mocks.NewMockService(mockCtrl)

			tc.expect(groupsMock.EXPECT(), vnetMock.EXPECT(), sgMock.EXPECT(), rtMock.EXPECT(), subnetsMock.EXPECT(), publicIPMock.EXPECT(), lbMock.EXPECT(), dnsMock.EXPECT())

			s := &azureClusterService{
				scope: &scope.ClusterScope{
					AzureCluster: &infrav1.AzureCluster{},
				},
				groupsSvc:        groupsMock,
				vnetSvc:          vnetMock,
				securityGroupSvc: sgMock,
				routeTableSvc:    rtMock,
				subnetsSvc:       subnetsMock,
				publicIPSvc:      publicIPMock,
				loadBalancerSvc:  lbMock,
				privateDNSSvc:    dnsMock,
				skuCache:         resourceskus.NewStaticCache([]compute.ResourceSku{}),
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

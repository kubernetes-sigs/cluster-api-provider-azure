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

package privatedns

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/privatedns/mgmt/2018-09-01/privatedns"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/privatedns/mock_privatedns"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

const (
	zoneName          = "my-zone"
	resourceGroup     = "my-rg"
	vnetName          = "my-vnet"
	vnetResourceGroup = "my-vnet-rg"
	linkName1         = "my-link-1"
	linkName2         = "my-link-2"
	clusterName       = "my-cluster"
	subscriptionID    = "my-subscription-id"
)

var (
	fakeZone = ZoneSpec{
		Name:           zoneName,
		ResourceGroup:  resourceGroup,
		ClusterName:    clusterName,
		AdditionalTags: nil,
	}

	fakeLink1 = LinkSpec{
		Name:              linkName1,
		ZoneName:          zoneName,
		SubscriptionID:    subscriptionID,
		VNetResourceGroup: vnetResourceGroup,
		VNetName:          vnetName,
		ResourceGroup:     resourceGroup,
		ClusterName:       clusterName,
		AdditionalTags:    nil,
	}

	fakeLink2 = LinkSpec{
		Name:              linkName2,
		ZoneName:          zoneName,
		SubscriptionID:    subscriptionID,
		VNetResourceGroup: vnetResourceGroup,
		VNetName:          vnetName,
		ResourceGroup:     resourceGroup,
		ClusterName:       clusterName,
		AdditionalTags:    nil,
	}

	fakeRecord1 = RecordSpec{
		Record:        infrav1.AddressRecord{Hostname: "my-host", IP: "10.0.0.8"},
		ZoneName:      zoneName,
		ResourceGroup: resourceGroup,
	}

	fakeAzurePrivateZoneManaged = privatedns.PrivateZone{Tags: map[string]*string{
		"sigs.k8s.io_cluster-api-provider-azure_cluster_" + clusterName: to.StringPtr("owned"),
	}}

	fakeAzurePrivateZoneUnmanaged = privatedns.PrivateZone{}

	fakeAzureVnetLinkManaged = privatedns.VirtualNetworkLink{Tags: map[string]*string{
		"sigs.k8s.io_cluster-api-provider-azure_cluster_" + clusterName: to.StringPtr("owned"),
	}}

	fakeAzureVnetLinkUnmanaged = privatedns.VirtualNetworkLink{}

	notDoneError  = azure.NewOperationNotDoneError(&infrav1.Future{Type: "resourceType", ResourceGroup: resourceGroup, Name: "resourceName"})
	errFake       = errors.New("this is an error")
	notFoundError = autorest.DetailedError{StatusCode: 404}
)

func TestReconcilePrivateDNS(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_privatedns.MockScopeMockRecorder, zoneReconiler, linksReconciler, recordsReconciler *mock_async.MockReconcilerMockRecorder,
			zoneGetter, linkGetter *mock_async.MockGetterMockRecorder)
	}{
		{
			name:          "no private dns",
			expectedError: "",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, z, l, r *mock_async.MockReconcilerMockRecorder, zg, lg *mock_async.MockGetterMockRecorder) {
				s.PrivateDNSSpec().Return(nil, nil, nil)
			},
		},
		{
			name:          "create private dns with multiple links successfully",
			expectedError: "",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, z, l, r *mock_async.MockReconcilerMockRecorder, zg, lg *mock_async.MockGetterMockRecorder) {
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)
				zg.Get(gomockinternal.AContext(), fakeZone).Return(nil, notFoundError)
				lg.Get(gomockinternal.AContext(), fakeLink1).Return(nil, notFoundError)
				lg.Get(gomockinternal.AContext(), fakeLink2).Return(nil, notFoundError)
				z.CreateResource(gomockinternal.AContext(), fakeZone, serviceName).Return(nil, nil)
				l.CreateResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(nil, nil)
				l.CreateResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil, nil)
				r.CreateResource(gomockinternal.AContext(), fakeRecord1, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSRecordReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "zone creation in progress",
			expectedError: "operation type resourceType on Azure resource my-rg/resourceName is not done",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, z, l, r *mock_async.MockReconcilerMockRecorder, zg, lg *mock_async.MockGetterMockRecorder) {
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)
				zg.Get(gomockinternal.AContext(), fakeZone).Return(nil, notFoundError)
				z.CreateResource(gomockinternal.AContext(), fakeZone, serviceName).Return(nil, notDoneError)
				s.UpdatePutStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, notDoneError)
			},
		},
		{
			name:          "zone creation fails",
			expectedError: "this is an error",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, z, l, r *mock_async.MockReconcilerMockRecorder, zg, lg *mock_async.MockGetterMockRecorder) {
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)
				zg.Get(gomockinternal.AContext(), fakeZone).Return(nil, notFoundError)
				z.CreateResource(gomockinternal.AContext(), fakeZone, serviceName).Return(nil, errFake)
				s.UpdatePutStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, errFake)
			},
		},
		{
			name: "unmanaged zone does not update ready condition",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, z, l, r *mock_async.MockReconcilerMockRecorder, zg, lg *mock_async.MockGetterMockRecorder) {
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)
				zg.Get(gomockinternal.AContext(), fakeZone).Return(fakeAzurePrivateZoneUnmanaged, nil)
				s.ClusterName()
				lg.Get(gomockinternal.AContext(), fakeLink1).Return(false, notFoundError)
				lg.Get(gomockinternal.AContext(), fakeLink2).Return(false, notFoundError)
				l.CreateResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(nil, nil)
				l.CreateResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil, nil)
				r.CreateResource(gomockinternal.AContext(), fakeRecord1, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSRecordReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "link 1 creation fails but still proceeds to link 2, and returns the error",
			expectedError: "this is an error",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, z, l, r *mock_async.MockReconcilerMockRecorder, zg, lg *mock_async.MockGetterMockRecorder) {
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)
				zg.Get(gomockinternal.AContext(), fakeZone).Return(nil, notFoundError)
				lg.Get(gomockinternal.AContext(), fakeLink1).Return(nil, notFoundError)
				lg.Get(gomockinternal.AContext(), fakeLink2).Return(nil, notFoundError)
				z.CreateResource(gomockinternal.AContext(), fakeZone, serviceName).Return(nil, nil)
				l.CreateResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(nil, errFake)
				l.CreateResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, errFake)
			},
		},
		{
			name:          "link 2 creation fails",
			expectedError: "this is an error",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, z, l, r *mock_async.MockReconcilerMockRecorder, zg, lg *mock_async.MockGetterMockRecorder) {
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)
				zg.Get(gomockinternal.AContext(), fakeZone).Return(nil, notFoundError)
				lg.Get(gomockinternal.AContext(), fakeLink1).Return(nil, notFoundError)
				lg.Get(gomockinternal.AContext(), fakeLink2).Return(nil, notFoundError)
				z.CreateResource(gomockinternal.AContext(), fakeZone, serviceName).Return(nil, nil)
				l.CreateResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(nil, nil)
				l.CreateResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil, errFake)
				s.UpdatePutStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, errFake)
			},
		},
		{
			name:          "link 1 is long running, link 2 fails, it returns the failure of link2",
			expectedError: "this is an error",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, z, l, r *mock_async.MockReconcilerMockRecorder, zg, lg *mock_async.MockGetterMockRecorder) {
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)
				zg.Get(gomockinternal.AContext(), fakeZone).Return(nil, notFoundError)
				lg.Get(gomockinternal.AContext(), fakeLink1).Return(nil, notFoundError)
				lg.Get(gomockinternal.AContext(), fakeLink2).Return(nil, notFoundError)
				z.CreateResource(gomockinternal.AContext(), fakeZone, serviceName).Return(nil, nil)
				l.CreateResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(nil, notDoneError)
				l.CreateResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil, errFake)
				s.UpdatePutStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, errFake)
			},
		},
		{
			name: "unmanaged link does not update ready condition",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, z, l, r *mock_async.MockReconcilerMockRecorder, zg, lg *mock_async.MockGetterMockRecorder) {
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)
				zg.Get(gomockinternal.AContext(), fakeZone).Return(nil, notFoundError)
				lg.Get(gomockinternal.AContext(), fakeLink1).Return(fakeAzureVnetLinkUnmanaged, nil)
				s.ClusterName()
				lg.Get(gomockinternal.AContext(), fakeLink2).Return(fakeAzureVnetLinkUnmanaged, nil)
				s.ClusterName()
				z.CreateResource(gomockinternal.AContext(), fakeZone, serviceName).Return(nil, nil)
				r.CreateResource(gomockinternal.AContext(), fakeRecord1, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSRecordReadyCondition, serviceName, nil)
			},
		},
		{
			name: "vnet link is considered managed if at least one of the links is managed",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, z, l, r *mock_async.MockReconcilerMockRecorder, zg, lg *mock_async.MockGetterMockRecorder) {
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)
				zg.Get(gomockinternal.AContext(), fakeZone).Return(nil, notFoundError)
				lg.Get(gomockinternal.AContext(), fakeLink1).Return(fakeAzureVnetLinkUnmanaged, nil)
				s.ClusterName()
				lg.Get(gomockinternal.AContext(), fakeLink2).Return(nil, notFoundError)
				z.CreateResource(gomockinternal.AContext(), fakeZone, serviceName).Return(nil, nil)
				l.CreateResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil, nil)
				r.CreateResource(gomockinternal.AContext(), fakeRecord1, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSRecordReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "record creation fails",
			expectedError: "this is an error",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, z, l, r *mock_async.MockReconcilerMockRecorder, zg, lg *mock_async.MockGetterMockRecorder) {
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)
				zg.Get(gomockinternal.AContext(), fakeZone).Return(nil, notFoundError)
				lg.Get(gomockinternal.AContext(), fakeLink1).Return(nil, notFoundError)
				lg.Get(gomockinternal.AContext(), fakeLink2).Return(nil, notFoundError)
				z.CreateResource(gomockinternal.AContext(), fakeZone, serviceName).Return(nil, nil)
				l.CreateResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(nil, nil)
				l.CreateResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil, nil)
				r.CreateResource(gomockinternal.AContext(), fakeRecord1, serviceName).Return(nil, errFake)
				s.UpdatePutStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSRecordReadyCondition, serviceName, errFake)
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
			scopeMock := mock_privatedns.NewMockScope(mockCtrl)
			zoneReconcilerMock := mock_async.NewMockReconciler(mockCtrl)
			vnetLinkReconcilerMock := mock_async.NewMockReconciler(mockCtrl)
			recordReconcilerMock := mock_async.NewMockReconciler(mockCtrl)
			zoneGetterMock := mock_async.NewMockGetter(mockCtrl)
			vnetLinkGetterMock := mock_async.NewMockGetter(mockCtrl)

			tc.expect(scopeMock.EXPECT(), zoneReconcilerMock.EXPECT(), vnetLinkReconcilerMock.EXPECT(), recordReconcilerMock.EXPECT(),
				zoneGetterMock.EXPECT(), vnetLinkGetterMock.EXPECT())

			s := &Service{
				Scope:              scopeMock,
				zoneReconciler:     zoneReconcilerMock,
				vnetLinkReconciler: vnetLinkReconcilerMock,
				recordReconciler:   recordReconcilerMock,
				zoneGetter:         zoneGetterMock,
				vnetLinkGetter:     vnetLinkGetterMock,
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

func TestDeletePrivateDNS(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_privatedns.MockScopeMockRecorder, linkReconciler, zoneReconciler *mock_async.MockReconcilerMockRecorder,
			linkGetter, zoneGetter *mock_async.MockGetterMockRecorder)
	}{
		{
			name:          "no private dns",
			expectedError: "",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, lr, zr *mock_async.MockReconcilerMockRecorder, lg, zg *mock_async.MockGetterMockRecorder) {
				s.PrivateDNSSpec().Return(nil, nil, nil)
			},
		},
		{
			name:          "dns and links deletion succeeds",
			expectedError: "",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, lr, zr *mock_async.MockReconcilerMockRecorder, lg, zg *mock_async.MockGetterMockRecorder) {
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)

				lg.Get(gomockinternal.AContext(), fakeLink1).Return(fakeAzureVnetLinkManaged, nil)
				s.ClusterName().Return(clusterName)
				lr.DeleteResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(nil)
				lg.Get(gomockinternal.AContext(), fakeLink2).Return(fakeAzureVnetLinkManaged, nil)
				s.ClusterName().Return(clusterName)
				lr.DeleteResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil)

				zg.Get(gomockinternal.AContext(), fakeZone).Return(fakeAzurePrivateZoneManaged, nil)
				s.ClusterName().Return(clusterName)
				zr.DeleteResource(gomockinternal.AContext(), fakeZone, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, nil)
				s.UpdateDeleteStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, nil)
				s.UpdateDeleteStatus(infrav1.PrivateDNSRecordReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "skips if zone and links are unmanaged",
			expectedError: "",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, lr, zr *mock_async.MockReconcilerMockRecorder, lg, zg *mock_async.MockGetterMockRecorder) {
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)

				lg.Get(gomockinternal.AContext(), fakeLink1).Return(fakeAzureVnetLinkUnmanaged, nil)
				s.ClusterName().Return(clusterName)
				lg.Get(gomockinternal.AContext(), fakeLink2).Return(fakeAzureVnetLinkUnmanaged, nil)
				s.ClusterName().Return(clusterName)

				zg.Get(gomockinternal.AContext(), fakeZone).Return(fakeAzurePrivateZoneUnmanaged, nil)
				s.ClusterName().Return(clusterName)
			},
		},
		{
			name:          "skips if unmanaged, but deletes the next resource if it is managed",
			expectedError: "",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, lr, zr *mock_async.MockReconcilerMockRecorder, lg, zg *mock_async.MockGetterMockRecorder) {
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)

				lg.Get(gomockinternal.AContext(), fakeLink1).Return(fakeAzureVnetLinkUnmanaged, nil)
				s.ClusterName().Return(clusterName)
				lg.Get(gomockinternal.AContext(), fakeLink2).Return(fakeAzureVnetLinkManaged, nil)
				s.ClusterName().Return(clusterName)
				lr.DeleteResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil)

				zg.Get(gomockinternal.AContext(), fakeZone).Return(fakeAzurePrivateZoneManaged, nil)
				s.ClusterName().Return(clusterName)
				zr.DeleteResource(gomockinternal.AContext(), fakeZone, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, nil)
				s.UpdateDeleteStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, nil)
				s.UpdateDeleteStatus(infrav1.PrivateDNSRecordReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "link1 is deleted, link2 is long running. It returns not done error",
			expectedError: "operation type resourceType on Azure resource my-rg/resourceName is not done",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, lr, zr *mock_async.MockReconcilerMockRecorder, lg, zg *mock_async.MockGetterMockRecorder) {
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1})

				lg.Get(gomockinternal.AContext(), fakeLink1).Return(fakeAzureVnetLinkManaged, nil)
				s.ClusterName().Return(clusterName)
				lr.DeleteResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(notDoneError)
				lg.Get(gomockinternal.AContext(), fakeLink2).Return(fakeAzureVnetLinkManaged, nil)
				s.ClusterName().Return(clusterName)
				lr.DeleteResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, notDoneError)
			},
		},
		{
			name:          "link1 deletion fails and link2 is long running, returns the more pressing error",
			expectedError: "this is an error",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, lr, zr *mock_async.MockReconcilerMockRecorder, lg, zg *mock_async.MockGetterMockRecorder) {
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1})

				lg.Get(gomockinternal.AContext(), fakeLink1).Return(fakeAzureVnetLinkManaged, nil)
				s.ClusterName().Return(clusterName)
				lr.DeleteResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(errFake)
				lg.Get(gomockinternal.AContext(), fakeLink2).Return(fakeAzureVnetLinkManaged, nil)
				s.ClusterName().Return(clusterName)
				lr.DeleteResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(notDoneError)
				s.UpdateDeleteStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, errFake)
			},
		},
		{
			name:          "links are deleted, zone is long running",
			expectedError: "operation type resourceType on Azure resource my-rg/resourceName is not done",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, lr, zr *mock_async.MockReconcilerMockRecorder, lg, zg *mock_async.MockGetterMockRecorder) {
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)

				lg.Get(gomockinternal.AContext(), fakeLink1).Return(fakeAzureVnetLinkManaged, nil)
				s.ClusterName().Return(clusterName)
				lr.DeleteResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(nil)
				lg.Get(gomockinternal.AContext(), fakeLink2).Return(fakeAzureVnetLinkManaged, nil)
				s.ClusterName().Return(clusterName)
				lr.DeleteResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil)

				zg.Get(gomockinternal.AContext(), fakeZone).Return(fakeAzurePrivateZoneManaged, nil)
				s.ClusterName().Return(clusterName)
				zr.DeleteResource(gomockinternal.AContext(), fakeZone, serviceName).Return(notDoneError)

				s.UpdateDeleteStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, nil)
				s.UpdateDeleteStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, notDoneError)
				s.UpdateDeleteStatus(infrav1.PrivateDNSRecordReadyCondition, serviceName, notDoneError)
			},
		},
		{
			name:          "links are deleted, zone deletion fails with error",
			expectedError: "this is an error",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, lr, zr *mock_async.MockReconcilerMockRecorder, lg, zg *mock_async.MockGetterMockRecorder) {
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)

				lg.Get(gomockinternal.AContext(), fakeLink1).Return(fakeAzureVnetLinkManaged, nil)
				s.ClusterName().Return(clusterName)
				lr.DeleteResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(nil)
				lg.Get(gomockinternal.AContext(), fakeLink2).Return(fakeAzureVnetLinkManaged, nil)
				s.ClusterName().Return(clusterName)
				lr.DeleteResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil)

				zg.Get(gomockinternal.AContext(), fakeZone).Return(fakeAzurePrivateZoneManaged, nil)
				s.ClusterName().Return(clusterName)
				zr.DeleteResource(gomockinternal.AContext(), fakeZone, serviceName).Return(errFake)

				s.UpdateDeleteStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, nil)
				s.UpdateDeleteStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, errFake)
				s.UpdateDeleteStatus(infrav1.PrivateDNSRecordReadyCondition, serviceName, errFake)
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
			scopeMock := mock_privatedns.NewMockScope(mockCtrl)
			vnetLinkReconcilerMock := mock_async.NewMockReconciler(mockCtrl)
			zoneReconcilerMock := mock_async.NewMockReconciler(mockCtrl)
			vnetLinkGetterMock := mock_async.NewMockGetter(mockCtrl)
			zoneGetterMock := mock_async.NewMockGetter(mockCtrl)

			tc.expect(scopeMock.EXPECT(), vnetLinkReconcilerMock.EXPECT(), zoneReconcilerMock.EXPECT(),
				vnetLinkGetterMock.EXPECT(), zoneGetterMock.EXPECT())

			s := &Service{
				Scope:              scopeMock,
				zoneReconciler:     zoneReconcilerMock,
				vnetLinkReconciler: vnetLinkReconcilerMock,
				zoneGetter:         zoneGetterMock,
				vnetLinkGetter:     vnetLinkGetterMock,
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

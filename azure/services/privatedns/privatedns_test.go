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
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/privatedns/mock_privatedns"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
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

	managedTags = armresources.TagsResource{
		Properties: &armresources.Tags{
			Tags: map[string]*string{
				"foo": ptr.To("bar"),
				"sigs.k8s.io_cluster-api-provider-azure_cluster_" + clusterName: ptr.To("owned"),
			},
		},
	}

	notDoneError  = azure.NewOperationNotDoneError(&infrav1.Future{Type: "resourceType", ResourceGroup: resourceGroup, Name: "resourceName"})
	errFake       = errors.New("this is an error")
	notFoundError = &azcore.ResponseError{StatusCode: http.StatusNotFound}
)

func TestReconcilePrivateDNS(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_privatedns.MockScopeMockRecorder, zoneReconiler, linksReconciler, recordsReconciler *mock_async.MockReconcilerMockRecorder,
			tagsGetter *mock_async.MockTagsGetterMockRecorder)
	}{
		{
			name:          "no private dns",
			expectedError: "",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, _, _, _ *mock_async.MockReconcilerMockRecorder, _ *mock_async.MockTagsGetterMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PrivateDNSSpec().Return(nil, nil, nil)
			},
		},
		{
			name:          "create private dns with multiple links successfully",
			expectedError: "",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, z, l, r *mock_async.MockReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.PrivateDNSZoneID("123", fakeZone.ResourceGroupName(), fakeZone.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink1.ResourceGroupName(), fakeLink1.OwnerResourceName(), fakeLink1.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink2.ResourceGroupName(), fakeLink2.OwnerResourceName(), fakeLink2.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				z.CreateOrUpdateResource(gomockinternal.AContext(), fakeZone, serviceName).Return(nil, nil)
				l.CreateOrUpdateResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(nil, nil)
				l.CreateOrUpdateResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), fakeRecord1, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSRecordReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "zone creation in progress",
			expectedError: "operation type resourceType on Azure resource my-rg/resourceName is not done",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, z, _, _ *mock_async.MockReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.PrivateDNSZoneID("123", fakeZone.ResourceGroupName(), fakeZone.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				z.CreateOrUpdateResource(gomockinternal.AContext(), fakeZone, serviceName).Return(nil, notDoneError)
				s.UpdatePutStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, notDoneError)
			},
		},
		{
			name:          "zone creation fails",
			expectedError: "this is an error",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, z, _, _ *mock_async.MockReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.PrivateDNSZoneID("123", fakeZone.ResourceGroupName(), fakeZone.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				z.CreateOrUpdateResource(gomockinternal.AContext(), fakeZone, serviceName).Return(nil, errFake)
				s.UpdatePutStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, errFake)
			},
		},
		{
			name: "unmanaged zone does not update ready condition",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, _, l, r *mock_async.MockReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.PrivateDNSZoneID("123", fakeZone.ResourceGroupName(), fakeZone.ResourceName())).Return(armresources.TagsResource{}, nil)
				s.ClusterName().Return(clusterName)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink1.ResourceGroupName(), fakeLink1.OwnerResourceName(), fakeLink1.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink2.ResourceGroupName(), fakeLink2.OwnerResourceName(), fakeLink2.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				l.CreateOrUpdateResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(nil, nil)
				l.CreateOrUpdateResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), fakeRecord1, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSRecordReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "link 1 creation fails but still proceeds to link 2, and returns the error",
			expectedError: "this is an error",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, z, l, _ *mock_async.MockReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.PrivateDNSZoneID("123", fakeZone.ResourceGroupName(), fakeZone.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink1.ResourceGroupName(), fakeLink1.OwnerResourceName(), fakeLink1.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink2.ResourceGroupName(), fakeLink2.OwnerResourceName(), fakeLink2.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				z.CreateOrUpdateResource(gomockinternal.AContext(), fakeZone, serviceName).Return(nil, nil)
				l.CreateOrUpdateResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(nil, errFake)
				l.CreateOrUpdateResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, errFake)
			},
		},
		{
			name:          "link 2 creation fails",
			expectedError: "this is an error",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, z, l, _ *mock_async.MockReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.PrivateDNSZoneID("123", fakeZone.ResourceGroupName(), fakeZone.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink1.ResourceGroupName(), fakeLink1.OwnerResourceName(), fakeLink1.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink2.ResourceGroupName(), fakeLink2.OwnerResourceName(), fakeLink2.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				z.CreateOrUpdateResource(gomockinternal.AContext(), fakeZone, serviceName).Return(nil, nil)
				l.CreateOrUpdateResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(nil, nil)
				l.CreateOrUpdateResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil, errFake)
				s.UpdatePutStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, errFake)
			},
		},
		{
			name:          "link 1 is long running, link 2 fails, it returns the failure of link2",
			expectedError: "this is an error",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, z, l, _ *mock_async.MockReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.PrivateDNSZoneID("123", fakeZone.ResourceGroupName(), fakeZone.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink1.ResourceGroupName(), fakeLink1.OwnerResourceName(), fakeLink1.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink2.ResourceGroupName(), fakeLink2.OwnerResourceName(), fakeLink2.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				z.CreateOrUpdateResource(gomockinternal.AContext(), fakeZone, serviceName).Return(nil, nil)
				l.CreateOrUpdateResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(nil, notDoneError)
				l.CreateOrUpdateResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil, errFake)
				s.UpdatePutStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, errFake)
			},
		},
		{
			name: "unmanaged link does not update ready condition",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, z, _, r *mock_async.MockReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.PrivateDNSZoneID("123", fakeZone.ResourceGroupName(), fakeZone.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink1.ResourceGroupName(), fakeLink1.OwnerResourceName(), fakeLink1.ResourceName())).Return(armresources.TagsResource{}, nil)
				s.ClusterName().Return(clusterName)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink2.ResourceGroupName(), fakeLink2.OwnerResourceName(), fakeLink2.ResourceName())).Return(armresources.TagsResource{}, nil)
				s.ClusterName().Return(clusterName)

				z.CreateOrUpdateResource(gomockinternal.AContext(), fakeZone, serviceName).Return(nil, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), fakeRecord1, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSRecordReadyCondition, serviceName, nil)
			},
		},
		{
			name: "vnet link is considered managed if at least one of the links is managed",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, z, l, r *mock_async.MockReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.PrivateDNSZoneID("123", fakeZone.ResourceGroupName(), fakeZone.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink1.ResourceGroupName(), fakeLink1.OwnerResourceName(), fakeLink1.ResourceName())).Return(armresources.TagsResource{}, nil)
				s.ClusterName().Return(clusterName)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink2.ResourceGroupName(), fakeLink2.OwnerResourceName(), fakeLink2.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				z.CreateOrUpdateResource(gomockinternal.AContext(), fakeZone, serviceName).Return(nil, nil)
				l.CreateOrUpdateResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), fakeRecord1, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSRecordReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "record creation fails",
			expectedError: "this is an error",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, z, l, r *mock_async.MockReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.PrivateDNSZoneID("123", fakeZone.ResourceGroupName(), fakeZone.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink1.ResourceGroupName(), fakeLink1.OwnerResourceName(), fakeLink1.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink2.ResourceGroupName(), fakeLink2.OwnerResourceName(), fakeLink2.ResourceName())).Return(armresources.TagsResource{}, notFoundError)

				z.CreateOrUpdateResource(gomockinternal.AContext(), fakeZone, serviceName).Return(nil, nil)
				l.CreateOrUpdateResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(nil, nil)
				l.CreateOrUpdateResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), fakeRecord1, serviceName).Return(nil, errFake)
				s.UpdatePutStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, nil)
				s.UpdatePutStatus(infrav1.PrivateDNSRecordReadyCondition, serviceName, errFake)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_privatedns.NewMockScope(mockCtrl)
			zoneReconcilerMock := mock_async.NewMockReconciler(mockCtrl)
			vnetLinkReconcilerMock := mock_async.NewMockReconciler(mockCtrl)
			recordReconcilerMock := mock_async.NewMockReconciler(mockCtrl)
			tagsGetterMock := mock_async.NewMockTagsGetter(mockCtrl)

			tc.expect(scopeMock.EXPECT(), zoneReconcilerMock.EXPECT(), vnetLinkReconcilerMock.EXPECT(), recordReconcilerMock.EXPECT(), tagsGetterMock.EXPECT())

			s := &Service{
				Scope:              scopeMock,
				zoneReconciler:     zoneReconcilerMock,
				vnetLinkReconciler: vnetLinkReconcilerMock,
				recordReconciler:   recordReconcilerMock,
				TagsGetter:         tagsGetterMock,
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
		expect        func(s *mock_privatedns.MockScopeMockRecorder, linkReconciler, zoneReconciler *mock_async.MockReconcilerMockRecorder, tagsGetter *mock_async.MockTagsGetterMockRecorder)
	}{
		{
			name:          "no private dns",
			expectedError: "",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, _, _ *mock_async.MockReconcilerMockRecorder, _ *mock_async.MockTagsGetterMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PrivateDNSSpec().Return(nil, nil, nil)
			},
		},
		{
			name:          "dns and links deletion succeeds",
			expectedError: "",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, lr, zr *mock_async.MockReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink1.ResourceGroupName(), fakeLink1.OwnerResourceName(), fakeLink1.ResourceName())).Return(managedTags, nil)
				s.ClusterName().Return(clusterName)

				lr.DeleteResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(nil)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink2.ResourceGroupName(), fakeLink2.OwnerResourceName(), fakeLink2.ResourceName())).Return(managedTags, nil)
				s.ClusterName().Return(clusterName)

				lr.DeleteResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.PrivateDNSZoneID("123", fakeZone.ResourceGroupName(), fakeZone.ResourceName())).Return(managedTags, nil)
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
			expect: func(s *mock_privatedns.MockScopeMockRecorder, _, _ *mock_async.MockReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink1.ResourceGroupName(), fakeLink1.OwnerResourceName(), fakeLink1.ResourceName())).Return(armresources.TagsResource{}, nil)
				s.ClusterName().Return(clusterName)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink2.ResourceGroupName(), fakeLink2.OwnerResourceName(), fakeLink2.ResourceName())).Return(armresources.TagsResource{}, nil)
				s.ClusterName().Return(clusterName)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.PrivateDNSZoneID("123", fakeZone.ResourceGroupName(), fakeZone.ResourceName())).Return(armresources.TagsResource{}, nil)
				s.ClusterName().Return(clusterName)
			},
		},
		{
			name:          "skips if unmanaged, but deletes the next resource if it is managed",
			expectedError: "",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, lr, zr *mock_async.MockReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink1.ResourceGroupName(), fakeLink1.OwnerResourceName(), fakeLink1.ResourceName())).Return(armresources.TagsResource{}, nil)
				s.ClusterName().Return(clusterName)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink2.ResourceGroupName(), fakeLink2.OwnerResourceName(), fakeLink2.ResourceName())).Return(managedTags, nil)
				s.ClusterName().Return(clusterName)
				lr.DeleteResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.PrivateDNSZoneID("123", fakeZone.ResourceGroupName(), fakeZone.ResourceName())).Return(managedTags, nil)
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
			expect: func(s *mock_privatedns.MockScopeMockRecorder, lr, _ *mock_async.MockReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1})

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink1.ResourceGroupName(), fakeLink1.OwnerResourceName(), fakeLink1.ResourceName())).Return(managedTags, nil)
				s.ClusterName().Return(clusterName)

				lr.DeleteResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(notDoneError)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink2.ResourceGroupName(), fakeLink2.OwnerResourceName(), fakeLink2.ResourceName())).Return(managedTags, nil)
				s.ClusterName().Return(clusterName)

				lr.DeleteResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, notDoneError)
			},
		},
		{
			name:          "link1 deletion fails and link2 is long running, returns the more pressing error",
			expectedError: "this is an error",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, lr, _ *mock_async.MockReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1})

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink1.ResourceGroupName(), fakeLink1.OwnerResourceName(), fakeLink1.ResourceName())).Return(managedTags, nil)
				s.ClusterName().Return(clusterName)

				lr.DeleteResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(errFake)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink2.ResourceGroupName(), fakeLink2.OwnerResourceName(), fakeLink2.ResourceName())).Return(managedTags, nil)
				s.ClusterName().Return(clusterName)

				lr.DeleteResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(notDoneError)
				s.UpdateDeleteStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, errFake)
			},
		},
		{
			name:          "links are deleted, zone is long running",
			expectedError: "operation type resourceType on Azure resource my-rg/resourceName is not done",
			expect: func(s *mock_privatedns.MockScopeMockRecorder, lr, zr *mock_async.MockReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink1.ResourceGroupName(), fakeLink1.OwnerResourceName(), fakeLink1.ResourceName())).Return(managedTags, nil)
				s.ClusterName().Return(clusterName)

				lr.DeleteResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(nil)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink2.ResourceGroupName(), fakeLink2.OwnerResourceName(), fakeLink2.ResourceName())).Return(managedTags, nil)
				s.ClusterName().Return(clusterName)

				lr.DeleteResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.PrivateDNSZoneID("123", fakeZone.ResourceGroupName(), fakeZone.ResourceName())).Return(managedTags, nil)
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
			expect: func(s *mock_privatedns.MockScopeMockRecorder, lr, zr *mock_async.MockReconcilerMockRecorder, tg *mock_async.MockTagsGetterMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.PrivateDNSSpec().Return(fakeZone, []azure.ResourceSpecGetter{fakeLink1, fakeLink2}, []azure.ResourceSpecGetter{fakeRecord1}).Times(2)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink1.ResourceGroupName(), fakeLink1.OwnerResourceName(), fakeLink1.ResourceName())).Return(managedTags, nil)
				s.ClusterName().Return(clusterName)

				lr.DeleteResource(gomockinternal.AContext(), fakeLink1, serviceName).Return(nil)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.VirtualNetworkLinkID("123", fakeLink2.ResourceGroupName(), fakeLink2.OwnerResourceName(), fakeLink2.ResourceName())).Return(managedTags, nil)
				s.ClusterName().Return(clusterName)

				lr.DeleteResource(gomockinternal.AContext(), fakeLink2, serviceName).Return(nil)

				s.SubscriptionID().Return("123")
				tg.GetAtScope(gomockinternal.AContext(), azure.PrivateDNSZoneID("123", fakeZone.ResourceGroupName(), fakeZone.ResourceName())).Return(managedTags, nil)
				s.ClusterName().Return(clusterName)

				zr.DeleteResource(gomockinternal.AContext(), fakeZone, serviceName).Return(errFake)

				s.UpdateDeleteStatus(infrav1.PrivateDNSLinkReadyCondition, serviceName, nil)
				s.UpdateDeleteStatus(infrav1.PrivateDNSZoneReadyCondition, serviceName, errFake)
				s.UpdateDeleteStatus(infrav1.PrivateDNSRecordReadyCondition, serviceName, errFake)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_privatedns.NewMockScope(mockCtrl)
			vnetLinkReconcilerMock := mock_async.NewMockReconciler(mockCtrl)
			zoneReconcilerMock := mock_async.NewMockReconciler(mockCtrl)
			tagsGetterMock := mock_async.NewMockTagsGetter(mockCtrl)

			tc.expect(scopeMock.EXPECT(), vnetLinkReconcilerMock.EXPECT(), zoneReconcilerMock.EXPECT(), tagsGetterMock.EXPECT())

			s := &Service{
				Scope:              scopeMock,
				zoneReconciler:     zoneReconcilerMock,
				vnetLinkReconciler: vnetLinkReconcilerMock,
				TagsGetter:         tagsGetterMock,
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

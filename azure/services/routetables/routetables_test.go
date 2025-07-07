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

package routetables

import (
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/routetables/mock_routetables"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
)

var (
	fakeRT = RouteTableSpec{
		Name:          "test-rt-1",
		ResourceGroup: "test-rg",
		Location:      "fake-location",
		ClusterName:   "test-cluster",
		AdditionalTags: map[string]string{
			"foo": "bar",
		},
	}
	fakeRT2 = RouteTableSpec{
		Name:          "test-rt-2",
		ResourceGroup: "test-rg",
		Location:      "fake-location",
		ClusterName:   "test-cluster",
	}
	errFake      = errors.New("this is an error")
	notDoneError = azure.NewOperationNotDoneError(&infrav1.Future{})
)

func TestReconcileRouteTables(t *testing.T) {
	testcases := []struct {
		name          string
		tags          infrav1.Tags
		expectedError string
		expect        func(s *mock_routetables.MockRouteTableScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no route table specs are found",
			expectedError: "",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(true)
				s.RouteTableSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "create multiple route tables succeeds",
			expectedError: "",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(true)
				s.RouteTableSpecs().Return([]azure.ResourceSpecGetter{&fakeRT, &fakeRT2})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeRT, serviceName).Return(nil, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeRT2, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.RouteTablesReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "first route table create fails",
			expectedError: errFake.Error(),
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(true)
				s.RouteTableSpecs().Return([]azure.ResourceSpecGetter{&fakeRT, &fakeRT2})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeRT, serviceName).Return(nil, errFake)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeRT2, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.RouteTablesReadyCondition, serviceName, errFake)
			},
		},
		{
			name:          "second route table create not done",
			expectedError: errFake.Error(),
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(true)
				s.RouteTableSpecs().Return([]azure.ResourceSpecGetter{&fakeRT, &fakeRT2})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeRT, serviceName).Return(nil, errFake)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeRT2, serviceName).Return(nil, notDoneError)
				s.UpdatePutStatus(infrav1.RouteTablesReadyCondition, serviceName, errFake)
			},
		},
		{
			name:          "noop if vnet is not managed",
			expectedError: "",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(false)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_routetables.NewMockRouteTableScope(mockCtrl)
			reconcilerMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), reconcilerMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: reconcilerMock,
			}

			err := s.Reconcile(t.Context())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteRouteTable(t *testing.T) {
	testcases := []struct {
		name          string
		tags          infrav1.Tags
		expectedError string
		expect        func(s *mock_routetables.MockRouteTableScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no route table specs are found",
			expectedError: "",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(true)
				s.RouteTableSpecs().Return([]azure.ResourceSpecGetter{})
			},
		},
		{
			name:          "delete multiple route tables succeeds",
			expectedError: "",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(true)
				s.RouteTableSpecs().Return([]azure.ResourceSpecGetter{&fakeRT, &fakeRT2})
				r.DeleteResource(gomockinternal.AContext(), &fakeRT, serviceName).Return(nil)
				r.DeleteResource(gomockinternal.AContext(), &fakeRT2, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.RouteTablesReadyCondition, serviceName, nil)
			},
		},
		{
			name:          "first route table delete fails",
			expectedError: errFake.Error(),
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(true)
				s.RouteTableSpecs().Return([]azure.ResourceSpecGetter{&fakeRT, &fakeRT2})
				r.DeleteResource(gomockinternal.AContext(), &fakeRT, serviceName).Return(errFake)
				r.DeleteResource(gomockinternal.AContext(), &fakeRT2, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.RouteTablesReadyCondition, serviceName, errFake)
			},
		},
		{
			name:          "second route table delete not done",
			expectedError: errFake.Error(),
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(true)
				s.RouteTableSpecs().Return([]azure.ResourceSpecGetter{&fakeRT, &fakeRT2})
				r.DeleteResource(gomockinternal.AContext(), &fakeRT, serviceName).Return(errFake)
				r.DeleteResource(gomockinternal.AContext(), &fakeRT2, serviceName).Return(notDoneError)
				s.UpdateDeleteStatus(infrav1.RouteTablesReadyCondition, serviceName, errFake)
			},
		},
		{
			name:          "noop if vnet is not managed",
			expectedError: "",
			expect: func(s *mock_routetables.MockRouteTableScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.DefaultedAzureServiceReconcileTimeout().Return(reconciler.DefaultAzureServiceReconcileTimeout)
				s.IsVnetManaged().Return(false)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_routetables.NewMockRouteTableScope(mockCtrl)
			reconcilerMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), reconcilerMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: reconcilerMock,
			}

			err := s.Delete(t.Context())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

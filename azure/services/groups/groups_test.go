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

package groups

import (
	"context"
	"errors"
	"testing"

	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	asoannotations "github.com/Azure/azure-service-operator/v2/pkg/common/annotations"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/aso/mock_aso"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/groups/mock_groups"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	fakeGroupSpec = GroupSpec{
		Name:           "test-group",
		Namespace:      "test-group-ns",
		Location:       "test-location",
		ClusterName:    "test-cluster",
		AdditionalTags: map[string]string{"foo": "bar"},
	}
	errInternal        = errors.New("internal error")
	sampleManagedGroup = &asoresourcesv1.ResourceGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-group",
			Namespace: "test-group-ns",
		},
		Spec: asoresourcesv1.ResourceGroup_Spec{
			Location: ptr.To("test-location"),
		},
	}
)

func TestReconcileGroups(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_groups.MockGroupScopeMockRecorder, r *mock_aso.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no group spec is found",
			expectedError: "",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, _ *mock_aso.MockReconcilerMockRecorder) {
				s.GroupSpec().Return(nil)
			},
		},
		{
			name:          "create group succeeds",
			expectedError: "",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, r *mock_aso.MockReconcilerMockRecorder) {
				s.GroupSpec().Return(&fakeGroupSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeGroupSpec, ServiceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.ResourceGroupReadyCondition, ServiceName, nil)
			},
		},
		{
			name:          "create resource group fails",
			expectedError: "internal error",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, r *mock_aso.MockReconcilerMockRecorder) {
				s.GroupSpec().Return(&fakeGroupSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &fakeGroupSpec, ServiceName).Return(nil, errInternal)
				s.UpdatePutStatus(infrav1.ResourceGroupReadyCondition, ServiceName, errInternal)
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_groups.NewMockGroupScope(mockCtrl)
			asoMock := mock_aso.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), asoMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: asoMock,
			}

			err := s.Reconcile(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(ContainSubstring(tc.expectedError)))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

type ErroringDeleteClient struct {
	client.Client
	err error
}

func (e *ErroringDeleteClient) Delete(_ context.Context, _ client.Object, _ ...client.DeleteOption) error {
	return e.err
}

func TestDeleteGroups(t *testing.T) {
	testcases := []struct {
		name          string
		clientBuilder func(g Gomega) client.Client
		expectedError string
		expect        func(s *mock_groups.MockGroupScopeMockRecorder, r *mock_aso.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no group spec is found",
			expectedError: "",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, _ *mock_aso.MockReconcilerMockRecorder) {
				s.GroupSpec().Return(nil)
			},
		},
		{
			name:          "delete operation is successful for managed resource group",
			expectedError: "",
			clientBuilder: func(g Gomega) client.Client {
				scheme := runtime.NewScheme()
				g.Expect(asoresourcesv1.AddToScheme(scheme)).To(Succeed())
				return fakeclient.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(sampleManagedGroup.DeepCopy()).
					Build()
			},
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, r *mock_aso.MockReconcilerMockRecorder) {
				s.GroupSpec().AnyTimes().Return(&fakeGroupSpec)
				r.DeleteResource(gomockinternal.AContext(), &fakeGroupSpec, ServiceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.ResourceGroupReadyCondition, ServiceName, nil)
			},
		},
		{
			name: "error occurs when deleting resource group",
			clientBuilder: func(g Gomega) client.Client {
				scheme := runtime.NewScheme()
				g.Expect(asoresourcesv1.AddToScheme(scheme)).To(Succeed())
				c := fakeclient.NewClientBuilder().
					WithScheme(scheme).
					WithObjects(sampleManagedGroup.DeepCopy()).
					Build()
				return &ErroringDeleteClient{Client: c, err: errInternal}
			},
			expectedError: "internal error",
			expect: func(s *mock_groups.MockGroupScopeMockRecorder, r *mock_aso.MockReconcilerMockRecorder) {
				s.GroupSpec().AnyTimes().Return(&fakeGroupSpec)
				r.DeleteResource(gomockinternal.AContext(), &fakeGroupSpec, ServiceName).Return(errInternal)
				s.UpdateDeleteStatus(infrav1.ResourceGroupReadyCondition, ServiceName, gomockinternal.ErrStrEq("internal error"))
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
			scopeMock := mock_groups.NewMockGroupScope(mockCtrl)
			asyncMock := mock_aso.NewMockReconciler(mockCtrl)

			var ctrlClient client.Client
			if tc.clientBuilder != nil {
				ctrlClient = tc.clientBuilder(g)
			}

			scopeMock.EXPECT().GetClient().Return(ctrlClient).AnyTimes()
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

func TestShouldDeleteIndividualResources(t *testing.T) {
	tests := []struct {
		name     string
		objects  []client.Object
		expect   func(s *mock_groups.MockGroupScopeMockRecorder)
		expected bool
	}{
		{
			name:    "error checking if group is managed",
			objects: nil,
			expect: func(s *mock_groups.MockGroupScopeMockRecorder) {
				s.GroupSpec().Return(&GroupSpec{}).AnyTimes()
				s.ClusterName().Return("").AnyTimes()
			},
			expected: true,
		},
		{
			name: "group is unmanaged",
			objects: []client.Object{
				&asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name",
						Namespace: "namespace",
						Labels: map[string]string{
							infrav1.OwnedByClusterLabelKey: "not-cluster",
						},
					},
				},
			},
			expect: func(s *mock_groups.MockGroupScopeMockRecorder) {
				s.GroupSpec().Return(&GroupSpec{
					Name:      "name",
					Namespace: "namespace",
				}).AnyTimes()
				s.ClusterName().Return("cluster").AnyTimes()
			},
			expected: true,
		},
		{
			name: "group is managed and has reconcile policy skip",
			objects: []client.Object{
				&asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name",
						Namespace: "namespace",
						Labels: map[string]string{
							infrav1.OwnedByClusterLabelKey: "cluster",
						},
						Annotations: map[string]string{
							asoannotations.ReconcilePolicy: string(asoannotations.ReconcilePolicySkip),
						},
					},
				},
			},
			expect: func(s *mock_groups.MockGroupScopeMockRecorder) {
				s.GroupSpec().Return(&GroupSpec{
					Name:      "name",
					Namespace: "namespace",
				}).AnyTimes()
				s.ClusterName().Return("cluster").AnyTimes()
			},
			expected: true,
		},
		{
			name: "group is managed and has reconcile policy manage",
			objects: []client.Object{
				&asoresourcesv1.ResourceGroup{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "name",
						Namespace: "namespace",
						Labels: map[string]string{
							infrav1.OwnedByClusterLabelKey: "cluster",
						},
						Annotations: map[string]string{
							asoannotations.ReconcilePolicy: string(asoannotations.ReconcilePolicyManage),
						},
					},
				},
			},
			expect: func(s *mock_groups.MockGroupScopeMockRecorder) {
				s.GroupSpec().Return(&GroupSpec{
					Name:      "name",
					Namespace: "namespace",
				}).AnyTimes()
				s.ClusterName().Return("cluster").AnyTimes()
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_groups.NewMockGroupScope(mockCtrl)

			scheme := runtime.NewScheme()
			g.Expect(asoresourcesv1.AddToScheme(scheme))
			ctrlClient := fakeclient.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(test.objects...).
				Build()
			scopeMock.EXPECT().GetClient().Return(ctrlClient).AnyTimes()
			test.expect(scopeMock.EXPECT())

			actual := New(scopeMock).ShouldDeleteIndividualResources(context.Background())
			g.Expect(actual).To(Equal(test.expected))
		})
	}
}

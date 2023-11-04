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
	"testing"

	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	asoannotations "github.com/Azure/azure-service-operator/v2/pkg/common/annotations"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/groups/mock_groups"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestIsManaged(t *testing.T) {
	tests := []struct {
		name          string
		objects       []client.Object
		expect        func(s *mock_groups.MockGroupScopeMockRecorder)
		expected      bool
		expectedError bool
	}{
		{
			name:    "error checking if group is managed",
			objects: nil,
			expect: func(s *mock_groups.MockGroupScopeMockRecorder) {
				s.GroupSpecs().Return([]azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{&GroupSpec{}}).AnyTimes()
				s.ClusterName().Return("").AnyTimes()
			},
			expectedError: true,
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
				s.GroupSpecs().Return([]azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
					&GroupSpec{
						Name:      "name",
						Namespace: "namespace",
					},
				}).AnyTimes()
				s.ClusterName().Return("cluster").AnyTimes()
			},
			expected: false,
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
				s.GroupSpecs().Return([]azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
					&GroupSpec{
						Name:      "name",
						Namespace: "namespace",
					},
				}).AnyTimes()
				s.ClusterName().Return("cluster").AnyTimes()
			},
			expected: false,
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
				s.GroupSpecs().Return([]azure.ASOResourceSpecGetter[*asoresourcesv1.ResourceGroup]{
					&GroupSpec{
						Name:      "name",
						Namespace: "namespace",
					},
				}).AnyTimes()
				s.ClusterName().Return("cluster").AnyTimes()
			},
			expected: true,
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

			actual, err := New(scopeMock).IsManaged(context.Background())
			if test.expectedError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(actual).To(Equal(test.expected))
			}
		})
	}
}

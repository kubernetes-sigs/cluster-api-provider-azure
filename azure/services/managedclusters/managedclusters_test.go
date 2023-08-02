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

package managedclusters

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2022-03-01/containerservice"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/managedclusters/mock_managedclusters"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var fakeManagedClusterSpec = &ManagedClusterSpec{Name: "my-managedcluster", ResourceGroup: "my-rg"}

func TestReconcile(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(m *mock_managedclusters.MockCredentialGetterMockRecorder, s *mock_managedclusters.MockManagedClusterScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if managedcluster spec is nil",
			expectedError: "",
			expect: func(m *mock_managedclusters.MockCredentialGetterMockRecorder, s *mock_managedclusters.MockManagedClusterScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.ManagedClusterSpec().Return(nil)
			},
		},
		{
			name:          "create managed cluster returns an error",
			expectedError: "some unexpected error occurred",
			expect: func(m *mock_managedclusters.MockCredentialGetterMockRecorder, s *mock_managedclusters.MockManagedClusterScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.ManagedClusterSpec().Return(fakeManagedClusterSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), fakeManagedClusterSpec, serviceName).Return(nil, errors.New("some unexpected error occurred"))
				s.UpdatePutStatus(infrav1.ManagedClusterRunningCondition, serviceName, errors.New("some unexpected error occurred"))
			},
		},
		{
			name:          "create managed cluster succeeds",
			expectedError: "",
			expect: func(m *mock_managedclusters.MockCredentialGetterMockRecorder, s *mock_managedclusters.MockManagedClusterScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.ManagedClusterSpec().Return(fakeManagedClusterSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), fakeManagedClusterSpec, serviceName).Return(containerservice.ManagedCluster{
					ManagedClusterProperties: &containerservice.ManagedClusterProperties{
						Fqdn:              pointer.String("my-managedcluster-fqdn"),
						ProvisioningState: pointer.String("Succeeded"),
						IdentityProfile: map[string]*containerservice.UserAssignedIdentity{
							kubeletIdentityKey: {
								ResourceID: pointer.String("kubelet-id"),
							},
						},
					},
				}, nil)
				s.SetControlPlaneEndpoint(clusterv1.APIEndpoint{
					Host: "my-managedcluster-fqdn",
					Port: 443,
				})
				m.GetCredentials(gomockinternal.AContext(), "my-rg", "my-managedcluster").Return([]byte("credentials"), nil)
				s.SetKubeConfigData([]byte("credentials"))
				s.SetKubeletIdentity("kubelet-id")
				s.UpdatePutStatus(infrav1.ManagedClusterRunningCondition, serviceName, nil)
			},
		},
		{
			name:          "fail to get managed cluster credentials",
			expectedError: "failed to get credentials for managed cluster: internal server error",
			expect: func(m *mock_managedclusters.MockCredentialGetterMockRecorder, s *mock_managedclusters.MockManagedClusterScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.ManagedClusterSpec().Return(fakeManagedClusterSpec)
				r.CreateOrUpdateResource(gomockinternal.AContext(), fakeManagedClusterSpec, serviceName).Return(containerservice.ManagedCluster{
					ManagedClusterProperties: &containerservice.ManagedClusterProperties{
						Fqdn:              pointer.String("my-managedcluster-fqdn"),
						ProvisioningState: pointer.String("Succeeded"),
					},
				}, nil)
				s.SetControlPlaneEndpoint(clusterv1.APIEndpoint{
					Host: "my-managedcluster-fqdn",
					Port: 443,
				})
				m.GetCredentials(gomockinternal.AContext(), "my-rg", "my-managedcluster").Return([]byte(""), errors.New("internal server error"))
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
			scopeMock := mock_managedclusters.NewMockManagedClusterScope(mockCtrl)
			credsGetterMock := mock_managedclusters.NewMockCredentialGetter(mockCtrl)
			reconcilerMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(credsGetterMock.EXPECT(), scopeMock.EXPECT(), reconcilerMock.EXPECT())

			s := &Service{
				Scope:            scopeMock,
				CredentialGetter: credsGetterMock,
				Reconciler:       reconcilerMock,
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

func TestDelete(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_managedclusters.MockManagedClusterScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "noop if no managed cluster spec is found",
			expectedError: "",
			expect: func(s *mock_managedclusters.MockManagedClusterScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.ManagedClusterSpec().Return(nil)
			},
		},
		{
			name:          "successfully delete an existing managed cluster",
			expectedError: "",
			expect: func(s *mock_managedclusters.MockManagedClusterScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.ManagedClusterSpec().Return(fakeManagedClusterSpec)
				r.DeleteResource(gomockinternal.AContext(), fakeManagedClusterSpec, serviceName).Return(nil)
				s.UpdateDeleteStatus(infrav1.ManagedClusterRunningCondition, serviceName, nil)
			},
		},
		{
			name:          "managed cluster deletion fails",
			expectedError: "internal error",
			expect: func(s *mock_managedclusters.MockManagedClusterScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.ManagedClusterSpec().Return(fakeManagedClusterSpec)
				r.DeleteResource(gomockinternal.AContext(), fakeManagedClusterSpec, serviceName).Return(errors.New("internal error"))
				s.UpdateDeleteStatus(infrav1.ManagedClusterRunningCondition, serviceName, errors.New("internal error"))
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
			scopeMock := mock_managedclusters.NewMockManagedClusterScope(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: asyncMock,
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

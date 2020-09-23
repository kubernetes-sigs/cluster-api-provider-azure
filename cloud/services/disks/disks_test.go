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

package disks

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/disks/mock_disks"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/golang/mock/gomock"

	"k8s.io/klog/klogr"
)

func TestDeleteDisk(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_disks.MockDiskScopeMockRecorder, m *mock_disks.MockClientMockRecorder)
	}{
		{
			name:          "delete the disk",
			expectedError: "",
			expect: func(s *mock_disks.MockDiskScopeMockRecorder, m *mock_disks.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.DiskSpecs().Return([]azure.DiskSpec{
					{
						Name: "my-disk-1",
					},
					{
						Name: "honk-disk",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(context.TODO(), "my-rg", "my-disk-1")
				m.Delete(context.TODO(), "my-rg", "honk-disk")
			},
		},
		{
			name:          "disk already deleted",
			expectedError: "",
			expect: func(s *mock_disks.MockDiskScopeMockRecorder, m *mock_disks.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.DiskSpecs().Return([]azure.DiskSpec{
					{
						Name: "my-disk-1",
					},
					{
						Name: "my-disk-2",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(context.TODO(), "my-rg", "my-disk-1").Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not Found"))
				m.Delete(context.TODO(), "my-rg", "my-disk-2").Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not Found"))
			},
		},
		{
			name:          "error while trying to delete the disk",
			expectedError: "failed to delete disk my-disk-1 in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_disks.MockDiskScopeMockRecorder, m *mock_disks.MockClientMockRecorder) {
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.DiskSpecs().Return([]azure.DiskSpec{
					{
						Name: "my-disk-1",
					},
					{
						Name: "my-disk-2",
					},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Delete(context.TODO(), "my-rg", "my-disk-1").Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
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
			scopeMock := mock_disks.NewMockDiskScope(mockCtrl)
			clientMock := mock_disks.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				Client: clientMock,
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

func TestDiskSpecs(t *testing.T) {
	testcases := []struct {
		name          string
		azureMachine  *infrav1.AzureMachine
		expectedDisks []azure.DiskSpec
	}{
		{
			name: "only os disk",
			azureMachine: &infrav1.AzureMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-azure-machine",
				},
				Spec: infrav1.AzureMachineSpec{
					OSDisk: infrav1.OSDisk{
						DiskSizeGB: 30,
						OSType:     "Linux",
					},
				},
			},
			expectedDisks: []azure.DiskSpec{
				{
					Name: "my-azure-machine_OSDisk",
				},
			},
		}, {
			name: "os and data disks",
			azureMachine: &infrav1.AzureMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-azure-machine",
				},
				Spec: infrav1.AzureMachineSpec{
					OSDisk: infrav1.OSDisk{
						DiskSizeGB: 30,
						OSType:     "Linux",
					},
					DataDisks: []infrav1.DataDisk{{
						NameSuffix: "etcddisk",
					}},
				},
			},
			expectedDisks: []azure.DiskSpec{
				{
					Name: "my-azure-machine_OSDisk",
				},
				{
					Name: "my-azure-machine_etcddisk",
				},
			},
		}, {
			name: "os and multiple data disks",
			azureMachine: &infrav1.AzureMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-azure-machine",
				},
				Spec: infrav1.AzureMachineSpec{
					OSDisk: infrav1.OSDisk{
						DiskSizeGB: 30,
						OSType:     "Linux",
					},
					DataDisks: []infrav1.DataDisk{
						{
							NameSuffix: "etcddisk",
						},
						{
							NameSuffix: "otherdisk",
						}},
				},
			},
			expectedDisks: []azure.DiskSpec{
				{
					Name: "my-azure-machine_OSDisk",
				},
				{
					Name: "my-azure-machine_etcddisk",
				},
				{
					Name: "my-azure-machine_otherdisk",
				},
			},
		}}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			scheme := runtime.NewScheme()
			g.Expect(infrav1.AddToScheme(scheme)).ToNot(HaveOccurred())
			g.Expect(clusterv1.AddToScheme(scheme)).ToNot(HaveOccurred())

			t.Parallel()
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-cluster",
				},
			}
			azureCluster := &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					SubscriptionID: "1234",
				},
			}
			machine := &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-machine",
				},
			}
			initObjects := []runtime.Object{
				cluster,
				machine,
				azureCluster,
				tc.azureMachine,
			}
			client := fake.NewFakeClientWithScheme(scheme, initObjects...)
			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				Client:       client,
				Cluster:      cluster,
				AzureCluster: azureCluster,
			})
			g.Expect(err).NotTo(HaveOccurred())
			machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
				Client:           client,
				ClusterDescriber: clusterScope,
				Machine:          machine,
				AzureMachine:     tc.azureMachine,
			})
			g.Expect(err).NotTo(HaveOccurred())

			output := machineScope.DiskSpecs()
			g.Expect(output).To(Equal(tc.expectedDisks))
		})
	}
}

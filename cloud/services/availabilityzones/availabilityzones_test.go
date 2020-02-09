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

package availabilityzones

import (
	"context"
	"net/http"
	"testing"

	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/availabilityzones/mock_availabilityzones"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	clusterv1.AddToScheme(scheme.Scheme)
}

func TestGetAvailabilityZones(t *testing.T) {
	testcases := []struct {
		name                 string
		availabilityZoneSpec Spec
		expectedError        string
		expect               func(m *mock_availabilityzones.MockClientMockRecorder)
	}{
		{
			name:                 "empty availability zones",
			availabilityZoneSpec: Spec{VMSize: "Standard_B2ms"},
			expectedError:        "",
			expect: func(m *mock_availabilityzones.MockClientMockRecorder) {
				m.ListComplete(context.TODO(), "").Return(compute.ResourceSkusResultIterator{}, nil)
			},
		},
		{
			name:                 "empty availability zones with error",
			availabilityZoneSpec: Spec{VMSize: "Standard_B2ms"},
			expectedError:        "#: Internal Server Error: StatusCode=500",
			expect: func(m *mock_availabilityzones.MockClientMockRecorder) {
				m.ListComplete(context.TODO(), "").Return(compute.ResourceSkusResultIterator{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:                 "availability zones exist",
			availabilityZoneSpec: Spec{VMSize: "Standard_B2ms"},
			expectedError:        "",
			expect: func(m *mock_availabilityzones.MockClientMockRecorder) {
				m.ListComplete(context.TODO(), "").Return(compute.NewResourceSkusResultIterator(compute.ResourceSkusResultPage{}), nil)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			azMock := mock_availabilityzones.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(azMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					SubscriptionID: "123",
					Authorizer:     autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						ResourceGroup: "my-rg",
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
							Subnets: []*infrav1.SubnetSpec{{
								Name: "my-subnet",
								Role: infrav1.SubnetNode,
							}},
						},
					},
				},
			})
			if err != nil {
				t.Fatalf("Failed to create test context: %v", err)
			}

			s := &Service{
				Scope:  clusterScope,
				Client: azMock,
			}

			if _, err := s.Get(context.TODO(), &tc.availabilityZoneSpec); err != nil {
				if tc.expectedError == "" || err.Error() != tc.expectedError {
					t.Fatalf("got an unexpected error: %v", err)
				}
			} else {
				if tc.expectedError != "" {
					t.Fatalf("expected an error: %v", tc.expectedError)

				}
			}
		})
	}
}

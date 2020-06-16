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

	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/availabilityzones/mock_availabilityzones"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-12-01/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"

	network "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
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

const (
	expectedInvalidSpec = "invalid availability zones specification"
	subscriptionID      = "123"
)

func TestInvalidAvailabilityZonesSpec(t *testing.T) {
	g := NewWithT(t)

	mockCtrl := gomock.NewController(t)
	agentpoolsMock := mock_availabilityzones.NewMockClient(mockCtrl)

	s := &Service{
		Client: agentpoolsMock,
	}

	// Wrong Spec
	wrongSpec := &network.LoadBalancer{}

	_, err := s.Get(context.TODO(), &wrongSpec)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(expectedInvalidSpec))
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
			availabilityZoneSpec: Spec{VMSize: to.StringPtr("Standard_B2ms")},
			expectedError:        "",
			expect: func(m *mock_availabilityzones.MockClientMockRecorder) {
				m.ListComplete(context.TODO(), "location eq 'centralus'").Return(compute.ResourceSkusResultIterator{}, nil)
			},
		},
		{
			name:                 "empty availability zones with error",
			availabilityZoneSpec: Spec{VMSize: to.StringPtr("Standard_B2ms")},
			expectedError:        "#: Internal Server Error: StatusCode=500",
			expect: func(m *mock_availabilityzones.MockClientMockRecorder) {
				m.ListComplete(context.TODO(), "location eq 'centralus'").Return(compute.ResourceSkusResultIterator{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:                 "availability zones exist",
			availabilityZoneSpec: Spec{VMSize: to.StringPtr("Standard_B2ms")},
			expectedError:        "",
			expect: func(m *mock_availabilityzones.MockClientMockRecorder) {
				m.ListComplete(context.TODO(), "location eq 'centralus'").Return(compute.NewResourceSkusResultIterator(compute.ResourceSkusResultPage{}), nil)
			},
		},
		{
			name:                 "no vmsize specified",
			availabilityZoneSpec: Spec{},
			expectedError:        "",
			expect: func(m *mock_availabilityzones.MockClientMockRecorder) {
				m.ListComplete(context.TODO(), "location eq 'centralus'").Return(compute.NewResourceSkusResultIterator(compute.ResourceSkusResultPage{}), nil)
			},
		},
		{
			name:                 "no vmsize (location unique)",
			availabilityZoneSpec: Spec{},
			expectedError:        "",
			expect: func(m *mock_availabilityzones.MockClientMockRecorder) {
				m.ListComplete(context.TODO(), "location eq 'centralus'").Return(compute.NewResourceSkusResultIterator(compute.ResourceSkusResultPage{}), nil)
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			mockCtrl := gomock.NewController(t)
			azMock := mock_availabilityzones.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(azMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location:       "centralus",
						ResourceGroup:  "my-rg",
						SubscriptionID: subscriptionID,
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
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:  clusterScope,
				Client: azMock,
			}

			_, err = s.Get(context.TODO(), &tc.availabilityZoneSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

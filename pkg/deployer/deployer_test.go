/*
Copyright 2018 The Kubernetes Authors.

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

package deployer_test

// TODO: Rewrite deployer tests
/*
import (
	"testing"

	//"github.com/azure/azure-sdk-go/azure"
	//"github.com/azure/azure-sdk-go/service/elb"
	"github.com/golang/mock/gomock"
	providerv1 "sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1" // nolint
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/compute/mock_ec2iface"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/elb/mock_elbiface"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloudtest"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/deployer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

type scopeGetter struct {
	actuators.AzureClients
}

func (s *scopeGetter) GetScope(params actuators.ScopeParams) (*actuators.Scope, error) {
	params.AzureClients = s.AzureClients
	return actuators.NewScope(params)
}

func TestGetIP(t *testing.T) {
	testcases := []struct {
		name       string
		cluster    *clusterv1.Cluster
		expectedIP string
		elbExpects func(*mock_elbiface.MockLBAPIMockRecorder)
	}{
		{
			name: "sunny day test",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test", ClusterName: "test", Namespace: "default"},
				Spec: clusterv1.ClusterSpec{
					ProviderSpec: clusterv1.ProviderSpec{
						Value: cloudtest.RuntimeRawExtension(t, &providerv1.AzureClusterProviderSpec{}),
					},
				},
				Status: clusterv1.ClusterStatus{
					ProviderStatus: cloudtest.RuntimeRawExtension(t, &providerv1.AzureClusterProviderStatus{}),
				},
			},
			expectedIP: "something",
			elbExpects: func(m *mock_elbiface.MockLBAPIMockRecorder) {
				m.DescribeLoadBalancers(&elb.DescribeLoadBalancersInput{
					LoadBalancerNames: []*string{azure.String("test-apiserver")},
				}).Return(&elb.DescribeLoadBalancersOutput{
					LoadBalancerDescriptions: []*elb.LoadBalancerDescription{
						{
							Scheme:  azure.String("internet-facing"),
							DNSName: azure.String("something"),
						},
					},
				}, nil)
			},
		},
		{
			name: "lookup IP if the status is empty",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test", ClusterName: "test", Namespace: "default"},
				Spec: clusterv1.ClusterSpec{
					ProviderSpec: clusterv1.ProviderSpec{
						Value: cloudtest.RuntimeRawExtension(t, &providerv1.AzureClusterProviderSpec{}),
					},
				},
			},
			expectedIP: "dunno",
			elbExpects: func(m *mock_elbiface.MockLBAPIMockRecorder) {
				m.DescribeLoadBalancers(&elb.DescribeLoadBalancersInput{
					LoadBalancerNames: []*string{azure.String("test-apiserver")},
				}).Return(&elb.DescribeLoadBalancersOutput{
					LoadBalancerDescriptions: []*elb.LoadBalancerDescription{
						{
							Scheme:  azure.String("internet-facing"),
							DNSName: azure.String("dunno"),
						},
					},
				}, nil)
			},
		},
		{
			name: "return the IP if it is stored in the status",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test", ClusterName: "test", Namespace: "default"},
				Spec: clusterv1.ClusterSpec{
					ProviderSpec: clusterv1.ProviderSpec{
						Value: cloudtest.RuntimeRawExtension(t, &providerv1.AzureClusterProviderSpec{}),
					},
				},
				Status: clusterv1.ClusterStatus{
					ProviderStatus: cloudtest.RuntimeRawExtension(t, &providerv1.AzureClusterProviderStatus{
						Network: providerv1.Network{
							APIServerLB: providerv1.LoadBalancer{
								DNSName: "banana",
							},
						},
					}),
				},
			},
			expectedIP: "banana",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			ec2Mock := mock_ec2iface.NewMockEC2API(mockCtrl)
			elbMock := mock_elbiface.NewMockLBAPI(mockCtrl)

			deployer := deployer.New(deployer.Params{ScopeGetter: &scopeGetter{
				actuators.AzureClients{
					EC2: ec2Mock,
					LB: elbMock,
				},
			}})

			if tc.elbExpects != nil {
				tc.elbExpects(elbMock.EXPECT())
			}

			ip, err := deployer.GetIP(tc.cluster, nil)
			if err != nil {
				t.Fatalf("failed to get API server address: %v", err)
			}

			if ip != tc.expectedIP {
				t.Fatalf("got the wrong IP. Found %v, wanted %v", ip, tc.expectedIP)
			}
		})
	}
}
*/

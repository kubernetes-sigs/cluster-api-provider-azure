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

package virtualmachines

import (
	"context"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/networkinterfaces/mock_networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips/mock_publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualmachines/mock_virtualmachines"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCreateVM(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bootstrap-data",
		},
		Data: map[string][]byte{
			"value": []byte("data"),
		},
	}

	testcases := []struct {
		name          string
		machine       clusterv1.Machine
		machineConfig *infrav1.AzureMachineSpec
		azureCluster  *infrav1.AzureCluster
		expect        func(m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder)
		checkError    func(err error)
	}{
		{
			name: "simple",
			machine: clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"set": "node"},
				},
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						Data: to.StringPtr("bootstrap-data"),
					},
					Version: to.StringPtr("1.15.7"),
				},
			},
			machineConfig: &infrav1.AzureMachineSpec{
				VMSize:   "Standard_B2ms",
				Location: "eastus",
				Image: &infrav1.Image{
					Publisher: to.StringPtr("test-publisher"),
					Offer:     to.StringPtr("test-offer"),
					SKU:       to.StringPtr("test-sku"),
					Version:   to.StringPtr("1.0.0"),
				},
			},
			azureCluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					NetworkSpec: infrav1.NetworkSpec{
						Subnets: infrav1.Subnets{
							&infrav1.SubnetSpec{
								Name: "subnet-1",
							},
							&infrav1.SubnetSpec{},
						},
					},
				},
				Status: infrav1.AzureClusterStatus{
					Network: infrav1.Network{
						SecurityGroups: map[infrav1.SecurityGroupRole]infrav1.SecurityGroup{
							infrav1.SecurityGroupControlPlane: {
								ID: "1",
							},
							infrav1.SecurityGroupNode: {
								ID: "2",
							},
						},
						APIServerIP: infrav1.PublicIP{
							DNSName: "azure-test-dns",
						},
					},
				},
			},
			expect: func(m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				mnic.Get(gomock.Any(), gomock.Any(), gomock.Any())
				m.CreateOrUpdate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			checkError: func(err error) {
				if err != nil {
					t.Fatalf("did not expect error: %v", err)
				}
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			vmMock := mock_virtualmachines.NewMockClient(mockCtrl)
			interfaceMock := mock_networkinterfaces.NewMockClient(mockCtrl)
			publicIPMock := mock_publicips.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test1",
				},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						ServiceDomain: "cluster.local",
						Services: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"192.168.0.0/16"},
						},
						Pods: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"192.168.0.0/16"},
						},
					},
				},
			}

			azureMachine := &infrav1.AzureMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "azure-test1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Machine",
							Name:       "test1",
						},
					},
				},
			}

			client := fake.NewFakeClient(secret, cluster, &tc.machine)

			machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
				Client:  client,
				Cluster: cluster,
				Machine: &tc.machine,
				AzureClients: scope.AzureClients{
					SubscriptionID: "123",
					Authorizer:     autorest.NullAuthorizer{},
				},
				AzureMachine: azureMachine,
				AzureCluster: tc.azureCluster,
			})
			if err != nil {
				t.Fatalf("Failed to create test context: %v", err)
			}
			machineScope.AzureMachine.Spec = *tc.machineConfig
			tc.expect(vmMock.EXPECT(), interfaceMock.EXPECT(), publicIPMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					SubscriptionID: "123",
					Authorizer:     autorest.NullAuthorizer{},
				},
				Client:       client,
				Cluster:      cluster,
				AzureCluster: tc.azureCluster,
			})
			if err != nil {
				t.Fatalf("Failed to create test context: %v", err)
			}

			s := &Service{
				Scope:            clusterScope,
				MachineScope:     machineScope,
				Client:           vmMock,
				InterfacesClient: interfaceMock,
				PublicIPsClient:  publicIPMock,
			}

			vmSpec := &Spec{
				Name:       machineScope.Name(),
				NICName:    "test-nic",
				SSHKeyData: "fake-key",
				Size:       machineScope.AzureMachine.Spec.VMSize,
				OSDisk:     machineScope.AzureMachine.Spec.OSDisk,
				Image:      *machineScope.AzureMachine.Spec.Image,
				CustomData: *machineScope.Machine.Spec.Bootstrap.Data,
			}
			err = s.Reconcile(context.TODO(), vmSpec)
			tc.checkError(err)
		})
	}
}

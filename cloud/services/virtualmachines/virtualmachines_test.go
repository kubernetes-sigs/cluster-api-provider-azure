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
	"net/http"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/networkinterfaces/mock_networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips/mock_publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/roleassignments/mock_roleassignments"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualmachines/mock_virtualmachines"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-12-01/compute"
	network "github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	expectedInvalidSpec = "invalid VM specification"
	subscriptionID      = "123"
)

func init() {
	clusterv1.AddToScheme(scheme.Scheme)
}

func TestInvalidVM(t *testing.T) {
	g := NewWithT(t)

	mockCtrl := gomock.NewController(t)
	vmMock := mock_virtualmachines.NewMockClient(mockCtrl)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
	}

	client := fake.NewFakeClient(cluster)

	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		AzureClients: scope.AzureClients{
			Authorizer: autorest.NullAuthorizer{},
		},
		Client:  client,
		Cluster: cluster,
		AzureCluster: &infrav1.AzureCluster{
			Spec: infrav1.AzureClusterSpec{
				Location: "test-location",
				ResourceGroup:  "my-rg",
				SubscriptionID: subscriptionID,
				NetworkSpec: infrav1.NetworkSpec{
					Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
				},
			},
		},
	})
	g.Expect(err).NotTo(HaveOccurred())

	s := &Service{
		Scope:  clusterScope,
		Client: vmMock,
	}

	// Wrong Spec
	wrongSpec := &network.PublicIPAddress{}

	_, err = s.Get(context.TODO(), &wrongSpec)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(expectedInvalidSpec))

	err = s.Reconcile(context.TODO(), &wrongSpec)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(expectedInvalidSpec))

	err = s.Delete(context.TODO(), &wrongSpec)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError(expectedInvalidSpec))
}

func TestGetVM(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name          string
		vmSpec        Spec
		expectedError string
		expect        func(m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder)
	}{
		{
			name: "get existing vm",
			vmSpec: Spec{
				Name: "my-vm",
			},
			expectedError: "",
			expect: func(m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				mpip.Get(context.TODO(), "my-rg", "my-publicIP-id").Return(network.PublicIPAddress{
					PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
						PublicIPAddressVersion:   network.IPv4,
						PublicIPAllocationMethod: network.Static,
						IPAddress:                to.StringPtr("4.3.2.1"),
					},
				}, nil)
				mnic.Get(context.TODO(), "my-rg", gomock.Any()).Return(network.Interface{
					InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
						IPConfigurations: &[]network.InterfaceIPConfiguration{
							{
								Name: to.StringPtr("pipConfig"),
								InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
									PrivateIPAddress: to.StringPtr("1.2.3.4"),
									PublicIPAddress: &network.PublicIPAddress{
										ID:   to.StringPtr("my-publicIP-id"),
										Name: to.StringPtr("my-publicIP"),
										PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
											PublicIPAddressVersion:   network.IPv4,
											PublicIPAllocationMethod: network.Static,
											IPAddress:                to.StringPtr("4.3.2.1"),
										},
									},
								},
							},
						},
					},
				}, nil)
				m.Get(context.TODO(), "my-rg", "my-vm").Return(compute.VirtualMachine{
					ID:   to.StringPtr("my-id"),
					Name: to.StringPtr("my-vm"),
					VirtualMachineProperties: &compute.VirtualMachineProperties{
						ProvisioningState: to.StringPtr("Succeeded"),
						NetworkProfile: &compute.NetworkProfile{
							NetworkInterfaces: &[]compute.NetworkInterfaceReference{
								{
									ID: to.StringPtr("my-nic-id"),
									NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{
										Primary: to.BoolPtr(true),
									},
								},
							},
						},
					},
				}, nil)
			},
		},
		{
			name: "vm not found",
			vmSpec: Spec{
				Name: "my-vm",
			},
			expectedError: "VM my-vm not found: #: Not found: StatusCode=404",
			expect: func(m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				mpip.Get(context.TODO(), "my-rg", "my-publicIP-id").Return(network.PublicIPAddress{}, nil)
				mnic.Get(context.TODO(), "my-rg", gomock.Any()).Return(network.Interface{}, nil)
				m.Get(context.TODO(), "my-rg", "my-vm").Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "vm retrieval fails",
			vmSpec: Spec{
				Name: "my-vm",
			},
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				mpip.Get(context.TODO(), "my-rg", "my-publicIP-id").Return(network.PublicIPAddress{}, nil)
				mnic.Get(context.TODO(), "my-rg", gomock.Any()).Return(network.Interface{}, nil)
				m.Get(context.TODO(), "my-rg", "my-vm").Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name: "get existing vm: error getting public IP",
			vmSpec: Spec{
				Name: "my-vm",
			},
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				mpip.Get(context.TODO(), "my-rg", "my-publicIP-id").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
				mnic.Get(context.TODO(), "my-rg", gomock.Any()).Return(network.Interface{
					InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
						IPConfigurations: &[]network.InterfaceIPConfiguration{
							{
								Name: to.StringPtr("pipConfig"),
								InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
									PrivateIPAddress: to.StringPtr("1.2.3.4"),
									PublicIPAddress: &network.PublicIPAddress{
										ID:   to.StringPtr("my-publicIP-id"),
										Name: to.StringPtr("my-publicIP"),
										PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
											PublicIPAddressVersion:   network.IPv4,
											PublicIPAllocationMethod: network.Static,
											IPAddress:                to.StringPtr("4.3.2.1"),
										},
									},
								},
							},
						},
					},
				}, nil)
				m.Get(context.TODO(), "my-rg", "my-vm").Return(compute.VirtualMachine{
					ID:   to.StringPtr("my-id"),
					Name: to.StringPtr("my-vm"),
					VirtualMachineProperties: &compute.VirtualMachineProperties{
						ProvisioningState: to.StringPtr("Succeeded"),
						NetworkProfile: &compute.NetworkProfile{
							NetworkInterfaces: &[]compute.NetworkInterfaceReference{
								{
									ID: to.StringPtr("my-nic-id"),
									NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{
										Primary: to.BoolPtr(true),
									},
								},
							},
						},
					},
				}, nil)
			},
		},
		{
			name: "get existing vm: public IP not found",
			vmSpec: Spec{
				Name: "my-vm",
			},
			expectedError: "#: Not Found: StatusCode=404",
			expect: func(m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				mpip.Get(context.TODO(), "my-rg", "my-publicIP-id").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not Found"))
				mnic.Get(context.TODO(), "my-rg", gomock.Any()).Return(network.Interface{
					InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
						IPConfigurations: &[]network.InterfaceIPConfiguration{
							{
								Name: to.StringPtr("pipConfig"),
								InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
									PrivateIPAddress: to.StringPtr("1.2.3.4"),
									PublicIPAddress: &network.PublicIPAddress{
										ID:   to.StringPtr("my-publicIP-id"),
										Name: to.StringPtr("my-publicIP"),
										PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
											PublicIPAddressVersion:   network.IPv4,
											PublicIPAllocationMethod: network.Static,
											IPAddress:                to.StringPtr("4.3.2.1"),
										},
									},
								},
							},
						},
					},
				}, nil)
				m.Get(context.TODO(), "my-rg", "my-vm").Return(compute.VirtualMachine{
					ID:   to.StringPtr("my-id"),
					Name: to.StringPtr("my-vm"),
					VirtualMachineProperties: &compute.VirtualMachineProperties{
						ProvisioningState: to.StringPtr("Succeeded"),
						NetworkProfile: &compute.NetworkProfile{
							NetworkInterfaces: &[]compute.NetworkInterfaceReference{
								{
									ID: to.StringPtr("my-nic-id"),
									NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{
										Primary: to.BoolPtr(true),
									},
								},
							},
						},
					},
				}, nil)
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
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(vmMock.EXPECT(), interfaceMock.EXPECT(), publicIPMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						ResourceGroup:  "my-rg",
						SubscriptionID: subscriptionID,
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
							Subnets: infrav1.Subnets{
								&infrav1.SubnetSpec{
									Name: "subnet-1",
								},
								&infrav1.SubnetSpec{},
							},
						},
					},
				},
			})
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:            clusterScope,
				Client:           vmMock,
				InterfacesClient: interfaceMock,
				PublicIPsClient:  publicIPMock,
			}

			_, err = s.Get(context.TODO(), &tc.vmSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestReconcileVM(t *testing.T) {
	g := NewWithT(t)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bootstrap-data",
		},
		Data: map[string][]byte{
			"value": []byte("data"),
		},
	}

	image := &infrav1.Image{
		Marketplace: &infrav1.AzureMarketplaceImage{
			Publisher: "test-publisher",
			Offer:     "test-offer",
			SKU:       "test-sku",
			Version:   "1.0.0.",
		},
	}

	testcases := []struct {
		name          string
		machine       clusterv1.Machine
		machineConfig *infrav1.AzureMachineSpec
		azureCluster  *infrav1.AzureCluster
		expect        func(m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder, mra *mock_roleassignments.MockClientMockRecorder)
		expectedError string
	}{
		{
			name: "can create a vm",
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
				Image:    image,
			},
			azureCluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					SubscriptionID: subscriptionID,
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
			expect: func(m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder, mra *mock_roleassignments.MockClientMockRecorder) {
				mnic.Get(gomock.Any(), gomock.Any(), gomock.Any())
				m.CreateOrUpdate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			expectedError: "",
		},
		{
			name: "can create a vm with system assigned identity",
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
				Image:    image,
				Identity: "SystemAssigned",
			},
			azureCluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					SubscriptionID: subscriptionID,
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
			expect: func(m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder, mra *mock_roleassignments.MockClientMockRecorder) {
				mnic.Get(gomock.Any(), gomock.Any(), gomock.Any())
				m.CreateOrUpdate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
				mra.Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			expectedError: "",
		},
		{
			name: "can create a vm with user assigned identity",
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
				VMSize:                 "Standard_B2ms",
				Location:               "eastus",
				Image:                  image,
				Identity:               "UserAssigned",
				UserAssignedIdentities: []infrav1.UserAssignedIdentity{{ProviderID: "azure:////subscriptions/123/resourcegroups/456/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id1"}},
			},
			azureCluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					SubscriptionID: subscriptionID,
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
			expect: func(m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder, mra *mock_roleassignments.MockClientMockRecorder) {
				mnic.Get(gomock.Any(), gomock.Any(), gomock.Any())
				m.CreateOrUpdate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
				mra.Create(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
			},
			expectedError: "",
		},
		{
			name: "vm creation fails",
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
				Image:    image,
			},
			azureCluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					SubscriptionID: subscriptionID,
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
			expect: func(m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder, mra *mock_roleassignments.MockClientMockRecorder) {
				mnic.Get(gomock.Any(), gomock.Any(), gomock.Any())
				m.CreateOrUpdate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
			expectedError: "cannot create VM: #: Internal Server Error: StatusCode=500",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			vmMock := mock_virtualmachines.NewMockClient(mockCtrl)
			interfaceMock := mock_networkinterfaces.NewMockClient(mockCtrl)
			publicIPMock := mock_publicips.NewMockClient(mockCtrl)
			roleAssignmentMock := mock_roleassignments.NewMockClient(mockCtrl)

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
					Authorizer: autorest.NullAuthorizer{},
				},
				AzureMachine: azureMachine,
				AzureCluster: tc.azureCluster,
			})
			g.Expect(err).NotTo(HaveOccurred())

			machineScope.AzureMachine.Spec = *tc.machineConfig
			tc.expect(vmMock.EXPECT(), interfaceMock.EXPECT(), publicIPMock.EXPECT(), roleAssignmentMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				Client:       client,
				Cluster:      cluster,
				AzureCluster: tc.azureCluster,
			})
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:                 clusterScope,
				MachineScope:          machineScope,
				Client:                vmMock,
				InterfacesClient:      interfaceMock,
				PublicIPsClient:       publicIPMock,
				RoleAssignmentsClient: roleAssignmentMock,
			}

			vmSpec := &Spec{
				Name:       machineScope.Name(),
				NICName:    "test-nic",
				SSHKeyData: "fake-key",
				Size:       machineScope.AzureMachine.Spec.VMSize,
				OSDisk:     machineScope.AzureMachine.Spec.OSDisk,
				Image:      machineScope.AzureMachine.Spec.Image,
				CustomData: *machineScope.Machine.Spec.Bootstrap.Data,
			}
			err = s.Reconcile(context.TODO(), vmSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteVM(t *testing.T) {
	g := NewWithT(t)

	testcases := []struct {
		name          string
		vmSpec        Spec
		expectedError string
		expect        func(m *mock_virtualmachines.MockClientMockRecorder)
	}{
		{
			name: "successfully delete an existing vm",
			vmSpec: Spec{
				Name: "my-vm",
			},
			expectedError: "",
			expect: func(m *mock_virtualmachines.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-vm")
			},
		},
		{
			name: "vm already deleted",
			vmSpec: Spec{
				Name: "my-vm",
			},
			expectedError: "",
			expect: func(m *mock_virtualmachines.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-vm").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name: "vm deletion fails",
			vmSpec: Spec{
				Name: "my-vm",
			},
			expectedError: "failed to delete VM my-vm in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(m *mock_virtualmachines.MockClientMockRecorder) {
				m.Delete(context.TODO(), "my-rg", "my-vm").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			publicIPsMock := mock_virtualmachines.NewMockClient(mockCtrl)

			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
			}

			client := fake.NewFakeClient(cluster)

			tc.expect(publicIPsMock.EXPECT())

			clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
				AzureClients: scope.AzureClients{
					Authorizer: autorest.NullAuthorizer{},
				},
				Client:  client,
				Cluster: cluster,
				AzureCluster: &infrav1.AzureCluster{
					Spec: infrav1.AzureClusterSpec{
						Location: "test-location",
						ResourceGroup:  "my-rg",
						SubscriptionID: subscriptionID,
						NetworkSpec: infrav1.NetworkSpec{
							Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
						},
					},
				},
			})
			g.Expect(err).NotTo(HaveOccurred())

			s := &Service{
				Scope:  clusterScope,
				Client: publicIPsMock,
			}

			err = s.Delete(context.TODO(), &tc.vmSpec)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

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

package scalesets

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	"github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/scalesets/mock_scalesets"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

func init() {
	_ = clusterv1.AddToScheme(scheme.Scheme)
}

func TestNewService(t *testing.T) {
	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
	}
	client := fake.NewFakeClientWithScheme(scheme.Scheme, cluster)
	s, err := scope.NewClusterScope(scope.ClusterScopeParams{
		AzureClients: scope.AzureClients{
			Authorizer: autorest.NullAuthorizer{},
		},
		Client:  client,
		Cluster: cluster,
		AzureCluster: &infrav1.AzureCluster{
			Spec: infrav1.AzureClusterSpec{
				Location: "test-location",
				ResourceGroup:  "my-rg",
				SubscriptionID: "123",
				NetworkSpec: infrav1.NetworkSpec{
					Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
				},
			},
		},
	})

	g := gomega.NewGomegaWithT(t)
	g.Expect(err).ToNot(gomega.HaveOccurred())

	mps, err := scope.NewMachinePoolScope(scope.MachinePoolScopeParams{
		Client:           client,
		Logger:           s.Logger,
		MachinePool:      new(clusterv1exp.MachinePool),
		AzureMachinePool: new(infrav1exp.AzureMachinePool),
		ClusterScope:     s,
	})
	g.Expect(err).ToNot(gomega.HaveOccurred())
	actual := NewService(mps, resourceskus.NewStaticCache(nil))
	g.Expect(actual).ToNot(gomega.BeNil())
}

func TestGetExistingVMSS(t *testing.T) {
	testcases := []struct {
		name          string
		vmssName      string
		result        *infrav1exp.VMSS
		expectedError string
		expect        func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder)
	}{
		{
			name:          "scale set not found",
			vmssName:      "my-vmss",
			result:        &infrav1exp.VMSS{},
			expectedError: "#: Not found: StatusCode=404",
			expect: func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name:     "get existing vmss",
			vmssName: "my-vmss",
			result: &infrav1exp.VMSS{
				ID:       "my-id",
				Name:     "my-vmss",
				State:    "Succeeded",
				Sku:      "Standard_D2",
				Identity: "",
				Tags:     nil,
				Capacity: int64(1),
				Instances: []infrav1exp.VMSSVM{
					{
						ID:         "id-1",
						InstanceID: "id-2",
						Name:       "instance-0",
						State:      "Succeeded",
					},
				},
			},
			expectedError: "",
			expect: func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").Return(compute.VirtualMachineScaleSet{
					ID:   to.StringPtr("my-id"),
					Name: to.StringPtr("my-vmss"),
					Sku: &compute.Sku{
						Capacity: to.Int64Ptr(1),
						Name:     to.StringPtr("Standard_D2"),
					},
					VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
						ProvisioningState: to.StringPtr("Succeeded"),
					},
				}, nil)
				m.ListInstances(gomockinternal.AContext(), "my-rg", "my-vmss").Return([]compute.VirtualMachineScaleSetVM{
					{
						InstanceID: to.StringPtr("id-2"),
						VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
							ProvisioningState: to.StringPtr("Succeeded"),
						},
						ID:   to.StringPtr("id-1"),
						Name: to.StringPtr("instance-0"),
					},
				}, nil)
			},
		},
		{
			name:          "list instances fails",
			vmssName:      "my-vmss",
			result:        &infrav1exp.VMSS{},
			expectedError: "#: Not found: StatusCode=404",
			expect: func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").Return(compute.VirtualMachineScaleSet{
					ID:   to.StringPtr("my-id"),
					Name: to.StringPtr("my-vmss"),
					VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
						ProvisioningState: to.StringPtr("Succeeded"),
					},
				}, nil)
				m.ListInstances(gomockinternal.AContext(), "my-rg", "my-vmss").Return([]compute.VirtualMachineScaleSetVM{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
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

			scopeMock := mock_scalesets.NewMockScaleSetScope(mockCtrl)
			clientMock := mock_scalesets.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:  scopeMock,
				Client: clientMock,
			}

			result, err := s.getExisting(context.TODO(), tc.vmssName)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(BeEquivalentTo(tc.result))
			}
		})
	}
}

func TestReconcileVMSS(t *testing.T) {
	testcases := []struct {
		name          string
		expect        func(g *gomega.WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder)
		expectedError string
	}{
		{
			name:          "can create a vmss",
			expectedError: "",
			expect: func(g *gomega.WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       "my-vmss",
					Size:       "VM_SIZE",
					Capacity:   2,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
					OSDisk: infrav1.OSDisk{
						OSType:     "Linux",
						DiskSizeGB: 120,
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "Premium_LRS",
						},
					},
					DataDisks: []infrav1.DataDisk{
						{
							NameSuffix: "my_disk",
							DiskSizeGB: 128,
							Lun:        to.Int32Ptr(0),
						},
					},
					SubnetName:                   "my-subnet",
					VNetName:                     "my-vnet",
					VNetResourceGroup:            "my-rg",
					PublicLBName:                 "capz-lb",
					PublicLBAddressPoolName:      "backendPool",
					AcceleratedNetworking:        nil,
					TerminateNotificationTimeout: to.IntPtr(7),
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AdditionalTags()
				s.Location().Return("test-location")
				s.ClusterName().Return("my-cluster")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").
					Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").
					Return(compute.VirtualMachineScaleSet{
						ID:   to.StringPtr("vmss-id"),
						Name: to.StringPtr("my-vmss"),
						VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
							ProvisioningState: to.StringPtr("Succeeded"),
						},
					}, nil)
				m.ListInstances(gomock.Any(), "my-rg", "my-vmss").Return([]compute.VirtualMachineScaleSetVM{
					{
						ID:         to.StringPtr("my-vm-id"),
						InstanceID: to.StringPtr("my-vm-1"),
						Name:       to.StringPtr("my-vm"),
						VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
							ProvisioningState: to.StringPtr("Succeeded"),
						},
					},
				}, nil)
				s.SetProviderID("azure://vmss-id")
				s.SetAnnotation("cluster-api-provider-azure", "true")
				s.SetProvisioningState(infrav1.VMStateSucceeded)
				s.UpdateInstanceStatuses(gomock.Any(), gomock.Len(1)).Return(nil)
				s.NeedsK8sVersionUpdate()
				s.SaveK8sVersion()
				s.GetVMImage().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher: "fake-publisher",
						Offer:     "my-offer",
						SKU:       "sku-id",
						Version:   "1.0",
					},
				}, nil)
				s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vmss", gomockinternal.DiffEq(compute.VirtualMachineScaleSet{
					Location: to.StringPtr("test-location"),
					Tags: map[string]*string{
						"Name": to.StringPtr("my-vmss"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"sigs.k8s.io_cluster-api-provider-azure_role":               to.StringPtr("node"),
					},
					Sku: &compute.Sku{
						Name:     to.StringPtr("VM_SIZE"),
						Tier:     to.StringPtr("Standard"),
						Capacity: to.Int64Ptr(2),
					},
					VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
						UpgradePolicy: &compute.UpgradePolicy{
							Mode: compute.UpgradeModeRolling,
						},
						VirtualMachineProfile: &compute.VirtualMachineScaleSetVMProfile{
							OsProfile: &compute.VirtualMachineScaleSetOSProfile{
								ComputerNamePrefix: to.StringPtr("my-vmss"),
								AdminUsername:      to.StringPtr(azure.DefaultUserName),
								CustomData:         to.StringPtr("fake-bootstrap-data"),
								LinuxConfiguration: &compute.LinuxConfiguration{
									SSH: &compute.SSHConfiguration{
										PublicKeys: &[]compute.SSHPublicKey{
											{
												Path:    to.StringPtr("/home/capi/.ssh/authorized_keys"),
												KeyData: to.StringPtr("fakesshkey\n"),
											},
										},
									},
									DisablePasswordAuthentication: to.BoolPtr(true),
								},
							},
							StorageProfile: &compute.VirtualMachineScaleSetStorageProfile{
								ImageReference: &compute.ImageReference{
									Publisher: to.StringPtr("fake-publisher"),
									Offer:     to.StringPtr("my-offer"),
									Sku:       to.StringPtr("sku-id"),
									Version:   to.StringPtr("1.0"),
								},
								OsDisk: &compute.VirtualMachineScaleSetOSDisk{
									OsType:       "Linux",
									CreateOption: "FromImage",
									DiskSizeGB:   to.Int32Ptr(120),
									ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
										StorageAccountType: "Premium_LRS",
									},
								},
								DataDisks: &[]compute.VirtualMachineScaleSetDataDisk{
									{
										Lun:          to.Int32Ptr(0),
										Name:         to.StringPtr("my-vmss_my_disk"),
										CreateOption: "Empty",
										DiskSizeGB:   to.Int32Ptr(128),
									},
								},
							},
							DiagnosticsProfile: &compute.DiagnosticsProfile{
								BootDiagnostics: &compute.BootDiagnostics{
									Enabled: to.BoolPtr(true),
								},
							},
							NetworkProfile: &compute.VirtualMachineScaleSetNetworkProfile{
								NetworkInterfaceConfigurations: &[]compute.VirtualMachineScaleSetNetworkConfiguration{
									{
										Name: to.StringPtr("my-vmss-netconfig"),
										VirtualMachineScaleSetNetworkConfigurationProperties: &compute.VirtualMachineScaleSetNetworkConfigurationProperties{
											Primary:                     to.BoolPtr(true),
											EnableAcceleratedNetworking: to.BoolPtr(false),
											EnableIPForwarding:          to.BoolPtr(true),
											IPConfigurations: &[]compute.VirtualMachineScaleSetIPConfiguration{
												{
													Name: to.StringPtr("my-vmss-ipconfig"),
													VirtualMachineScaleSetIPConfigurationProperties: &compute.VirtualMachineScaleSetIPConfigurationProperties{
														Subnet: &compute.APIEntityReference{
															ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet"),
														},
														Primary:                         to.BoolPtr(true),
														PrivateIPAddressVersion:         compute.IPv4,
														LoadBalancerBackendAddressPools: &[]compute.SubResource{{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/capz-lb/backendAddressPools/backendPool")}},
													},
												},
											},
										},
									},
								},
							},
							ScheduledEventsProfile: &compute.ScheduledEventsProfile{
								TerminateNotificationProfile: &compute.TerminateNotificationProfile{
									Enable:           to.BoolPtr(true),
									NotBeforeTimeout: to.StringPtr("PT7M"),
								},
							},
						},
					},
				}))
			},
		},
		{
			name:          "with accelerated networking enabled",
			expectedError: "",
			expect: func(g *gomega.WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       "my-vmss",
					Size:       "VM_SIZE_AN",
					Capacity:   2,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
					OSDisk: infrav1.OSDisk{
						OSType:     "Linux",
						DiskSizeGB: 120,
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "Premium_LRS",
						},
					},
					DataDisks: []infrav1.DataDisk{
						{
							NameSuffix: "my_disk",
							DiskSizeGB: 128,
							Lun:        to.Int32Ptr(0),
						},
					},
					SubnetName:              "my-subnet",
					VNetName:                "my-vnet",
					VNetResourceGroup:       "my-rg",
					PublicLBName:            "capz-lb",
					PublicLBAddressPoolName: "backendPool",
					AcceleratedNetworking:   nil,
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AdditionalTags()
				s.Location().Return("test-location")
				s.ClusterName().Return("my-cluster")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").
					Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").
					Return(compute.VirtualMachineScaleSet{
						ID:   to.StringPtr("vmss-id"),
						Name: to.StringPtr("my-vmss"),
						VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
							ProvisioningState: to.StringPtr("Succeeded"),
						},
					}, nil)
				m.ListInstances(gomock.Any(), "my-rg", "my-vmss").Return([]compute.VirtualMachineScaleSetVM{
					{
						ID:         to.StringPtr("my-vm-id"),
						InstanceID: to.StringPtr("my-vm-1"),
						Name:       to.StringPtr("my-vm"),
						VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
							ProvisioningState: to.StringPtr("Succeeded"),
						},
					},
				}, nil)
				s.SetProviderID("azure://vmss-id")
				s.SetAnnotation("cluster-api-provider-azure", "true")
				s.SetProvisioningState(infrav1.VMStateSucceeded)
				s.UpdateInstanceStatuses(gomock.Any(), gomock.Len(1)).Return(nil)
				s.NeedsK8sVersionUpdate()
				s.SaveK8sVersion()
				s.GetVMImage().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher: "fake-publisher",
						Offer:     "my-offer",
						SKU:       "sku-id",
						Version:   "1.0",
					},
				}, nil)
				s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vmss", gomockinternal.DiffEq(compute.VirtualMachineScaleSet{
					Location: to.StringPtr("test-location"),
					Tags: map[string]*string{
						"Name": to.StringPtr("my-vmss"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"sigs.k8s.io_cluster-api-provider-azure_role":               to.StringPtr("node"),
					},
					Sku: &compute.Sku{
						Name:     to.StringPtr("VM_SIZE_AN"),
						Tier:     to.StringPtr("Standard"),
						Capacity: to.Int64Ptr(2),
					},
					VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
						UpgradePolicy: &compute.UpgradePolicy{
							Mode: compute.UpgradeModeManual,
						},
						VirtualMachineProfile: &compute.VirtualMachineScaleSetVMProfile{
							OsProfile: &compute.VirtualMachineScaleSetOSProfile{
								ComputerNamePrefix: to.StringPtr("my-vmss"),
								AdminUsername:      to.StringPtr(azure.DefaultUserName),
								CustomData:         to.StringPtr("fake-bootstrap-data"),
								LinuxConfiguration: &compute.LinuxConfiguration{
									SSH: &compute.SSHConfiguration{
										PublicKeys: &[]compute.SSHPublicKey{
											{
												Path:    to.StringPtr("/home/capi/.ssh/authorized_keys"),
												KeyData: to.StringPtr("fakesshkey\n"),
											},
										},
									},
									DisablePasswordAuthentication: to.BoolPtr(true),
								},
							},
							StorageProfile: &compute.VirtualMachineScaleSetStorageProfile{
								ImageReference: &compute.ImageReference{
									Publisher: to.StringPtr("fake-publisher"),
									Offer:     to.StringPtr("my-offer"),
									Sku:       to.StringPtr("sku-id"),
									Version:   to.StringPtr("1.0"),
								},
								OsDisk: &compute.VirtualMachineScaleSetOSDisk{
									OsType:       "Linux",
									CreateOption: "FromImage",
									DiskSizeGB:   to.Int32Ptr(120),
									ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
										StorageAccountType: "Premium_LRS",
									},
								},
								DataDisks: &[]compute.VirtualMachineScaleSetDataDisk{
									{
										Lun:          to.Int32Ptr(0),
										Name:         to.StringPtr("my-vmss_my_disk"),
										CreateOption: "Empty",
										DiskSizeGB:   to.Int32Ptr(128),
									},
								},
							},
							DiagnosticsProfile: &compute.DiagnosticsProfile{
								BootDiagnostics: &compute.BootDiagnostics{
									Enabled: to.BoolPtr(true),
								},
							},
							NetworkProfile: &compute.VirtualMachineScaleSetNetworkProfile{
								NetworkInterfaceConfigurations: &[]compute.VirtualMachineScaleSetNetworkConfiguration{
									{
										Name: to.StringPtr("my-vmss-netconfig"),
										VirtualMachineScaleSetNetworkConfigurationProperties: &compute.VirtualMachineScaleSetNetworkConfigurationProperties{
											Primary:                     to.BoolPtr(true),
											EnableAcceleratedNetworking: to.BoolPtr(true),
											EnableIPForwarding:          to.BoolPtr(true),
											IPConfigurations: &[]compute.VirtualMachineScaleSetIPConfiguration{
												{
													Name: to.StringPtr("my-vmss-ipconfig"),
													VirtualMachineScaleSetIPConfigurationProperties: &compute.VirtualMachineScaleSetIPConfigurationProperties{
														Subnet: &compute.APIEntityReference{
															ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet"),
														},
														Primary:                         to.BoolPtr(true),
														PrivateIPAddressVersion:         compute.IPv4,
														LoadBalancerBackendAddressPools: &[]compute.SubResource{{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/capz-lb/backendAddressPools/backendPool")}},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				}))
			},
		},
		{
			name:          "can create a vmss with spot vm",
			expectedError: "",
			expect: func(g *gomega.WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       "my-vmss",
					Size:       "VM_SIZE_AN",
					Capacity:   2,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
					OSDisk: infrav1.OSDisk{
						OSType:     "Linux",
						DiskSizeGB: 120,
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "Premium_LRS",
						},
					},
					DataDisks: []infrav1.DataDisk{
						{
							NameSuffix: "my_disk",
							DiskSizeGB: 128,
							Lun:        to.Int32Ptr(0),
						},
					},
					SubnetName:              "my-subnet",
					VNetName:                "my-vnet",
					VNetResourceGroup:       "my-rg",
					PublicLBName:            "capz-lb",
					PublicLBAddressPoolName: "backendPool",
					AcceleratedNetworking:   nil,
					SpotVMOptions:           &infrav1.SpotVMOptions{},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AdditionalTags()
				s.Location().Return("test-location")
				s.ClusterName().Return("my-cluster")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").
					Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").
					Return(compute.VirtualMachineScaleSet{
						ID:   to.StringPtr("vmss-id"),
						Name: to.StringPtr("my-vmss"),
						VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
							ProvisioningState: to.StringPtr("Succeeded"),
						},
					}, nil)
				m.ListInstances(gomock.Any(), "my-rg", "my-vmss").Return([]compute.VirtualMachineScaleSetVM{
					{
						ID:         to.StringPtr("my-vm-id"),
						InstanceID: to.StringPtr("my-vm-1"),
						Name:       to.StringPtr("my-vm"),
						VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
							ProvisioningState: to.StringPtr("Succeeded"),
						},
					},
				}, nil)
				s.SetProviderID("azure://vmss-id")
				s.SetAnnotation("cluster-api-provider-azure", "true")
				s.SetProvisioningState(infrav1.VMStateSucceeded)
				s.UpdateInstanceStatuses(gomock.Any(), gomock.Len(1)).Return(nil)
				s.NeedsK8sVersionUpdate()
				s.SaveK8sVersion()
				s.GetVMImage().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher: "fake-publisher",
						Offer:     "my-offer",
						SKU:       "sku-id",
						Version:   "1.0",
					},
				}, nil)
				s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vmss", gomockinternal.DiffEq(compute.VirtualMachineScaleSet{
					Location: to.StringPtr("test-location"),
					Tags: map[string]*string{
						"Name": to.StringPtr("my-vmss"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"sigs.k8s.io_cluster-api-provider-azure_role":               to.StringPtr("node"),
					},
					Sku: &compute.Sku{
						Name:     to.StringPtr("VM_SIZE_AN"),
						Tier:     to.StringPtr("Standard"),
						Capacity: to.Int64Ptr(2),
					},
					VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
						UpgradePolicy: &compute.UpgradePolicy{
							Mode: compute.UpgradeModeManual,
						},
						VirtualMachineProfile: &compute.VirtualMachineScaleSetVMProfile{
							OsProfile: &compute.VirtualMachineScaleSetOSProfile{
								ComputerNamePrefix: to.StringPtr("my-vmss"),
								AdminUsername:      to.StringPtr(azure.DefaultUserName),
								CustomData:         to.StringPtr("fake-bootstrap-data"),
								LinuxConfiguration: &compute.LinuxConfiguration{
									SSH: &compute.SSHConfiguration{
										PublicKeys: &[]compute.SSHPublicKey{
											{
												Path:    to.StringPtr("/home/capi/.ssh/authorized_keys"),
												KeyData: to.StringPtr("fakesshkey\n"),
											},
										},
									},
									DisablePasswordAuthentication: to.BoolPtr(true),
								},
							},
							StorageProfile: &compute.VirtualMachineScaleSetStorageProfile{
								ImageReference: &compute.ImageReference{
									Publisher: to.StringPtr("fake-publisher"),
									Offer:     to.StringPtr("my-offer"),
									Sku:       to.StringPtr("sku-id"),
									Version:   to.StringPtr("1.0"),
								},
								OsDisk: &compute.VirtualMachineScaleSetOSDisk{
									OsType:       "Linux",
									CreateOption: "FromImage",
									DiskSizeGB:   to.Int32Ptr(120),
									ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
										StorageAccountType: "Premium_LRS",
									},
								},
								DataDisks: &[]compute.VirtualMachineScaleSetDataDisk{
									{
										Lun:          to.Int32Ptr(0),
										Name:         to.StringPtr("my-vmss_my_disk"),
										CreateOption: "Empty",
										DiskSizeGB:   to.Int32Ptr(128),
									},
								},
							},
							DiagnosticsProfile: &compute.DiagnosticsProfile{
								BootDiagnostics: &compute.BootDiagnostics{
									Enabled: to.BoolPtr(true),
								},
							},
							NetworkProfile: &compute.VirtualMachineScaleSetNetworkProfile{
								NetworkInterfaceConfigurations: &[]compute.VirtualMachineScaleSetNetworkConfiguration{
									{
										Name: to.StringPtr("my-vmss-netconfig"),
										VirtualMachineScaleSetNetworkConfigurationProperties: &compute.VirtualMachineScaleSetNetworkConfigurationProperties{
											Primary:                     to.BoolPtr(true),
											EnableAcceleratedNetworking: to.BoolPtr(true),
											EnableIPForwarding:          to.BoolPtr(true),
											IPConfigurations: &[]compute.VirtualMachineScaleSetIPConfiguration{
												{
													Name: to.StringPtr("my-vmss-ipconfig"),
													VirtualMachineScaleSetIPConfigurationProperties: &compute.VirtualMachineScaleSetIPConfigurationProperties{
														Subnet: &compute.APIEntityReference{
															ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet"),
														},
														Primary:                         to.BoolPtr(true),
														PrivateIPAddressVersion:         compute.IPv4,
														LoadBalancerBackendAddressPools: &[]compute.SubResource{{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/capz-lb/backendAddressPools/backendPool")}},
													},
												},
											},
										},
									},
								},
							},
							Priority:       compute.Spot,
							EvictionPolicy: compute.Deallocate,
							BillingProfile: nil,
						},
					},
				}))
			},
		},
		{
			name:          "can create a vmss with encryption",
			expectedError: "",
			expect: func(g *gomega.WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       "my-vmss",
					Size:       "VM_SIZE",
					Capacity:   2,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
					OSDisk: infrav1.OSDisk{
						OSType:     "Linux",
						DiskSizeGB: 120,
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "Premium_LRS",
							DiskEncryptionSet: &infrav1.DiskEncryptionSetParameters{
								ID: "my-diskencryptionset-id",
							},
						},
					},
					DataDisks: []infrav1.DataDisk{
						{
							NameSuffix: "my_disk",
							DiskSizeGB: 128,
							Lun:        to.Int32Ptr(0),
						},
					},
					SubnetName:                   "my-subnet",
					VNetName:                     "my-vnet",
					VNetResourceGroup:            "my-rg",
					PublicLBName:                 "capz-lb",
					PublicLBAddressPoolName:      "backendPool",
					AcceleratedNetworking:        nil,
					TerminateNotificationTimeout: to.IntPtr(7),
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AdditionalTags()
				s.Location().Return("test-location")
				s.ClusterName().Return("my-cluster")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").
					Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.GetVMImage().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher: "fake-publisher",
						Offer:     "my-offer",
						SKU:       "sku-id",
						Version:   "1.0",
					},
				}, nil)
				s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vmss", gomockinternal.DiffEq(compute.VirtualMachineScaleSet{
					Location: to.StringPtr("test-location"),
					Tags: map[string]*string{
						"Name": to.StringPtr("my-vmss"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"sigs.k8s.io_cluster-api-provider-azure_role":               to.StringPtr("node"),
					},
					Sku: &compute.Sku{
						Name:     to.StringPtr("VM_SIZE"),
						Tier:     to.StringPtr("Standard"),
						Capacity: to.Int64Ptr(2),
					},
					VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
						UpgradePolicy: &compute.UpgradePolicy{
							Mode: compute.UpgradeModeRolling,
						},
						VirtualMachineProfile: &compute.VirtualMachineScaleSetVMProfile{
							OsProfile: &compute.VirtualMachineScaleSetOSProfile{
								ComputerNamePrefix: to.StringPtr("my-vmss"),
								AdminUsername:      to.StringPtr(azure.DefaultUserName),
								CustomData:         to.StringPtr("fake-bootstrap-data"),
								LinuxConfiguration: &compute.LinuxConfiguration{
									SSH: &compute.SSHConfiguration{
										PublicKeys: &[]compute.SSHPublicKey{
											{
												Path:    to.StringPtr("/home/capi/.ssh/authorized_keys"),
												KeyData: to.StringPtr("fakesshkey\n"),
											},
										},
									},
									DisablePasswordAuthentication: to.BoolPtr(true),
								},
							},
							StorageProfile: &compute.VirtualMachineScaleSetStorageProfile{
								ImageReference: &compute.ImageReference{
									Publisher: to.StringPtr("fake-publisher"),
									Offer:     to.StringPtr("my-offer"),
									Sku:       to.StringPtr("sku-id"),
									Version:   to.StringPtr("1.0"),
								},
								OsDisk: &compute.VirtualMachineScaleSetOSDisk{
									OsType:       "Linux",
									CreateOption: "FromImage",
									DiskSizeGB:   to.Int32Ptr(120),
									ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
										StorageAccountType: "Premium_LRS",
										DiskEncryptionSet: &compute.DiskEncryptionSetParameters{
											ID: to.StringPtr("my-diskencryptionset-id"),
										},
									},
								},
								DataDisks: &[]compute.VirtualMachineScaleSetDataDisk{
									{
										Lun:          to.Int32Ptr(0),
										Name:         to.StringPtr("my-vmss_my_disk"),
										CreateOption: "Empty",
										DiskSizeGB:   to.Int32Ptr(128),
									},
								},
							},
							DiagnosticsProfile: &compute.DiagnosticsProfile{
								BootDiagnostics: &compute.BootDiagnostics{
									Enabled: to.BoolPtr(true),
								},
							},
							NetworkProfile: &compute.VirtualMachineScaleSetNetworkProfile{
								NetworkInterfaceConfigurations: &[]compute.VirtualMachineScaleSetNetworkConfiguration{
									{
										Name: to.StringPtr("my-vmss-netconfig"),
										VirtualMachineScaleSetNetworkConfigurationProperties: &compute.VirtualMachineScaleSetNetworkConfigurationProperties{
											Primary:                     to.BoolPtr(true),
											EnableAcceleratedNetworking: to.BoolPtr(false),
											EnableIPForwarding:          to.BoolPtr(true),
											IPConfigurations: &[]compute.VirtualMachineScaleSetIPConfiguration{
												{
													Name: to.StringPtr("my-vmss-ipconfig"),
													VirtualMachineScaleSetIPConfigurationProperties: &compute.VirtualMachineScaleSetIPConfigurationProperties{
														Subnet: &compute.APIEntityReference{
															ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet"),
														},
														Primary:                         to.BoolPtr(true),
														PrivateIPAddressVersion:         compute.IPv4,
														LoadBalancerBackendAddressPools: &[]compute.SubResource{{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/capz-lb/backendAddressPools/backendPool")}},
													},
												},
											},
										},
									},
								},
							},
							ScheduledEventsProfile: &compute.ScheduledEventsProfile{
								TerminateNotificationProfile: &compute.TerminateNotificationProfile{
									Enable:           to.BoolPtr(true),
									NotBeforeTimeout: to.StringPtr("PT7M"),
								},
							},
						},
					},
				}))
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").
					Return(compute.VirtualMachineScaleSet{
						ID:   to.StringPtr("vmss-id"),
						Name: to.StringPtr("my-vmss"),
						VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
							ProvisioningState: to.StringPtr("Succeeded"),
						},
					}, nil)
				m.ListInstances(gomockinternal.AContext(), "my-rg", "my-vmss").Return([]compute.VirtualMachineScaleSetVM{
					{
						InstanceID: to.StringPtr("id-2"),
						VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
							ProvisioningState: to.StringPtr("Succeeded"),
						},
						ID:   to.StringPtr("id-1"),
						Name: to.StringPtr("instance-0"),
					},
				}, nil)
				s.SaveK8sVersion()
				s.NeedsK8sVersionUpdate()
				s.UpdateInstanceStatuses(gomock.Any(), gomock.Len(1)).Return(nil)
				s.SetProviderID("azure://vmss-id")
				s.SetAnnotation("cluster-api-provider-azure", "true")
				s.SetProvisioningState(infrav1.VMStateSucceeded)
			},
		},
		{
			name:          "can create a vmss with user assigned identity",
			expectedError: "",
			expect: func(g *gomega.WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       "my-vmss",
					Size:       "VM_SIZE",
					Capacity:   2,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
					OSDisk: infrav1.OSDisk{
						OSType:     "Linux",
						DiskSizeGB: 120,
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "Premium_LRS",
						},
					},
					DataDisks: []infrav1.DataDisk{
						{
							NameSuffix: "my_disk",
							DiskSizeGB: 128,
							Lun:        to.Int32Ptr(0),
						},
					},
					SubnetName:                   "my-subnet",
					VNetName:                     "my-vnet",
					VNetResourceGroup:            "my-rg",
					PublicLBName:                 "capz-lb",
					PublicLBAddressPoolName:      "backendPool",
					AcceleratedNetworking:        nil,
					TerminateNotificationTimeout: to.IntPtr(7),
					Identity:                     infrav1.VMIdentityUserAssigned,
					UserAssignedIdentities: []infrav1.UserAssignedIdentity{
						{
							ProviderID: "azure:////subscriptions/123/resourcegroups/456/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id1",
						},
					},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AdditionalTags()
				s.Location().Return("test-location")
				s.ClusterName().Return("my-cluster")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").
					Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").
					Return(compute.VirtualMachineScaleSet{
						ID:   to.StringPtr("vmss-id"),
						Name: to.StringPtr("my-vmss"),
						VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
							ProvisioningState: to.StringPtr("Succeeded"),
						},
					}, nil)
				m.ListInstances(gomock.Any(), "my-rg", "my-vmss").Return([]compute.VirtualMachineScaleSetVM{
					{
						ID:         to.StringPtr("my-vm-id"),
						InstanceID: to.StringPtr("my-vm-1"),
						Name:       to.StringPtr("my-vm"),
						VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
							ProvisioningState: to.StringPtr("Succeeded"),
						},
					},
				}, nil)
				s.SetProviderID("azure://vmss-id")
				s.SetAnnotation("cluster-api-provider-azure", "true")
				s.SetProvisioningState(infrav1.VMStateSucceeded)
				s.UpdateInstanceStatuses(gomock.Any(), gomock.Len(1)).Return(nil)
				s.NeedsK8sVersionUpdate()
				s.SaveK8sVersion()
				s.GetVMImage().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher: "fake-publisher",
						Offer:     "my-offer",
						SKU:       "sku-id",
						Version:   "1.0",
					},
				}, nil)
				s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vmss", gomockinternal.DiffEq(compute.VirtualMachineScaleSet{
					Location: to.StringPtr("test-location"),
					Tags: map[string]*string{
						"Name": to.StringPtr("my-vmss"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"sigs.k8s.io_cluster-api-provider-azure_role":               to.StringPtr("node"),
					},
					Sku: &compute.Sku{
						Name:     to.StringPtr("VM_SIZE"),
						Tier:     to.StringPtr("Standard"),
						Capacity: to.Int64Ptr(2),
					},
					VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
						UpgradePolicy: &compute.UpgradePolicy{
							Mode: compute.UpgradeModeRolling,
						},
						VirtualMachineProfile: &compute.VirtualMachineScaleSetVMProfile{
							OsProfile: &compute.VirtualMachineScaleSetOSProfile{
								ComputerNamePrefix: to.StringPtr("my-vmss"),
								AdminUsername:      to.StringPtr(azure.DefaultUserName),
								CustomData:         to.StringPtr("fake-bootstrap-data"),
								LinuxConfiguration: &compute.LinuxConfiguration{
									SSH: &compute.SSHConfiguration{
										PublicKeys: &[]compute.SSHPublicKey{
											{
												Path:    to.StringPtr("/home/capi/.ssh/authorized_keys"),
												KeyData: to.StringPtr("fakesshkey\n"),
											},
										},
									},
									DisablePasswordAuthentication: to.BoolPtr(true),
								},
							},
							StorageProfile: &compute.VirtualMachineScaleSetStorageProfile{
								ImageReference: &compute.ImageReference{
									Publisher: to.StringPtr("fake-publisher"),
									Offer:     to.StringPtr("my-offer"),
									Sku:       to.StringPtr("sku-id"),
									Version:   to.StringPtr("1.0"),
								},
								OsDisk: &compute.VirtualMachineScaleSetOSDisk{
									OsType:       "Linux",
									CreateOption: "FromImage",
									DiskSizeGB:   to.Int32Ptr(120),
									ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
										StorageAccountType: "Premium_LRS",
									},
								},
								DataDisks: &[]compute.VirtualMachineScaleSetDataDisk{
									{
										Lun:          to.Int32Ptr(0),
										Name:         to.StringPtr("my-vmss_my_disk"),
										CreateOption: "Empty",
										DiskSizeGB:   to.Int32Ptr(128),
									},
								},
							},
							DiagnosticsProfile: &compute.DiagnosticsProfile{
								BootDiagnostics: &compute.BootDiagnostics{
									Enabled: to.BoolPtr(true),
								},
							},
							NetworkProfile: &compute.VirtualMachineScaleSetNetworkProfile{
								NetworkInterfaceConfigurations: &[]compute.VirtualMachineScaleSetNetworkConfiguration{
									{
										Name: to.StringPtr("my-vmss-netconfig"),
										VirtualMachineScaleSetNetworkConfigurationProperties: &compute.VirtualMachineScaleSetNetworkConfigurationProperties{
											Primary:                     to.BoolPtr(true),
											EnableAcceleratedNetworking: to.BoolPtr(false),
											EnableIPForwarding:          to.BoolPtr(true),
											IPConfigurations: &[]compute.VirtualMachineScaleSetIPConfiguration{
												{
													Name: to.StringPtr("my-vmss-ipconfig"),
													VirtualMachineScaleSetIPConfigurationProperties: &compute.VirtualMachineScaleSetIPConfigurationProperties{
														Subnet: &compute.APIEntityReference{
															ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet"),
														},
														Primary:                         to.BoolPtr(true),
														PrivateIPAddressVersion:         compute.IPv4,
														LoadBalancerBackendAddressPools: &[]compute.SubResource{{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/capz-lb/backendAddressPools/backendPool")}},
													},
												},
											},
										},
									},
								},
							},
							ScheduledEventsProfile: &compute.ScheduledEventsProfile{
								TerminateNotificationProfile: &compute.TerminateNotificationProfile{
									Enable:           to.BoolPtr(true),
									NotBeforeTimeout: to.StringPtr("PT7M"),
								},
							},
						},
					},
					Identity: &compute.VirtualMachineScaleSetIdentity{
						Type: compute.ResourceIdentityTypeUserAssigned,
						UserAssignedIdentities: map[string]*compute.VirtualMachineScaleSetIdentityUserAssignedIdentitiesValue{
							"/subscriptions/123/resourcegroups/456/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id1": {},
						},
					},
				}))
			},
		},
		{
			name:          "can create a vmss with encryption at host enabled",
			expectedError: "",
			expect: func(g *gomega.WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       "my-vmss",
					Size:       "VM_SIZE_EAH",
					Capacity:   2,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
					OSDisk: infrav1.OSDisk{
						OSType:     "Linux",
						DiskSizeGB: 120,
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "Premium_LRS",
						},
					},
					SubnetName:                   "my-subnet",
					VNetName:                     "my-vnet",
					VNetResourceGroup:            "my-rg",
					PublicLBName:                 "capz-lb",
					PublicLBAddressPoolName:      "backendPool",
					AcceleratedNetworking:        nil,
					TerminateNotificationTimeout: to.IntPtr(7),
					SecurityProfile:              &infrav1.SecurityProfile{EncryptionAtHost: to.BoolPtr(true)},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AdditionalTags()
				s.Location().Return("test-location")
				s.ClusterName().Return("my-cluster")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").
					Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.GetVMImage().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher: "fake-publisher",
						Offer:     "my-offer",
						SKU:       "sku-id",
						Version:   "1.0",
					},
				}, nil)
				s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vmss", gomock.AssignableToTypeOf(compute.VirtualMachineScaleSet{})).Do(
					func(_, _, _ interface{}, vmss compute.VirtualMachineScaleSet) {
						encryptionAtHost := *vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.SecurityProfile.EncryptionAtHost
						g.Expect(encryptionAtHost).To(Equal(true))
					})
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").
					Return(compute.VirtualMachineScaleSet{
						ID:   to.StringPtr("vmss-id"),
						Name: to.StringPtr("my-vmss"),
						VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
							ProvisioningState: to.StringPtr("Succeeded"),
						},
					}, nil)
				m.ListInstances(gomockinternal.AContext(), "my-rg", "my-vmss").Return([]compute.VirtualMachineScaleSetVM{
					{
						InstanceID: to.StringPtr("id-2"),
						VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
							ProvisioningState: to.StringPtr("Succeeded"),
						},
						ID:   to.StringPtr("id-1"),
						Name: to.StringPtr("instance-0"),
					},
				}, nil)
				s.SaveK8sVersion()
				s.NeedsK8sVersionUpdate()
				s.UpdateInstanceStatuses(gomock.Any(), gomock.Len(1)).Return(nil)
				s.SetProviderID("azure://vmss-id")
				s.SetAnnotation("cluster-api-provider-azure", "true")
				s.SetProvisioningState(infrav1.VMStateSucceeded)
			},
		},
		{
			name:          "creating a vmss with encryption at host enabled for unsupported VM type fails",
			expectedError: "reconcile error occurred that cannot be recovered. Object will not be requeued. The actual error is: encryption at host is not supported for VM type VM_SIZE",
			expect: func(g *gomega.WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:            "my-vmss",
					Size:            "VM_SIZE",
					Capacity:        2,
					SSHKeyData:      "ZmFrZXNzaGtleQo=",
					SecurityProfile: &infrav1.SecurityProfile{EncryptionAtHost: to.BoolPtr(true)},
				})
				s.GetVMImage().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher: "fake-publisher",
						Offer:     "my-offer",
						SKU:       "sku-id",
						Version:   "1.0",
					},
				}, nil)
			},
		},
		{
			name:          "scale set already exists",
			expectedError: "",
			expect: func(g *gomega.WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       "my-vmss",
					Size:       "VM_SIZE_AN",
					Capacity:   1,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
					OSDisk: infrav1.OSDisk{
						OSType:     "Linux",
						DiskSizeGB: 120,
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "Premium_LRS",
						},
					},
					DataDisks: []infrav1.DataDisk{
						{
							NameSuffix: "my_disk",
							DiskSizeGB: 128,
							Lun:        to.Int32Ptr(0),
						},
					},
					SubnetName:              "my-subnet",
					VNetName:                "my-vnet",
					VNetResourceGroup:       "my-rg",
					PublicLBName:            "capz-lb",
					PublicLBAddressPoolName: "backendPool",
					AcceleratedNetworking:   to.BoolPtr(true),
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AdditionalTags()
				s.Location().Return("test-location")
				s.ClusterName().Return("my-cluster")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").
					Return(compute.VirtualMachineScaleSet{
						Name:     to.StringPtr("my-vmss"),
						ID:       to.StringPtr("vmss-id"),
						Location: to.StringPtr("test-location"),
						Tags: map[string]*string{
							"Name": to.StringPtr("my-vmss"),
							"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
							"sigs.k8s.io_cluster-api-provider-azure_role":               to.StringPtr("node"),
						},
						Sku: &compute.Sku{
							Name:     to.StringPtr("VM_SIZE_AN"),
							Tier:     to.StringPtr("Standard"),
							Capacity: to.Int64Ptr(1),
						},
						VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
							ProvisioningState: to.StringPtr("Succeeded"),
							UpgradePolicy: &compute.UpgradePolicy{
								Mode: compute.UpgradeModeManual,
							},
							VirtualMachineProfile: &compute.VirtualMachineScaleSetVMProfile{
								OsProfile: &compute.VirtualMachineScaleSetOSProfile{
									ComputerNamePrefix: to.StringPtr("my-vmss"),
									AdminUsername:      to.StringPtr(azure.DefaultUserName),
									CustomData:         to.StringPtr("fake-bootstrap-data"),
									LinuxConfiguration: &compute.LinuxConfiguration{
										SSH: &compute.SSHConfiguration{
											PublicKeys: &[]compute.SSHPublicKey{
												{
													Path:    to.StringPtr("/home/capi/.ssh/authorized_keys"),
													KeyData: to.StringPtr("fakesshkey\n"),
												},
											},
										},
										DisablePasswordAuthentication: to.BoolPtr(true),
									},
								},
								StorageProfile: &compute.VirtualMachineScaleSetStorageProfile{
									ImageReference: &compute.ImageReference{
										Publisher: to.StringPtr("fake-publisher"),
										Offer:     to.StringPtr("my-offer"),
										Sku:       to.StringPtr("sku-id"),
										Version:   to.StringPtr("1.0"),
									},
									OsDisk: &compute.VirtualMachineScaleSetOSDisk{
										OsType:       "Linux",
										CreateOption: "FromImage",
										DiskSizeGB:   to.Int32Ptr(120),
										ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
											StorageAccountType: "Premium_LRS",
										},
									},
									DataDisks: &[]compute.VirtualMachineScaleSetDataDisk{
										{
											Lun:          to.Int32Ptr(0),
											Name:         to.StringPtr("my-vmss_my_disk"),
											CreateOption: "Empty",
											DiskSizeGB:   to.Int32Ptr(128),
										},
									},
								},
								DiagnosticsProfile: &compute.DiagnosticsProfile{
									BootDiagnostics: &compute.BootDiagnostics{
										Enabled: to.BoolPtr(true),
									},
								},
								NetworkProfile: &compute.VirtualMachineScaleSetNetworkProfile{
									NetworkInterfaceConfigurations: &[]compute.VirtualMachineScaleSetNetworkConfiguration{
										{
											Name: to.StringPtr("my-vmss-netconfig"),
											VirtualMachineScaleSetNetworkConfigurationProperties: &compute.VirtualMachineScaleSetNetworkConfigurationProperties{
												Primary:                     to.BoolPtr(true),
												EnableAcceleratedNetworking: to.BoolPtr(true),
												EnableIPForwarding:          to.BoolPtr(true),
												IPConfigurations: &[]compute.VirtualMachineScaleSetIPConfiguration{
													{
														Name: to.StringPtr("my-vmss-ipconfig"),
														VirtualMachineScaleSetIPConfigurationProperties: &compute.VirtualMachineScaleSetIPConfigurationProperties{
															Subnet: &compute.APIEntityReference{
																ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet"),
															},
															Primary:                         to.BoolPtr(true),
															PrivateIPAddressVersion:         compute.IPv4,
															LoadBalancerBackendAddressPools: &[]compute.SubResource{{ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/capz-lb/backendAddressPools/backendPool")}},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					}, nil)
				m.ListInstances(gomockinternal.AContext(), "my-rg", "my-vmss").Return([]compute.VirtualMachineScaleSetVM{
					{
						InstanceID: to.StringPtr("id-2"),
						VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
							ProvisioningState: to.StringPtr("Succeeded"),
						},
						ID:   to.StringPtr("id-1"),
						Name: to.StringPtr("instance-0"),
					},
				}, nil)
				s.GetVMImage().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher: "fake-publisher",
						Offer:     "my-offer",
						SKU:       "sku-id",
						Version:   "1.0",
					},
				}, nil)
				s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
				m.Update(gomockinternal.AContext(), "my-rg", "my-vmss", gomockinternal.DiffEq(compute.VirtualMachineScaleSetUpdate{
					Tags: map[string]*string{
						"Name": to.StringPtr("my-vmss"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"sigs.k8s.io_cluster-api-provider-azure_role":               to.StringPtr("node"),
					},
					Sku: &compute.Sku{
						Name:     to.StringPtr("VM_SIZE_AN"),
						Tier:     to.StringPtr("Standard"),
						Capacity: to.Int64Ptr(1),
					},
					VirtualMachineScaleSetUpdateProperties: &compute.VirtualMachineScaleSetUpdateProperties{
						UpgradePolicy: &compute.UpgradePolicy{
							Mode: compute.UpgradeModeManual,
						},
						VirtualMachineProfile: &compute.VirtualMachineScaleSetUpdateVMProfile{
							OsProfile: &compute.VirtualMachineScaleSetUpdateOSProfile{
								CustomData: to.StringPtr("fake-bootstrap-data"),
								LinuxConfiguration: &compute.LinuxConfiguration{
									SSH: &compute.SSHConfiguration{
										PublicKeys: &[]compute.SSHPublicKey{
											{
												Path:    to.StringPtr("/home/capi/.ssh/authorized_keys"),
												KeyData: to.StringPtr("fakesshkey\n"),
											},
										},
									},
									DisablePasswordAuthentication: to.BoolPtr(true),
								},
							},
							DiagnosticsProfile: &compute.DiagnosticsProfile{
								BootDiagnostics: &compute.BootDiagnostics{
									Enabled: to.BoolPtr(true),
								},
							},
							StorageProfile: &compute.VirtualMachineScaleSetUpdateStorageProfile{
								ImageReference: &compute.ImageReference{
									Publisher: to.StringPtr("fake-publisher"),
									Offer:     to.StringPtr("my-offer"),
									Sku:       to.StringPtr("sku-id"),
									Version:   to.StringPtr("1.0"),
								},
								OsDisk: &compute.VirtualMachineScaleSetUpdateOSDisk{
									DiskSizeGB: to.Int32Ptr(120),
									ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
										StorageAccountType: "Premium_LRS",
									},
								},
								DataDisks: &[]compute.VirtualMachineScaleSetDataDisk{
									{
										Lun:          to.Int32Ptr(0),
										Name:         to.StringPtr("my-vmss_my_disk"),
										CreateOption: "Empty",
										DiskSizeGB:   to.Int32Ptr(128),
									},
								},
							},
						},
					},
				})).Return(nil)
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").
					Return(compute.VirtualMachineScaleSet{
						ID:   to.StringPtr("vmss-id"),
						Name: to.StringPtr("my-vmss"),
						VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
							ProvisioningState: to.StringPtr("Succeeded"),
						},
					}, nil)
				m.ListInstances(gomock.Any(), "my-rg", "my-vmss").Return([]compute.VirtualMachineScaleSetVM{
					{
						ID:         to.StringPtr("my-vm-id"),
						InstanceID: to.StringPtr("my-vm-1"),
						Name:       to.StringPtr("my-vm"),
						VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
							ProvisioningState: to.StringPtr("Succeeded"),
						},
					},
				}, nil)
				s.SetProviderID("azure://vmss-id")
				s.SetAnnotation("cluster-api-provider-azure", "true")
				s.SetProvisioningState(infrav1.VMStateSucceeded)
				s.NeedsK8sVersionUpdate()
				s.UpdateInstanceStatuses(gomock.Any(), gomock.Len(1)).Return(nil)
			},
		},
		{
			name:          "less than 2 vCPUs",
			expectedError: "vm size should be bigger or equal to at least 2 vCPUs",
			expect: func(g *gomega.WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       "my-vmss",
					Size:       "VM_SIZE_1_CPU",
					Capacity:   2,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
				})
			},
		},
		{
			name:          "Memory is less than 2Gi",
			expectedError: "vm memory should be bigger or equal to at least 2Gi",
			expect: func(g *gomega.WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       "my-vmss",
					Size:       "VM_SIZE_1_MEM",
					Capacity:   2,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
				})
			},
		},
		{
			name:          "failed to get SKU",
			expectedError: "failed to get find SKU INVALID_VM_SIZE in compute api: resource sku with name 'INVALID_VM_SIZE' and category 'virtualMachines' not found",
			expect: func(g *gomega.WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       "my-vmss",
					Size:       "INVALID_VM_SIZE",
					Capacity:   2,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
				})
			},
		},
		{
			name:          "fails with internal error",
			expectedError: "cannot create VMSS: #: Internal error: StatusCode=500",
			expect: func(g *gomega.WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       "my-vmss",
					Size:       "VM_SIZE",
					Capacity:   2,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
					OSDisk: infrav1.OSDisk{
						OSType:     "Linux",
						DiskSizeGB: 120,
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "Premium_LRS",
						},
					},
					DataDisks: []infrav1.DataDisk{
						{
							NameSuffix: "my_disk",
							DiskSizeGB: 128,
							Lun:        to.Int32Ptr(0),
						},
					},
					SubnetName:              "my-subnet",
					VNetName:                "my-vnet",
					VNetResourceGroup:       "my-rg",
					PublicLBName:            "capz-lb",
					PublicLBAddressPoolName: "backendPool",
					AcceleratedNetworking:   nil,
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AdditionalTags()
				s.Location().Return("test-location")
				s.ClusterName().Return("my-cluster")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").
					Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.GetVMImage().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher: "fake-publisher",
						Offer:     "my-offer",
						SKU:       "sku-id",
						Version:   "1.0",
					},
				}, nil)
				s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vmss", gomock.AssignableToTypeOf(compute.VirtualMachineScaleSet{})).
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal error"))
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

			scopeMock := mock_scalesets.NewMockScaleSetScope(mockCtrl)
			clientMock := mock_scalesets.NewMockClient(mockCtrl)

			tc.expect(g, scopeMock.EXPECT(), clientMock.EXPECT())

			s := &Service{
				Scope:            scopeMock,
				Client:           clientMock,
				resourceSKUCache: resourceskus.NewStaticCache(getFakeSkus()),
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

func TestDeleteVMSS(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder)
	}{
		{
			name:          "successfully delete an existing vmss",
			expectedError: "",
			expect: func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:     "my-existing-vmss",
					Size:     "VM_SIZE",
					Capacity: 3,
				})
				s.ResourceGroup().AnyTimes().Return("my-existing-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Delete(gomockinternal.AContext(), "my-existing-rg", "my-existing-vmss")
			},
		},
		{
			name:          "vmss already deleted",
			expectedError: "",
			expect: func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:     "my-vmss",
					Size:     "VM_SIZE",
					Capacity: 3,
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Delete(gomockinternal.AContext(), "my-rg", "my-vmss").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name:          "vmss deletion fails",
			expectedError: "failed to delete VMSS my-vmss in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:     "my-vmss",
					Size:     "VM_SIZE",
					Capacity: 3,
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Delete(gomockinternal.AContext(), "my-rg", "my-vmss").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
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
			scopeMock := mock_scalesets.NewMockScaleSetScope(mockCtrl)
			clientMock := mock_scalesets.NewMockClient(mockCtrl)

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

func getFakeSkus() []compute.ResourceSku {
	return []compute.ResourceSku{
		{
			Name: to.StringPtr("VM_SIZE"),
			Kind: to.StringPtr(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: to.StringPtr("test-location"),
					Zones:    &[]string{"1"},
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  to.StringPtr(resourceskus.AcceleratedNetworking),
					Value: to.StringPtr(string(resourceskus.CapabilityUnsupported)),
				},
				{
					Name:  to.StringPtr(resourceskus.VCPUs),
					Value: to.StringPtr("4"),
				},
				{
					Name:  to.StringPtr(resourceskus.MemoryGB),
					Value: to.StringPtr("4"),
				},
			},
		},
		{
			Name: to.StringPtr("VM_SIZE_AN"),
			Kind: to.StringPtr(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: to.StringPtr("test-location"),
					Zones:    &[]string{"1"},
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  to.StringPtr(resourceskus.AcceleratedNetworking),
					Value: to.StringPtr(string(resourceskus.CapabilitySupported)),
				},
				{
					Name:  to.StringPtr(resourceskus.VCPUs),
					Value: to.StringPtr("4"),
				},
				{
					Name:  to.StringPtr(resourceskus.MemoryGB),
					Value: to.StringPtr("6"),
				},
			},
		},
		{
			Name: to.StringPtr("VM_SIZE_1_CPU"),
			Kind: to.StringPtr(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: to.StringPtr("test-location"),
					Zones:    &[]string{"1"},
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  to.StringPtr(resourceskus.AcceleratedNetworking),
					Value: to.StringPtr(string(resourceskus.CapabilityUnsupported)),
				},
				{
					Name:  to.StringPtr(resourceskus.VCPUs),
					Value: to.StringPtr("1"),
				},
				{
					Name:  to.StringPtr(resourceskus.MemoryGB),
					Value: to.StringPtr("4"),
				},
			},
		},
		{
			Name: to.StringPtr("VM_SIZE_1_MEM"),
			Kind: to.StringPtr(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: to.StringPtr("test-location"),
					Zones:    &[]string{"1"},
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  to.StringPtr(resourceskus.AcceleratedNetworking),
					Value: to.StringPtr(string(resourceskus.CapabilityUnsupported)),
				},
				{
					Name:  to.StringPtr(resourceskus.VCPUs),
					Value: to.StringPtr("2"),
				},
				{
					Name:  to.StringPtr(resourceskus.MemoryGB),
					Value: to.StringPtr("1"),
				},
			},
		},
		{
			Name: to.StringPtr("VM_SIZE_EAH"),
			Kind: to.StringPtr(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: to.StringPtr("test-location"),
					Zones:    &[]string{"1"},
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  to.StringPtr(resourceskus.VCPUs),
					Value: to.StringPtr("4"),
				},
				{
					Name:  to.StringPtr(resourceskus.MemoryGB),
					Value: to.StringPtr("8"),
				},
				{
					Name:  to.StringPtr(resourceskus.EncryptionAtHost),
					Value: to.StringPtr(string(resourceskus.CapabilitySupported)),
				},
			},
		},
	}
}

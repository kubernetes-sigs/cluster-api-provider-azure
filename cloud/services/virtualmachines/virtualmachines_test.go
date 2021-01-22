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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/klogr"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/availabilitysets/mock_availabilitysets"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/networkinterfaces/mock_networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips/mock_publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualmachines/mock_virtualmachines"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

func TestGetExistingVM(t *testing.T) {
	testcases := []struct {
		name          string
		vmName        string
		result        *infrav1.VM
		expectedError string
		expect        func(s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder)
	}{
		{
			name:   "get existing vm",
			vmName: "my-vm",
			result: &infrav1.VM{
				ID:       "my-id",
				Name:     "my-vm",
				State:    "Succeeded",
				Identity: "",
				Tags:     nil,
				Addresses: []corev1.NodeAddress{
					{
						Type:    "InternalIP",
						Address: "1.2.3.4",
					},
					{
						Type:    "ExternalIP",
						Address: "4.3.2.1",
					},
				},
			},
			expectedError: "",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				mpip.Get(gomockinternal.AContext(), "my-rg", "my-publicIP-id").Return(network.PublicIPAddress{
					PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
						PublicIPAddressVersion:   network.IPv4,
						PublicIPAllocationMethod: network.Static,
						IPAddress:                to.StringPtr("4.3.2.1"),
					},
				}, nil)
				mnic.Get(gomockinternal.AContext(), "my-rg", gomock.Any()).Return(network.Interface{
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
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").Return(compute.VirtualMachine{
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
			name:          "vm not found",
			vmName:        "my-vm",
			result:        &infrav1.VM{},
			expectedError: "#: Not found: StatusCode=404",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name:          "vm retrieval fails",
			vmName:        "my-vm",
			result:        &infrav1.VM{},
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
		},
		{
			name:          "get existing vm: error getting public IP",
			vmName:        "my-vm",
			result:        &infrav1.VM{},
			expectedError: "#: Internal Server Error: StatusCode=500",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				mpip.Get(gomockinternal.AContext(), "my-rg", "my-publicIP-id").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
				mnic.Get(gomockinternal.AContext(), "my-rg", gomock.Any()).Return(network.Interface{
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
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").Return(compute.VirtualMachine{
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
			name:          "get existing vm: public IP not found",
			vmName:        "my-vm",
			result:        &infrav1.VM{},
			expectedError: "#: Not Found: StatusCode=404",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				mpip.Get(gomockinternal.AContext(), "my-rg", "my-publicIP-id").Return(network.PublicIPAddress{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not Found"))
				mnic.Get(gomockinternal.AContext(), "my-rg", gomock.Any()).Return(network.Interface{
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
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").Return(compute.VirtualMachine{
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
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			scopeMock := mock_virtualmachines.NewMockVMScope(mockCtrl)
			clientMock := mock_virtualmachines.NewMockClient(mockCtrl)
			interfaceMock := mock_networkinterfaces.NewMockClient(mockCtrl)
			publicIPMock := mock_publicips.NewMockClient(mockCtrl)

			tc.expect(scopeMock.EXPECT(), clientMock.EXPECT(), interfaceMock.EXPECT(), publicIPMock.EXPECT())

			s := &Service{
				Scope:            scopeMock,
				Client:           clientMock,
				interfacesClient: interfaceMock,
				publicIPsClient:  publicIPMock,
			}

			result, err := s.getExisting(context.TODO(), tc.vmName)
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

func TestReconcileVM(t *testing.T) {
	testcases := []struct {
		Name          string
		Expect        func(g *WithT, s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder)
		ExpectedError string
		SetupSKUs     func(svc *Service)
	}{
		{
			Name: "can create a vm",
			Expect: func(g *WithT, s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder,
				mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.VMSpec().Return(azure.VMSpec{
					Name:       "my-vm",
					Role:       infrav1.ControlPlane,
					NICNames:   []string{"my-nic", "second-nic"},
					SSHKeyData: "ZmFrZXNzaGtleQo=",
					Size:       "Standard_D2v3",
					Zone:       "1",
					Identity:   infrav1.VMIdentityNone,
					OSDisk: infrav1.OSDisk{
						OSType:     "Linux",
						DiskSizeGB: 128,
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "Premium_LRS",
						},
					},
					DataDisks: []infrav1.DataDisk{
						{
							NameSuffix: "mydisk",
							DiskSizeGB: 64,
							Lun:        to.Int32Ptr(0),
						},
					},
					UserAssignedIdentities: nil,
					SpotVMOptions:          nil,
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AdditionalTags()
				s.Location().Return("test-location")
				s.ClusterName().Return("my-cluster")
				s.ProviderID().Return("")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").
					Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.GetVMImage().AnyTimes().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher: "fake-publisher",
						Offer:     "my-offer",
						SKU:       "sku-id",
						Version:   "1.0",
					},
				}, nil)
				s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
				s.AvailabilitySet().Return("", false)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vm", gomockinternal.DiffEq(compute.VirtualMachine{
					VirtualMachineProperties: &compute.VirtualMachineProperties{
						HardwareProfile: &compute.HardwareProfile{VMSize: "Standard_D2v3"},
						StorageProfile: &compute.StorageProfile{
							ImageReference: &compute.ImageReference{
								Publisher: to.StringPtr("fake-publisher"),
								Offer:     to.StringPtr("my-offer"),
								Sku:       to.StringPtr("sku-id"),
								Version:   to.StringPtr("1.0"),
							},
							OsDisk: &compute.OSDisk{
								OsType:       "Linux",
								Name:         to.StringPtr("my-vm_OSDisk"),
								CreateOption: "FromImage",
								DiskSizeGB:   to.Int32Ptr(128),
								ManagedDisk: &compute.ManagedDiskParameters{
									StorageAccountType: "Premium_LRS",
								},
							},
							DataDisks: &[]compute.DataDisk{
								{
									Lun:          to.Int32Ptr(0),
									Name:         to.StringPtr("my-vm_mydisk"),
									CreateOption: "Empty",
									DiskSizeGB:   to.Int32Ptr(64),
								},
							},
						},
						OsProfile: &compute.OSProfile{
							ComputerName:  to.StringPtr("my-vm"),
							AdminUsername: to.StringPtr("capi"),
							CustomData:    to.StringPtr("fake-bootstrap-data"),
							LinuxConfiguration: &compute.LinuxConfiguration{
								DisablePasswordAuthentication: to.BoolPtr(true),
								SSH: &compute.SSHConfiguration{
									PublicKeys: &[]compute.SSHPublicKey{
										{
											Path:    to.StringPtr("/home/capi/.ssh/authorized_keys"),
											KeyData: to.StringPtr("fakesshkey\n"),
										},
									},
								},
							},
						},
						DiagnosticsProfile: &compute.DiagnosticsProfile{
							BootDiagnostics: &compute.BootDiagnostics{
								Enabled: to.BoolPtr(true),
							},
						},
						NetworkProfile: &compute.NetworkProfile{
							NetworkInterfaces: &[]compute.NetworkInterfaceReference{
								{
									NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{Primary: to.BoolPtr(true)},
									ID:                                  to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/networkInterfaces/my-nic"),
								},
								{
									NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{Primary: to.BoolPtr(false)},
									ID:                                  to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/networkInterfaces/second-nic"),
								},
							},
						},
					},
					Resources: nil,
					Identity:  nil,
					Zones:     &[]string{"1"},
					ID:        nil,
					Name:      nil,
					Type:      nil,
					Location:  to.StringPtr("test-location"),
					Tags: map[string]*string{
						"Name": to.StringPtr("my-vm"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"sigs.k8s.io_cluster-api-provider-azure_role":               to.StringPtr("control-plane"),
					},
				}))
			},
			ExpectedError: "",
			SetupSKUs: func(svc *Service) {
				skus := []compute.ResourceSku{
					{
						Name: to.StringPtr("Standard_D2v3"),
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
								Value: to.StringPtr("2"),
							},
							{
								Name:  to.StringPtr(resourceskus.MemoryGB),
								Value: to.StringPtr("4"),
							},
						},
					},
				}
				resourceSkusCache := resourceskus.NewStaticCache(skus)
				svc.resourceSKUCache = resourceSkusCache

			},
		},
		{
			Name: "can create a vm with system assigned identity",
			Expect: func(g *WithT, s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.VMSpec().Return(azure.VMSpec{
					Name:                   "my-vm",
					Role:                   infrav1.Node,
					NICNames:               []string{"my-nic"},
					SSHKeyData:             "fakesshpublickey",
					Size:                   "Standard_D2v3",
					Zone:                   "1",
					Identity:               infrav1.VMIdentitySystemAssigned,
					OSDisk:                 infrav1.OSDisk{},
					DataDisks:              nil,
					UserAssignedIdentities: nil,
					SpotVMOptions:          nil,
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AdditionalTags()
				s.Location().Return("test-location")
				s.ClusterName().Return("my-cluster")
				s.ProviderID().Return("")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").
					Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.GetVMImage().AnyTimes().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher: "fake-publisher",
						Offer:     "my-offer",
						SKU:       "sku-id",
						Version:   "1.0",
					},
				}, nil)
				s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
				s.AvailabilitySet().Return("", false)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vm", gomock.AssignableToTypeOf(compute.VirtualMachine{})).Do(func(_, _, _ interface{}, vm compute.VirtualMachine) {
					g.Expect(vm.Identity.Type).To(Equal(compute.ResourceIdentityTypeSystemAssigned))
					g.Expect(vm.Identity.UserAssignedIdentities).To(HaveLen(0))
				})
			},
			ExpectedError: "",
			SetupSKUs: func(svc *Service) {
				skus := []compute.ResourceSku{
					{
						Name: to.StringPtr("Standard_D2v3"),
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
								Value: to.StringPtr("2"),
							},
							{
								Name:  to.StringPtr(resourceskus.MemoryGB),
								Value: to.StringPtr("4"),
							},
						},
					},
				}
				resourceSkusCache := resourceskus.NewStaticCache(skus)
				svc.resourceSKUCache = resourceSkusCache

			},
		},
		{
			Name: "can create a vm with user assigned identity",
			Expect: func(g *WithT, s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.VMSpec().Return(azure.VMSpec{
					Name:                   "my-vm",
					Role:                   infrav1.Node,
					NICNames:               []string{"my-nic"},
					SSHKeyData:             "fakesshpublickey",
					Size:                   "Standard_D2v3",
					Zone:                   "1",
					Identity:               infrav1.VMIdentityUserAssigned,
					OSDisk:                 infrav1.OSDisk{},
					DataDisks:              nil,
					UserAssignedIdentities: []infrav1.UserAssignedIdentity{{ProviderID: "my-user-id"}},
					SpotVMOptions:          nil,
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AdditionalTags()
				s.Location().Return("test-location")
				s.ClusterName().Return("my-cluster")
				s.ProviderID().Return("")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").
					Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.GetVMImage().AnyTimes().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher: "fake-publisher",
						Offer:     "my-offer",
						SKU:       "sku-id",
						Version:   "1.0",
					},
				}, nil)
				s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
				s.AvailabilitySet().Return("", false)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vm", gomock.AssignableToTypeOf(compute.VirtualMachine{})).Do(func(_, _, _ interface{}, vm compute.VirtualMachine) {
					g.Expect(vm.Identity.Type).To(Equal(compute.ResourceIdentityTypeUserAssigned))
					g.Expect(vm.Identity.UserAssignedIdentities).To(Equal(map[string]*compute.VirtualMachineIdentityUserAssignedIdentitiesValue{"my-user-id": {}}))
				})
			},
			ExpectedError: "",
			SetupSKUs: func(svc *Service) {
				skus := []compute.ResourceSku{
					{
						Name: to.StringPtr("Standard_D2v3"),
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
								Value: to.StringPtr("2"),
							},
							{
								Name:  to.StringPtr(resourceskus.MemoryGB),
								Value: to.StringPtr("4"),
							},
						},
					},
				}
				resourceSkusCache := resourceskus.NewStaticCache(skus)
				svc.resourceSKUCache = resourceSkusCache

			},
		},
		{
			Name: "can create a spot vm",
			Expect: func(g *WithT, s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.VMSpec().Return(azure.VMSpec{
					Name:                   "my-vm",
					Role:                   infrav1.Node,
					NICNames:               []string{"my-nic"},
					SSHKeyData:             "fakesshpublickey",
					Size:                   "Standard_D2v3",
					Zone:                   "1",
					Identity:               "",
					OSDisk:                 infrav1.OSDisk{},
					DataDisks:              nil,
					UserAssignedIdentities: nil,
					SpotVMOptions:          &infrav1.SpotVMOptions{},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AdditionalTags()
				s.Location().Return("test-location")
				s.ClusterName().Return("my-cluster")
				s.ProviderID().Return("")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").
					Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.GetVMImage().AnyTimes().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher: "fake-publisher",
						Offer:     "my-offer",
						SKU:       "sku-id",
						Version:   "1.0",
					},
				}, nil)
				s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
				s.AvailabilitySet().Return("", false)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vm", gomock.AssignableToTypeOf(compute.VirtualMachine{})).Do(func(_, _, _ interface{}, vm compute.VirtualMachine) {
					g.Expect(vm.Priority).To(Equal(compute.Spot))
					g.Expect(vm.EvictionPolicy).To(Equal(compute.Deallocate))
					g.Expect(vm.BillingProfile).To(BeNil())
				})
			},
			ExpectedError: "",
			SetupSKUs: func(svc *Service) {
				skus := []compute.ResourceSku{
					{
						Name: to.StringPtr("Standard_D2v3"),
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
								Value: to.StringPtr("2"),
							},
							{
								Name:  to.StringPtr(resourceskus.MemoryGB),
								Value: to.StringPtr("4"),
							},
						},
					},
				}
				resourceSkusCache := resourceskus.NewStaticCache(skus)
				svc.resourceSKUCache = resourceSkusCache

			},
		},
		{
			Name: "can create a windows vm",
			Expect: func(g *WithT, s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.VMSpec().Return(azure.VMSpec{

					Name:       "my-vm",
					Role:       infrav1.ControlPlane,
					NICNames:   []string{"my-nic", "second-nic"},
					SSHKeyData: "ZmFrZXNzaGtleQo=",
					Size:       "Standard_D2v3",
					Zone:       "1",
					Identity:   infrav1.VMIdentityNone,
					OSDisk: infrav1.OSDisk{
						OSType:     "Windows",
						DiskSizeGB: 128,
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "Premium_LRS",
						},
					},
					DataDisks: []infrav1.DataDisk{
						{
							NameSuffix: "mydisk",
							DiskSizeGB: 64,
							Lun:        to.Int32Ptr(0),
						},
					},
					UserAssignedIdentities: nil,
					SpotVMOptions:          nil,
				},
				)
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AdditionalTags()
				s.Location().Return("test-location")
				s.ClusterName().Return("my-cluster")
				s.ProviderID().Return("")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").
					Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.GetVMImage().AnyTimes().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher: "fake-publisher",
						Offer:     "my-offer",
						SKU:       "sku-id",
						Version:   "1.0",
					},
				}, nil)
				s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
				s.AvailabilitySet().Return("", false)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vm", gomock.AssignableToTypeOf(compute.VirtualMachine{})).Do(func(_, _, _ interface{}, vm compute.VirtualMachine) {
					g.Expect(vm.VirtualMachineProperties.StorageProfile.OsDisk.OsType).To(Equal(compute.Windows))
					g.Expect(*vm.VirtualMachineProperties.OsProfile.AdminPassword).Should(HaveLen(123))
					g.Expect(*vm.VirtualMachineProperties.OsProfile.AdminUsername).Should(Equal("capi"))
					g.Expect(*vm.VirtualMachineProperties.OsProfile.WindowsConfiguration.EnableAutomaticUpdates).Should(Equal(false))

				})
			},
			ExpectedError: "",
			SetupSKUs: func(svc *Service) {
				skus := []compute.ResourceSku{
					{
						Name: to.StringPtr("Standard_D2v3"),
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
								Value: to.StringPtr("2"),
							},
							{
								Name:  to.StringPtr(resourceskus.MemoryGB),
								Value: to.StringPtr("4"),
							},
						},
					},
				}
				resourceSkusCache := resourceskus.NewStaticCache(skus)
				svc.resourceSKUCache = resourceSkusCache
			},
		},
		{
			Name: "can create a vm with encryption",
			Expect: func(g *WithT, s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.VMSpec().Return(azure.VMSpec{
					Name:       "my-vm",
					Role:       infrav1.Node,
					NICNames:   []string{"my-nic"},
					SSHKeyData: "fakesshpublickey",
					Size:       "Standard_D2v3",
					Zone:       "1",
					Identity:   "",
					OSDisk: infrav1.OSDisk{
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "Premium_LRS",
							DiskEncryptionSet: &infrav1.DiskEncryptionSetParameters{
								ID: "my-diskencryptionset-id",
							},
						},
					},
					DataDisks:              nil,
					UserAssignedIdentities: nil,
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AdditionalTags()
				s.Location().Return("test-location")
				s.ClusterName().Return("my-cluster")
				s.ProviderID().Return("")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").
					Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.GetVMImage().AnyTimes().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher: "fake-publisher",
						Offer:     "my-offer",
						SKU:       "sku-id",
						Version:   "1.0",
					},
				}, nil)
				s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
				s.AvailabilitySet().Return("", false)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vm", gomock.AssignableToTypeOf(compute.VirtualMachine{})).Do(func(_, _, _ interface{}, vm compute.VirtualMachine) {
					g.Expect(vm.VirtualMachineProperties.StorageProfile.OsDisk.ManagedDisk.DiskEncryptionSet.ID).To(Equal(to.StringPtr("my-diskencryptionset-id")))

				})
			},
			ExpectedError: "",
			SetupSKUs: func(svc *Service) {
				skus := []compute.ResourceSku{
					{
						Name: to.StringPtr("Standard_D2v3"),
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
								Value: to.StringPtr("2"),
							},
							{
								Name:  to.StringPtr(resourceskus.MemoryGB),
								Value: to.StringPtr("4"),
							},
						},
					},
				}
				resourceSkusCache := resourceskus.NewStaticCache(skus)
				svc.resourceSKUCache = resourceSkusCache

			},
		},
		{
			Name: "can create a vm with encryption at host",
			Expect: func(g *WithT, s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.VMSpec().Return(azure.VMSpec{
					Name:            "my-vm",
					Role:            infrav1.Node,
					NICNames:        []string{"my-nic"},
					SSHKeyData:      "fakesshpublickey",
					Size:            "Standard_D2v3",
					Zone:            "1",
					OSDisk:          infrav1.OSDisk{},
					SecurityProfile: &infrav1.SecurityProfile{EncryptionAtHost: to.BoolPtr(true)},
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AdditionalTags()
				s.Location().Return("test-location")
				s.ClusterName().Return("my-cluster")
				s.ProviderID().Return("")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").
					Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.GetVMImage().AnyTimes().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher: "fake-publisher",
						Offer:     "my-offer",
						SKU:       "sku-id",
						Version:   "1.0",
					},
				}, nil)
				s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
				s.AvailabilitySet().Return("", false)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vm", gomock.AssignableToTypeOf(compute.VirtualMachine{})).Do(func(_, _, _ interface{}, vm compute.VirtualMachine) {
					g.Expect(*vm.VirtualMachineProperties.SecurityProfile.EncryptionAtHost).To(Equal(true))

				})
			},
			ExpectedError: "",
			SetupSKUs: func(svc *Service) {
				skus := []compute.ResourceSku{
					{
						Name: to.StringPtr("Standard_D2v3"),
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
								Value: to.StringPtr("2"),
							},
							{
								Name:  to.StringPtr(resourceskus.MemoryGB),
								Value: to.StringPtr("4"),
							},
							{
								Name:  to.StringPtr(resourceskus.EncryptionAtHost),
								Value: to.StringPtr(string(resourceskus.CapabilitySupported)),
							},
						},
					},
				}

				svc.resourceSKUCache = resourceskus.NewStaticCache(skus)

			},
		},
		{
			Name: "can create a vm and assign it to an availability set",
			Expect: func(g *WithT, s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.VMSpec().Return(azure.VMSpec{
					Name:       "my-vm",
					Role:       infrav1.ControlPlane,
					NICNames:   []string{"my-nic", "second-nic"},
					SSHKeyData: "ZmFrZXNzaGtleQo=",
					Size:       "Standard_D2v3",
					Zone:       "",
					Identity:   infrav1.VMIdentityNone,
					OSDisk: infrav1.OSDisk{
						OSType:     "Linux",
						DiskSizeGB: 128,
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "Premium_LRS",
						},
					},
					DataDisks: []infrav1.DataDisk{
						{
							NameSuffix: "mydisk",
							DiskSizeGB: 64,
							Lun:        to.Int32Ptr(0),
						},
					},
					UserAssignedIdentities: nil,
					SpotVMOptions:          nil,
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AdditionalTags()
				s.Location().Return("test-location")
				s.ClusterName().Return("my-cluster")
				s.ProviderID().Return("")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").
					Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.GetVMImage().AnyTimes().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher: "fake-publisher",
						Offer:     "my-offer",
						SKU:       "sku-id",
						Version:   "1.0",
					},
				}, nil)
				s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
				s.AvailabilitySet().Return("as-name", true)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vm", gomockinternal.DiffEq(compute.VirtualMachine{
					VirtualMachineProperties: &compute.VirtualMachineProperties{
						HardwareProfile: &compute.HardwareProfile{VMSize: "Standard_D2v3"},
						StorageProfile: &compute.StorageProfile{
							ImageReference: &compute.ImageReference{
								Publisher: to.StringPtr("fake-publisher"),
								Offer:     to.StringPtr("my-offer"),
								Sku:       to.StringPtr("sku-id"),
								Version:   to.StringPtr("1.0"),
							},
							OsDisk: &compute.OSDisk{
								OsType:       "Linux",
								Name:         to.StringPtr("my-vm_OSDisk"),
								CreateOption: "FromImage",
								DiskSizeGB:   to.Int32Ptr(128),
								ManagedDisk: &compute.ManagedDiskParameters{
									StorageAccountType: "Premium_LRS",
								},
							},
							DataDisks: &[]compute.DataDisk{
								{
									Lun:          to.Int32Ptr(0),
									Name:         to.StringPtr("my-vm_mydisk"),
									CreateOption: "Empty",
									DiskSizeGB:   to.Int32Ptr(64),
								},
							},
						},
						OsProfile: &compute.OSProfile{
							ComputerName:  to.StringPtr("my-vm"),
							AdminUsername: to.StringPtr("capi"),
							CustomData:    to.StringPtr("fake-bootstrap-data"),
							LinuxConfiguration: &compute.LinuxConfiguration{
								DisablePasswordAuthentication: to.BoolPtr(true),
								SSH: &compute.SSHConfiguration{
									PublicKeys: &[]compute.SSHPublicKey{
										{
											Path:    to.StringPtr("/home/capi/.ssh/authorized_keys"),
											KeyData: to.StringPtr("fakesshkey\n"),
										},
									},
								},
							},
						},
						DiagnosticsProfile: &compute.DiagnosticsProfile{
							BootDiagnostics: &compute.BootDiagnostics{
								Enabled: to.BoolPtr(true),
							},
						},
						AvailabilitySet: &compute.SubResource{
							ID: to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Compute/availabilitySets/as-name"),
						},
						NetworkProfile: &compute.NetworkProfile{
							NetworkInterfaces: &[]compute.NetworkInterfaceReference{
								{
									NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{Primary: to.BoolPtr(true)},
									ID:                                  to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/networkInterfaces/my-nic"),
								},
								{
									NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{Primary: to.BoolPtr(false)},
									ID:                                  to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/networkInterfaces/second-nic"),
								},
							},
						},
					},
					Resources: nil,
					Identity:  nil,
					ID:        nil,
					Name:      nil,
					Type:      nil,
					Location:  to.StringPtr("test-location"),
					Tags: map[string]*string{
						"Name": to.StringPtr("my-vm"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"sigs.k8s.io_cluster-api-provider-azure_role":               to.StringPtr("control-plane"),
					},
				}))
			},
			ExpectedError: "",
			SetupSKUs: func(svc *Service) {
				skus := []compute.ResourceSku{
					{
						Name: to.StringPtr("Standard_D2v3"),
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
								Value: to.StringPtr("2"),
							},
							{
								Name:  to.StringPtr(resourceskus.MemoryGB),
								Value: to.StringPtr("4"),
							},
						},
					},
				}
				resourceSkusCache := resourceskus.NewStaticCache(skus)
				svc.resourceSKUCache = resourceSkusCache

			},
		},
		{
			Name: "creating a vm with encryption at host enabled for unsupported VM type fails",
			Expect: func(g *WithT, s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.VMSpec().Return(azure.VMSpec{
					Name:            "my-vm",
					Role:            infrav1.Node,
					NICNames:        []string{"my-nic"},
					SSHKeyData:      "fakesshpublickey",
					Size:            "Standard_D2v3",
					OSDisk:          infrav1.OSDisk{},
					SecurityProfile: &infrav1.SecurityProfile{EncryptionAtHost: to.BoolPtr(true)},
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.GetVMImage().AnyTimes().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher: "fake-publisher",
						Offer:     "my-offer",
						SKU:       "sku-id",
						Version:   "1.0",
					},
				}, nil)
				s.ProviderID().Return("")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").
					Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
			ExpectedError: "reconcile error occurred that cannot be recovered. Object will not be requeued. The actual error is: encryption at host is not supported for VM type Standard_D2v3",
			SetupSKUs: func(svc *Service) {
				skus := []compute.ResourceSku{
					{
						Name: to.StringPtr("Standard_D2v3"),
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
								Value: to.StringPtr("2"),
							},
							{
								Name:  to.StringPtr(resourceskus.MemoryGB),
								Value: to.StringPtr("4"),
							},
						},
					},
				}

				svc.resourceSKUCache = resourceskus.NewStaticCache(skus)
			},
		},
		{
			Name: "vm creation fails",
			Expect: func(g *WithT, s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.VMSpec().Return(azure.VMSpec{
					Name:                   "my-vm",
					Role:                   infrav1.ControlPlane,
					NICNames:               []string{"my-nic"},
					SSHKeyData:             "fakesshpublickey",
					Size:                   "Standard_D2v3",
					Zone:                   "1",
					Identity:               "",
					OSDisk:                 infrav1.OSDisk{},
					DataDisks:              nil,
					UserAssignedIdentities: nil,
					SpotVMOptions:          nil,
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AdditionalTags()
				s.Location().Return("test-location")
				s.ClusterName().Return("my-cluster")
				s.ProviderID().Return("")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").
					Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.GetVMImage().AnyTimes().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher: "fake-publisher",
						Offer:     "my-offer",
						SKU:       "sku-id",
						Version:   "1.0",
					},
				}, nil)
				s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
				s.AvailabilitySet().Return("", false)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vm", gomock.AssignableToTypeOf(compute.VirtualMachine{})).Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 500}, "Internal Server Error"))
			},
			ExpectedError: "failed to create VM my-vm in resource group my-rg: #: Internal Server Error: StatusCode=500",
			SetupSKUs: func(svc *Service) {
				skus := []compute.ResourceSku{
					{
						Name: to.StringPtr("Standard_D2v3"),
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
								Value: to.StringPtr("2"),
							},
							{
								Name:  to.StringPtr(resourceskus.MemoryGB),
								Value: to.StringPtr("4"),
							},
						},
					},
				}
				resourceSkusCache := resourceskus.NewStaticCache(skus)
				svc.resourceSKUCache = resourceSkusCache

			},
		},
		{
			Name: "cannot create vm if vCPU is less than 2",
			Expect: func(g *WithT, s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.VMSpec().Return(azure.VMSpec{
					Name:       "my-vm",
					Role:       infrav1.ControlPlane,
					NICNames:   []string{"my-nic", "second-nic"},
					SSHKeyData: "ZmFrZXNzaGtleQo=",
					Size:       "Standard_D1v3",
					Zone:       "1",
					Identity:   infrav1.VMIdentityNone,
					OSDisk: infrav1.OSDisk{
						OSType:     "Linux",
						DiskSizeGB: 128,
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "Premium_LRS",
						},
					},
					DataDisks: []infrav1.DataDisk{
						{
							NameSuffix: "mydisk",
							DiskSizeGB: 64,
							Lun:        to.Int32Ptr(0),
						},
					},
					UserAssignedIdentities: nil,
					SpotVMOptions:          nil,
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ProviderID().Return("")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").
					Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
			ExpectedError: "vm size should be bigger or equal to at least 2 vCPUs",
			SetupSKUs: func(svc *Service) {
				skus := []compute.ResourceSku{
					{
						Name: to.StringPtr("Standard_D1v3"),
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
								Value: to.StringPtr("1"),
							},
							{
								Name:  to.StringPtr(resourceskus.MemoryGB),
								Value: to.StringPtr("4"),
							},
						},
					},
				}
				resourceSkusCache := resourceskus.NewStaticCache(skus)
				svc.resourceSKUCache = resourceSkusCache

			},
		},
		{
			Name: "cannot create vm if memory is less than 2Gi",
			Expect: func(g *WithT, s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.VMSpec().Return(azure.VMSpec{
					Name:       "my-vm",
					Role:       infrav1.ControlPlane,
					NICNames:   []string{"my-nic", "second-nic"},
					SSHKeyData: "ZmFrZXNzaGtleQo=",
					Size:       "Standard_D2v3",
					Zone:       "1",
					Identity:   infrav1.VMIdentityNone,
					OSDisk: infrav1.OSDisk{
						OSType:     "Linux",
						DiskSizeGB: 128,
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "Premium_LRS",
						},
					},
					DataDisks: []infrav1.DataDisk{
						{
							NameSuffix: "mydisk",
							DiskSizeGB: 64,
							Lun:        to.Int32Ptr(0),
						},
					},
					UserAssignedIdentities: nil,
					SpotVMOptions:          nil,
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ProviderID().Return("")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").
					Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
			ExpectedError: "vm memory should be bigger or equal to at least 2Gi",
			SetupSKUs: func(svc *Service) {
				skus := []compute.ResourceSku{
					{
						Name: to.StringPtr("Standard_D2v3"),
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
								Value: to.StringPtr("2"),
							},
							{
								Name:  to.StringPtr(resourceskus.MemoryGB),
								Value: to.StringPtr("1"),
							},
						},
					},
				}
				resourceSkusCache := resourceskus.NewStaticCache(skus)
				svc.resourceSKUCache = resourceSkusCache

			},
		},
		{
			Name: "cannot create vm if does not support ephemeral os",
			Expect: func(g *WithT, s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.VMSpec().Return(azure.VMSpec{
					Name:       "my-vm",
					Role:       infrav1.ControlPlane,
					NICNames:   []string{"my-nic", "second-nic"},
					SSHKeyData: "ZmFrZXNzaGtleQo=",
					Size:       "Standard_D2v3",
					Zone:       "1",
					Identity:   infrav1.VMIdentityNone,
					OSDisk: infrav1.OSDisk{
						OSType:     "Linux",
						DiskSizeGB: 128,
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "Premium_LRS",
						},
						DiffDiskSettings: &infrav1.DiffDiskSettings{
							Option: string(compute.Local),
						},
					},
					DataDisks: []infrav1.DataDisk{
						{
							NameSuffix: "mydisk",
							DiskSizeGB: 64,
							Lun:        to.Int32Ptr(0),
						},
					},
					UserAssignedIdentities: nil,
					SpotVMOptions:          nil,
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ProviderID().Return("")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").
					Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
			ExpectedError: "vm size Standard_D2v3 does not support ephemeral os. select a different vm size or disable ephemeral os",
			SetupSKUs: func(svc *Service) {
				skus := []compute.ResourceSku{
					{
						Name: to.StringPtr("Standard_D2v3"),
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
								Name:  to.StringPtr(resourceskus.EphemeralOSDisk),
								Value: to.StringPtr("False"),
							},
							{
								Name:  to.StringPtr(resourceskus.VCPUs),
								Value: to.StringPtr("2"),
							},
							{
								Name:  to.StringPtr(resourceskus.MemoryGB),
								Value: to.StringPtr("4"),
							},
						},
					},
				}
				resourceSkusCache := resourceskus.NewStaticCache(skus)
				svc.resourceSKUCache = resourceSkusCache

			},
		},
		{
			Name: "can create a vm with EphemeralOSDisk",
			Expect: func(g *WithT, s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.VMSpec().Return(azure.VMSpec{
					Name:       "my-vm",
					Role:       infrav1.ControlPlane,
					NICNames:   []string{"my-nic", "second-nic"},
					SSHKeyData: "ZmFrZXNzaGtleQo=",
					Size:       "Standard_D2v3",
					Zone:       "1",
					Identity:   infrav1.VMIdentityNone,
					OSDisk: infrav1.OSDisk{
						OSType:     "Linux",
						DiskSizeGB: 128,
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "Premium_LRS",
						},
						DiffDiskSettings: &infrav1.DiffDiskSettings{
							Option: string(compute.Local),
						},
					},
					DataDisks: []infrav1.DataDisk{
						{
							NameSuffix: "mydisk",
							DiskSizeGB: 64,
							Lun:        to.Int32Ptr(0),
						},
					},
					UserAssignedIdentities: nil,
					SpotVMOptions:          nil,
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AdditionalTags()
				s.Location().Return("test-location")
				s.ClusterName().Return("my-cluster")
				s.ProviderID().Return("")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").
					Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.GetVMImage().AnyTimes().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher: "fake-publisher",
						Offer:     "my-offer",
						SKU:       "sku-id",
						Version:   "1.0",
					},
				}, nil)
				s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
				s.AvailabilitySet().Return("", false)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vm", gomockinternal.DiffEq(compute.VirtualMachine{
					VirtualMachineProperties: &compute.VirtualMachineProperties{
						HardwareProfile: &compute.HardwareProfile{VMSize: "Standard_D2v3"},
						StorageProfile: &compute.StorageProfile{
							ImageReference: &compute.ImageReference{
								Publisher: to.StringPtr("fake-publisher"),
								Offer:     to.StringPtr("my-offer"),
								Sku:       to.StringPtr("sku-id"),
								Version:   to.StringPtr("1.0"),
							},
							OsDisk: &compute.OSDisk{
								OsType:       "Linux",
								Name:         to.StringPtr("my-vm_OSDisk"),
								CreateOption: "FromImage",
								DiskSizeGB:   to.Int32Ptr(128),
								ManagedDisk: &compute.ManagedDiskParameters{
									StorageAccountType: "Premium_LRS",
								},
								DiffDiskSettings: &compute.DiffDiskSettings{
									Option: compute.Local,
								},
							},
							DataDisks: &[]compute.DataDisk{
								{
									Lun:          to.Int32Ptr(0),
									Name:         to.StringPtr("my-vm_mydisk"),
									CreateOption: "Empty",
									DiskSizeGB:   to.Int32Ptr(64),
								},
							},
						},
						DiagnosticsProfile: &compute.DiagnosticsProfile{
							BootDiagnostics: &compute.BootDiagnostics{
								Enabled: to.BoolPtr(true),
							},
						},
						OsProfile: &compute.OSProfile{
							ComputerName:  to.StringPtr("my-vm"),
							AdminUsername: to.StringPtr("capi"),
							CustomData:    to.StringPtr("fake-bootstrap-data"),
							LinuxConfiguration: &compute.LinuxConfiguration{
								DisablePasswordAuthentication: to.BoolPtr(true),
								SSH: &compute.SSHConfiguration{
									PublicKeys: &[]compute.SSHPublicKey{
										{
											Path:    to.StringPtr("/home/capi/.ssh/authorized_keys"),
											KeyData: to.StringPtr("fakesshkey\n"),
										},
									},
								},
							},
						},
						NetworkProfile: &compute.NetworkProfile{
							NetworkInterfaces: &[]compute.NetworkInterfaceReference{
								{
									NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{Primary: to.BoolPtr(true)},
									ID:                                  to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/networkInterfaces/my-nic"),
								},
								{
									NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{Primary: to.BoolPtr(false)},
									ID:                                  to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/networkInterfaces/second-nic"),
								},
							},
						},
					},
					Resources: nil,
					Identity:  nil,
					Zones:     &[]string{"1"},
					ID:        nil,
					Name:      nil,
					Type:      nil,
					Location:  to.StringPtr("test-location"),
					Tags: map[string]*string{
						"Name": to.StringPtr("my-vm"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"sigs.k8s.io_cluster-api-provider-azure_role":               to.StringPtr("control-plane"),
					},
				}))
			},
			ExpectedError: "",
			SetupSKUs: func(svc *Service) {
				skus := []compute.ResourceSku{
					{
						Name: to.StringPtr("Standard_D2v3"),
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
								Name:  to.StringPtr(resourceskus.EphemeralOSDisk),
								Value: to.StringPtr("True"),
							},
							{
								Name:  to.StringPtr(resourceskus.VCPUs),
								Value: to.StringPtr("2"),
							},
							{
								Name:  to.StringPtr(resourceskus.MemoryGB),
								Value: to.StringPtr("4"),
							},
						},
					},
				}
				resourceSkusCache := resourceskus.NewStaticCache(skus)
				svc.resourceSKUCache = resourceSkusCache

			},
		},
		{
			Name: "can create a vm with a marketplace image using a plan",
			Expect: func(g *WithT, s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.VMSpec().Return(azure.VMSpec{
					Name:       "my-vm",
					Role:       infrav1.ControlPlane,
					NICNames:   []string{"my-nic", "second-nic"},
					SSHKeyData: "ZmFrZXNzaGtleQo=",
					Size:       "Standard_D2v3",
					Zone:       "1",
					Identity:   infrav1.VMIdentityNone,
					OSDisk: infrav1.OSDisk{
						OSType:     "Linux",
						DiskSizeGB: 128,
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "Premium_LRS",
						},
					},
					DataDisks: []infrav1.DataDisk{
						{
							NameSuffix: "mydisk",
							DiskSizeGB: 64,
							Lun:        to.Int32Ptr(0),
						},
					},
					UserAssignedIdentities: nil,
					SpotVMOptions:          nil,
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.AdditionalTags()
				s.Location().Return("test-location")
				s.ClusterName().Return("my-cluster")
				s.ProviderID().Return("")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").
					Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
				s.GetVMImage().AnyTimes().Return(&infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher:       "fake-publisher",
						Offer:           "my-offer",
						SKU:             "sku-id",
						Version:         "1.0",
						ThirdPartyImage: true,
					},
				}, nil)
				s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
				s.AvailabilitySet().Return("", false)
				m.CreateOrUpdate(gomockinternal.AContext(), "my-rg", "my-vm", gomockinternal.DiffEq(compute.VirtualMachine{
					Plan: &compute.Plan{
						Name:      to.StringPtr("sku-id"),
						Publisher: to.StringPtr("fake-publisher"),
						Product:   to.StringPtr("my-offer"),
					},
					VirtualMachineProperties: &compute.VirtualMachineProperties{
						HardwareProfile: &compute.HardwareProfile{VMSize: "Standard_D2v3"},
						StorageProfile: &compute.StorageProfile{
							ImageReference: &compute.ImageReference{
								Publisher: to.StringPtr("fake-publisher"),
								Offer:     to.StringPtr("my-offer"),
								Sku:       to.StringPtr("sku-id"),
								Version:   to.StringPtr("1.0"),
							},
							OsDisk: &compute.OSDisk{
								OsType:       "Linux",
								Name:         to.StringPtr("my-vm_OSDisk"),
								CreateOption: "FromImage",
								DiskSizeGB:   to.Int32Ptr(128),
								ManagedDisk: &compute.ManagedDiskParameters{
									StorageAccountType: "Premium_LRS",
								},
							},
							DataDisks: &[]compute.DataDisk{
								{
									Lun:          to.Int32Ptr(0),
									Name:         to.StringPtr("my-vm_mydisk"),
									CreateOption: "Empty",
									DiskSizeGB:   to.Int32Ptr(64),
								},
							},
						},
						OsProfile: &compute.OSProfile{
							ComputerName:  to.StringPtr("my-vm"),
							AdminUsername: to.StringPtr("capi"),
							CustomData:    to.StringPtr("fake-bootstrap-data"),
							LinuxConfiguration: &compute.LinuxConfiguration{
								DisablePasswordAuthentication: to.BoolPtr(true),
								SSH: &compute.SSHConfiguration{
									PublicKeys: &[]compute.SSHPublicKey{
										{
											Path:    to.StringPtr("/home/capi/.ssh/authorized_keys"),
											KeyData: to.StringPtr("fakesshkey\n"),
										},
									},
								},
							},
						},
						DiagnosticsProfile: &compute.DiagnosticsProfile{
							BootDiagnostics: &compute.BootDiagnostics{
								Enabled: to.BoolPtr(true),
							},
						},
						NetworkProfile: &compute.NetworkProfile{
							NetworkInterfaces: &[]compute.NetworkInterfaceReference{
								{
									NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{Primary: to.BoolPtr(true)},
									ID:                                  to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/networkInterfaces/my-nic"),
								},
								{
									NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{Primary: to.BoolPtr(false)},
									ID:                                  to.StringPtr("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/networkInterfaces/second-nic"),
								},
							},
						},
					},
					Resources: nil,
					Identity:  nil,
					Zones:     &[]string{"1"},
					ID:        nil,
					Name:      nil,
					Type:      nil,
					Location:  to.StringPtr("test-location"),
					Tags: map[string]*string{
						"Name": to.StringPtr("my-vm"),
						"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": to.StringPtr("owned"),
						"sigs.k8s.io_cluster-api-provider-azure_role":               to.StringPtr("control-plane"),
					},
				}))
			},
			ExpectedError: "",
			SetupSKUs: func(svc *Service) {
				skus := []compute.ResourceSku{
					{
						Name: to.StringPtr("Standard_D2v3"),
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
								Value: to.StringPtr("2"),
							},
							{
								Name:  to.StringPtr(resourceskus.MemoryGB),
								Value: to.StringPtr("4"),
							},
						},
					},
				}
				resourceSkusCache := resourceskus.NewStaticCache(skus)
				svc.resourceSKUCache = resourceSkusCache
			},
		},
		{
			Name: "fails when there is a provider id present, but cannot find vm ",
			Expect: func(g *WithT, s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder, mnic *mock_networkinterfaces.MockClientMockRecorder, mpip *mock_publicips.MockClientMockRecorder) {
				s.VMSpec().Return(azure.VMSpec{
					Name: "my-vm",
				})
				s.SubscriptionID().AnyTimes().Return("123")
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				s.ProviderID().Times(2).Return("ExistingVM-ProviderID")
				s.SetVMState(infrav1.VMStateDeleted)
				m.Get(gomockinternal.AContext(), "my-rg", "my-vm").
					Return(compute.VirtualMachine{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
			ExpectedError: "VM with provider id \"ExistingVM-ProviderID\" has been deleted",
			SetupSKUs:     func(svc *Service) {},
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			scopeMock := mock_virtualmachines.NewMockVMScope(mockCtrl)
			clientMock := mock_virtualmachines.NewMockClient(mockCtrl)
			interfaceMock := mock_networkinterfaces.NewMockClient(mockCtrl)
			publicIPMock := mock_publicips.NewMockClient(mockCtrl)
			availabilitySetsMock := mock_availabilitysets.NewMockClient(mockCtrl)

			tc.Expect(g, scopeMock.EXPECT(), clientMock.EXPECT(), interfaceMock.EXPECT(), publicIPMock.EXPECT())

			s := &Service{
				Scope:                  scopeMock,
				Client:                 clientMock,
				interfacesClient:       interfaceMock,
				publicIPsClient:        publicIPMock,
				availabilitySetsClient: availabilitySetsMock,
				resourceSKUCache:       resourceskus.NewStaticCache(nil),
			}

			tc.SetupSKUs(s)

			err := s.Reconcile(context.TODO())
			if tc.ExpectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.ExpectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteVM(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder)
	}{
		{
			name:          "successfully delete an existing vm",
			expectedError: "",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder) {
				s.VMSpec().Return(azure.VMSpec{
					Name:                   "my-existing-vm",
					Role:                   infrav1.ControlPlane,
					NICNames:               []string{"my-nic"},
					SSHKeyData:             "fakesshpublickey",
					Size:                   "Standard_D2v3",
					Zone:                   "",
					Identity:               "",
					OSDisk:                 infrav1.OSDisk{},
					DataDisks:              nil,
					UserAssignedIdentities: nil,
					SpotVMOptions:          nil,
				})
				s.ResourceGroup().AnyTimes().Return("my-existing-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Delete(gomockinternal.AContext(), "my-existing-rg", "my-existing-vm")
			},
		},
		{
			name:          "vm already deleted",
			expectedError: "",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder) {
				s.VMSpec().Return(azure.VMSpec{
					Name:                   "my-vm",
					Role:                   infrav1.ControlPlane,
					NICNames:               []string{"my-nic"},
					SSHKeyData:             "fakesshpublickey",
					Size:                   "Standard_D2v3",
					Zone:                   "",
					Identity:               "",
					OSDisk:                 infrav1.OSDisk{},
					DataDisks:              nil,
					UserAssignedIdentities: nil,
					SpotVMOptions:          nil,
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Delete(gomockinternal.AContext(), "my-rg", "my-vm").
					Return(autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: 404}, "Not found"))
			},
		},
		{
			name:          "vm deletion fails",
			expectedError: "failed to delete VM my-vm in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_virtualmachines.MockVMScopeMockRecorder, m *mock_virtualmachines.MockClientMockRecorder) {
				s.VMSpec().Return(azure.VMSpec{
					Name:                   "my-vm",
					Role:                   infrav1.ControlPlane,
					NICNames:               []string{"my-nic"},
					SSHKeyData:             "fakesshpublickey",
					Size:                   "Standard_D2v3",
					Zone:                   "",
					Identity:               "",
					OSDisk:                 infrav1.OSDisk{},
					DataDisks:              nil,
					UserAssignedIdentities: nil,
					SpotVMOptions:          nil,
				})
				s.ResourceGroup().AnyTimes().Return("my-rg")
				s.V(gomock.AssignableToTypeOf(2)).AnyTimes().Return(klogr.New())
				m.Delete(gomockinternal.AContext(), "my-rg", "my-vm").
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
			scopeMock := mock_virtualmachines.NewMockVMScope(mockCtrl)
			clientMock := mock_virtualmachines.NewMockClient(mockCtrl)

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

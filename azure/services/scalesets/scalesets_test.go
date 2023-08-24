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

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/go-autorest/autorest"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/scalesets/mock_scalesets"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	defaultSubscriptionID = "123"
	defaultResourceGroup  = "my-rg"
	defaultVMSSName       = "my-vmss"
	vmSizeEPH             = "VM_SIZE_EPH"
)

func init() {
	_ = clusterv1.AddToScheme(scheme.Scheme)
}

func TestGetExistingVMSS(t *testing.T) {
	testcases := []struct {
		name          string
		vmssName      string
		result        *azure.VMSS
		expectedError string
		expect        func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder)
	}{
		{
			name:          "scale set not found",
			vmssName:      "my-vmss",
			result:        &azure.VMSS{},
			expectedError: "failed to get existing vmss: #: Not found: StatusCode=404",
			expect: func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusNotFound}, "Not found"))
			},
		},
		{
			name:     "get existing vmss",
			vmssName: "my-vmss",
			result: &azure.VMSS{
				ID:       "my-id",
				Name:     "my-vmss",
				State:    "Succeeded",
				Sku:      "Standard_D2",
				Identity: "",
				Tags:     nil,
				Capacity: int64(1),
				Zones:    []string{"1", "3"},
				Instances: []azure.VMSSVM{
					{
						ID:         "my-vm-id",
						InstanceID: "my-vm-1",
						Name:       "instance-000001",
						State:      "Succeeded",
					},
				},
			},
			expectedError: "",
			expect: func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").Return(compute.VirtualMachineScaleSet{
					ID:   ptr.To("my-id"),
					Name: ptr.To("my-vmss"),
					Sku: &compute.Sku{
						Capacity: ptr.To[int64](1),
						Name:     ptr.To("Standard_D2"),
					},
					VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
						SinglePlacementGroup: ptr.To(false),
						ProvisioningState:    ptr.To("Succeeded"),
					},
					Zones: &[]string{"1", "3"},
				}, nil)
				m.ListInstances(gomock.Any(), "my-rg", "my-vmss").Return([]compute.VirtualMachineScaleSetVM{
					{
						ID:         ptr.To("my-vm-id"),
						InstanceID: ptr.To("my-vm-1"),
						Name:       ptr.To("my-vm"),
						VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
							ProvisioningState: ptr.To("Succeeded"),
							OsProfile: &compute.OSProfile{
								ComputerName: ptr.To("instance-000001"),
							},
						},
					},
				}, nil)
			},
		},
		{
			name:          "list instances fails",
			vmssName:      "my-vmss",
			result:        &azure.VMSS{},
			expectedError: "failed to list instances: #: Not found: StatusCode=404",
			expect: func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ResourceGroup().AnyTimes().Return("my-rg")
				m.Get(gomockinternal.AContext(), "my-rg", "my-vmss").Return(compute.VirtualMachineScaleSet{
					ID:   ptr.To("my-id"),
					Name: ptr.To("my-vmss"),
					VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
						SinglePlacementGroup: ptr.To(false),
						ProvisioningState:    ptr.To("Succeeded"),
					},
				}, nil)
				m.ListInstances(gomockinternal.AContext(), "my-rg", "my-vmss").Return([]compute.VirtualMachineScaleSetVM{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusNotFound}, "Not found"))
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

			result, err := s.getVirtualMachineScaleSet(context.TODO(), tc.vmssName)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				t.Log(err.Error())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(BeEquivalentTo(tc.result))
			}
		})
	}
}

func TestReconcileVMSS(t *testing.T) {
	var (
		putFuture = &infrav1.Future{
			Type:          infrav1.PutFuture,
			ResourceGroup: defaultResourceGroup,
			Name:          defaultVMSSName,
		}

		patchFuture = &infrav1.Future{
			Type:          infrav1.PatchFuture,
			ResourceGroup: defaultResourceGroup,
			Name:          defaultVMSSName,
		}
	)

	testcases := []struct {
		name          string
		expect        func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder)
		expectedError string
	}{
		{
			name:          "should start creating a vmss",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PUT on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				defaultSpec := newDefaultVMSSSpec()
				defaultSpec.DataDisks = append(defaultSpec.DataDisks, infrav1.DataDisk{
					NameSuffix: "my_disk_with_ultra_disks",
					DiskSizeGB: 128,
					Lun:        ptr.To[int32](3),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "UltraSSD_LRS",
					},
				})
				s.ScaleSetSpec().Return(defaultSpec).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS("VM_SIZE")
				vmss.VirtualMachineScaleSetProperties.AdditionalCapabilities = &compute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}
				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(vmss)).
					Return(putFuture, nil)
				setupCreatingSucceededExpectations(s, m, newDefaultExistingVMSS("VM_SIZE"), putFuture)
			},
		},
		{
			name:          "should finish creating a vmss when long running operation is done",
			expectedError: "",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				defaultSpec := newDefaultVMSSSpec()
				s.ScaleSetSpec().Return(defaultSpec).AnyTimes()
				createdVMSS := newDefaultVMSS("VM_SIZE")
				instances := newDefaultInstances()

				setupDefaultVMSSInProgressOperationDoneExpectations(s, m, createdVMSS, instances)
				s.DeleteLongRunningOperationState(defaultSpec.Name, serviceName, infrav1.PutFuture)
				s.DeleteLongRunningOperationState(defaultSpec.Name, serviceName, infrav1.PatchFuture)
				s.UpdatePutStatus(infrav1.BootstrapSucceededCondition, serviceName, nil)
				s.HasReplicasExternallyManaged(gomockinternal.AContext()).Return(false)
			},
		},
		{
			name:          "Windows VMSS should not get patched",
			expectedError: "",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				defaultSpec := newWindowsVMSSSpec()
				s.ScaleSetSpec().Return(defaultSpec).AnyTimes()
				createdVMSS := newDefaultWindowsVMSS()
				instances := newDefaultInstances()

				setupDefaultVMSSInProgressOperationDoneExpectations(s, m, createdVMSS, instances)
				s.DeleteLongRunningOperationState(defaultSpec.Name, serviceName, infrav1.PutFuture)
				s.DeleteLongRunningOperationState(defaultSpec.Name, serviceName, infrav1.PatchFuture)
				s.UpdatePutStatus(infrav1.BootstrapSucceededCondition, serviceName, nil)
				s.HasReplicasExternallyManaged(gomockinternal.AContext()).Return(false)
			},
		},
		{
			name:          "should start creating vmss with defaulted accelerated networking when size allows",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PUT on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				spec.Size = "VM_SIZE_AN"
				s.ScaleSetSpec().Return(spec).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS("VM_SIZE_AN")
				netConfigs := vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.NetworkProfile.NetworkInterfaceConfigurations
				(*netConfigs)[0].EnableAcceleratedNetworking = ptr.To(true)
				vmss.Sku.Name = ptr.To(spec.Size)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(vmss)).
					Return(putFuture, nil)
				setupCreatingSucceededExpectations(s, m, newDefaultExistingVMSS("VM_SIZE_AN"), putFuture)
			},
		},
		{
			name:          "should start creating vmss with custom subnet when specified",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PUT on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				spec.Size = "VM_SIZE_AN"
				spec.NetworkInterfaces = []infrav1.NetworkInterface{
					{
						SubnetName:       "somesubnet",
						PrivateIPConfigs: 1, // defaulter sets this to one
					},
				}
				s.ScaleSetSpec().Return(spec).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS("VM_SIZE_AN")
				netConfigs := vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.NetworkProfile.NetworkInterfaceConfigurations
				(*netConfigs)[0].Name = ptr.To("my-vmss-nic-0")
				(*netConfigs)[0].EnableIPForwarding = ptr.To(true)
				(*netConfigs)[0].EnableAcceleratedNetworking = ptr.To(true)
				nic1IPConfigs := (*netConfigs)[0].IPConfigurations
				(*nic1IPConfigs)[0].Name = ptr.To("ipConfig0")
				(*nic1IPConfigs)[0].PrivateIPAddressVersion = compute.IPVersionIPv4
				(*nic1IPConfigs)[0].Subnet = &compute.APIEntityReference{
					ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/somesubnet"),
				}
				(*netConfigs)[0].EnableAcceleratedNetworking = ptr.To(true)
				(*netConfigs)[0].Primary = ptr.To(true)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(vmss)).
					Return(putFuture, nil)
				setupCreatingSucceededExpectations(s, m, newDefaultExistingVMSS("VM_SIZE_AN"), putFuture)
			},
		},
		{
			name:          "should start creating vmss with custom networking when specified",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PUT on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				spec.DataDisks = append(spec.DataDisks, infrav1.DataDisk{
					NameSuffix: "my_disk_with_ultra_disks",
					DiskSizeGB: 128,
					Lun:        ptr.To[int32](3),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "UltraSSD_LRS",
					},
				})
				spec.NetworkInterfaces = []infrav1.NetworkInterface{
					{
						SubnetName:            "my-subnet",
						PrivateIPConfigs:      1,
						AcceleratedNetworking: ptr.To(true),
					},
					{
						SubnetName:            "subnet2",
						PrivateIPConfigs:      2,
						AcceleratedNetworking: ptr.To(true),
					},
				}
				s.ScaleSetSpec().Return(spec).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS("VM_SIZE")
				vmss.VirtualMachineScaleSetProperties.AdditionalCapabilities = &compute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}
				netConfigs := vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.NetworkProfile.NetworkInterfaceConfigurations
				(*netConfigs)[0].Name = ptr.To("my-vmss-nic-0")
				(*netConfigs)[0].EnableIPForwarding = ptr.To(true)
				nic1IPConfigs := (*netConfigs)[0].IPConfigurations
				(*nic1IPConfigs)[0].Name = ptr.To("ipConfig0")
				(*nic1IPConfigs)[0].PrivateIPAddressVersion = compute.IPVersionIPv4
				(*netConfigs)[0].EnableAcceleratedNetworking = ptr.To(true)
				(*netConfigs)[0].Primary = ptr.To(true)
				vmssIPConfigs := []compute.VirtualMachineScaleSetIPConfiguration{
					{
						Name: ptr.To("ipConfig0"),
						VirtualMachineScaleSetIPConfigurationProperties: &compute.VirtualMachineScaleSetIPConfigurationProperties{
							Primary:                 ptr.To(true),
							PrivateIPAddressVersion: compute.IPVersionIPv4,
							Subnet: &compute.APIEntityReference{
								ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/subnet2"),
							},
						},
					},
					{
						Name: ptr.To("ipConfig1"),
						VirtualMachineScaleSetIPConfigurationProperties: &compute.VirtualMachineScaleSetIPConfigurationProperties{
							PrivateIPAddressVersion: compute.IPVersionIPv4,
							Subnet: &compute.APIEntityReference{
								ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/subnet2"),
							},
						},
					},
				}
				*netConfigs = append(*netConfigs, compute.VirtualMachineScaleSetNetworkConfiguration{
					Name: ptr.To("my-vmss-nic-1"),
					VirtualMachineScaleSetNetworkConfigurationProperties: &compute.VirtualMachineScaleSetNetworkConfigurationProperties{
						EnableAcceleratedNetworking: ptr.To(true),
						IPConfigurations:            &vmssIPConfigs,
						EnableIPForwarding:          ptr.To(true),
					},
				})
				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(vmss)).
					Return(putFuture, nil)
				setupCreatingSucceededExpectations(s, m, newDefaultExistingVMSS("VM_SIZE"), putFuture)
			},
		},
		{
			name:          "should start creating a vmss with spot vm",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PUT on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				spec.SpotVMOptions = &infrav1.SpotVMOptions{}
				spec.DataDisks = append(spec.DataDisks, infrav1.DataDisk{
					NameSuffix: "my_disk_with_ultra_disks",
					DiskSizeGB: 128,
					Lun:        ptr.To[int32](3),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "UltraSSD_LRS",
					},
				})
				s.ScaleSetSpec().Return(spec).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS("VM_SIZE")
				vmss.VirtualMachineScaleSetProperties.AdditionalCapabilities = &compute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.Priority = compute.VirtualMachinePriorityTypesSpot
				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(vmss)).
					Return(putFuture, nil)
				setupCreatingSucceededExpectations(s, m, newDefaultExistingVMSS("VM_SIZE"), putFuture)
			},
		},
		{
			name:          "should start creating a vmss with spot vm and ephemeral disk",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PUT on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				spec.Size = vmSizeEPH
				spec.SpotVMOptions = &infrav1.SpotVMOptions{}
				spec.OSDisk.DiffDiskSettings = &infrav1.DiffDiskSettings{
					Option: string(compute.DiffDiskOptionsLocal),
				}
				s.ScaleSetSpec().Return(spec).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS(vmSizeEPH)
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.StorageProfile.OsDisk.DiffDiskSettings = &compute.DiffDiskSettings{
					Option: compute.DiffDiskOptionsLocal,
				}
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.Priority = compute.VirtualMachinePriorityTypesSpot
				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(vmss)).
					Return(putFuture, nil)
				setupCreatingSucceededExpectations(s, m, newDefaultExistingVMSS(vmSizeEPH), putFuture)
			},
		},
		{
			name:          "should start creating a vmss with spot vm and a defined delete evictionPolicy",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PUT on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				spec.Size = vmSizeEPH
				deletePolicy := infrav1.SpotEvictionPolicyDelete
				spec.SpotVMOptions = &infrav1.SpotVMOptions{EvictionPolicy: &deletePolicy}
				s.ScaleSetSpec().Return(spec).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS(vmSizeEPH)
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.Priority = compute.VirtualMachinePriorityTypesSpot
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.EvictionPolicy = compute.VirtualMachineEvictionPolicyTypesDelete
				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(vmss)).
					Return(putFuture, nil)
				setupCreatingSucceededExpectations(s, m, newDefaultExistingVMSS(vmSizeEPH), putFuture)
			},
		},
		{
			name:          "should start creating a vmss with spot vm and a maximum price",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PUT on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				maxPrice := resource.MustParse("0.001")
				spec.SpotVMOptions = &infrav1.SpotVMOptions{
					MaxPrice: &maxPrice,
				}
				spec.DataDisks = append(spec.DataDisks, infrav1.DataDisk{
					NameSuffix: "my_disk_with_ultra_disks",
					DiskSizeGB: 128,
					Lun:        ptr.To[int32](3),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "UltraSSD_LRS",
					},
				})
				s.ScaleSetSpec().Return(spec).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS("VM_SIZE")
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.Priority = compute.VirtualMachinePriorityTypesSpot
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.BillingProfile = &compute.BillingProfile{
					MaxPrice: ptr.To[float64](0.001),
				}
				vmss.VirtualMachineScaleSetProperties.AdditionalCapabilities = &compute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}
				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(vmss)).
					Return(putFuture, nil)
				setupCreatingSucceededExpectations(s, m, newDefaultExistingVMSS("VM_SIZE"), putFuture)
			},
		},
		{
			name:          "should start creating a vmss with encryption",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PUT on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				spec.OSDisk.ManagedDisk.DiskEncryptionSet = &infrav1.DiskEncryptionSetParameters{
					ID: "my-diskencryptionset-id",
				}
				spec.DataDisks = append(spec.DataDisks, infrav1.DataDisk{
					NameSuffix: "my_disk_with_ultra_disks",
					DiskSizeGB: 128,
					Lun:        ptr.To[int32](3),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "UltraSSD_LRS",
					},
				})
				s.ScaleSetSpec().Return(spec).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS("VM_SIZE")
				vmss.VirtualMachineScaleSetProperties.AdditionalCapabilities = &compute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}
				osdisk := vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.StorageProfile.OsDisk
				osdisk.ManagedDisk = &compute.VirtualMachineScaleSetManagedDiskParameters{
					StorageAccountType: "Premium_LRS",
					DiskEncryptionSet: &compute.DiskEncryptionSetParameters{
						ID: ptr.To("my-diskencryptionset-id"),
					},
				}
				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(vmss)).
					Return(putFuture, nil)
				setupCreatingSucceededExpectations(s, m, newDefaultExistingVMSS("VM_SIZE"), putFuture)
			},
		},
		{
			name:          "can start creating a vmss with user assigned identity",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PUT on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				spec.DataDisks = append(spec.DataDisks, infrav1.DataDisk{
					NameSuffix: "my_disk_with_ultra_disks",
					DiskSizeGB: 128,
					Lun:        ptr.To[int32](3),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "UltraSSD_LRS",
					},
				})
				spec.Identity = infrav1.VMIdentityUserAssigned
				spec.UserAssignedIdentities = []infrav1.UserAssignedIdentity{
					{
						ProviderID: "azure:///subscriptions/123/resourcegroups/456/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id1",
					},
				}
				s.ScaleSetSpec().Return(spec).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS("VM_SIZE")
				vmss.VirtualMachineScaleSetProperties.AdditionalCapabilities = &compute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}
				vmss.Identity = &compute.VirtualMachineScaleSetIdentity{
					Type: compute.ResourceIdentityTypeUserAssigned,
					UserAssignedIdentities: map[string]*compute.VirtualMachineScaleSetIdentityUserAssignedIdentitiesValue{
						"/subscriptions/123/resourcegroups/456/providers/Microsoft.ManagedIdentity/userAssignedIdentities/id1": {},
					},
				}
				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(vmss)).
					Return(putFuture, nil)
				setupCreatingSucceededExpectations(s, m, newDefaultExistingVMSS("VM_SIZE"), putFuture)
			},
		},
		{
			name:          "should start creating a vmss with encryption at host enabled",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PUT on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				spec.Size = "VM_SIZE_EAH"
				spec.SecurityProfile = &infrav1.SecurityProfile{EncryptionAtHost: ptr.To(true)}
				s.ScaleSetSpec().Return(spec).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS("VM_SIZE_EAH")
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.SecurityProfile = &compute.SecurityProfile{
					EncryptionAtHost: ptr.To(true),
				}
				vmss.Sku.Name = ptr.To(spec.Size)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(vmss)).
					Return(putFuture, nil)
				setupCreatingSucceededExpectations(s, m, newDefaultExistingVMSS("VM_SIZE_EAH"), putFuture)
			},
		},
		{
			name:          "creating a vmss with encryption at host enabled for unsupported VM type fails",
			expectedError: "reconcile error that cannot be recovered occurred: encryption at host is not supported for VM type VM_SIZE. Object will not be requeued",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:            defaultVMSSName,
					Size:            "VM_SIZE",
					Capacity:        2,
					SSHKeyData:      "ZmFrZXNzaGtleQo=",
					SecurityProfile: &infrav1.SecurityProfile{EncryptionAtHost: ptr.To(true)},
				})
			},
		},
		{
			name:          "should start creating a vmss with ephemeral osdisk",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PUT on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				defaultSpec := newDefaultVMSSSpec()
				defaultSpec.Size = "VM_SIZE_EPH"
				defaultSpec.OSDisk.DiffDiskSettings = &infrav1.DiffDiskSettings{
					Option: "Local",
				}
				defaultSpec.OSDisk.CachingType = "ReadOnly"

				s.ScaleSetSpec().Return(defaultSpec).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS("VM_SIZE_EPH")
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.StorageProfile.OsDisk.DiffDiskSettings = &compute.DiffDiskSettings{
					Option: compute.DiffDiskOptionsLocal,
				}
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.StorageProfile.OsDisk.Caching = compute.CachingTypesReadOnly

				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(vmss)).
					Return(putFuture, nil)
				setupCreatingSucceededExpectations(s, m, newDefaultExistingVMSS("VM_SIZE_EPH"), putFuture)
			},
		},
		{
			name:          "should start updating when scale set already exists and not currently in a long running operation",
			expectedError: "failed to get VMSS my-vmss after create or update: failed to get result from future: operation type PATCH on Azure resource my-rg/my-vmss is not done",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				spec.Capacity = 2
				spec.DataDisks = append(spec.DataDisks, infrav1.DataDisk{
					NameSuffix: "my_disk_with_ultra_disks",
					DiskSizeGB: 128,
					Lun:        ptr.To[int32](3),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "UltraSSD_LRS",
					},
				})
				s.ScaleSetSpec().Return(spec).AnyTimes()

				setupDefaultVMSSUpdateExpectations(s)
				existingVMSS := newDefaultExistingVMSS("VM_SIZE")
				existingVMSS.VirtualMachineScaleSetProperties.AdditionalCapabilities = &compute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}
				existingVMSS.Sku.Capacity = ptr.To[int64](2)
				existingVMSS.VirtualMachineScaleSetProperties.AdditionalCapabilities = &compute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}
				instances := newDefaultInstances()
				m.Get(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).Return(existingVMSS, nil)
				m.ListInstances(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).Return(instances, nil)

				clone := newDefaultExistingVMSS("VM_SIZE")
				clone.Sku.Capacity = ptr.To[int64](3)
				clone.VirtualMachineScaleSetProperties.AdditionalCapabilities = &compute.AdditionalCapabilities{UltraSSDEnabled: ptr.To(true)}

				patchVMSS, err := getVMSSUpdateFromVMSS(clone)
				g.Expect(err).NotTo(HaveOccurred())
				patchVMSS.VirtualMachineProfile.StorageProfile.ImageReference.Version = ptr.To("2.0")
				patchVMSS.VirtualMachineProfile.NetworkProfile = nil
				m.UpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(patchVMSS)).
					Return(patchFuture, nil)
				s.SetLongRunningOperationState(patchFuture)
				m.GetResultIfDone(gomockinternal.AContext(), patchFuture).Return(compute.VirtualMachineScaleSet{}, azure.NewOperationNotDoneError(patchFuture))
				m.Get(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).Return(clone, nil)
				m.ListInstances(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).Return(instances, nil)
				s.HasReplicasExternallyManaged(gomockinternal.AContext()).Times(2).Return(false)
			},
		},
		{
			name:          "less than 2 vCPUs",
			expectedError: "reconcile error that cannot be recovered occurred: vm size should be bigger or equal to at least 2 vCPUs. Object will not be requeued",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       defaultVMSSName,
					Size:       "VM_SIZE_1_CPU",
					Capacity:   2,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
				})
			},
		},
		{
			name:          "Memory is less than 2Gi",
			expectedError: "reconcile error that cannot be recovered occurred: vm memory should be bigger or equal to at least 2Gi. Object will not be requeued",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       defaultVMSSName,
					Size:       "VM_SIZE_1_MEM",
					Capacity:   2,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
				})
			},
		},
		{
			name:          "failed to get SKU",
			expectedError: "failed to get SKU INVALID_VM_SIZE in compute api: reconcile error that cannot be recovered occurred: resource sku with name 'INVALID_VM_SIZE' and category 'virtualMachines' not found in location 'test-location'. Object will not be requeued",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       defaultVMSSName,
					Size:       "INVALID_VM_SIZE",
					Capacity:   2,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
				})
			},
		},
		{
			name:          "fails with internal error",
			expectedError: "failed to start creating VMSS: cannot create VMSS: #: Internal error: StatusCode=500",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				s.ScaleSetSpec().Return(spec).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				m.CreateOrUpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomock.AssignableToTypeOf(compute.VirtualMachineScaleSet{})).
					Return(nil, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal error"))
				m.Get(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).
					Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusNotFound}, "Not found"))
			},
		},
		{
			name:          "fail to create a vm with ultra disk implicitly enabled by data disk, when location not supported",
			expectedError: "reconcile error that cannot be recovered occurred: vm size VM_SIZE_USSD does not support ultra disks in location test-location. select a different vm size or disable ultra disks. Object will not be requeued",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       defaultVMSSName,
					Size:       "VM_SIZE_USSD",
					Capacity:   2,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
					DataDisks: []infrav1.DataDisk{
						{
							ManagedDisk: &infrav1.ManagedDiskParameters{
								StorageAccountType: "UltraSSD_LRS",
							},
						},
					},
				})
				s.Location().AnyTimes().Return("test-location")
			},
		},
		{
			name:          "fail to create a vm with ultra disk explicitly enabled via additional capabilities, when location not supported",
			expectedError: "reconcile error that cannot be recovered occurred: vm size VM_SIZE_USSD does not support ultra disks in location test-location. select a different vm size or disable ultra disks. Object will not be requeued",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       defaultVMSSName,
					Size:       "VM_SIZE_USSD",
					Capacity:   2,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
					AdditionalCapabilities: &infrav1.AdditionalCapabilities{
						UltraSSDEnabled: ptr.To(true),
					},
				})
				s.Location().AnyTimes().Return("test-location")
			},
		},
		{
			name:          "fail to create a vm with ultra disk explicitly enabled via additional capabilities, when location not supported",
			expectedError: "reconcile error that cannot be recovered occurred: vm size VM_SIZE_USSD does not support ultra disks in location test-location. select a different vm size or disable ultra disks. Object will not be requeued",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       defaultVMSSName,
					Size:       "VM_SIZE_USSD",
					Capacity:   2,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
					DataDisks: []infrav1.DataDisk{
						{
							ManagedDisk: &infrav1.ManagedDiskParameters{
								StorageAccountType: "UltraSSD_LRS",
							},
						},
					},
					AdditionalCapabilities: &infrav1.AdditionalCapabilities{
						UltraSSDEnabled: ptr.To(false),
					},
				})
				s.Location().AnyTimes().Return("test-location")
			},
		},
		{
			name:          "fail to create a vm with diagnostics set to User Managed but empty StorageAccountURI",
			expectedError: "reconcile error that cannot be recovered occurred: userManaged must be specified when storageAccountType is 'UserManaged'. Object will not be requeued",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:       defaultVMSSName,
					Size:       "VM_SIZE",
					Capacity:   2,
					SSHKeyData: "ZmFrZXNzaGtleQo=",
					DiagnosticsProfile: &infrav1.Diagnostics{
						Boot: &infrav1.BootDiagnostics{
							StorageAccountType: infrav1.UserManagedDiagnosticsStorage,
							UserManaged:        nil,
						},
					},
				})
				s.Location().AnyTimes().Return("test-location")
			},
		},
		{
			name:          "successfully create a vm with diagnostics set to User Managed and StorageAccountURI set",
			expectedError: "",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				storageURI := "https://fakeurl"

				spec := newDefaultVMSSSpec()
				spec.DiagnosticsProfile = &infrav1.Diagnostics{
					Boot: &infrav1.BootDiagnostics{
						StorageAccountType: infrav1.UserManagedDiagnosticsStorage,
						UserManaged: &infrav1.UserManagedBootDiagnostics{
							StorageAccountURI: storageURI,
						},
					},
				}
				s.ScaleSetSpec().Return(spec).AnyTimes()

				vmss := newDefaultVMSS("VM_SIZE")
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.DiagnosticsProfile = &compute.DiagnosticsProfile{BootDiagnostics: &compute.BootDiagnostics{
					Enabled:    ptr.To(true),
					StorageURI: &storageURI,
				}}

				instances := newDefaultInstances()

				setupDefaultVMSSInProgressOperationDoneExpectations(s, m, vmss, instances)
				s.DeleteLongRunningOperationState(spec.Name, serviceName, infrav1.PutFuture)
				s.DeleteLongRunningOperationState(spec.Name, serviceName, infrav1.PatchFuture)
				s.UpdatePutStatus(infrav1.BootstrapSucceededCondition, serviceName, nil)
				s.Location().AnyTimes().Return("test-location")
				s.HasReplicasExternallyManaged(gomockinternal.AContext()).Return(false)
			},
		},
		{
			name:          "successfully create a vm with diagnostics set to Managed",
			expectedError: "",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				spec.DiagnosticsProfile = &infrav1.Diagnostics{
					Boot: &infrav1.BootDiagnostics{
						StorageAccountType: infrav1.ManagedDiagnosticsStorage,
					},
				}

				s.ScaleSetSpec().Return(spec).AnyTimes()
				vmss := newDefaultVMSS("VM_SIZE")
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.DiagnosticsProfile = &compute.DiagnosticsProfile{BootDiagnostics: &compute.BootDiagnostics{
					Enabled: ptr.To(true),
				}}

				instances := newDefaultInstances()

				setupDefaultVMSSInProgressOperationDoneExpectations(s, m, vmss, instances)
				s.DeleteLongRunningOperationState(spec.Name, serviceName, infrav1.PutFuture)
				s.DeleteLongRunningOperationState(spec.Name, serviceName, infrav1.PatchFuture)
				s.UpdatePutStatus(infrav1.BootstrapSucceededCondition, serviceName, nil)
				s.Location().AnyTimes().Return("test-location")
				s.HasReplicasExternallyManaged(gomockinternal.AContext()).Return(false)
			},
		},
		{
			name:          "successfully create a vm with diagnostics set to Disabled",
			expectedError: "",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				spec.DiagnosticsProfile = &infrav1.Diagnostics{
					Boot: &infrav1.BootDiagnostics{
						StorageAccountType: infrav1.DisabledDiagnosticsStorage,
					},
				}
				s.ScaleSetSpec().Return(spec).AnyTimes()

				vmss := newDefaultVMSS("VM_SIZE")
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.DiagnosticsProfile = &compute.DiagnosticsProfile{BootDiagnostics: &compute.BootDiagnostics{
					Enabled: ptr.To(false),
				}}
				instances := newDefaultInstances()

				setupDefaultVMSSInProgressOperationDoneExpectations(s, m, vmss, instances)
				s.DeleteLongRunningOperationState(spec.Name, serviceName, infrav1.PutFuture)
				s.DeleteLongRunningOperationState(spec.Name, serviceName, infrav1.PatchFuture)
				s.UpdatePutStatus(infrav1.BootstrapSucceededCondition, serviceName, nil)
				s.Location().AnyTimes().Return("test-location")
				s.HasReplicasExternallyManaged(gomockinternal.AContext()).Return(false)
			},
		},
		{
			name:          "should not panic when DiagnosticsProfile is nil",
			expectedError: "",
			expect: func(g *WithT, s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				spec := newDefaultVMSSSpec()
				spec.DiagnosticsProfile = nil
				s.ScaleSetSpec().Return(spec).AnyTimes()

				vmss := newDefaultVMSS("VM_SIZE")
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.DiagnosticsProfile = nil

				instances := newDefaultInstances()

				setupDefaultVMSSInProgressOperationDoneExpectations(s, m, vmss, instances)
				s.DeleteLongRunningOperationState(spec.Name, serviceName, infrav1.PutFuture)
				s.DeleteLongRunningOperationState(spec.Name, serviceName, infrav1.PatchFuture)
				s.UpdatePutStatus(infrav1.BootstrapSucceededCondition, serviceName, nil)
				s.Location().AnyTimes().Return("test-location")
				s.HasReplicasExternallyManaged(gomockinternal.AContext()).Return(false)
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
				resourceSKUCache: resourceskus.NewStaticCache(getFakeSkus(), "test-location"),
			}

			err := s.Reconcile(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError), err.Error())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestDeleteVMSS(t *testing.T) {
	const (
		resourceGroup = "my-rg"
		name          = "my-vmss"
	)

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
				}).AnyTimes()
				s.ResourceGroup().AnyTimes().Return("my-existing-rg")
				future := &infrav1.Future{}
				s.GetLongRunningOperationState("my-existing-vmss", serviceName, infrav1.DeleteFuture).Return(future)
				m.GetResultIfDone(gomockinternal.AContext(), future).Return(compute.VirtualMachineScaleSet{}, nil)
				m.Get(gomockinternal.AContext(), "my-existing-rg", "my-existing-vmss").
					Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusNotFound}, "Not found"))
				s.DeleteLongRunningOperationState("my-existing-vmss", serviceName, infrav1.DeleteFuture)
				s.UpdateDeleteStatus(infrav1.BootstrapSucceededCondition, serviceName, nil)
			},
		},
		{
			name:          "vmss already deleted",
			expectedError: "",
			expect: func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:     name,
					Size:     "VM_SIZE",
					Capacity: 3,
				}).AnyTimes()
				s.ResourceGroup().AnyTimes().Return(resourceGroup)
				s.GetLongRunningOperationState(name, serviceName, infrav1.DeleteFuture).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), resourceGroup, name).
					Return(nil, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusNotFound}, "Not found"))
				m.Get(gomockinternal.AContext(), resourceGroup, name).
					Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusNotFound}, "Not found"))
			},
		},
		{
			name:          "vmss deletion fails",
			expectedError: "failed to delete VMSS my-vmss in resource group my-rg: #: Internal Server Error: StatusCode=500",
			expect: func(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
				s.ScaleSetSpec().Return(azure.ScaleSetSpec{
					Name:     name,
					Size:     "VM_SIZE",
					Capacity: 3,
				}).AnyTimes()
				s.ResourceGroup().AnyTimes().Return(resourceGroup)
				s.GetLongRunningOperationState(name, serviceName, infrav1.DeleteFuture).Return(nil)
				m.DeleteAsync(gomockinternal.AContext(), resourceGroup, name).
					Return(nil, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error"))
				m.Get(gomockinternal.AContext(), resourceGroup, name).
					Return(newDefaultVMSS("VM_SIZE"), nil)
				m.ListInstances(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).Return(newDefaultInstances(), nil).AnyTimes()
				s.SetVMSSState(gomock.AssignableToTypeOf(&azure.VMSS{}))
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
			Name:         ptr.To("VM_SIZE"),
			ResourceType: ptr.To(string(resourceskus.VirtualMachines)),
			Kind:         ptr.To(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: ptr.To("test-location"),
					Zones:    &[]string{"1", "3"},
					ZoneDetails: &[]compute.ResourceSkuZoneDetails{
						{
							Capabilities: &[]compute.ResourceSkuCapabilities{
								{
									Name:  ptr.To("UltraSSDAvailable"),
									Value: ptr.To("True"),
								},
							},
							Name: &[]string{"1", "3"},
						},
					},
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  ptr.To(resourceskus.AcceleratedNetworking),
					Value: ptr.To(string(resourceskus.CapabilityUnsupported)),
				},
				{
					Name:  ptr.To(resourceskus.VCPUs),
					Value: ptr.To("4"),
				},
				{
					Name:  ptr.To(resourceskus.MemoryGB),
					Value: ptr.To("4"),
				},
			},
		},
		{
			Name:         ptr.To("VM_SIZE_AN"),
			ResourceType: ptr.To(string(resourceskus.VirtualMachines)),
			Kind:         ptr.To(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: ptr.To("test-location"),
					Zones:    &[]string{"1", "3"},
					// ZoneDetails: &[]compute.ResourceSkuZoneDetails{
					//    {
					//        	Capabilities: &[]compute.ResourceSkuCapabilities{
					//        		{
					//        			Name:  ptr.To("UltraSSDAvailable"),
					//        			Value: ptr.To("True"),
					//        		},
					//        	},
					//        },
					//    },
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  ptr.To(resourceskus.AcceleratedNetworking),
					Value: ptr.To(string(resourceskus.CapabilitySupported)),
				},
				{
					Name:  ptr.To(resourceskus.VCPUs),
					Value: ptr.To("4"),
				},
				{
					Name:  ptr.To(resourceskus.MemoryGB),
					Value: ptr.To("6"),
				},
			},
		},
		{
			Name:         ptr.To("VM_SIZE_1_CPU"),
			ResourceType: ptr.To(string(resourceskus.VirtualMachines)),
			Kind:         ptr.To(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: ptr.To("test-location"),
					Zones:    &[]string{"1", "3"},
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  ptr.To(resourceskus.AcceleratedNetworking),
					Value: ptr.To(string(resourceskus.CapabilityUnsupported)),
				},
				{
					Name:  ptr.To(resourceskus.VCPUs),
					Value: ptr.To("1"),
				},
				{
					Name:  ptr.To(resourceskus.MemoryGB),
					Value: ptr.To("4"),
				},
			},
		},
		{
			Name:         ptr.To("VM_SIZE_1_MEM"),
			ResourceType: ptr.To(string(resourceskus.VirtualMachines)),
			Kind:         ptr.To(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: ptr.To("test-location"),
					Zones:    &[]string{"1", "3"},
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  ptr.To(resourceskus.AcceleratedNetworking),
					Value: ptr.To(string(resourceskus.CapabilityUnsupported)),
				},
				{
					Name:  ptr.To(resourceskus.VCPUs),
					Value: ptr.To("2"),
				},
				{
					Name:  ptr.To(resourceskus.MemoryGB),
					Value: ptr.To("1"),
				},
			},
		},
		{
			Name:         ptr.To("VM_SIZE_EAH"),
			ResourceType: ptr.To(string(resourceskus.VirtualMachines)),
			Kind:         ptr.To(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: ptr.To("test-location"),
					Zones:    &[]string{"1", "3"},
					//  ZoneDetails: &[]compute.ResourceSkuZoneDetails{
					//	    {
					//    		Capabilities: &[]compute.ResourceSkuCapabilities{
					//    			{
					//    				Name:  ptr.To("UltraSSDAvailable"),
					//    				Value: ptr.To("True"),
					//    			},
					//    		},
					//	    },
					//  },
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  ptr.To(resourceskus.VCPUs),
					Value: ptr.To("4"),
				},
				{
					Name:  ptr.To(resourceskus.MemoryGB),
					Value: ptr.To("8"),
				},
				{
					Name:  ptr.To(resourceskus.EncryptionAtHost),
					Value: ptr.To(string(resourceskus.CapabilitySupported)),
				},
			},
		},
		{
			Name:         ptr.To("VM_SIZE_USSD"),
			ResourceType: ptr.To(string(resourceskus.VirtualMachines)),
			Kind:         ptr.To(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: ptr.To("test-location"),
					Zones:    &[]string{"1", "3"},
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  ptr.To(resourceskus.AcceleratedNetworking),
					Value: ptr.To(string(resourceskus.CapabilitySupported)),
				},
				{
					Name:  ptr.To(resourceskus.VCPUs),
					Value: ptr.To("4"),
				},
				{
					Name:  ptr.To(resourceskus.MemoryGB),
					Value: ptr.To("6"),
				},
			},
		},
		{
			Name:         ptr.To("VM_SIZE_EPH"),
			ResourceType: ptr.To(string(resourceskus.VirtualMachines)),
			Kind:         ptr.To(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: ptr.To("test-location"),
					Zones:    &[]string{"1", "3"},
					ZoneDetails: &[]compute.ResourceSkuZoneDetails{
						{
							Capabilities: &[]compute.ResourceSkuCapabilities{
								{
									Name:  ptr.To("UltraSSDAvailable"),
									Value: ptr.To("True"),
								},
							},
							Name: &[]string{"1", "3"},
						},
					},
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  ptr.To(resourceskus.AcceleratedNetworking),
					Value: ptr.To(string(resourceskus.CapabilityUnsupported)),
				},
				{
					Name:  ptr.To(resourceskus.VCPUs),
					Value: ptr.To("4"),
				},
				{
					Name:  ptr.To(resourceskus.MemoryGB),
					Value: ptr.To("4"),
				},
				{
					Name:  ptr.To(resourceskus.EphemeralOSDisk),
					Value: ptr.To("True"),
				},
			},
		},
	}
}

func newDefaultVMSSSpec() azure.ScaleSetSpec {
	return azure.ScaleSetSpec{
		Name:       defaultVMSSName,
		Size:       "VM_SIZE",
		Capacity:   2,
		SSHKeyData: "ZmFrZXNzaGtleQo=",
		OSDisk: infrav1.OSDisk{
			OSType:     "Linux",
			DiskSizeGB: ptr.To[int32](120),
			ManagedDisk: &infrav1.ManagedDiskParameters{
				StorageAccountType: "Premium_LRS",
			},
		},
		DataDisks: []infrav1.DataDisk{
			{
				NameSuffix: "my_disk",
				DiskSizeGB: 128,
				Lun:        ptr.To[int32](0),
			},
			{
				NameSuffix: "my_disk_with_managed_disk",
				DiskSizeGB: 128,
				Lun:        ptr.To[int32](1),
				ManagedDisk: &infrav1.ManagedDiskParameters{
					StorageAccountType: "Standard_LRS",
				},
			},
			{
				NameSuffix: "managed_disk_with_encryption",
				DiskSizeGB: 128,
				Lun:        ptr.To[int32](2),
				ManagedDisk: &infrav1.ManagedDiskParameters{
					StorageAccountType: "Standard_LRS",
					DiskEncryptionSet: &infrav1.DiskEncryptionSetParameters{
						ID: "encryption_id",
					},
				},
			},
		},
		DiagnosticsProfile: &infrav1.Diagnostics{
			Boot: &infrav1.BootDiagnostics{
				StorageAccountType: infrav1.ManagedDiagnosticsStorage,
			},
		},
		SubnetName:                   "my-subnet",
		VNetName:                     "my-vnet",
		VNetResourceGroup:            defaultResourceGroup,
		PublicLBName:                 "capz-lb",
		PublicLBAddressPoolName:      "backendPool",
		AcceleratedNetworking:        nil,
		TerminateNotificationTimeout: ptr.To(7),
		FailureDomains:               []string{"1", "3"},
		NetworkInterfaces: []infrav1.NetworkInterface{
			{
				SubnetName:       "my-subnet",
				PrivateIPConfigs: 1,
			},
		},
	}
}

func newWindowsVMSSSpec() azure.ScaleSetSpec {
	vmss := newDefaultVMSSSpec()
	vmss.OSDisk.OSType = azure.WindowsOS
	return vmss
}

func newDefaultExistingVMSS(vmSize string) compute.VirtualMachineScaleSet {
	vmss := newDefaultVMSS(vmSize)
	vmss.ID = ptr.To("subscriptions/1234/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-vm")
	return vmss
}

func newDefaultWindowsVMSS() compute.VirtualMachineScaleSet {
	vmss := newDefaultVMSS("VM_SIZE")
	vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.StorageProfile.OsDisk.OsType = compute.OperatingSystemTypesWindows
	vmss.VirtualMachineProfile.OsProfile.LinuxConfiguration = nil
	vmss.VirtualMachineProfile.OsProfile.WindowsConfiguration = &compute.WindowsConfiguration{
		EnableAutomaticUpdates: ptr.To(false),
	}
	return vmss
}

func newDefaultVMSS(vmSize string) compute.VirtualMachineScaleSet {
	dataDisk := fetchDataDiskBasedOnSize(vmSize)
	return compute.VirtualMachineScaleSet{
		Location: ptr.To("test-location"),
		Tags: map[string]*string{
			"Name": ptr.To(defaultVMSSName),
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": ptr.To("owned"),
			"sigs.k8s.io_cluster-api-provider-azure_role":               ptr.To("node"),
		},
		Sku: &compute.Sku{
			Name:     ptr.To(vmSize),
			Tier:     ptr.To("Standard"),
			Capacity: ptr.To[int64](2),
		},
		Zones: &[]string{"1", "3"},
		VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
			SinglePlacementGroup: ptr.To(false),
			UpgradePolicy: &compute.UpgradePolicy{
				Mode: compute.UpgradeModeManual,
			},
			Overprovision:     ptr.To(false),
			OrchestrationMode: compute.OrchestrationModeUniform,
			VirtualMachineProfile: &compute.VirtualMachineScaleSetVMProfile{
				OsProfile: &compute.VirtualMachineScaleSetOSProfile{
					ComputerNamePrefix: ptr.To(defaultVMSSName),
					AdminUsername:      ptr.To(azure.DefaultUserName),
					CustomData:         ptr.To("fake-bootstrap-data"),
					LinuxConfiguration: &compute.LinuxConfiguration{
						SSH: &compute.SSHConfiguration{
							PublicKeys: &[]compute.SSHPublicKey{
								{
									Path:    ptr.To("/home/capi/.ssh/authorized_keys"),
									KeyData: ptr.To("fakesshkey\n"),
								},
							},
						},
						DisablePasswordAuthentication: ptr.To(true),
					},
				},
				ScheduledEventsProfile: &compute.ScheduledEventsProfile{
					TerminateNotificationProfile: &compute.TerminateNotificationProfile{
						NotBeforeTimeout: ptr.To("PT7M"),
						Enable:           ptr.To(true),
					},
				},
				StorageProfile: &compute.VirtualMachineScaleSetStorageProfile{
					ImageReference: &compute.ImageReference{
						Publisher: ptr.To("fake-publisher"),
						Offer:     ptr.To("my-offer"),
						Sku:       ptr.To("sku-id"),
						Version:   ptr.To("1.0"),
					},
					OsDisk: &compute.VirtualMachineScaleSetOSDisk{
						OsType:       "Linux",
						CreateOption: "FromImage",
						DiskSizeGB:   ptr.To[int32](120),
						ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
							StorageAccountType: "Premium_LRS",
						},
					},
					DataDisks: dataDisk,
				},
				DiagnosticsProfile: &compute.DiagnosticsProfile{
					BootDiagnostics: &compute.BootDiagnostics{
						Enabled: ptr.To(true),
					},
				},
				NetworkProfile: &compute.VirtualMachineScaleSetNetworkProfile{
					NetworkInterfaceConfigurations: &[]compute.VirtualMachineScaleSetNetworkConfiguration{
						{
							Name: ptr.To("my-vmss-nic-0"),
							VirtualMachineScaleSetNetworkConfigurationProperties: &compute.VirtualMachineScaleSetNetworkConfigurationProperties{
								Primary:                     ptr.To(true),
								EnableAcceleratedNetworking: ptr.To(false),
								EnableIPForwarding:          ptr.To(true),
								IPConfigurations: &[]compute.VirtualMachineScaleSetIPConfiguration{
									{
										Name: ptr.To("ipConfig0"),
										VirtualMachineScaleSetIPConfigurationProperties: &compute.VirtualMachineScaleSetIPConfigurationProperties{
											Subnet: &compute.APIEntityReference{
												ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet"),
											},
											Primary:                         ptr.To(true),
											PrivateIPAddressVersion:         compute.IPVersionIPv4,
											LoadBalancerBackendAddressPools: &[]compute.SubResource{{ID: ptr.To("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/capz-lb/backendAddressPools/backendPool")}},
										},
									},
								},
							},
						},
					},
				},
				ExtensionProfile: &compute.VirtualMachineScaleSetExtensionProfile{
					Extensions: &[]compute.VirtualMachineScaleSetExtension{
						{
							Name: ptr.To("someExtension"),
							VirtualMachineScaleSetExtensionProperties: &compute.VirtualMachineScaleSetExtensionProperties{
								Publisher:          ptr.To("somePublisher"),
								Type:               ptr.To("someExtension"),
								TypeHandlerVersion: ptr.To("someVersion"),
								Settings: map[string]string{
									"someSetting": "someValue",
								},
								ProtectedSettings: map[string]string{
									"commandToExecute": "echo hello",
								},
							},
						},
					},
				},
			},
		},
	}
}

func fetchDataDiskBasedOnSize(vmSize string) *[]compute.VirtualMachineScaleSetDataDisk {
	var dataDisk *[]compute.VirtualMachineScaleSetDataDisk
	if vmSize == "VM_SIZE" {
		dataDisk = &[]compute.VirtualMachineScaleSetDataDisk{
			{
				Lun:          ptr.To[int32](0),
				Name:         ptr.To("my-vmss_my_disk"),
				CreateOption: "Empty",
				DiskSizeGB:   ptr.To[int32](128),
			},
			{
				Lun:          ptr.To[int32](1),
				Name:         ptr.To("my-vmss_my_disk_with_managed_disk"),
				CreateOption: "Empty",
				DiskSizeGB:   ptr.To[int32](128),
				ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
					StorageAccountType: "Standard_LRS",
				},
			},
			{
				Lun:          ptr.To[int32](2),
				Name:         ptr.To("my-vmss_managed_disk_with_encryption"),
				CreateOption: "Empty",
				DiskSizeGB:   ptr.To[int32](128),
				ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
					StorageAccountType: "Standard_LRS",
					DiskEncryptionSet: &compute.DiskEncryptionSetParameters{
						ID: ptr.To("encryption_id"),
					},
				},
			},
			{
				Lun:          ptr.To[int32](3),
				Name:         ptr.To("my-vmss_my_disk_with_ultra_disks"),
				CreateOption: "Empty",
				DiskSizeGB:   ptr.To[int32](128),
				ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
					StorageAccountType: "UltraSSD_LRS",
				},
			},
		}
	} else {
		dataDisk = &[]compute.VirtualMachineScaleSetDataDisk{
			{
				Lun:          ptr.To[int32](0),
				Name:         ptr.To("my-vmss_my_disk"),
				CreateOption: "Empty",
				DiskSizeGB:   ptr.To[int32](128),
			},
			{
				Lun:          ptr.To[int32](1),
				Name:         ptr.To("my-vmss_my_disk_with_managed_disk"),
				CreateOption: "Empty",
				DiskSizeGB:   ptr.To[int32](128),
				ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
					StorageAccountType: "Standard_LRS",
				},
			},
			{
				Lun:          ptr.To[int32](2),
				Name:         ptr.To("my-vmss_managed_disk_with_encryption"),
				CreateOption: "Empty",
				DiskSizeGB:   ptr.To[int32](128),
				ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
					StorageAccountType: "Standard_LRS",
					DiskEncryptionSet: &compute.DiskEncryptionSetParameters{
						ID: ptr.To("encryption_id"),
					},
				},
			},
		}
	}
	return dataDisk
}

func newDefaultInstances() []compute.VirtualMachineScaleSetVM {
	return []compute.VirtualMachineScaleSetVM{
		{
			ID:         ptr.To("my-vm-id"),
			InstanceID: ptr.To("my-vm-1"),
			Name:       ptr.To("my-vm"),
			VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
				ProvisioningState: ptr.To("Succeeded"),
				OsProfile: &compute.OSProfile{
					ComputerName: ptr.To("instance-000001"),
				},
				StorageProfile: &compute.StorageProfile{
					ImageReference: &compute.ImageReference{
						Publisher: ptr.To("fake-publisher"),
						Offer:     ptr.To("my-offer"),
						Sku:       ptr.To("sku-id"),
						Version:   ptr.To("1.0"),
					},
				},
			},
		},
		{
			ID:         ptr.To("my-vm-id"),
			InstanceID: ptr.To("my-vm-2"),
			Name:       ptr.To("my-vm"),
			VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
				ProvisioningState: ptr.To("Succeeded"),
				OsProfile: &compute.OSProfile{
					ComputerName: ptr.To("instance-000002"),
				},
				StorageProfile: &compute.StorageProfile{
					ImageReference: &compute.ImageReference{
						Publisher: ptr.To("fake-publisher"),
						Offer:     ptr.To("my-offer"),
						Sku:       ptr.To("sku-id"),
						Version:   ptr.To("1.0"),
					},
				},
			},
		},
	}
}

func setupDefaultVMSSInProgressOperationDoneExpectations(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder, createdVMSS compute.VirtualMachineScaleSet, instances []compute.VirtualMachineScaleSetVM) {
	createdVMSS.ID = ptr.To("subscriptions/1234/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-vm")
	createdVMSS.ProvisioningState = ptr.To(string(infrav1.Succeeded))
	setupDefaultVMSSExpectations(s)
	future := &infrav1.Future{
		Type:          infrav1.PutFuture,
		ResourceGroup: defaultResourceGroup,
		Name:          defaultVMSSName,
		Data:          "",
	}
	s.GetLongRunningOperationState(defaultVMSSName, serviceName, infrav1.PutFuture).Return(future)
	m.GetResultIfDone(gomockinternal.AContext(), future).Return(createdVMSS, nil).AnyTimes()
	m.ListInstances(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).Return(instances, nil).AnyTimes()
	s.MaxSurge().Return(1, nil)
	s.SetVMSSState(gomock.Any())
	s.SetProviderID(azureutil.ProviderIDPrefix + *createdVMSS.ID)
}

func setupDefaultVMSSStartCreatingExpectations(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder) {
	setupDefaultVMSSExpectations(s)
	s.GetLongRunningOperationState(defaultVMSSName, serviceName, infrav1.PutFuture).Return(nil)
	s.GetLongRunningOperationState(defaultVMSSName, serviceName, infrav1.PatchFuture).Return(nil)
	m.Get(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).
		Return(compute.VirtualMachineScaleSet{}, autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusNotFound}, "Not found"))
}

func setupCreatingSucceededExpectations(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder, vmss compute.VirtualMachineScaleSet, future *infrav1.Future) {
	s.SetLongRunningOperationState(future)
	m.GetResultIfDone(gomockinternal.AContext(), future).Return(compute.VirtualMachineScaleSet{}, azure.NewOperationNotDoneError(future))
	m.Get(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).Return(vmss, nil)
	m.ListInstances(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).Return(newDefaultInstances(), nil).AnyTimes()
	s.SetVMSSState(gomock.Any())
	s.SetProviderID(azureutil.ProviderIDPrefix + *vmss.ID)
}

func setupDefaultVMSSExpectations(s *mock_scalesets.MockScaleSetScopeMockRecorder) {
	setupVMSSExpectationsWithoutVMImage(s)
	image := &infrav1.Image{
		Marketplace: &infrav1.AzureMarketplaceImage{
			ImagePlan: infrav1.ImagePlan{
				Publisher: "fake-publisher",
				Offer:     "my-offer",
				SKU:       "sku-id",
			},
			Version: "1.0",
		},
	}
	s.GetVMImage(gomockinternal.AContext()).Return(image, nil).AnyTimes()
	s.SaveVMImageToStatus(image)
}

func setupUpdateVMSSExpectations(s *mock_scalesets.MockScaleSetScopeMockRecorder) {
	setupVMSSExpectationsWithoutVMImage(s)
	image := &infrav1.Image{
		Marketplace: &infrav1.AzureMarketplaceImage{
			ImagePlan: infrav1.ImagePlan{
				Publisher: "fake-publisher",
				Offer:     "my-offer",
				SKU:       "sku-id",
			},
			Version: "2.0",
		},
	}
	s.GetVMImage(gomockinternal.AContext()).Return(image, nil).AnyTimes()
	s.SaveVMImageToStatus(image)
}

func setupVMSSExpectationsWithoutVMImage(s *mock_scalesets.MockScaleSetScopeMockRecorder) {
	s.SubscriptionID().AnyTimes().Return(defaultSubscriptionID)
	s.ResourceGroup().AnyTimes().Return(defaultResourceGroup)
	s.AdditionalTags()
	s.Location().AnyTimes().Return("test-location")
	s.ClusterName().Return("my-cluster")
	s.GetBootstrapData(gomockinternal.AContext()).Return("fake-bootstrap-data", nil)
	s.VMSSExtensionSpecs().Return([]azure.ResourceSpecGetter{
		&VMSSExtensionSpec{
			ExtensionSpec: azure.ExtensionSpec{
				Name:      "someExtension",
				VMName:    "my-vmss",
				Publisher: "somePublisher",
				Version:   "someVersion",
				Settings: map[string]string{
					"someSetting": "someValue",
				},
				ProtectedSettings: map[string]string{
					"commandToExecute": "echo hello",
				},
			},
			ResourceGroup: "my-rg",
		},
	}).AnyTimes()
	s.ReconcileReplicas(gomock.Any(), gomock.Any()).AnyTimes()
}

func setupDefaultVMSSUpdateExpectations(s *mock_scalesets.MockScaleSetScopeMockRecorder) {
	setupUpdateVMSSExpectations(s)
	s.SetProviderID(azureutil.ProviderIDPrefix + "subscriptions/1234/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-vm")
	s.GetLongRunningOperationState(defaultVMSSName, serviceName, infrav1.PutFuture).Return(nil)
	s.GetLongRunningOperationState(defaultVMSSName, serviceName, infrav1.PatchFuture).Return(nil)
	s.MaxSurge().Return(1, nil)
	s.SetVMSSState(gomock.Any())
}

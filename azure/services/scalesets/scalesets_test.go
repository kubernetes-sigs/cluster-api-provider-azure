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
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/scalesets/mock_scalesets"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
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
					ID:   pointer.String("my-id"),
					Name: pointer.String("my-vmss"),
					Sku: &compute.Sku{
						Capacity: pointer.Int64(1),
						Name:     pointer.String("Standard_D2"),
					},
					VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
						SinglePlacementGroup: pointer.Bool(false),
						ProvisioningState:    pointer.String("Succeeded"),
					},
					Zones: &[]string{"1", "3"},
				}, nil)
				m.ListInstances(gomock.Any(), "my-rg", "my-vmss").Return([]compute.VirtualMachineScaleSetVM{
					{
						ID:         pointer.String("my-vm-id"),
						InstanceID: pointer.String("my-vm-1"),
						Name:       pointer.String("my-vm"),
						VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
							ProvisioningState: pointer.String("Succeeded"),
							OsProfile: &compute.OSProfile{
								ComputerName: pointer.String("instance-000001"),
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
					ID:   pointer.String("my-id"),
					Name: pointer.String("my-vmss"),
					VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
						SinglePlacementGroup: pointer.Bool(false),
						ProvisioningState:    pointer.String("Succeeded"),
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
					Lun:        pointer.Int32(3),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "UltraSSD_LRS",
					},
				})
				s.ScaleSetSpec().Return(defaultSpec).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS("VM_SIZE")
				vmss.VirtualMachineScaleSetProperties.AdditionalCapabilities = &compute.AdditionalCapabilities{UltraSSDEnabled: pointer.Bool(true)}
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
				(*netConfigs)[0].EnableAcceleratedNetworking = pointer.Bool(true)
				vmss.Sku.Name = pointer.String(spec.Size)
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
					Lun:        pointer.Int32(3),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "UltraSSD_LRS",
					},
				})
				spec.NetworkInterfaces = []infrav1.NetworkInterface{
					{
						SubnetName:            "my-subnet",
						PrivateIPConfigs:      1,
						AcceleratedNetworking: pointer.Bool(true),
					},
					{
						SubnetName:            "subnet2",
						PrivateIPConfigs:      2,
						AcceleratedNetworking: pointer.Bool(true),
					},
				}
				s.ScaleSetSpec().Return(spec).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS("VM_SIZE")
				vmss.VirtualMachineScaleSetProperties.AdditionalCapabilities = &compute.AdditionalCapabilities{UltraSSDEnabled: pointer.Bool(true)}
				netConfigs := vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.NetworkProfile.NetworkInterfaceConfigurations
				(*netConfigs)[0].Name = pointer.String("my-vmss-0")
				(*netConfigs)[0].EnableIPForwarding = nil
				nic1IPConfigs := (*netConfigs)[0].IPConfigurations
				(*nic1IPConfigs)[0].Name = pointer.String("private-ipConfig-0")
				(*nic1IPConfigs)[0].PrivateIPAddressVersion = compute.IPVersionIPv4
				(*netConfigs)[0].EnableAcceleratedNetworking = pointer.Bool(true)
				(*netConfigs)[0].Primary = pointer.Bool(true)
				vmssIPConfigs := []compute.VirtualMachineScaleSetIPConfiguration{
					{
						Name: pointer.String("private-ipConfig-0"),
						VirtualMachineScaleSetIPConfigurationProperties: &compute.VirtualMachineScaleSetIPConfigurationProperties{
							Primary:                 pointer.Bool(true),
							PrivateIPAddressVersion: compute.IPVersionIPv4,
							Subnet: &compute.APIEntityReference{
								ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/subnet2"),
							},
						},
					},
					{
						Name: pointer.String("private-ipConfig-1"),
						VirtualMachineScaleSetIPConfigurationProperties: &compute.VirtualMachineScaleSetIPConfigurationProperties{
							PrivateIPAddressVersion: compute.IPVersionIPv4,
							Subnet: &compute.APIEntityReference{
								ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/subnet2"),
							},
						},
					},
				}
				*netConfigs = append(*netConfigs, compute.VirtualMachineScaleSetNetworkConfiguration{
					Name: pointer.String("my-vmss-1"),
					VirtualMachineScaleSetNetworkConfigurationProperties: &compute.VirtualMachineScaleSetNetworkConfigurationProperties{
						EnableAcceleratedNetworking: pointer.Bool(true),
						IPConfigurations:            &vmssIPConfigs,
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
					Lun:        pointer.Int32(3),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "UltraSSD_LRS",
					},
				})
				s.ScaleSetSpec().Return(spec).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS("VM_SIZE")
				vmss.VirtualMachineScaleSetProperties.AdditionalCapabilities = &compute.AdditionalCapabilities{UltraSSDEnabled: pointer.Bool(true)}
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
					Lun:        pointer.Int32(3),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "UltraSSD_LRS",
					},
				})
				s.ScaleSetSpec().Return(spec).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS("VM_SIZE")
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.Priority = compute.VirtualMachinePriorityTypesSpot
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.BillingProfile = &compute.BillingProfile{
					MaxPrice: pointer.Float64(0.001),
				}
				vmss.VirtualMachineScaleSetProperties.AdditionalCapabilities = &compute.AdditionalCapabilities{UltraSSDEnabled: pointer.Bool(true)}
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
					Lun:        pointer.Int32(3),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "UltraSSD_LRS",
					},
				})
				s.ScaleSetSpec().Return(spec).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS("VM_SIZE")
				vmss.VirtualMachineScaleSetProperties.AdditionalCapabilities = &compute.AdditionalCapabilities{UltraSSDEnabled: pointer.Bool(true)}
				osdisk := vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.StorageProfile.OsDisk
				osdisk.ManagedDisk = &compute.VirtualMachineScaleSetManagedDiskParameters{
					StorageAccountType: "Premium_LRS",
					DiskEncryptionSet: &compute.DiskEncryptionSetParameters{
						ID: pointer.String("my-diskencryptionset-id"),
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
					Lun:        pointer.Int32(3),
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
				vmss.VirtualMachineScaleSetProperties.AdditionalCapabilities = &compute.AdditionalCapabilities{UltraSSDEnabled: pointer.Bool(true)}
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
				spec.SecurityProfile = &infrav1.SecurityProfile{EncryptionAtHost: pointer.Bool(true)}
				s.ScaleSetSpec().Return(spec).AnyTimes()
				setupDefaultVMSSStartCreatingExpectations(s, m)
				vmss := newDefaultVMSS("VM_SIZE_EAH")
				vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.SecurityProfile = &compute.SecurityProfile{
					EncryptionAtHost: pointer.Bool(true),
				}
				vmss.Sku.Name = pointer.String(spec.Size)
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
					SecurityProfile: &infrav1.SecurityProfile{EncryptionAtHost: pointer.Bool(true)},
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
					Lun:        pointer.Int32(3),
					ManagedDisk: &infrav1.ManagedDiskParameters{
						StorageAccountType: "UltraSSD_LRS",
					},
				})
				s.ScaleSetSpec().Return(spec).AnyTimes()

				setupDefaultVMSSUpdateExpectations(s)
				existingVMSS := newDefaultExistingVMSS("VM_SIZE")
				existingVMSS.VirtualMachineScaleSetProperties.AdditionalCapabilities = &compute.AdditionalCapabilities{UltraSSDEnabled: pointer.Bool(true)}
				existingVMSS.Sku.Capacity = pointer.Int64(2)
				existingVMSS.VirtualMachineScaleSetProperties.AdditionalCapabilities = &compute.AdditionalCapabilities{UltraSSDEnabled: pointer.Bool(true)}
				instances := newDefaultInstances()
				m.Get(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).Return(existingVMSS, nil)
				m.ListInstances(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).Return(instances, nil)

				clone := newDefaultExistingVMSS("VM_SIZE")
				clone.Sku.Capacity = pointer.Int64(3)
				clone.VirtualMachineScaleSetProperties.AdditionalCapabilities = &compute.AdditionalCapabilities{UltraSSDEnabled: pointer.Bool(true)}

				patchVMSS, err := getVMSSUpdateFromVMSS(clone)
				g.Expect(err).NotTo(HaveOccurred())
				patchVMSS.VirtualMachineProfile.StorageProfile.ImageReference.Version = pointer.String("2.0")
				patchVMSS.VirtualMachineProfile.NetworkProfile = nil
				m.UpdateAsync(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName, gomockinternal.DiffEq(patchVMSS)).
					Return(patchFuture, nil)
				s.SetLongRunningOperationState(patchFuture)
				m.GetResultIfDone(gomockinternal.AContext(), patchFuture).Return(compute.VirtualMachineScaleSet{}, azure.NewOperationNotDoneError(patchFuture))
				m.Get(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).Return(clone, nil)
				m.ListInstances(gomockinternal.AContext(), defaultResourceGroup, defaultVMSSName).Return(instances, nil)
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
						UltraSSDEnabled: pointer.Bool(true),
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
						UltraSSDEnabled: pointer.Bool(false),
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
					Enabled:    pointer.Bool(true),
					StorageURI: &storageURI,
				}}

				instances := newDefaultInstances()

				setupDefaultVMSSInProgressOperationDoneExpectations(s, m, vmss, instances)
				s.DeleteLongRunningOperationState(spec.Name, serviceName, infrav1.PutFuture)
				s.DeleteLongRunningOperationState(spec.Name, serviceName, infrav1.PatchFuture)
				s.UpdatePutStatus(infrav1.BootstrapSucceededCondition, serviceName, nil)
				s.Location().AnyTimes().Return("test-location")
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
					Enabled: pointer.Bool(true),
				}}

				instances := newDefaultInstances()

				setupDefaultVMSSInProgressOperationDoneExpectations(s, m, vmss, instances)
				s.DeleteLongRunningOperationState(spec.Name, serviceName, infrav1.PutFuture)
				s.DeleteLongRunningOperationState(spec.Name, serviceName, infrav1.PatchFuture)
				s.UpdatePutStatus(infrav1.BootstrapSucceededCondition, serviceName, nil)
				s.Location().AnyTimes().Return("test-location")
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
					Enabled: pointer.Bool(false),
				}}
				instances := newDefaultInstances()

				setupDefaultVMSSInProgressOperationDoneExpectations(s, m, vmss, instances)
				s.DeleteLongRunningOperationState(spec.Name, serviceName, infrav1.PutFuture)
				s.DeleteLongRunningOperationState(spec.Name, serviceName, infrav1.PatchFuture)
				s.UpdatePutStatus(infrav1.BootstrapSucceededCondition, serviceName, nil)
				s.Location().AnyTimes().Return("test-location")
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
			Name:         pointer.String("VM_SIZE"),
			ResourceType: pointer.String(string(resourceskus.VirtualMachines)),
			Kind:         pointer.String(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: pointer.String("test-location"),
					Zones:    &[]string{"1", "3"},
					ZoneDetails: &[]compute.ResourceSkuZoneDetails{
						{
							Capabilities: &[]compute.ResourceSkuCapabilities{
								{
									Name:  pointer.String("UltraSSDAvailable"),
									Value: pointer.String("True"),
								},
							},
							Name: &[]string{"1", "3"},
						},
					},
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  pointer.String(resourceskus.AcceleratedNetworking),
					Value: pointer.String(string(resourceskus.CapabilityUnsupported)),
				},
				{
					Name:  pointer.String(resourceskus.VCPUs),
					Value: pointer.String("4"),
				},
				{
					Name:  pointer.String(resourceskus.MemoryGB),
					Value: pointer.String("4"),
				},
			},
		},
		{
			Name:         pointer.String("VM_SIZE_AN"),
			ResourceType: pointer.String(string(resourceskus.VirtualMachines)),
			Kind:         pointer.String(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: pointer.String("test-location"),
					Zones:    &[]string{"1", "3"},
					// ZoneDetails: &[]compute.ResourceSkuZoneDetails{
					//    {
					//        	Capabilities: &[]compute.ResourceSkuCapabilities{
					//        		{
					//        			Name:  pointer.String("UltraSSDAvailable"),
					//        			Value: pointer.String("True"),
					//        		},
					//        	},
					//        },
					//    },
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  pointer.String(resourceskus.AcceleratedNetworking),
					Value: pointer.String(string(resourceskus.CapabilitySupported)),
				},
				{
					Name:  pointer.String(resourceskus.VCPUs),
					Value: pointer.String("4"),
				},
				{
					Name:  pointer.String(resourceskus.MemoryGB),
					Value: pointer.String("6"),
				},
			},
		},
		{
			Name:         pointer.String("VM_SIZE_1_CPU"),
			ResourceType: pointer.String(string(resourceskus.VirtualMachines)),
			Kind:         pointer.String(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: pointer.String("test-location"),
					Zones:    &[]string{"1", "3"},
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  pointer.String(resourceskus.AcceleratedNetworking),
					Value: pointer.String(string(resourceskus.CapabilityUnsupported)),
				},
				{
					Name:  pointer.String(resourceskus.VCPUs),
					Value: pointer.String("1"),
				},
				{
					Name:  pointer.String(resourceskus.MemoryGB),
					Value: pointer.String("4"),
				},
			},
		},
		{
			Name:         pointer.String("VM_SIZE_1_MEM"),
			ResourceType: pointer.String(string(resourceskus.VirtualMachines)),
			Kind:         pointer.String(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: pointer.String("test-location"),
					Zones:    &[]string{"1", "3"},
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  pointer.String(resourceskus.AcceleratedNetworking),
					Value: pointer.String(string(resourceskus.CapabilityUnsupported)),
				},
				{
					Name:  pointer.String(resourceskus.VCPUs),
					Value: pointer.String("2"),
				},
				{
					Name:  pointer.String(resourceskus.MemoryGB),
					Value: pointer.String("1"),
				},
			},
		},
		{
			Name:         pointer.String("VM_SIZE_EAH"),
			ResourceType: pointer.String(string(resourceskus.VirtualMachines)),
			Kind:         pointer.String(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: pointer.String("test-location"),
					Zones:    &[]string{"1", "3"},
					//  ZoneDetails: &[]compute.ResourceSkuZoneDetails{
					//	    {
					//    		Capabilities: &[]compute.ResourceSkuCapabilities{
					//    			{
					//    				Name:  pointer.String("UltraSSDAvailable"),
					//    				Value: pointer.String("True"),
					//    			},
					//    		},
					//	    },
					//  },
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  pointer.String(resourceskus.VCPUs),
					Value: pointer.String("4"),
				},
				{
					Name:  pointer.String(resourceskus.MemoryGB),
					Value: pointer.String("8"),
				},
				{
					Name:  pointer.String(resourceskus.EncryptionAtHost),
					Value: pointer.String(string(resourceskus.CapabilitySupported)),
				},
			},
		},
		{
			Name:         pointer.String("VM_SIZE_USSD"),
			ResourceType: pointer.String(string(resourceskus.VirtualMachines)),
			Kind:         pointer.String(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: pointer.String("test-location"),
					Zones:    &[]string{"1", "3"},
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  pointer.String(resourceskus.AcceleratedNetworking),
					Value: pointer.String(string(resourceskus.CapabilitySupported)),
				},
				{
					Name:  pointer.String(resourceskus.VCPUs),
					Value: pointer.String("4"),
				},
				{
					Name:  pointer.String(resourceskus.MemoryGB),
					Value: pointer.String("6"),
				},
			},
		},
		{
			Name:         pointer.String("VM_SIZE_EPH"),
			ResourceType: pointer.String(string(resourceskus.VirtualMachines)),
			Kind:         pointer.String(string(resourceskus.VirtualMachines)),
			Locations: &[]string{
				"test-location",
			},
			LocationInfo: &[]compute.ResourceSkuLocationInfo{
				{
					Location: pointer.String("test-location"),
					Zones:    &[]string{"1", "3"},
					ZoneDetails: &[]compute.ResourceSkuZoneDetails{
						{
							Capabilities: &[]compute.ResourceSkuCapabilities{
								{
									Name:  pointer.String("UltraSSDAvailable"),
									Value: pointer.String("True"),
								},
							},
							Name: &[]string{"1", "3"},
						},
					},
				},
			},
			Capabilities: &[]compute.ResourceSkuCapabilities{
				{
					Name:  pointer.String(resourceskus.AcceleratedNetworking),
					Value: pointer.String(string(resourceskus.CapabilityUnsupported)),
				},
				{
					Name:  pointer.String(resourceskus.VCPUs),
					Value: pointer.String("4"),
				},
				{
					Name:  pointer.String(resourceskus.MemoryGB),
					Value: pointer.String("4"),
				},
				{
					Name:  pointer.String(resourceskus.EphemeralOSDisk),
					Value: pointer.String("True"),
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
			DiskSizeGB: pointer.Int32(120),
			ManagedDisk: &infrav1.ManagedDiskParameters{
				StorageAccountType: "Premium_LRS",
			},
		},
		DataDisks: []infrav1.DataDisk{
			{
				NameSuffix: "my_disk",
				DiskSizeGB: 128,
				Lun:        pointer.Int32(0),
			},
			{
				NameSuffix: "my_disk_with_managed_disk",
				DiskSizeGB: 128,
				Lun:        pointer.Int32(1),
				ManagedDisk: &infrav1.ManagedDiskParameters{
					StorageAccountType: "Standard_LRS",
				},
			},
			{
				NameSuffix: "managed_disk_with_encryption",
				DiskSizeGB: 128,
				Lun:        pointer.Int32(2),
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
		TerminateNotificationTimeout: pointer.Int(7),
		FailureDomains:               []string{"1", "3"},
	}
}

func newWindowsVMSSSpec() azure.ScaleSetSpec {
	vmss := newDefaultVMSSSpec()
	vmss.OSDisk.OSType = azure.WindowsOS
	return vmss
}

func newDefaultExistingVMSS(vmSize string) compute.VirtualMachineScaleSet {
	vmss := newDefaultVMSS(vmSize)
	vmss.ID = pointer.String("subscriptions/1234/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-vm")
	return vmss
}

func newDefaultWindowsVMSS() compute.VirtualMachineScaleSet {
	vmss := newDefaultVMSS("VM_SIZE")
	vmss.VirtualMachineScaleSetProperties.VirtualMachineProfile.StorageProfile.OsDisk.OsType = compute.OperatingSystemTypesWindows
	vmss.VirtualMachineProfile.OsProfile.LinuxConfiguration = nil
	vmss.VirtualMachineProfile.OsProfile.WindowsConfiguration = &compute.WindowsConfiguration{
		EnableAutomaticUpdates: pointer.Bool(false),
	}
	return vmss
}

func newDefaultVMSS(vmSize string) compute.VirtualMachineScaleSet {
	dataDisk := fetchDataDiskBasedOnSize(vmSize)
	return compute.VirtualMachineScaleSet{
		Location: pointer.String("test-location"),
		Tags: map[string]*string{
			"Name": pointer.String(defaultVMSSName),
			"sigs.k8s.io_cluster-api-provider-azure_cluster_my-cluster": pointer.String("owned"),
			"sigs.k8s.io_cluster-api-provider-azure_role":               pointer.String("node"),
		},
		Sku: &compute.Sku{
			Name:     pointer.String(vmSize),
			Tier:     pointer.String("Standard"),
			Capacity: pointer.Int64(2),
		},
		Zones: &[]string{"1", "3"},
		VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
			SinglePlacementGroup: pointer.Bool(false),
			UpgradePolicy: &compute.UpgradePolicy{
				Mode: compute.UpgradeModeManual,
			},
			Overprovision:     pointer.Bool(false),
			OrchestrationMode: compute.OrchestrationModeUniform,
			VirtualMachineProfile: &compute.VirtualMachineScaleSetVMProfile{
				OsProfile: &compute.VirtualMachineScaleSetOSProfile{
					ComputerNamePrefix: pointer.String(defaultVMSSName),
					AdminUsername:      pointer.String(azure.DefaultUserName),
					CustomData:         pointer.String("fake-bootstrap-data"),
					LinuxConfiguration: &compute.LinuxConfiguration{
						SSH: &compute.SSHConfiguration{
							PublicKeys: &[]compute.SSHPublicKey{
								{
									Path:    pointer.String("/home/capi/.ssh/authorized_keys"),
									KeyData: pointer.String("fakesshkey\n"),
								},
							},
						},
						DisablePasswordAuthentication: pointer.Bool(true),
					},
				},
				ScheduledEventsProfile: &compute.ScheduledEventsProfile{
					TerminateNotificationProfile: &compute.TerminateNotificationProfile{
						NotBeforeTimeout: pointer.String("PT7M"),
						Enable:           pointer.Bool(true),
					},
				},
				StorageProfile: &compute.VirtualMachineScaleSetStorageProfile{
					ImageReference: &compute.ImageReference{
						Publisher: pointer.String("fake-publisher"),
						Offer:     pointer.String("my-offer"),
						Sku:       pointer.String("sku-id"),
						Version:   pointer.String("1.0"),
					},
					OsDisk: &compute.VirtualMachineScaleSetOSDisk{
						OsType:       "Linux",
						CreateOption: "FromImage",
						DiskSizeGB:   pointer.Int32(120),
						ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
							StorageAccountType: "Premium_LRS",
						},
					},
					DataDisks: dataDisk,
				},
				DiagnosticsProfile: &compute.DiagnosticsProfile{
					BootDiagnostics: &compute.BootDiagnostics{
						Enabled: pointer.Bool(true),
					},
				},
				NetworkProfile: &compute.VirtualMachineScaleSetNetworkProfile{
					NetworkInterfaceConfigurations: &[]compute.VirtualMachineScaleSetNetworkConfiguration{
						{
							Name: pointer.String("my-vmss"),
							VirtualMachineScaleSetNetworkConfigurationProperties: &compute.VirtualMachineScaleSetNetworkConfigurationProperties{
								Primary:                     pointer.Bool(true),
								EnableAcceleratedNetworking: pointer.Bool(false),
								EnableIPForwarding:          pointer.Bool(true),
								IPConfigurations: &[]compute.VirtualMachineScaleSetIPConfiguration{
									{
										Name: pointer.String("my-vmss"),
										VirtualMachineScaleSetIPConfigurationProperties: &compute.VirtualMachineScaleSetIPConfigurationProperties{
											Subnet: &compute.APIEntityReference{
												ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/virtualNetworks/my-vnet/subnets/my-subnet"),
											},
											Primary:                         pointer.Bool(true),
											PrivateIPAddressVersion:         compute.IPVersionIPv4,
											LoadBalancerBackendAddressPools: &[]compute.SubResource{{ID: pointer.String("/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Network/loadBalancers/capz-lb/backendAddressPools/backendPool")}},
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
							Name: pointer.String("someExtension"),
							VirtualMachineScaleSetExtensionProperties: &compute.VirtualMachineScaleSetExtensionProperties{
								Publisher:          pointer.String("somePublisher"),
								Type:               pointer.String("someExtension"),
								TypeHandlerVersion: pointer.String("someVersion"),
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
				Lun:          pointer.Int32(0),
				Name:         pointer.String("my-vmss_my_disk"),
				CreateOption: "Empty",
				DiskSizeGB:   pointer.Int32(128),
			},
			{
				Lun:          pointer.Int32(1),
				Name:         pointer.String("my-vmss_my_disk_with_managed_disk"),
				CreateOption: "Empty",
				DiskSizeGB:   pointer.Int32(128),
				ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
					StorageAccountType: "Standard_LRS",
				},
			},
			{
				Lun:          pointer.Int32(2),
				Name:         pointer.String("my-vmss_managed_disk_with_encryption"),
				CreateOption: "Empty",
				DiskSizeGB:   pointer.Int32(128),
				ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
					StorageAccountType: "Standard_LRS",
					DiskEncryptionSet: &compute.DiskEncryptionSetParameters{
						ID: pointer.String("encryption_id"),
					},
				},
			},
			{
				Lun:          pointer.Int32(3),
				Name:         pointer.String("my-vmss_my_disk_with_ultra_disks"),
				CreateOption: "Empty",
				DiskSizeGB:   pointer.Int32(128),
				ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
					StorageAccountType: "UltraSSD_LRS",
				},
			},
		}
	} else {
		dataDisk = &[]compute.VirtualMachineScaleSetDataDisk{
			{
				Lun:          pointer.Int32(0),
				Name:         pointer.String("my-vmss_my_disk"),
				CreateOption: "Empty",
				DiskSizeGB:   pointer.Int32(128),
			},
			{
				Lun:          pointer.Int32(1),
				Name:         pointer.String("my-vmss_my_disk_with_managed_disk"),
				CreateOption: "Empty",
				DiskSizeGB:   pointer.Int32(128),
				ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
					StorageAccountType: "Standard_LRS",
				},
			},
			{
				Lun:          pointer.Int32(2),
				Name:         pointer.String("my-vmss_managed_disk_with_encryption"),
				CreateOption: "Empty",
				DiskSizeGB:   pointer.Int32(128),
				ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
					StorageAccountType: "Standard_LRS",
					DiskEncryptionSet: &compute.DiskEncryptionSetParameters{
						ID: pointer.String("encryption_id"),
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
			ID:         pointer.String("my-vm-id"),
			InstanceID: pointer.String("my-vm-1"),
			Name:       pointer.String("my-vm"),
			VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
				ProvisioningState: pointer.String("Succeeded"),
				OsProfile: &compute.OSProfile{
					ComputerName: pointer.String("instance-000001"),
				},
				StorageProfile: &compute.StorageProfile{
					ImageReference: &compute.ImageReference{
						Publisher: pointer.String("fake-publisher"),
						Offer:     pointer.String("my-offer"),
						Sku:       pointer.String("sku-id"),
						Version:   pointer.String("1.0"),
					},
				},
			},
		},
		{
			ID:         pointer.String("my-vm-id"),
			InstanceID: pointer.String("my-vm-2"),
			Name:       pointer.String("my-vm"),
			VirtualMachineScaleSetVMProperties: &compute.VirtualMachineScaleSetVMProperties{
				ProvisioningState: pointer.String("Succeeded"),
				OsProfile: &compute.OSProfile{
					ComputerName: pointer.String("instance-000002"),
				},
				StorageProfile: &compute.StorageProfile{
					ImageReference: &compute.ImageReference{
						Publisher: pointer.String("fake-publisher"),
						Offer:     pointer.String("my-offer"),
						Sku:       pointer.String("sku-id"),
						Version:   pointer.String("1.0"),
					},
				},
			},
		},
	}
}

func setupDefaultVMSSInProgressOperationDoneExpectations(s *mock_scalesets.MockScaleSetScopeMockRecorder, m *mock_scalesets.MockClientMockRecorder, createdVMSS compute.VirtualMachineScaleSet, instances []compute.VirtualMachineScaleSetVM) {
	createdVMSS.ID = pointer.String("subscriptions/1234/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-vm")
	createdVMSS.ProvisioningState = pointer.String(string(infrav1.Succeeded))
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
	s.SetProviderID(azure.ProviderIDPrefix + *createdVMSS.ID)
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
	s.SetProviderID(azure.ProviderIDPrefix + *vmss.ID)
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
	s.SetProviderID(azure.ProviderIDPrefix + "subscriptions/1234/resourceGroups/my_resource_group/providers/Microsoft.Compute/virtualMachines/my-vm")
	s.GetLongRunningOperationState(defaultVMSSName, serviceName, infrav1.PutFuture).Return(nil)
	s.GetLongRunningOperationState(defaultVMSSName, serviceName, infrav1.PatchFuture).Return(nil)
	s.MaxSurge().Return(1, nil)
	s.SetVMSSState(gomock.Any())
}

/*
Copyright 2021 The Kubernetes Authors.

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

package v1beta1

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	validSSHPublicKey = generateSSHPublicKey(true)
	validOSDisk       = generateValidOSDisk()
)

func TestAzureMachine_ValidateCreate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name    string
		machine *AzureMachine
		wantErr bool
	}{
		{
			name:    "azuremachine with marketplace image - full",
			machine: createMachineWithMarketPlaceImage("PUB1234", "OFFER1234", "SKU1234", "1.0.0"),
			wantErr: false,
		},
		{
			name:    "azuremachine with marketplace image - missing publisher",
			machine: createMachineWithMarketPlaceImage("", "OFFER1235", "SKU1235", "2.0.0"),
			wantErr: true,
		},
		{
			name:    "azuremachine with shared gallery image - full",
			machine: createMachineWithSharedImage("SUB123", "RG123", "NAME123", "GALLERY1", "1.0.0"),
			wantErr: false,
		},
		{
			name:    "azuremachine with marketplace image - missing subscription",
			machine: createMachineWithSharedImage("", "RG124", "NAME124", "GALLERY1", "2.0.0"),
			wantErr: true,
		},
		{
			name:    "azuremachine with image by - with id",
			machine: createMachineWithImageByID("ID123"),
			wantErr: false,
		},
		{
			name:    "azuremachine with image by - without id",
			machine: createMachineWithImageByID(""),
			wantErr: true,
		},
		{
			name:    "azuremachine with valid SSHPublicKey",
			machine: createMachineWithSSHPublicKey(validSSHPublicKey),
			wantErr: false,
		},
		{
			name:    "azuremachine without SSHPublicKey",
			machine: createMachineWithSSHPublicKey(""),
			wantErr: true,
		},
		{
			name:    "azuremachine with invalid SSHPublicKey",
			machine: createMachineWithSSHPublicKey("invalid ssh key"),
			wantErr: true,
		},
		{
			name: "azuremachine with list of user-assigned identities",
			machine: createMachineWithUserAssignedIdentities([]UserAssignedIdentity{
				{ProviderID: "azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Compute/virtualMachines/default-12345-control-plane-9d5x5"},
				{ProviderID: "azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Compute/virtualMachines/default-12345-control-plane-a1b2c"},
			}),
			wantErr: false,
		},
		{
			name:    "azuremachine with empty list of user-assigned identities",
			machine: createMachineWithUserAssignedIdentities([]UserAssignedIdentity{}),
			wantErr: true,
		},
		{
			name:    "azuremachine with valid osDisk cache type",
			machine: createMachineWithOsDiskCacheType(string(armcompute.PossibleCachingTypesValues()[1])),
			wantErr: false,
		},
		{
			name:    "azuremachine with invalid osDisk cache type",
			machine: createMachineWithOsDiskCacheType("invalid_cache_type"),
			wantErr: true,
		},
		{
			name:    "azuremachinepool with managed diagnostics profile",
			machine: createMachineWithDiagnostics(ManagedDiagnosticsStorage, nil),
			wantErr: false,
		},
		{
			name:    "azuremachine with disabled diagnostics profile",
			machine: createMachineWithDiagnostics(ManagedDiagnosticsStorage, nil),
			wantErr: false,
		},
		{
			name:    "azuremachine with user managed diagnostics profile and defined user managed storage account",
			machine: createMachineWithDiagnostics(UserManagedDiagnosticsStorage, &UserManagedBootDiagnostics{StorageAccountURI: "https://fakeurl"}),
			wantErr: false,
		},
		{
			name:    "azuremachine with empty diagnostics profile",
			machine: createMachineWithDiagnostics("", nil),
			wantErr: false,
		},
		{
			name:    "azuremachine with user managed diagnostics profile, but empty user managed storage account",
			machine: createMachineWithDiagnostics(UserManagedDiagnosticsStorage, nil),
			wantErr: true,
		},
		{
			name:    "azuremachine with invalid network configuration",
			machine: createMachineWithNetworkConfig("subnet", nil, []NetworkInterface{{SubnetName: "subnet1"}}),
			wantErr: true,
		},
		{
			name:    "azuremachine with valid legacy network configuration",
			machine: createMachineWithNetworkConfig("subnet", nil, []NetworkInterface{}),
			wantErr: false,
		},
		{
			name:    "azuremachine with valid network configuration",
			machine: createMachineWithNetworkConfig("", nil, []NetworkInterface{{SubnetName: "subnet", PrivateIPConfigs: 1}}),
			wantErr: false,
		},
		{
			name:    "azuremachine without confidential compute properties and encryption at host enabled",
			machine: createMachineWithConfidentialCompute("", "", true, false, false),
			wantErr: false,
		},
		{
			name:    "azuremachine with confidential compute VMGuestStateOnly encryption and encryption at host enabled",
			machine: createMachineWithConfidentialCompute(SecurityEncryptionTypeVMGuestStateOnly, SecurityTypesConfidentialVM, true, false, false),
			wantErr: true,
		},
		{
			name:    "azuremachine with confidential compute DiskWithVMGuestState encryption and encryption at host enabled",
			machine: createMachineWithConfidentialCompute(SecurityEncryptionTypeDiskWithVMGuestState, SecurityTypesConfidentialVM, true, true, true),
			wantErr: true,
		},
		{
			name:    "azuremachine with confidential compute VMGuestStateOnly encryption, vTPM and SecureBoot enabled",
			machine: createMachineWithConfidentialCompute(SecurityEncryptionTypeVMGuestStateOnly, SecurityTypesConfidentialVM, false, true, true),
			wantErr: false,
		},
		{
			name:    "azuremachine with confidential compute VMGuestStateOnly encryption enabled, vTPM enabled and SecureBoot disabled",
			machine: createMachineWithConfidentialCompute(SecurityEncryptionTypeVMGuestStateOnly, SecurityTypesConfidentialVM, false, true, false),
			wantErr: false,
		},
		{
			name:    "azuremachine with confidential compute VMGuestStateOnly encryption enabled, vTPM disabled and SecureBoot enabled",
			machine: createMachineWithConfidentialCompute(SecurityEncryptionTypeVMGuestStateOnly, SecurityTypesConfidentialVM, false, false, true),
			wantErr: true,
		},
		{
			name:    "azuremachine with confidential compute VMGuestStateOnly encryption enabled, vTPM enabled, SecureBoot disabled and SecurityType empty",
			machine: createMachineWithConfidentialCompute(SecurityEncryptionTypeVMGuestStateOnly, "", false, true, false),
			wantErr: true,
		},
		{
			name:    "azuremachine with confidential compute VMGuestStateOnly encryption enabled, vTPM and SecureBoot empty",
			machine: createMachineWithConfidentialCompute(SecurityEncryptionTypeVMGuestStateOnly, SecurityTypesConfidentialVM, false, false, false),
			wantErr: true,
		},
		{
			name:    "azuremachine with confidential compute DiskWithVMGuestState encryption, vTPM and SecureBoot enabled",
			machine: createMachineWithConfidentialCompute(SecurityEncryptionTypeDiskWithVMGuestState, SecurityTypesConfidentialVM, false, true, true),
			wantErr: false,
		},
		{
			name:    "azuremachine with confidential compute DiskWithVMGuestState encryption enabled, vTPM enabled and SecureBoot disabled",
			machine: createMachineWithConfidentialCompute(SecurityEncryptionTypeDiskWithVMGuestState, SecurityTypesConfidentialVM, false, true, false),
			wantErr: true,
		},
		{
			name:    "azuremachine with confidential compute DiskWithVMGuestState encryption enabled, vTPM disabled and SecureBoot enabled",
			machine: createMachineWithConfidentialCompute(SecurityEncryptionTypeDiskWithVMGuestState, SecurityTypesConfidentialVM, false, false, true),
			wantErr: true,
		},
		{
			name:    "azuremachine with confidential compute DiskWithVMGuestState encryption enabled, vTPM disabled and SecureBoot disabled",
			machine: createMachineWithConfidentialCompute(SecurityEncryptionTypeDiskWithVMGuestState, SecurityTypesConfidentialVM, false, false, false),
			wantErr: true,
		},
		{
			name:    "azuremachine with confidential compute DiskWithVMGuestState encryption enabled, vTPM enabled, SecureBoot disabled and SecurityType empty",
			machine: createMachineWithConfidentialCompute(SecurityEncryptionTypeDiskWithVMGuestState, "", false, true, false),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mw := &azureMachineWebhook{}
			_, err := mw.ValidateCreate(context.Background(), tc.machine)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureMachine_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name       string
		oldMachine *AzureMachine
		newMachine *AzureMachine
		wantErr    bool
	}{
		{
			name: "invalidTest: azuremachine.spec.image is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Image: &Image{
						ID: ptr.To("imageID-1"),
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Image: &Image{
						ID: ptr.To("imageID-2"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.image is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Image: &Image{
						ID: ptr.To("imageID-1"),
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Image: &Image{
						ID: ptr.To("imageID-1"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.Identity is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Identity: VMIdentityUserAssigned,
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Identity: VMIdentityNone,
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.Identity is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Identity: VMIdentityNone,
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Identity: VMIdentityNone,
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.UserAssignedIdentities is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					UserAssignedIdentities: []UserAssignedIdentity{
						{ProviderID: "providerID-1"},
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					UserAssignedIdentities: []UserAssignedIdentity{
						{ProviderID: "providerID-2"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.UserAssignedIdentities is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					UserAssignedIdentities: []UserAssignedIdentity{
						{ProviderID: "providerID-1"},
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					UserAssignedIdentities: []UserAssignedIdentity{
						{ProviderID: "providerID-1"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.RoleAssignmentName is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					RoleAssignmentName: "role",
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					RoleAssignmentName: "not-role",
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.RoleAssignmentName is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					RoleAssignmentName: "role",
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					RoleAssignmentName: "role",
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.RoleAssignmentName is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SystemAssignedIdentityRole: &SystemAssignedIdentityRole{
						Name:         "role",
						Scope:        "scope",
						DefinitionID: "definitionID",
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SystemAssignedIdentityRole: &SystemAssignedIdentityRole{
						Name:         "not-role",
						Scope:        "scope",
						DefinitionID: "definitionID",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.SystemAssignedIdentityRole is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SystemAssignedIdentityRole: &SystemAssignedIdentityRole{
						Name:         "role",
						Scope:        "scope",
						DefinitionID: "definitionID",
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SystemAssignedIdentityRole: &SystemAssignedIdentityRole{
						Name:         "role",
						Scope:        "scope",
						DefinitionID: "definitionID",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.OSDisk is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					OSDisk: OSDisk{
						OSType: "osType-0",
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					OSDisk: OSDisk{
						OSType: "osType-1",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.OSDisk is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					OSDisk: OSDisk{
						OSType: "osType-1",
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					OSDisk: OSDisk{
						OSType: "osType-1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.DataDisks is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					DataDisks: []DataDisk{
						{
							DiskSizeGB: 128,
						},
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					DataDisks: []DataDisk{
						{
							DiskSizeGB: 64,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.DataDisks is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					DataDisks: []DataDisk{
						{
							DiskSizeGB: 128,
						},
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					DataDisks: []DataDisk{
						{
							DiskSizeGB: 128,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.SSHPublicKey is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SSHPublicKey: "validKey",
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SSHPublicKey: "invalidKey",
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.SSHPublicKey is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SSHPublicKey: "validKey",
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SSHPublicKey: "validKey",
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.AllocatePublicIP is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					AllocatePublicIP: true,
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					AllocatePublicIP: false,
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.AllocatePublicIP is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					AllocatePublicIP: true,
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					AllocatePublicIP: true,
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.EnableIPForwarding is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					EnableIPForwarding: true,
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					EnableIPForwarding: false,
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.EnableIPForwarding is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					EnableIPForwarding: true,
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					EnableIPForwarding: true,
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.AcceleratedNetworking is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					AcceleratedNetworking: ptr.To(true),
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					AcceleratedNetworking: ptr.To(false),
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.AcceleratedNetworking is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					AcceleratedNetworking: ptr.To(true),
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					AcceleratedNetworking: ptr.To(true),
				},
			},
			wantErr: false,
		},
		{
			name: "validTest: azuremachine.spec.AcceleratedNetworking transition(from true) to nil is acceptable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					AcceleratedNetworking: ptr.To(true),
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					AcceleratedNetworking: nil,
				},
			},
			wantErr: false,
		},
		{
			name: "validTest: azuremachine.spec.AcceleratedNetworking transition(from false) to nil is acceptable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					AcceleratedNetworking: ptr.To(false),
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					AcceleratedNetworking: nil,
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.SpotVMOptions is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SpotVMOptions: &SpotVMOptions{
						MaxPrice: &resource.Quantity{Format: "vmoptions-0"},
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SpotVMOptions: &SpotVMOptions{
						MaxPrice: &resource.Quantity{Format: "vmoptions-1"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.SpotVMOptions is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SpotVMOptions: &SpotVMOptions{
						MaxPrice: &resource.Quantity{Format: "vmoptions-1"},
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SpotVMOptions: &SpotVMOptions{
						MaxPrice: &resource.Quantity{Format: "vmoptions-1"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.SecurityProfile is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SecurityProfile: &SecurityProfile{EncryptionAtHost: ptr.To(true)},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SecurityProfile: &SecurityProfile{EncryptionAtHost: ptr.To(false)},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.SecurityProfile is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SecurityProfile: &SecurityProfile{EncryptionAtHost: ptr.To(true)},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					SecurityProfile: &SecurityProfile{EncryptionAtHost: ptr.To(true)},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.Diagnostics is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Diagnostics: &Diagnostics{Boot: &BootDiagnostics{StorageAccountType: DisabledDiagnosticsStorage}},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Diagnostics: &Diagnostics{Boot: &BootDiagnostics{StorageAccountType: ManagedDiagnosticsStorage}},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.Diagnostics is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Diagnostics: &Diagnostics{Boot: &BootDiagnostics{StorageAccountType: DisabledDiagnosticsStorage}},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Diagnostics: &Diagnostics{Boot: &BootDiagnostics{StorageAccountType: DisabledDiagnosticsStorage}},
				},
			},
			wantErr: false,
		},
		{
			name: "validTest: azuremachine.spec.Diagnostics should not error on updating nil diagnostics",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Diagnostics: &Diagnostics{Boot: &BootDiagnostics{StorageAccountType: ManagedDiagnosticsStorage}},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.Diagnostics is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Diagnostics: &Diagnostics{},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Diagnostics: &Diagnostics{Boot: &BootDiagnostics{StorageAccountType: ManagedDiagnosticsStorage}},
				},
			},
			wantErr: true,
		},
		{
			name: "invalidTest: azuremachine.spec.Diagnostics is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Diagnostics: &Diagnostics{
						Boot: &BootDiagnostics{},
					},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					Diagnostics: &Diagnostics{Boot: &BootDiagnostics{StorageAccountType: ManagedDiagnosticsStorage}},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.networkInterfaces is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					NetworkInterfaces: []NetworkInterface{{SubnetName: "subnet"}},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					NetworkInterfaces: []NetworkInterface{{SubnetName: "subnet"}},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidtest: azuremachine.spec.networkInterfaces is immutable",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					NetworkInterfaces: []NetworkInterface{{SubnetName: "subnet1"}},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					NetworkInterfaces: []NetworkInterface{{SubnetName: "subnet2"}},
				},
			},
			wantErr: true,
		},
		{
			name: "invalidtest: updating subnet name from empty to non empty",
			oldMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					NetworkInterfaces: []NetworkInterface{{SubnetName: ""}},
				},
			},
			newMachine: &AzureMachine{
				Spec: AzureMachineSpec{
					NetworkInterfaces: []NetworkInterface{{SubnetName: "subnet1"}},
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mw := &azureMachineWebhook{}
			_, err := mw.ValidateUpdate(context.Background(), tc.oldMachine, tc.newMachine)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

type mockDefaultClient struct {
	client.Client
	SubscriptionID string
}

func (m mockDefaultClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	switch obj := obj.(type) {
	case *AzureCluster:
		obj.Spec.SubscriptionID = m.SubscriptionID
	case *clusterv1.Cluster:
		obj.Spec.InfrastructureRef = &corev1.ObjectReference{
			Kind: "AzureCluster",
			Name: "test-cluster",
		}
	default:
		return errors.New("invalid object type")
	}
	return nil
}

func TestAzureMachine_Default(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		machine *AzureMachine
	}

	testSubscriptionID := "test-subscription-id"
	mockClient := mockDefaultClient{SubscriptionID: testSubscriptionID}
	existingPublicKey := validSSHPublicKey
	publicKeyExistTest := test{machine: createMachineWithSSHPublicKey(existingPublicKey)}
	publicKeyNotExistTest := test{machine: createMachineWithSSHPublicKey("")}
	testObjectMeta := metav1.ObjectMeta{
		Labels: map[string]string{
			clusterv1.ClusterNameLabel: "test-cluster",
		},
	}

	mw := &azureMachineWebhook{
		Client: mockClient,
	}

	err := mw.Default(context.Background(), publicKeyExistTest.machine)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(publicKeyExistTest.machine.Spec.SSHPublicKey).To(Equal(existingPublicKey))

	err = mw.Default(context.Background(), publicKeyNotExistTest.machine)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(publicKeyNotExistTest.machine.Spec.SSHPublicKey).To(Not(BeEmpty()))

	cacheTypeNotSpecifiedTest := test{machine: &AzureMachine{ObjectMeta: testObjectMeta, Spec: AzureMachineSpec{OSDisk: OSDisk{CachingType: ""}}}}
	err = mw.Default(context.Background(), cacheTypeNotSpecifiedTest.machine)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cacheTypeNotSpecifiedTest.machine.Spec.OSDisk.CachingType).To(Equal("None"))

	for _, possibleCachingType := range armcompute.PossibleCachingTypesValues() {
		cacheTypeSpecifiedTest := test{machine: &AzureMachine{ObjectMeta: testObjectMeta, Spec: AzureMachineSpec{OSDisk: OSDisk{CachingType: string(possibleCachingType)}}}}
		err = mw.Default(context.Background(), cacheTypeSpecifiedTest.machine)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(cacheTypeSpecifiedTest.machine.Spec.OSDisk.CachingType).To(Equal(string(possibleCachingType)))
	}
}

func createMachineWithNetworkConfig(subnetName string, acceleratedNetworking *bool, interfaces []NetworkInterface) *AzureMachine {
	return &AzureMachine{
		Spec: AzureMachineSpec{
			SubnetName:            subnetName,
			NetworkInterfaces:     interfaces,
			AcceleratedNetworking: acceleratedNetworking,
			OSDisk:                validOSDisk,
			SSHPublicKey:          validSSHPublicKey,
		},
	}
}

func createMachineWithSharedImage(subscriptionID, resourceGroup, name, gallery, version string) *AzureMachine {
	image := &Image{
		SharedGallery: &AzureSharedGalleryImage{
			SubscriptionID: subscriptionID,
			ResourceGroup:  resourceGroup,
			Name:           name,
			Gallery:        gallery,
			Version:        version,
		},
	}

	return &AzureMachine{
		Spec: AzureMachineSpec{
			Image:        image,
			SSHPublicKey: validSSHPublicKey,
			OSDisk:       validOSDisk,
		},
	}
}

func createMachineWithMarketPlaceImage(publisher, offer, sku, version string) *AzureMachine {
	image := &Image{
		Marketplace: &AzureMarketplaceImage{
			ImagePlan: ImagePlan{
				Publisher: publisher,
				Offer:     offer,
				SKU:       sku,
			},
			Version: version,
		},
	}

	return &AzureMachine{
		Spec: AzureMachineSpec{
			Image:        image,
			SSHPublicKey: validSSHPublicKey,
			OSDisk:       validOSDisk,
		},
	}
}

func createMachineWithImageByID(imageID string) *AzureMachine {
	image := &Image{
		ID: &imageID,
	}

	return &AzureMachine{
		Spec: AzureMachineSpec{
			Image:        image,
			SSHPublicKey: validSSHPublicKey,
			OSDisk:       validOSDisk,
		},
	}
}

func createMachineWithOsDiskCacheType(cacheType string) *AzureMachine {
	machine := &AzureMachine{
		Spec: AzureMachineSpec{
			SSHPublicKey: validSSHPublicKey,
			OSDisk:       validOSDisk,
		},
	}
	machine.Spec.OSDisk.CachingType = cacheType
	return machine
}

func createMachineWithSystemAssignedIdentityRoleName() *AzureMachine {
	machine := &AzureMachine{
		Spec: AzureMachineSpec{
			SSHPublicKey: validSSHPublicKey,
			OSDisk:       validOSDisk,
			Identity:     VMIdentitySystemAssigned,
			SystemAssignedIdentityRole: &SystemAssignedIdentityRole{
				Name:         "c6e3443d-bc11-4335-8819-ab6637b10586",
				Scope:        "test-scope",
				DefinitionID: "test-definition-id",
			},
		},
	}
	return machine
}

func createMachineWithoutSystemAssignedIdentityRoleName() *AzureMachine {
	machine := &AzureMachine{
		Spec: AzureMachineSpec{
			SSHPublicKey: validSSHPublicKey,
			OSDisk:       validOSDisk,
			Identity:     VMIdentitySystemAssigned,
			SystemAssignedIdentityRole: &SystemAssignedIdentityRole{
				Scope:        "test-scope",
				DefinitionID: "test-definition-id",
			},
		},
	}
	return machine
}

func createMachineWithoutRoleAssignmentName() *AzureMachine {
	machine := &AzureMachine{
		Spec: AzureMachineSpec{
			SSHPublicKey: validSSHPublicKey,
			OSDisk:       validOSDisk,
		},
	}
	return machine
}

func createMachineWithRoleAssignmentName() *AzureMachine {
	machine := &AzureMachine{
		Spec: AzureMachineSpec{
			SSHPublicKey:       validSSHPublicKey,
			OSDisk:             validOSDisk,
			RoleAssignmentName: "test-role-assignment",
		},
	}
	return machine
}

func createMachineWithDiagnostics(diagnosticsType BootDiagnosticsStorageAccountType, userManaged *UserManagedBootDiagnostics) *AzureMachine {
	var diagnostics *Diagnostics

	if diagnosticsType != "" {
		diagnostics = &Diagnostics{
			Boot: &BootDiagnostics{
				StorageAccountType: diagnosticsType,
			},
		}
	}

	if userManaged != nil {
		diagnostics.Boot.UserManaged = userManaged
	}

	return &AzureMachine{
		Spec: AzureMachineSpec{
			SSHPublicKey: validSSHPublicKey,
			OSDisk:       validOSDisk,
			Diagnostics:  diagnostics,
		},
	}
}

func createMachineWithConfidentialCompute(securityEncryptionType SecurityEncryptionType, securityType SecurityTypes, encryptionAtHost, vTpmEnabled, secureBootEnabled bool) *AzureMachine {
	securityProfile := &SecurityProfile{
		EncryptionAtHost: &encryptionAtHost,
		SecurityType:     securityType,
		UefiSettings: &UefiSettings{
			VTpmEnabled:       &vTpmEnabled,
			SecureBootEnabled: &secureBootEnabled,
		},
	}

	osDisk := OSDisk{
		DiskSizeGB: ptr.To[int32](30),
		OSType:     LinuxOS,
		ManagedDisk: &ManagedDiskParameters{
			StorageAccountType: "Premium_LRS",
			SecurityProfile: &VMDiskSecurityProfile{
				SecurityEncryptionType: securityEncryptionType,
			},
		},
		CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
	}

	return &AzureMachine{
		Spec: AzureMachineSpec{
			SSHPublicKey:    validSSHPublicKey,
			OSDisk:          osDisk,
			SecurityProfile: securityProfile,
		},
	}
}

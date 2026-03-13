/*
Copyright The Kubernetes Authors.

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

package webhooks

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	apifixtures "sigs.k8s.io/cluster-api-provider-azure/internal/test/apifixtures"
)

var (
	validSSHPublicKey = apifixtures.GenerateSSHPublicKey(true)
	validOSDisk       = apifixtures.GenerateValidOSDisk()
)

func TestAzureMachine_ValidateCreate(t *testing.T) {
	tests := []struct {
		name    string
		machine *infrav1.AzureMachine
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
			machine: apifixtures.CreateMachineWithSSHPublicKey(validSSHPublicKey),
			wantErr: false,
		},
		{
			name:    "azuremachine without SSHPublicKey",
			machine: apifixtures.CreateMachineWithSSHPublicKey(""),
			wantErr: true,
		},
		{
			name:    "azuremachine with invalid SSHPublicKey",
			machine: apifixtures.CreateMachineWithSSHPublicKey("invalid ssh key"),
			wantErr: true,
		},
		{
			name: "azuremachine with list of user-assigned identities",
			machine: apifixtures.CreateMachineWithUserAssignedIdentities([]infrav1.UserAssignedIdentity{
				{ProviderID: "azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Compute/virtualMachines/default-12345-control-plane-9d5x5"},
				{ProviderID: "azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Compute/virtualMachines/default-12345-control-plane-a1b2c"},
			}),
			wantErr: false,
		},
		{
			name: "azuremachine with list of user-assigned identities with wrong identity type",
			machine: apifixtures.CreateMachineWithUserAssignedIdentitiesWithBadIdentity([]infrav1.UserAssignedIdentity{
				{ProviderID: "azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Compute/virtualMachines/default-12345-control-plane-9d5x5"},
				{ProviderID: "azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Compute/virtualMachines/default-12345-control-plane-a1b2c"},
			}),
			wantErr: true,
		},
		{
			name:    "azuremachine with empty list of user-assigned identities",
			machine: apifixtures.CreateMachineWithUserAssignedIdentities([]infrav1.UserAssignedIdentity{}),
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
			machine: createMachineWithDiagnostics(infrav1.ManagedDiagnosticsStorage, nil),
			wantErr: false,
		},
		{
			name:    "azuremachine with disabled diagnostics profile",
			machine: createMachineWithDiagnostics(infrav1.ManagedDiagnosticsStorage, nil),
			wantErr: false,
		},
		{
			name:    "azuremachine with user managed diagnostics profile and defined user managed storage account",
			machine: createMachineWithDiagnostics(infrav1.UserManagedDiagnosticsStorage, &infrav1.UserManagedBootDiagnostics{StorageAccountURI: "https://fakeurl"}),
			wantErr: false,
		},
		{
			name:    "azuremachine with empty diagnostics profile",
			machine: createMachineWithDiagnostics("", nil),
			wantErr: false,
		},
		{
			name:    "azuremachine with user managed diagnostics profile, but empty user managed storage account",
			machine: createMachineWithDiagnostics(infrav1.UserManagedDiagnosticsStorage, nil),
			wantErr: true,
		},
		{
			name:    "azuremachine with invalid network configuration",
			machine: createMachineWithNetworkConfig("subnet", nil, []infrav1.NetworkInterface{{SubnetName: "subnet1"}}),
			wantErr: true,
		},
		{
			name:    "azuremachine with valid legacy network configuration",
			machine: createMachineWithNetworkConfig("subnet", nil, []infrav1.NetworkInterface{}),
			wantErr: false,
		},
		{
			name:    "azuremachine with valid network configuration",
			machine: createMachineWithNetworkConfig("", nil, []infrav1.NetworkInterface{{SubnetName: "subnet", PrivateIPConfigs: 1}}),
			wantErr: false,
		},
		{
			name:    "azuremachine without confidential compute properties and encryption at host enabled",
			machine: createMachineWithConfidentialCompute("", "", true, false, false),
			wantErr: false,
		},
		{
			name:    "azuremachine with confidential compute VMGuestStateOnly encryption and encryption at host enabled",
			machine: createMachineWithConfidentialCompute(infrav1.SecurityEncryptionTypeVMGuestStateOnly, infrav1.SecurityTypesConfidentialVM, true, false, false),
			wantErr: true,
		},
		{
			name:    "azuremachine with confidential compute DiskWithVMGuestState encryption and encryption at host enabled",
			machine: createMachineWithConfidentialCompute(infrav1.SecurityEncryptionTypeDiskWithVMGuestState, infrav1.SecurityTypesConfidentialVM, true, true, true),
			wantErr: true,
		},
		{
			name:    "azuremachine with confidential compute VMGuestStateOnly encryption, vTPM and SecureBoot enabled",
			machine: createMachineWithConfidentialCompute(infrav1.SecurityEncryptionTypeVMGuestStateOnly, infrav1.SecurityTypesConfidentialVM, false, true, true),
			wantErr: false,
		},
		{
			name:    "azuremachine with confidential compute VMGuestStateOnly encryption enabled, vTPM enabled and SecureBoot disabled",
			machine: createMachineWithConfidentialCompute(infrav1.SecurityEncryptionTypeVMGuestStateOnly, infrav1.SecurityTypesConfidentialVM, false, true, false),
			wantErr: false,
		},
		{
			name:    "azuremachine with confidential compute VMGuestStateOnly encryption enabled, vTPM disabled and SecureBoot enabled",
			machine: createMachineWithConfidentialCompute(infrav1.SecurityEncryptionTypeVMGuestStateOnly, infrav1.SecurityTypesConfidentialVM, false, false, true),
			wantErr: true,
		},
		{
			name:    "azuremachine with confidential compute VMGuestStateOnly encryption enabled, vTPM enabled, SecureBoot disabled and SecurityType empty",
			machine: createMachineWithConfidentialCompute(infrav1.SecurityEncryptionTypeVMGuestStateOnly, "", false, true, false),
			wantErr: true,
		},
		{
			name:    "azuremachine with confidential compute VMGuestStateOnly encryption enabled, vTPM and SecureBoot empty",
			machine: createMachineWithConfidentialCompute(infrav1.SecurityEncryptionTypeVMGuestStateOnly, infrav1.SecurityTypesConfidentialVM, false, false, false),
			wantErr: true,
		},
		{
			name:    "azuremachine with confidential compute DiskWithVMGuestState encryption, vTPM and SecureBoot enabled",
			machine: createMachineWithConfidentialCompute(infrav1.SecurityEncryptionTypeDiskWithVMGuestState, infrav1.SecurityTypesConfidentialVM, false, true, true),
			wantErr: false,
		},
		{
			name:    "azuremachine with confidential compute DiskWithVMGuestState encryption enabled, vTPM enabled and SecureBoot disabled",
			machine: createMachineWithConfidentialCompute(infrav1.SecurityEncryptionTypeDiskWithVMGuestState, infrav1.SecurityTypesConfidentialVM, false, true, false),
			wantErr: true,
		},
		{
			name:    "azuremachine with confidential compute DiskWithVMGuestState encryption enabled, vTPM disabled and SecureBoot enabled",
			machine: createMachineWithConfidentialCompute(infrav1.SecurityEncryptionTypeDiskWithVMGuestState, infrav1.SecurityTypesConfidentialVM, false, false, true),
			wantErr: true,
		},
		{
			name:    "azuremachine with confidential compute DiskWithVMGuestState encryption enabled, vTPM disabled and SecureBoot disabled",
			machine: createMachineWithConfidentialCompute(infrav1.SecurityEncryptionTypeDiskWithVMGuestState, infrav1.SecurityTypesConfidentialVM, false, false, false),
			wantErr: true,
		},
		{
			name:    "azuremachine with confidential compute DiskWithVMGuestState encryption enabled, vTPM enabled, SecureBoot disabled and SecurityType empty",
			machine: createMachineWithConfidentialCompute(infrav1.SecurityEncryptionTypeDiskWithVMGuestState, "", false, true, false),
			wantErr: true,
		},
		{
			name:    "azuremachine with empty capacity reservation group id",
			machine: createMachineWithCapacityReservaionGroupID(""),
			wantErr: false,
		},
		{
			name:    "azuremachine with valid capacity reservation group id",
			machine: createMachineWithCapacityReservaionGroupID("azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Compute/capacityReservationGroups/capacity-reservation-group-name"),
			wantErr: false,
		},
		{
			name:    "azuremachine with invalid capacity reservation group id",
			machine: createMachineWithCapacityReservaionGroupID("invalid-capacity-group-id"),
			wantErr: true,
		},
		{
			name:    "azuremachine with DisableExtensionOperations true and without VMExtensions",
			machine: createMachineWithDisableExtenionOperations(),
			wantErr: false,
		},
		{
			name:    "azuremachine with DisableExtensionOperations true and with VMExtension",
			machine: createMachineWithDisableExtenionOperationsAndHasExtension(),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			mw := &AzureMachineWebhook{}
			_, err := mw.ValidateCreate(t.Context(), tc.machine)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureMachine_ValidateUpdate(t *testing.T) {
	tests := []struct {
		name       string
		oldMachine *infrav1.AzureMachine
		newMachine *infrav1.AzureMachine
		wantErr    bool
	}{
		{
			name: "invalidTest: azuremachine.spec.image is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					Image: &infrav1.Image{
						ID: ptr.To("imageID-1"),
					},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					Image: &infrav1.Image{
						ID: ptr.To("imageID-2"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.image is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					Image: &infrav1.Image{
						ID: ptr.To("imageID-1"),
					},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					Image: &infrav1.Image{
						ID: ptr.To("imageID-1"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.Identity is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					Identity: infrav1.VMIdentityUserAssigned,
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					Identity: infrav1.VMIdentityNone,
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.Identity is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					Identity: infrav1.VMIdentityNone,
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					Identity: infrav1.VMIdentityNone,
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.UserAssignedIdentities is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					UserAssignedIdentities: []infrav1.UserAssignedIdentity{
						{ProviderID: "providerID-1"},
					},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					UserAssignedIdentities: []infrav1.UserAssignedIdentity{
						{ProviderID: "providerID-2"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.UserAssignedIdentities is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					UserAssignedIdentities: []infrav1.UserAssignedIdentity{
						{ProviderID: "providerID-1"},
					},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					UserAssignedIdentities: []infrav1.UserAssignedIdentity{
						{ProviderID: "providerID-1"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.RoleAssignmentName is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					RoleAssignmentName: "role",
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					RoleAssignmentName: "not-role",
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.RoleAssignmentName is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					RoleAssignmentName: "role",
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					RoleAssignmentName: "role",
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.RoleAssignmentName is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
						Name:         "role",
						Scope:        "scope",
						DefinitionID: "definitionID",
					},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
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
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
						Name:         "role",
						Scope:        "scope",
						DefinitionID: "definitionID",
					},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
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
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					OSDisk: infrav1.OSDisk{
						OSType: "osType-0",
					},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					OSDisk: infrav1.OSDisk{
						OSType: "osType-1",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.OSDisk is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					OSDisk: infrav1.OSDisk{
						OSType: "osType-1",
					},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					OSDisk: infrav1.OSDisk{
						OSType: "osType-1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.DataDisks is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					DataDisks: []infrav1.DataDisk{
						{
							DiskSizeGB: 128,
						},
					},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					DataDisks: []infrav1.DataDisk{
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
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					DataDisks: []infrav1.DataDisk{
						{
							DiskSizeGB: 128,
						},
					},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					DataDisks: []infrav1.DataDisk{
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
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					SSHPublicKey: "validKey",
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					SSHPublicKey: "invalidKey",
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.SSHPublicKey is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					SSHPublicKey: "validKey",
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					SSHPublicKey: "validKey",
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.AllocatePublicIP is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					AllocatePublicIP: true,
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					AllocatePublicIP: false,
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.AllocatePublicIP is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					AllocatePublicIP: true,
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					AllocatePublicIP: true,
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.EnableIPForwarding is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					EnableIPForwarding: true,
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					EnableIPForwarding: false,
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.EnableIPForwarding is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					EnableIPForwarding: true,
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					EnableIPForwarding: true,
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.AcceleratedNetworking is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					AcceleratedNetworking: ptr.To(true),
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					AcceleratedNetworking: ptr.To(false),
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.AcceleratedNetworking is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					AcceleratedNetworking: ptr.To(true),
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					AcceleratedNetworking: ptr.To(true),
				},
			},
			wantErr: false,
		},
		{
			name: "validTest: azuremachine.spec.AcceleratedNetworking transition(from true) to nil is acceptable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					AcceleratedNetworking: ptr.To(true),
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					AcceleratedNetworking: nil,
				},
			},
			wantErr: false,
		},
		{
			name: "validTest: azuremachine.spec.AcceleratedNetworking transition(from false) to nil is acceptable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					AcceleratedNetworking: ptr.To(false),
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					AcceleratedNetworking: nil,
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.SpotVMOptions is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					SpotVMOptions: &infrav1.SpotVMOptions{
						MaxPrice: &resource.Quantity{Format: "vmoptions-0"},
					},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					SpotVMOptions: &infrav1.SpotVMOptions{
						MaxPrice: &resource.Quantity{Format: "vmoptions-1"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.SpotVMOptions is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					SpotVMOptions: &infrav1.SpotVMOptions{
						MaxPrice: &resource.Quantity{Format: "vmoptions-1"},
					},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					SpotVMOptions: &infrav1.SpotVMOptions{
						MaxPrice: &resource.Quantity{Format: "vmoptions-1"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.SecurityProfile is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					SecurityProfile: &infrav1.SecurityProfile{EncryptionAtHost: ptr.To(true)},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					SecurityProfile: &infrav1.SecurityProfile{EncryptionAtHost: ptr.To(false)},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.SecurityProfile is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					SecurityProfile: &infrav1.SecurityProfile{EncryptionAtHost: ptr.To(true)},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					SecurityProfile: &infrav1.SecurityProfile{EncryptionAtHost: ptr.To(true)},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.Diagnostics is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					Diagnostics: &infrav1.Diagnostics{Boot: &infrav1.BootDiagnostics{StorageAccountType: infrav1.DisabledDiagnosticsStorage}},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					Diagnostics: &infrav1.Diagnostics{Boot: &infrav1.BootDiagnostics{StorageAccountType: infrav1.ManagedDiagnosticsStorage}},
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.Diagnostics is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					Diagnostics: &infrav1.Diagnostics{Boot: &infrav1.BootDiagnostics{StorageAccountType: infrav1.DisabledDiagnosticsStorage}},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					Diagnostics: &infrav1.Diagnostics{Boot: &infrav1.BootDiagnostics{StorageAccountType: infrav1.DisabledDiagnosticsStorage}},
				},
			},
			wantErr: false,
		},
		{
			name: "validTest: azuremachine.spec.Diagnostics should not error on updating nil diagnostics",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					Diagnostics: &infrav1.Diagnostics{Boot: &infrav1.BootDiagnostics{StorageAccountType: infrav1.ManagedDiagnosticsStorage}},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidTest: azuremachine.spec.Diagnostics is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					Diagnostics: &infrav1.Diagnostics{},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					Diagnostics: &infrav1.Diagnostics{Boot: &infrav1.BootDiagnostics{StorageAccountType: infrav1.ManagedDiagnosticsStorage}},
				},
			},
			wantErr: true,
		},
		{
			name: "invalidTest: azuremachine.spec.Diagnostics is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					Diagnostics: &infrav1.Diagnostics{
						Boot: &infrav1.BootDiagnostics{},
					},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					Diagnostics: &infrav1.Diagnostics{Boot: &infrav1.BootDiagnostics{StorageAccountType: infrav1.ManagedDiagnosticsStorage}},
				},
			},
			wantErr: true,
		},
		{
			name: "invalidTest: azuremachine.spec.disableExtensionOperations is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					DisableExtensionOperations: ptr.To(true),
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					DisableExtensionOperations: ptr.To(false),
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.disableExtensionOperations is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					DisableExtensionOperations: ptr.To(true),
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					DisableExtensionOperations: ptr.To(true),
				},
			},
			wantErr: false,
		},
		{
			name: "validTest: azuremachine.spec.networkInterfaces is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					NetworkInterfaces: []infrav1.NetworkInterface{{SubnetName: "subnet"}},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					NetworkInterfaces: []infrav1.NetworkInterface{{SubnetName: "subnet"}},
				},
			},
			wantErr: false,
		},
		{
			name: "invalidtest: azuremachine.spec.networkInterfaces is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					NetworkInterfaces: []infrav1.NetworkInterface{{SubnetName: "subnet1"}},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					NetworkInterfaces: []infrav1.NetworkInterface{{SubnetName: "subnet2"}},
				},
			},
			wantErr: true,
		},
		{
			name: "invalidtest: updating subnet name from empty to non empty",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					NetworkInterfaces: []infrav1.NetworkInterface{{SubnetName: ""}},
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					NetworkInterfaces: []infrav1.NetworkInterface{{SubnetName: "subnet1"}},
				},
			},
			wantErr: true,
		},
		{
			name: "invalidTest: azuremachine.spec.capacityReservationGroupID is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					CapacityReservationGroupID: ptr.To("capacityReservationGroupID-1"),
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					CapacityReservationGroupID: ptr.To("capacityReservationGroupID-2"),
				},
			},
			wantErr: true,
		},
		{
			name: "invalidTest: updating azuremachine.spec.capacityReservationGroupID from empty to non-empty",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					CapacityReservationGroupID: nil,
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					CapacityReservationGroupID: ptr.To("capacityReservationGroupID-1"),
				},
			},
			wantErr: true,
		},
		{
			name: "invalidTest: updating azuremachine.spec.capacityReservationGroupID from non-empty to empty",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					CapacityReservationGroupID: ptr.To("capacityReservationGroupID-1"),
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					CapacityReservationGroupID: nil,
				},
			},
			wantErr: true,
		},
		{
			name: "validTest: azuremachine.spec.capacityReservationGroupID is immutable",
			oldMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					CapacityReservationGroupID: ptr.To("capacityReservationGroupID-1"),
				},
			},
			newMachine: &infrav1.AzureMachine{
				Spec: infrav1.AzureMachineSpec{
					CapacityReservationGroupID: ptr.To("capacityReservationGroupID-1"),
				},
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			mw := &AzureMachineWebhook{}
			_, err := mw.ValidateUpdate(t.Context(), tc.oldMachine, tc.newMachine)
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
	case *infrav1.AzureCluster:
		obj.Spec.SubscriptionID = m.SubscriptionID
	case *clusterv1.Cluster:
		obj.Spec.InfrastructureRef = clusterv1.ContractVersionedObjectReference{
			Kind: infrav1.AzureClusterKind,
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
		machine *infrav1.AzureMachine
	}

	testSubscriptionID := "test-subscription-id"
	mockClient := mockDefaultClient{SubscriptionID: testSubscriptionID}
	existingPublicKey := validSSHPublicKey
	publicKeyExistTest := test{machine: apifixtures.CreateMachineWithSSHPublicKey(existingPublicKey)}
	publicKeyNotExistTest := test{machine: apifixtures.CreateMachineWithSSHPublicKey("")}
	testObjectMeta := metav1.ObjectMeta{
		Labels: map[string]string{
			clusterv1.ClusterNameLabel: "test-cluster",
		},
	}

	mw := &AzureMachineWebhook{
		client: mockClient,
	}

	err := mw.Default(t.Context(), publicKeyExistTest.machine)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(publicKeyExistTest.machine.Spec.SSHPublicKey).To(Equal(existingPublicKey))

	err = mw.Default(t.Context(), publicKeyNotExistTest.machine)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(publicKeyNotExistTest.machine.Spec.SSHPublicKey).To(Not(BeEmpty()))

	for _, possibleCachingType := range armcompute.PossibleCachingTypesValues() {
		cacheTypeSpecifiedTest := test{machine: &infrav1.AzureMachine{ObjectMeta: testObjectMeta, Spec: infrav1.AzureMachineSpec{OSDisk: infrav1.OSDisk{CachingType: string(possibleCachingType)}}}}
		err = mw.Default(t.Context(), cacheTypeSpecifiedTest.machine)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(cacheTypeSpecifiedTest.machine.Spec.OSDisk.CachingType).To(Equal(string(possibleCachingType)))
	}
}

func createMachineWithNetworkConfig(subnetName string, acceleratedNetworking *bool, interfaces []infrav1.NetworkInterface) *infrav1.AzureMachine {
	return &infrav1.AzureMachine{
		Spec: infrav1.AzureMachineSpec{
			SubnetName:            subnetName,
			NetworkInterfaces:     interfaces,
			AcceleratedNetworking: acceleratedNetworking,
			OSDisk:                validOSDisk,
			SSHPublicKey:          validSSHPublicKey,
		},
	}
}

func createMachineWithSharedImage(subscriptionID, resourceGroup, name, gallery, version string) *infrav1.AzureMachine {
	image := &infrav1.Image{
		SharedGallery: &infrav1.AzureSharedGalleryImage{
			SubscriptionID: subscriptionID,
			ResourceGroup:  resourceGroup,
			Name:           name,
			Gallery:        gallery,
			Version:        version,
		},
	}

	return &infrav1.AzureMachine{
		Spec: infrav1.AzureMachineSpec{
			Image:        image,
			SSHPublicKey: validSSHPublicKey,
			OSDisk:       validOSDisk,
		},
	}
}

func createMachineWithMarketPlaceImage(publisher, offer, sku, version string) *infrav1.AzureMachine {
	image := &infrav1.Image{
		Marketplace: &infrav1.AzureMarketplaceImage{
			ImagePlan: infrav1.ImagePlan{
				Publisher: publisher,
				Offer:     offer,
				SKU:       sku,
			},
			Version: version,
		},
	}

	return &infrav1.AzureMachine{
		Spec: infrav1.AzureMachineSpec{
			Image:        image,
			SSHPublicKey: validSSHPublicKey,
			OSDisk:       validOSDisk,
		},
	}
}

func createMachineWithImageByID(imageID string) *infrav1.AzureMachine {
	image := &infrav1.Image{
		ID: &imageID,
	}

	return &infrav1.AzureMachine{
		Spec: infrav1.AzureMachineSpec{
			Image:        image,
			SSHPublicKey: validSSHPublicKey,
			OSDisk:       validOSDisk,
		},
	}
}

func createMachineWithOsDiskCacheType(cacheType string) *infrav1.AzureMachine {
	machine := &infrav1.AzureMachine{
		Spec: infrav1.AzureMachineSpec{
			SSHPublicKey: validSSHPublicKey,
			OSDisk:       validOSDisk,
		},
	}
	machine.Spec.OSDisk.CachingType = cacheType
	return machine
}

func createMachineWithSystemAssignedIdentityRoleName() *infrav1.AzureMachine {
	machine := &infrav1.AzureMachine{
		Spec: infrav1.AzureMachineSpec{
			SSHPublicKey: validSSHPublicKey,
			OSDisk:       validOSDisk,
			Identity:     infrav1.VMIdentitySystemAssigned,
			SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
				Name:         "c6e3443d-bc11-4335-8819-ab6637b10586",
				Scope:        "test-scope",
				DefinitionID: "test-definition-id",
			},
		},
	}
	return machine
}

func createMachineWithoutSystemAssignedIdentityRoleName() *infrav1.AzureMachine {
	machine := &infrav1.AzureMachine{
		Spec: infrav1.AzureMachineSpec{
			SSHPublicKey: validSSHPublicKey,
			OSDisk:       validOSDisk,
			Identity:     infrav1.VMIdentitySystemAssigned,
			SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
				Scope:        "test-scope",
				DefinitionID: "test-definition-id",
			},
		},
	}
	return machine
}

func createMachineWithoutRoleAssignmentName() *infrav1.AzureMachine {
	machine := &infrav1.AzureMachine{
		Spec: infrav1.AzureMachineSpec{
			SSHPublicKey: validSSHPublicKey,
			OSDisk:       validOSDisk,
		},
	}
	return machine
}

func createMachineWithRoleAssignmentName() *infrav1.AzureMachine {
	machine := &infrav1.AzureMachine{
		Spec: infrav1.AzureMachineSpec{
			SSHPublicKey:       validSSHPublicKey,
			OSDisk:             validOSDisk,
			RoleAssignmentName: "test-role-assignment",
		},
	}
	return machine
}

func createMachineWithDiagnostics(diagnosticsType infrav1.BootDiagnosticsStorageAccountType, userManaged *infrav1.UserManagedBootDiagnostics) *infrav1.AzureMachine {
	var diagnostics *infrav1.Diagnostics

	if diagnosticsType != "" {
		diagnostics = &infrav1.Diagnostics{
			Boot: &infrav1.BootDiagnostics{
				StorageAccountType: diagnosticsType,
			},
		}
	}

	if userManaged != nil {
		diagnostics.Boot.UserManaged = userManaged
	}

	return &infrav1.AzureMachine{
		Spec: infrav1.AzureMachineSpec{
			SSHPublicKey: validSSHPublicKey,
			OSDisk:       validOSDisk,
			Diagnostics:  diagnostics,
		},
	}
}

func createMachineWithConfidentialCompute(securityEncryptionType infrav1.SecurityEncryptionType, securityType infrav1.SecurityTypes, encryptionAtHost, vTpmEnabled, secureBootEnabled bool) *infrav1.AzureMachine {
	securityProfile := &infrav1.SecurityProfile{
		EncryptionAtHost: &encryptionAtHost,
		SecurityType:     securityType,
		UefiSettings: &infrav1.UefiSettings{
			VTpmEnabled:       &vTpmEnabled,
			SecureBootEnabled: &secureBootEnabled,
		},
	}

	osDisk := infrav1.OSDisk{
		DiskSizeGB: ptr.To[int32](30),
		OSType:     infrav1.LinuxOS,
		ManagedDisk: &infrav1.ManagedDiskParameters{
			StorageAccountType: "Premium_LRS",
			SecurityProfile: &infrav1.VMDiskSecurityProfile{
				SecurityEncryptionType: securityEncryptionType,
			},
		},
		CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
	}

	return &infrav1.AzureMachine{
		Spec: infrav1.AzureMachineSpec{
			SSHPublicKey:    validSSHPublicKey,
			OSDisk:          osDisk,
			SecurityProfile: securityProfile,
		},
	}
}

func createMachineWithCapacityReservaionGroupID(capacityReservationGroupID string) *infrav1.AzureMachine {
	var strPtr *string
	if capacityReservationGroupID != "" {
		strPtr = ptr.To(capacityReservationGroupID)
	}

	return &infrav1.AzureMachine{
		Spec: infrav1.AzureMachineSpec{
			SSHPublicKey:               validSSHPublicKey,
			OSDisk:                     validOSDisk,
			CapacityReservationGroupID: strPtr,
		},
	}
}

func createMachineWithDisableExtenionOperationsAndHasExtension() *infrav1.AzureMachine {
	return &infrav1.AzureMachine{
		Spec: infrav1.AzureMachineSpec{
			SSHPublicKey:               validSSHPublicKey,
			OSDisk:                     validOSDisk,
			DisableExtensionOperations: ptr.To(true),
			VMExtensions: []infrav1.VMExtension{{
				Name:      "test-extension",
				Publisher: "test-publiher",
				Version:   "v0.0.1-test",
			}},
		},
	}
}

func createMachineWithDisableExtenionOperations() *infrav1.AzureMachine {
	return &infrav1.AzureMachine{
		Spec: infrav1.AzureMachineSpec{
			SSHPublicKey:               validSSHPublicKey,
			OSDisk:                     validOSDisk,
			DisableExtensionOperations: ptr.To(true),
		},
	}
}

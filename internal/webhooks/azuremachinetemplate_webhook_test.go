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
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	apifixtures "sigs.k8s.io/cluster-api-provider-azure/internal/test/apifixtures"
)

func TestAzureMachineTemplate_ValidateCreate(t *testing.T) {
	tests := []struct {
		name            string
		machineTemplate *infrav1.AzureMachineTemplate
		wantErr         bool
	}{
		{
			name: "azuremachinetemplate with marketplane image - full",
			machineTemplate: createAzureMachineTemplateFromMachine(
				createMachineWithMarketPlaceImage("PUB1234", "OFFER1234", "SKU1234", "1.0.0"),
			),
			wantErr: false,
		},
		{
			name: "azuremachinetemplate with marketplace image - missing publisher",
			machineTemplate: createAzureMachineTemplateFromMachine(
				createMachineWithMarketPlaceImage("", "OFFER1234", "SKU1234", "1.0.0"),
			),
			wantErr: true,
		},
		{
			name: "azuremachinetemplate with shared gallery image - full",
			machineTemplate: createAzureMachineTemplateFromMachine(
				createMachineWithSharedImage("SUB123", "RG123", "NAME123", "GALLERY1", "1.0.0"),
			),
			wantErr: false,
		},
		{
			name: "azuremachinetemplate with marketplace image - missing subscription",
			machineTemplate: createAzureMachineTemplateFromMachine(
				createMachineWithSharedImage("", "RG123", "NAME123", "GALLERY2", "1.0.0"),
			),
			wantErr: true,
		},
		{
			name: "azuremachinetemplate with image by - with id",
			machineTemplate: createAzureMachineTemplateFromMachine(
				createMachineWithImageByID("ID123"),
			),
			wantErr: false,
		},
		{
			name: "azuremachinetemplate with image by - without id",
			machineTemplate: createAzureMachineTemplateFromMachine(
				createMachineWithImageByID(""),
			),
			wantErr: true,
		},
		{
			name: "azuremachinetemplate with valid SSHPublicKey",
			machineTemplate: createAzureMachineTemplateFromMachine(
				apifixtures.CreateMachineWithSSHPublicKey(validSSHPublicKey),
			),
			wantErr: false,
		},
		{
			name: "azuremachinetemplate without SSHPublicKey",
			machineTemplate: createAzureMachineTemplateFromMachine(
				apifixtures.CreateMachineWithSSHPublicKey(""),
			),
			wantErr: true,
		},
		{
			name: "azuremachinetemplate with invalid SSHPublicKey",
			machineTemplate: createAzureMachineTemplateFromMachine(
				apifixtures.CreateMachineWithSSHPublicKey("invalid ssh key"),
			),
			wantErr: true,
		},
		{
			name: "azuremachinetemplate with list of user-assigned identities",
			machineTemplate: createAzureMachineTemplateFromMachine(
				apifixtures.CreateMachineWithUserAssignedIdentities([]infrav1.UserAssignedIdentity{
					{ProviderID: "azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Compute/virtualMachines/default-09091-control-plane-f1b2c"},
					{ProviderID: "azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Compute/virtualMachines/default-09091-control-plane-9a8b7"},
				}),
			),
			wantErr: false,
		},
		{
			name: "azuremachinetemplate with empty list of user-assigned identities",
			machineTemplate: createAzureMachineTemplateFromMachine(
				apifixtures.CreateMachineWithUserAssignedIdentities([]infrav1.UserAssignedIdentity{}),
			),
			wantErr: true,
		},
		{
			name: "azuremachinetemplate with valid osDisk cache type",
			machineTemplate: createAzureMachineTemplateFromMachine(
				createMachineWithOsDiskCacheType(string(armcompute.PossibleCachingTypesValues()[1])),
			),
			wantErr: false,
		},
		{
			name: "azuremachinetemplate with invalid osDisk cache type",
			machineTemplate: createAzureMachineTemplateFromMachine(
				createMachineWithOsDiskCacheType("invalid_cache_type"),
			),
			wantErr: true,
		},
		{
			name:            "azuremachinetemplate with SystemAssignedIdentityRoleName",
			machineTemplate: createAzureMachineTemplateFromMachine(createMachineWithSystemAssignedIdentityRoleName()),
			wantErr:         true,
		},
		{
			name:            "azuremachinetemplate without SystemAssignedIdentityRoleName",
			machineTemplate: createAzureMachineTemplateFromMachine(createMachineWithoutSystemAssignedIdentityRoleName()),
			wantErr:         false,
		},
		{
			name:            "azuremachinetemplate with RoleAssignmentName",
			machineTemplate: createAzureMachineTemplateFromMachine(createMachineWithRoleAssignmentName()),
			wantErr:         true,
		},
		{
			name:            "azuremachinetemplate with DisableExtensionOperations true and without VMExtensions",
			machineTemplate: createAzureMachineTemplateFromMachine(createMachineWithDisableExtenionOperations()),
			wantErr:         false,
		},
		{
			name:            "azuremachinetempalte with DisableExtensionOperations true and with VMExtension",
			machineTemplate: createAzureMachineTemplateFromMachine(createMachineWithDisableExtenionOperationsAndHasExtension()),
			wantErr:         true,
		},
		{
			name:            "azuremachinetemplate without RoleAssignmentName",
			machineTemplate: createAzureMachineTemplateFromMachine(createMachineWithoutRoleAssignmentName()),
			wantErr:         false,
		},
		{
			name: "azuremachinetemplate with network interfaces > 0 and subnet name",
			machineTemplate: createAzureMachineTemplateFromMachine(
				createMachineWithNetworkConfig(
					"test-subnet",
					nil,
					[]infrav1.NetworkInterface{
						{SubnetName: "subnet1", PrivateIPConfigs: 1},
						{SubnetName: "subnet2", PrivateIPConfigs: 1},
					},
				),
			),
			wantErr: true,
		},
		{
			name: "azuremachinetemplate with network interfaces > 0 and no subnet name",
			machineTemplate: createAzureMachineTemplateFromMachine(
				createMachineWithNetworkConfig(
					"",
					nil,
					[]infrav1.NetworkInterface{
						{SubnetName: "subnet1", PrivateIPConfigs: 1},
						{SubnetName: "subnet2", PrivateIPConfigs: 1},
					},
				),
			),
			wantErr: false,
		},
		{
			name: "azuremachinetemplate with network interfaces > 0 and AcceleratedNetworking not nil",
			machineTemplate: createAzureMachineTemplateFromMachine(
				createMachineWithNetworkConfig(
					"",
					ptr.To(true),
					[]infrav1.NetworkInterface{
						{SubnetName: "subnet1", PrivateIPConfigs: 1},
						{SubnetName: "subnet2", PrivateIPConfigs: 1},
					},
				),
			),
			wantErr: true,
		},
		{
			name: "azuremachinetemplate with network interfaces > 0 and AcceleratedNetworking nil",
			machineTemplate: createAzureMachineTemplateFromMachine(
				createMachineWithNetworkConfig(
					"",
					nil,
					[]infrav1.NetworkInterface{
						{SubnetName: "subnet1", PrivateIPConfigs: 1},
						{SubnetName: "subnet2", PrivateIPConfigs: 1},
					},
				),
			),
			wantErr: false,
		},
		{
			name: "azuremachinetemplate with network interfaces and PrivateIPConfigs < 1",
			machineTemplate: createAzureMachineTemplateFromMachine(
				createMachineWithNetworkConfig(
					"",
					nil,
					[]infrav1.NetworkInterface{
						{SubnetName: "subnet1", PrivateIPConfigs: 0},
						{SubnetName: "subnet2", PrivateIPConfigs: -1},
					},
				),
			),
			wantErr: true,
		},
		{
			name: "azuremachinetemplate with network interfaces and PrivateIPConfigs >= 1",
			machineTemplate: createAzureMachineTemplateFromMachine(
				createMachineWithNetworkConfig(
					"",
					nil,
					[]infrav1.NetworkInterface{
						{SubnetName: "subnet1", PrivateIPConfigs: 1},
						{SubnetName: "subnet2", PrivateIPConfigs: 2},
					},
				),
			),
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			ctx := t.Context()
			_, err := (&AzureMachineTemplateWebhook{}).ValidateCreate(ctx, test.machineTemplate)
			if test.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureMachineTemplate_ValidateUpdate(t *testing.T) {
	failureDomain := "domaintest"

	tests := []struct {
		name        string
		oldTemplate *infrav1.AzureMachineTemplate
		template    *infrav1.AzureMachineTemplate
		wantErr     bool
	}{
		{
			name: "AzureMachineTemplate with immutable spec",
			oldTemplate: &infrav1.AzureMachineTemplate{
				Spec: infrav1.AzureMachineTemplateSpec{
					Template: infrav1.AzureMachineTemplateResource{
						Spec: infrav1.AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: infrav1.OSDisk{
								OSType:     "type",
								DiskSizeGB: ptr.To[int32](11),
							},
							DataDisks:    []infrav1.DataDisk{},
							SSHPublicKey: "",
						},
					},
				},
			},
			template: &infrav1.AzureMachineTemplate{
				Spec: infrav1.AzureMachineTemplateSpec{
					Template: infrav1.AzureMachineTemplateResource{
						Spec: infrav1.AzureMachineSpec{
							VMSize:        "size1",
							FailureDomain: &failureDomain,
							OSDisk: infrav1.OSDisk{
								OSType:     "type",
								DiskSizeGB: ptr.To[int32](11),
							},
							DataDisks:    []infrav1.DataDisk{},
							SSHPublicKey: "fake ssh key",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureMachineTemplate with mutable metadata",
			oldTemplate: &infrav1.AzureMachineTemplate{
				Spec: infrav1.AzureMachineTemplateSpec{
					Template: infrav1.AzureMachineTemplateResource{
						Spec: infrav1.AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: infrav1.OSDisk{
								OSType:     "type",
								DiskSizeGB: ptr.To[int32](11),
							},
							DataDisks:    []infrav1.DataDisk{},
							SSHPublicKey: "fake ssh key",
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "OldTemplate",
				},
			},
			template: &infrav1.AzureMachineTemplate{
				Spec: infrav1.AzureMachineTemplateSpec{
					Template: infrav1.AzureMachineTemplateResource{
						Spec: infrav1.AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: infrav1.OSDisk{
								OSType:     "type",
								DiskSizeGB: ptr.To[int32](11),
							},
							DataDisks:    []infrav1.DataDisk{},
							SSHPublicKey: "fake ssh key",
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "NewTemplate",
				},
			},
			wantErr: false,
		},
		{
			name: "AzureMachineTemplate ssh key removed",
			oldTemplate: &infrav1.AzureMachineTemplate{
				Spec: infrav1.AzureMachineTemplateSpec{
					Template: infrav1.AzureMachineTemplateResource{
						Spec: infrav1.AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: infrav1.OSDisk{
								OSType:      "type",
								DiskSizeGB:  ptr.To[int32](11),
								CachingType: "None",
							},
							DataDisks:    []infrav1.DataDisk{},
							SSHPublicKey: "some key",
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "OldTemplate",
				},
			},
			template: &infrav1.AzureMachineTemplate{
				Spec: infrav1.AzureMachineTemplateSpec{
					Template: infrav1.AzureMachineTemplateResource{
						Spec: infrav1.AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: infrav1.OSDisk{
								OSType:      "type",
								DiskSizeGB:  ptr.To[int32](11),
								CachingType: "None",
							},
							DataDisks:    []infrav1.DataDisk{},
							SSHPublicKey: "",
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "NewTemplate",
				},
			},
			wantErr: true,
		},
		{
			name: "AzureMachineTemplate with legacy subnetName updated to new networkInterfaces",
			oldTemplate: &infrav1.AzureMachineTemplate{
				Spec: infrav1.AzureMachineTemplateSpec{
					Template: infrav1.AzureMachineTemplateResource{
						Spec: infrav1.AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: infrav1.OSDisk{
								OSType:      "type",
								DiskSizeGB:  ptr.To[int32](11),
								CachingType: "None",
							},
							DataDisks:             []infrav1.DataDisk{},
							SSHPublicKey:          "fake ssh key",
							SubnetName:            "subnet1",
							AcceleratedNetworking: ptr.To(true),
						},
					},
				},
			},
			template: &infrav1.AzureMachineTemplate{
				Spec: infrav1.AzureMachineTemplateSpec{
					Template: infrav1.AzureMachineTemplateResource{
						Spec: infrav1.AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: infrav1.OSDisk{
								OSType:      "type",
								DiskSizeGB:  ptr.To[int32](11),
								CachingType: "None",
							},
							DataDisks:             []infrav1.DataDisk{},
							SSHPublicKey:          "fake ssh key",
							SubnetName:            "",
							AcceleratedNetworking: nil,
							NetworkInterfaces: []infrav1.NetworkInterface{
								{
									SubnetName:            "subnet1",
									AcceleratedNetworking: ptr.To(true),
									PrivateIPConfigs:      1,
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AzureMachineTemplate with legacy AcceleratedNetworking updated to new networkInterfaces",
			oldTemplate: &infrav1.AzureMachineTemplate{
				Spec: infrav1.AzureMachineTemplateSpec{
					Template: infrav1.AzureMachineTemplateResource{
						Spec: infrav1.AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: infrav1.OSDisk{
								OSType:      "type",
								DiskSizeGB:  ptr.To[int32](11),
								CachingType: "None",
							},
							DataDisks:             []infrav1.DataDisk{},
							SSHPublicKey:          "fake ssh key",
							SubnetName:            "",
							AcceleratedNetworking: ptr.To(true),
							NetworkInterfaces:     []infrav1.NetworkInterface{},
						},
					},
				},
			},
			template: &infrav1.AzureMachineTemplate{
				Spec: infrav1.AzureMachineTemplateSpec{
					Template: infrav1.AzureMachineTemplateResource{
						Spec: infrav1.AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: infrav1.OSDisk{
								OSType:      "type",
								DiskSizeGB:  ptr.To[int32](11),
								CachingType: "None",
							},
							DataDisks:             []infrav1.DataDisk{},
							SSHPublicKey:          "fake ssh key",
							SubnetName:            "",
							AcceleratedNetworking: nil,
							NetworkInterfaces: []infrav1.NetworkInterface{
								{
									SubnetName:            "",
									AcceleratedNetworking: ptr.To(true),
									PrivateIPConfigs:      1,
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "AzureMachineTemplate with modified networkInterfaces is immutable",
			oldTemplate: &infrav1.AzureMachineTemplate{
				Spec: infrav1.AzureMachineTemplateSpec{
					Template: infrav1.AzureMachineTemplateResource{
						Spec: infrav1.AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: infrav1.OSDisk{
								OSType:      "type",
								DiskSizeGB:  ptr.To[int32](11),
								CachingType: "None",
							},
							DataDisks:    []infrav1.DataDisk{},
							SSHPublicKey: "fake ssh key",
							NetworkInterfaces: []infrav1.NetworkInterface{
								{
									SubnetName:            "subnet1",
									AcceleratedNetworking: ptr.To(true),
									PrivateIPConfigs:      1,
								},
							},
						},
					},
				},
			},
			template: &infrav1.AzureMachineTemplate{
				Spec: infrav1.AzureMachineTemplateSpec{
					Template: infrav1.AzureMachineTemplateResource{
						Spec: infrav1.AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: infrav1.OSDisk{
								OSType:      "type",
								DiskSizeGB:  ptr.To[int32](11),
								CachingType: "None",
							},
							DataDisks:    []infrav1.DataDisk{},
							SSHPublicKey: "fake ssh key",
							NetworkInterfaces: []infrav1.NetworkInterface{
								{
									SubnetName:            "subnet2",
									AcceleratedNetworking: ptr.To(true),
									PrivateIPConfigs:      1,
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	// dry-run=true
	for _, amt := range tests {
		t.Run(amt.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := admission.NewContextWithRequest(t.Context(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: ptr.To(true)}})
			_, err := (&AzureMachineTemplateWebhook{}).ValidateUpdate(ctx, amt.oldTemplate, amt.template)
			if amt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
	// dry-run=false
	for _, amt := range tests {
		t.Run(amt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			ctx := admission.NewContextWithRequest(t.Context(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: ptr.To(false)}})
			_, err := (&AzureMachineTemplateWebhook{}).ValidateUpdate(ctx, amt.oldTemplate, amt.template)
			if amt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func createAzureMachineTemplateFromMachine(machine *infrav1.AzureMachine) *infrav1.AzureMachineTemplate {
	return &infrav1.AzureMachineTemplate{
		Spec: infrav1.AzureMachineTemplateSpec{
			Template: infrav1.AzureMachineTemplateResource{
				Spec: machine.Spec,
			},
		},
	}
}

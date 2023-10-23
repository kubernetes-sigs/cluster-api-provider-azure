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
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestAzureMachineTemplate_ValidateCreate(t *testing.T) {
	tests := []struct {
		name            string
		machineTemplate *AzureMachineTemplate
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
				createMachineWithSSHPublicKey(validSSHPublicKey),
			),
			wantErr: false,
		},
		{
			name: "azuremachinetemplate without SSHPublicKey",
			machineTemplate: createAzureMachineTemplateFromMachine(
				createMachineWithSSHPublicKey(""),
			),
			wantErr: true,
		},
		{
			name: "azuremachinetemplate with invalid SSHPublicKey",
			machineTemplate: createAzureMachineTemplateFromMachine(
				createMachineWithSSHPublicKey("invalid ssh key"),
			),
			wantErr: true,
		},
		{
			name: "azuremachinetemplate with list of user-assigned identities",
			machineTemplate: createAzureMachineTemplateFromMachine(
				createMachineWithUserAssignedIdentities([]UserAssignedIdentity{
					{ProviderID: "azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Compute/virtualMachines/default-09091-control-plane-f1b2c"},
					{ProviderID: "azure:///subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/my-resource-group/providers/Microsoft.Compute/virtualMachines/default-09091-control-plane-9a8b7"},
				}),
			),
			wantErr: false,
		},
		{
			name: "azuremachinetemplate with empty list of user-assigned identities",
			machineTemplate: createAzureMachineTemplateFromMachine(
				createMachineWithUserAssignedIdentities([]UserAssignedIdentity{}),
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
					[]NetworkInterface{
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
					[]NetworkInterface{
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
					[]NetworkInterface{
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
					[]NetworkInterface{
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
					[]NetworkInterface{
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
					[]NetworkInterface{
						{SubnetName: "subnet1", PrivateIPConfigs: 1},
						{SubnetName: "subnet2", PrivateIPConfigs: 2},
					},
				),
			),
			wantErr: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			ctx := context.Background()
			_, err := test.machineTemplate.ValidateCreate(ctx, test.machineTemplate)
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
		oldTemplate *AzureMachineTemplate
		template    *AzureMachineTemplate
		wantErr     bool
	}{
		{
			name: "AzureMachineTemplate with immutable spec",
			oldTemplate: &AzureMachineTemplate{
				Spec: AzureMachineTemplateSpec{
					Template: AzureMachineTemplateResource{
						Spec: AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: OSDisk{
								OSType:     "type",
								DiskSizeGB: ptr.To[int32](11),
							},
							DataDisks:    []DataDisk{},
							SSHPublicKey: "",
						},
					},
				},
			},
			template: &AzureMachineTemplate{
				Spec: AzureMachineTemplateSpec{
					Template: AzureMachineTemplateResource{
						Spec: AzureMachineSpec{
							VMSize:        "size1",
							FailureDomain: &failureDomain,
							OSDisk: OSDisk{
								OSType:     "type",
								DiskSizeGB: ptr.To[int32](11),
							},
							DataDisks:    []DataDisk{},
							SSHPublicKey: "fake ssh key",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AzureMachineTemplate with mutable metadata",
			oldTemplate: &AzureMachineTemplate{
				Spec: AzureMachineTemplateSpec{
					Template: AzureMachineTemplateResource{
						Spec: AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: OSDisk{
								OSType:     "type",
								DiskSizeGB: ptr.To[int32](11),
							},
							DataDisks:    []DataDisk{},
							SSHPublicKey: "fake ssh key",
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "OldTemplate",
				},
			},
			template: &AzureMachineTemplate{
				Spec: AzureMachineTemplateSpec{
					Template: AzureMachineTemplateResource{
						Spec: AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: OSDisk{
								OSType:     "type",
								DiskSizeGB: ptr.To[int32](11),
							},
							DataDisks:    []DataDisk{},
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
			name: "AzureMachineTemplate with default mismatch",
			oldTemplate: &AzureMachineTemplate{
				Spec: AzureMachineTemplateSpec{
					Template: AzureMachineTemplateResource{
						Spec: AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: OSDisk{
								OSType:      "type",
								DiskSizeGB:  ptr.To[int32](11),
								CachingType: "",
							},
							DataDisks:    []DataDisk{},
							SSHPublicKey: "",
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "OldTemplate",
				},
			},
			template: &AzureMachineTemplate{
				Spec: AzureMachineTemplateSpec{
					Template: AzureMachineTemplateResource{
						Spec: AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: OSDisk{
								OSType:      "type",
								DiskSizeGB:  ptr.To[int32](11),
								CachingType: "None",
							},
							DataDisks:    []DataDisk{},
							SSHPublicKey: "fake ssh key",
							NetworkInterfaces: []NetworkInterface{{
								PrivateIPConfigs: 1,
							}},
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
			oldTemplate: &AzureMachineTemplate{
				Spec: AzureMachineTemplateSpec{
					Template: AzureMachineTemplateResource{
						Spec: AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: OSDisk{
								OSType:      "type",
								DiskSizeGB:  ptr.To[int32](11),
								CachingType: "None",
							},
							DataDisks:    []DataDisk{},
							SSHPublicKey: "some key",
						},
					},
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "OldTemplate",
				},
			},
			template: &AzureMachineTemplate{
				Spec: AzureMachineTemplateSpec{
					Template: AzureMachineTemplateResource{
						Spec: AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: OSDisk{
								OSType:      "type",
								DiskSizeGB:  ptr.To[int32](11),
								CachingType: "None",
							},
							DataDisks:    []DataDisk{},
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
			oldTemplate: &AzureMachineTemplate{
				Spec: AzureMachineTemplateSpec{
					Template: AzureMachineTemplateResource{
						Spec: AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: OSDisk{
								OSType:      "type",
								DiskSizeGB:  ptr.To[int32](11),
								CachingType: "None",
							},
							DataDisks:             []DataDisk{},
							SSHPublicKey:          "fake ssh key",
							SubnetName:            "subnet1",
							AcceleratedNetworking: ptr.To(true),
						},
					},
				},
			},
			template: &AzureMachineTemplate{
				Spec: AzureMachineTemplateSpec{
					Template: AzureMachineTemplateResource{
						Spec: AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: OSDisk{
								OSType:      "type",
								DiskSizeGB:  ptr.To[int32](11),
								CachingType: "None",
							},
							DataDisks:             []DataDisk{},
							SSHPublicKey:          "fake ssh key",
							SubnetName:            "",
							AcceleratedNetworking: nil,
							NetworkInterfaces: []NetworkInterface{
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
			oldTemplate: &AzureMachineTemplate{
				Spec: AzureMachineTemplateSpec{
					Template: AzureMachineTemplateResource{
						Spec: AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: OSDisk{
								OSType:      "type",
								DiskSizeGB:  ptr.To[int32](11),
								CachingType: "None",
							},
							DataDisks:             []DataDisk{},
							SSHPublicKey:          "fake ssh key",
							SubnetName:            "",
							AcceleratedNetworking: ptr.To(true),
							NetworkInterfaces:     []NetworkInterface{},
						},
					},
				},
			},
			template: &AzureMachineTemplate{
				Spec: AzureMachineTemplateSpec{
					Template: AzureMachineTemplateResource{
						Spec: AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: OSDisk{
								OSType:      "type",
								DiskSizeGB:  ptr.To[int32](11),
								CachingType: "None",
							},
							DataDisks:             []DataDisk{},
							SSHPublicKey:          "fake ssh key",
							SubnetName:            "",
							AcceleratedNetworking: nil,
							NetworkInterfaces: []NetworkInterface{
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
			oldTemplate: &AzureMachineTemplate{
				Spec: AzureMachineTemplateSpec{
					Template: AzureMachineTemplateResource{
						Spec: AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: OSDisk{
								OSType:      "type",
								DiskSizeGB:  ptr.To[int32](11),
								CachingType: "None",
							},
							DataDisks:    []DataDisk{},
							SSHPublicKey: "fake ssh key",
							NetworkInterfaces: []NetworkInterface{
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
			template: &AzureMachineTemplate{
				Spec: AzureMachineTemplateSpec{
					Template: AzureMachineTemplateResource{
						Spec: AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: OSDisk{
								OSType:      "type",
								DiskSizeGB:  ptr.To[int32](11),
								CachingType: "None",
							},
							DataDisks:    []DataDisk{},
							SSHPublicKey: "fake ssh key",
							NetworkInterfaces: []NetworkInterface{
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
		amt := amt
		t.Run(amt.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := admission.NewContextWithRequest(context.Background(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: ptr.To(true)}})
			_, err := amt.template.ValidateUpdate(ctx, amt.oldTemplate, amt.template)
			if amt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
	// dry-run=false
	for _, amt := range tests {
		amt := amt
		t.Run(amt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			ctx := admission.NewContextWithRequest(context.Background(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{DryRun: ptr.To(false)}})
			_, err := amt.template.ValidateUpdate(ctx, amt.oldTemplate, amt.template)
			if amt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func createAzureMachineTemplateFromMachine(machine *AzureMachine) *AzureMachineTemplate {
	return &AzureMachineTemplate{
		Spec: AzureMachineTemplateSpec{
			Template: AzureMachineTemplateResource{
				Spec: machine.Spec,
			},
		},
	}
}

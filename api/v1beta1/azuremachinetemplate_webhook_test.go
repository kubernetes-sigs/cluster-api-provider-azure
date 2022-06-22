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
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAzureMachineTemplate_ValidateCreate(t *testing.T) {
	g := NewWithT(t)

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
				createMachineWithUserAssignedIdentities([]UserAssignedIdentity{{ProviderID: "azure:///123"}, {ProviderID: "azure:///456"}}),
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
				createMachineWithOsDiskCacheType(string(compute.PossibleCachingTypesValues()[1])),
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
			name:            "azuremachinetemplate with RoleAssignmentName",
			machineTemplate: createAzureMachineTemplateFromMachine(createMachineWithRoleAssignmentName()),
			wantErr:         true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.machineTemplate.ValidateCreate()
			if test.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureMachineTemplate_ValidateUpdate(t *testing.T) {
	g := NewWithT(t)
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
								DiskSizeGB: to.Int32Ptr(11),
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
								DiskSizeGB: to.Int32Ptr(11),
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
								DiskSizeGB: to.Int32Ptr(11),
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
								DiskSizeGB: to.Int32Ptr(11),
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
								DiskSizeGB:  to.Int32Ptr(11),
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
								DiskSizeGB:  to.Int32Ptr(11),
								CachingType: "None",
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
			name: "AzureMachineTemplate ssh key removed",
			oldTemplate: &AzureMachineTemplate{
				Spec: AzureMachineTemplateSpec{
					Template: AzureMachineTemplateResource{
						Spec: AzureMachineSpec{
							VMSize:        "size",
							FailureDomain: &failureDomain,
							OSDisk: OSDisk{
								OSType:      "type",
								DiskSizeGB:  to.Int32Ptr(11),
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
								DiskSizeGB:  to.Int32Ptr(11),
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
	}

	for _, amt := range tests {
		amt := amt
		t.Run(amt.name, func(t *testing.T) {
			t.Parallel()
			err := amt.template.ValidateUpdate(amt.oldTemplate)
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

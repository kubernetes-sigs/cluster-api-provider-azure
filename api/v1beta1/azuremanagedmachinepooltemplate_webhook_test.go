/*
Copyright 2023 The Kubernetes Authors.

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

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	azureutil "sigs.k8s.io/cluster-api-provider-azure/util/azure"
)

func TestManagedMachinePoolTemplateDefaultingWebhook(t *testing.T) {
	g := NewWithT(t)

	t.Logf("Testing ammpt defaulting webhook with no baseline")
	ammpt := getAzureManagedMachinePoolTemplate()
	mmptw := &azureManagedMachinePoolTemplateWebhook{}
	err := mmptw.Default(context.Background(), ammpt)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ammpt.Labels).To(Equal(map[string]string{
		LabelAgentPoolMode: "System",
	}))
	g.Expect(ammpt.Spec.Template.Spec.Name).To(Equal(ptr.To("fooName")))
	g.Expect(ammpt.Spec.Template.Spec.OSType).To(Equal(ptr.To("Linux")))

	t.Logf("Testing ammpt defaulting webhook with baseline")
	ammpt = getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
		ammpt.Spec.Template.Spec.Mode = "User"
		ammpt.Spec.Template.Spec.Name = ptr.To("barName")
		ammpt.Spec.Template.Spec.OSType = ptr.To("Windows")
	})
	err = mmptw.Default(context.Background(), ammpt)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ammpt.Labels).To(Equal(map[string]string{
		LabelAgentPoolMode: "User",
	}))
	g.Expect(ammpt.Spec.Template.Spec.Name).To(Equal(ptr.To("barName")))
	g.Expect(ammpt.Spec.Template.Spec.OSType).To(Equal(ptr.To("Windows")))
}

func TestManagedMachinePoolTemplateUpdateWebhook(t *testing.T) {
	tests := []struct {
		name                   string
		oldMachinePoolTemplate *AzureManagedMachinePoolTemplate
		machinePoolTemplate    *AzureManagedMachinePoolTemplate
		wantErr                bool
	}{
		{
			name:                   "azuremanagedmachinepooltemplate no changes - valid spec",
			oldMachinePoolTemplate: getAzureManagedMachinePoolTemplate(),
			machinePoolTemplate:    getAzureManagedMachinePoolTemplate(),
			wantErr:                false,
		},
		{
			name:                   "azuremanagedmachinepooltemplate name is immutable",
			oldMachinePoolTemplate: getAzureManagedMachinePoolTemplate(),
			machinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.Name = ptr.To("barName")
			}),
			wantErr: true,
		},
		{
			name:                   "azuremanagedmachinepooltemplate invalid nodeLabel",
			oldMachinePoolTemplate: getAzureManagedMachinePoolTemplate(),
			machinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.NodeLabels = map[string]string{
					azureutil.AzureSystemNodeLabelPrefix: "foo",
				}
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedmachinepooltemplate osType is immutable",
			oldMachinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.OSType = ptr.To("Windows")
			}),
			machinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.OSType = ptr.To("Linux")
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedmachinepooltemplate SKU is immutable",
			oldMachinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.SKU = "Standard_D2s_v3"
			}),
			machinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.SKU = "Standard_D4s_v3"
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedmachinepooltemplate OSDiskSizeGB is immutable",
			oldMachinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.OSDiskSizeGB = ptr.To(128)
			}),
			machinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.OSDiskSizeGB = ptr.To(256)
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedmachinepooltemplate SubnetName is immutable",
			oldMachinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.SubnetName = ptr.To("fooSubnet")
			}),
			machinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.SubnetName = ptr.To("barSubnet")
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedmachinepooltemplate enableFIPS is immutable",
			oldMachinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.EnableFIPS = ptr.To(true)
			}),
			machinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.EnableFIPS = ptr.To(false)
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedmachinepooltemplate MaxPods is immutable",
			oldMachinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.MaxPods = ptr.To(128)
			}),
			machinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.MaxPods = ptr.To(256)
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedmachinepooltemplate OSDiskType is immutable",
			oldMachinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.OsDiskType = ptr.To("Standard_LRS")
			}),
			machinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.OsDiskType = ptr.To("Premium_LRS")
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedmachinepooltemplate scaleSetPriority is immutable",
			oldMachinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.ScaleSetPriority = ptr.To("Regular")
			}),
			machinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.ScaleSetPriority = ptr.To("Spot")
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedmachinepooltemplate enableUltraSSD is immutable",
			oldMachinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.EnableUltraSSD = ptr.To(true)
			}),
			machinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.EnableUltraSSD = ptr.To(false)
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedmachinepooltemplate enableNodePublicIP is immutable",
			oldMachinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.EnableNodePublicIP = ptr.To(true)
			}),
			machinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.EnableNodePublicIP = ptr.To(false)
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedmachinepooltemplate nodePublicIPPrefixID is immutable",
			oldMachinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.NodePublicIPPrefixID = ptr.To("fooPublicIPPrefixID")
			}),
			machinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.NodePublicIPPrefixID = ptr.To("barPublicIPPrefixID")
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedmachinepooltemplate kubeletConfig is immutable",
			oldMachinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.KubeletConfig = &KubeletConfig{}
			}),
			machinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.KubeletConfig = &KubeletConfig{
					FailSwapOn: ptr.To(true),
				}
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedmachinepooltemplate kubeletDiskType is immutable",
			oldMachinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.KubeletDiskType = ptr.To(KubeletDiskTypeOS)
			}),
			machinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.KubeletDiskType = ptr.To(KubeletDiskTypeTemporary)
			}),
			wantErr: true,
		},
		{
			name: "azuremanagedmachinepooltemplate linuxOSConfig is immutable",
			oldMachinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.LinuxOSConfig = &LinuxOSConfig{}
			}),
			machinePoolTemplate: getAzureManagedMachinePoolTemplate(func(ammpt *AzureManagedMachinePoolTemplate) {
				ammpt.Spec.Template.Spec.LinuxOSConfig = &LinuxOSConfig{
					SwapFileSizeMB: ptr.To(128),
				}
			}),
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			mpw := &azureManagedMachinePoolTemplateWebhook{}
			_, err := mpw.ValidateUpdate(context.Background(), tc.oldMachinePoolTemplate, tc.machinePoolTemplate)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func getAzureManagedMachinePoolTemplate(changes ...func(*AzureManagedMachinePoolTemplate)) *AzureManagedMachinePoolTemplate {
	input := &AzureManagedMachinePoolTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fooName",
		},
		Spec: AzureManagedMachinePoolTemplateSpec{
			Template: AzureManagedMachinePoolTemplateResource{
				Spec: AzureManagedMachinePoolTemplateResourceSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode: "System",
					},
				},
			},
		},
	}

	for _, change := range changes {
		change(input)
	}

	return input
}

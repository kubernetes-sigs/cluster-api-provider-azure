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

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-05-01/containerservice"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestAzureManagedMachinePoolDefaultingWebhook(t *testing.T) {
	g := NewWithT(t)

	t.Logf("Testing ammp defaulting webhook with mode system")
	ammp := &AzureManagedMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fooName",
		},
		Spec: AzureManagedMachinePoolSpec{
			Mode:         "System",
			SKU:          "StandardD2S_V3",
			OSDiskSizeGB: to.Int32Ptr(512),
		},
	}
	var client client.Client
	ammp.Default(client)
	g.Expect(ammp.Labels).ToNot(BeNil())
	val, ok := ammp.Labels[LabelAgentPoolMode]
	g.Expect(ok).To(BeTrue())
	g.Expect(val).To(Equal("System"))
	g.Expect(*ammp.Spec.Name).To(Equal("fooName"))

	t.Logf("Testing ammp defaulting webhook with empty string name specified in Spec")
	emptyName := ""
	ammp.Spec.Name = &emptyName
	ammp.Default(client)
	g.Expect(*ammp.Spec.Name).To(Equal("fooName"))

	t.Logf("Testing ammp defaulting webhook with normal name specified in Spec")
	normalName := "barName"
	ammp.Spec.Name = &normalName
	ammp.Default(client)
	g.Expect(*ammp.Spec.Name).To(Equal("barName"))

	t.Logf("Testing ammp defaulting webhook with normal OsDiskType specified in Spec")
	normalOsDiskType := "Ephemeral"
	ammp.Spec.OsDiskType = &normalOsDiskType
	ammp.Default(client)
	g.Expect(*ammp.Spec.OsDiskType).To(Equal("Ephemeral"))
}

func TestAzureManagedMachinePoolUpdatingWebhook(t *testing.T) {
	g := NewWithT(t)

	t.Logf("Testing ammp updating webhook with mode system")

	tests := []struct {
		name    string
		new     *AzureManagedMachinePool
		old     *AzureManagedMachinePool
		wantErr bool
	}{
		{
			name: "Cannot change SKU of the agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:         "System",
					SKU:          "StandardD2S_V3",
					OSDiskSizeGB: to.Int32Ptr(512),
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:         "System",
					SKU:          "StandardD2S_V4",
					OSDiskSizeGB: to.Int32Ptr(512),
				},
			},
			wantErr: true,
		},
		{
			name: "Cannot change OSDiskSizeGB of the agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:         "System",
					SKU:          "StandardD2S_V3",
					OSDiskSizeGB: to.Int32Ptr(512),
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:         "System",
					SKU:          "StandardD2S_V3",
					OSDiskSizeGB: to.Int32Ptr(1024),
				},
			},
			wantErr: true,
		},
		{
			name: "Cannot add AvailabilityZones after creating agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:              "System",
					SKU:               "StandardD2S_V3",
					OSDiskSizeGB:      to.Int32Ptr(512),
					AvailabilityZones: []string{"1", "2", "3"},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:         "System",
					SKU:          "StandardD2S_V3",
					OSDiskSizeGB: to.Int32Ptr(512),
				},
			},
			wantErr: true,
		},
		{
			name: "Cannot remove AvailabilityZones after creating agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:         "System",
					SKU:          "StandardD2S_V3",
					OSDiskSizeGB: to.Int32Ptr(512),
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:              "System",
					SKU:               "StandardD2S_V3",
					OSDiskSizeGB:      to.Int32Ptr(512),
					AvailabilityZones: []string{"1", "2", "3"},
				},
			},
			wantErr: true,
		},
		{
			name: "Cannot change AvailabilityZones of the agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:              "System",
					SKU:               "StandardD2S_V3",
					OSDiskSizeGB:      to.Int32Ptr(512),
					AvailabilityZones: []string{"1", "2"},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:              "System",
					SKU:               "StandardD2S_V3",
					OSDiskSizeGB:      to.Int32Ptr(512),
					AvailabilityZones: []string{"1", "2", "3"},
				},
			},
			wantErr: true,
		},
		{
			name: "AvailabilityZones order can be different",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:              "System",
					SKU:               "StandardD2S_V3",
					OSDiskSizeGB:      to.Int32Ptr(512),
					AvailabilityZones: []string{"1", "3", "2"},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:              "System",
					SKU:               "StandardD2S_V3",
					OSDiskSizeGB:      to.Int32Ptr(512),
					AvailabilityZones: []string{"1", "2", "3"},
				},
			},
			wantErr: false,
		},
		{
			name: "Cannot change MaxPods of the agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:         "System",
					SKU:          "StandardD2S_V3",
					OSDiskSizeGB: to.Int32Ptr(512),
					MaxPods:      to.Int32Ptr(24),
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:         "System",
					SKU:          "StandardD2S_V3",
					OSDiskSizeGB: to.Int32Ptr(512),
					MaxPods:      to.Int32Ptr(25),
				},
			},
			wantErr: true,
		},
		{
			name: "Unchanged MaxPods in an agentpool should not result in an error",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:         "System",
					SKU:          "StandardD2S_V3",
					OSDiskSizeGB: to.Int32Ptr(512),
					MaxPods:      to.Int32Ptr(30),
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:         "System",
					SKU:          "StandardD2S_V3",
					OSDiskSizeGB: to.Int32Ptr(512),
					MaxPods:      to.Int32Ptr(30),
				},
			},
			wantErr: false,
		},
		{
			name: "Cannot change OSDiskType of the agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:         "System",
					SKU:          "StandardD2S_V3",
					OSDiskSizeGB: to.Int32Ptr(512),
					MaxPods:      to.Int32Ptr(24),
					OsDiskType:   to.StringPtr(string(containerservice.OSDiskTypeEphemeral)),
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:         "System",
					SKU:          "StandardD2S_V3",
					OSDiskSizeGB: to.Int32Ptr(512),
					MaxPods:      to.Int32Ptr(24),
					OsDiskType:   to.StringPtr(string(containerservice.OSDiskTypeManaged)),
				},
			},
			wantErr: true,
		},
		{
			name: "custom header annotation values are immutable",
			old: &AzureManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"infrastructure.cluster.x-k8s.io/custom-header-SomeFeature": "true",
					},
				},
				Spec: AzureManagedMachinePoolSpec{},
			},
			new: &AzureManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"infrastructure.cluster.x-k8s.io/custom-header-SomeFeature": "false",
					},
				},
				Spec: AzureManagedMachinePoolSpec{},
			},
			wantErr: true,
		},
		{
			name: "cannot remove custom header annotation after resource creation",
			old: &AzureManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"infrastructure.cluster.x-k8s.io/custom-header-SomeFeature": "true",
					},
				},
				Spec: AzureManagedMachinePoolSpec{},
			},
			new: &AzureManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{},
				},
				Spec: AzureManagedMachinePoolSpec{},
			},
			wantErr: true,
		},
		{
			name: "cannot add new custom header annotations after resource creation",
			old: &AzureManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"infrastructure.cluster.x-k8s.io/custom-header-SomeFeature": "true",
					},
				},
				Spec: AzureManagedMachinePoolSpec{},
			},
			new: &AzureManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"infrastructure.cluster.x-k8s.io/custom-header-SomeFeature":    "true",
						"infrastructure.cluster.x-k8s.io/custom-header-AnotherFeature": "true",
					},
				},
				Spec: AzureManagedMachinePoolSpec{},
			},
			wantErr: true,
		},
		{
			name: "non-custom headers annotations are mutable",
			old: &AzureManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"annotation-a": "true",
						"infrastructure.cluster.x-k8s.io/custom-header-SomeFeature": "true",
					},
				},
				Spec: AzureManagedMachinePoolSpec{},
			},
			new: &AzureManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"infrastructure.cluster.x-k8s.io/custom-header-SomeFeature": "true",
						"annotation-b": "true",
					},
				},
				Spec: AzureManagedMachinePoolSpec{},
			},
			wantErr: false,
		},
		{
			name: "Unchanged OSDiskType in an agentpool should not result in an error",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:         "System",
					SKU:          "StandardD2S_V3",
					OSDiskSizeGB: to.Int32Ptr(512),
					MaxPods:      to.Int32Ptr(30),
					OsDiskType:   to.StringPtr(string(containerservice.OSDiskTypeManaged)),
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:         "System",
					SKU:          "StandardD2S_V3",
					OSDiskSizeGB: to.Int32Ptr(512),
					MaxPods:      to.Int32Ptr(30),
					OsDiskType:   to.StringPtr(string(containerservice.OSDiskTypeManaged)),
				},
			},
			wantErr: false,
		},
		{
			name: "Unexpected error, value EnableUltraSSD is unchanged",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableUltraSSD: to.BoolPtr(true),
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableUltraSSD: to.BoolPtr(true),
				},
			},
			wantErr: false,
		},
		{
			name: "Unexpected error, value EnableUltraSSD is unchanged",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableUltraSSD: to.BoolPtr(false),
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableUltraSSD: to.BoolPtr(false),
				},
			},
			wantErr: false,
		},
		{
			name: "EnableUltraSSD feature is immutable and currently enabled on this agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableUltraSSD: to.BoolPtr(false),
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableUltraSSD: to.BoolPtr(true),
				},
			},
			wantErr: true,
		},
		{
			name: "EnableUltraSSD feature is immutable and currently disabled on this agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableUltraSSD: to.BoolPtr(true),
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableUltraSSD: to.BoolPtr(false),
				},
			},
			wantErr: true,
		},
		{
			name: "EnableUltraSSD feature is immutable and currently disabled on this agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableUltraSSD: nil,
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableUltraSSD: to.BoolPtr(false),
				},
			},
			wantErr: true,
		},
		{
			name: "EnableUltraSSD feature is immutable and currently enabled on this agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableUltraSSD: nil,
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableUltraSSD: to.BoolPtr(true),
				},
			},
			wantErr: true,
		},
		{
			name: "Cannot change EnableFIPS of the agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:         "System",
					SKU:          "StandardD2S_V3",
					OSDiskSizeGB: to.Int32Ptr(512),
					EnableFIPS:   to.BoolPtr(true),
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:         "System",
					SKU:          "StandardD2S_V3",
					OSDiskSizeGB: to.Int32Ptr(512),
					EnableFIPS:   to.BoolPtr(false),
				},
			},
			wantErr: true,
		},
		{
			name: "Cannot change EnableNodePublicIP of the agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:               "System",
					SKU:                "StandardD2S_V3",
					OSDiskSizeGB:       to.Int32Ptr(512),
					EnableNodePublicIP: to.BoolPtr(true),
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:               "System",
					SKU:                "StandardD2S_V3",
					OSDiskSizeGB:       to.Int32Ptr(512),
					EnableNodePublicIP: to.BoolPtr(false),
				},
			},
			wantErr: true,
		},
		{
			name: "Cannot change ScaleSetPriority of the agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:             "System",
					SKU:              "StandardD2S_V3",
					OSDiskSizeGB:     to.Int32Ptr(512),
					ScaleSetPriority: to.StringPtr("Regular"),
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:             "System",
					SKU:              "StandardD2S_V3",
					OSDiskSizeGB:     to.Int32Ptr(512),
					ScaleSetPriority: to.StringPtr("Spot"),
				},
			},
			wantErr: true,
		},
		{
			name: "Cannot change EnableEncryptionAtHost of the agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:                   "System",
					SKU:                    "StandardD2S_V3",
					OSDiskSizeGB:           to.Int32Ptr(512),
					EnableEncryptionAtHost: to.BoolPtr(true),
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:                   "System",
					SKU:                    "StandardD2S_V3",
					OSDiskSizeGB:           to.Int32Ptr(512),
					EnableEncryptionAtHost: to.BoolPtr(false),
				},
			},
			wantErr: true,
		},
	}
	var client client.Client
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.new.ValidateUpdate(tc.old, client)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureManagedMachinePool_ValidateCreate(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name     string
		ammp     *AzureManagedMachinePool
		wantErr  bool
		errorLen int
	}{
		{
			name: "valid",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					MaxPods:    to.Int32Ptr(30),
					OsDiskType: to.StringPtr(string(containerservice.OSDiskTypeEphemeral)),
				},
			},
			wantErr: false,
		},
		{
			name: "another valid permutation",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					MaxPods:    to.Int32Ptr(249),
					OsDiskType: to.StringPtr(string(containerservice.OSDiskTypeManaged)),
				},
			},
			wantErr: false,
		},
		{
			name: "valid - optional configuration not present",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{},
			},
			wantErr: false,
		},
		{
			name: "too many MaxPods",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					MaxPods: to.Int32Ptr(251),
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "too few MaxPods",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					MaxPods: to.Int32Ptr(9),
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "valid AllowedUnsafeSysctls",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:          "User",
					SKU:           "StandardD2S_V3",
					OSDiskSizeGB:  to.Int32Ptr(512),
					KubeletConfig: &KubeletConfig{AllowedUnsafeSysctls: &[]string{"net.*"}},
				},
			},
			wantErr: false,
		},
		{
			name: "AllowedUnsafeSysctls should be one of \"kernel.shm*\", \"kernel.msg*\", \"kernel.sem\", \"fs.mqueue.*\", \"net.*\"",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:          "User",
					SKU:           "StandardD2S_V3",
					OSDiskSizeGB:  to.Int32Ptr(512),
					KubeletConfig: &KubeletConfig{AllowedUnsafeSysctls: &[]string{"net4.*"}},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
	}
	var client client.Client
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.ammp.ValidateCreate(client)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(HaveLen(tc.errorLen))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

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

	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231001"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilfeature "k8s.io/component-base/featuregate/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api-provider-azure/feature"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterctlv1alpha3 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	capifeature "sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAzureManagedMachinePoolDefaultingWebhook(t *testing.T) {
	g := NewWithT(t)

	t.Logf("Testing ammp defaulting webhook with mode system")
	ammp := &AzureManagedMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fooname",
		},
		Spec: AzureManagedMachinePoolSpec{
			AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
				Mode:         "System",
				SKU:          "StandardD2S_V3",
				OSDiskSizeGB: ptr.To(512),
			},
		},
	}
	var client client.Client
	mw := &azureManagedMachinePoolWebhook{
		Client: client,
	}
	err := mw.Default(context.Background(), ammp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(ammp.Labels).NotTo(BeNil())
	val, ok := ammp.Labels[LabelAgentPoolMode]
	g.Expect(ok).To(BeTrue())
	g.Expect(val).To(Equal("System"))
	g.Expect(*ammp.Spec.Name).To(Equal("fooname"))
	g.Expect(*ammp.Spec.OSType).To(Equal(LinuxOS))

	t.Logf("Testing ammp defaulting webhook with empty string name specified in Spec")
	emptyName := ""
	ammp.Spec.Name = &emptyName
	err = mw.Default(context.Background(), ammp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(*ammp.Spec.Name).To(Equal("fooname"))

	t.Logf("Testing ammp defaulting webhook with normal name specified in Spec")
	normalName := "barname"
	ammp.Spec.Name = &normalName
	err = mw.Default(context.Background(), ammp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(*ammp.Spec.Name).To(Equal("barname"))

	t.Logf("Testing ammp defaulting webhook with normal OsDiskType specified in Spec")
	normalOsDiskType := "Ephemeral"
	ammp.Spec.OsDiskType = &normalOsDiskType
	err = mw.Default(context.Background(), ammp)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(*ammp.Spec.OsDiskType).To(Equal("Ephemeral"))
}

func TestAzureManagedMachinePoolUpdatingWebhook(t *testing.T) {
	t.Logf("Testing ammp updating webhook with mode system")

	tests := []struct {
		name    string
		new     *AzureManagedMachinePool
		old     *AzureManagedMachinePool
		wantErr bool
	}{
		{
			name: "Cannot change Name of the agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Name: ptr.To("pool-new"),
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Name: ptr.To("pool-old"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Cannot change SKU of the agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:         "System",
						SKU:          "StandardD2S_V3",
						OSDiskSizeGB: ptr.To(512),
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:         "System",
						SKU:          "StandardD2S_V4",
						OSDiskSizeGB: ptr.To(512),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Cannot change OSType of the agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						OSType:       ptr.To(LinuxOS),
						Mode:         "System",
						SKU:          "StandardD2S_V3",
						OSDiskSizeGB: ptr.To(512),
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						OSType:       ptr.To(WindowsOS),
						Mode:         "System",
						SKU:          "StandardD2S_V4",
						OSDiskSizeGB: ptr.To(512),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Cannot change OSDiskSizeGB of the agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:         "System",
						SKU:          "StandardD2S_V3",
						OSDiskSizeGB: ptr.To(512),
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:         "System",
						SKU:          "StandardD2S_V3",
						OSDiskSizeGB: ptr.To(1024),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Cannot add AvailabilityZones after creating agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:              "System",
						SKU:               "StandardD2S_V3",
						OSDiskSizeGB:      ptr.To(512),
						AvailabilityZones: []string{"1", "2", "3"},
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:         "System",
						SKU:          "StandardD2S_V3",
						OSDiskSizeGB: ptr.To(512),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Cannot remove AvailabilityZones after creating agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:         "System",
						SKU:          "StandardD2S_V3",
						OSDiskSizeGB: ptr.To(512),
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:              "System",
						SKU:               "StandardD2S_V3",
						OSDiskSizeGB:      ptr.To(512),
						AvailabilityZones: []string{"1", "2", "3"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Cannot change AvailabilityZones of the agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:              "System",
						SKU:               "StandardD2S_V3",
						OSDiskSizeGB:      ptr.To(512),
						AvailabilityZones: []string{"1", "2"},
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:              "System",
						SKU:               "StandardD2S_V3",
						OSDiskSizeGB:      ptr.To(512),
						AvailabilityZones: []string{"1", "2", "3"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "AvailabilityZones order can be different",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:              "System",
						SKU:               "StandardD2S_V3",
						OSDiskSizeGB:      ptr.To(512),
						AvailabilityZones: []string{"1", "3", "2"},
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:              "System",
						SKU:               "StandardD2S_V3",
						OSDiskSizeGB:      ptr.To(512),
						AvailabilityZones: []string{"1", "2", "3"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Cannot change MaxPods of the agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:         "System",
						SKU:          "StandardD2S_V3",
						OSDiskSizeGB: ptr.To(512),
						MaxPods:      ptr.To(24),
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:         "System",
						SKU:          "StandardD2S_V3",
						OSDiskSizeGB: ptr.To(512),
						MaxPods:      ptr.To(25),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Unchanged MaxPods in an agentpool should not result in an error",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:         "System",
						SKU:          "StandardD2S_V3",
						OSDiskSizeGB: ptr.To(512),
						MaxPods:      ptr.To(30),
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:         "System",
						SKU:          "StandardD2S_V3",
						OSDiskSizeGB: ptr.To(512),
						MaxPods:      ptr.To(30),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Cannot change OSDiskType of the agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:         "System",
						SKU:          "StandardD2S_V3",
						OSDiskSizeGB: ptr.To(512),
						MaxPods:      ptr.To(24),
						OsDiskType:   ptr.To(string(asocontainerservicev1.OSDiskType_Ephemeral)),
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:         "System",
						SKU:          "StandardD2S_V3",
						OSDiskSizeGB: ptr.To(512),
						MaxPods:      ptr.To(24),
						OsDiskType:   ptr.To(string(asocontainerservicev1.OSDiskType_Managed)),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Unchanged OSDiskType in an agentpool should not result in an error",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:         "System",
						SKU:          "StandardD2S_V3",
						OSDiskSizeGB: ptr.To(512),
						MaxPods:      ptr.To(30),
						OsDiskType:   ptr.To(string(asocontainerservicev1.OSDiskType_Managed)),
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:         "System",
						SKU:          "StandardD2S_V3",
						OSDiskSizeGB: ptr.To(512),
						MaxPods:      ptr.To(30),
						OsDiskType:   ptr.To(string(asocontainerservicev1.OSDiskType_Managed)),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Unexpected error, value EnableUltraSSD is unchanged",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						EnableUltraSSD: ptr.To(true),
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						EnableUltraSSD: ptr.To(true),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "EnableUltraSSD feature is immutable and currently enabled on this agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						EnableUltraSSD: ptr.To(false),
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						EnableUltraSSD: ptr.To(true),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Unexpected error, value EnableNodePublicIP is unchanged",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						EnableNodePublicIP: ptr.To(true),
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						EnableNodePublicIP: ptr.To(true),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "EnableNodePublicIP feature is immutable and currently enabled on this agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						EnableNodePublicIP: ptr.To(false),
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						EnableNodePublicIP: ptr.To(true),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "NodeTaints are mutable",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Taints: []Taint{
							{
								Effect: TaintEffect("NoSchedule"),
								Key:    "foo",
								Value:  "baz",
							},
						},
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Taints: []Taint{
							{
								Effect: TaintEffect("NoSchedule"),
								Key:    "foo",
								Value:  "bar",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Can't add a node label that begins with kubernetes.azure.com",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						NodeLabels: map[string]string{
							"foo":                                   "bar",
							"kubernetes.azure.com/scalesetpriority": "spot",
						},
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						NodeLabels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Can't update kubeletconfig",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						KubeletConfig: &KubeletConfig{
							CPUCfsQuota: ptr.To(true),
						},
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						KubeletConfig: &KubeletConfig{
							CPUCfsQuota: ptr.To(false),
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Can't update LinuxOSConfig",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						LinuxOSConfig: &LinuxOSConfig{
							SwapFileSizeMB: ptr.To(10),
						},
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						LinuxOSConfig: &LinuxOSConfig{
							SwapFileSizeMB: ptr.To(5),
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Can't update SubnetName with error",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						SubnetName: ptr.To("my-subnet"),
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						SubnetName: ptr.To("my-subnet-1"),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Can update SubnetName if subnetName is empty",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						SubnetName: ptr.To("my-subnet"),
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						SubnetName: nil,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Can't update SubnetName without error",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						SubnetName: ptr.To("my-subnet"),
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						SubnetName: ptr.To("my-subnet"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Cannot update enableFIPS",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						EnableFIPS: ptr.To(true),
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						EnableFIPS: ptr.To(false),
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Cannot update enableEncryptionAtHost",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						EnableEncryptionAtHost: ptr.To(true),
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						EnableEncryptionAtHost: ptr.To(false),
					},
				},
			},
			wantErr: true,
		},
	}
	var client client.Client
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			mw := &azureManagedMachinePoolWebhook{
				Client: client,
			}
			_, err := mw.ValidateUpdate(context.Background(), tc.old, tc.new)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureManagedMachinePool_ValidateCreate(t *testing.T) {
	tests := []struct {
		name     string
		ammp     *AzureManagedMachinePool
		wantErr  bool
		errorLen int
	}{
		{
			name:    "valid",
			ammp:    getKnownValidAzureManagedMachinePool(),
			wantErr: false,
		},
		{
			name: "another valid permutation",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						MaxPods:    ptr.To(249),
						OsDiskType: ptr.To(string(asocontainerservicev1.OSDiskType_Managed)),
					},
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
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						MaxPods: ptr.To(251),
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "invalid subnetname",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						SubnetName: ptr.To("1+subnet"),
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "invalid subnetname",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						SubnetName: ptr.To("1"),
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "invalid subnetname",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						SubnetName: ptr.To("-a_b-c"),
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "invalid subnetname with versioning",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						SubnetName: ptr.To("workload-ampt-v0.1.0."),
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "invalid subnetname",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						SubnetName: ptr.To("-_-_"),
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "invalid subnetname",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						SubnetName: ptr.To("abc@#$"),
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "invalid subnetname with character length 81",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						SubnetName: ptr.To("3DgIb8EZMkLs0KlyPaTcNxoJU9ufmW6jvXrweqz1hVp5nS4RtH2QY7AFOiC5nS4RtH2QY7AFOiC3DgIb8"),
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "valid subnetname with character length 80",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						SubnetName: ptr.To("3DgIb8EZMkLs0KlyPaTcNxoJU9ufmW6jvXrweqz1hVp5nS4RtH2QY7AFOiC5nS4RtH2QY7AFOiC3DgIb"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid subnetname with versioning",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						SubnetName: ptr.To("workload-ampt-v0.1.0"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid subnetname",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						SubnetName: ptr.To("1abc"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid subnetname",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						SubnetName: ptr.To("1-a-b-c"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid subnetname",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						SubnetName: ptr.To("my-subnet"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "too few MaxPods",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						MaxPods: ptr.To(9),
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "ostype Windows with System mode not allowed",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:   "System",
						OSType: ptr.To(WindowsOS),
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "ostype windows with User mode",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:   "User",
						OSType: ptr.To(WindowsOS),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Windows clusters with 6char or less name",
			ammp: &AzureManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool0",
				},
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:   "User",
						OSType: ptr.To(WindowsOS),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Windows clusters with more than 6char names are not allowed",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Name:   ptr.To("pool0-name-too-long"),
						Mode:   "User",
						OSType: ptr.To(WindowsOS),
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "valid label",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:   "User",
						OSType: ptr.To(LinuxOS),
						NodeLabels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "kubernetes.azure.com label",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						Mode:   "User",
						OSType: ptr.To(LinuxOS),
						NodeLabels: map[string]string{
							"kubernetes.azure.com/scalesetpriority": "spot",
						},
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "pool with invalid public ip prefix",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						EnableNodePublicIP:   ptr.To(true),
						NodePublicIPPrefixID: ptr.To("not a valid resource ID"),
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "pool with public ip prefix cannot omit node public IP",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						EnableNodePublicIP:   nil,
						NodePublicIPPrefixID: ptr.To("subscriptions/11111111-2222-aaaa-bbbb-cccccccccccc/resourceGroups/public-ip-test/providers/Microsoft.Network/publicipprefixes/public-ip-prefix"),
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "pool with public ip prefix cannot disable node public IP",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						EnableNodePublicIP:   ptr.To(false),
						NodePublicIPPrefixID: ptr.To("subscriptions/11111111-2222-aaaa-bbbb-cccccccccccc/resourceGroups/public-ip-test/providers/Microsoft.Network/publicipprefixes/public-ip-prefix"),
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "pool with public ip prefix with node public IP enabled ok",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						EnableNodePublicIP:   ptr.To(true),
						NodePublicIPPrefixID: ptr.To("subscriptions/11111111-2222-aaaa-bbbb-cccccccccccc/resourceGroups/public-ip-test/providers/Microsoft.Network/publicipprefixes/public-ip-prefix"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "pool with public ip prefix with leading slash with node public IP enabled ok",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						EnableNodePublicIP:   ptr.To(true),
						NodePublicIPPrefixID: ptr.To("/subscriptions/11111111-2222-aaaa-bbbb-cccccccccccc/resourceGroups/public-ip-test/providers/Microsoft.Network/publicipprefixes/public-ip-prefix"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "pool without public ip prefix with node public IP unset ok",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						EnableNodePublicIP: nil,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "pool without public ip prefix with node public IP enabled ok",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						EnableNodePublicIP: ptr.To(true),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "pool without public ip prefix with node public IP disabled ok",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						EnableNodePublicIP: ptr.To(false),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "KubeletConfig CPUCfsQuotaPeriod needs 'ms' suffix",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						KubeletConfig: &KubeletConfig{
							CPUCfsQuotaPeriod: ptr.To("100"),
						},
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "KubeletConfig CPUCfsQuotaPeriod has valid 'ms' suffix",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						KubeletConfig: &KubeletConfig{
							CPUCfsQuotaPeriod: ptr.To("100ms"),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "KubeletConfig ImageGcLowThreshold can't be more than ImageGcHighThreshold",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						KubeletConfig: &KubeletConfig{
							ImageGcLowThreshold:  ptr.To(100),
							ImageGcHighThreshold: ptr.To(99),
						},
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "KubeletConfig ImageGcLowThreshold is lower than ImageGcHighThreshold",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						KubeletConfig: &KubeletConfig{
							ImageGcLowThreshold:  ptr.To(99),
							ImageGcHighThreshold: ptr.To(100),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid KubeletConfig AllowedUnsafeSysctls values",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						KubeletConfig: &KubeletConfig{
							AllowedUnsafeSysctls: []string{
								"kernel.shm*",
								"kernel.msg*",
								"kernel.sem",
								"fs.mqueue.*",
								"net.*",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "more valid KubeletConfig AllowedUnsafeSysctls values",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						KubeletConfig: &KubeletConfig{
							AllowedUnsafeSysctls: []string{
								"kernel.shm.something",
								"kernel.msg.foo.bar",
								"kernel.sem",
								"fs.mqueue.baz",
								"net.my.configuration.path",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "an invalid KubeletConfig AllowedUnsafeSysctls value in a set",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						KubeletConfig: &KubeletConfig{
							AllowedUnsafeSysctls: []string{
								"kernel.shm.something",
								"kernel.msg.foo.bar",
								"kernel.sem",
								"fs.mqueue.baz",
								"net.my.configuration.path",
								"kernel.not.allowed",
							},
						},
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "validLinuxOSConfig Sysctls NetIpv4IpLocalPortRange.First is less than NetIpv4IpLocalPortRange.Last",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						LinuxOSConfig: &LinuxOSConfig{
							Sysctls: &SysctlConfig{
								NetIpv4IPLocalPortRange: ptr.To("2000 33000"),
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "an invalid LinuxOSConfig Sysctls NetIpv4IpLocalPortRange.First string is ill-formed",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						LinuxOSConfig: &LinuxOSConfig{
							Sysctls: &SysctlConfig{
								NetIpv4IPLocalPortRange: ptr.To("wrong 33000"),
							},
						},
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "an invalid LinuxOSConfig Sysctls NetIpv4IpLocalPortRange.Last string is ill-formed",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						LinuxOSConfig: &LinuxOSConfig{
							Sysctls: &SysctlConfig{
								NetIpv4IPLocalPortRange: ptr.To("2000 wrong"),
							},
						},
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "an invalid LinuxOSConfig Sysctls NetIpv4IpLocalPortRange.First less than allowed value",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						LinuxOSConfig: &LinuxOSConfig{
							Sysctls: &SysctlConfig{
								NetIpv4IPLocalPortRange: ptr.To("1020 32999"),
							},
						},
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "an invalid LinuxOSConfig Sysctls NetIpv4IpLocalPortRange.Last less than allowed value",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						LinuxOSConfig: &LinuxOSConfig{
							Sysctls: &SysctlConfig{
								NetIpv4IPLocalPortRange: ptr.To("1024 32000"),
							},
						},
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "an invalid LinuxOSConfig Sysctls NetIpv4IpLocalPortRange.First is greater than NetIpv4IpLocalPortRange.Last",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						LinuxOSConfig: &LinuxOSConfig{
							Sysctls: &SysctlConfig{
								NetIpv4IPLocalPortRange: ptr.To("33000 32999"),
							},
						},
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "valid LinuxOSConfig Sysctls is set by disabling FailSwapOn",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						KubeletConfig: &KubeletConfig{
							FailSwapOn: ptr.To(false),
						},
						LinuxOSConfig: &LinuxOSConfig{
							SwapFileSizeMB: ptr.To(1500),
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "an invalid LinuxOSConfig Sysctls is set with FailSwapOn set to true",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						KubeletConfig: &KubeletConfig{
							FailSwapOn: ptr.To(true),
						},
						LinuxOSConfig: &LinuxOSConfig{
							SwapFileSizeMB: ptr.To(1500),
						},
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "an invalid LinuxOSConfig Sysctls is set without disabling FailSwapOn",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
						LinuxOSConfig: &LinuxOSConfig{
							SwapFileSizeMB: ptr.To(1500),
						},
					},
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
	}

	var client client.Client
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			mw := &azureManagedMachinePoolWebhook{
				Client: client,
			}
			_, err := mw.ValidateCreate(context.Background(), tc.ammp)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(HaveLen(tc.errorLen))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureManagedMachinePool_ValidateCreateFailure(t *testing.T) {
	tests := []struct {
		name        string
		ammp        *AzureManagedMachinePool
		deferFunc   func()
		expectError bool
	}{
		{
			name:        "feature gate explicitly disabled",
			ammp:        getKnownValidAzureManagedMachinePool(),
			deferFunc:   utilfeature.SetFeatureGateDuringTest(t, feature.Gates, capifeature.MachinePool, false),
			expectError: true,
		},
		{
			name:        "feature gate implicitly enabled",
			ammp:        getKnownValidAzureManagedMachinePool(),
			deferFunc:   func() {},
			expectError: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.deferFunc()
			g := NewWithT(t)
			mw := &azureManagedMachinePoolWebhook{}
			_, err := mw.ValidateCreate(context.Background(), tc.ammp)
			if tc.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestAzureManagedMachinePool_validateLastSystemNodePool(t *testing.T) {
	deletionTime := metav1.Now()
	finalizers := []string{"test"}
	systemMachinePool := getManagedMachinePoolWithSystemMode()
	systemMachinePoolWithDeletionAnnotation := getAzureManagedMachinePoolWithChanges(
		// Add the DeleteForMoveAnnotation annotation to the AMMP
		func(azureManagedMachinePool *AzureManagedMachinePool) {
			azureManagedMachinePool.Annotations = map[string]string{
				clusterctlv1alpha3.DeleteForMoveAnnotation: "true",
			}
		},
	)
	tests := []struct {
		name    string
		ammp    *AzureManagedMachinePool
		cluster *clusterv1.Cluster
		wantErr bool
	}{
		{
			// AzureManagedMachinePool will be deleted since AMMP has DeleteForMoveAnnotation annotation
			// Note that Owner Cluster's deletion timestamp is nil and Owner cluster being paused does not matter anymore.
			name: "AzureManagedMachinePool (AMMP) should be deleted if this AMMP has the annotation 'cluster.x-k8s.io/move-to-delete' with the owner cluster being paused and 'No' deletion timestamp",
			ammp: systemMachinePoolWithDeletionAnnotation,
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:       systemMachinePool.GetLabels()[clusterv1.ClusterNameLabel],
					Namespace:  systemMachinePool.Namespace,
					Finalizers: finalizers,
				},
			},
			wantErr: false,
		},
		{
			// AzureManagedMachinePool will be deleted since Owner Cluster has been marked for deletion
			name: "AzureManagedMachinePool should be deleted since the Cluster is paused with a deletion timestamp",
			ammp: systemMachinePool,
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:              systemMachinePool.GetLabels()[clusterv1.ClusterNameLabel],
					Namespace:         systemMachinePool.Namespace,
					DeletionTimestamp: &deletionTime,
					Finalizers:        finalizers,
				},
			},
			wantErr: false,
		},
		{
			name: "AzureManagedMachinePool should not be deleted without a deletion timestamp on Owner Cluster and having one system pool node(invalid delete)",
			ammp: systemMachinePool,
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      systemMachinePool.GetLabels()[clusterv1.ClusterNameLabel],
					Namespace: systemMachinePool.Namespace,
				},
			},
			wantErr: true,
		},
		{
			name: "AzureManagedMachinePool should be deleted when Cluster is set with a deletion timestamp having one system pool node(valid delete)",
			ammp: systemMachinePool,
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:              systemMachinePool.GetLabels()[clusterv1.ClusterNameLabel],
					Namespace:         systemMachinePool.Namespace,
					DeletionTimestamp: &deletionTime,
					Finalizers:        finalizers,
				},
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			scheme := runtime.NewScheme()
			_ = AddToScheme(scheme)
			_ = clusterv1.AddToScheme(scheme)
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tc.cluster, tc.ammp).Build()
			err := validateLastSystemNodePool(fakeClient, tc.ammp.Spec.NodeLabels, tc.ammp.Namespace, tc.ammp.Annotations)
			if tc.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func getKnownValidAzureManagedMachinePool() *AzureManagedMachinePool {
	return &AzureManagedMachinePool{
		Spec: AzureManagedMachinePoolSpec{
			AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
				MaxPods:    ptr.To(30),
				OsDiskType: ptr.To(string(asocontainerservicev1.OSDiskType_Ephemeral)),
			},
		},
	}
}

func getManagedMachinePoolWithSystemMode() *AzureManagedMachinePool {
	return &AzureManagedMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "test-cluster",
				LabelAgentPoolMode:         string(NodePoolModeSystem),
			},
		},
		Spec: AzureManagedMachinePoolSpec{
			AzureManagedMachinePoolClassSpec: AzureManagedMachinePoolClassSpec{
				NodeLabels: map[string]string{
					clusterv1.ClusterNameLabel: "test-cluster",
				},
			},
		},
	}
}

func getAzureManagedMachinePoolWithChanges(changes ...func(*AzureManagedMachinePool)) *AzureManagedMachinePool {
	ammp := getManagedMachinePoolWithSystemMode().DeepCopy()
	for _, change := range changes {
		change(ammp)
	}
	return ammp
}

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
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2022-03-01/containerservice"
	"github.com/Azure/go-autorest/autorest/to"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilfeature "k8s.io/component-base/featuregate/testing"
	"sigs.k8s.io/cluster-api-provider-azure/feature"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capifeature "sigs.k8s.io/cluster-api/feature"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
	g.Expect(ammp.Labels).NotTo(BeNil())
	val, ok := ammp.Labels[LabelAgentPoolMode]
	g.Expect(ok).To(BeTrue())
	g.Expect(val).To(Equal("System"))
	g.Expect(*ammp.Spec.Name).To(Equal("fooName"))
	g.Expect(*ammp.Spec.OSType).To(Equal(LinuxOS))

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
			name: "Cannot change Name of the agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Name: to.StringPtr("pool-new"),
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Name: to.StringPtr("pool-old"),
				},
			},
			wantErr: true,
		},
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
			name: "Cannot change OSType of the agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					OSType:       to.StringPtr(LinuxOS),
					Mode:         "System",
					SKU:          "StandardD2S_V3",
					OSDiskSizeGB: to.Int32Ptr(512),
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					OSType:       to.StringPtr(WindowsOS),
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
			name: "Unexpected error, value EnableNodePublicIP is unchanged",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableNodePublicIP: to.BoolPtr(true),
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableNodePublicIP: to.BoolPtr(true),
				},
			},
			wantErr: false,
		},
		{
			name: "EnableNodePublicIP feature is immutable and currently enabled on this agentpool",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableNodePublicIP: to.BoolPtr(false),
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableNodePublicIP: to.BoolPtr(true),
				},
			},
			wantErr: true,
		},
		{
			name: "NodeTaints are mutable",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Taints: []Taint{
						{
							Effect: TaintEffect("NoSchedule"),
							Key:    "foo",
							Value:  "baz",
						},
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Taints: []Taint{
						{
							Effect: TaintEffect("NoSchedule"),
							Key:    "foo",
							Value:  "bar",
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
					NodeLabels: map[string]string{
						"foo":                                   "bar",
						"kubernetes.azure.com/scalesetpriority": "spot",
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					NodeLabels: map[string]string{
						"foo": "bar",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Can't update kubeletconfig",
			new: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					KubeletConfig: &KubeletConfig{
						CPUCfsQuota: to.BoolPtr(true),
					},
				},
			},
			old: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					KubeletConfig: &KubeletConfig{
						CPUCfsQuota: to.BoolPtr(false),
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
	// NOTE: AzureManagedMachinePool is behind AKS feature gate flag; the webhook
	// must prevent creating new objects in case the feature flag is disabled.
	defer utilfeature.SetFeatureGateDuringTest(t, feature.Gates, capifeature.MachinePool, true)()
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
			name: "ostype Windows with System mode not allowed",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:   "System",
					OSType: to.StringPtr(WindowsOS),
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "ostype windows with User mode",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:   "User",
					OSType: to.StringPtr(WindowsOS),
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
					Mode:   "User",
					OSType: to.StringPtr(WindowsOS),
				},
			},
			wantErr: false,
		},
		{
			name: "Windows clusters with more than 6char names are not allowed",
			ammp: &AzureManagedMachinePool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pool0-name-too-long",
				},
				Spec: AzureManagedMachinePoolSpec{
					Mode:   "User",
					OSType: to.StringPtr(WindowsOS),
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "valid label",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:   "User",
					OSType: to.StringPtr(LinuxOS),
					NodeLabels: map[string]string{
						"foo": "bar",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "kubernetes.azure.com label",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					Mode:   "User",
					OSType: to.StringPtr(LinuxOS),
					NodeLabels: map[string]string{
						"kubernetes.azure.com/scalesetpriority": "spot",
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
					EnableNodePublicIP:   to.BoolPtr(true),
					NodePublicIPPrefixID: to.StringPtr("not a valid resource ID"),
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "pool with public ip prefix cannot omit node public IP",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableNodePublicIP:   nil,
					NodePublicIPPrefixID: to.StringPtr("subscriptions/11111111-2222-aaaa-bbbb-cccccccccccc/resourceGroups/public-ip-test/providers/Microsoft.Network/publicipprefixes/public-ip-prefix"),
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "pool with public ip prefix cannot disable node public IP",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableNodePublicIP:   to.BoolPtr(false),
					NodePublicIPPrefixID: to.StringPtr("subscriptions/11111111-2222-aaaa-bbbb-cccccccccccc/resourceGroups/public-ip-test/providers/Microsoft.Network/publicipprefixes/public-ip-prefix"),
				},
			},
			wantErr:  true,
			errorLen: 1,
		},
		{
			name: "pool with public ip prefix with node public IP enabled ok",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableNodePublicIP:   to.BoolPtr(true),
					NodePublicIPPrefixID: to.StringPtr("subscriptions/11111111-2222-aaaa-bbbb-cccccccccccc/resourceGroups/public-ip-test/providers/Microsoft.Network/publicipprefixes/public-ip-prefix"),
				},
			},
			wantErr: false,
		},
		{
			name: "pool with public ip prefix with leading slash with node public IP enabled ok",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableNodePublicIP:   to.BoolPtr(true),
					NodePublicIPPrefixID: to.StringPtr("/subscriptions/11111111-2222-aaaa-bbbb-cccccccccccc/resourceGroups/public-ip-test/providers/Microsoft.Network/publicipprefixes/public-ip-prefix"),
				},
			},
			wantErr: false,
		},
		{
			name: "pool without public ip prefix with node public IP unset ok",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableNodePublicIP: nil,
				},
			},
			wantErr: false,
		},
		{
			name: "pool without public ip prefix with node public IP enabled ok",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableNodePublicIP: to.BoolPtr(true),
				},
			},
			wantErr: false,
		},
		{
			name: "pool without public ip prefix with node public IP disabled ok",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					EnableNodePublicIP: to.BoolPtr(false),
				},
			},
			wantErr: false,
		},
		{
			name: "KubeletConfig CPUCfsQuotaPeriod needs 'ms' suffix",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					KubeletConfig: &KubeletConfig{
						CPUCfsQuotaPeriod: to.StringPtr("100"),
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
					KubeletConfig: &KubeletConfig{
						CPUCfsQuotaPeriod: to.StringPtr("100ms"),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "KubeletConfig ImageGcLowThreshold can't be more than ImageGcHighThreshold",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
					KubeletConfig: &KubeletConfig{
						ImageGcLowThreshold:  to.Int32Ptr(100),
						ImageGcHighThreshold: to.Int32Ptr(99),
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
					KubeletConfig: &KubeletConfig{
						ImageGcLowThreshold:  to.Int32Ptr(99),
						ImageGcHighThreshold: to.Int32Ptr(100),
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid KubeletConfig AllowedUnsafeSysctls values",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
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
			wantErr: false,
		},
		{
			name: "more valid KubeletConfig AllowedUnsafeSysctls values",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
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
			wantErr: false,
		},
		{
			name: "an invalid KubeletConfig AllowedUnsafeSysctls value in a set",
			ammp: &AzureManagedMachinePool{
				Spec: AzureManagedMachinePoolSpec{
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
			wantErr:  true,
			errorLen: 1,
		},
	}
	var client client.Client
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
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

func TestAzureManagedMachinePool_ValidateCreateFailure(t *testing.T) {
	g := NewWithT(t)

	tests := []struct {
		name      string
		ammp      *AzureManagedMachinePool
		deferFunc func()
	}{
		{
			name:      "feature gate explicitly disabled",
			ammp:      getKnownValidAzureManagedMachinePool(),
			deferFunc: utilfeature.SetFeatureGateDuringTest(t, feature.Gates, capifeature.MachinePool, false),
		},
		{
			name:      "feature gate implicitly disabled",
			ammp:      getKnownValidAzureManagedMachinePool(),
			deferFunc: func() {},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			defer tc.deferFunc()
			err := tc.ammp.ValidateCreate(nil)
			g.Expect(err).To(HaveOccurred())
		})
	}
}

func TestAzureManagedMachinePool_validateLastSystemNodePool(t *testing.T) {
	deletionTime := metav1.Now()
	systemMachinePool := getManagedMachinePoolWithSystemMode()
	tests := []struct {
		name    string
		ammp    *AzureManagedMachinePool
		cluster *clusterv1.Cluster
		wantErr bool
	}{
		{
			name: "Test with paused cluster without deletion timestamp having one system pool node(valid delete:move operation)",
			ammp: systemMachinePool,
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:              systemMachinePool.GetLabels()[clusterv1.ClusterLabelName],
					Namespace:         systemMachinePool.Namespace,
					DeletionTimestamp: &deletionTime,
				},
				Spec: clusterv1.ClusterSpec{
					Paused: true,
				},
			},
			wantErr: false,
		},
		{
			name: "Test with paused cluster with deletion timestamp having one system pool node(valid delete)",
			ammp: systemMachinePool,
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:              systemMachinePool.GetLabels()[clusterv1.ClusterLabelName],
					Namespace:         systemMachinePool.Namespace,
					DeletionTimestamp: &deletionTime,
				},
				Spec: clusterv1.ClusterSpec{
					Paused: true,
				},
			},
			wantErr: false,
		},
		{
			name: "Test with running cluster without deletion timestamp having one system pool node(invalid delete)",
			ammp: systemMachinePool,
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      systemMachinePool.GetLabels()[clusterv1.ClusterLabelName],
					Namespace: systemMachinePool.Namespace,
				},
			},
			wantErr: true,
		},
		{
			name: "Test with running cluster with deletion timestamp having one system pool node(valid delete)",
			ammp: systemMachinePool,
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:              systemMachinePool.GetLabels()[clusterv1.ClusterLabelName],
					Namespace:         systemMachinePool.Namespace,
					DeletionTimestamp: &deletionTime,
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
			err := tc.ammp.validateLastSystemNodePool(fakeClient)
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
			MaxPods:    to.Int32Ptr(30),
			OsDiskType: to.StringPtr(string(containerservice.OSDiskTypeEphemeral)),
		},
	}
}

func getManagedMachinePoolWithSystemMode() *AzureManagedMachinePool {
	return &AzureManagedMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				clusterv1.ClusterLabelName: "test-cluster",
				LabelAgentPoolMode:         string(NodePoolModeSystem),
			},
		},
	}
}

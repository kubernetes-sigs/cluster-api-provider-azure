/*
Copyright 2025 The Kubernetes Authors.

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

package controllers

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
)

func TestExtractCapacityFromSKU(t *testing.T) {
	tests := []struct {
		name           string
		sku            resourceskus.SKU
		expectedCPU    string
		expectedMemory string
		expectError    bool
	}{
		{
			name: "Standard_D2s_v3 - 2 CPU, 8GB memory",
			sku: resourceskus.SKU(armcompute.ResourceSKU{
				Capabilities: []*armcompute.ResourceSKUCapabilities{
					{Name: ptr.To("vCPUs"), Value: ptr.To("2")},
					{Name: ptr.To("MemoryGB"), Value: ptr.To("8")},
				},
			}),
			expectedCPU:    "2",
			expectedMemory: "8Gi",
			expectError:    false,
		},
		{
			name: "fractional memory - 3.5 GiB",
			sku: resourceskus.SKU(armcompute.ResourceSKU{
				Capabilities: []*armcompute.ResourceSKUCapabilities{
					{Name: ptr.To("vCPUs"), Value: ptr.To("1")},
					{Name: ptr.To("MemoryGB"), Value: ptr.To("3.5")},
				},
			}),
			expectedCPU:    "1",
			expectedMemory: "3584Mi",
			expectError:    false,
		},
		{
			name: "missing vCPUs capability",
			sku: resourceskus.SKU(armcompute.ResourceSKU{
				Capabilities: []*armcompute.ResourceSKUCapabilities{
					{Name: ptr.To("MemoryGB"), Value: ptr.To("8")},
				},
			}),
			expectError: true,
		},
		{
			name: "missing MemoryGB capability",
			sku: resourceskus.SKU(armcompute.ResourceSKU{
				Capabilities: []*armcompute.ResourceSKUCapabilities{
					{Name: ptr.To("vCPUs"), Value: ptr.To("2")},
				},
			}),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			capacity, err := extractCapacityFromSKU(tt.sku)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(capacity).ToNot(BeNil())

			// Check CPU
			if tt.expectedCPU != "" {
				cpu := capacity[corev1.ResourceCPU]
				expectedCPU := resource.MustParse(tt.expectedCPU)
				g.Expect(cpu.Equal(expectedCPU)).To(BeTrue(),
					"CPU: expected %s, got %s", expectedCPU.String(), cpu.String())
			}

			// Check Memory
			if tt.expectedMemory != "" {
				memory := capacity[corev1.ResourceMemory]
				expectedMemory := resource.MustParse(tt.expectedMemory)
				g.Expect(memory.Equal(expectedMemory)).To(BeTrue(),
					"Memory: expected %s, got %s", expectedMemory.String(), memory.String())
			}
		})
	}
}

func TestAzureMachineTemplateReconcile(t *testing.T) {
	g := NewWithT(t)
	scheme, err := newScheme()
	g.Expect(err).NotTo(HaveOccurred())

	defaultCluster := getFakeCluster()
	defaultAzureCluster := getFakeAzureCluster()
	defaultAzureMachineTemplate := getFakeAzureMachineTemplate()

	cases := map[string]struct {
		objects []runtime.Object
		fail    bool
		err     string
		event   string
	}{
		"should not fail if template is not found": {
			objects: []runtime.Object{
				defaultCluster,
				defaultAzureCluster,
			},
		},
		"should return if template has not yet set ownerref": {
			objects: []runtime.Object{
				defaultCluster,
				defaultAzureCluster,
				getFakeAzureMachineTemplate(func(amt *infrav1.AzureMachineTemplate) {
					amt.OwnerReferences = nil
				}),
			},
		},
		"should fail if cluster does not exist": {
			objects: []runtime.Object{
				defaultAzureCluster,
				defaultAzureMachineTemplate,
			},
			fail: true,
		},
		"should return if cluster is paused": {
			objects: []runtime.Object{
				func() *clusterv1.Cluster {
					c := getFakeCluster()
					c.Spec.Paused = ptr.To(true)
					return c
				}(),
				defaultAzureCluster,
				defaultAzureMachineTemplate,
			},
		},
		"should return if AzureMachineTemplate is paused": {
			objects: []runtime.Object{
				defaultCluster,
				defaultAzureCluster,
				getFakeAzureMachineTemplate(func(amt *infrav1.AzureMachineTemplate) {
					amt.Annotations = map[string]string{
						clusterv1.PausedAnnotation: "true",
					}
				}),
			},
		},
		"should return if infraRef is not defined": {
			objects: []runtime.Object{
				func() *clusterv1.Cluster {
					c := getFakeCluster()
					c.Spec.InfrastructureRef = clusterv1.ContractVersionedObjectReference{}
					return c
				}(),
				defaultAzureCluster,
				defaultAzureMachineTemplate,
			},
		},
		"should return if infraRef is not AzureCluster": {
			objects: []runtime.Object{
				func() *clusterv1.Cluster {
					c := getFakeCluster()
					c.Spec.InfrastructureRef.Kind = "DockerCluster"
					return c
				}(),
				defaultAzureCluster,
				defaultAzureMachineTemplate,
			},
		},
		"should fail if AzureCluster not found": {
			objects: []runtime.Object{
				func() *clusterv1.Cluster {
					c := getFakeCluster()
					c.Spec.InfrastructureRef.Name = "non-existent-cluster"
					return c
				}(),
				defaultAzureMachineTemplate,
			},
			fail: true,
			err:  "azureclusters.infrastructure.cluster.x-k8s.io \"non-existent-cluster\" not found",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithRuntimeObjects(tc.objects...).
				WithStatusSubresource(&infrav1.AzureMachineTemplate{}).
				Build()

			r := &AzureMachineTemplateReconciler{
				Client:           fakeClient,
				Recorder:         record.NewFakeRecorder(128),
				CredentialCache:  azure.NewCredentialCache(),
				Timeouts:         reconciler.Timeouts{},
				WatchFilterValue: "",
			}

			_, err := r.Reconcile(t.Context(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "default",
					Name:      "my-template",
				},
			})

			if tc.event != "" {
				g.Expect(r.Recorder.(*record.FakeRecorder).Events).To(Receive(ContainSubstring(tc.event)))
			}
			if tc.fail {
				g.Expect(err).To(HaveOccurred())
				if tc.err != "" {
					g.Expect(err.Error()).To(ContainSubstring(tc.err))
				}
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

// TestGetVMSizeCapacity tests VM size capacity retrieval.
func TestGetVMSizeCapacity(t *testing.T) {
	tests := []struct {
		name      string
		sku       armcompute.ResourceSKU
		expectCPU string
		expectMem string
		expectErr bool
	}{
		{"valid SKU", buildTestSKU("Standard_D2s_v3", "2", "8", ""), "2", "8Gi", false},
		{"fractional memory", buildTestSKU("Standard_E4s_v3", "4", "31.5", ""), "4", "32256Mi", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			r := &AzureMachineTemplateReconciler{}
			cache := resourceskus.NewStaticCache([]armcompute.ResourceSKU{tt.sku}, "westus2")

			capacity, err := r.getVMSizeCapacity(t.Context(), cache, *tt.sku.Name)
			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(capacity[corev1.ResourceCPU]).To(Equal(resource.MustParse(tt.expectCPU)))
			g.Expect(capacity[corev1.ResourceMemory]).To(Equal(resource.MustParse(tt.expectMem)))
		})
	}
}

// TestGetVMSizeNodeInfo tests node info retrieval.
func TestGetVMSizeNodeInfo(t *testing.T) {
	tests := []struct {
		name       string
		sku        armcompute.ResourceSKU
		osType     string
		expectArch infrav1.Architecture
		expectOS   infrav1.OperatingSystem
		expectErr  bool
	}{
		{
			"x64 Linux",
			buildTestSKU("Standard_D2s_v3", "2", "8", string(armcompute.ArchitectureX64)),
			azure.LinuxOS,
			infrav1.ArchitectureAmd64,
			infrav1.OperatingSystemLinux,
			false,
		},
		{
			"Arm64 Windows",
			buildTestSKU("Standard_D2ps_v5", "2", "8", string(armcompute.ArchitectureArm64)),
			azure.WindowsOS,
			infrav1.ArchitectureArm64,
			infrav1.OperatingSystemWindows,
			false,
		},
		{
			"missing arch",
			buildTestSKU("Standard_Invalid", "2", "8", ""),
			azure.LinuxOS,
			"",
			"",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			r := &AzureMachineTemplateReconciler{}
			cache := resourceskus.NewStaticCache([]armcompute.ResourceSKU{tt.sku}, "westus2")

			nodeInfo, err := r.getVMSizeNodeInfo(t.Context(), cache, *tt.sku.Name, tt.osType)
			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(nodeInfo.Architecture).To(Equal(tt.expectArch))
			g.Expect(nodeInfo.OperatingSystem).To(Equal(tt.expectOS))
		})
	}
}

// getFakeAzureMachineTemplate creates a test AzureMachineTemplate with optional changes.
func getFakeAzureMachineTemplate(changes ...func(*infrav1.AzureMachineTemplate)) *infrav1.AzureMachineTemplate {
	input := &infrav1.AzureMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-template",
			Namespace: "default",
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "my-cluster",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
					Name:       "my-cluster",
				},
			},
		},
		Spec: infrav1.AzureMachineTemplateSpec{
			Template: infrav1.AzureMachineTemplateResource{
				Spec: infrav1.AzureMachineSpec{
					VMSize: "Standard_D2s_v3",
					OSDisk: infrav1.OSDisk{
						OSType: azure.LinuxOS,
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

// buildTestSKU creates a test SKU with specified capabilities.
func buildTestSKU(name, cpu, memory, arch string) armcompute.ResourceSKU {
	caps := []*armcompute.ResourceSKUCapabilities{
		{Name: ptr.To("vCPUs"), Value: ptr.To(cpu)},
		{Name: ptr.To("MemoryGB"), Value: ptr.To(memory)},
	}
	if arch != "" {
		caps = append(caps, &armcompute.ResourceSKUCapabilities{
			Name:  ptr.To("CpuArchitectureType"),
			Value: ptr.To(arch),
		})
	}
	return armcompute.ResourceSKU{Name: ptr.To(name), Capabilities: caps}
}

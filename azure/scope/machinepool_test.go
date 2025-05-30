/*
Copyright 2020 The Kubernetes Authors.

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

package scope

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/roleassignments"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/scalesets"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
)

func TestMachinePoolScope_Name(t *testing.T) {
	tests := []struct {
		name             string
		machinePoolScope MachinePoolScope
		want             string
		testLength       bool
	}{
		{
			name: "linux can be any length",
			machinePoolScope: MachinePoolScope{
				MachinePool: nil,
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "some-really-really-long-name",
					},
				},
				ClusterScoper: nil,
			},
			want: "some-really-really-long-name",
		},
		{
			name: "windows longer than 9 should be shortened",
			machinePoolScope: MachinePoolScope{
				MachinePool: nil,
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machine-90123456",
					},
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							OSDisk: infrav1.OSDisk{
								OSType: "Windows",
							},
						},
					},
				},
				ClusterScoper: nil,
			},
			want: "win-23456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.machinePoolScope.Name()
			if got != tt.want {
				t.Errorf("MachinePoolScope.Name() = %v, want %v", got, tt.want)
			}

			if tt.testLength && len(got) > 9 {
				t.Errorf("Length of MachinePoolScope.Name() = %v, want less than %v", len(got), 9)
			}
		})
	}
}

func TestMachinePoolScope_ProviderID(t *testing.T) {
	tests := []struct {
		name             string
		machinePoolScope MachinePoolScope
		want             string
	}{
		{
			name: "valid providerID",
			machinePoolScope: MachinePoolScope{
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					Spec: infrav1exp.AzureMachinePoolSpec{
						ProviderID: "azure:///subscriptions/1234/resourcegroups/my-rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/cloud-provider-user-identity",
					},
				},
			},
			want: "cloud-provider-user-identity",
		},
		{
			name: "valid providerID: VMSS Flex instance",
			machinePoolScope: MachinePoolScope{
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					Spec: infrav1exp.AzureMachinePoolSpec{
						ProviderID: "azure:///subscriptions/1234/resourceGroups/my-cluster/providers/Microsoft.Compute/virtualMachines/machine-0",
					},
				},
			},
			want: "machine-0",
		},
		{
			name: "valid providerID: VMSS Uniform instance",
			machinePoolScope: MachinePoolScope{
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					Spec: infrav1exp.AzureMachinePoolSpec{
						ProviderID: "azure:///subscriptions/1234/resourceGroups/my-cluster/providers/Microsoft.Compute/virtualMachineScaleSets/my-cluster-mp-0/virtualMachines/0",
					},
				},
			},
			want: "0",
		},
		{
			name: "invalid providerID: no cloud provider",
			machinePoolScope: MachinePoolScope{
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					Spec: infrav1exp.AzureMachinePoolSpec{
						ProviderID: "subscriptions/123/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm",
					},
				},
			},
			want: "",
		},
		{
			name: "invalid providerID: incomplete URL",
			machinePoolScope: MachinePoolScope{
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					Spec: infrav1exp.AzureMachinePoolSpec{
						ProviderID: "azure:///",
					},
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.machinePoolScope.ProviderID()
			if got != tt.want {
				t.Errorf("MachinePoolScope.ProviderID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMachinePoolScope_NetworkInterfaces(t *testing.T) {
	tests := []struct {
		name             string
		machinePoolScope MachinePoolScope
		want             int
	}{
		{
			name: "zero network interfaces",
			machinePoolScope: MachinePoolScope{
				MachinePool: nil,
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "default-nics",
					},
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							AcceleratedNetworking: ptr.To(true),
							SubnetName:            "node-subnet",
						},
					},
				},
				ClusterScoper: nil,
			},
			want: 0,
		},
		{
			name: "one network interface",
			machinePoolScope: MachinePoolScope{
				MachinePool: nil,
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "single-nic",
					},
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							NetworkInterfaces: []infrav1.NetworkInterface{
								{
									SubnetName: "node-subnet",
								},
							},
						},
					},
				},
				ClusterScoper: nil,
			},
			want: 1,
		},
		{
			name: "two network interfaces",
			machinePoolScope: MachinePoolScope{
				MachinePool: nil,
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "dual-nics",
					},
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							NetworkInterfaces: []infrav1.NetworkInterface{
								{
									SubnetName: "control-plane-subnet",
								},
								{
									SubnetName: "node-subnet",
								},
							},
						},
					},
				},
				ClusterScoper: nil,
			},
			want: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := len(tt.machinePoolScope.AzureMachinePool.Spec.Template.NetworkInterfaces)
			if got != tt.want {
				t.Errorf("MachinePoolScope.Name() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMachinePoolScope_MaxSurge(t *testing.T) {
	cases := []struct {
		Name   string
		Setup  func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool)
		Verify func(g *WithT, surge int, err error)
	}{
		{
			Name: "default surge should be 1 if no deployment strategy is set",
			Verify: func(g *WithT, surge int, err error) {
				g.Expect(surge).To(Equal(1))
				g.Expect(err).NotTo(HaveOccurred())
			},
		},
		{
			Name: "default surge should be 1 regardless of replica count with no surger",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool) {
				mp.Spec.Replicas = ptr.To[int32](3)
			},
			Verify: func(g *WithT, surge int, err error) {
				g.Expect(surge).To(Equal(1))
				g.Expect(err).NotTo(HaveOccurred())
			},
		},
		{
			Name: "default surge should be 2 as specified by the surger",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool) {
				mp.Spec.Replicas = ptr.To[int32](3)
				two := intstr.FromInt(2)
				amp.Spec.Strategy = infrav1exp.AzureMachinePoolDeploymentStrategy{
					Type: infrav1exp.RollingUpdateAzureMachinePoolDeploymentStrategyType,
					RollingUpdate: &infrav1exp.MachineRollingUpdateDeployment{
						MaxSurge: &two,
					},
				}
			},
			Verify: func(g *WithT, surge int, err error) {
				g.Expect(surge).To(Equal(2))
				g.Expect(err).NotTo(HaveOccurred())
			},
		},
		{
			Name: "default surge should be 2 (50%) of the desired replicas",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool) {
				mp.Spec.Replicas = ptr.To[int32](4)
				fiftyPercent := intstr.FromString("50%")
				amp.Spec.Strategy = infrav1exp.AzureMachinePoolDeploymentStrategy{
					Type: infrav1exp.RollingUpdateAzureMachinePoolDeploymentStrategyType,
					RollingUpdate: &infrav1exp.MachineRollingUpdateDeployment{
						MaxSurge: &fiftyPercent,
					},
				}
			},
			Verify: func(g *WithT, surge int, err error) {
				g.Expect(surge).To(Equal(2))
				g.Expect(err).NotTo(HaveOccurred())
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			var (
				g        = NewWithT(t)
				mockCtrl = gomock.NewController(t)
				amp      = &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "amp1",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "mp1",
								Kind:       "MachinePool",
								APIVersion: expv1.GroupVersion.String(),
							},
						},
					},
				}
				mp = &expv1.MachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mp1",
						Namespace: "default",
					},
				}
			)
			defer mockCtrl.Finish()

			if c.Setup != nil {
				c.Setup(mp, amp)
			}

			s := &MachinePoolScope{
				MachinePool:      mp,
				AzureMachinePool: amp,
			}
			surge, err := s.MaxSurge()
			c.Verify(g, surge, err)
		})
	}
}

func TestMachinePoolScope_SaveVMImageToStatus(t *testing.T) {
	var (
		g        = NewWithT(t)
		mockCtrl = gomock.NewController(t)
		amp      = &infrav1exp.AzureMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "amp1",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					{
						Name:       "mp1",
						Kind:       "MachinePool",
						APIVersion: expv1.GroupVersion.String(),
					},
				},
			},
		}
		s = &MachinePoolScope{
			AzureMachinePool: amp,
		}
		image = &infrav1.Image{
			Marketplace: &infrav1.AzureMarketplaceImage{
				ImagePlan: infrav1.ImagePlan{
					Publisher: "cncf-upstream",
					Offer:     "capi",
					SKU:       "k8s-1dot19dot11-ubuntu-1804",
				},
				Version:         "latest",
				ThirdPartyImage: false,
			},
		}
	)
	defer mockCtrl.Finish()

	s.SaveVMImageToStatus(image)
	g.Expect(s.AzureMachinePool.Status.Image).To(Equal(image))
}

func TestMachinePoolScope_GetVMImage(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	clusterMock := mock_azure.NewMockClusterScoper(mockCtrl)
	clusterMock.EXPECT().Location().AnyTimes()
	clusterMock.EXPECT().SubscriptionID().AnyTimes()
	clusterMock.EXPECT().CloudEnvironment().AnyTimes()
	clusterMock.EXPECT().Token().Return(&azidentity.DefaultAzureCredential{}).AnyTimes()
	cases := []struct {
		Name   string
		Setup  func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool)
		Verify func(g *WithT, amp *infrav1exp.AzureMachinePool, vmImage *infrav1.Image, err error)
	}{
		{
			Name: "should set and default the image if no image is specified for the AzureMachinePool",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool) {
				mp.Spec.Template.Spec.Version = ptr.To("v1.19.11")
			},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, vmImage *infrav1.Image, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				image := &infrav1.Image{
					ComputeGallery: &infrav1.AzureComputeGalleryImage{
						Gallery: "ClusterAPI-f72ceb4f-5159-4c26-a0fe-2ea738f0d019",
						Name:    "capi-ubun2-2404",
						Version: "1.19.11",
					},
				}
				g.Expect(vmImage).To(Equal(image))
				g.Expect(amp.Spec.Template.Image).To(BeNil())
			},
		},
		{
			Name: "should not default or set the image on the AzureMachinePool if it already exists",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool) {
				mp.Spec.Template.Spec.Version = ptr.To("v1.19.11")
				amp.Spec.Template.Image = &infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						ImagePlan: infrav1.ImagePlan{
							Publisher: "cncf-upstream",
							Offer:     "capi",
							SKU:       "k8s-1dot19dot19-ubuntu-1804",
						},
						Version:         "latest",
						ThirdPartyImage: false,
					},
				}
			},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, vmImage *infrav1.Image, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				image := &infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						ImagePlan: infrav1.ImagePlan{
							Publisher: "cncf-upstream",
							Offer:     "capi",
							SKU:       "k8s-1dot19dot19-ubuntu-1804",
						},
						Version:         "latest",
						ThirdPartyImage: false,
					},
				}
				g.Expect(vmImage).To(Equal(image))
				g.Expect(amp.Spec.Template.Image).To(Equal(image))
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			var (
				g        = NewWithT(t)
				mockCtrl = gomock.NewController(t)
				amp      = &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "amp1",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "mp1",
								Kind:       "MachinePool",
								APIVersion: expv1.GroupVersion.String(),
							},
						},
					},
				}
				mp = &expv1.MachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mp1",
						Namespace: "default",
					},
				}
			)
			defer mockCtrl.Finish()

			if c.Setup != nil {
				c.Setup(mp, amp)
			}

			s := &MachinePoolScope{
				MachinePool:      mp,
				AzureMachinePool: amp,
				ClusterScoper:    clusterMock,
			}
			image, err := s.GetVMImage(t.Context())
			c.Verify(g, amp, image, err)
		})
	}
}

func TestMachinePoolScope_NeedsRequeue(t *testing.T) {
	cases := []struct {
		Name   string
		Setup  func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, vmss *azure.VMSS)
		Verify func(g *WithT, requeue bool)
	}{
		{
			Name: "should requeue if the machine is not in succeeded state",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, vmss *azure.VMSS) {
				creating := infrav1.Creating
				mp.Spec.Replicas = ptr.To[int32](0)
				amp.Status.ProvisioningState = &creating
			},
			Verify: func(g *WithT, requeue bool) {
				g.Expect(requeue).To(BeTrue())
			},
		},
		{
			Name: "should not requeue if the machine is in succeeded state",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, vmss *azure.VMSS) {
				succeeded := infrav1.Succeeded
				mp.Spec.Replicas = ptr.To[int32](0)
				amp.Status.ProvisioningState = &succeeded
			},
			Verify: func(g *WithT, requeue bool) {
				g.Expect(requeue).To(BeFalse())
			},
		},
		{
			Name: "should requeue if the machine is in succeeded state but desired replica count does not match",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, vmss *azure.VMSS) {
				succeeded := infrav1.Succeeded
				mp.Spec.Replicas = ptr.To[int32](1)
				amp.Status.ProvisioningState = &succeeded
			},
			Verify: func(g *WithT, requeue bool) {
				g.Expect(requeue).To(BeTrue())
			},
		},
		{
			Name: "should not requeue if the machine is in succeeded state but desired replica count does match",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, vmss *azure.VMSS) {
				succeeded := infrav1.Succeeded
				mp.Spec.Replicas = ptr.To[int32](1)
				amp.Status.ProvisioningState = &succeeded
				vmss.Instances = []azure.VMSSVM{
					{
						Name: "instance1",
					},
				}
			},
			Verify: func(g *WithT, requeue bool) {
				g.Expect(requeue).To(BeFalse())
			},
		},
		{
			Name: "should requeue if an instance VM image does not match the VM image of the VMSS",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, vmss *azure.VMSS) {
				succeeded := infrav1.Succeeded
				mp.Spec.Replicas = ptr.To[int32](1)
				amp.Status.ProvisioningState = &succeeded
				vmss.Instances = []azure.VMSSVM{
					{
						Name: "instance1",
						Image: infrav1.Image{
							Marketplace: &infrav1.AzureMarketplaceImage{
								Version: "foo1",
							},
						},
					},
				}
			},
			Verify: func(g *WithT, requeue bool) {
				g.Expect(requeue).To(BeTrue())
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			var (
				g        = NewWithT(t)
				mockCtrl = gomock.NewController(t)
				amp      = &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "amp1",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "mp1",
								Kind:       "MachinePool",
								APIVersion: expv1.GroupVersion.String(),
							},
						},
					},
				}
				mp = &expv1.MachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mp1",
						Namespace: "default",
					},
				}
				vmssState = &azure.VMSS{}
			)
			defer mockCtrl.Finish()

			if c.Setup != nil {
				c.Setup(mp, amp, vmssState)
			}

			s := &MachinePoolScope{
				vmssState:        vmssState,
				MachinePool:      mp,
				AzureMachinePool: amp,
			}
			c.Verify(g, s.NeedsRequeue())
		})
	}
}

func TestMachinePoolScope_updateReplicasAndProviderIDs(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1exp.AddToScheme(scheme)
	_ = expv1.AddToScheme(scheme)

	cases := []struct {
		Name   string
		Setup  func(cb *fake.ClientBuilder)
		Verify func(g *WithT, amp *infrav1exp.AzureMachinePool, err error)
	}{
		{
			Name: "if there are three ready machines with matching labels, then should count them",
			Setup: func(cb *fake.ClientBuilder) {
				for _, machine := range getReadyAzureMachinePoolMachines(3) {
					obj := machine
					cb.WithObjects(&obj)
				}
			},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(amp.Status.Replicas).To(BeEquivalentTo(3))
				g.Expect(amp.Spec.ProviderIDList).To(ConsistOf("azure://foo/ampm0", "azure://foo/ampm1", "azure://foo/ampm2"))
			},
		},
		{
			Name: "should only count machines with matching machine pool label",
			Setup: func(cb *fake.ClientBuilder) {
				machines := getReadyAzureMachinePoolMachines(3)
				machines[0].Labels[infrav1exp.MachinePoolNameLabel] = "not_correct"
				for _, machine := range machines {
					obj := machine
					cb.WithObjects(&obj)
				}
			},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(amp.Status.Replicas).To(BeEquivalentTo(2))
			},
		},
		{
			Name: "should only count machines with matching cluster name label",
			Setup: func(cb *fake.ClientBuilder) {
				machines := getReadyAzureMachinePoolMachines(3)
				machines[0].Labels[clusterv1.ClusterNameLabel] = "not_correct"
				for _, machine := range machines {
					obj := machine
					cb.WithObjects(&obj)
				}
			},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(amp.Status.Replicas).To(BeEquivalentTo(2))
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			var (
				g        = NewWithT(t)
				mockCtrl = gomock.NewController(t)
				cb       = fake.NewClientBuilder().WithScheme(scheme)
				cluster  = &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: clusterv1.ClusterSpec{
						InfrastructureRef: &corev1.ObjectReference{
							Name: "azCluster1",
						},
					},
					Status: clusterv1.ClusterStatus{
						InfrastructureReady: true,
					},
				}
				mp = &expv1.MachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mp1",
						Namespace: "default",
					},
				}
				amp = &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "amp1",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "mp1",
								Kind:       "MachinePool",
								APIVersion: expv1.GroupVersion.String(),
							},
						},
					},
				}
			)
			defer mockCtrl.Finish()

			c.Setup(cb.WithObjects(amp, cluster))
			s := &MachinePoolScope{
				client: cb.Build(),
				ClusterScoper: &ClusterScope{
					Cluster: cluster,
				},
				AzureMachinePool: amp,
				MachinePool:      mp,
			}
			err := s.updateReplicasAndProviderIDs(t.Context())
			c.Verify(g, s.AzureMachinePool, err)
		})
	}
}

func TestMachinePoolScope_RoleAssignmentSpecs(t *testing.T) {
	tests := []struct {
		name             string
		machinePoolScope MachinePoolScope
		want             []azure.ResourceSpecGetter
	}{
		{
			name: "returns empty if VM identity is not system assigned",
			machinePoolScope: MachinePoolScope{
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machine-name",
					},
				},
			},
			want: []azure.ResourceSpecGetter{},
		},
		{
			name: "returns role assignment spec if VM identity is system assigned",
			machinePoolScope: MachinePoolScope{
				MachinePool: &expv1.MachinePool{},
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machine-name",
					},
					Spec: infrav1exp.AzureMachinePoolSpec{
						Identity: infrav1.VMIdentitySystemAssigned,
						SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
							Name: "role-assignment-name",
						},
					},
				},
				ClusterScoper: &ClusterScope{
					AzureClients: AzureClients{
						EnvironmentSettings: auth.EnvironmentSettings{
							Values: map[string]string{
								auth.SubscriptionID: "123",
							},
						},
					},
					AzureCluster: &infrav1.AzureCluster{
						Spec: infrav1.AzureClusterSpec{
							ResourceGroup: "my-rg",
							AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
								Location: "westus",
							},
						},
					},
				},
			},
			want: []azure.ResourceSpecGetter{
				&roleassignments.RoleAssignmentSpec{
					ResourceType:  azure.VirtualMachineScaleSet,
					MachineName:   "machine-name",
					Name:          "role-assignment-name",
					ResourceGroup: "my-rg",
					PrincipalID:   ptr.To("fakePrincipalID"),
					PrincipalType: armauthorization.PrincipalTypeServicePrincipal,
				},
			},
		},
		{
			name: "returns role assignment spec if scope and role definition ID are set",
			machinePoolScope: MachinePoolScope{
				MachinePool: &expv1.MachinePool{},
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machine-name",
					},
					Spec: infrav1exp.AzureMachinePoolSpec{
						Identity: infrav1.VMIdentitySystemAssigned,
						SystemAssignedIdentityRole: &infrav1.SystemAssignedIdentityRole{
							Name:         "role-assignment-name",
							Scope:        "scope",
							DefinitionID: "role-definition-id",
						},
					},
				},
				ClusterScoper: &ClusterScope{
					AzureClients: AzureClients{
						EnvironmentSettings: auth.EnvironmentSettings{
							Values: map[string]string{
								auth.SubscriptionID: "123",
							},
						},
					},
					AzureCluster: &infrav1.AzureCluster{
						Spec: infrav1.AzureClusterSpec{
							ResourceGroup: "my-rg",
							AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
								Location: "westus",
							},
						},
					},
				},
			},
			want: []azure.ResourceSpecGetter{
				&roleassignments.RoleAssignmentSpec{
					ResourceType:     azure.VirtualMachineScaleSet,
					MachineName:      "machine-name",
					Name:             "role-assignment-name",
					ResourceGroup:    "my-rg",
					Scope:            "scope",
					RoleDefinitionID: "role-definition-id",
					PrincipalID:      ptr.To("fakePrincipalID"),
					PrincipalType:    armauthorization.PrincipalTypeServicePrincipal,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.machinePoolScope.RoleAssignmentSpecs(ptr.To("fakePrincipalID")); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RoleAssignmentSpecs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMachinePoolScope_VMSSExtensionSpecs(t *testing.T) {
	tests := []struct {
		name             string
		machinePoolScope MachinePoolScope
		want             []azure.ResourceSpecGetter
	}{
		{
			name: "If OS type is Linux and cloud is AzurePublicCloud, it returns ExtensionSpec",
			machinePoolScope: MachinePoolScope{
				MachinePool: &expv1.MachinePool{},
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machinepool-name",
					},
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							OSDisk: infrav1.OSDisk{
								OSType: "Linux",
							},
						},
					},
				},
				ClusterScoper: &ClusterScope{
					AzureClients: AzureClients{
						EnvironmentSettings: auth.EnvironmentSettings{
							Environment: azureautorest.Environment{
								Name: azureautorest.PublicCloud.Name,
							},
						},
					},
					AzureCluster: &infrav1.AzureCluster{
						Spec: infrav1.AzureClusterSpec{
							ResourceGroup: "my-rg",
						},
					},
				},
				cache: &MachinePoolCache{
					VMSKU: resourceskus.SKU{},
				},
			},
			want: []azure.ResourceSpecGetter{
				&scalesets.VMSSExtensionSpec{
					ExtensionSpec: azure.ExtensionSpec{
						Name:      "CAPZ.Linux.Bootstrapping",
						VMName:    "machinepool-name",
						Publisher: "Microsoft.Azure.ContainerUpstream",
						Version:   "1.0",
						ProtectedSettings: map[string]string{
							"commandToExecute": azure.LinuxBootstrapExtensionCommand,
						},
					},
					ResourceGroup: "my-rg",
				},
			},
		},
		{
			name: "If OS type is Linux and cloud is not AzurePublicCloud, it returns empty",
			machinePoolScope: MachinePoolScope{
				MachinePool: &expv1.MachinePool{},
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machinepool-name",
					},
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							OSDisk: infrav1.OSDisk{
								OSType: "Linux",
							},
						},
					},
				},
				ClusterScoper: &ClusterScope{
					AzureClients: AzureClients{
						EnvironmentSettings: auth.EnvironmentSettings{
							Environment: azureautorest.Environment{
								Name: azureautorest.USGovernmentCloud.Name,
							},
						},
					},
					AzureCluster: &infrav1.AzureCluster{
						Spec: infrav1.AzureClusterSpec{
							ResourceGroup: "my-rg",
						},
					},
				},
				cache: &MachinePoolCache{
					VMSKU: resourceskus.SKU{},
				},
			},
			want: []azure.ResourceSpecGetter{},
		},
		{
			name: "If OS type is Windows and cloud is AzurePublicCloud, it returns ExtensionSpec",
			machinePoolScope: MachinePoolScope{
				MachinePool: &expv1.MachinePool{},
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						// Note: machine pool names longer than 9 characters get truncated. See MachinePoolScope::Name() for more details.
						Name: "winpool",
					},
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							OSDisk: infrav1.OSDisk{
								OSType: "Windows",
							},
						},
					},
				},
				ClusterScoper: &ClusterScope{
					AzureClients: AzureClients{
						EnvironmentSettings: auth.EnvironmentSettings{
							Environment: azureautorest.Environment{
								Name: azureautorest.PublicCloud.Name,
							},
						},
					},
					AzureCluster: &infrav1.AzureCluster{
						Spec: infrav1.AzureClusterSpec{
							ResourceGroup: "my-rg",
						},
					},
				},
				cache: &MachinePoolCache{
					VMSKU: resourceskus.SKU{},
				},
			},
			want: []azure.ResourceSpecGetter{
				&scalesets.VMSSExtensionSpec{
					ExtensionSpec: azure.ExtensionSpec{
						Name: "CAPZ.Windows.Bootstrapping",
						// Note: machine pool names longer than 9 characters get truncated. See MachinePoolScope::Name() for more details.
						VMName:    "winpool",
						Publisher: "Microsoft.Azure.ContainerUpstream",
						Version:   "1.0",
						ProtectedSettings: map[string]string{
							"commandToExecute": azure.WindowsBootstrapExtensionCommand,
						},
					},
					ResourceGroup: "my-rg",
				},
			},
		},
		{
			name: "If OS type is Windows and cloud is not AzurePublicCloud, it returns empty",
			machinePoolScope: MachinePoolScope{
				MachinePool: &expv1.MachinePool{},
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machinepool-name",
					},
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							OSDisk: infrav1.OSDisk{
								OSType: "Windows",
							},
						},
					},
				},
				ClusterScoper: &ClusterScope{
					AzureClients: AzureClients{
						EnvironmentSettings: auth.EnvironmentSettings{
							Environment: azureautorest.Environment{
								Name: azureautorest.USGovernmentCloud.Name,
							},
						},
					},
					AzureCluster: &infrav1.AzureCluster{
						Spec: infrav1.AzureClusterSpec{
							ResourceGroup: "my-rg",
						},
					},
				},
				cache: &MachinePoolCache{
					VMSKU: resourceskus.SKU{},
				},
			},
			want: []azure.ResourceSpecGetter{},
		},
		{
			name: "If OS type is not Linux or Windows and cloud is AzurePublicCloud, it returns empty",
			machinePoolScope: MachinePoolScope{
				MachinePool: &expv1.MachinePool{},
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machinepool-name",
					},
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							OSDisk: infrav1.OSDisk{
								OSType: "Other",
							},
						},
					},
				},
				ClusterScoper: &ClusterScope{
					AzureClients: AzureClients{
						EnvironmentSettings: auth.EnvironmentSettings{
							Environment: azureautorest.Environment{
								Name: azureautorest.PublicCloud.Name,
							},
						},
					},
					AzureCluster: &infrav1.AzureCluster{
						Spec: infrav1.AzureClusterSpec{
							ResourceGroup: "my-rg",
						},
					},
				},
				cache: &MachinePoolCache{
					VMSKU: resourceskus.SKU{},
				},
			},
			want: []azure.ResourceSpecGetter{},
		},
		{
			name: "If OS type is not Windows or Linux and cloud is not AzurePublicCloud, it returns empty",
			machinePoolScope: MachinePoolScope{
				MachinePool: &expv1.MachinePool{},
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machinepool-name",
					},
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							OSDisk: infrav1.OSDisk{
								OSType: "Other",
							},
						},
					},
				},
				ClusterScoper: &ClusterScope{
					AzureClients: AzureClients{
						EnvironmentSettings: auth.EnvironmentSettings{
							Environment: azureautorest.Environment{
								Name: azureautorest.USGovernmentCloud.Name,
							},
						},
					},
					AzureCluster: &infrav1.AzureCluster{
						Spec: infrav1.AzureClusterSpec{
							ResourceGroup: "my-rg",
						},
					},
				},
				cache: &MachinePoolCache{
					VMSKU: resourceskus.SKU{},
				},
			},
			want: []azure.ResourceSpecGetter{},
		},
		{
			name: "If a custom VM extension is specified, it returns the custom VM extension",
			machinePoolScope: MachinePoolScope{
				MachinePool: &expv1.MachinePool{},
				AzureMachinePool: &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name: "machinepool-name",
					},
					Spec: infrav1exp.AzureMachinePoolSpec{
						Template: infrav1exp.AzureMachinePoolMachineTemplate{
							OSDisk: infrav1.OSDisk{
								OSType: "Linux",
							},
							VMExtensions: []infrav1.VMExtension{
								{
									Name:      "custom-vm-extension",
									Publisher: "Microsoft.Azure.Extensions",
									Version:   "2.0",
									Settings: map[string]string{
										"timestamp": "1234567890",
									},
									ProtectedSettings: map[string]string{
										"commandToExecute": "echo hello world",
									},
								},
							},
						},
					},
				},
				ClusterScoper: &ClusterScope{
					AzureClients: AzureClients{
						EnvironmentSettings: auth.EnvironmentSettings{
							Environment: azureautorest.Environment{
								Name: azureautorest.PublicCloud.Name,
							},
						},
					},
					AzureCluster: &infrav1.AzureCluster{
						Spec: infrav1.AzureClusterSpec{
							ResourceGroup: "my-rg",
							AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
								Location: "westus",
							},
						},
					},
				},
				cache: &MachinePoolCache{
					VMSKU: resourceskus.SKU{},
				},
			},
			want: []azure.ResourceSpecGetter{
				&scalesets.VMSSExtensionSpec{
					ExtensionSpec: azure.ExtensionSpec{
						Name:      "custom-vm-extension",
						VMName:    "machinepool-name",
						Publisher: "Microsoft.Azure.Extensions",
						Version:   "2.0",
						Settings: map[string]string{
							"timestamp": "1234567890",
						},
						ProtectedSettings: map[string]string{
							"commandToExecute": "echo hello world",
						},
					},
					ResourceGroup: "my-rg",
				},
				&scalesets.VMSSExtensionSpec{
					ExtensionSpec: azure.ExtensionSpec{
						Name:      "CAPZ.Linux.Bootstrapping",
						VMName:    "machinepool-name",
						Publisher: "Microsoft.Azure.ContainerUpstream",
						Version:   "1.0",
						ProtectedSettings: map[string]string{
							"commandToExecute": azure.LinuxBootstrapExtensionCommand,
						},
					},
					ResourceGroup: "my-rg",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.machinePoolScope.VMSSExtensionSpecs(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("VMSSExtensionSpecs() = \n%s, want \n%s", specArrayToString(got), specArrayToString(tt.want))
			}
		})
	}
}

func getReadyAzureMachinePoolMachines(count int32) []infrav1exp.AzureMachinePoolMachine {
	machines := make([]infrav1exp.AzureMachinePoolMachine, count)
	succeeded := infrav1.Succeeded
	for i := 0; i < int(count); i++ {
		machines[i] = infrav1exp.AzureMachinePoolMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("ampm%d", i),
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					{
						Name:       "amp",
						Kind:       infrav1.AzureMachinePoolKind,
						APIVersion: infrav1exp.GroupVersion.String(),
					},
				},
				Labels: map[string]string{
					clusterv1.ClusterNameLabel:      "cluster1",
					infrav1exp.MachinePoolNameLabel: "amp1",
					clusterv1.MachinePoolNameLabel:  "mp1",
					"cluster1":                      string(infrav1.ResourceLifecycleOwned),
				},
			},
			Spec: infrav1exp.AzureMachinePoolMachineSpec{
				ProviderID: fmt.Sprintf("azure://foo/ampm%d", i),
			},
			Status: infrav1exp.AzureMachinePoolMachineStatus{
				Ready:             true,
				ProvisioningState: &succeeded,
			},
		}
	}

	return machines
}

func getAzureMachinePoolMachine(index int) infrav1exp.AzureMachinePoolMachine {
	return infrav1exp.AzureMachinePoolMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("ampm%d", index),
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       "amp",
					Kind:       "AzureMachinePool",
					APIVersion: infrav1exp.GroupVersion.String(),
				},
			},
			Labels: map[string]string{
				clusterv1.ClusterNameLabel:      "cluster1",
				infrav1exp.MachinePoolNameLabel: "amp1",
				clusterv1.MachinePoolNameLabel:  "mp1",
				"cluster1":                      string(infrav1.ResourceLifecycleOwned),
			},
		},
		Spec: infrav1exp.AzureMachinePoolMachineSpec{
			ProviderID: fmt.Sprintf("azure:///subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachineScaleSets/my-vmss/virtualMachines/%d", index),
		},
		Status: infrav1exp.AzureMachinePoolMachineStatus{
			Ready:             true,
			ProvisioningState: ptr.To(infrav1.Succeeded),
		},
	}
}

func getAzureMachinePoolMachineWithOwnerMachine(index int) (clusterv1.Machine, infrav1exp.AzureMachinePoolMachine) {
	ampm := getAzureMachinePoolMachine(index)
	machine := clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("mpm%d", index),
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       "mp",
					Kind:       "MachinePool",
					APIVersion: expv1.GroupVersion.String(),
				},
			},
			Labels: map[string]string{
				clusterv1.ClusterNameLabel:     "cluster1",
				clusterv1.MachinePoolNameLabel: "mp1",
			},
		},
		Spec: clusterv1.MachineSpec{
			ProviderID: &ampm.Spec.ProviderID,
			InfrastructureRef: corev1.ObjectReference{
				Kind:      "AzureMachinePoolMachine",
				Name:      ampm.Name,
				Namespace: ampm.Namespace,
			},
		},
	}

	ampm.OwnerReferences = append(ampm.OwnerReferences, metav1.OwnerReference{
		Name:       machine.Name,
		Kind:       "Machine",
		APIVersion: clusterv1.GroupVersion.String(),
	})

	return machine, ampm
}

func TestMachinePoolScope_SetInfrastructureMachineKind(t *testing.T) {
	testcases := []struct {
		name             string
		azureMachinePool infrav1exp.AzureMachinePool
		updated          bool
	}{
		{
			name: "should set infrastructure machine kind",
			azureMachinePool: infrav1exp.AzureMachinePool{
				Status: infrav1exp.AzureMachinePoolStatus{},
			},
			updated: true,
		},
		{
			name: "already set infrastructure machine kind",
			azureMachinePool: infrav1exp.AzureMachinePool{
				Status: infrav1exp.AzureMachinePoolStatus{
					InfrastructureMachineKind: infrav1exp.AzureMachinePoolMachineKind,
				},
			},
			updated: false,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			machinePoolScope := &MachinePoolScope{
				AzureMachinePool: &tt.azureMachinePool,
			}

			got := machinePoolScope.SetInfrastructureMachineKind()
			g.Expect(machinePoolScope.AzureMachinePool.Status.InfrastructureMachineKind).To(Equal(infrav1exp.AzureMachinePoolMachineKind))
			g.Expect(got).To(Equal(tt.updated))
		})
	}
}

func TestMachinePoolScope_applyAzureMachinePoolMachines(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1exp.AddToScheme(scheme)

	tests := []struct {
		Name   string
		Setup  func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, vmssState *azure.VMSS, cb *fake.ClientBuilder)
		Verify func(g *WithT, amp *infrav1exp.AzureMachinePool, c client.Client, err error)
	}{
		{
			Name: "if MachinePool is externally managed and overProvisionCount > 0, do not try to reduce replicas",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, vmssState *azure.VMSS, cb *fake.ClientBuilder) {
				mp.Annotations = map[string]string{clusterv1.ReplicasManagedByAnnotation: "cluster-autoscaler"}
				mp.Spec.Replicas = ptr.To[int32](1)

				mpm1, ampm1 := getAzureMachinePoolMachineWithOwnerMachine(1)
				mpm2, ampm2 := getAzureMachinePoolMachineWithOwnerMachine(2)
				objects := []client.Object{}
				objects = append(objects, &mpm1, &ampm1, &mpm2, &ampm2)
				cb.WithObjects(objects...)
				vmssState.Instances = []azure.VMSSVM{
					{
						ID:   "/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachineScaleSets/my-vmss/virtualMachines/1",
						Name: "ampm1",
					},
					{
						ID:   "/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachineScaleSets/my-vmss/virtualMachines/2",
						Name: "ampm2",
					},
				}
			},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, c client.Client, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				list := clusterv1.MachineList{}
				g.Expect(c.List(ctx, &list)).NotTo(HaveOccurred())
				g.Expect(list.Items).Should(HaveLen(2))
			},
		},
		{
			Name: "if MachinePool is not externally managed and overProvisionCount > 0, reduce replicas",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, vmssState *azure.VMSS, cb *fake.ClientBuilder) {
				mp.Spec.Replicas = ptr.To[int32](1)

				mpm1, ampm1 := getAzureMachinePoolMachineWithOwnerMachine(1)
				mpm2, ampm2 := getAzureMachinePoolMachineWithOwnerMachine(2)
				objects := []client.Object{}
				objects = append(objects, &mpm1, &ampm1, &mpm2, &ampm2)
				cb.WithObjects(objects...)

				vmssState.Instances = []azure.VMSSVM{
					{
						ID:   "/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachineScaleSets/my-vmss/virtualMachines/1",
						Name: "ampm1",
					},
					{
						ID:   "/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachineScaleSets/my-vmss/virtualMachines/2",
						Name: "ampm2",
					},
				}
			},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, c client.Client, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				list := clusterv1.MachineList{}
				g.Expect(c.List(ctx, &list)).NotTo(HaveOccurred())
				g.Expect(list.Items).Should(HaveLen(1))
			},
		},
		{
			Name: "if MachinePool is not externally managed, and Machines have delete machine annotation, and overProvisionCount > 0, delete machines with deleteMachine annotation first",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, vmssState *azure.VMSS, cb *fake.ClientBuilder) {
				mp.Spec.Replicas = ptr.To[int32](2)

				mpm1, ampm1 := getAzureMachinePoolMachineWithOwnerMachine(1)

				mpm2, ampm2 := getAzureMachinePoolMachineWithOwnerMachine(2)
				mpm2.Annotations = map[string]string{
					clusterv1.DeleteMachineAnnotation: time.Now().String(),
				}

				mpm3, ampm3 := getAzureMachinePoolMachineWithOwnerMachine(3)
				objects := []client.Object{&mpm1, &ampm1, &mpm2, &ampm2, &mpm3, &ampm3}
				cb.WithObjects(objects...)

				vmssState.Instances = []azure.VMSSVM{
					{
						ID:   "/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachineScaleSets/my-vmss/virtualMachines/1",
						Name: "ampm1",
					},
					{
						ID:   "/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachineScaleSets/my-vmss/virtualMachines/2",
						Name: "ampm2",
					},
					{
						ID:   "/subscriptions/123/resourceGroups/my-rg/providers/Microsoft.Compute/virtualMachineScaleSets/my-vmss/virtualMachines/3",
						Name: "ampm3",
					},
				}
			},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, c client.Client, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				list := clusterv1.MachineList{}
				g.Expect(c.List(ctx, &list)).NotTo(HaveOccurred())
				g.Expect(list.Items).Should(HaveLen(2))
				g.Expect(list.Items[0].Name).Should(Equal("mpm1"))
				g.Expect(list.Items[1].Name).Should(Equal("mpm3"))
			},
		},
		{
			Name: "if existing MachinePool is not present, reduce replicas",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, vmssState *azure.VMSS, cb *fake.ClientBuilder) {
				mp.Spec.Replicas = ptr.To[int32](1)

				vmssState.Instances = []azure.VMSSVM{
					{
						ID:   "/subscriptions/123/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm",
						Name: "vm",
					},
				}
			},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, c client.Client, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				list := infrav1exp.AzureMachinePoolMachineList{}
				g.Expect(c.List(ctx, &list)).NotTo(HaveOccurred())
				g.Expect(list.Items).Should(HaveLen(1))
			},
		},
		{
			Name: "if existing MachinePool is not present and Instances ID is in wrong format, reduce replicas",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, vmssState *azure.VMSS, cb *fake.ClientBuilder) {
				mp.Spec.Replicas = ptr.To[int32](1)

				vmssState.Instances = []azure.VMSSVM{
					{
						ID:   "foo/ampm0",
						Name: "ampm0",
					},
				}
			},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, c client.Client, err error) {
				g.Expect(err).To(HaveOccurred())
			},
		},
		{
			Name: "if existing MachinePool is present but in deleting state, do not recreate AzureMachinePoolMachines",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, vmssState *azure.VMSS, cb *fake.ClientBuilder) {
				mp.Spec.Replicas = ptr.To[int32](1)

				vmssState.Instances = []azure.VMSSVM{
					{
						ID:    "/subscriptions/123/resourceGroups/rg/providers/Microsoft.Compute/virtualMachines/vm",
						Name:  "vm",
						State: infrav1.Deleting,
					},
				}
			},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, c client.Client, err error) {
				g.Expect(err).NotTo(HaveOccurred())
				list := infrav1exp.AzureMachinePoolMachineList{}
				g.Expect(c.List(ctx, &list)).NotTo(HaveOccurred())
				g.Expect(list.Items).Should(BeEmpty())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			var (
				g        = NewWithT(t)
				mockCtrl = gomock.NewController(t)
				cb       = fake.NewClientBuilder().WithScheme(scheme)
				cluster  = &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: clusterv1.ClusterSpec{
						InfrastructureRef: &corev1.ObjectReference{
							Name: "azCluster1",
						},
					},
					Status: clusterv1.ClusterStatus{
						InfrastructureReady: true,
					},
				}
				mp = &expv1.MachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mp1",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "cluster1",
								Kind:       "Cluster",
								APIVersion: clusterv1.GroupVersion.String(),
							},
						},
					},
				}
				amp = &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "amp1",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "mp1",
								Kind:       "MachinePool",
								APIVersion: expv1.GroupVersion.String(),
							},
						},
					},
				}
				vmssState = &azure.VMSS{}
			)
			defer mockCtrl.Finish()

			tt.Setup(mp, amp, vmssState, cb.WithObjects(amp, cluster))
			s := &MachinePoolScope{
				client: cb.Build(),
				ClusterScoper: &ClusterScope{
					Cluster: cluster,
				},
				MachinePool:      mp,
				AzureMachinePool: amp,
				vmssState:        vmssState,
			}
			err := s.applyAzureMachinePoolMachines(ctx)
			tt.Verify(g, s.AzureMachinePool, s.client, err)
		})
	}
}

func TestMachinePoolScope_setProvisioningStateAndConditions(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1exp.AddToScheme(scheme)

	tests := []struct {
		Name              string
		Setup             func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, cb *fake.ClientBuilder)
		Verify            func(g *WithT, amp *infrav1exp.AzureMachinePool, c client.Client)
		ProvisioningState infrav1.ProvisioningState
	}{
		{
			Name: "if provisioning state is set to Succeeded and replicas match, MachinePool is ready and conditions match",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, cb *fake.ClientBuilder) {
				mp.Spec.Replicas = ptr.To[int32](1)
				amp.Status.Replicas = 1
			},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, c client.Client) {
				g.Expect(amp.Status.Ready).To(BeTrue())
				g.Expect(conditions.Get(amp, infrav1.ScaleSetRunningCondition).Status).To(Equal(corev1.ConditionTrue))
				g.Expect(conditions.Get(amp, infrav1.ScaleSetModelUpdatedCondition).Status).To(Equal(corev1.ConditionTrue))
				g.Expect(conditions.Get(amp, infrav1.ScaleSetDesiredReplicasCondition).Status).To(Equal(corev1.ConditionTrue))
			},
			ProvisioningState: infrav1.Succeeded,
		},
		{
			Name: "if provisioning state is set to Succeeded and replicas are higher on AzureMachinePool, MachinePool is ready and ScalingDown",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, cb *fake.ClientBuilder) {
				mp.Spec.Replicas = ptr.To[int32](1)
				amp.Status.Replicas = 2
			},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, c client.Client) {
				g.Expect(amp.Status.Ready).To(BeTrue())
				condition := conditions.Get(amp, infrav1.ScaleSetDesiredReplicasCondition)
				g.Expect(condition.Status).To(Equal(corev1.ConditionFalse))
				g.Expect(condition.Reason).To(Equal(infrav1.ScaleSetScaleDownReason))
			},
			ProvisioningState: infrav1.Succeeded,
		},
		{
			Name: "if provisioning state is set to Succeeded and replicas are lower on AzureMachinePool, MachinePool is ready and ScalingUp",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, cb *fake.ClientBuilder) {
				mp.Spec.Replicas = ptr.To[int32](2)
				amp.Status.Replicas = 1
			},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, c client.Client) {
				g.Expect(amp.Status.Ready).To(BeTrue())
				condition := conditions.Get(amp, infrav1.ScaleSetDesiredReplicasCondition)
				g.Expect(condition.Status).To(Equal(corev1.ConditionFalse))
				g.Expect(condition.Reason).To(Equal(infrav1.ScaleSetScaleUpReason))
			},
			ProvisioningState: infrav1.Succeeded,
		},
		{
			Name:  "if provisioning state is set to Updating, MachinePool is ready and scale set model is set to OutOfDate",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, cb *fake.ClientBuilder) {},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, c client.Client) {
				g.Expect(amp.Status.Ready).To(BeTrue())
				condition := conditions.Get(amp, infrav1.ScaleSetModelUpdatedCondition)
				g.Expect(condition.Status).To(Equal(corev1.ConditionFalse))
				g.Expect(condition.Reason).To(Equal(infrav1.ScaleSetModelOutOfDateReason))
			},
			ProvisioningState: infrav1.Updating,
		},
		{
			Name:  "if provisioning state is set to Creating, MachinePool is NotReady and scale set running condition is set to Creating",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, cb *fake.ClientBuilder) {},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, c client.Client) {
				g.Expect(amp.Status.Ready).To(BeFalse())
				condition := conditions.Get(amp, infrav1.ScaleSetRunningCondition)
				g.Expect(condition.Status).To(Equal(corev1.ConditionFalse))
				g.Expect(condition.Reason).To(Equal(infrav1.ScaleSetCreatingReason))
			},
			ProvisioningState: infrav1.Creating,
		},
		{
			Name:  "if provisioning state is set to Deleting, MachinePool is NotReady and scale set running condition is set to Deleting",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, cb *fake.ClientBuilder) {},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, c client.Client) {
				g.Expect(amp.Status.Ready).To(BeFalse())
				condition := conditions.Get(amp, infrav1.ScaleSetRunningCondition)
				g.Expect(condition.Status).To(Equal(corev1.ConditionFalse))
				g.Expect(condition.Reason).To(Equal(infrav1.ScaleSetDeletingReason))
			},
			ProvisioningState: infrav1.Deleting,
		},
		{
			Name:  "if provisioning state is set to Failed, MachinePool ready state is not adjusted, and scale set running condition is set to Failed",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, cb *fake.ClientBuilder) {},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, c client.Client) {
				condition := conditions.Get(amp, infrav1.ScaleSetRunningCondition)
				g.Expect(condition.Status).To(Equal(corev1.ConditionFalse))
				g.Expect(condition.Reason).To(Equal(infrav1.ScaleSetProvisionFailedReason))
			},
			ProvisioningState: infrav1.Failed,
		},
		{
			Name:  "if provisioning state is set to something not explicitly handled, MachinePool ready state is not adjusted, and scale set running condition is set to the ProvisioningState",
			Setup: func(mp *expv1.MachinePool, amp *infrav1exp.AzureMachinePool, cb *fake.ClientBuilder) {},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, c client.Client) {
				condition := conditions.Get(amp, infrav1.ScaleSetRunningCondition)
				g.Expect(condition.Status).To(Equal(corev1.ConditionFalse))
				g.Expect(condition.Reason).To(Equal(string(infrav1.Migrating)))
			},
			ProvisioningState: infrav1.Migrating,
		},
	}
	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			var (
				g        = NewWithT(t)
				mockCtrl = gomock.NewController(t)
				cb       = fake.NewClientBuilder().WithScheme(scheme)
				cluster  = &clusterv1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "default",
					},
					Spec: clusterv1.ClusterSpec{
						InfrastructureRef: &corev1.ObjectReference{
							Name: "azCluster1",
						},
					},
					Status: clusterv1.ClusterStatus{
						InfrastructureReady: true,
					},
				}
				mp = &expv1.MachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mp1",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "cluster1",
								Kind:       "Cluster",
								APIVersion: clusterv1.GroupVersion.String(),
							},
						},
					},
				}
				amp = &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "amp1",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "mp1",
								Kind:       "MachinePool",
								APIVersion: expv1.GroupVersion.String(),
							},
						},
					},
				}
				vmssState = &azure.VMSS{}
			)
			defer mockCtrl.Finish()

			tt.Setup(mp, amp, cb.WithObjects(amp, cluster))
			s := &MachinePoolScope{
				client: cb.Build(),
				ClusterScoper: &ClusterScope{
					Cluster: cluster,
				},
				MachinePool:      mp,
				AzureMachinePool: amp,
				vmssState:        vmssState,
			}
			s.setProvisioningStateAndConditions(tt.ProvisioningState)
			tt.Verify(g, s.AzureMachinePool, s.client)
		})
	}
}

func TestBootstrapDataChanges(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = infrav1exp.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	var (
		g        = NewWithT(t)
		mockCtrl = gomock.NewController(t)
		cb       = fake.NewClientBuilder().WithScheme(scheme)
		cluster  = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster1",
				Namespace: "default",
			},
			Spec: clusterv1.ClusterSpec{
				InfrastructureRef: &corev1.ObjectReference{
					Name: "azCluster1",
				},
			},
			Status: clusterv1.ClusterStatus{
				InfrastructureReady: true,
			},
		}
		azureCluster = &infrav1.AzureCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "azCluster1",
				Namespace: "default",
			},
			Spec: infrav1.AzureClusterSpec{
				AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
					Location: "test",
				},
			},
		}
		mp = &expv1.MachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mp1",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					{
						Name:       "cluster1",
						Kind:       "Cluster",
						APIVersion: clusterv1.GroupVersion.String(),
					},
				},
			},
			Spec: expv1.MachinePoolSpec{
				Template: clusterv1.MachineTemplateSpec{
					Spec: clusterv1.MachineSpec{
						Bootstrap: clusterv1.Bootstrap{
							DataSecretName: ptr.To("mp-secret"),
						},
						Version: ptr.To("v1.31.0"),
					},
				},
			},
		}
		bootstrapData     = "test"
		bootstrapDataHash = sha256Hash(base64.StdEncoding.EncodeToString([]byte(bootstrapData)))
		bootstrapSecret   = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mp-secret",
				Namespace: "default",
			},
			Data: map[string][]byte{"value": []byte(bootstrapData)},
		}
		amp = &infrav1exp.AzureMachinePool{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "amp1",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					{
						Name:       "mp1",
						Kind:       "MachinePool",
						APIVersion: expv1.GroupVersion.String(),
					},
				},
				Annotations: map[string]string{
					azure.CustomDataHashAnnotation: fmt.Sprintf("%x", bootstrapDataHash),
				},
			},
			Spec: infrav1exp.AzureMachinePoolSpec{
				Template: infrav1exp.AzureMachinePoolMachineTemplate{
					Image: &infrav1.Image{},
					NetworkInterfaces: []infrav1.NetworkInterface{
						{
							SubnetName: "test",
						},
					},
					VMSize: "VM_SIZE",
				},
			},
		}
		vmssState = &azure.VMSS{}
	)
	defer mockCtrl.Finish()

	s := &MachinePoolScope{
		client: cb.
			WithObjects(&bootstrapSecret).
			Build(),
		ClusterScoper: &ClusterScope{
			Cluster:      cluster,
			AzureCluster: azureCluster,
		},
		skuCache: resourceskus.NewStaticCache([]armcompute.ResourceSKU{
			{
				Name: ptr.To("VM_SIZE"),
			},
		}, "test"),
		MachinePool:      mp,
		AzureMachinePool: amp,
		vmssState:        vmssState,
	}

	g.Expect(s.InitMachinePoolCache(ctx)).NotTo(HaveOccurred())

	spec := s.ScaleSetSpec(ctx)
	sSpec := spec.(*scalesets.ScaleSetSpec)
	g.Expect(sSpec.ShouldPatchCustomData).To(BeFalse())

	amp.Annotations[azure.CustomDataHashAnnotation] = "old"

	// reset cache to be able to build up the cache again
	s.cache = nil
	g.Expect(s.InitMachinePoolCache(ctx)).NotTo(HaveOccurred())

	spec = s.ScaleSetSpec(ctx)
	sSpec = spec.(*scalesets.ScaleSetSpec)
	g.Expect(sSpec.ShouldPatchCustomData).To(BeTrue())
}

func sha256Hash(text string) []byte {
	h := sha256.New()
	_, err := io.WriteString(h, text)
	if err != nil {
		panic(err)
	}
	return h.Sum(nil)
}

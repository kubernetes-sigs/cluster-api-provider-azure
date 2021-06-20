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
	"fmt"
	"testing"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/klog/v2/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

func TestMachinePoolScope_SetBootstrapConditions(t *testing.T) {
	cases := []struct {
		Name   string
		Setup  func() (provisioningState string, extensionName string)
		Verify func(g *WithT, amp *infrav1exp.AzureMachinePool, err error)
	}{
		{
			Name: "should set bootstrap succeeded condition if provisioning state succeeded",
			Setup: func() (provisioningState string, extensionName string) {
				return string(infrav1.Succeeded), "foo"
			},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, err error) {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(conditions.IsTrue(amp, infrav1.BootstrapSucceededCondition))
			},
		},
		{
			Name: "should set bootstrap succeeded false condition with reason if provisioning state creating",
			Setup: func() (provisioningState string, extensionName string) {
				return string(infrav1.Creating), "bazz"
			},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, err error) {
				g.Expect(err).To(MatchError("transient reconcile error occurred: extension is still in provisioning state. This likely means that bootstrapping has not yet completed on the VM. Object will be requeued after 30s"))
				g.Expect(conditions.IsFalse(amp, infrav1.BootstrapSucceededCondition))
				g.Expect(conditions.GetReason(amp, infrav1.BootstrapSucceededCondition)).To(Equal(infrav1.BootstrapInProgressReason))
				severity := conditions.GetSeverity(amp, infrav1.BootstrapSucceededCondition)
				g.Expect(severity).ToNot(BeNil())
				g.Expect(*severity).To(Equal(clusterv1.ConditionSeverityInfo))
			},
		},
		{
			Name: "should set bootstrap succeeded false condition with reason if provisioning state failed",
			Setup: func() (provisioningState string, extensionName string) {
				return string(infrav1.Failed), "buzz"
			},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, err error) {
				g.Expect(err).To(MatchError("reconcile error that cannot be recovered occurred: extension state failed. This likely means the Kubernetes node bootstrapping process failed or timed out. Check VM boot diagnostics logs to learn more. Object will not be requeued"))
				g.Expect(conditions.IsFalse(amp, infrav1.BootstrapSucceededCondition))
				g.Expect(conditions.GetReason(amp, infrav1.BootstrapSucceededCondition)).To(Equal(infrav1.BootstrapFailedReason))
				severity := conditions.GetSeverity(amp, infrav1.BootstrapSucceededCondition)
				g.Expect(severity).ToNot(BeNil())
				g.Expect(*severity).To(Equal(clusterv1.ConditionSeverityError))
			},
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			var (
				g        = NewWithT(t)
				mockCtrl = gomock.NewController(t)
			)
			defer mockCtrl.Finish()

			state, name := c.Setup()
			s := &MachinePoolScope{
				AzureMachinePool: &infrav1exp.AzureMachinePool{},
				Logger:           klogr.New(),
			}
			err := s.SetBootstrapConditions(state, name)
			c.Verify(g, s.AzureMachinePool, err)
		})
	}
}

func TestMachinePoolScope_MaxSurge(t *testing.T) {
	cases := []struct {
		Name   string
		Setup  func(mp *clusterv1exp.MachinePool, amp *infrav1exp.AzureMachinePool)
		Verify func(g *WithT, surge int, err error)
	}{
		{
			Name: "default surge should be 1 if no deployment strategy is set",
			Verify: func(g *WithT, surge int, err error) {
				g.Expect(surge).To(Equal(1))
				g.Expect(err).ToNot(HaveOccurred())
			},
		},
		{
			Name: "default surge should be 1 regardless of replica count with no surger",
			Setup: func(mp *clusterv1exp.MachinePool, amp *infrav1exp.AzureMachinePool) {
				mp.Spec.Replicas = to.Int32Ptr(3)
			},
			Verify: func(g *WithT, surge int, err error) {
				g.Expect(surge).To(Equal(1))
				g.Expect(err).ToNot(HaveOccurred())
			},
		},
		{
			Name: "default surge should be 2 as specified by the surger",
			Setup: func(mp *clusterv1exp.MachinePool, amp *infrav1exp.AzureMachinePool) {
				mp.Spec.Replicas = to.Int32Ptr(3)
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
				g.Expect(err).ToNot(HaveOccurred())
			},
		},
		{
			Name: "default surge should be 2 (50%) of the desired replicas",
			Setup: func(mp *clusterv1exp.MachinePool, amp *infrav1exp.AzureMachinePool) {
				mp.Spec.Replicas = to.Int32Ptr(4)
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
				g.Expect(err).ToNot(HaveOccurred())
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
								APIVersion: clusterv1exp.GroupVersion.String(),
							},
						},
					},
				}
				mp = &clusterv1exp.MachinePool{
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
				Logger:           klogr.New(),
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
						APIVersion: clusterv1exp.GroupVersion.String(),
					},
				},
			},
		}
		s = &MachinePoolScope{
			AzureMachinePool: amp,
			Logger:           klogr.New(),
		}
		image = &infrav1.Image{
			Marketplace: &infrav1.AzureMarketplaceImage{
				Publisher:       "cncf-upstream",
				Offer:           "capi",
				SKU:             "k8s-1dot19dot11-ubuntu-1804",
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
	cases := []struct {
		Name   string
		Setup  func(mp *clusterv1exp.MachinePool, amp *infrav1exp.AzureMachinePool)
		Verify func(g *WithT, amp *infrav1exp.AzureMachinePool, vmImage *infrav1.Image, err error)
	}{
		{
			Name: "should set and default the image if no image is specified for the AzureMachinePool",
			Setup: func(mp *clusterv1exp.MachinePool, amp *infrav1exp.AzureMachinePool) {
				mp.Spec.Template.Spec.Version = to.StringPtr("v1.19.11")
			},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, vmImage *infrav1.Image, err error) {
				g.Expect(err).ToNot(HaveOccurred())
				image := &infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher:       "cncf-upstream",
						Offer:           "capi",
						SKU:             "k8s-1dot19dot11-ubuntu-1804",
						Version:         "latest",
						ThirdPartyImage: false,
					},
				}
				g.Expect(vmImage).To(Equal(image))
				g.Expect(amp.Spec.Template.Image).To(BeNil())
			},
		},
		{
			Name: "should not default or set the image on the AzureMachinePool if it already exists",
			Setup: func(mp *clusterv1exp.MachinePool, amp *infrav1exp.AzureMachinePool) {
				mp.Spec.Template.Spec.Version = to.StringPtr("v1.19.11")
				amp.Spec.Template.Image = &infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher:       "cncf-upstream",
						Offer:           "capi",
						SKU:             "k8s-1dot19dot19-ubuntu-1804",
						Version:         "latest",
						ThirdPartyImage: false,
					},
				}
			},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, vmImage *infrav1.Image, err error) {
				g.Expect(err).ToNot(HaveOccurred())
				image := &infrav1.Image{
					Marketplace: &infrav1.AzureMarketplaceImage{
						Publisher:       "cncf-upstream",
						Offer:           "capi",
						SKU:             "k8s-1dot19dot19-ubuntu-1804",
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
								APIVersion: clusterv1exp.GroupVersion.String(),
							},
						},
					},
				}
				mp = &clusterv1exp.MachinePool{
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
				Logger:           klogr.New(),
			}
			image, err := s.GetVMImage()
			c.Verify(g, amp, image, err)
		})
	}
}

func TestMachinePoolScope_NeedsRequeue(t *testing.T) {
	cases := []struct {
		Name   string
		Setup  func(mp *clusterv1exp.MachinePool, amp *infrav1exp.AzureMachinePool, vmss *azure.VMSS)
		Verify func(g *WithT, requeue bool)
	}{
		{
			Name: "should requeue if the machine is not in succeeded state",
			Setup: func(mp *clusterv1exp.MachinePool, amp *infrav1exp.AzureMachinePool, vmss *azure.VMSS) {
				creating := infrav1.Creating
				mp.Spec.Replicas = to.Int32Ptr(0)
				amp.Status.ProvisioningState = &creating
			},
			Verify: func(g *WithT, requeue bool) {
				g.Expect(requeue).To(BeTrue())
			},
		},
		{
			Name: "should not requeue if the machine is in succeeded state",
			Setup: func(mp *clusterv1exp.MachinePool, amp *infrav1exp.AzureMachinePool, vmss *azure.VMSS) {
				succeeded := infrav1.Succeeded
				mp.Spec.Replicas = to.Int32Ptr(0)
				amp.Status.ProvisioningState = &succeeded
			},
			Verify: func(g *WithT, requeue bool) {
				g.Expect(requeue).To(BeFalse())
			},
		},
		{
			Name: "should requeue if the machine is in succeeded state but desired replica count does not match",
			Setup: func(mp *clusterv1exp.MachinePool, amp *infrav1exp.AzureMachinePool, vmss *azure.VMSS) {
				succeeded := infrav1.Succeeded
				mp.Spec.Replicas = to.Int32Ptr(1)
				amp.Status.ProvisioningState = &succeeded
			},
			Verify: func(g *WithT, requeue bool) {
				g.Expect(requeue).To(BeTrue())
			},
		},
		{
			Name: "should not requeue if the machine is in succeeded state but desired replica count does match",
			Setup: func(mp *clusterv1exp.MachinePool, amp *infrav1exp.AzureMachinePool, vmss *azure.VMSS) {
				succeeded := infrav1.Succeeded
				mp.Spec.Replicas = to.Int32Ptr(1)
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
			Setup: func(mp *clusterv1exp.MachinePool, amp *infrav1exp.AzureMachinePool, vmss *azure.VMSS) {
				succeeded := infrav1.Succeeded
				mp.Spec.Replicas = to.Int32Ptr(1)
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
								APIVersion: clusterv1exp.GroupVersion.String(),
							},
						},
					},
				}
				mp = &clusterv1exp.MachinePool{
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
				Logger:           klogr.New(),
			}
			c.Verify(g, s.NeedsRequeue())
		})
	}
}

func TestMachinePoolScope_updateReplicasAndProviderIDs(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1exp.AddToScheme(scheme)

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
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(amp.Status.Replicas).To(BeEquivalentTo(3))
				g.Expect(amp.Spec.ProviderIDList).To(ConsistOf("/foo/ampm0", "/foo/ampm1", "/foo/ampm2"))
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
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(amp.Status.Replicas).To(BeEquivalentTo(2))
			},
		},
		{
			Name: "should only count machines with matching cluster name label",
			Setup: func(cb *fake.ClientBuilder) {
				machines := getReadyAzureMachinePoolMachines(3)
				machines[0].Labels[clusterv1.ClusterLabelName] = "not_correct"
				for _, machine := range machines {
					obj := machine
					cb.WithObjects(&obj)
				}
			},
			Verify: func(g *WithT, amp *infrav1exp.AzureMachinePool, err error) {
				g.Expect(err).ToNot(HaveOccurred())
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
				amp = &infrav1exp.AzureMachinePool{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "amp1",
						Namespace: "default",
						OwnerReferences: []metav1.OwnerReference{
							{
								Name:       "mp1",
								Kind:       "MachinePool",
								APIVersion: clusterv1exp.GroupVersion.String(),
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
				Logger:           klogr.New(),
			}
			err := s.updateReplicasAndProviderIDs(context.TODO())
			c.Verify(g, s.AzureMachinePool, err)
		})
	}
}

func getReadyAzureMachinePoolMachines(count int32) []infrav1exp.AzureMachinePoolMachine {
	machines := make([]infrav1exp.AzureMachinePoolMachine, count)
	for i := 0; i < int(count); i++ {
		machines[i] = infrav1exp.AzureMachinePoolMachine{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("ampm%d", i),
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					{
						Name:       "amp",
						Kind:       "AzureMachinePool",
						APIVersion: infrav1exp.GroupVersion.String(),
					},
				},
				Labels: map[string]string{
					clusterv1.ClusterLabelName:      "cluster1",
					infrav1exp.MachinePoolNameLabel: "amp1",
				},
			},
			Spec: infrav1exp.AzureMachinePoolMachineSpec{
				ProviderID: fmt.Sprintf("/foo/ampm%d", i),
			},
			Status: infrav1exp.AzureMachinePoolMachineStatus{
				Ready: true,
			},
		}
	}

	return machines
}

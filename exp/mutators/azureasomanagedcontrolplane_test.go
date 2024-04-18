/*
Copyright 2024 The Kubernetes Authors.

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

package mutators

import (
	"context"
	"encoding/json"
	"testing"

	asocontainerservicev1preview "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20230202preview"
	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231001"
	"github.com/google/go-cmp/cmp"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

func TestSetManagedClusterDefaults(t *testing.T) {
	ctx := context.Background()
	g := NewGomegaWithT(t)

	tests := []struct {
		name                   string
		asoManagedControlPlane *infrav1exp.AzureASOManagedControlPlane
		cluster                *clusterv1.Cluster
		expected               []*unstructured.Unstructured
		expectedErr            error
	}{
		{
			name: "no ManagedCluster",
			asoManagedControlPlane: &infrav1exp.AzureASOManagedControlPlane{
				Spec: infrav1exp.AzureASOManagedControlPlaneSpec{
					AzureASOManagedControlPlaneTemplateResourceSpec: infrav1exp.AzureASOManagedControlPlaneTemplateResourceSpec{
						Resources: []runtime.RawExtension{},
					},
				},
			},
			expectedErr: ErrNoManagedClusterDefined,
		},
		{
			name: "success",
			asoManagedControlPlane: &infrav1exp.AzureASOManagedControlPlane{
				Spec: infrav1exp.AzureASOManagedControlPlaneSpec{
					AzureASOManagedControlPlaneTemplateResourceSpec: infrav1exp.AzureASOManagedControlPlaneTemplateResourceSpec{
						Version: "vCAPI k8s version",
						Resources: []runtime.RawExtension{
							{
								Raw: mcJSON(g, &asocontainerservicev1.ManagedCluster{}),
							},
						},
					},
				},
			},
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Pods: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"pod-0", "pod-1"},
						},
						Services: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"svc-0", "svc-1"},
						},
					},
				},
			},
			expected: []*unstructured.Unstructured{
				mcUnstructured(g, &asocontainerservicev1.ManagedCluster{
					Spec: asocontainerservicev1.ManagedCluster_Spec{
						KubernetesVersion: ptr.To("CAPI k8s version"),
						NetworkProfile: &asocontainerservicev1.ContainerServiceNetworkProfile{
							ServiceCidr: ptr.To("svc-0"),
							PodCidr:     ptr.To("pod-0"),
						},
					},
				}),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			s := runtime.NewScheme()
			g.Expect(asocontainerservicev1.AddToScheme(s)).To(Succeed())
			g.Expect(infrav1exp.AddToScheme(s)).To(Succeed())
			c := fakeclient.NewClientBuilder().
				WithScheme(s).
				Build()

			mutator := SetManagedClusterDefaults(c, test.asoManagedControlPlane, test.cluster)
			actual, err := ApplyMutators(ctx, test.asoManagedControlPlane.Spec.Resources, mutator)
			if test.expectedErr != nil {
				g.Expect(err).To(MatchError(test.expectedErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			g.Expect(cmp.Diff(test.expected, actual)).To(BeEmpty())
		})
	}
}

func TestSetManagedClusterKubernetesVersion(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name                   string
		asoManagedControlPlane *infrav1exp.AzureASOManagedControlPlane
		managedCluster         *asocontainerservicev1.ManagedCluster
		expected               *asocontainerservicev1.ManagedCluster
		expectedErr            error
	}{
		{
			name:                   "no CAPI opinion",
			asoManagedControlPlane: &infrav1exp.AzureASOManagedControlPlane{},
			managedCluster: &asocontainerservicev1.ManagedCluster{
				Spec: asocontainerservicev1.ManagedCluster_Spec{
					KubernetesVersion: ptr.To("user k8s version"),
				},
			},
			expected: &asocontainerservicev1.ManagedCluster{
				Spec: asocontainerservicev1.ManagedCluster_Spec{
					KubernetesVersion: ptr.To("user k8s version"),
				},
			},
		},
		{
			name: "set from CAPI opinion",
			asoManagedControlPlane: &infrav1exp.AzureASOManagedControlPlane{
				Spec: infrav1exp.AzureASOManagedControlPlaneSpec{
					AzureASOManagedControlPlaneTemplateResourceSpec: infrav1exp.AzureASOManagedControlPlaneTemplateResourceSpec{
						Version: "vCAPI k8s version",
					},
				},
			},
			managedCluster: &asocontainerservicev1.ManagedCluster{},
			expected: &asocontainerservicev1.ManagedCluster{
				Spec: asocontainerservicev1.ManagedCluster_Spec{
					KubernetesVersion: ptr.To("CAPI k8s version"),
				},
			},
		},
		{
			name: "user value matching CAPI ok",
			asoManagedControlPlane: &infrav1exp.AzureASOManagedControlPlane{
				Spec: infrav1exp.AzureASOManagedControlPlaneSpec{
					AzureASOManagedControlPlaneTemplateResourceSpec: infrav1exp.AzureASOManagedControlPlaneTemplateResourceSpec{
						Version: "vCAPI k8s version",
					},
				},
			},
			managedCluster: &asocontainerservicev1.ManagedCluster{
				Spec: asocontainerservicev1.ManagedCluster_Spec{
					KubernetesVersion: ptr.To("CAPI k8s version"),
				},
			},
			expected: &asocontainerservicev1.ManagedCluster{
				Spec: asocontainerservicev1.ManagedCluster_Spec{
					KubernetesVersion: ptr.To("CAPI k8s version"),
				},
			},
		},
		{
			name: "incompatible",
			asoManagedControlPlane: &infrav1exp.AzureASOManagedControlPlane{
				Spec: infrav1exp.AzureASOManagedControlPlaneSpec{
					AzureASOManagedControlPlaneTemplateResourceSpec: infrav1exp.AzureASOManagedControlPlaneTemplateResourceSpec{
						Version: "vCAPI k8s version",
					},
				},
			},
			managedCluster: &asocontainerservicev1.ManagedCluster{
				Spec: asocontainerservicev1.ManagedCluster_Spec{
					KubernetesVersion: ptr.To("user k8s version"),
				},
			},
			expectedErr: Incompatible{
				mutation: mutation{
					location: ".spec.kubernetesVersion",
					val:      "CAPI k8s version",
					reason:   "because spec.version is set to vCAPI k8s version",
				},
				userVal: "user k8s version",
			},
		},
	}

	s := runtime.NewScheme()
	NewGomegaWithT(t).Expect(asocontainerservicev1.AddToScheme(s)).To(Succeed())

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			before := test.managedCluster.DeepCopy()
			umc := mcUnstructured(g, test.managedCluster)

			err := setManagedClusterKubernetesVersion(ctx, test.asoManagedControlPlane, "", umc)
			g.Expect(s.Convert(umc, test.managedCluster, nil)).To(Succeed())
			if test.expectedErr != nil {
				g.Expect(err).To(MatchError(test.expectedErr))
				g.Expect(cmp.Diff(before, test.managedCluster)).To(BeEmpty()) // errors should never modify the resource.
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(cmp.Diff(test.expected, test.managedCluster)).To(BeEmpty())
			}
		})
	}
}

func TestSetManagedClusterServiceCIDR(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		cluster        *clusterv1.Cluster
		managedCluster *asocontainerservicev1.ManagedCluster
		expected       *asocontainerservicev1.ManagedCluster
		expectedErr    error
	}{
		{
			name:    "no CAPI opinion",
			cluster: &clusterv1.Cluster{},
			managedCluster: &asocontainerservicev1.ManagedCluster{
				Spec: asocontainerservicev1.ManagedCluster_Spec{
					NetworkProfile: &asocontainerservicev1.ContainerServiceNetworkProfile{
						ServiceCidr: ptr.To("user cidr"),
					},
				},
			},
			expected: &asocontainerservicev1.ManagedCluster{
				Spec: asocontainerservicev1.ManagedCluster_Spec{
					NetworkProfile: &asocontainerservicev1.ContainerServiceNetworkProfile{
						ServiceCidr: ptr.To("user cidr"),
					},
				},
			},
		},
		{
			name: "set from CAPI opinion",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Services: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"capi cidr"},
						},
					},
				},
			},
			managedCluster: &asocontainerservicev1.ManagedCluster{},
			expected: &asocontainerservicev1.ManagedCluster{
				Spec: asocontainerservicev1.ManagedCluster_Spec{
					NetworkProfile: &asocontainerservicev1.ContainerServiceNetworkProfile{
						ServiceCidr: ptr.To("capi cidr"),
					},
				},
			},
		},
		{
			name: "user value matching CAPI ok",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Services: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"capi cidr"},
						},
					},
				},
			},
			managedCluster: &asocontainerservicev1.ManagedCluster{
				Spec: asocontainerservicev1.ManagedCluster_Spec{
					NetworkProfile: &asocontainerservicev1.ContainerServiceNetworkProfile{
						ServiceCidr: ptr.To("capi cidr"),
					},
				},
			},
			expected: &asocontainerservicev1.ManagedCluster{
				Spec: asocontainerservicev1.ManagedCluster_Spec{
					NetworkProfile: &asocontainerservicev1.ContainerServiceNetworkProfile{
						ServiceCidr: ptr.To("capi cidr"),
					},
				},
			},
		},
		{
			name: "incompatible",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "ns",
				},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Services: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"capi cidr"},
						},
					},
				},
			},
			managedCluster: &asocontainerservicev1.ManagedCluster{
				Spec: asocontainerservicev1.ManagedCluster_Spec{
					NetworkProfile: &asocontainerservicev1.ContainerServiceNetworkProfile{
						ServiceCidr: ptr.To("user cidr"),
					},
				},
			},
			expectedErr: Incompatible{
				mutation: mutation{
					location: ".spec.networkProfile.serviceCidr",
					val:      "capi cidr",
					reason:   "because spec.clusterNetwork.services.cidrBlocks[0] in Cluster ns/name is set to capi cidr",
				},
				userVal: "user cidr",
			},
		},
	}

	s := runtime.NewScheme()
	NewGomegaWithT(t).Expect(asocontainerservicev1.AddToScheme(s)).To(Succeed())

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			before := test.managedCluster.DeepCopy()
			umc := mcUnstructured(g, test.managedCluster)

			err := setManagedClusterServiceCIDR(ctx, test.cluster, "", umc)
			g.Expect(s.Convert(umc, test.managedCluster, nil)).To(Succeed())
			if test.expectedErr != nil {
				g.Expect(err).To(MatchError(test.expectedErr))
				g.Expect(cmp.Diff(before, test.managedCluster)).To(BeEmpty()) // errors should never modify the resource.
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(cmp.Diff(test.expected, test.managedCluster)).To(BeEmpty())
			}
		})
	}
}

func TestSetManagedClusterPodCIDR(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		cluster        *clusterv1.Cluster
		managedCluster *asocontainerservicev1.ManagedCluster
		expected       *asocontainerservicev1.ManagedCluster
		expectedErr    error
	}{
		{
			name:    "no CAPI opinion",
			cluster: &clusterv1.Cluster{},
			managedCluster: &asocontainerservicev1.ManagedCluster{
				Spec: asocontainerservicev1.ManagedCluster_Spec{
					NetworkProfile: &asocontainerservicev1.ContainerServiceNetworkProfile{
						PodCidr: ptr.To("user cidr"),
					},
				},
			},
			expected: &asocontainerservicev1.ManagedCluster{
				Spec: asocontainerservicev1.ManagedCluster_Spec{
					NetworkProfile: &asocontainerservicev1.ContainerServiceNetworkProfile{
						PodCidr: ptr.To("user cidr"),
					},
				},
			},
		},
		{
			name: "set from CAPI opinion",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Pods: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"capi cidr"},
						},
					},
				},
			},
			managedCluster: &asocontainerservicev1.ManagedCluster{},
			expected: &asocontainerservicev1.ManagedCluster{
				Spec: asocontainerservicev1.ManagedCluster_Spec{
					NetworkProfile: &asocontainerservicev1.ContainerServiceNetworkProfile{
						PodCidr: ptr.To("capi cidr"),
					},
				},
			},
		},
		{
			name: "user value matching CAPI ok",
			cluster: &clusterv1.Cluster{
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Pods: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"capi cidr"},
						},
					},
				},
			},
			managedCluster: &asocontainerservicev1.ManagedCluster{
				Spec: asocontainerservicev1.ManagedCluster_Spec{
					NetworkProfile: &asocontainerservicev1.ContainerServiceNetworkProfile{
						PodCidr: ptr.To("capi cidr"),
					},
				},
			},
			expected: &asocontainerservicev1.ManagedCluster{
				Spec: asocontainerservicev1.ManagedCluster_Spec{
					NetworkProfile: &asocontainerservicev1.ContainerServiceNetworkProfile{
						PodCidr: ptr.To("capi cidr"),
					},
				},
			},
		},
		{
			name: "incompatible",
			cluster: &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "ns",
				},
				Spec: clusterv1.ClusterSpec{
					ClusterNetwork: &clusterv1.ClusterNetwork{
						Pods: &clusterv1.NetworkRanges{
							CIDRBlocks: []string{"capi cidr"},
						},
					},
				},
			},
			managedCluster: &asocontainerservicev1.ManagedCluster{
				Spec: asocontainerservicev1.ManagedCluster_Spec{
					NetworkProfile: &asocontainerservicev1.ContainerServiceNetworkProfile{
						PodCidr: ptr.To("user cidr"),
					},
				},
			},
			expectedErr: Incompatible{
				mutation: mutation{
					location: ".spec.networkProfile.podCidr",
					val:      "capi cidr",
					reason:   "because spec.clusterNetwork.pods.cidrBlocks[0] in Cluster ns/name is set to capi cidr",
				},
				userVal: "user cidr",
			},
		},
	}

	s := runtime.NewScheme()
	NewGomegaWithT(t).Expect(asocontainerservicev1.AddToScheme(s)).To(Succeed())

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewGomegaWithT(t)

			before := test.managedCluster.DeepCopy()
			umc := mcUnstructured(g, test.managedCluster)

			err := setManagedClusterPodCIDR(ctx, test.cluster, "", umc)
			g.Expect(s.Convert(umc, test.managedCluster, nil)).To(Succeed())
			if test.expectedErr != nil {
				g.Expect(err).To(MatchError(test.expectedErr))
				g.Expect(cmp.Diff(before, test.managedCluster)).To(BeEmpty()) // errors should never modify the resource.
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(cmp.Diff(test.expected, test.managedCluster)).To(BeEmpty())
			}
		})
	}
}

func TestSetManagedClusterAgentPoolProfiles(t *testing.T) {
	g := NewGomegaWithT(t)
	ctx := context.Background()
	s := runtime.NewScheme()
	g.Expect(asocontainerservicev1.AddToScheme(s)).To(Succeed())
	g.Expect(infrav1exp.AddToScheme(s)).To(Succeed())
	g.Expect(expv1.AddToScheme(s)).To(Succeed())
	fakeClientBuilder := func() *fakeclient.ClientBuilder {
		return fakeclient.NewClientBuilder().WithScheme(s)
	}

	t.Run("agent pools should not be defined on user's ManagedCluster", func(t *testing.T) {
		g := NewGomegaWithT(t)

		umc := mcUnstructured(g, &asocontainerservicev1.ManagedCluster{
			Spec: asocontainerservicev1.ManagedCluster_Spec{
				AgentPoolProfiles: []asocontainerservicev1.ManagedClusterAgentPoolProfile{{}},
			},
		})

		err := setManagedClusterAgentPoolProfiles(ctx, nil, "", nil, "", umc)
		g.Expect(err).To(MatchError(Incompatible{
			mutation: mutation{
				location: ".spec.agentPoolProfiles",
				val:      "nil",
				reason:   "because agent pool definitions must be inherited from AzureASOManagedMachinePools",
			},
			userVal: "<slice of length 1>",
		}))
	})

	t.Run("agent pool profiles already created", func(t *testing.T) {
		g := NewGomegaWithT(t)

		namespace := "ns"
		managedCluster := &asocontainerservicev1.ManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "mc",
				Namespace: namespace,
			},
			Status: asocontainerservicev1.ManagedCluster_STATUS{
				AgentPoolProfiles: []asocontainerservicev1.ManagedClusterAgentPoolProfile_STATUS{{}},
			},
		}
		umc := mcUnstructured(g, managedCluster)

		c := fakeClientBuilder().
			WithObjects(managedCluster).
			Build()

		err := setManagedClusterAgentPoolProfiles(ctx, c, namespace, nil, "", umc)
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("agent pool profiles derived from managed machine pools", func(t *testing.T) {
		g := NewGomegaWithT(t)

		namespace := "ns"
		clusterName := "cluster"
		managedCluster := &asocontainerservicev1.ManagedCluster{}
		umc := mcUnstructured(g, managedCluster)

		asoManagedMachinePools := &infrav1exp.AzureASOManagedMachinePoolList{
			Items: []infrav1exp.AzureASOManagedMachinePool{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "wrong-label",
						Namespace: namespace,
						Labels: map[string]string{
							clusterv1.ClusterNameLabel: "not-" + clusterName,
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: expv1.GroupVersion.Identifier(),
								Kind:       "MachinePool",
								Name:       "wrong-label",
							},
						},
					},
					Spec: infrav1exp.AzureASOManagedMachinePoolSpec{
						AzureASOManagedMachinePoolTemplateResourceSpec: infrav1exp.AzureASOManagedMachinePoolTemplateResourceSpec{
							Resources: []runtime.RawExtension{
								{
									Raw: apJSON(g, &asocontainerservicev1.ManagedClustersAgentPool{
										Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
											AzureName: "no",
										},
									}),
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "wrong-namespace",
						Namespace: "not-" + namespace,
						Labels: map[string]string{
							clusterv1.ClusterNameLabel: clusterName,
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: expv1.GroupVersion.Identifier(),
								Kind:       "MachinePool",
								Name:       "wrong-namespace",
							},
						},
					},
					Spec: infrav1exp.AzureASOManagedMachinePoolSpec{
						AzureASOManagedMachinePoolTemplateResourceSpec: infrav1exp.AzureASOManagedMachinePoolTemplateResourceSpec{
							Resources: []runtime.RawExtension{
								{
									Raw: apJSON(g, &asocontainerservicev1.ManagedClustersAgentPool{
										Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
											AzureName: "no",
										},
									}),
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pool0",
						Namespace: namespace,
						Labels: map[string]string{
							clusterv1.ClusterNameLabel: clusterName,
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: expv1.GroupVersion.Identifier(),
								Kind:       "MachinePool",
								Name:       "pool0",
							},
						},
					},
					Spec: infrav1exp.AzureASOManagedMachinePoolSpec{
						AzureASOManagedMachinePoolTemplateResourceSpec: infrav1exp.AzureASOManagedMachinePoolTemplateResourceSpec{
							Resources: []runtime.RawExtension{
								{
									Raw: apJSON(g, &asocontainerservicev1.ManagedClustersAgentPool{
										Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
											AzureName: "azpool0",
										},
									}),
								},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pool1",
						Namespace: namespace,
						Labels: map[string]string{
							clusterv1.ClusterNameLabel: clusterName,
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: expv1.GroupVersion.Identifier(),
								Kind:       "MachinePool",
								Name:       "pool1",
							},
						},
					},
					Spec: infrav1exp.AzureASOManagedMachinePoolSpec{
						AzureASOManagedMachinePoolTemplateResourceSpec: infrav1exp.AzureASOManagedMachinePoolTemplateResourceSpec{
							Resources: []runtime.RawExtension{
								{
									Raw: apJSON(g, &asocontainerservicev1.ManagedClustersAgentPool{
										Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
											AzureName: "azpool1",
										},
									}),
								},
							},
						},
					},
				},
			},
		}
		machinePools := &expv1.MachinePoolList{
			Items: []expv1.MachinePool{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "wrong-label",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "not-" + namespace,
						Name:      "wrong-namespace",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "pool0",
					},
					Spec: expv1.MachinePoolSpec{
						Replicas: ptr.To[int32](1),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: namespace,
						Name:      "pool1",
					},
					Spec: expv1.MachinePoolSpec{
						Replicas: ptr.To[int32](2),
					},
				},
			},
		}
		expected := &asocontainerservicev1.ManagedCluster{
			Spec: asocontainerservicev1.ManagedCluster_Spec{
				AgentPoolProfiles: []asocontainerservicev1.ManagedClusterAgentPoolProfile{
					{Name: ptr.To("azpool0"), Count: ptr.To(1)},
					{Name: ptr.To("azpool1"), Count: ptr.To(2)},
				},
			},
		}

		c := fakeClientBuilder().
			WithLists(asoManagedMachinePools, machinePools).
			Build()

		cluster := &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: clusterName}}
		err := setManagedClusterAgentPoolProfiles(ctx, c, namespace, cluster, "", umc)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(s.Convert(umc, managedCluster, nil)).To(Succeed())
		g.Expect(cmp.Diff(expected, managedCluster)).To(BeEmpty())
	})
}

func TestSetAgentPoolProfilesFromAgentPools(t *testing.T) {
	t.Run("stable with no pools", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mc := &asocontainerservicev1.ManagedCluster{}
		var pools []conversion.Convertible
		var expected []asocontainerservicev1.ManagedClusterAgentPoolProfile

		err := setAgentPoolProfilesFromAgentPools(mc, pools)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(cmp.Diff(expected, mc.Spec.AgentPoolProfiles)).To(BeEmpty())
	})

	t.Run("stable with pools", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mc := &asocontainerservicev1.ManagedCluster{}
		pools := []conversion.Convertible{
			&asocontainerservicev1.ManagedClustersAgentPool{
				Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
					AzureName: "pool0",
					MaxCount:  ptr.To(1),
				},
			},
			// Not all pools have to be the same version, or the same version as the cluster.
			&asocontainerservicev1preview.ManagedClustersAgentPool{
				Spec: asocontainerservicev1preview.ManagedClusters_AgentPool_Spec{
					AzureName:           "pool1",
					MinCount:            ptr.To(2),
					EnableCustomCATrust: ptr.To(true),
				},
			},
		}
		expected := []asocontainerservicev1.ManagedClusterAgentPoolProfile{
			{
				Name:     ptr.To("pool0"),
				MaxCount: ptr.To(1),
			},
			{
				Name:     ptr.To("pool1"),
				MinCount: ptr.To(2),
				// EnableCustomCATrust is a preview-only feature that can't be represented here, so it should be lost.
			},
		}

		err := setAgentPoolProfilesFromAgentPools(mc, pools)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(cmp.Diff(expected, mc.Spec.AgentPoolProfiles)).To(BeEmpty())
	})

	t.Run("preview with pools", func(t *testing.T) {
		g := NewGomegaWithT(t)

		mc := &asocontainerservicev1preview.ManagedCluster{}
		pools := []conversion.Convertible{
			&asocontainerservicev1.ManagedClustersAgentPool{
				Spec: asocontainerservicev1.ManagedClusters_AgentPool_Spec{
					AzureName: "pool0",
					MaxCount:  ptr.To(1),
				},
			},
			&asocontainerservicev1preview.ManagedClustersAgentPool{
				Spec: asocontainerservicev1preview.ManagedClusters_AgentPool_Spec{
					AzureName:           "pool1",
					MinCount:            ptr.To(2),
					EnableCustomCATrust: ptr.To(true),
				},
			},
		}
		expected := []asocontainerservicev1preview.ManagedClusterAgentPoolProfile{
			{
				Name:     ptr.To("pool0"),
				MaxCount: ptr.To(1),
			},
			{
				Name:                ptr.To("pool1"),
				MinCount:            ptr.To(2),
				EnableCustomCATrust: ptr.To(true),
			},
		}

		err := setAgentPoolProfilesFromAgentPools(mc, pools)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(cmp.Diff(expected, mc.Spec.AgentPoolProfiles)).To(BeEmpty())
	})
}

func mcJSON(g Gomega, mc *asocontainerservicev1.ManagedCluster) []byte {
	mc.SetGroupVersionKind(asocontainerservicev1.GroupVersion.WithKind("ManagedCluster"))
	j, err := json.Marshal(mc)
	g.Expect(err).NotTo(HaveOccurred())
	return j
}

func apJSON(g Gomega, mc *asocontainerservicev1.ManagedClustersAgentPool) []byte {
	mc.SetGroupVersionKind(asocontainerservicev1.GroupVersion.WithKind("ManagedClustersAgentPool"))
	j, err := json.Marshal(mc)
	g.Expect(err).NotTo(HaveOccurred())
	return j
}

func mcUnstructured(g Gomega, mc *asocontainerservicev1.ManagedCluster) *unstructured.Unstructured {
	s := runtime.NewScheme()
	g.Expect(asocontainerservicev1.AddToScheme(s)).To(Succeed())
	u := &unstructured.Unstructured{}
	g.Expect(s.Convert(mc, u, nil)).To(Succeed())
	return u
}

func apUnstructured(g Gomega, ap *asocontainerservicev1.ManagedClustersAgentPool) *unstructured.Unstructured {
	s := runtime.NewScheme()
	g.Expect(asocontainerservicev1.AddToScheme(s)).To(Succeed())
	u := &unstructured.Unstructured{}
	g.Expect(s.Convert(ap, u, nil)).To(Succeed())
	return u
}

// /*
// Copyright 2019 The Kubernetes Authors.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

package e2e_test

// import (
// 	"context"
// 	"fmt"
// 	"time"

// 	. "github.com/onsi/ginkgo"
// 	. "github.com/onsi/gomega"
// 	. "github.com/onsi/gomega/gstruct"
// 	"github.com/onsi/gomega/types"

// 	corev1 "k8s.io/api/core/v1"
// 	apierrors "k8s.io/apimachinery/pkg/api/errors"
// 	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
// 	apimachinerytypes "k8s.io/apimachinery/pkg/types"
// 	"k8s.io/client-go/kubernetes"
// 	capz "sigs.k8s.io/cluster-api-provider-azure/apis/v1alpha2"
// 	capi "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
// 	"sigs.k8s.io/cluster-api/pkg/util"
// 	crclient "sigs.k8s.io/controller-runtime/pkg/client"
// )

// const (
// 	workerClusterK8sVersion = "v1.15.3"
// )

// var _ = Describe("functional tests", func() {
// 	var (
// 		namespace   string
// 		clusterName string
// 	)
// 	BeforeEach(func() {
// 		namespace = "test-" + util.RandomString(6)
// 		fmt.Fprintf(GinkgoWriter, "creating namespace %q\n", namespace)
// 		createNamespace(kindCluster.KubeClient(), namespace)
// 		clusterName = "test-" + util.RandomString(6)
// 	})

// 	Describe("workload cluster lifecycle", func() {
// 		It("It should be creatable and deletable", func() {
// 			By("Creating a Cluster resource")
// 			fmt.Fprintf(GinkgoWriter, "Creating Cluster named %q\n", clusterName)
// 			Expect(kindClient.Create(context.TODO(), makeCluster(clusterName))).To(BeNil())

// 			fmt.Fprintf(GinkgoWriter, "Ensuring cluster infrastructure is ready\n")
// 			Eventually(
// 				func() (map[string]string, error) {
// 					cluster := &capi.Cluster{}
// 					if err := kindClient.Get(context.TODO(), apimachinerytypes.NamespacedName{Namespace: namespace, Name: clusterName}, cluster); err != nil {
// 						return nil, err
// 					}
// 					return cluster.Annotations, nil
// 				},
// 				10*time.Minute, 15*time.Second,
// 			).Should(HaveKeyWithValue(capz.AnnotationClusterInfrastructureReady, capz.ValueReady))

// 			By("Creating the first control plane Machine resource")
// 			machineName := "cp-1"
// 			fmt.Fprintf(GinkgoWriter, "Creating Machine named %q for Cluster %q\n", machineName, clusterName)
// 			Expect(kindClient.Create(context.TODO(), makeMachine(machineName, clusterName, "controlplane", workerClusterK8sVersion))).To(BeNil())

// 			fmt.Fprintf(GinkgoWriter, "Ensuring first control plane Machine is ready\n")
// 			Eventually(
// 				func() (*capz.AzureMachineProviderStatus, error) {
// 					machine := &capi.Machine{}
// 					if err := kindClient.Get(context.TODO(), apimachinerytypes.NamespacedName{Namespace: namespace, Name: machineName}, machine); err != nil {
// 						return nil, err
// 					}
// 					if machine.Status.ProviderStatus == nil {
// 						return &capz.AzureMachineProviderStatus{
// 							VMState: &capz.VMStateCreating,
// 						}, nil
// 					}
// 					return capz.MachineStatusFromProviderStatus(machine.Status.ProviderStatus)
// 				},
// 				10*time.Minute, 15*time.Second,
// 			).Should(beHealthy())

// 			fmt.Fprintf(GinkgoWriter, "Ensuring first control plane Machine NodeRef is set\n")
// 			Eventually(
// 				func() (*corev1.ObjectReference, error) {
// 					machine := &capi.Machine{}
// 					if err := kindClient.Get(context.TODO(), apimachinerytypes.NamespacedName{Namespace: namespace, Name: machineName}, machine); err != nil {
// 						return nil, err
// 					}
// 					return machine.Status.NodeRef, nil

// 				},
// 				10*time.Minute, 15*time.Second,
// 			).ShouldNot(BeNil())

// 			fmt.Fprintf(GinkgoWriter, "Ensuring Cluster reports the Control Plane is ready\n")
// 			Eventually(
// 				func() (map[string]string, error) {
// 					cluster := &capi.Cluster{}
// 					if err := kindClient.Get(context.TODO(), apimachinerytypes.NamespacedName{Namespace: namespace, Name: clusterName}, cluster); err != nil {
// 						return nil, err
// 					}
// 					return cluster.Annotations, nil
// 				},
// 				10*time.Minute, 15*time.Second,
// 			).Should(HaveKeyWithValue(capz.AnnotationControlPlaneReady, capz.ValueReady))

// 			// TODO: Retrieve Cluster kubeconfig
// 			// TODO: Deploy Addons
// 			// TODO: Validate Node Ready
// 			// TODO: Deploy additional Control Plane Nodes
// 			// TODO: Deploy a MachineDeployment
// 			// TODO: Scale MachineDeployment up
// 			// TODO: Scale MachineDeployment down

// 			By("Deleting cluster")
// 			fmt.Fprintf(GinkgoWriter, "Deleting Cluster named %q\n", clusterName)
// 			Expect(kindClient.Delete(context.TODO(), &capi.Cluster{
// 				ObjectMeta: metav1.ObjectMeta{
// 					Namespace: namespace,
// 					Name:      clusterName,
// 				},
// 			}, noOptionsDelete())).To(BeNil())

// 			Eventually(
// 				func() *capi.Cluster {
// 					cluster := &capi.Cluster{}
// 					if err := kindClient.Get(context.TODO(), apimachinerytypes.NamespacedName{Namespace: namespace, Name: clusterName}, cluster); err != nil {
// 						if apierrors.IsNotFound(err) {
// 							return nil
// 						}
// 						return &capi.Cluster{}
// 					}
// 					return cluster
// 				},
// 				20*time.Minute, 15*time.Second,
// 			).Should(BeNil())
// 		})
// 	})
// })

// func noOptionsDelete() crclient.DeleteOptionFunc {
// 	return func(opts *crclient.DeleteOptions) {}
// }

// func beHealthy() types.GomegaMatcher {
// 	return PointTo(
// 		MatchFields(IgnoreExtras, Fields{
// 			"VMState": PointTo(Equal(capz.VMStateSucceeded)),
// 		}),
// 	)
// }

// func makeCluster(name string) *capi.Cluster {
// 	providerSpecValue, err := capz.EncodeClusterSpec(&capz.AzureClusterProviderSpec{
// 		// TODO: Determine bare minimum cluster spec to define here
// 	})
// 	Expect(err).To(BeNil())

// 	cluster := &capi.Cluster{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name: name,
// 		},
// 		Spec: capi.ClusterSpec{
// 			ClusterNetwork: capi.ClusterNetworkingConfig{
// 				Services: capi.NetworkRanges{
// 					CIDRBlocks: []string{"10.96.0.0/12"},
// 				},
// 				Pods: capi.NetworkRanges{
// 					CIDRBlocks: []string{"192.168.0.0/16"},
// 				},
// 				ServiceDomain: "cluster.local",
// 			},
// 			ProviderSpec: capi.ProviderSpec{
// 				Value: providerSpecValue,
// 			},
// 		},
// 	}

// 	return cluster
// }

// func makeMachine(name, clusterName, role, k8sVersion string) *capi.Machine {
// 	var instanceRole string
// 	machineVersionInfo := capi.MachineVersionInfo{
// 		Kubelet: k8sVersion,
// 	}

// 	switch role {
// 	case "control-plane":
// 		instanceRole = "control-plane.cluster-api-provider-azure.sigs.k8s.io"
// 		machineVersionInfo.ControlPlane = k8sVersion
// 	case "node":
// 		instanceRole = "nodes.cluster-api-provider-azure.sigs.k8s.io"
// 	}
// 	Expect(instanceRole).ToNot(BeEmpty())

// 	providerSpecValue, err := capz.EncodeMachineSpec(&capz.AzureMachineProviderSpec{
// 		// TODO: Determine bare minimum machine spec to define here
// 		VMSize: "Standard_B2ms",
// 	})
// 	Expect(err).To(BeNil())

// 	machine := &capi.Machine{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name: name,
// 			Labels: map[string]string{
// 				capi.MachineClusterLabelName: clusterName,
// 				"set":                        role,
// 			},
// 		},
// 		Spec: capi.MachineSpec{
// 			Versions: machineVersionInfo,
// 			ProviderSpec: capi.ProviderSpec{
// 				Value: providerSpecValue,
// 			},
// 		},
// 	}

// 	return machine
// }

// func createNamespace(client kubernetes.Interface, namespace string) {
// 	_, err := client.CoreV1().Namespaces().Create(&corev1.Namespace{
// 		ObjectMeta: metav1.ObjectMeta{
// 			Name: namespace,
// 		},
// 	})
// 	Expect(err).To(BeNil())
// }

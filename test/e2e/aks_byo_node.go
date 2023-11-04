//go:build e2e
// +build e2e

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

package e2e

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type AKSBYONodeSpecInput struct {
	Cluster             *clusterv1.Cluster
	KubernetesVersion   string
	WaitIntervals       []interface{}
	ExpectedWorkerNodes int32
}

func AKSBYONodeSpec(ctx context.Context, inputGetter func() AKSBYONodeSpecInput) {
	input := inputGetter()

	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())

	infraControlPlane := &infrav1.AzureManagedControlPlane{}
	err := mgmtClient.Get(ctx, client.ObjectKey{Namespace: input.Cluster.Spec.ControlPlaneRef.Namespace, Name: input.Cluster.Spec.ControlPlaneRef.Name}, infraControlPlane)
	Expect(err).NotTo(HaveOccurred())

	By("Creating a self-managed machine pool with 2 nodes")
	infraMachinePool := &infrav1exp.AzureMachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "byo-pool",
			Namespace: input.Cluster.Namespace,
		},
		Spec: infrav1exp.AzureMachinePoolSpec{
			Location: infraControlPlane.Spec.Location,
			Template: infrav1exp.AzureMachinePoolMachineTemplate{
				VMSize: os.Getenv("AZURE_NODE_MACHINE_TYPE"),
			},
		},
	}
	err = mgmtClient.Create(ctx, infraMachinePool)
	Expect(err).NotTo(HaveOccurred())

	kubeadmConfig := &bootstrapv1.KubeadmConfig{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: infraMachinePool.Namespace,
			Name:      infraMachinePool.Name,
		},
		Spec: bootstrapv1.KubeadmConfigSpec{
			Files: []bootstrapv1.File{
				{
					ContentFrom: &bootstrapv1.FileSource{
						Secret: bootstrapv1.SecretFileSource{
							Name: infraMachinePool.Name + "-azure-json",
							Key:  "worker-node-azure.json",
						},
					},
					Path:        "/etc/kubernetes/azure.json",
					Permissions: "0644",
					Owner:       "root:root",
				},
				{
					ContentFrom: &bootstrapv1.FileSource{
						Secret: bootstrapv1.SecretFileSource{
							Name: input.Cluster.Name + "-kubeconfig",
							Key:  "value",
						},
					},
					Path:        "/etc/kubernetes/admin.conf",
					Permissions: "0644",
					Owner:       "root:root",
				},
			},
			JoinConfiguration: &bootstrapv1.JoinConfiguration{
				Discovery: bootstrapv1.Discovery{
					File: &bootstrapv1.FileDiscovery{
						KubeConfigPath: "/etc/kubernetes/admin.conf",
					},
				},
				NodeRegistration: bootstrapv1.NodeRegistrationOptions{
					Name: "{{ ds.meta_data[\"local_hostname\"] }}",
					KubeletExtraArgs: map[string]string{
						"cloud-provider":                  "external",
						"azure-container-registry-config": "/etc/kubernetes/azure.json",
					},
				},
			},
			PreKubeadmCommands: []string{"kubeadm init phase upload-config all"},
		},
	}
	err = mgmtClient.Create(ctx, kubeadmConfig)
	Expect(err).NotTo(HaveOccurred())

	machinePool := &expv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: infraMachinePool.Namespace,
			Name:      infraMachinePool.Name,
		},
		Spec: expv1.MachinePoolSpec{
			ClusterName: input.Cluster.Name,
			Replicas:    ptr.To[int32](2),
			Template: clusterv1.MachineTemplateSpec{
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						ConfigRef: &corev1.ObjectReference{
							APIVersion: bootstrapv1.GroupVersion.String(),
							Kind:       "KubeadmConfig",
							Name:       kubeadmConfig.Name,
						},
					},
					ClusterName: input.Cluster.Name,
					InfrastructureRef: corev1.ObjectReference{
						APIVersion: infrav1.GroupVersion.String(),
						Kind:       "AzureMachinePool",
						Name:       infraMachinePool.Name,
					},
					Version: ptr.To(input.KubernetesVersion),
				},
			},
		},
	}
	err = mgmtClient.Create(ctx, machinePool)
	Expect(err).NotTo(HaveOccurred())

	By("creating a Kubernetes client to the workload cluster")
	workloadClusterProxy := bootstrapClusterProxy.GetWorkloadCluster(ctx, input.Cluster.Spec.ControlPlaneRef.Namespace, input.Cluster.Spec.ControlPlaneRef.Name)
	Expect(workloadClusterProxy).NotTo(BeNil())
	clientset := workloadClusterProxy.GetClientSet()
	Expect(clientset).NotTo(BeNil())

	By("Verifying the bootstrap succeeded")
	Eventually(func(g Gomega) {
		pool := &infrav1exp.AzureMachinePool{}
		err := mgmtClient.Get(ctx, client.ObjectKeyFromObject(infraMachinePool), pool)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(conditions.IsTrue(pool, infrav1.BootstrapSucceededCondition)).To(BeTrue())
	}, input.WaitIntervals...).Should(Succeed())

	By("Adding the expected AKS labels to the nodes")
	// TODO: move this to the MachinePool object once MachinePools support label propagation
	Eventually(func(g Gomega) {
		nodeList, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(int32(len(nodeList.Items))).To(Equal(input.ExpectedWorkerNodes + 2))
		for i, node := range nodeList.Items {
			if _, ok := node.Labels["kubernetes.azure.com/cluster"]; !ok {
				node.Labels["kubernetes.azure.com/cluster"] = infraControlPlane.Spec.NodeResourceGroupName
				_, err := clientset.CoreV1().Nodes().Update(ctx, &nodeList.Items[i], metav1.UpdateOptions{})
				g.Expect(err).NotTo(HaveOccurred())
			}
		}
	}, input.WaitIntervals...).Should(Succeed())

	By("Verifying the MachinePool becomes ready")
	Eventually(func(g Gomega) {
		pool := &expv1.MachinePool{}
		err := mgmtClient.Get(ctx, client.ObjectKeyFromObject(machinePool), pool)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(conditions.IsTrue(pool, clusterv1.ReadyCondition)).To(BeTrue())
	}, input.WaitIntervals...).Should(Succeed())
}

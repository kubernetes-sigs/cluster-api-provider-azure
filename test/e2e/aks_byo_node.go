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
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	bootstrapv1 "sigs.k8s.io/cluster-api/api/bootstrap/kubeadm/v1beta2"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	conditions "sigs.k8s.io/cluster-api/util/conditions"
	deprecatedv1beta1conditions "sigs.k8s.io/cluster-api/util/conditions/deprecated/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta2"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
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
	err := mgmtClient.Get(ctx, client.ObjectKey{Namespace: input.Cluster.Namespace, Name: input.Cluster.Spec.ControlPlaneRef.Name}, infraControlPlane)
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
					ContentFrom: bootstrapv1.FileSource{
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
					ContentFrom: bootstrapv1.FileSource{
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
			JoinConfiguration: bootstrapv1.JoinConfiguration{
				Discovery: bootstrapv1.Discovery{
					File: bootstrapv1.FileDiscovery{
						KubeConfigPath: "/etc/kubernetes/admin.conf",
					},
				},
				NodeRegistration: bootstrapv1.NodeRegistrationOptions{
					Name: "{{ ds.meta_data[\"local_hostname\"] }}",
					KubeletExtraArgs: []bootstrapv1.Arg{
						{
							Name:  "cloud-provider",
							Value: ptr.To("external"),
						},
					},
				},
			},
			// Copy the AKS admin kubeconfig to a non-default path before running
			// "upload-config". As of kubeadm v1.35.5, "init" special-cases the
			// default /etc/kubernetes/admin.conf path and reroutes the client to
			// the local API endpoint (this node's own IP), which a self-managed
			// worker doesn't run, so upload-config fails and the kubeadm-config
			// ConfigMap is never created. Pointing --kubeconfig at a non-default
			// path bypasses that rewrite and targets the AKS control plane. This
			// is backward-compatible with older kubeadm versions.
			PreKubeadmCommands: []string{
				"cp /etc/kubernetes/admin.conf /etc/kubernetes/aks-admin.conf",
				"kubeadm init phase upload-config all --kubeconfig /etc/kubernetes/aks-admin.conf",
			},
		},
	}
	err = mgmtClient.Create(ctx, kubeadmConfig)
	Expect(err).NotTo(HaveOccurred())

	machinePool := &clusterv1.MachinePool{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: infraMachinePool.Namespace,
			Name:      infraMachinePool.Name,
		},
		Spec: clusterv1.MachinePoolSpec{
			ClusterName: input.Cluster.Name,
			Replicas:    ptr.To[int32](2),
			Template: clusterv1.MachineTemplateSpec{
				Spec: clusterv1.MachineSpec{
					Bootstrap: clusterv1.Bootstrap{
						ConfigRef: clusterv1.ContractVersionedObjectReference{
							APIGroup: bootstrapv1.GroupVersion.Group,
							Kind:     "KubeadmConfig",
							Name:     kubeadmConfig.Name,
						},
					},
					ClusterName: input.Cluster.Name,
					InfrastructureRef: clusterv1.ContractVersionedObjectReference{
						APIGroup: infrav1.GroupVersion.Group,
						Kind:     "AzureMachinePool",
						Name:     infraMachinePool.Name,
					},
					Version: input.KubernetesVersion,
				},
			},
		},
	}
	err = mgmtClient.Create(ctx, machinePool)
	Expect(err).NotTo(HaveOccurred())

	By("creating a Kubernetes client to the workload cluster")
	workloadClusterProxy := bootstrapClusterProxy.GetWorkloadCluster(ctx, input.Cluster.Namespace, input.Cluster.Spec.ControlPlaneRef.Name)
	Expect(workloadClusterProxy).NotTo(BeNil())
	clientset := workloadClusterProxy.GetClientSet()
	Expect(clientset).NotTo(BeNil())

	By("Verifying the bootstrap succeeded")
	Eventually(func(g Gomega) {
		pool := &infrav1exp.AzureMachinePool{}
		err := mgmtClient.Get(ctx, client.ObjectKeyFromObject(infraMachinePool), pool)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(conditions.IsTrue(pool, string(infrav1.BootstrapSucceededCondition))).To(BeTrue())
	}, input.WaitIntervals...).Should(Succeed())

	By("Adding the expected AKS labels to the nodes")
	// TODO: move this to the MachinePool object once MachinePools support label propagation
	// The standard e2e log collector can't reach BYO nodes that never joined
	// (it SSHes through a control-plane bastion, which a managed AKS control
	// plane doesn't have). If the nodes never appear, collect BYO node
	// diagnostics inline (before any cleanup runs) so the failure is
	// diagnosable from CI artifacts. See #6354.
	if failure := InterceptGomegaFailure(func() {
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
	}); failure != nil {
		outputPath := filepath.Join(artifactFolder, "clusters", input.Cluster.Name, "byo-pool-debug")
		collectBYONodeDiagnostics(ctx, clientset, getSubscriptionID(Default), infraControlPlane.Spec.NodeResourceGroupName, "byo-pool", outputPath)
		Fail(failure.Error())
	}

	By("Verifying the MachinePool becomes ready")
	Eventually(func(g Gomega) {
		pool := &clusterv1.MachinePool{}
		err := mgmtClient.Get(ctx, client.ObjectKeyFromObject(machinePool), pool)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(deprecatedv1beta1conditions.IsTrue(pool, clusterv1.ReadyV1Beta1Condition)).To(BeTrue())
	}, input.WaitIntervals...).Should(Succeed())
}

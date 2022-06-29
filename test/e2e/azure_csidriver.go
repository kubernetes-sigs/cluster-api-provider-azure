//go:build e2e
// +build e2e

/*
Copyright 2022 The Kubernetes Authors.

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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	deploymentBuilder "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/deployment"
	e2e_pvc "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/pvc"
	e2e_sc "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/storageclass"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util"
)

type AzureDiskCSISpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	SkipCleanup           bool
}

// AzureDiskCSISpec implements a test that verifies out of tree azure disk csi driver
// can be used to create a PVC that is usable by a pod.
func AzureDiskCSISpec(ctx context.Context, inputGetter func() AzureDiskCSISpecInput){
	specName    := "azurediskcsi-driver"
	input := inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)

	By("creating a Kubernetes client to the workload cluster")
	clusterProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
	Expect(clusterProxy).NotTo(BeNil())
	clientset := clusterProxy.GetClientSet()
	Expect(clientset).NotTo(BeNil())

	By("Deploying managed disk storage class")
	e2e_sc.Create("managedhdd").WithWaitForFirstConsumer().DeployStorageClass(clientset)

	By("Deploying persistent volume claim")
	b,err := e2e_pvc.Create("dd-managed-hdd-5g", "5Gi")
	Expect(err).To(BeNil())
	b.DeployPVC(clientset)

	By("creating a deployment that uses pvc")
	deploymentName := "stateful" + util.RandomString(6)
	statefulDeployment := deploymentBuilder.Create("nginx", deploymentName, corev1.NamespaceDefault).AddPVC("dd-managed-hdd-5g")
	deployment, err := statefulDeployment.Deploy(ctx, clientset)
	Expect(err).NotTo(HaveOccurred())
	deployInput := WaitForDeploymentsAvailableInput{
		Getter:     deploymentsClientAdapter{client: statefulDeployment.Client(clientset)},
		Deployment: deployment,
		Clientset:  clientset,
	}
	WaitForDeploymentsAvailable(ctx, deployInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
}

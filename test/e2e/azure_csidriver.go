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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	deploymentBuilder "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/deployment"
	e2e_pvc "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/pvc"
	e2e_sc "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/storageclass"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util"
)

const (
	DriverTypeInternal = "Internal"
)

type AzureDiskCSISpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	SkipCleanup           bool
	DriverType            string
}

// AzureDiskCSISpec implements a test that verifies out of tree azure disk csi driver
// can be used to create a PVC that is usable by a pod.
func AzureDiskCSISpec(ctx context.Context, inputGetter func() AzureDiskCSISpecInput) *appsv1.Deployment {
	specName := "azurediskcsi-driver"
	input := inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)

	By("creating a Kubernetes client to the workload cluster")
	clusterProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
	Expect(clusterProxy).NotTo(BeNil())
	clientset := clusterProxy.GetClientSet()
	Expect(clientset).NotTo(BeNil())

	var pvcName string
	if input.DriverType == DriverTypeInternal {
		By("[In-tree]Deploying storage class and pvc")
		By("Deploying managed disk storage class")
		scName := "managedhdd" + util.RandomString(6)
		e2e_sc.Create(scName).WithWaitForFirstConsumer().DeployStorageClass(clientset)
		By("Deploying persistent volume claim")
		pvcName = "dd-managed-hdd-5g" + util.RandomString(6)
		pvcBuilder, err := e2e_pvc.Create(pvcName, "5Gi")
		Expect(err).NotTo(HaveOccurred())
		annotations := map[string]string{
			"volume.beta.kubernetes.io/storage-class": scName,
		}
		pvcBuilder.WithAnnotations(annotations)
		err = pvcBuilder.DeployPVC(clientset)
		Expect(err).NotTo(HaveOccurred())
	} else {
		By("[External]Deploying storage class and pvc")
		By("Deploying managed disk storage class")
		scName := "oot-managedhdd" + util.RandomString(6)
		e2e_sc.Create(scName).WithWaitForFirstConsumer().
			WithOotProvisionerName().
			WithOotParameters().
			DeployStorageClass(clientset)
		By("Deploying persistent volume claim")
		pvcName = "oot-dd-managed-hdd-5g" + util.RandomString(6)
		pvcBuilder, err := e2e_pvc.Create(pvcName, "5Gi")
		Expect(err).NotTo(HaveOccurred())
		pvcBuilder.WithStorageClass(scName)
		err = pvcBuilder.DeployPVC(clientset)
		Expect(err).NotTo(HaveOccurred())
	}

	By("creating a deployment that uses pvc")
	deploymentName := "stateful" + util.RandomString(6)
	statefulDeployment := deploymentBuilder.Create("nginx", deploymentName, corev1.NamespaceDefault).AddPVC(pvcName)
	deployment, err := statefulDeployment.Deploy(ctx, clientset)
	Expect(err).NotTo(HaveOccurred())
	waitForDeploymentAvailable(ctx, deployment, clientset, specName)
	return deployment
}

func waitForDeploymentAvailable(ctx context.Context, deployment *appsv1.Deployment, clientset *kubernetes.Clientset, specName string) {
	deployInput := WaitForDeploymentsAvailableInput{
		Getter:     deploymentsClientAdapter{client: clientset.AppsV1().Deployments(deployment.Namespace)},
		Deployment: deployment,
		Clientset:  clientset,
	}
	WaitForDeploymentsAvailable(ctx, deployInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)
}

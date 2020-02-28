// +build e2e

/*
Copyright 2019 The Kubernetes Authors.

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
	"time"

	. "github.com/onsi/ginkgo"

	"k8s.io/client-go/kubernetes/scheme"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	capiv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/test/framework"
)

func init() {
	clusterv1.AddToScheme(scheme.Scheme)
}

var _ = Describe("CAPZ e2e tests", func() {
	Describe("Cluster creation", func() {

		var (
			clusterGen   *ClusterGenerator
			nodeGen      *NodeGenerator
			cluster      *capiv1.Cluster
			infraCluster *infrav1.AzureCluster
			input        *ControlPlaneClusterInput
			//machineDeploymentGen = &MachineDeploymentGenerator{}
		)

		BeforeEach(func() {
			clusterGen = &ClusterGenerator{}
			nodeGen = &NodeGenerator{}
			cluster, infraCluster = clusterGen.GenerateCluster(namespace)
		})

		AfterEach(func() {
			By("cleaning up e2e resources")
			ensureCAPZArtifactsDeleted(input)
		})

		Context("Create single controlplane cluster", func() {
			It("Should create a single node cluster", func() {
				nodes := []framework.Node{nodeGen.GenerateNode(creds, cluster.GetName())}
				input = &ControlPlaneClusterInput{
					Management:    mgmt,
					Cluster:       cluster,
					InfraCluster:  infraCluster,
					Nodes:         nodes,
					CreateTimeout: 30 * time.Minute,
				}
				ControlPlaneCluster(input)
			})
		})

		// todo: re-enable this test once we fix it
		// Context("Create multiple controlplane cluster with machine deployments", func() {
		// 	It("Should create a 3 node cluster", func() {
		// 		nodes := []framework.Node{nodeGen.GenerateNode(creds, cluster.GetName()), nodeGen.GenerateNode(creds, cluster.GetName()), nodeGen.GenerateNode(creds, cluster.GetName())}
		// 		machineDeployment := machineDeploymentGen.Generate(creds, cluster.GetNamespace(), cluster.GetName(), 1)
		// 		ControlPlaneCluster(&ControlPlaneClusterInput{
		// 			Management:        mgmt,
		// 			Cluster:           cluster,
		// 			InfraCluster:      infraCluster,
		// 			Nodes:             nodes,
		// 			MachineDeployment: machineDeployment,
		// 			CreateTimeout:     30 * time.Minute,
		// 		})
		// 	})
		// })
	})
})

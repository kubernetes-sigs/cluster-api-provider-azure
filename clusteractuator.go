/*
Copyright 2018 The Kubernetes Authors.
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

package azureactuator

import (
	"fmt"

	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	client "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
)

type AzureClusterClient struct {
	clusterClient client.ClusterInterface
}

type ClusterActuatorParams struct {
	ClusterClient client.ClusterInterface
}

func NewClusterActuator(params ClusterActuatorParams) (*AzureClusterClient, error) {
	return &AzureClusterClient{
		clusterClient: params.ClusterClient,
	}, nil
}

func (gce *AzureClusterClient) Reconcile(cluster *clusterv1.Cluster) error {
	return fmt.Errorf("NYI: Cluster Reconciles are not yet supported")
}

func (gce *AzureClusterClient) Delete(cluster *clusterv1.Cluster) error {
	return fmt.Errorf("NYI: Cluster Deletions are not yet supported")
}

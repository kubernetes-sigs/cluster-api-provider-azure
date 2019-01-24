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

package e2e

import (
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/clientcmd"
	"sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"

	clientv1alpha1 "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
)

// KubeClient provides the interfaces to interact with the kubernetes clusters.
type KubeClient struct {
	Kube            *kubernetes.Clientset
	ClusterV1Client clientv1alpha1.ClusterV1alpha1Interface
	//machInterface   clientv1alpha1.MachineInterface
}

// NewKubeClient returns a new instance of the KubeClient object.
func NewKubeClient(kubeconfig string) (*KubeClient, error) {
	kubeClientSet, err := clientcmd.NewCoreClientSetForDefaultSearchPath(kubeconfig, clientcmd.NewConfigOverrides())
	if err != nil {
		return nil, fmt.Errorf("error creating core clientset: %v", err)
	}
	clusterapiClientset, err := clientcmd.NewClusterApiClientForDefaultSearchPath(kubeconfig, clientcmd.NewConfigOverrides())
	if err != nil {
		return nil, fmt.Errorf("error creating rest config: %v", err)
	}
	return &KubeClient{
		Kube:            kubeClientSet,
		ClusterV1Client: clusterapiClientset.ClusterV1alpha1(),
	}, nil
}

// GetPod retrieves a pod resource.
func (kc *KubeClient) GetPod(namespace string, name string) (*v1.Pod, error) {
	pod, err := kc.Kube.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
	return pod, err
}

// GetNode retrieves a node resource.
func (kc *KubeClient) GetNode(name string) (*v1.Node, error) {
	pod, err := kc.Kube.CoreV1().Nodes().Get(name, metav1.GetOptions{})
	return pod, err
}

// GetCluster retrieved a custom Cluster resource.
func (kc *KubeClient) GetCluster(namespace string, name string) (*v1alpha1.Cluster, error) {
	cluster, err := kc.ClusterV1Client.Clusters(namespace).Get(name, metav1.GetOptions{})
	return cluster, err
}

// GetMachine retrieves a custom Machine resource.
func (kc *KubeClient) GetMachine(namespace string, name string, options metav1.GetOptions) (*v1alpha1.Machine, error) {
	machine, err := kc.ClusterV1Client.Machines(namespace).Get(name, options)
	return machine, err
}

// ListMachine lists the custom Machine resources.
func (kc *KubeClient) ListMachine(namespace string, options metav1.ListOptions) (*v1alpha1.MachineList, error) {
	machine, err := kc.ClusterV1Client.Machines(namespace).List(options)
	return machine, err
}

package e2e

import (
	"fmt"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/clientcmd"

	clientv1alpha1 "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
)

type KubeClient struct {
	Kube            *kubernetes.Clientset
	ClusterV1Client clientv1alpha1.ClusterV1alpha1Interface
	machInterface   clientv1alpha1.MachineInterface
}

func NewKubeClient(kubeconfig string) (*KubeClient, error) {
	kubeClientSet, clusterapiClientset, err := clientcmd.NewClientsForDefaultSearchpath(kubeconfig, clientcmd.NewConfigOverrides())
	if err != nil {
		return nil, fmt.Errorf("error creating rest config: %v", err)
	}
	return &KubeClient{
		Kube:            kubeClientSet,
		ClusterV1Client: clusterapiClientset.ClusterV1alpha1(),
	}, nil
}

func (kc *KubeClient) GetPod(namespace string, name string) (*v1.Pod, error) {
	pod, err := kc.Kube.CoreV1().Pods(namespace).Get(name, metav1.GetOptions{})
	return pod, err
}

func (kc *KubeClient) GetNode(name string) (*v1.Node, error) {
	pod, err := kc.Kube.CoreV1().Nodes().Get(name, metav1.GetOptions{})
	return pod, err
}

func (kc *KubeClient) GetCluster(namespace string, name string) (*v1alpha1.Cluster, error) {
	cluster, err := kc.ClusterV1Client.Clusters(namespace).Get(name, metav1.GetOptions{})
	return cluster, err
}

func (kc *KubeClient) GetMachine(namespace string, name string, options metav1.GetOptions) (*v1alpha1.Machine, error) {
	machine, err := kc.ClusterV1Client.Machines(namespace).Get(name, options)
	return machine, err
}

func (kc *KubeClient) ListMachine(namespace string, options metav1.ListOptions) (*v1alpha1.MachineList, error) {
	machine, err := kc.ClusterV1Client.Machines(namespace).List(options)
	return machine, err
}

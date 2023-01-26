//go:build e2e
// +build e2e

/*
Copyright 2020 The Kubernetes Authors.

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

package networkpolicy

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"
	e2e_pod "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/pod"
)

const (
	networkPolicyOperationTimeout             = 30 * time.Second
	networkPolicyOperationSleepBetweenRetries = 3 * time.Second
)

// CreateNetworkPolicyFromFile will create a NetworkPolicy from file with a name
func CreateNetworkPolicyFromFile(ctx context.Context, clientset *kubernetes.Clientset, filename, namespace string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode

	obj, _, err := decode(data, nil, nil)
	if err != nil {
		return err
	}

	switch o := obj.(type) {
	case *networkingv1.NetworkPolicy:
		return createNetworkPolicyV1(ctx, clientset, namespace, obj.(*networkingv1.NetworkPolicy))
	default:
		return fmt.Errorf("unsupported k8s manifest type %T", o)
	}
}

func createNetworkPolicyV1(ctx context.Context, clientset *kubernetes.Clientset, namespace string, networkPolicy *networkingv1.NetworkPolicy) error {
	Eventually(func(g Gomega) {
		_, err := clientset.NetworkingV1().NetworkPolicies(namespace).Create(ctx, networkPolicy, metav1.CreateOptions{})
		if err != nil {
			log.Printf("failed trying to create NetworkPolicy (%s):%s\n", networkPolicy.Name, err.Error())
		}
		g.Expect(err).NotTo(HaveOccurred())
	}, networkPolicyOperationTimeout, networkPolicyOperationSleepBetweenRetries).Should(Succeed())
	return nil
}

// DeleteNetworkPolicy will create a NetworkPolicy from file with a name
func DeleteNetworkPolicy(ctx context.Context, clientset *kubernetes.Clientset, name, namespace string) {
	opts := metav1.DeleteOptions{}
	Eventually(func(g Gomega) {
		err := clientset.NetworkingV1().NetworkPolicies(namespace).Delete(ctx, name, opts)
		if err != nil {
			log.Printf("failed trying to delete NetworkPolicy (%s):%s\n", name, err.Error())
		}
		g.Expect(err).NotTo(HaveOccurred())
	}, networkPolicyOperationTimeout, networkPolicyOperationSleepBetweenRetries).Should(Succeed())
}

func EnsureOutboundInternetAccess(clientset *kubernetes.Clientset, config *restclient.Config, pods []corev1.Pod) {
	for _, pod := range pods {
		CheckOutboundConnection(clientset, config, pod)
	}
}

func EnsureConnectivityResultBetweenPods(clientset *kubernetes.Clientset, config *restclient.Config, fromPods []corev1.Pod, toPods []corev1.Pod, shouldHaveConnection bool) {
	for _, fromPod := range fromPods {
		for _, toPod := range toPods {
			command := []string{"curl", "-S", "-s", "-o", "/dev/null", toPod.Status.PodIP}
			err := e2e_pod.Exec(clientset, config, fromPod, command, shouldHaveConnection)
			Expect(err).NotTo(HaveOccurred())
		}
	}
}

func CheckOutboundConnection(clientset *kubernetes.Clientset, config *restclient.Config, pod corev1.Pod) {
	command := []string{"curl", "-S", "-s", "-o", "/dev/null", "www.bing.com"}
	err := e2e_pod.Exec(clientset, config, pod, command, true)
	Expect(err).NotTo(HaveOccurred())
}

func ApplyNetworkPolicy(ctx context.Context, clientset *kubernetes.Clientset, nwpolicyName string, namespace string, nwpolicyFileName string, policyDir string) {
	err := CreateNetworkPolicyFromFile(ctx, clientset, filepath.Join(policyDir, nwpolicyFileName), namespace)
	Expect(err).NotTo(HaveOccurred())
}

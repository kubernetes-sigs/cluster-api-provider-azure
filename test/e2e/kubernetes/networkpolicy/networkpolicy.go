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
	"fmt"
	"io/ioutil"
	"path/filepath"

	. "github.com/onsi/gomega"

	e2e_pod "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/pod"

	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"
)

// CreateNetworkPolicyFromFile will create a NetworkPolicy from file with a name
func CreateNetworkPolicyFromFile(clientset *kubernetes.Clientset, filename, namespace string) error {
	data, err := ioutil.ReadFile(filename)
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
		return createNetworkPolicyV1(clientset, namespace, obj.(*networkingv1.NetworkPolicy))
	default:
		return fmt.Errorf("Error: unsupported k8s manifest type %T", o)
	}

	return nil
}

func createNetworkPolicyV1(clientset *kubernetes.Clientset, namespace string, networkPolicy *networkingv1.NetworkPolicy) error {
	_, err := clientset.NetworkingV1().NetworkPolicies(namespace).Create(networkPolicy)
	return err
}

// DeleteNetworkPolicy will create a NetworkPolicy from file with a name
func DeleteNetworkPolicy(clientset *kubernetes.Clientset, name, namespace string) {
	opts := &metav1.DeleteOptions{}
	err := clientset.NetworkingV1().NetworkPolicies(namespace).Delete(name, opts)
	Expect(err).NotTo(HaveOccurred())
}

func EnsureOutboundInternetAccess(clientset *kubernetes.Clientset, config *restclient.Config, pods []v1.Pod) {
	for _, pod := range pods {
		err := CheckOutboundConnection(clientset, config, pod)
		Expect(err).NotTo(HaveOccurred())
	}
}

func EnsureConnectivityResultBetweenPods(clientset *kubernetes.Clientset, config *restclient.Config, fromPods []v1.Pod, toPods []v1.Pod, shouldHaveConnection bool) {
	for _, fromPod := range fromPods {
		for _, toPod := range toPods {
			command := []string{"curl", "-S", "-s", "-o", "/dev/null", toPod.Status.PodIP}
			err := e2e_pod.Exec(clientset, config, fromPod, command)
			if shouldHaveConnection {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).Should(HaveOccurred())
			}
		}
	}
}

func CheckOutboundConnection(clientset *kubernetes.Clientset, config *restclient.Config, pod v1.Pod) error {
	command := []string{"curl", "-S", "-s", "-o", "/dev/null", "www.bing.com"}
	return e2e_pod.Exec(clientset, config, pod, command)
}

func ApplyNetworkPolicy(clientset *kubernetes.Clientset, nwpolicyName string, namespace string, nwpolicyFileName string, policyDir string) {
	err := CreateNetworkPolicyFromFile(clientset, filepath.Join(policyDir, nwpolicyFileName), namespace)
	Expect(err).NotTo(HaveOccurred())
}

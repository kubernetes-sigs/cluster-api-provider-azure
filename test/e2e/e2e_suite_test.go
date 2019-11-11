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

package e2e_test

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	common "sigs.k8s.io/cluster-api/test/helpers/components"
	capiFlag "sigs.k8s.io/cluster-api/test/helpers/flag"
	"sigs.k8s.io/cluster-api/test/helpers/kind"
	"sigs.k8s.io/cluster-api/test/helpers/scheme"
	"sigs.k8s.io/cluster-api/util"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	junitPath := fmt.Sprintf("junit.e2e_suite.%d.xml", config.GinkgoConfig.ParallelNode)
	junitReporter := reporters.NewJUnitReporter(junitPath)
	RunSpecsWithDefaultAndCustomReporters(t, "e2e Suite", []Reporter{junitReporter})
}

const (
	capiNamespace       = "capi-system"
	capiDeploymentName  = "capi-controller-manager"
	capzNamespace       = "capz-system"
	capzDeploymentName  = "capz-controller-manager"
	cabpkNamespace      = "cabpk-system"
	cabpkDeploymentName = "cabpk-controller-manager"
	setupTimeout        = 10 * 60
	//capa
	// stackName   = "cluster-api-provider-aws-sigs-k8s-io"
	// keyPairName = "cluster-api-provider-aws-sigs-k8s-io"
	//capz
	// capzProviderNamespace = "azure-provider-system"
	// capzStatefulSetName   = "azure-provider-controller-manager"
)

var (
	providerComponentsYAML = capiFlag.DefineOrLookupStringFlag("providerComponentsYAML", "../../examples/_out/provider-components.yaml", "TODO")

	kindCluster kind.Cluster
	kindClient  crclient.Client
	clientSet   *kubernetes.Clientset
)

var _ = BeforeSuite(func() {
	fmt.Fprintf(GinkgoWriter, "Setting up kind cluster\n")
	kindCluster = kind.Cluster{
		Name: "capz-test-" + util.RandomString(6),
	}
	kindCluster.Setup()

	restConfig := kindCluster.RestConfig()
	mapper, err := apiutil.NewDynamicRESTMapper(restConfig, apiutil.WithLazyDiscovery)
	Expect(err).NotTo(HaveOccurred())
	kindClient, err = crclient.New(kindCluster.RestConfig(), crclient.Options{Scheme: setupScheme(), Mapper: mapper})
	Expect(err).NotTo(HaveOccurred())
	clientSet, err = kubernetes.NewForConfig(kindCluster.RestConfig())
	Expect(err).NotTo(HaveOccurred())

	applyManifests(kindCluster, providerComponentsYAML)

	common.WaitDeployment(kindClient, capiNamespace, capiDeploymentName)
	go func() {
		defer GinkgoRecover()
		watchLogs(capiNamespace, capiDeploymentName, "examples")
	}()

	common.WaitDeployment(kindClient, capzNamespace, capzDeploymentName)
	go func() {
		defer GinkgoRecover()
		watchLogs(capzNamespace, capzDeploymentName, "examples")
	}()

	common.WaitDeployment(kindClient, cabpkNamespace, cabpkDeploymentName)
	go func() {
		defer GinkgoRecover()
		watchLogs(cabpkNamespace, cabpkDeploymentName, "examples")
	}()

}, setupTimeout)

var _ = AfterSuite(func() {
	fmt.Fprintf(GinkgoWriter, "Tearing down kind cluster\n")
	kindCluster.Teardown()
})

func applyManifests(kindCluster kind.Cluster, manifests *string) {
	Expect(manifests).ToNot(BeNil())
	fmt.Fprintf(GinkgoWriter, "Applying manifests for %s\n", *manifests)
	Expect(*manifests).ToNot(BeEmpty())
	kindCluster.ApplyYAML(*manifests)
}

func setupScheme() *runtime.Scheme {
	s := scheme.SetupScheme()
	Expect(clusterv1.AddToScheme(s)).To(Succeed())
	Expect(infrav1.AddToScheme(s)).To(Succeed())
	return s
}

func watchLogs(namespace, deploymentName, logDir string) {
	deployment := &appsv1.Deployment{}
	Expect(kindClient.Get(context.TODO(), crclient.ObjectKey{Namespace: namespace, Name: deploymentName}, deployment)).To(Succeed())

	selector, err := metav1.LabelSelectorAsMap(deployment.Spec.Selector)
	Expect(err).NotTo(HaveOccurred())

	pods := &corev1.PodList{}
	Expect(kindClient.List(context.TODO(), pods, crclient.InNamespace(namespace), crclient.MatchingLabels(selector))).To(Succeed())

	for _, pod := range pods.Items {
		for _, container := range deployment.Spec.Template.Spec.Containers {
			logFile := path.Join(logDir, deploymentName, pod.Name, container.Name+".log")
			fmt.Fprintf(GinkgoWriter, "Creating directory: %s\n", filepath.Dir(logFile))
			Expect(os.MkdirAll(filepath.Dir(logFile), 0755)).To(Succeed())

			opts := &corev1.PodLogOptions{
				Container: container.Name,
				Follow:    true,
			}

			podLogs, err := clientSet.CoreV1().Pods(namespace).GetLogs(pod.Name, opts).Stream()
			Expect(err).NotTo(HaveOccurred())
			defer podLogs.Close()

			f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			Expect(err).NotTo(HaveOccurred())
			defer f.Close()

			out := bufio.NewWriter(f)
			defer out.Flush()
			_, err = out.ReadFrom(podLogs)
			if err != nil && err.Error() != "unexpected EOF" {
				Expect(err).NotTo(HaveOccurred())
			}
		}
	}
}

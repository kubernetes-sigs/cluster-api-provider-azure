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
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"

	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/auth"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/generators"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/utils"
	capiv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	"sigs.k8s.io/cluster-api/test/framework"
	frameworkgenerator "sigs.k8s.io/cluster-api/test/framework/generators"
	"sigs.k8s.io/cluster-api/test/framework/management/kind"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestE2E(t *testing.T) {
	artifactPath, _ := os.LookupEnv("ARTIFACTS")
	junitXML := fmt.Sprintf("junit.e2e_suite.%d.xml", config.GinkgoConfig.ParallelNode)
	junitPath := path.Join(artifactPath, junitXML)
	junitReporter := reporters.NewJUnitReporter(junitPath)

	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "capz e2e suite", []Reporter{junitReporter})
}

var (
	ctx     = context.Background()
	creds   auth.Creds
	mgmt    *kind.Cluster
	logPath string
)

var _ = BeforeSuite(func() {
	var err error

	By("creating the logs directory")
	artifactPath := os.Getenv("ARTIFACTS")
	logPath = path.Join(artifactPath, "logs")
	Expect(os.MkdirAll(filepath.Dir(logPath), 0755)).To(Succeed())

	By("Loading Azure credentials")
	if credsFile, found := os.LookupEnv("AZURE_CREDENTIALS"); found {
		creds, err = auth.LoadFromFile(credsFile)
	} else {
		creds, err = auth.LoadFromEnvironment()
	}
	Expect(err).NotTo(HaveOccurred())
	Expect(creds).NotTo(BeNil())
	Expect(creds.TenantID).NotTo(BeEmpty())
	Expect(creds.SubscriptionID).NotTo(BeEmpty())
	Expect(creds.ClientID).NotTo(BeEmpty())
	Expect(creds.ClientSecret).NotTo(BeEmpty())

	By("Creating management cluster")
	scheme := runtime.NewScheme()
	Expect(appsv1.AddToScheme(scheme)).To(Succeed())
	Expect(corev1.AddToScheme(scheme)).To(Succeed())
	Expect(capiv1.AddToScheme(scheme)).To(Succeed())
	Expect(bootstrapv1.AddToScheme(scheme)).To(Succeed())
	Expect(infrav1.AddToScheme(scheme)).To(Succeed())
	Expect(controlplanev1.AddToScheme(scheme)).To(Succeed())

	managerImage, found := os.LookupEnv("MANAGER_IMAGE")
	Expect(found).To(BeTrue(), fmt.Sprint("MANAGER_IMAGE not set"))

	mgmt, err = kind.NewCluster(ctx, "mgmt", scheme, managerImage)
	Expect(err).NotTo(HaveOccurred())
	Expect(mgmt).NotTo(BeNil())

	// set up cert manager generator
	cm := &frameworkgenerator.CertManager{ReleaseVersion: "v0.11.0"}
	// install cert manager first
	framework.InstallComponents(ctx, mgmt, cm)
	framework.WaitForAPIServiceAvailable(ctx, mgmt, "v1beta1.webhook.cert-manager.io")

	// Wait for CertManager to be available before continuing
	c, err := mgmt.GetClient()
	Expect(err).NotTo(HaveOccurred())
	waitDeployment(c, "cert-manager", "cert-manager-webhook")

	// Deploy the CAPI and CABPK components from Cluster API repository,
	capi := &frameworkgenerator.ClusterAPI{Version: "v0.3.5"}
	cabpk := &frameworkgenerator.KubeadmBootstrap{Version: "v0.3.5"}
	kcp := &frameworkgenerator.KubeadmControlPlane{Version: "v0.3.5"}
	infra := &generators.Infra{Creds: creds}

	framework.InstallComponents(ctx, mgmt, capi, cabpk, kcp, infra)
	framework.WaitForPodsReadyInNamespace(ctx, mgmt, "capi-system")
	framework.WaitForPodsReadyInNamespace(ctx, mgmt, "capz-system")

	// go func() {
	// 	defer GinkgoRecover()
	// 	watchDeployment(mgmt, "cabpk-system", "cabpk-controller-manager")
	// }()
	// go func() {
	// 	defer GinkgoRecover()
	// 	watchDeployment(mgmt, "capz-system", "capz-controller-manager")
	// }()
})

var _ = AfterSuite(func() {
	// DO NOT stream "capi-controller-manager" logs as it prints out azure.json
	Expect(writeLogs(mgmt, "capi-kubeadm-bootstrap-system", "capi-kubeadm-bootstrap-controller-manager", logPath)).To(Succeed())
	Expect(writeLogs(mgmt, "capz-system", "capz-controller-manager", logPath)).To(Succeed())
	By("Tearing down management cluster")
	mgmt.Teardown(ctx)
})

func watchDeployment(mgmt *kind.Cluster, namespace, name string) {
	artifactPath, _ := os.LookupEnv("ARTIFACTS")
	logDir := path.Join(artifactPath, "logs")

	c, err := mgmt.GetClient()
	Expect(err).NotTo(HaveOccurred())

	waitDeployment(c, namespace, name)

	deployment := &appsv1.Deployment{}
	deploymentKey := client.ObjectKey{Namespace: namespace, Name: name}
	Expect(c.Get(ctx, deploymentKey, deployment)).To(Succeed())

	selector, err := metav1.LabelSelectorAsMap(deployment.Spec.Selector)
	Expect(err).NotTo(HaveOccurred())

	pods := &corev1.PodList{}
	Expect(c.List(ctx, pods, client.InNamespace(namespace), client.MatchingLabels(selector))).To(Succeed())

	for _, pod := range pods.Items {
		for _, container := range deployment.Spec.Template.Spec.Containers {
			if container.Name != "manager" {
				continue
			}
			logFile := path.Join(logDir, name, pod.Name, container.Name+".log")
			Expect(os.MkdirAll(filepath.Dir(logFile), 0755)).To(Succeed())

			clientSet, err := mgmt.GetClientSet()
			Expect(err).NotTo(HaveOccurred())

			opts := &corev1.PodLogOptions{Container: container.Name, Follow: true}
			logsStream, err := clientSet.CoreV1().Pods(namespace).GetLogs(pod.Name, opts).Stream()
			Expect(err).NotTo(HaveOccurred())
			defer logsStream.Close()

			f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			Expect(err).NotTo(HaveOccurred())
			defer f.Close()

			out := bufio.NewWriter(f)
			defer out.Flush()

			_, err = out.ReadFrom(logsStream)
			if err != nil && err.Error() != "unexpected EOF" {
				Expect(err).NotTo(HaveOccurred())
			}
		}
	}
}

func waitDeployment(c client.Client, namespace, name string) {
	Eventually(func() (int32, error) {
		deployment := &appsv1.Deployment{}
		deploymentKey := client.ObjectKey{Namespace: namespace, Name: name}
		if err := c.Get(context.TODO(), deploymentKey, deployment); err != nil {
			return 0, err
		}
		return deployment.Status.ReadyReplicas, nil
	}, 5*time.Minute, 15*time.Second,
		fmt.Sprintf("Deployment %s/%s could not reach the ready state", namespace, name),
	).ShouldNot(BeZero())
}

func writeLogs(mgmt *kind.Cluster, namespace, deploymentName, logDir string) error {
	c, err := mgmt.GetClient()
	if err != nil {
		return err
	}
	clientSet, err := mgmt.GetClientSet()
	if err != nil {
		return err
	}
	deployment := &appsv1.Deployment{}
	if err := c.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: deploymentName}, deployment); err != nil {
		return err
	}

	selector, err := metav1.LabelSelectorAsMap(deployment.Spec.Selector)
	if err != nil {
		return err
	}

	pods := &corev1.PodList{}
	if err := c.List(context.TODO(), pods, client.InNamespace(namespace), client.MatchingLabels(selector)); err != nil {
		return err
	}

	for _, pod := range pods.Items {
		for _, container := range deployment.Spec.Template.Spec.Containers {
			logFile := path.Join(logDir, deploymentName, pod.Name, container.Name+".log")
			fmt.Fprintf(GinkgoWriter, "Creating directory: %s\n", filepath.Dir(logFile))
			if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
				return errors.Wrapf(err, "error making logDir %q", filepath.Dir(logFile))
			}

			opts := &corev1.PodLogOptions{
				Container: container.Name,
				Follow:    false,
			}

			podLogs, err := clientSet.CoreV1().Pods(namespace).GetLogs(pod.Name, opts).Stream()
			if err != nil {
				return errors.Wrapf(err, "error getting pod stream for pod name %q/%q", pod.Namespace, pod.Name)
			}
			defer podLogs.Close()

			f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return errors.Wrapf(err, "error opening created logFile %q", logFile)
			}
			defer f.Close()

			logs, err := ioutil.ReadAll(podLogs)
			if err != nil {
				return errors.Wrapf(err, "failed to read podLogs %q/%q", pod.Namespace, pod.Name)
			}
			if err := ioutil.WriteFile(f.Name(), logs, 0644); err != nil {
				return errors.Wrapf(err, "error writing pod logFile %q", f.Name())
			}
		}
	}
	return nil
}

func ensureCAPZArtifactsDeleted(input *ControlPlaneClusterInput) {
	input.SetDefaults()
	groupName := input.Management.GetName()

	mgmtClient, err := input.Management.GetClient()
	Expect(err).NotTo(HaveOccurred(), "stack: %+v", err)

	By("Deleting cluster")
	ctx := context.Background()
	Expect(mgmtClient.Delete(ctx, input.Cluster)).NotTo(HaveOccurred())

	Eventually(func() []clusterv1.Cluster {
		clusters := clusterv1.ClusterList{}
		c, err := input.Management.GetClient()
		Expect(err).NotTo(HaveOccurred())
		Expect(c.List(ctx, &clusters)).NotTo(HaveOccurred())
		return clusters.Items
	}, input.DeleteTimeout, 20*time.Second).Should(HaveLen(0))

	By("Making sure there are all Azure resources are deleted")
	_, err = utils.GetGroup(ctx, creds, groupName)
	if err == nil {
		log.Printf("resource group %s still exist, cleaning\n", groupName)
		err = utils.CleanupE2EResources(ctx, creds, groupName)
		if err != nil {
			log.Printf("failed to delete the group %s: %s\n", groupName, err.Error())
			return
		}
	}
}

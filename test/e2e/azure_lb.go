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

package e2e

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	k8snet "k8s.io/utils/net"
	deploymentBuilder "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/deployment"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/job"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/node"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/windows"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util"
)

// AzureLBSpecInput is the input for AzureLBSpec.
type AzureLBSpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	SkipCleanup           bool
	Windows               bool
	IPFamilies            []corev1.IPFamily
}

// AzureLBSpec implements a test that verifies Azure internal and external load balancers can
// be created and work properly through a Kubernetes service.
func AzureLBSpec(ctx context.Context, inputGetter func() AzureLBSpecInput) {
	var (
		specName     = "azure-lb"
		input        AzureLBSpecInput
		clusterProxy framework.ClusterProxy
		clientset    *kubernetes.Clientset
	)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	By("creating a Kubernetes client to the workload cluster")
	clusterProxy = input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
	Expect(clusterProxy).NotTo(BeNil())
	clientset = clusterProxy.GetClientSet()
	Expect(clientset).NotTo(BeNil())

	By("creating an HTTP deployment")
	deploymentName := "web" + util.RandomString(6)
	// if case of input.SkipCleanup we need a unique name for windows
	if input.Windows {
		deploymentName = "web-windows" + util.RandomString(6)
	}

	webDeployment := deploymentBuilder.Create("httpd", deploymentName, corev1.NamespaceDefault, int32(1))
	webDeployment.AddContainerPort("http", "http", 80, corev1.ProtocolTCP)

	if input.Windows {
		var windowsVersion windows.OSVersion
		Eventually(func(g Gomega) {
			var err error
			windowsVersion, err = node.GetWindowsVersion(ctx, clientset)
			g.Expect(err).NotTo(HaveOccurred())
		}, 300*time.Second, 5*time.Second).Should(Succeed())
		iisImage := windows.GetWindowsImage(windows.Httpd, windowsVersion)
		webDeployment.SetImage(deploymentName, iisImage)
		webDeployment.AddWindowsSelectors()
	}

	deployment, err := webDeployment.Deploy(ctx, clientset)
	Expect(err).NotTo(HaveOccurred())
	deployInput := WaitForDeploymentsAvailableInput{
		Getter:     deploymentsClientAdapter{client: webDeployment.Client(clientset)},
		Deployment: deployment,
		Clientset:  clientset,
	}
	WaitForDeploymentsAvailable(ctx, deployInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)

	servicesClient := clientset.CoreV1().Services(corev1.NamespaceDefault)
	jobsClient := clientset.BatchV1().Jobs(corev1.NamespaceDefault)

	ports := []corev1.ServicePort{
		{
			Name:     "http",
			Port:     80,
			Protocol: corev1.ProtocolTCP,
		},
		{
			Name:     "https",
			Port:     443,
			Protocol: corev1.ProtocolTCP,
		},
	}

	By("creating an internal Load Balancer service")

	ilbService := webDeployment.CreateServiceResourceSpec(ports, deploymentBuilder.InternalLoadbalancer, input.IPFamilies)
	Log("starting to create an internal Load Balancer service")
	Eventually(func(g Gomega) {
		_, err := servicesClient.Create(ctx, ilbService, metav1.CreateOptions{})
		if err != nil {
			LogWarningf("failed creating service (%s):%s\n", ilbService.Name, err.Error())
		}
		g.Expect(err).NotTo(HaveOccurred())
	}, retryableOperationTimeout, retryableOperationSleepBetweenRetries).Should(Succeed())
	ilbSvcInput := WaitForServiceAvailableInput{
		Getter:    servicesClientAdapter{client: servicesClient},
		Service:   ilbService,
		Clientset: clientset,
	}
	WaitForServiceAvailable(ctx, ilbSvcInput, e2eConfig.GetIntervals(specName, "wait-service")...)

	By("connecting to the internal LB service from a curl pod")

	var svc *corev1.Service
	Eventually(func(g Gomega) {
		var err error
		svc, err = servicesClient.Get(ctx, ilbService.Name, metav1.GetOptions{})
		if err != nil {
			LogWarningf("failed getting service (%s):%s\n", ilbService.Name, err.Error())
		}
		g.Expect(err).NotTo(HaveOccurred())
	}, retryableOperationTimeout, retryableOperationSleepBetweenRetries).Should(Succeed())
	ilbIP := extractServiceIP(svc)

	ilbJob := job.CreateCurlJobResourceSpec("curl-to-ilb-job", ilbIP)
	Log("starting to create a curl to ilb job")
	Eventually(func(g Gomega) {
		_, err := jobsClient.Create(ctx, ilbJob, metav1.CreateOptions{})
		if err != nil {
			LogWarningf("failed creating job (%s):%s\n", ilbJob.Name, err.Error())
		}
		g.Expect(err).NotTo(HaveOccurred())
	}, retryableOperationTimeout, retryableOperationSleepBetweenRetries).Should(Succeed())
	ilbJobInput := WaitForJobCompleteInput{
		Getter:    jobsClientAdapter{client: jobsClient},
		Job:       ilbJob,
		Clientset: clientset,
	}
	WaitForJobComplete(ctx, ilbJobInput, e2eConfig.GetIntervals(specName, "wait-job")...)

	if !input.SkipCleanup {
		By("deleting the ilb test resources")
		Logf("starting to delete the ilb service: %s", ilbService.Name)
		Eventually(func(g Gomega) {
			err := servicesClient.Delete(ctx, ilbService.Name, metav1.DeleteOptions{})
			if err != nil {
				LogWarningf("failed deleting service (%s):%s\n", ilbService.Name, err.Error())
			}
			g.Expect(err).NotTo(HaveOccurred())
		}, retryableDeleteOperationTimeout, retryableOperationSleepBetweenRetries).Should(Succeed())
		Logf("waiting for the ilb service to be deleted: %s", ilbService.Name)
		Eventually(func(g Gomega) {
			_, err := servicesClient.Get(ctx, ilbService.GetName(), metav1.GetOptions{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, deleteOperationTimeout, retryableOperationSleepBetweenRetries).Should(Succeed())
		Logf("deleting the ilb job: %s", ilbJob.Name)
		Eventually(func(g Gomega) {
			err := jobsClient.Delete(ctx, ilbJob.Name, metav1.DeleteOptions{})
			if err != nil {
				LogWarningf("failed deleting job (%s):%s\n", ilbJob.Name, err.Error())
			}
			g.Expect(err).NotTo(HaveOccurred())
		}, deleteOperationTimeout, retryableOperationSleepBetweenRetries).Should(Succeed())
	}

	By("creating an external Load Balancer service")
	elbService := webDeployment.CreateServiceResourceSpec(ports, deploymentBuilder.ExternalLoadbalancer, input.IPFamilies)
	Log("starting to create an external Load Balancer service")
	Eventually(func(g Gomega) {
		_, err := servicesClient.Create(ctx, elbService, metav1.CreateOptions{})
		if err != nil {
			LogWarningf("failed creating service (%s):%s\n", elbService.Name, err.Error())
		}
		g.Expect(err).NotTo(HaveOccurred())
	}, retryableOperationTimeout, retryableOperationSleepBetweenRetries).Should(Succeed())
	elbSvcInput := WaitForServiceAvailableInput{
		Getter:    servicesClientAdapter{client: servicesClient},
		Service:   elbService,
		Clientset: clientset,
	}
	WaitForServiceAvailable(ctx, elbSvcInput, e2eConfig.GetIntervals(specName, "wait-service")...)

	By("connecting to the external LB service from a curl pod")
	Eventually(func(g Gomega) {
		var err error
		svc, err = servicesClient.Get(ctx, elbService.Name, metav1.GetOptions{})
		if err != nil {
			LogWarningf("failed getting service (%s):%s\n", elbService.Name, err.Error())
		}
		g.Expect(err).NotTo(HaveOccurred())
	}, retryableOperationTimeout, retryableOperationSleepBetweenRetries).Should(Succeed())

	elbIP := extractServiceIP(svc)
	Log("starting to create curl-to-elb job")
	elbJob := job.CreateCurlJobResourceSpec("curl-to-elb-job"+util.RandomString(6), elbIP)
	Eventually(func(g Gomega) {
		_, err := jobsClient.Create(ctx, elbJob, metav1.CreateOptions{})
		if err != nil {
			LogWarningf("failed creating job (%s):%s\n", elbJob.Name, err.Error())
		}
		g.Expect(err).NotTo(HaveOccurred())
	}, retryableOperationTimeout, retryableOperationSleepBetweenRetries).Should(Succeed())
	elbJobInput := WaitForJobCompleteInput{
		Getter:    jobsClientAdapter{client: jobsClient},
		Job:       elbJob,
		Clientset: clientset,
	}
	WaitForJobComplete(ctx, elbJobInput, e2eConfig.GetIntervals(specName, "wait-job")...)

	// connecting directly to the external LB service only works for IPv4
	// for IPv6 this is only possible when externalTrafficPolicy is set to "Local"
	if k8snet.IsIPv4String(elbIP) {
		By("connecting directly to the external LB service")
		url := fmt.Sprintf("http://%s", elbIP)
		Log("starting attempts to connect directly to the external LB service")
		resp, err := retryablehttp.Get(url)
		if resp != nil {
			defer resp.Body.Close()
		}
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(200))
		Log("successfully connected to the external LB service")
	}

	if input.SkipCleanup {
		return
	}
	By("deleting the test resources")
	Logf("starting to delete external LB service %s", elbService.Name)
	Eventually(func(g Gomega) {
		err := servicesClient.Delete(ctx, elbService.Name, metav1.DeleteOptions{})
		if err != nil {
			LogWarningf("failed deleting service (%s):%s\n", elbService.Name, err.Error())
		}
		g.Expect(err).NotTo(HaveOccurred())
	}, retryableDeleteOperationTimeout, retryableOperationSleepBetweenRetries).Should(Succeed())
	Logf("waiting for the external LB service to be deleted: %s", elbService.Name)
	Eventually(func(g Gomega) {
		_, err := servicesClient.Get(ctx, elbService.GetName(), metav1.GetOptions{})
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	}, deleteOperationTimeout, retryableOperationSleepBetweenRetries).Should(Succeed())
	Logf("starting to delete deployment %s", deployment.Name)
	Eventually(func(g Gomega) {
		err := webDeployment.Client(clientset).Delete(ctx, deployment.Name, metav1.DeleteOptions{})
		if err != nil {
			LogWarningf("failed deleting deployment (%s):%s\n", deployment.Name, err.Error())
		}
		g.Expect(err).NotTo(HaveOccurred())
	}, deleteOperationTimeout, retryableOperationSleepBetweenRetries).Should(Succeed())
	Logf("starting to delete job %s", elbJob.Name)
	Eventually(func(g Gomega) {
		err := jobsClient.Delete(ctx, elbJob.Name, metav1.DeleteOptions{})
		if err != nil {
			LogWarningf("failed deleting job (%s):%s\n", elbJob.Name, err.Error())
		}
		g.Expect(err).NotTo(HaveOccurred())
	}, deleteOperationTimeout, retryableOperationSleepBetweenRetries).Should(Succeed())
}

func extractServiceIP(svc *corev1.Service) string {
	var ilbIP string
	for _, i := range svc.Status.LoadBalancer.Ingress {
		if net.ParseIP(i.IP) != nil {
			if k8snet.IsIPv6String(i.IP) {
				ilbIP = fmt.Sprintf("[%s]", i.IP)
				break
			}
			ilbIP = i.IP
			break
		}
	}

	return ilbIP
}

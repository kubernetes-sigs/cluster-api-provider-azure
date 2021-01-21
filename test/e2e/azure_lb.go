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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	k8snet "k8s.io/utils/net"

	"github.com/hashicorp/go-retryablehttp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cluster-api/test/framework"

	deploymentBuilder "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/deployment"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/job"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/node"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/windows"
)

// AzureLBSpecInput is the input for AzureLBSpec.
type AzureLBSpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	SkipCleanup           bool
	IPv6                  bool
	Windows               bool
	IsVMSS                bool
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
	clusterProxy = input.BootstrapClusterProxy.GetWorkloadCluster(context.TODO(), input.Namespace.Name, input.ClusterName)
	Expect(clusterProxy).NotTo(BeNil())
	clientset = clusterProxy.GetClientSet()
	Expect(clientset).NotTo(BeNil())

	By("creating an HTTP deployment")
	deploymentName := "web"
	// if case of input.SkipCleanup we need a unique name for windows
	if input.Windows {
		deploymentName = "web-windows"
	}

	webDeployment := deploymentBuilder.CreateDeployment("httpd", deploymentName, corev1.NamespaceDefault)
	webDeployment.AddContainerPort("http", "http", 80, corev1.ProtocolTCP)

	if input.Windows {
		windowsVersion, err := node.GetWindowsVersion(clientset)
		Expect(err).NotTo(HaveOccurred())
		iisImage := windows.GetWindowsImage(windows.Httpd, windowsVersion)
		webDeployment.SetImage(deploymentName, iisImage)
		webDeployment.AddWindowsSelectors()
	}

	deployment, err := webDeployment.Deploy(clientset)
	Expect(err).NotTo(HaveOccurred())
	deployInput := WaitForDeploymentsAvailableInput{
		Getter:     deploymentsClientAdapter{client: webDeployment.Client(clientset)},
		Deployment: deployment,
		Clientset:  clientset,
	}
	WaitForDeploymentsAvailable(context.TODO(), deployInput, e2eConfig.GetIntervals(specName, "wait-deployment")...)

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

	// TODO: fix and enable this. Internal LBs + IPv6 is currently in preview.
	// https://docs.microsoft.com/en-us/azure/virtual-network/ipv6-dual-stack-standard-internal-load-balancer-powershell
	//
	// TODO: fix and enable this for VMSS after NRP / CRP sync bug is resolved
	if !input.IPv6 && !input.IsVMSS {
		By("creating an internal Load Balancer service")

		ilbService := webDeployment.GetService(ports, deploymentBuilder.InternalLoadbalancer)
		_, err = servicesClient.Create(ilbService)
		Expect(err).NotTo(HaveOccurred())
		ilbSvcInput := WaitForServiceAvailableInput{
			Getter:    servicesClientAdapter{client: servicesClient},
			Service:   ilbService,
			Clientset: clientset,
		}
		WaitForServiceAvailable(context.TODO(), ilbSvcInput, e2eConfig.GetIntervals(specName, "wait-service")...)

		By("connecting to the internal LB service from a curl pod")

		svc, err := servicesClient.Get(ilbService.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		ilbIP := extractServiceIp(svc)

		ilbJob := job.CreateCurlJob("curl-to-ilb-job", ilbIP)
		_, err = jobsClient.Create(ilbJob)
		Expect(err).NotTo(HaveOccurred())
		ilbJobInput := WaitForJobCompleteInput{
			Getter:    jobsClientAdapter{client: jobsClient},
			Job:       ilbJob,
			Clientset: clientset,
		}
		WaitForJobComplete(context.TODO(), ilbJobInput, e2eConfig.GetIntervals(specName, "wait-job")...)

		if !input.SkipCleanup {
			By("deleting the ilb test resources")
			err = servicesClient.Delete(ilbService.Name, &metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
			err = jobsClient.Delete(ilbJob.Name, &metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		}
	}

	By("creating an external Load Balancer service")
	elbService := webDeployment.GetService(ports, deploymentBuilder.ExternalLoadbalancer)
	_, err = servicesClient.Create(elbService)
	Expect(err).NotTo(HaveOccurred())
	elbSvcInput := WaitForServiceAvailableInput{
		Getter:    servicesClientAdapter{client: servicesClient},
		Service:   elbService,
		Clientset: clientset,
	}
	WaitForServiceAvailable(context.TODO(), elbSvcInput, e2eConfig.GetIntervals(specName, "wait-service")...)

	By("connecting to the external LB service from a curl pod")
	svc, err := servicesClient.Get(elbService.Name, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	elbIP := extractServiceIp(svc)
	elbJob := job.CreateCurlJob("curl-to-elb-job", elbIP)
	_, err = jobsClient.Create(elbJob)
	Expect(err).NotTo(HaveOccurred())
	elbJobInput := WaitForJobCompleteInput{
		Getter:    jobsClientAdapter{client: jobsClient},
		Job:       elbJob,
		Clientset: clientset,
	}
	WaitForJobComplete(context.TODO(), elbJobInput, e2eConfig.GetIntervals(specName, "wait-job")...)

	// TODO: determine root issue of failures of addressing the ELB from prow and fix
	// see https://kubernetes.slack.com/archives/CEX9HENG7/p1610547551019900

	if !input.IPv6 {
		By("connecting directly to the external LB service")
		url := fmt.Sprintf("http://%s", elbIP)
		resp, err := retryablehttp.Get(url)
		if resp != nil {
			defer resp.Body.Close()
		}
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(200))
		Expect(err).NotTo(HaveOccurred())
	}

	if input.SkipCleanup {
		return
	}
	By("deleting the test resources")
	err = servicesClient.Delete(elbService.Name, &metav1.DeleteOptions{})
	Expect(err).NotTo(HaveOccurred())
	err = webDeployment.Client(clientset).Delete(deployment.Name, &metav1.DeleteOptions{})
	Expect(err).NotTo(HaveOccurred())
	err = jobsClient.Delete(elbJob.Name, &metav1.DeleteOptions{})
	Expect(err).NotTo(HaveOccurred())
}

func extractServiceIp(svc *corev1.Service) string {
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

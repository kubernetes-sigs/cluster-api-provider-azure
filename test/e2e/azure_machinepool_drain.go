// +build e2e

/*
Copyright 2021 The Kubernetes Authors.

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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
	deployments "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/deployment"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/node"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/windows"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	clusterv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha4"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AzureMachinePoolDrainSpecName = "azure-mp-drain"
)

// AzureMachinePoolDrainSpecInput is the input for AzureMachinePoolDrainSpec.
type (
	AzureMachinePoolDrainSpecInput struct {
		BootstrapClusterProxy framework.ClusterProxy
		Namespace             *corev1.Namespace
		ClusterName           string
		SkipCleanup           bool
		IPv6                  bool
	}

	deployCustomizerOption func(builder *deployments.Builder, service *corev1.Service)
)

// AzureMachinePoolDrainSpec implements a test that verifies Azure AzureMachinePool cordon and drain by creating a load
// balanced service in a MachinePool with 1+ nodes, verifies the workload is running on each of the nodes, then reduces
// the replica count -1 watching to ensure the workload is gracefully terminated and migrated to another node in the
// machine pool prior to deleting the Azure infrastructure.
func AzureMachinePoolDrainSpec(ctx context.Context, inputGetter func() AzureMachinePoolDrainSpecInput) {
	input := inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", AzureMachinePoolDrainSpecName)
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", AzureMachinePoolDrainSpecName)

	var (
		bootstrapClusterProxy = input.BootstrapClusterProxy
		workloadClusterProxy  = input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
		clientset             = workloadClusterProxy.GetClientSet()
		labels                = map[string]string{clusterv1.ClusterLabelName: workloadClusterProxy.GetName()}
	)

	Expect(workloadClusterProxy).NotTo(BeNil())
	Expect(clientset).NotTo(BeNil())

	By(fmt.Sprintf("listing AzureMachinePools in the cluster in namespace %s", input.Namespace.Name))
	ampList := &v1alpha4.AzureMachinePoolList{}
	Expect(bootstrapClusterProxy.GetClient().List(ctx, ampList, client.InNamespace(input.Namespace.Name), client.MatchingLabels(labels))).ToNot(HaveOccurred())
	for _, amp := range ampList.Items {
		testMachinePoolCordonAndDrain(ctx, bootstrapClusterProxy, workloadClusterProxy, amp)
	}

}

func testMachinePoolCordonAndDrain(ctx context.Context, mgmtClusterProxy, workloadClusterProxy framework.ClusterProxy, amp v1alpha4.AzureMachinePool) {
	var (
		isWindows           = amp.Spec.Template.OSDisk.OSType == azure.WindowsOS
		clientset           = workloadClusterProxy.GetClientSet()
		owningMachinePool   = func() *clusterv1exp.MachinePool {
			mp, err := getOwnerMachinePool(ctx, mgmtClusterProxy.GetClient(), amp.ObjectMeta)
			Expect(err).ToNot(HaveOccurred())
			return mp
		}()
		
		machinePoolReplicas = func() int32 {
			Expect(owningMachinePool.Spec.Replicas).ToNot(BeNil(), "owning machine pool replicas must not be nil")
			Expect(*owningMachinePool.Spec.Replicas).To(BeNumerically(">=", 2), "owning machine pool replicas must be greater than or equal to 2")
			return *owningMachinePool.Spec.Replicas
		}()

		deploymentReplicas = func() int32 {
			return machinePoolReplicas * 2
		}()

		customizers = []deployCustomizerOption{
			func(builder *deployments.Builder, _ *corev1.Service) {
				antiAffinity := corev1.PodAntiAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
						{
							Weight: 90,
							PodAffinityTerm: corev1.PodAffinityTerm{
								TopologyKey: corev1.LabelHostname,
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "app",
											Operator: metav1.LabelSelectorOpIn,
											Values: []string{
												builder.GetName(),
											},
										},
									},
								},
							},
						},
					},
				}
				builder.AddMachinePoolSelectors(owningMachinePool.Name).
					SetReplicas(deploymentReplicas).
					AddPodAntiAffinity(antiAffinity)
			},
		}
	)

	By("labeling the machine pool nodes with machine pool type and name")
	ampmls, err := getAzureMachinePoolMachines(ctx, mgmtClusterProxy, workloadClusterProxy, amp)
	Expect(err).ToNot(HaveOccurred())
	labelNodesWithMachinePoolName(ctx, workloadClusterProxy.GetClient(), amp.Name, ampmls)

	By(fmt.Sprintf("deploying a publicly exposed HTTP service with pod anti-affinity on machine pool: %s/%s", amp.Namespace, amp.Name))
	_, _, _, cleanup := deployHttpService(ctx, clientset, isWindows, customizers...)
	defer cleanup()

	By(fmt.Sprintf("decreasing the replica count by 1 on the machine pool: %s/%s", amp.Namespace, amp.Name))
	Eventually(func() error {
		helper, err := patch.NewHelper(owningMachinePool, mgmtClusterProxy.GetClient())
		if err != nil {
			return err
		}

		decreasedReplicas := *owningMachinePool.Spec.Replicas - int32(1)
		owningMachinePool.Spec.Replicas = &decreasedReplicas
		return helper.Patch(ctx, owningMachinePool)
	})

	By(fmt.Sprintf("checking for a machine to start draining for machine pool: %s/%s", amp.Namespace, amp.Name))
	Eventually(func() error {
		ampmls, err := getAzureMachinePoolMachines(ctx, mgmtClusterProxy, workloadClusterProxy, amp)
		if err != nil {
			return errors.Wrap(err, "failed to list the azure machine pool machines")
		}

		for _, machine := range ampmls {
			if conditions.Has(&machine, clusterv1.DrainingSucceededCondition) && conditions.IsFalse(&machine, clusterv1.DrainingSucceededCondition) {
				return nil // started draining the node prior to delete
			}
		}

		return errors.New("no machine has started to drain")
	})

	By(fmt.Sprintf("checking for a machine to successfully complete draining for machine pool: %s/%s", amp.Namespace, amp.Name))
	Eventually(func() error {
		ampmls, err := getAzureMachinePoolMachines(ctx, mgmtClusterProxy, workloadClusterProxy, amp)
		if err != nil {
			return errors.Wrap(err, "failed to list the azure machine pool machines")
		}

		for _, machine := range ampmls {
			if conditions.Has(&machine, clusterv1.DrainingSucceededCondition) && conditions.IsTrue(&machine, clusterv1.DrainingSucceededCondition) {
				return nil // started draining the node prior to delete
			}
		}

		return errors.New("no machine has finished draining")
	})
}

func labelNodesWithMachinePoolName(ctx context.Context, workloadClient client.Client, mpName string, ampms []infrav1exp.AzureMachinePoolMachine) {
	for _, ampm := range ampms {
		n := &corev1.Node{}
		Expect(workloadClient.Get(ctx, client.ObjectKey{
			Name:      ampm.Status.NodeRef.Name,
			Namespace: ampm.Status.NodeRef.Namespace,
		}, n)).ToNot(HaveOccurred())
		n.Labels[clusterv1.OwnerKindAnnotation] = "MachinePool"
		n.Labels[clusterv1.OwnerNameAnnotation] = mpName
		Expect(workloadClient.Update(ctx, n)).ToNot(HaveOccurred())
	}
}

func getAzureMachinePoolMachines(ctx context.Context, mgmtClusterProxy, workloadClusterProxy framework.ClusterProxy, amp infrav1exp.AzureMachinePool) ([]infrav1exp.AzureMachinePoolMachine, error) {
	labels := map[string]string{
		clusterv1.ClusterLabelName:      workloadClusterProxy.GetName(),
		infrav1exp.MachinePoolNameLabel: amp.Name,
	}
	ampml := &infrav1exp.AzureMachinePoolMachineList{}
	if err := mgmtClusterProxy.GetClient().List(ctx, ampml, client.InNamespace(amp.Namespace), client.MatchingLabels(labels)); err != nil {
		return ampml.Items, errors.Wrap(err, "failed to list the azure machine pool machines")
	}

	return ampml.Items, nil
}

// getOwnerMachinePool returns the name of MachinePool object owning the current resource.
func getOwnerMachinePool(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*clusterv1exp.MachinePool, error) {
	for _, ref := range obj.OwnerReferences {
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, err
		}

		if ref.Kind == "MachinePool" && gv.Group == clusterv1exp.GroupVersion.Group {
			mp := &clusterv1exp.MachinePool{}
			err := c.Get(ctx, client.ObjectKey{
				Name:      ref.Name,
				Namespace: obj.Namespace,
			}, mp)
			return mp, err
		}
	}

	return nil, fmt.Errorf("failed to find owner machine pool for obj %+v", obj)
}

// deployHttpService creates a publicly exposed http service for Linux or Windows
func deployHttpService(ctx context.Context, clientset *kubernetes.Clientset, isWindows bool, opts ...deployCustomizerOption) (*deployments.Builder, *v1.Deployment, *corev1.Service, func()) {
	var (
		deploymentName = func() string {
			if isWindows {
				return "web-windows" + util.RandomString(6)
			}

			return "web" + util.RandomString(6)
		}()
		webDeploymentBuilder = deployments.Create("httpd", deploymentName, corev1.NamespaceDefault)
		servicesClient       = clientset.CoreV1().Services(corev1.NamespaceDefault)
		ports                = []corev1.ServicePort{
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
	)

	webDeploymentBuilder.AddContainerPort("http", "http", 80, corev1.ProtocolTCP)

	if isWindows {
		var windowsVersion windows.OSVersion
		Eventually(func() error {
			version, err := node.GetWindowsVersion(ctx, clientset)
			windowsVersion = version
			return err
		}, 300*time.Second, 5*time.Second).Should(Succeed())
		iisImage := windows.GetWindowsImage(windows.Httpd, windowsVersion)
		webDeploymentBuilder.SetImage(deploymentName, iisImage)
		webDeploymentBuilder.AddWindowsSelectors()
	}

	elbService := webDeploymentBuilder.GetService(ports, deployments.ExternalLoadbalancer)

	for _, opt := range opts {
		opt(webDeploymentBuilder, elbService)
	}

	Log("creating deployment and service")
	deployment, err := webDeploymentBuilder.Deploy(ctx, clientset)
	Expect(err).NotTo(HaveOccurred())
	deployInput := WaitForDeploymentsAvailableInput{
		Getter:     deploymentsClientAdapter{client: webDeploymentBuilder.Client(clientset)},
		Deployment: deployment,
		Clientset:  clientset,
	}
	WaitForDeploymentsAvailable(ctx, deployInput, e2eConfig.GetIntervals(AzureMachinePoolDrainSpecName, "wait-deployment")...)

	service, err := servicesClient.Create(ctx, elbService, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
	elbSvcInput := WaitForServiceAvailableInput{
		Getter:    servicesClientAdapter{client: servicesClient},
		Service:   elbService,
		Clientset: clientset,
	}
	WaitForServiceAvailable(ctx, elbSvcInput, e2eConfig.GetIntervals(AzureMachinePoolDrainSpecName, "wait-service")...)

	return webDeploymentBuilder, deployment, service, func() {
		Expect(servicesClient.Delete(ctx, elbService.Name, metav1.DeleteOptions{})).ToNot(HaveOccurred())
		Expect(webDeploymentBuilder.Client(clientset).Delete(ctx, deployment.Name, metav1.DeleteOptions{})).ToNot(HaveOccurred())
	}
}

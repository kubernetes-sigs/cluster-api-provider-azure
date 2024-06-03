//go:build e2e
// +build e2e

/*
Copyright 2023 The Kubernetes Authors.

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
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	kubedrain "k8s.io/kubectl/pkg/drain"
	"k8s.io/utils/ptr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	deploymentBuilder "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/deployment"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	clusterAutoscalerHelmRepoURL                   = "https://kubernetes.github.io/autoscaler"
	clusterAutoscalerChartName                     = "cluster-autoscaler"
	clusterAutoscalerDeploymentLabelKey            = "app.kubernetes.io/name"
	clusterAutoscalerCAPIDeploymentLabelVal        = "clusterapi-cluster-autoscaler"
	clusterAutoscalerAzureDeploymentLabelVal       = "azure-cluster-autoscaler"
	defaultKubeletMaxPods                    int32 = 110
)

// ClusterAutoscalerInstallInput is the input for InstallClusterAutoscaler.
type ClusterAutoscalerInstallInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	E2EConfig             *clusterctl.E2EConfig
}

// InstallClusterAutoscaler implements a test that verifies cluster-autoscaler behaviors.
func InstallClusterAutoscaler(ctx context.Context, inputGetter func() ClusterAutoscalerInstallInput) {
	var (
		specName = "clusterAutoscaler"
		input    ClusterAutoscalerInstallInput
	)

	Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).ToNot(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	Expect(input.ClusterName).ToNot(BeEmpty(), "Invalid argument. input.ClusterName can't be empty when calling %s spec", specName)

	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())
	workloadClusterProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
	workloadClusterClient := workloadClusterProxy.GetClient()
	Expect(workloadClusterClient).NotTo(BeNil())
	Expect(workloadClusterProxy).NotTo(BeNil())
	mdList := framework.GetMachineDeploymentsByCluster(ctx, framework.GetMachineDeploymentsByClusterInput{
		Lister:      mgmtClient,
		ClusterName: input.ClusterName,
		Namespace:   input.Namespace.Name,
	})
	mpList := framework.GetMachinePoolsByCluster(ctx, framework.GetMachinePoolsByClusterInput{
		Lister:      mgmtClient,
		ClusterName: input.ClusterName,
		Namespace:   input.Namespace.Name,
	})
	amcpList := &infrav1.AzureManagedControlPlaneList{}
	Eventually(func() error {
		return mgmtClient.List(ctx, amcpList, []client.ListOption{
			client.InNamespace(input.Namespace.Name),
			client.MatchingLabels{
				clusterv1.ClusterNameLabel: input.ClusterName,
			},
		}...)
	}, retryableOperationTimeout, retryableOperationSleepBetweenRetries).Should(Succeed(), "Failed to list MachinePools object for Cluster %s", klog.KRef(input.Namespace.Name, input.ClusterName))
	var hasAKSAutoscalerCapability bool
	if len(amcpList.Items) > 0 {
		hasAKSAutoscalerCapability = true
	}
	var hasCAPIClusterAutoscalerConfig bool
	for _, md := range mdList {
		if _, ok := md.GetAnnotations()["cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size"]; ok {
			if _, ok := md.GetAnnotations()["cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size"]; ok {
				hasCAPIClusterAutoscalerConfig = true
			}
		}
	}
	var autoscalerEnabledMachinePool *expv1.MachinePool
	for _, mp := range mpList {
		if _, ok := mp.GetAnnotations()["cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size"]; ok {
			if _, ok := mp.GetAnnotations()["cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size"]; ok {
				hasCAPIClusterAutoscalerConfig = true
			}
		}
	}
	var options *HelmOptions
	var targetCluster framework.ClusterProxy
	if hasCAPIClusterAutoscalerConfig {
		options = &HelmOptions{
			Values: []string{
				"extraArgs.balance-similar-node-groups=true",
				"extraArgs.scale-down-unneeded-time=1m",
				"cloudProvider=clusterapi",
				fmt.Sprintf("autoDiscovery.clusterName=%s", input.ClusterName),
				fmt.Sprintf("clusterAPIKubeconfigSecret=%s-kubeconfig", input.ClusterName),
				"clusterAPIMode=kubeconfig-incluster",
				"extraArgs.enable-provisioning-requests=true",
			},
			StringValues: []string{
				"extraArgs.scan-interval=1m",
			},
		}
		targetCluster = bootstrapClusterProxy
	}
	var hasExternalAutoscalerConfig bool
	var vmssName string
	for _, mp := range mpList {
		if _, ok := mp.GetAnnotations()["cluster.x-k8s.io/replicas-managed-by"]; ok {
			hasExternalAutoscalerConfig = !hasAKSAutoscalerCapability
			vmssName = mp.Spec.Template.Spec.InfrastructureRef.Name
			autoscalerEnabledMachinePool = mp
		}
	}
	if hasExternalAutoscalerConfig {
		options = &HelmOptions{
			Values: []string{
				"cloudProvider=azure",
				"azureVMType=vmss",
				fmt.Sprintf("autoscalingGroups[0].name=%s", vmssName),
				fmt.Sprintf("azureResourceGroup=%s", input.ClusterName),
			},
			StringValues: []string{
				fmt.Sprintf("azureClientID=%s", input.E2EConfig.GetVariable(AzureClientID)),
				fmt.Sprintf("azureClientSecret=%s", input.E2EConfig.GetVariable(AzureClientSecret)),
				fmt.Sprintf("azureSubscriptionID=%s", input.E2EConfig.GetVariable(AzureSubscriptionID)),
				fmt.Sprintf("azureTenantID=%s", input.E2EConfig.GetVariable(AzureTenantID)),
				fmt.Sprintf("autoscalingGroups[0].minSize=%s", input.E2EConfig.GetVariable(ClusterAutoscalerExternalMinNodes)),
				fmt.Sprintf("autoscalingGroups[0].maxSize=%s", input.E2EConfig.GetVariable(ClusterAutoscalerExternalMaxNodes)),
				"extraArgs.scan-interval=1m",
				"extraArgs.scale-down-unneeded-time=1m",
				"extraArgs.enable-provisioning-requests=true",
			},
		}
		targetCluster = workloadClusterProxy
	}
	ammpList := &infrav1.AzureManagedMachinePoolList{}
	if hasAKSAutoscalerCapability {
		Expect(autoscalerEnabledMachinePool).NotTo(BeNil())
		var aksVMSSName, aksVMSSResourceGroup string
		Eventually(func() error {
			return mgmtClient.List(ctx, ammpList, byClusterOptions(input.ClusterName, input.Namespace.Name)...)
		}, retryableOperationTimeout, retryableOperationSleepBetweenRetries).Should(Succeed(), "Failed to list MachinePools object for Cluster %s", input.ClusterName)
		r := regexp.MustCompile(regexpUniformInstance)
		for _, ammp := range ammpList.Items {
			if ammp.Spec.Mode == string(infrav1.NodePoolModeUser) && ammp.Name == autoscalerEnabledMachinePool.Name {
				for _, providerID := range ammp.Spec.ProviderIDList {
					matches := r.FindStringSubmatch(providerID)
					Expect(matches).To(HaveLen(4))
					aksVMSSResourceGroup = matches[2]
					aksVMSSName = matches[3]
				}
				if aksVMSSName != "" {
					break
				}
			}
		}
		options = &HelmOptions{
			Values: []string{
				"cloudProvider=azure",
				"azureVMType=vmss",
				fmt.Sprintf("autoscalingGroups[0].name=%s", aksVMSSName),
				fmt.Sprintf("azureResourceGroup=%s", aksVMSSResourceGroup),
			},
			StringValues: []string{
				fmt.Sprintf("azureClientID=%s", input.E2EConfig.GetVariable(AzureClientID)),
				fmt.Sprintf("azureClientSecret=%s", input.E2EConfig.GetVariable(AzureClientSecret)),
				fmt.Sprintf("azureSubscriptionID=%s", input.E2EConfig.GetVariable(AzureSubscriptionID)),
				fmt.Sprintf("azureTenantID=%s", input.E2EConfig.GetVariable(AzureTenantID)),
				fmt.Sprintf("autoscalingGroups[0].minSize=%s", input.E2EConfig.GetVariable(ClusterAutoscalerExternalMinNodes)),
				fmt.Sprintf("autoscalingGroups[0].maxSize=%s", input.E2EConfig.GetVariable(ClusterAutoscalerExternalMaxNodes)),
				fmt.Sprintf("azureScaleDownPolicy=%s", input.E2EConfig.GetVariable(ClusterAutoscalerExternalMaxNodes)),
				"extraArgs.scan-interval=10s",
				"extraArgs.scale-down-delay-after-add=1m",
				"extraArgs.scale-down-delay-after-delete=10s",
				"extraArgs.scale-down-delay-after-failure=3m",
				"extraArgs.scale-down-unneeded-time=1m",
				"extraArgs.scale-down-unready-time=20m",
				"extraArgs.scale-down-utilization-threshold=0.5",
				"extraArgs.max-graceful-termination-sec=600",
				"extraArgs.balance-similar-node-groups=false",
				"extraArgs.skip-nodes-with-local-storage=false",
				"extraArgs.skip-nodes-with-system-pods=true",
				"extraArgs.max-empty-bulk-delete=10",
				"extraArgs.expander=random",
				"extraArgs.new-pod-scale-up-delay=0s",
				"extraArgs.max-total-unready-percentage=45",
				"extraArgs.ok-total-unready-count=100",
				"extraArgs.max-node-provision-time=15m",
				"extraArgs.enable-provisioning-requests=true",
				"extraArgs.v=2",
			},
		}
		targetCluster = workloadClusterProxy
	}
	imageRepo := input.E2EConfig.GetVariable(ClusterAutoscalerImageRepo)
	if imageRepo != "" {
		options.Values = append(options.Values, fmt.Sprintf("image.repository=%s", imageRepo))
	}
	imageTag := input.E2EConfig.GetVariable(ClusterAutoscalerImageTag)
	if imageTag != "" {
		options.Values = append(options.Values, fmt.Sprintf("image.tag=%s", imageTag))
	}
	InstallHelmChart(ctx, targetCluster, input.Namespace.Name, clusterAutoscalerHelmRepoURL, clusterAutoscalerChartName, input.ClusterName, options, "")
	Eventually(func(g Gomega) bool {
		var d = &appsv1.DeploymentList{}
		var deploymentLabelVal string
		if hasCAPIClusterAutoscalerConfig {
			deploymentLabelVal = clusterAutoscalerCAPIDeploymentLabelVal
		} else if hasExternalAutoscalerConfig || hasAKSAutoscalerCapability {
			deploymentLabelVal = clusterAutoscalerAzureDeploymentLabelVal
		}
		Logf("Listing deployments in namespace %s with label %s=%s", input.Namespace.Name, clusterAutoscalerDeploymentLabelKey, deploymentLabelVal)
		err := targetCluster.GetClient().List(ctx, d, client.InNamespace(input.Namespace.Name), client.MatchingLabels{clusterAutoscalerDeploymentLabelKey: deploymentLabelVal})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(d).NotTo(BeNil())
		g.Expect(d.Items).To(HaveLen(1))
		for _, c := range d.Items[0].Status.Conditions {
			if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
				return true
			}
		}
		return false
	}, e2eConfig.GetIntervals(specName, "wait-deployment")...).Should(BeTrue())
}

// AzureAutoscalerSpecInput is the input for AzureAutoscalerSpec.
type AzureAutoscalerSpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	E2EConfig             *clusterctl.E2EConfig
}

// AzureAutoscalerSpec implements a test that verifies we can autoscale.
func AzureAutoscalerSpec(ctx context.Context, inputGetter func() AzureAutoscalerSpecInput) {
	var (
		specName = "azure-autoscaler"
		input    AzureAutoscalerSpecInput
	)

	Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	Expect(input.Namespace).ToNot(BeNil(), "Invalid argument. input.Namespace can't be nil when calling %s spec", specName)
	Expect(input.ClusterName).ToNot(BeEmpty(), "Invalid argument. input.ClusterName can't be empty when calling %s spec", specName)

	By("Creating a Kubernetes client to the workload cluster")
	workloadClusterProxy := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
	Expect(workloadClusterProxy).NotTo(BeNil())
	mgmtClient := bootstrapClusterProxy.GetClient()
	Expect(mgmtClient).NotTo(BeNil())
	clientset := workloadClusterProxy.GetClientSet()
	Expect(clientset).NotTo(BeNil())
	workloadClusterClient := workloadClusterProxy.GetClient()
	Expect(workloadClusterClient).NotTo(BeNil())

	By("Calculating how many pod replicas we need to scale out to maximum")
	mdList := framework.GetMachineDeploymentsByCluster(ctx, framework.GetMachineDeploymentsByClusterInput{
		Lister:      mgmtClient,
		ClusterName: input.ClusterName,
		Namespace:   input.Namespace.Name,
	})
	mpList := framework.GetMachinePoolsByCluster(ctx, framework.GetMachinePoolsByClusterInput{
		Lister:      mgmtClient,
		ClusterName: input.ClusterName,
		Namespace:   input.Namespace.Name,
	})

	clusterAutoscalerMaxNodes, hasNodeHeadroom, hasCapiClusterAutoscaler, hasExternalClusterAutoscaler := getAutoscalerContext(input, mdList, mpList)
	if !hasNodeHeadroom {
		By("Bypassing cluster-autoscaler scale out because our current node count is not less than our max-size configuration for any pools")
		return
	}
	By(fmt.Sprintf("Will validate cluster autoscaler can scale out to %d nodes", clusterAutoscalerMaxNodes))

	nodes := &corev1.NodeList{}
	By("Verifying that no pods are scheduled to the autoscaler-enabled MachineDeployment nodes")
	Eventually(func(g Gomega) {
		mdList := framework.GetMachineDeploymentsByCluster(ctx, framework.GetMachineDeploymentsByClusterInput{
			Lister:      mgmtClient,
			ClusterName: input.ClusterName,
			Namespace:   input.Namespace.Name,
		})
		for _, md := range mdList {
			_, ok := md.GetAnnotations()["cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size"]
			if !ok {
				continue
			}
			// get nodes that start with name
			g.Expect(workloadClusterClient.List(ctx, nodes)).To(Succeed())
			drainer := &kubedrain.Helper{
				Client:              clientset,
				Ctx:                 ctx,
				Force:               true,
				IgnoreAllDaemonSets: true,
				DeleteEmptyDirData:  true,
				GracePeriodSeconds:  -1,
				// If a pod is not evicted in 20 seconds, retry the eviction next time the
				// machine gets reconciled again (to allow other machines to be reconciled).
				Timeout: 20 * time.Second,
				ErrOut:  os.Stderr,
				Out:     os.Stdout,
			}
			By(fmt.Sprintf("Cordoning and Draining all replicaset pods from MachineDeployment %s", md.Name))
			for _, n := range nodes.Items {
				cordonDrainNode := n
				if strings.Contains(n.Name, md.Name) {
					g.Expect(kubedrain.RunCordonOrUncordon(drainer, &cordonDrainNode, true)).To(Succeed())
					g.Expect(kubedrain.RunNodeDrain(drainer, cordonDrainNode.Name)).To(Succeed())
					uncordonDrainer := &kubedrain.Helper{
						Client: clientset,
						ErrOut: os.Stderr,
						Out:    os.Stdout,
						Ctx:    ctx,
					}
					g.Expect(kubedrain.RunCordonOrUncordon(uncordonDrainer, &cordonDrainNode, false)).To(Succeed())
				}
			}
		}
	}, e2eConfig.GetIntervals(specName, "node-drain/wait-machine-deleted")...).Should(Succeed())
	By("Verifying that no pods are scheduled to the autoscaler-enabled MachinePool nodes")
	Eventually(func(g Gomega) {
		mpList := framework.GetMachinePoolsByCluster(ctx, framework.GetMachinePoolsByClusterInput{
			Lister:      mgmtClient,
			ClusterName: input.ClusterName,
			Namespace:   input.Namespace.Name,
		})
		for _, mp := range mpList {
			if _, ok := mp.GetAnnotations()["cluster.x-k8s.io/replicas-managed-by"]; !ok {
				continue
			}
			// get nodes that start with name
			Expect(workloadClusterClient.List(ctx, nodes)).To(Succeed())
			drainer := &kubedrain.Helper{
				Client:              clientset,
				Ctx:                 ctx,
				Force:               true,
				IgnoreAllDaemonSets: true,
				DeleteEmptyDirData:  true,
				GracePeriodSeconds:  -1,
				// If a pod is not evicted in 20 seconds, retry the eviction next time the
				// machine gets reconciled again (to allow other machines to be reconciled).
				Timeout: 20 * time.Second,
				ErrOut:  os.Stderr,
				Out:     os.Stdout,
			}
			By(fmt.Sprintf("Cordoning and Draining all replicaset pods from MachinePool %s", mp.Name))
			for _, n := range nodes.Items {
				cordonDrainNode := n
				if strings.Contains(n.Name, mp.Name) {
					g.Expect(kubedrain.RunCordonOrUncordon(drainer, &cordonDrainNode, true)).To(Succeed())
					g.Expect(kubedrain.RunNodeDrain(drainer, cordonDrainNode.Name)).To(Succeed())
					uncordonDrainer := &kubedrain.Helper{
						Client: clientset,
						ErrOut: os.Stderr,
						Out:    os.Stdout,
						Ctx:    ctx,
					}
					g.Expect(kubedrain.RunCordonOrUncordon(uncordonDrainer, &cordonDrainNode, false)).To(Succeed())
				}
			}
		}
	}, e2eConfig.GetIntervals(specName, "node-drain/wait-machine-deleted")...).Should(Succeed())

	By("Verifying that all nodes have scaled in")
	validateScaleIn(ctx, specName, input, mgmtClient, workloadClusterClient)

	numReplicas := (int32(len(nodes.Items)) + clusterAutoscalerMaxNodes) * defaultKubeletMaxPods
	By(fmt.Sprintf("creating an HTTP deployment with %d replicas", numReplicas))
	deploymentName := "web" + util.RandomString(6)
	webDeployment := deploymentBuilder.Create("httpd", deploymentName, corev1.NamespaceDefault, numReplicas)
	webDeployment.AddContainerPort("http", "http", 80, corev1.ProtocolTCP)

	deployment, err := webDeployment.Deploy(ctx, clientset)
	Expect(err).NotTo(HaveOccurred())

	if hasCapiClusterAutoscaler {
		By("Verifying that all cluster-autoscaler-enabled MachineDeployments have scaled out")
		Eventually(func(g Gomega) {
			mdList := framework.GetMachineDeploymentsByCluster(ctx, framework.GetMachineDeploymentsByClusterInput{
				Lister:      mgmtClient,
				ClusterName: input.ClusterName,
				Namespace:   input.Namespace.Name,
			})
			for _, md := range mdList {
				val, ok := md.GetAnnotations()["cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size"]
				if !ok {
					continue
				}
				maxSize, err := strconv.ParseInt(val, 10, 32)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(*md.Spec.Replicas).To(Equal(int32(maxSize)))
				g.Expect(md.Status.Phase).To(Equal(string(clusterv1.MachineDeploymentPhaseRunning)))
				nodes := &corev1.NodeList{}
				g.Expect(workloadClusterClient.List(ctx, nodes)).To(Succeed())
				numRunningNodes := 0
				for _, n := range nodes.Items {
					if strings.Contains(n.Name, md.Name) {
						for _, condition := range n.Status.Conditions {
							if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
								numRunningNodes++
							}
						}
					}
				}
				g.Expect(numRunningNodes).To(Equal(int(maxSize)))
			}
		}, e2eConfig.GetIntervals(specName, "wait-autoscale-out")...).Should(Succeed())
	}

	if hasExternalClusterAutoscaler {
		By("Verifying that cluster-autoscaler-enabled MachinePool has scaled out")
		Eventually(func(g Gomega) {
			mpList := framework.GetMachinePoolsByCluster(ctx, framework.GetMachinePoolsByClusterInput{
				Lister:      mgmtClient,
				ClusterName: input.ClusterName,
				Namespace:   input.Namespace.Name,
			})
			var autoscalerEnabledMachinePool *expv1.MachinePool
			for _, mp := range mpList {
				if _, ok := mp.GetAnnotations()["cluster.x-k8s.io/replicas-managed-by"]; ok {
					autoscalerEnabledMachinePool = mp
				}
			}
			g.Expect(autoscalerEnabledMachinePool).ToNot(BeNil())
			maxSize, err := strconv.ParseInt(input.E2EConfig.GetVariable(ClusterAutoscalerExternalMaxNodes), 10, 32)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(*autoscalerEnabledMachinePool.Spec.Replicas).To(Equal(int32(maxSize)))
			g.Expect(autoscalerEnabledMachinePool.Status.ReadyReplicas).To(Equal(int32(maxSize)))
			g.Expect(autoscalerEnabledMachinePool.Status.Phase).To(Equal(string(expv1.MachinePoolPhaseRunning)))
			g.Expect(autoscalerEnabledMachinePool.Spec.ProviderIDList).To(HaveLen(int(maxSize)))
			g.Expect(autoscalerEnabledMachinePool.Status.NodeRefs).To(HaveLen(int(maxSize)))
			nodes := &corev1.NodeList{}
			g.Expect(workloadClusterClient.List(ctx, nodes)).To(Succeed())
			numNodes := 0
			split := strings.Split(autoscalerEnabledMachinePool.Status.NodeRefs[0].Name, "-")
			nodeNamePrefix := strings.Join(split[:len(split)-1], "-")
			for _, n := range nodes.Items {
				if strings.Contains(n.Name, nodeNamePrefix) {
					numNodes++
				}
			}
			g.Expect(numNodes).To(BeNumerically(">=", int(maxSize)))
		}, e2eConfig.GetIntervals(specName, "wait-autoscale-out")...).Should(Succeed())
	}

	By("Reducing deployment replicas to zero")
	Eventually(func(g Gomega) {
		d, err := clientset.AppsV1().Deployments(deployment.Namespace).Get(ctx, deployment.Name, metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		d.Spec.Replicas = ptr.To[int32](0)
		err = workloadClusterClient.Update(ctx, d)
		g.Expect(err).NotTo(HaveOccurred())
	}, e2eConfig.GetIntervals(specName, "wait-deployment")...).Should(Succeed())
	By("Verifying that all nodes have scaled back in")
	validateScaleIn(ctx, specName, input, mgmtClient, workloadClusterClient)
}

func validateScaleIn(ctx context.Context, specName string, input AzureAutoscalerSpecInput, mgmtClient, workloadClusterClient client.Client) {
	Eventually(func(g Gomega) {
		mdList := framework.GetMachineDeploymentsByCluster(ctx, framework.GetMachineDeploymentsByClusterInput{
			Lister:      mgmtClient,
			ClusterName: input.ClusterName,
			Namespace:   input.Namespace.Name,
		})
		mpList := framework.GetMachinePoolsByCluster(ctx, framework.GetMachinePoolsByClusterInput{
			Lister:      mgmtClient,
			ClusterName: input.ClusterName,
			Namespace:   input.Namespace.Name,
		})
		minSize, err := strconv.ParseInt(input.E2EConfig.GetVariable(ClusterAutoscalerExternalMinNodes), 10, 32)
		g.Expect(err).NotTo(HaveOccurred())
		for _, mp := range mpList {
			if _, ok := mp.GetAnnotations()["cluster.x-k8s.io/replicas-managed-by"]; !ok {
				continue
			}
			By(fmt.Sprintf("Waiting for MachinePool %s to scale down to %d or fewer replicas", mp.Name, minSize))
			g.Expect(int64(*mp.Spec.Replicas) <= minSize).To(BeTrue())
			nodes := &corev1.NodeList{}
			g.Expect(workloadClusterClient.List(ctx, nodes)).To(Succeed())
		}
		for _, md := range mdList {
			val, ok := md.GetAnnotations()["cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size"]
			if !ok {
				continue
			}
			minSize, err := strconv.ParseInt(val, 10, 32)
			g.Expect(err).NotTo(HaveOccurred())
			By(fmt.Sprintf("Waiting for MachineDeployment %s to scale down to %d replicas", md.Name, minSize))
			g.Expect(int64(*md.Spec.Replicas)).To(Equal(minSize))
			nodes := &corev1.NodeList{}
			g.Expect(workloadClusterClient.List(ctx, nodes)).To(Succeed())
		}
	}, e2eConfig.GetIntervals(specName, "wait-autoscale-in")...).Should(Succeed())
}

func getAutoscalerContext(input AzureAutoscalerSpecInput,
	mdList []*clusterv1.MachineDeployment,
	mpList []*expv1.MachinePool) (
	clusterAutoscalerMaxNodes int32,
	hasNodeHeadroom, hasCapiClusterAutoscaler, hasExternalClusterAutoscaler bool) {
	for _, md := range mdList {
		val, ok := md.GetAnnotations()["cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size"]
		if !ok {
			continue
		}
		hasCapiClusterAutoscaler = true
		maxSize, err := strconv.ParseInt(val, 10, 32)
		Expect(err).NotTo(HaveOccurred())
		if maxSize > int64(*md.Spec.Replicas) {
			hasNodeHeadroom = true
		}
		clusterAutoscalerMaxNodes += int32(maxSize)
	}
	for _, mp := range mpList {
		if _, ok := mp.GetAnnotations()["cluster.x-k8s.io/replicas-managed-by"]; ok {
			hasExternalClusterAutoscaler = true
			maxSize, err := strconv.ParseInt(input.E2EConfig.GetVariable(ClusterAutoscalerExternalMaxNodes), 10, 32)
			Expect(err).NotTo(HaveOccurred())
			if maxSize > int64(*mp.Spec.Replicas) {
				hasNodeHeadroom = true
			}
			clusterAutoscalerMaxNodes += int32(maxSize)
		}
		if val, ok := mp.GetAnnotations()["cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size"]; ok {
			hasCapiClusterAutoscaler = true
			maxSize, err := strconv.ParseInt(val, 10, 32)
			Expect(err).NotTo(HaveOccurred())
			if maxSize > int64(*mp.Spec.Replicas) {
				hasNodeHeadroom = true
			}
			clusterAutoscalerMaxNodes += int32(maxSize)
		}
	}
	return clusterAutoscalerMaxNodes, hasNodeHeadroom, hasCapiClusterAutoscaler, hasExternalClusterAutoscaler
}

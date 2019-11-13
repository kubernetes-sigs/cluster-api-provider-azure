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
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachinerytypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	capz "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha2"
	capi "sigs.k8s.io/cluster-api/api/v1alpha2"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha2"
	kubeadmv1beta1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/kubeadm/v1beta1"
	"sigs.k8s.io/cluster-api/util"
)

const (
	prefix       = "capz-e2e-"
	k8sVersion   = "v1.16.2"
	controlplane = "controlplane"
)

var _ = Describe("functional tests", func() {
	var (
		namespace     string
		clusterName   string
		cancelWatches context.CancelFunc
	)

	BeforeEach(func() {
		suffix := util.RandomString(6)

		clusterName = prefix + suffix

		var ctx context.Context
		ctx, cancelWatches = context.WithCancel(context.Background())

		namespace = prefix + suffix
		createNamespace(namespace)

		go func() {
			defer GinkgoRecover()
			watchEvents(ctx, namespace)
		}()
	})

	AfterEach(func() {
		defer cancelWatches()
	})

	Describe("workload cluster lifecycle", func() {
		It("It should be creatable and deletable", func() {
			By("Creating Cluster infrastructure")
			createClusterInfrastructure(namespace, clusterName)
			By("Ensuring Cluster infrastructure")
			ensureClusterInfrastructure(namespace, clusterName)

			By("Creating first control plane Machine")
			cp0 := clusterName + "-controlplane-0"
			createFirstControlPlaneMachine(namespace, clusterName, cp0, k8sVersion)
			By("Ensuring control plane Machine")
			ensureMachine(namespace, cp0)

			By("Creating second control plane Machine")
			cp1 := clusterName + "-controlplane-1"
			addControlPlaneMachine(namespace, clusterName, cp1, k8sVersion)
			By("Ensuring control plane Machine")
			ensureMachine(namespace, cp1)

			By("Creating third control plane Machine")
			cp2 := clusterName + "-controlplane-2"
			addControlPlaneMachine(namespace, clusterName, cp2, k8sVersion)
			By("Ensuring control plane Machine")
			ensureMachine(namespace, cp2)

			// TODO: Retrieve Cluster kubeconfig
			// TODO: Deploy Addons
			// TODO: Validate Node Ready
			// TODO: Deploy additional Control Plane Nodes
			// TODO: Deploy a MachineDeployment
			// TODO: Scale MachineDeployment up
			// TODO: Scale MachineDeployment down

			By("Deleting cluster")
			deleteCluster(namespace, clusterName)
			ensureDeleted(namespace, clusterName)
		})
	})
})

func createNamespace(namespace string) {
	fmt.Fprintf(GinkgoWriter, "Creating namespace \"%q\"\n", namespace)
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	Expect(kindClient.Create(context.TODO(), ns)).To(Succeed())
}

func createClusterInfrastructure(namespace, clusterName string) {
	createAzureCluster(namespace, clusterName)
	createCluster(namespace, clusterName)
}

func createAzureCluster(namespace, clusterName string) {
	fmt.Fprintf(GinkgoWriter, "Creating Cluster resource \"%q\"\n", clusterName)
	cluster := &capz.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
		},
		Spec: capz.AzureClusterSpec{
			Location:      "westus2",
			ResourceGroup: clusterName,
			NetworkSpec: capz.NetworkSpec{
				Vnet: capz.VnetSpec{Name: clusterName + "-vnet"},
			},
		},
	}
	Expect(kindClient.Create(context.TODO(), cluster)).To(BeNil())
}

func createCluster(namespace, clusterName string) {
	fmt.Fprintf(GinkgoWriter, "Creating Cluster resource \"%q\"\n", clusterName)
	cluster := &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
		},
		Spec: capi.ClusterSpec{
			ClusterNetwork: &capi.ClusterNetwork{
				Pods: &capi.NetworkRanges{
					CIDRBlocks: []string{"192.168.0.0/16"},
				},
			},
			InfrastructureRef: &corev1.ObjectReference{
				Kind:       "AzureCluster",
				APIVersion: capz.GroupVersion.String(),
				Name:       clusterName,
				Namespace:  namespace,
			},
		},
	}
	Expect(kindClient.Create(context.TODO(), cluster)).To(BeNil())
}

func ensureClusterInfrastructure(namespace, clusterName string) {
	fmt.Fprintf(GinkgoWriter, "Ensuring cluster infrastructure is ready\n")
	Eventually(
		func() (bool, error) {
			ns := apimachinerytypes.NamespacedName{Namespace: namespace, Name: clusterName}
			cluster := &capz.AzureCluster{}
			if err := kindClient.Get(context.TODO(), ns, cluster); err != nil {
				return false, err
			}
			return cluster.Status.Ready, nil
		},
		10*time.Minute, 15*time.Second,
	).Should(BeTrue())

	fmt.Fprintf(GinkgoWriter, "Ensuring cluster is ready\n")
	Eventually(
		func() (bool, error) {
			ns := apimachinerytypes.NamespacedName{Namespace: namespace, Name: clusterName}
			cluster := &capi.Cluster{}
			if err := kindClient.Get(context.TODO(), ns, cluster); err != nil {
				return false, err
			}
			return cluster.Status.InfrastructureReady, nil
		},
		10*time.Minute, 15*time.Second,
	).Should(BeTrue())
}

func createFirstControlPlaneMachine(namespace, clusterName, machineName, k8sVersion string) {
	createAzureMachine(namespace, machineName)
	createKubeadmConfig(namespace, machineName, true)
	createMachine(namespace, machineName, clusterName, k8sVersion)
}

func addControlPlaneMachine(namespace, clusterName, machineName, k8sVersion string) {
	createAzureMachine(namespace, machineName)
	createKubeadmConfig(namespace, machineName, false)
	createMachine(namespace, machineName, clusterName, k8sVersion)
}

func createAzureMachine(namespace, name string) {
	fmt.Fprintf(GinkgoWriter, "Creating Azure Machine %s/%s\n", namespace, name)
	imageOffer := "capi"
	imagePublisher := "cncf-upstream"
	imageSKU := "k8s-1dot16-ubuntu-1804"
	imageVersion := "latest"
	azureMachine := &capz.AzureMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: capz.AzureMachineSpec{
			VMSize:       "Standard_B2ms",
			Location:     "westus2",
			SSHPublicKey: "c3NoLXJzYSBBQUFBQjNOemFDMXljMkVBQUFBREFRQUJBQUFCQVFEYUhTODQyRWMvU1V5OCszaVVXT28vdlU3UzhicXNMOE9uOTFxYlVYN3RKcSs1SG9aNnlsWldWTHRZb0xkR25ld0NWRVZoQmJxK1M1T0wvOUVyOS9wMDZyUU1URVlzZEhCc0tneFZxWmRjMEE2bEEyQVV1YUVFUEhtNXlYQWlXUHlISTVhR2lDaFY0TnFyRW12NWV3OWROLzkySnhRZXhmVFNxRVNFTk5BN2hiZmtvM1F1bUVaZ0JkVFViSm91MTloOWJtVklRZTNCN3ZFTi9KbWxQWUJzaHVkUkE2bWZDL3FWTWZ2eTAxSnZqUEN1eS9wZ3FHSUlTb1JyaStmeVZ0WVN2MWJUMnQwOFdFTlR4Vks3VlBvZ0NjL3RKNTJwQlpack1DamsvWXcwQnpHcjY0Nk91NGtjOFFiSFQ1bUlEejZwbVcvcGNIT3VkeGtZcFgvVzNEOUogamFkYXJzaWVASmF2aWVycy1NQlAuZ3Vlc3QuY29ycC5taWNyb3NvZnQuY29tCg==",
			Image: capz.Image{
				Offer:     &imageOffer,
				Publisher: &imagePublisher,
				SKU:       &imageSKU,
				Version:   &imageVersion,
			},
			OSDisk: capz.OSDisk{
				DiskSizeGB: 30,
				OSType:     "Linux",
				ManagedDisk: capz.ManagedDisk{
					StorageAccountType: "Premium_LRS",
				},
			},
		},
	}
	Expect(kindClient.Create(context.TODO(), azureMachine)).To(Succeed())
}

func createKubeadmConfig(namespace, clusterName string, firstControlPlane bool) {
	fmt.Fprintf(GinkgoWriter, "Creating Init KubeadmConfig %s/%s\n", namespace, clusterName)
	kc := &bootstrapv1.KubeadmConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
		},
		Spec: bootstrapv1.KubeadmConfigSpec{
			Files: []bootstrapv1.File{
				{
					Owner:       "root:root",
					Path:        "/etc/kubernetes/azure.json",
					Permissions: "0644",
					Content:     azureJson(clusterName),
				},
			},
		},
	}
	if firstControlPlane {
		kc.Spec.ClusterConfiguration = &kubeadmv1beta1.ClusterConfiguration{
			APIServer: kubeadmv1beta1.APIServer{
				ControlPlaneComponent: kubeadmv1beta1.ControlPlaneComponent{
					ExtraArgs: map[string]string{
						"cloud-provider": "azure",
						"cloud-config":   "/etc/kubernetes/azure.json",
					},
					ExtraVolumes: []kubeadmv1beta1.HostPathMount{
						{
							Name:      "cloud-config",
							HostPath:  "/etc/kubernetes/azure.json",
							MountPath: "/etc/kubernetes/azure.json",
							ReadOnly:  true,
						},
					},
				},
				TimeoutForControlPlane: &metav1.Duration{20 * time.Minute},
			},
			ControllerManager: kubeadmv1beta1.ControlPlaneComponent{
				ExtraArgs: map[string]string{
					"allocate-node-cidrs": "false",
					"cloud-provider":      "azure",
					"cloud-config":        "/etc/kubernetes/azure.json",
				},
				ExtraVolumes: []kubeadmv1beta1.HostPathMount{
					{
						Name:      "cloud-config",
						HostPath:  "/etc/kubernetes/azure.json",
						MountPath: "/etc/kubernetes/azure.json",
						ReadOnly:  true,
					},
				},
			},
		}
	}
	nd := kubeadmv1beta1.NodeRegistrationOptions{
		Name: "{{ ds.meta_data[\"local_hostname\"] }}",
		KubeletExtraArgs: map[string]string{
			"cloud-provider": "azure",
			"cloud-config":   "/etc/kubernetes/azure.json",
		},
	}
	if firstControlPlane {
		kc.Spec.InitConfiguration = &kubeadmv1beta1.InitConfiguration{NodeRegistration: nd}
	} else {
		kc.Spec.JoinConfiguration = &kubeadmv1beta1.JoinConfiguration{NodeRegistration: nd, ControlPlane: nil}
	}
	Expect(kindClient.Create(context.TODO(), kc)).To(Succeed())
}

func createMachine(namespace, name, clusterName, k8sVersion string) {
	fmt.Fprintf(GinkgoWriter, "Creating Machine %s/%s\n", namespace, name)
	machine := &capi.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				capi.MachineControlPlaneLabelName: "true",
				capi.MachineClusterLabelName:      clusterName,
			},
		},
		Spec: capi.MachineSpec{
			Bootstrap: capi.Bootstrap{
				ConfigRef: &corev1.ObjectReference{
					Kind:       "KubeadmConfig",
					APIVersion: bootstrapv1.GroupVersion.String(),
					Name:       name,
					Namespace:  namespace,
				},
			},
			InfrastructureRef: corev1.ObjectReference{
				Kind:       "AzureMachine",
				APIVersion: capz.GroupVersion.String(),
				Name:       name,
				Namespace:  namespace,
			},
			Version: &k8sVersion,
		},
	}
	Expect(kindClient.Create(context.TODO(), machine)).To(Succeed())
}

func ensureMachine(namespace, name string) {
	fmt.Fprintf(GinkgoWriter, "Ensuring control plane initialized for cluster %s/%s\n", namespace, name)
	Eventually(
		func() (bool, error) {
			ns := apimachinerytypes.NamespacedName{Namespace: namespace, Name: name}
			machine := &capz.AzureMachine{}
			if err := kindClient.Get(context.TODO(), ns, machine); err != nil {
				return false, err
			}
			return machine.Status.Ready, nil
		},
		5*time.Minute, 15*time.Second,
	).Should(BeTrue(), fmt.Sprintf("You run out of time AzureMachine %s", name))

	fmt.Fprintf(GinkgoWriter, "Ensuring control plane initialized for cluster %s/%s\n", namespace, name)
	Eventually(
		func() (bool, error) {
			ns := apimachinerytypes.NamespacedName{Namespace: namespace, Name: name}
			bootstrap := &bootstrapv1.KubeadmConfig{}
			if err := kindClient.Get(context.TODO(), ns, bootstrap); err != nil {
				return false, err
			}
			return bootstrap.Status.Ready, nil
		},
		5*time.Minute, 15*time.Second,
	).Should(BeTrue(), fmt.Sprintf("You run out of time KubeadmConfig %s", name))

	fmt.Fprintf(GinkgoWriter, "Ensuring control plane initialized for cluster %s/%s\n", namespace, name)
	Eventually(
		func() (bool, error) {
			ns := apimachinerytypes.NamespacedName{Namespace: namespace, Name: name}
			machine := &capi.Machine{}
			if err := kindClient.Get(context.TODO(), ns, machine); err != nil {
				return false, err
			}
			return machine.Status.Phase == "provisioned", nil
		},
		5*time.Minute, 15*time.Second,
	).Should(BeTrue(), fmt.Sprintf("You run out of time Machine %s", name))
}

func deleteCluster(namespace, clusterName string) {
	fmt.Fprintf(GinkgoWriter, "Deleting Cluster named %q\n", clusterName)
	Expect(kindClient.Delete(context.TODO(), &capi.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      clusterName,
		},
	})).To(BeNil())
}

func ensureDeleted(namespace, clusterName string) {
	Eventually(
		func() *capi.Cluster {
			ns := apimachinerytypes.NamespacedName{Namespace: namespace, Name: clusterName}
			cluster := &capi.Cluster{}
			if err := kindClient.Get(context.TODO(), ns, cluster); err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				return &capi.Cluster{}
			}
			return cluster
		},
		20*time.Minute, 15*time.Second,
	).Should(BeNil())
}

func watchEvents(ctx context.Context, namespace string) {
	logFile := path.Join("examples", "resources", namespace, "events.log")
	fmt.Fprintf(GinkgoWriter, "Creating directory: %s\n", filepath.Dir(logFile))
	Expect(os.MkdirAll(filepath.Dir(logFile), 0755)).To(Succeed())

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()

	informerFactory := informers.NewSharedInformerFactoryWithOptions(
		clientSet,
		10*time.Minute,
		informers.WithNamespace(namespace),
	)
	eventInformer := informerFactory.Core().V1().Events().Informer()
	eventInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			e := obj.(*corev1.Event)
			f.WriteString(fmt.Sprintf("[New Event] %s/%s\n\tresource: %s/%s/%s\n\treason: %s\n\tmessage: %s\n\tfull: %#v\n",
				e.Namespace, e.Name, e.InvolvedObject.APIVersion, e.InvolvedObject.Kind, e.InvolvedObject.Name, e.Reason, e.Message, e))
		},
		UpdateFunc: func(_, obj interface{}) {
			e := obj.(*corev1.Event)
			f.WriteString(fmt.Sprintf("[Updated Event] %s/%s\n\tresource: %s/%s/%s\n\treason: %s\n\tmessage: %s\n\tfull: %#v\n",
				e.Namespace, e.Name, e.InvolvedObject.APIVersion, e.InvolvedObject.Kind, e.InvolvedObject.Name, e.Reason, e.Message, e))
		},
		DeleteFunc: func(obj interface{}) {},
	})

	stopInformer := make(chan struct{})
	defer close(stopInformer)
	informerFactory.Start(stopInformer)
	<-ctx.Done()
	stopInformer <- struct{}{}
}

func azureJson(cn string) string {
	return fmt.Sprintf(`
{
    "cloud": "AzurePublicCloud",
    "tenantId": "%s",
    "subscriptionId": "%s",
    "aadClientId": "%s",
    "aadClientSecret": "%s",
    "resourceGroup": "%s",
    "securityGroupName": "%s-controlplane-nsg",
    "location": "westus2",
    "vmType": "standard",
    "vnetName": "%s",
    "vnetResourceGroup": "%s",
    "subnetName": "%s-controlplane-subnet",
    "routeTableName": "%s-node-routetable",
    "userAssignedID": "%s",
    "loadBalancerSku": "standard",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": false,
    "useInstanceMetadata": true
}`, TenantID, SubscriptionID, ClientID, ClientSecret, cn, cn, cn, cn, cn, cn, cn)
}

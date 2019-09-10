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
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	kubessh "k8s.io/kubernetes/pkg/ssh"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	capz "sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators/machine"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloudtest"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/config"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/util/kind"
	capi "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clientset "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset"
	"sigs.k8s.io/cluster-api/pkg/util"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	capzProviderNamespace = "azure-provider-system"
	capzStatefulSetName   = "azure-provider-controller-manager"
	setupTimeout          = 10 * 60
)

var (
	kindClient crclient.Client
	testConfig *config.Config
)

var _ = Describe("Azure", func() {
	var (
		kindCluster kind.Cluster
		client      *clientset.Clientset
	)
	BeforeEach(func() {
		var err error
		testConfig, err = config.ParseConfig()
		Expect(err).To(BeNil())
		fmt.Fprintf(GinkgoWriter, "Running in Azure location: %s\n", testConfig.Location)
		fmt.Fprintf(GinkgoWriter, "Setting up kind cluster\n")
		kindCluster = kind.Cluster{
			Name: "capz-test-" + util.RandomString(6),
		}
		kindCluster.Setup()

		cfg := kindCluster.RestConfig()
		client, err = clientset.NewForConfig(cfg)
		Expect(err).To(BeNil())
	}, setupTimeout)

	AfterEach(func() {
		fmt.Fprintf(GinkgoWriter, "Tearing down kind cluster\n")
		kindCluster.Teardown()
	})

	Describe("control plane node", func() {
		It("should be running", func() {
			namespace := "test-" + util.RandomString(6)
			createNamespace(kindCluster.KubeClient(), namespace)

			fmt.Fprintf(GinkgoWriter, "ClusterConfigPath: %s\n", testConfig.ClusterConfigPath)
			fmt.Fprintf(GinkgoWriter, "Location: %s\n", testConfig.Location)

			By("Creating a Cluster resource")
			clusterName := "capz-e2e-" + util.RandomString(6)
			fmt.Fprintf(GinkgoWriter, "Creating Cluster named %q\n", clusterName)
			clusterapi := client.ClusterV1alpha1().Clusters(namespace)
			_, err := clusterapi.Create(makeClusterFromConfig(clusterName))
			Expect(err).To(BeNil())

			By("Creating a machine")
			machineName := clusterName + "-cp-0"
			fmt.Fprintf(GinkgoWriter, "Creating Machine named %q for Cluster %q\n", machineName, clusterName)
			machineapi := client.ClusterV1alpha1().Machines(namespace)
			_, err = machineapi.Create(makeControlPlaneMachineFromConfig(machineName, clusterName))
			Expect(err).To(BeNil())

			// Make sure that the Machine eventually reports that the VM state is running
			fmt.Fprintf(GinkgoWriter, "Ensuring first control plane Machine is ready\n")
			Eventually(
				func() (*capz.AzureMachineProviderStatus, error) {
					machine, err := machineapi.Get(machineName, metav1.GetOptions{})
					if err != nil {
						return nil, err
					}
					if machine.Status.ProviderStatus == nil {
						return &capz.AzureMachineProviderStatus{
							VMState: &capz.VMStateCreating,
						}, nil
					}
					return capz.MachineStatusFromProviderStatus(machine.Status.ProviderStatus)
				},
				10*time.Minute, 15*time.Second,
			).Should(beHealthy())

			// Make sure that the Machine eventually reports that the Machine NodeRef is set
			fmt.Fprintf(GinkgoWriter, "Ensuring first control plane Machine NodeRef is set\n")
			Eventually(
				func() (*corev1.ObjectReference, error) {
					machine, err := machineapi.Get(machineName, metav1.GetOptions{})
					if err != nil {
						return nil, err
					}
					return machine.Status.NodeRef, nil

				},
				10*time.Minute, 15*time.Second,
			).ShouldNot(BeNil())

			// Make sure that the Cluster reports the Control Plane is ready
			fmt.Fprintf(GinkgoWriter, "Ensuring Cluster reports the Control Plane is ready\n")
			Eventually(
				func() (map[string]string, error) {
					cluster, err := clusterapi.Get(clusterName, metav1.GetOptions{})
					if err != nil {
						return nil, err
					}
					return cluster.Annotations, nil
				},
				10*time.Minute, 15*time.Second,
			).Should(HaveKeyWithValue(capz.AnnotationControlPlaneReady, capz.ValueReady))

			// TODO: Retrieve Cluster kubeconfig
			// TODO: Deploy Addons
			// TODO: Validate Node Ready
			// TODO: Deploy additional Control Plane Nodes
			// TODO: Deploy a MachineDeployment
			// TODO: Scale MachineDeployment up
			// TODO: Scale MachineDeployment down

			By("Deleting cluster")
			fmt.Fprintf(GinkgoWriter, "Deleting Cluster named %q\n", clusterName)
			Expect(kindClient.Delete(context.TODO(), &capi.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      clusterName,
				},
			}, noOptionsDelete())).To(BeNil())

			Eventually(
				func() *capi.Cluster {
					cluster, err := clusterapi.Get(clusterName, metav1.GetOptions{})
					if err != nil {
						if apierrors.IsNotFound(err) {
							return nil
						}
						return &capi.Cluster{}
					}
					return cluster
				},
				20*time.Minute, 15*time.Second,
			).Should(BeNil())
		})
	})
})

func noOptionsDelete() crclient.DeleteOptionFunc {
	return func(opts *crclient.DeleteOptions) {}
}

func beHealthy() types.GomegaMatcher {
	return PointTo(
		MatchFields(IgnoreExtras, Fields{
			"VMState": PointTo(Equal(capz.VMStateSucceeded)),
		}),
	)
}

func makeClusterFromConfig(name string) *capi.Cluster {
	wd, _ := os.Getwd()
	fmt.Fprintf(GinkgoWriter, "%s\n", wd)
	fmt.Fprintf(GinkgoWriter, "ClusterConfigPath: %s\n", testConfig.ClusterConfigPath)
	fmt.Fprintf(GinkgoWriter, "Location: %s\n", testConfig.Location)

	bytes, err := ioutil.ReadFile(testConfig.ClusterConfigPath)
	if err != nil {
		fmt.Fprintf(GinkgoWriter, "%s\n", err)
	}
	Expect(err).To(BeNil())

	deserializer := serializer.NewCodecFactory(getScheme()).UniversalDeserializer()
	cluster := &capi.Cluster{}
	obj, _, err := deserializer.Decode(bytes, nil, cluster)
	Expect(err).To(BeNil())
	cluster, ok := obj.(*capi.Cluster)
	Expect(ok).To(BeTrue(), "Wanted cluster, got %T", obj)

	cluster.ObjectMeta.Name = name

	azureSpec, err := capz.ClusterConfigFromProviderSpec(cluster.Spec.ProviderSpec)
	Expect(err).To(BeNil())
	azureSpec.ResourceGroup = name
	azureSpec.Location = testConfig.Location
	azureSpec.NetworkSpec.Vnet.Name = name + "-vnet"
	cluster.Spec.ProviderSpec.Value, err = capz.EncodeClusterSpec(azureSpec)
	Expect(err).To(BeNil())

	return cluster
}

func makeControlPlaneMachineFromConfig(name, clusterName string) *capi.Machine {
	bytes, err := ioutil.ReadFile(testConfig.MachineConfigPath)
	Expect(err).To(BeNil())

	deserializer := serializer.NewCodecFactory(getScheme()).UniversalDeserializer()
	obj, _, err := deserializer.Decode(bytes, nil, &capi.MachineList{})
	Expect(err).To(BeNil())
	machineList, ok := obj.(*capi.MachineList)
	Expect(ok).To(BeTrue(), "Wanted MachineList, got %T", obj)

	machines := machine.GetControlPlaneMachines(machineList)
	Expect(machines).NotTo(BeEmpty())

	machine := machines[0]
	machine.ObjectMeta.Name = name
	machine.ObjectMeta.Labels[capi.MachineClusterLabelName] = clusterName
	machine.Spec.Versions.Kubelet = testConfig.KubernetesVersion
	machine.Spec.Versions.ControlPlane = testConfig.KubernetesVersion

	azureSpec, err := actuators.MachineConfigFromProviderSpec(nil, machine.Spec.ProviderSpec, &cloudtest.Log{})
	Expect(err).To(BeNil())
	azureSpec.Location = testConfig.Location
	if testConfig.PublicSSHKey != "" {
		azureSpec.SSHPublicKey = testConfig.PublicSSHKey
	} else {
		publicKey, _, err := genKeyPairs()
		Expect(err).To(BeNil())
		azureSpec.SSHPublicKey = base64.StdEncoding.EncodeToString(publicKey)
	}

	machine.Spec.ProviderSpec.Value, err = capz.EncodeMachineSpec(azureSpec)
	Expect(err).To(BeNil())

	return machine
}

func createNamespace(client kubernetes.Interface, namespace string) {
	_, err := client.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	})
	Expect(err).To(BeNil())
}

func getAzureAuthorizer() autorest.Authorizer {
	// create an authorizer from env vars or Azure Managed Service Idenity
	authorizer, err := auth.NewAuthorizerFromEnvironment()
	Expect(err).To(BeNil())
	return authorizer
}

func getScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	capi.SchemeBuilder.AddToScheme(s)
	capz.SchemeBuilder.AddToScheme(s)
	return s
}

func genKeyPairs() (publicKey []byte, privateKey []byte, err error) {
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	public, err := ssh.NewPublicKey(&private.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	publicKeyBytes := ssh.MarshalAuthorizedKey(public)
	privateKeyBytes := kubessh.EncodePrivateKey(private)

	return publicKeyBytes, privateKeyBytes, err
}

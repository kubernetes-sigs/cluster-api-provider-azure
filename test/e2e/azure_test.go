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

package e2e_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	bootstrapv1 "sigs.k8s.io/cluster-api-bootstrap-provider-kubeadm/api/v1alpha2"
	kubeadmv1beta1 "sigs.k8s.io/cluster-api-bootstrap-provider-kubeadm/kubeadm/v1beta1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/framework"
	capiv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/cluster-api/util"
)

var _ = Describe("CAPZ e2e tests", func() {
	Describe("Cluster creation", func() {

		clusterGen := &ClusterGenerator{}
		nodeGen := &NodeGenerator{}

		Context("Create one controlplane cluster", func() {
			It("Should create a single node cluster and delete it before exiting", func() {
				cluster, infraCluster := clusterGen.GenerateCluster(namespace)
				node := nodeGen.GenerateNode(cluster.GetName())
				OneNodeCluster(&OneNodeClusterInput{
					Management:    mgmt,
					Cluster:       cluster,
					InfraCluster:  infraCluster,
					Node:          node,
					CreateTimeout: 60 * time.Minute,
				})
				framework.CleanUp(&framework.CleanUpInput{
					Management: mgmt,
					Cluster:    cluster,
				})
			})
		})

		// TODO: Deploy multiple Control Plane
		// TODO: Deploy Addons
		// TODO: Deploy MachineDeployments
		// TODO: Scale up
		// TODO: Scale down
	})
})

type ClusterGenerator struct{}

func (c *ClusterGenerator) GenerateCluster(namespace string) (*capiv1.Cluster, *infrav1.AzureCluster) {
	name := "capz-" + util.RandomString(6)
	vnetName := name + "-vnet"
	infraCluster := &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: infrav1.AzureClusterSpec{
			Location:      location,
			ResourceGroup: name,
			NetworkSpec: infrav1.NetworkSpec{
				Vnet: infrav1.VnetSpec{Name: vnetName},
			},
		},
	}
	cluster := &capiv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: capiv1.ClusterSpec{
			ClusterNetwork: &capiv1.ClusterNetwork{
				Pods: &capiv1.NetworkRanges{CIDRBlocks: []string{"192.168.0.0/16"}},
			},
			InfrastructureRef: &corev1.ObjectReference{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       framework.TypeToKind(infraCluster),
				Namespace:  infraCluster.GetNamespace(),
				Name:       infraCluster.GetName(),
			},
		},
	}
	return cluster, infraCluster
}

type NodeGenerator struct {
	counter int
}

func (n *NodeGenerator) GenerateNode(clusterName string) framework.Node {
	sshkey, err := sshkey()
	Expect(err).NotTo(HaveOccurred())

	firstControlPlane := n.counter == 0
	name := fmt.Sprintf("%s-controlplane-%d", clusterName, n.counter)
	n.counter++

	infraMachine := &infrav1.AzureMachine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: infrav1.AzureMachineSpec{
			VMSize:       vmSize,
			Location:     location,
			SSHPublicKey: sshkey,
			Image: &infrav1.Image{
				Offer:     &imageOffer,
				Publisher: &imagePublisher,
				SKU:       &imageSKU,
				Version:   &imageVersion,
			},
			OSDisk: infrav1.OSDisk{
				DiskSizeGB: 30,
				OSType:     "Linux",
				ManagedDisk: infrav1.ManagedDisk{
					StorageAccountType: "Premium_LRS",
				},
			},
		},
	}
	bootstrapConfig := &bootstrapv1.KubeadmConfig{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: bootstrapv1.KubeadmConfigSpec{
			Files: []bootstrapv1.File{
				{
					Owner:       "root:root",
					Path:        "/etc/kubernetes/azure.json",
					Permissions: "0644",
					Content:     cloudConfig(clusterName),
				},
			},
			InitConfiguration: &kubeadmv1beta1.InitConfiguration{},
			JoinConfiguration: &kubeadmv1beta1.JoinConfiguration{},
		},
	}
	registrationOptions := kubeadmv1beta1.NodeRegistrationOptions{
		Name: "{{ ds.meta_data[\"local_hostname\"] }}",
		KubeletExtraArgs: map[string]string{
			"cloud-provider": "azure",
			"cloud-config":   "/etc/kubernetes/azure.json",
		},
	}

	if firstControlPlane {
		cpInitConfiguration(bootstrapConfig, registrationOptions)
	} else {
		cpJoinConfiguration(bootstrapConfig, registrationOptions)
	}

	machine := &capiv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels: map[string]string{
				capiv1.MachineControlPlaneLabelName: "true",
				capiv1.MachineClusterLabelName:      clusterName,
			},
		},
		Spec: capiv1.MachineSpec{
			Bootstrap: capiv1.Bootstrap{
				ConfigRef: &corev1.ObjectReference{
					APIVersion: bootstrapv1.GroupVersion.String(),
					Kind:       framework.TypeToKind(bootstrapConfig),
					Namespace:  bootstrapConfig.GetNamespace(),
					Name:       bootstrapConfig.GetName(),
				},
			},
			InfrastructureRef: corev1.ObjectReference{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       framework.TypeToKind(infraMachine),
				Namespace:  infraMachine.GetNamespace(),
				Name:       infraMachine.GetName(),
			},
			Version: &k8sVersion,
		},
	}
	return framework.Node{
		Machine:         machine,
		InfraMachine:    infraMachine,
		BootstrapConfig: bootstrapConfig,
	}
}

func cloudConfig(cn string) string {
	// TODO This is ugly
	return fmt.Sprintf(`{
    "cloud": "AzurePublicCloud",
    "tenantId": "%s",
    "subscriptionId": "%s",
    "aadClientId": "%s",
    "aadClientSecret": "%s",
    "resourceGroup": "%s",
    "securityGroupName": "%s-controlplane-nsg",
    "location": "westus2",
    "vmType": "standard",
    "vnetName": "%s-vnet",
    "vnetResourceGroup": "%s",
    "subnetName": "%s-controlplane-subnet",
    "routeTableName": "%s-node-routetable",
    "userAssignedID": "%s",
    "loadBalancerSku": "standard",
    "maximumLoadBalancerRuleCount": 250,
    "useManagedIdentityExtension": false,
    "useInstanceMetadata": true
}`, creds.TenantID, creds.SubscriptionID, creds.ClientID, creds.ClientSecret, cn, cn, cn, cn, cn, cn, cn)
}

func sshkey() (string, error) {
	// TODO Load from AZURE_SSH_PUBLIC_KEY_FILE if set
	prv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", errors.Wrap(err, "Failed to generate private key")
	}
	pub, err := ssh.NewPublicKey(&prv.PublicKey)
	if err != nil {
		return "", errors.Wrap(err, "Failed to generate public key")
	}
	return base64.StdEncoding.EncodeToString(ssh.MarshalAuthorizedKey(pub)), nil
}

func cpInitConfiguration(kubeadmConfig *bootstrapv1.KubeadmConfig, registrationOptions kubeadmv1beta1.NodeRegistrationOptions) {
	kubeadmConfig.Spec.ClusterConfiguration = &kubeadmv1beta1.ClusterConfiguration{
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
			TimeoutForControlPlane: &metav1.Duration{Duration: 20 * time.Minute},
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
	kubeadmConfig.Spec.InitConfiguration = &kubeadmv1beta1.InitConfiguration{NodeRegistration: registrationOptions}
}

func cpJoinConfiguration(kubeadmConfig *bootstrapv1.KubeadmConfig, registrationOptions kubeadmv1beta1.NodeRegistrationOptions) {
	kubeadmConfig.Spec.JoinConfiguration = &kubeadmv1beta1.JoinConfiguration{NodeRegistration: registrationOptions, ControlPlane: nil}
}

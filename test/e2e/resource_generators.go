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
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	random "math/rand"
	"os"
	"time"

	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/auth"
	capiv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	bootstrapv1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha3"
	kubeadmv1beta1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/types/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1alpha3"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util"
)

type ClusterGenerator struct{}

type azureConfig struct {
	Cloud                        string `json:"cloud"`
	TenantID                     string `json:"tenantID"`
	SubscriptionID               string `json:"subscriptionId"`
	AADClientID                  string `json:"aadClientId"`
	AADClientSecret              string `json:"aadClientSecret"`
	ResourceGroup                string `json:"resourceGroup"`
	SecurityGroupName            string `json:"securityGroupName"`
	Location                     string `json:"location"`
	VMType                       string `json:"vmType"`
	VnetName                     string `json:"vnetName"`
	VnetResourceGroup            string `json:"vnetResourceGroup"`
	SubnetName                   string `json:"subnetName"`
	RouteTableName               string `json:"routeTableName"`
	UserAssignedID               string `json:"userAssignedID"`
	LoadBalancerSku              string `json:"loadBalancerSku"`
	MaximumLoadBalancerRuleCount int    `json:"maximumLoadBalancerRuleCount"`
	UseManagedIdentityExtension  bool   `json:"useManagedIdentityExtension"`
	UseInstanceMetadata          bool   `json:"useInstanceMetadata"`
}

var (
	location       string
	vmSize         string
	namespace      string
	imageOffer     string
	imagePublisher string
	imageSKU       string
	imageVersion   string
)

func (c *ClusterGenerator) VariablesInit() {

	v := viper.New()

	v.SetDefault("location", GetRegion())
	v.SetDefault("vmSize", "Standard_D2s_v3")
	v.SetDefault("namespace", "default")
	v.SetDefault("imageOffer", "capi")
	v.SetDefault("imagePublisher", "cncf-upstream")
	v.SetDefault("imageSKU", "k8s-1dot16dot6-ubuntu-1804")
	v.SetDefault("imageVersion", "latest")

	v.AddConfigPath(".")
	v.AutomaticEnv()

	v.SetConfigFile(".env")
	err := v.ReadInConfig()
	if err != nil {
		log.Printf("Error while reading config file .env. Trying config-capz yaml file. Error: %s", err.Error())
		v.SetConfigName("config-capz")
		v.SetConfigType("yaml")
		err = v.ReadInConfig()
		if err != nil {
			log.Printf("Error reading the config-capz file. Using the default values or values set via environment variables. Error: %s", err.Error())
		}
	}

	location = v.GetString("location")
	vmSize = v.GetString("vmSize")
	namespace = v.GetString("namespace")
	imageOffer = v.GetString("imageOffer")
	imagePublisher = v.GetString("imagePublisher")
	imageSKU = v.GetString("imageSKU")
	imageVersion = v.GetString("imageVersion")
}

func (c *ClusterGenerator) GenerateCluster(namespace string) (*capiv1.Cluster, *infrav1.AzureCluster) {
	name := "capz-e2e" + util.RandomString(6)
	vnetName := name + "-vnet"
	tags := map[string]string{
		"creationTimestamp": time.Now().UTC().Format(time.RFC3339),
		"jobName":           "cluster-api-provider-azure",
	}
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
			AdditionalTags: tags,
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
			ControlPlaneRef: &corev1.ObjectReference{
				APIVersion: controlplanev1.GroupVersion.String(),
				Kind:       "KubeadmControlPlane",
				Namespace:  namespace,
				Name:       fmt.Sprintf("%s-control-plane", name),
			},
		},
	}
	return cluster, infraCluster
}

type NodeGenerator struct {
	counter int
}

func (n *NodeGenerator) GenerateMachineTemplate(creds auth.Creds, clusterName string) runtime.Object {
	sshkey, err := sshkey()
	Expect(err).NotTo(HaveOccurred())

	return &infrav1.AzureMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      fmt.Sprintf("%s-control-plane", clusterName),
		},
		Spec: infrav1.AzureMachineTemplateSpec{
			Template: infrav1.AzureMachineTemplateResource{
				Spec: infrav1.AzureMachineSpec{
					VMSize:       vmSize,
					Location:     location,
					SSHPublicKey: sshkey,
					Image: &infrav1.Image{
						Marketplace: &infrav1.AzureMarketplaceImage{
							Publisher: imagePublisher,
							Offer:     imageOffer,
							SKU:       imageSKU,
							Version:   imageVersion,
						},
					},
					OSDisk: infrav1.OSDisk{
						DiskSizeGB: 128,
						OSType:     "Linux",
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "Premium_LRS",
						},
					},
				},
			},
		},
	}
}

func (n *NodeGenerator) GenerateKubeadmControlplane(creds auth.Creds, clusterName string, controlplaneMachineCount int32) *controlplanev1.KubeadmControlPlane {
	defaultConfig, _ := framework.DefaultConfig()
	defaultConfig.Defaults()

	controlplane := controlplanev1.KubeadmControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      fmt.Sprintf("%s-control-plane", clusterName),
		},
		Spec: controlplanev1.KubeadmControlPlaneSpec{
			Replicas: pointer.Int32Ptr(controlplaneMachineCount),
			InfrastructureTemplate: corev1.ObjectReference{
				Kind:       "AzureMachineTemplate",
				APIVersion: infrav1.GroupVersion.String(),
				Name:       fmt.Sprintf("%s-control-plane", clusterName),
			},
			KubeadmConfigSpec: bootstrapv1.KubeadmConfigSpec{
				UseExperimentalRetryJoin: true,
				Files: []bootstrapv1.File{
					{
						Owner:       "root:root",
						Path:        "/etc/kubernetes/azure.json",
						Permissions: "0644",
						Content:     cloudConfig(clusterName, creds),
					},
				},
				InitConfiguration: &kubeadmv1beta1.InitConfiguration{
					NodeRegistration: kubeadmv1beta1.NodeRegistrationOptions{
						Name: `{{ ds.meta_data["local_hostname"] }}`,
						KubeletExtraArgs: map[string]string{
							"cloud-provider": "azure",
							"cloud-config":   "/etc/kubernetes/azure.json",
						},
					},
				},
				JoinConfiguration: &kubeadmv1beta1.JoinConfiguration{
					NodeRegistration: kubeadmv1beta1.NodeRegistrationOptions{
						Name: `{{ ds.meta_data["local_hostname"] }}`,
						KubeletExtraArgs: map[string]string{
							"cloud-provider": "azure",
							"cloud-config":   "/etc/kubernetes/azure.json",
						},
					},
				},
				ClusterConfiguration: &kubeadmv1beta1.ClusterConfiguration{
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
				},
			},
			Version: defaultConfig.KubernetesVersion,
		},
	}
	return &controlplane
}

func cloudConfig(clusterName string, creds auth.Creds) string {
	config := azureConfig{
		Cloud:                        "AzurePublicCloud",
		TenantID:                     creds.TenantID,
		SubscriptionID:               creds.SubscriptionID,
		AADClientID:                  creds.ClientID,
		AADClientSecret:              creds.ClientSecret,
		ResourceGroup:                clusterName,
		SecurityGroupName:            clusterName + "-controlplane-nsg",
		Location:                     "westus2",
		VMType:                       "standard",
		VnetName:                     clusterName + "-vnet",
		VnetResourceGroup:            clusterName,
		SubnetName:                   clusterName + "-controlplane-subnet",
		RouteTableName:               clusterName + "-node-routetable",
		UserAssignedID:               clusterName,
		LoadBalancerSku:              "standard",
		MaximumLoadBalancerRuleCount: 250,
		UseManagedIdentityExtension:  false,
		UseInstanceMetadata:          true,
	}
	res, _ := json.Marshal(config)
	return string(res)
}

func sshkey() (string, error) {
	var pub ssh.PublicKey

	if os.Getenv("AZURE_SSH_PUBLIC_KEY_FILE") != "" {
		authorizedKeysBytes, err := ioutil.ReadFile(os.Getenv("AZURE_SSH_PUBLIC_KEY_FILE"))
		if err != nil {
			return "", errors.Wrap(err, "Failed to load public key provided via environment variable")
		}

		// double checking if the public key provided is valid
		pub, _, _, _, err = ssh.ParseAuthorizedKey(authorizedKeysBytes)
		if err != nil {
			return "", errors.Wrap(err, "Failed to parse public key provided via environment variable")
		}
	} else {
		prv, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return "", errors.Wrap(err, "Failed to generate private key")
		}
		pub, err = ssh.NewPublicKey(&prv.PublicKey)
		if err != nil {
			return "", errors.Wrap(err, "Failed to generate public key")
		}
	}

	return base64.StdEncoding.EncodeToString(ssh.MarshalAuthorizedKey(pub)), nil
}

// MachineDeploymentGenerator may be used to generate the resources
// required to create a machine deployment for testing.
type MachineDeploymentGenerator struct {
	counter int
}

// Generate returns the resources required to create a machine deployment.
func (n *MachineDeploymentGenerator) Generate(creds auth.Creds, namespace string, clusterName string, replicas int32) framework.MachineDeployment {
	sshkey, err := sshkey()
	Expect(err).NotTo(HaveOccurred())
	generatedName := fmt.Sprintf("%s-%d", clusterName, n.counter)

	defaultConfig, _ := framework.DefaultConfig()
	defaultConfig.Defaults()

	infraMachineTemplate := &infrav1.AzureMachineTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      generatedName,
		},
		Spec: infrav1.AzureMachineTemplateSpec{
			Template: infrav1.AzureMachineTemplateResource{
				Spec: infrav1.AzureMachineSpec{
					VMSize:       vmSize,
					Location:     location,
					SSHPublicKey: sshkey,
					Image: &infrav1.Image{
						Marketplace: &infrav1.AzureMarketplaceImage{
							Publisher: imagePublisher,
							Offer:     imageOffer,
							SKU:       imageSKU,
							Version:   imageVersion,
						},
					},
					OSDisk: infrav1.OSDisk{
						DiskSizeGB: 30,
						OSType:     "Linux",
						ManagedDisk: infrav1.ManagedDisk{
							StorageAccountType: "Premium_LRS",
						},
					},
				},
			},
		},
	}

	bootstrapConfigTemplate := &bootstrapv1.KubeadmConfigTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      generatedName,
		},
		Spec: bootstrapv1.KubeadmConfigTemplateSpec{
			Template: bootstrapv1.KubeadmConfigTemplateResource{
				Spec: bootstrapv1.KubeadmConfigSpec{
					JoinConfiguration: &kubeadmv1beta1.JoinConfiguration{
						NodeRegistration: kubeadmv1beta1.NodeRegistrationOptions{
							Name: `{{ ds.meta_data["local_hostname"] }}`,
							KubeletExtraArgs: map[string]string{
								"cloud-provider": "azure",
								"cloud-config":   "/etc/kubernetes/azure.json",
							},
						},
					},
					Files: []bootstrapv1.File{
						{
							Owner:       "root:root",
							Path:        "/etc/kubernetes/azure.json",
							Permissions: "0644",
							Content:     cloudConfig(clusterName, creds),
						},
					},
				},
			},
		},
	}

	machineDeployment := &capiv1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      generatedName,
		},
		Spec: capiv1.MachineDeploymentSpec{
			ClusterName: clusterName,
			Replicas:    &replicas,
			Template: capiv1.MachineTemplateSpec{
				Spec: capiv1.MachineSpec{
					Bootstrap: capiv1.Bootstrap{
						ConfigRef: &corev1.ObjectReference{
							APIVersion: bootstrapv1.GroupVersion.String(),
							Kind:       framework.TypeToKind(bootstrapConfigTemplate),
							Namespace:  bootstrapConfigTemplate.GetNamespace(),
							Name:       bootstrapConfigTemplate.GetName(),
						},
					},
					InfrastructureRef: corev1.ObjectReference{
						APIVersion: infrav1.GroupVersion.String(),
						Kind:       framework.TypeToKind(infraMachineTemplate),
						Namespace:  infraMachineTemplate.GetNamespace(),
						Name:       infraMachineTemplate.GetName(),
					},
					Version:     pointer.StringPtr(defaultConfig.KubernetesVersion),
					ClusterName: clusterName,
				},
			},
		},
	}

	return framework.MachineDeployment{
		MachineDeployment:       machineDeployment,
		BootstrapConfigTemplate: bootstrapConfigTemplate,
		InfraMachineTemplate:    infraMachineTemplate,
	}
}

// GetRegion gets a random region to use in the tests unless explicit region specified in env var
func GetRegion() string {
	region := os.Getenv("E2E_REGION")
	if region != "" {
		return region
	}

	regions := []string{"eastus", "eastus2", "southcentralus", "westus2", "westeurope"}
	log.Printf("Picking Random Region from list %s\n", regions)
	r := random.New(random.NewSource(time.Now().UnixNano()))
	location := regions[r.Intn(len(regions))]
	log.Printf("Picked Random Region:%s\n", location)
	return location
}

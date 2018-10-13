/*
Copyright 2018 The Kubernetes Authors.
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

package machine

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"time"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/golang/glog"
	"github.com/joho/godotenv"
	"github.com/platform9/azure-provider/cloud/azure/actuators/machine/machinesetup"
	"github.com/platform9/azure-provider/cloud/azure/actuators/machine/wrappers"
	azureconfigv1 "github.com/platform9/azure-provider/cloud/azure/providerconfig/v1alpha1"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/runtime"
	clustercommon "sigs.k8s.io/cluster-api/pkg/apis/cluster/common"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	client "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
)

// The Azure Client, also used as a machine actuator
type AzureClient struct {
	v1Alpha1Client           client.ClusterV1alpha1Interface
	SubscriptionID           string
	Authorizer               autorest.Authorizer
	kubeadmToken             string
	ctx                      context.Context
	scheme                   *runtime.Scheme
	azureProviderConfigCodec *azureconfigv1.AzureProviderConfigCodec
	machineSetupConfigs      machinesetup.MachineSetup
}

// Parameter object used to create a machine actuator.
// These are not indicative of all requirements for a machine actuator, environment variables are also necessary.
type MachineActuatorParams struct {
	V1Alpha1Client         client.ClusterV1alpha1Interface
	KubeadmToken           string
	MachineSetupConfigPath string
}

const (
	templateFile      = "deployment-template.json"
	ProviderName      = "azure"
	SSHUser           = "ClusterAPI"
	NameAnnotationKey = "azure-name"
	RGAnnotationKey   = "azure-rg"
)

func init() {
	actuator, err := NewMachineActuator(MachineActuatorParams{})
	if err != nil {
		glog.Fatalf("Error creating cluster provisioner for azure : %v", err)
	}
	clustercommon.RegisterClusterProvisioner(ProviderName, actuator)
}

// Creates a new azure client to be used as a machine actuator
func NewMachineActuator(params MachineActuatorParams) (*AzureClient, error) {
	scheme, azureProviderConfigCodec, err := azureconfigv1.NewSchemeAndCodecs()
	if err != nil {
		return nil, err
	}
	//Parse in environment variables if necessary
	if os.Getenv("AZURE_SUBSCRIPTION_ID") == "" {
		err = godotenv.Load()
		if err == nil && os.Getenv("AZURE_SUBSCRIPTION_ID") == "" {
			err = errors.New("AZURE_SUBSCRIPTION_ID: \"\"")
		}
		if err != nil {
			log.Fatalf("Failed to load environment variables: %v", err)
			return nil, err
		}
	}
	authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		log.Fatalf("Failed to get OAuth config: %v", err)
		return nil, err
	}
	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	if err != nil {
		return nil, err
	}
	return &AzureClient{
		v1Alpha1Client:           params.V1Alpha1Client,
		SubscriptionID:           subscriptionID,
		Authorizer:               authorizer,
		kubeadmToken:             params.KubeadmToken,
		ctx:                      context.Background(),
		scheme:                   scheme,
		azureProviderConfigCodec: azureProviderConfigCodec,
	}, nil
}

// Create a machine based on the cluster and machine spec passed
func (azure *AzureClient) Create(cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	clusterConfig, err := azure.azureProviderConfigCodec.ClusterProviderFromProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		return err
	}
	_, err = azure.createOrUpdateGroup(cluster)
	if err != nil {
		return err
	}
	_, err = azure.createOrUpdateDeployment(cluster, machine)
	if err != nil {
		return err
	}
	if machine.ObjectMeta.Annotations == nil {
		machine.ObjectMeta.Annotations = make(map[string]string)
	}
	if azure.v1Alpha1Client != nil {
		machine.ObjectMeta.Annotations[NameAnnotationKey] = machine.ObjectMeta.Name
		machine.ObjectMeta.Annotations[RGAnnotationKey] = clusterConfig.ResourceGroup
		azure.v1Alpha1Client.Machines(machine.Namespace).Update(machine)
	} else {
		glog.V(1).Info("ClusterAPI client not found, not updating machine object")
	}
	return nil
}

// Update an existing machine based on the cluster and machine spec passed.
// Currently only checks machine existence and does not update anything.
func (azure *AzureClient) Update(cluster *clusterv1.Cluster, goalMachine *clusterv1.Machine) error {
	//Parse in configurations
	_, err := azure.decodeMachineProviderConfig(goalMachine.Spec.ProviderConfig)
	if err != nil {
		return err
	}
	_, err = azure.azureProviderConfigCodec.ClusterProviderFromProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		return err
	}
	_, err = azure.vmIfExists(cluster, goalMachine)
	if err != nil {
		return err
	}
	// TODO: Update objects
	return nil
}

// Delete an existing machine based on the cluster and machine spec passed.
// Will block until the machine has been successfully deleted, or an error is returned.
func (azure *AzureClient) Delete(cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	//Parse in configurations
	_, err := azure.decodeMachineProviderConfig(machine.Spec.ProviderConfig)
	if err != nil {
		return err
	}
	clusterConfig, err := azure.azureProviderConfigCodec.ClusterProviderFromProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		return err
	}
	//Check if the machine exists
	vm, err := azure.vmIfExists(cluster, machine)
	if err != nil {
		return err
	}
	if vm == nil {
		//Skip deleting if we couldn't find anything to delete
		return nil
	}

	/*
		TODO: See if this is the last remaining machine, and if so,
		delete the resource group, which will automatically delete
		all associated resources
	*/

	groupsClient := wrappers.GetGroupsClient(azure.SubscriptionID)
	groupsClient.SetAuthorizer(azure.Authorizer)
	groupsDeleteFuture, err := groupsClient.Delete(azure.ctx, clusterConfig.ResourceGroup)
	if err != nil {
		return err
	}
	return groupsDeleteFuture.WaitForCompletion(azure.ctx, groupsClient.Client.BaseClient.Client)
}

// Get the kubeconfig of a machine based on the cluster and machine spec passed.
// Has not been fully tested as k8s is not yet bootstrapped on created machines.
func (azure *AzureClient) GetKubeConfig(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	_, err := azure.azureProviderConfigCodec.ClusterProviderFromProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		return "", err
	}
	machineConfig, err := azure.decodeMachineProviderConfig(machine.Spec.ProviderConfig)
	if err != nil {
		return "", err
	}

	decoded, err := base64.StdEncoding.DecodeString(machineConfig.SSHPrivateKey)
	privateKey := string(decoded)
	if err != nil {
		return "", err
	}

	ip, err := azure.GetIP(cluster, machine)
	if err != nil {
		return "", err
	}
	sshclient, err := GetSshClient(ip, privateKey)
	if err != nil {
		return "", fmt.Errorf("unable to get ssh client: %v", err)
	}
	sftpClient, err := sftp.NewClient(sshclient)
	if err != nil {
		return "", fmt.Errorf("Error setting sftp client: %s", err)
	}

	remoteFile := fmt.Sprintf("/home/%s/.kube/config", SSHUser)
	srcFile, err := sftpClient.Open(remoteFile)
	if err != nil {
		return "", fmt.Errorf("Error opening %s: %s", remoteFile, err)
	}

	defer srcFile.Close()
	dstFileName := "kubeconfig"
	dstFile, err := os.Create(dstFileName)
	if err != nil {
		return "", fmt.Errorf("unable to write local kubeconfig: %v", err)
	}

	defer dstFile.Close()
	srcFile.WriteTo(dstFile)

	content, err := ioutil.ReadFile(dstFileName)
	if err != nil {
		return "", fmt.Errorf("unable to read local kubeconfig: %v", err)
	}
	return string(content), nil
}

func GetSshClient(host string, privatekey string) (*ssh.Client, error) {
	key := []byte(privatekey)
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("unable to parse private key: %v", err)
	}
	config := &ssh.ClientConfig{
		User: SSHUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         60 * time.Second,
	}
	client, err := ssh.Dial("tcp", host+":22", config)
	return client, err
}

// Determine whether a machine exists based on the cluster and machine spec passed.
func (azure *AzureClient) Exists(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (bool, error) {
	rgExists, err := azure.checkResourceGroupExists(cluster)
	if err != nil {
		return false, err
	}
	if !rgExists {
		return false, nil
	}
	vm, err := azure.vmIfExists(cluster, machine)
	if err != nil {
		return false, err
	}
	return vm != nil, nil
}

func (azure *AzureClient) decodeMachineProviderConfig(providerConfig clusterv1.ProviderConfig) (*azureconfigv1.AzureMachineProviderConfig, error) {
	var config azureconfigv1.AzureMachineProviderConfig
	err := azure.azureProviderConfigCodec.DecodeFromProviderConfig(providerConfig, &config)
	if err != nil {
		return nil, err
	}
	return &config, err
}

func readJSON(path string) (*map[string]interface{}, error) {
	fileContents, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	data := make(map[string]interface{})
	err = json.Unmarshal(fileContents, &data)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

func base64EncodeCommand(command string) *string {
	encoded := base64.StdEncoding.EncodeToString([]byte(command))
	return &encoded
}

func (azure *AzureClient) convertMachineToDeploymentParams(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (*map[string]interface{}, error) {
	machineConfig, err := azure.decodeMachineProviderConfig(machine.Spec.ProviderConfig)
	if err != nil {
		return nil, err
	}
	startupScript, err := azure.getStartupScript(*machineConfig)
	if err != nil {
		return nil, err
	}
	decoded, err := base64.StdEncoding.DecodeString(machineConfig.SSHPublicKey)
	publicKey := string(decoded)
	if err != nil {
		return nil, err
	}
	params := map[string]interface{}{
		"virtualNetworks_ClusterAPIVM_vnet_name": map[string]interface{}{
			"value": "ClusterAPIVnet",
		},
		"virtualMachines_ClusterAPIVM_name": map[string]interface{}{
			"value": getVMName(machine),
		},
		"networkInterfaces_ClusterAPI_name": map[string]interface{}{
			"value": getNetworkInterfaceName(machine),
		},
		"publicIPAddresses_ClusterAPI_ip_name": map[string]interface{}{
			"value": getPublicIPName(machine),
		},
		"networkSecurityGroups_ClusterAPIVM_nsg_name": map[string]interface{}{
			"value": "ClusterAPINSG",
		},
		"subnets_default_name": map[string]interface{}{
			"value": "ClusterAPISubnet",
		},
		"securityRules_default_allow_ssh_name": map[string]interface{}{
			"value": "ClusterAPISSH",
		},
		"image_publisher": map[string]interface{}{
			"value": machineConfig.Image.Publisher,
		},
		"image_offer": map[string]interface{}{
			"value": machineConfig.Image.Offer,
		},
		"image_sku": map[string]interface{}{
			"value": machineConfig.Image.SKU,
		},
		"image_version": map[string]interface{}{
			"value": machineConfig.Image.Version,
		},
		"osDisk_name": map[string]interface{}{
			"value": getOSDiskName(machine),
		},
		"os_type": map[string]interface{}{
			"value": machineConfig.OSDisk.OSType,
		},
		"storage_account_type": map[string]interface{}{
			"value": machineConfig.OSDisk.ManagedDisk.StorageAccountType,
		},
		"disk_size_GB": map[string]interface{}{
			"value": machineConfig.OSDisk.DiskSizeGB,
		},
		"vm_user": map[string]interface{}{
			"value": "ClusterAPI",
		},
		"vm_size": map[string]interface{}{
			"value": machineConfig.VMSize,
		},
		"location": map[string]interface{}{
			"value": machineConfig.Location,
		},
		"startup_script": map[string]interface{}{
			"value": *base64EncodeCommand(startupScript),
		},
		"sshPublicKey": map[string]interface{}{
			"value": publicKey,
		},
	}
	return &params, nil
}

func parseMachineSetupConfig(path string) (*machinesetup.MachineSetup, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var machineSetupList *machinesetup.MachineSetup
	err = yaml.Unmarshal(data, machineSetupList)
	if err != nil {
		return nil, err
	}
	return machineSetupList, nil
}

// Get the startup script from the machine_set_configs, taking into account the role of the given machine
func (azure *AzureClient) getStartupScript(machineConfig azureconfigv1.AzureMachineProviderConfig) (string, error) {
	if machineConfig.Roles[0] == azureconfigv1.Master {
		const startupScript = `(
apt-get update
apt-get install -y docker.io
apt-get update && apt-get install -y apt-transport-https curl prips
curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
cat <<EOF >/etc/apt/sources.list.d/kubernetes.list
deb http://apt.kubernetes.io/ kubernetes-xenial main
EOF
apt-get update
apt-get install -y kubelet kubeadm kubectl

CLUSTER_DNS_SERVER=$(prips "10.96.0.0/12" | head -n 11 | tail -n 1)
CLUSTER_DNS_DOMAIN="cluster.local"
# Override network args to use kubenet instead of cni and override Kubelet DNS args.
cat > /etc/systemd/system/kubelet.service.d/20-kubenet.conf <<EOF
[Service]
Environment="KUBELET_NETWORK_ARGS=--network-plugin=kubenet"
Environment="KUBELET_DNS_ARGS=--cluster-dns=${CLUSTER_DNS_SERVER} --cluster-domain=${CLUSTER_DNS_DOMAIN}"
EOF
systemctl daemon-reload
systemctl restart kubelet.service

PORT=6443
PRIVATEIP=$(curl -H Metadata:true "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/privateIpAddress?api-version=2017-04-01&format=text")
PUBLICIP=$(curl -H Metadata:true "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress?api-version=2017-04-02&format=text")

# Set up kubeadm config file to pass parameters to kubeadm init.
cat > kubeadm_config.yaml <<EOF
apiVersion: kubeadm.k8s.io/v1alpha1
kind: MasterConfiguration
api:
  advertiseAddress: ${PUBLICIP}
  bindPort: ${PORT}
networking:
  serviceSubnet: "10.96.0.0/12"
token: "testtoken"
controllerManagerExtraArgs:
  cluster-cidr: "192.168.0.0/16"
  service-cluster-ip-range: "10.96.0.0/12"
  allocate-node-cidrs: "true"
EOF

# Create and set bridge-nf-call-iptables to 1 to pass the kubeadm preflight check.
# Workaround was found here:
# http://zeeshanali.com/sysadmin/fixed-sysctl-cannot-stat-procsysnetbridgebridge-nf-call-iptables/
modprobe br_netfilter

kubeadm init --config ./kubeadm_config.yaml

mkdir -p /home/ClusterAPI/.kube
cp -i /etc/kubernetes/admin.conf /home/ClusterAPI/.kube/config
chown $(id -u ClusterAPI):$(id -g ClusterAPI) /home/ClusterAPI/.kube/config

KUBECONFIG=/etc/kubernetes/admin.conf kubectl apply -f https://raw.githubusercontent.com/cloudnativelabs/kube-router/master/daemonset/kubeadm-kuberouter.yaml
) 2>&1 | tee /var/log/startup.log`
		return startupScript, nil
	} else if machineConfig.Roles[0] == azureconfigv1.Node {
		const startupScript = `(
apt-get update
apt-get install -y docker.io
apt-get update && apt-get install -y apt-transport-https curl prips
curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
cat <<EOF >/etc/apt/sources.list.d/kubernetes.list
deb http://apt.kubernetes.io/ kubernetes-xenial main
EOF
apt-get update
apt-get install -y kubelet kubeadm kubectl

CLUSTER_DNS_SERVER=$(prips "10.96.0.0/12" | head -n 11 | tail -n 1)
CLUSTER_DNS_DOMAIN="cluster.local"
# Override network args to use kubenet instead of cni and override Kubelet DNS args.
cat > /etc/systemd/system/kubelet.service.d/20-kubenet.conf <<EOF
[Service]
Environment="KUBELET_NETWORK_ARGS=--network-plugin=kubenet"
Environment="KUBELET_DNS_ARGS=--cluster-dns=${CLUSTER_DNS_SERVER} --cluster-domain=${CLUSTER_DNS_DOMAIN}"
EOF
systemctl daemon-reload
systemctl restart kubelet.service

kubeadm join --token "${TOKEN}" "${MASTER}" --ignore-preflight-errors=all --discovery-token-unsafe-skip-ca-verification
) 2>&1 | tee /var/log/startup.log`
		return startupScript, nil
	}
	return "", errors.New("unable to get startup script: unknown machine role")
}

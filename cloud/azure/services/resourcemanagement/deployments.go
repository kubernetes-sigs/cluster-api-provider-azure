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
package resourcemanagement

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"

	azureconfigv1 "github.com/platform9/azure-provider/cloud/azure/providerconfig/v1alpha1"
	"github.com/platform9/azure-provider/cloud/azure/services/network"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
)

const (
	templateFile = "deployment-template.json"
)

func (s *Service) CreateOrUpdateDeployment(machine *clusterv1.Machine, clusterConfig *azureconfigv1.AzureClusterProviderConfig, machineConfig *azureconfigv1.AzureMachineProviderConfig) (*resources.DeploymentsCreateOrUpdateFuture, error) {
	// Parse the ARM template
	template, err := readJSON(templateFile)
	if err != nil {
		return nil, err
	}
	params, err := convertMachineToDeploymentParams(machine, machineConfig)
	if err != nil {
		return nil, err
	}
	deployment := resources.Deployment{
		Properties: &resources.DeploymentProperties{
			Template:   template,
			Parameters: params,
			Mode:       resources.Incremental,
		},
	}

	deploymentFuture, err := s.DeploymentsClient.CreateOrUpdate(s.ctx, clusterConfig.ResourceGroup, machine.ObjectMeta.Name, deployment)
	if err != nil {
		return nil, err
	}
	return &deploymentFuture, nil
}
func (s *Service) ValidateDeployment(machine *clusterv1.Machine, clusterConfig *azureconfigv1.AzureClusterProviderConfig, machineConfig *azureconfigv1.AzureMachineProviderConfig) error {
	// Parse the ARM template
	template, err := readJSON(templateFile)
	if err != nil {
		return err
	}
	params, err := convertMachineToDeploymentParams(machine, machineConfig)
	if err != nil {
		return err
	}
	deployment := resources.Deployment{
		Properties: &resources.DeploymentProperties{
			Template:   template,
			Parameters: params,
			Mode:       resources.Incremental, // Do not delete and re-create matching resources that already exist
		},
	}
	res, err := s.DeploymentsClient.Validate(s.ctx, clusterConfig.ResourceGroup, machine.ObjectMeta.Name, deployment)
	if res.Error != nil {
		return errors.New(*res.Error.Message)
	}
	return err
}

func (s *Service) GetDeploymentResult(future resources.DeploymentsCreateOrUpdateFuture) (de resources.DeploymentExtended, err error) {
	return future.Result(s.DeploymentsClient)
}

func (s *Service) WaitForDeploymentsCreateOrUpdateFuture(future resources.DeploymentsCreateOrUpdateFuture) error {
	return future.WaitForCompletionRef(s.ctx, s.DeploymentsClient.Client)
}

func convertMachineToDeploymentParams(machine *clusterv1.Machine, machineConfig *azureconfigv1.AzureMachineProviderConfig) (*map[string]interface{}, error) {
	startupScript, err := getStartupScript(*machineConfig)
	if err != nil {
		return nil, err
	}
	decoded, err := base64.StdEncoding.DecodeString(machineConfig.SSHPublicKey)
	publicKey := string(decoded)
	if err != nil {
		return nil, err
	}
	params := map[string]interface{}{
		"clusterAPI_machine_name": map[string]interface{}{
			"value": machine.ObjectMeta.Name,
		},
		"virtualNetworks_ClusterAPIVM_vnet_name": map[string]interface{}{
			"value": network.VnetDefaultName,
		},
		"virtualMachines_ClusterAPIVM_name": map[string]interface{}{
			"value": GetVMName(machine),
		},
		"networkInterfaces_ClusterAPI_name": map[string]interface{}{
			"value": GetNetworkInterfaceName(machine),
		},
		"publicIPAddresses_ClusterAPI_ip_name": map[string]interface{}{
			"value": GetPublicIPName(machine),
		},
		"networkSecurityGroups_ClusterAPIVM_nsg_name": map[string]interface{}{
			"value": "ClusterAPINSG",
		},
		"subnets_default_name": map[string]interface{}{
			"value": network.SubnetDefaultName,
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
			"value": GetOSDiskName(machine),
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

// Get the startup script from the machine_set_configs, taking into account the role of the given machine
func getStartupScript(machineConfig azureconfigv1.AzureMachineProviderConfig) (string, error) {
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
apt-get install -y kubelet=1.11.3-00 kubeadm=1.11.3-00 kubectl=1.11.3-00 kubernetes-cni=0.6.0-00

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
PUBLICIP=$(curl -H Metadata:true "http://169.254.169.254/metadata/instance/network/interface/0/ipv4/ipAddress/0/publicIpAddress?api-version=2017-08-01&format=text")
# Set up kubeadm config file to pass parameters to kubeadm init.
cat > kubeadm_config.yaml <<EOF
apiVersion: kubeadm.k8s.io/v1alpha2
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
refactor-azure-clients-2apt-get install -y kubelet kubeadm kubectl

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

func GetPublicIPName(machine *clusterv1.Machine) string {
	return fmt.Sprintf("ClusterAPIIP-%s", machine.ObjectMeta.Name)
}

func GetNetworkInterfaceName(machine *clusterv1.Machine) string {
	return fmt.Sprintf("ClusterAPINIC-%s", GetVMName(machine))
}

func GetVMName(machine *clusterv1.Machine) string {
	return fmt.Sprintf("ClusterAPIVM-%s", machine.ObjectMeta.Name)
}

func GetOSDiskName(machine *clusterv1.Machine) string {
	return fmt.Sprintf("%s_OSDisk", GetVMName(machine))
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

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

package resources

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	"k8s.io/klog"
	providerv1 "sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/network"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

const (
	templateFile = "deployment-template.json"
)

// CreateOrUpdateDeployment is used to create or update a kubernetes cluster. It does so by creating or updating an ARM deployment.
func (s *Service) CreateOrUpdateDeployment(machine *clusterv1.Machine, clusterConfig *providerv1.AzureClusterProviderSpec, machineConfig *providerv1.AzureMachineProviderSpec, startupScript string) (*resources.DeploymentsCreateOrUpdateFuture, error) {
	// TODO: Remove debug
	klog.V(2).Info("CreateOrUpdateDeployment start")
	// Parse the ARM template
	// TODO: Remove debug
	klog.V(2).Info("CreateOrUpdateDeployment: reading template")
	template, err := readJSON(templateFile)
	if err != nil {
		// TODO: Remove debug
		klog.V(2).Info("CreateOrUpdateDeployment: could not read template")
		return nil, err
	}
	params, err := s.convertVMToDeploymentParams(machine, machineConfig, startupScript)
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

	deploymentFuture, err := s.scope.AzureClients.Deployments.CreateOrUpdate(s.scope.Context, clusterConfig.ResourceGroup, machine.ObjectMeta.Name, deployment)
	if err != nil {
		return nil, err
	}
	return &deploymentFuture, nil
}

// ValidateDeployment validates the parameters of the cluster by calling the ARM validate method.
func (s *Service) ValidateDeployment(machine *clusterv1.Machine, clusterConfig *providerv1.AzureClusterProviderSpec, machineConfig *providerv1.AzureMachineProviderSpec, startupScript string) error {
	// TODO: Remove debug
	klog.V(2).Info("ValidateDeployment start")
	// Parse the ARM template
	template, err := readJSON(templateFile)
	// TODO: Remove debug
	klog.V(2).Info("ValidateDeployment: reading template")
	if err != nil {
		return err
	}
	params, err := s.convertVMToDeploymentParams(machine, machineConfig, startupScript)
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
	res, err := s.scope.AzureClients.Deployments.Validate(s.scope.Context, clusterConfig.ResourceGroup, machine.ObjectMeta.Name, deployment)
	if res.Error != nil {
		return errors.New(*res.Error.Message)
	}
	return err
}

// GetDeploymentResult retrieves the result of the ARM deployment operation.
func (s *Service) GetDeploymentResult(future resources.DeploymentsCreateOrUpdateFuture) (de resources.DeploymentExtended, err error) {
	return future.Result(s.scope.AzureClients.Deployments)
}

// WaitForDeploymentsCreateOrUpdateFuture returns when the ARM operation completes.
func (s *Service) WaitForDeploymentsCreateOrUpdateFuture(future resources.DeploymentsCreateOrUpdateFuture) error {
	return future.WaitForCompletionRef(s.scope.Context, s.scope.AzureClients.Deployments.Client)
}

func (s *Service) convertVMToDeploymentParams(machine *clusterv1.Machine, machineConfig *providerv1.AzureMachineProviderSpec, startupScript string) (*map[string]interface{}, error) {
	// TODO: Remove debug
	klog.V(2).Info("convertVMToDeploymentParams start")
	decoded, err := base64.StdEncoding.DecodeString(machineConfig.SSHPublicKey)
	publicKey := string(decoded)
	if err != nil {
		// TODO: Remove debug
		klog.V(2).Info("convertVMToDeploymentParams: could not decode SSH key")
		return nil, err
	}

	// TODO: Allow parameterized value or set defaults
	params := map[string]interface{}{
		"clusterAPI_machine_name": map[string]interface{}{
			"value": machine.ObjectMeta.Name,
		},
		"virtualNetworks_ClusterAPIVM_vnet_name": map[string]interface{}{
			"value": network.VnetDefaultName,
		},
		"virtualMachines_ClusterAPIVM_name": map[string]interface{}{
			"value": s.GetVMName(machine),
		},
		"networkInterfaces_ClusterAPI_name": map[string]interface{}{
			"value": s.GetNetworkInterfaceName(machine),
		},
		"publicIPAddresses_ClusterAPI_ip_name": map[string]interface{}{
			"value": s.GetPublicIPName(machine),
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
			"value": s.GetOSDiskName(machine),
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
			"value": "capi",
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
func getStartupScript(machine *clusterv1.Machine, machineConfig providerv1.AzureMachineProviderSpec) (string, error) {
	if machineConfig.Roles[0] == providerv1.Master {
		startupScript := fmt.Sprintf(`(
apt-get update
apt-get install -y docker.io
apt-get update && apt-get install -y apt-transport-https curl prips
curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
cat <<EOF >/etc/apt/sources.list.d/kubernetes.list
deb http://apt.kubernetes.io/ kubernetes-xenial main
EOF
apt-get update
apt-get install -y kubelet=%[1]v-00 kubeadm=%[2]v-00 kubectl=%[2]v-00 kubernetes-cni=0.6.0-00

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
kubernetesVersion: v%[2]v
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
) 2>&1 | tee /var/log/startup.log`, machine.Spec.Versions.Kubelet, machine.Spec.Versions.ControlPlane)
		return startupScript, nil
	} else if machineConfig.Roles[0] == providerv1.Node {
		startupScript := fmt.Sprintf(`(
apt-get update
apt-get install -y docker.io
apt-get update && apt-get install -y apt-transport-https curl prips
curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key add -
cat <<EOF >/etc/apt/sources.list.d/kubernetes.list
deb http://apt.kubernetes.io/ kubernetes-xenial main
EOF
apt-get update
apt-get install -y kubelet=%[1]v-00 kubeadm=%[1]v-00 kubectl=%[1]v-00

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
) 2>&1 | tee /var/log/startup.log`, machine.Spec.Versions.Kubelet)
		return startupScript, nil
	}
	return "", errors.New("unable to get startup script: unknown machine role")
}

// GetVMName returns the VM resource name of the machine.
func (s *Service) GetVMName(machine *clusterv1.Machine) string {
	return fmt.Sprintf("%s-%s", s.scope.Name(), machine.ObjectMeta.Name)
}

// GetPublicIPName returns the public IP resource name of the machine.
// TODO: Move to network package
func (s *Service) GetPublicIPName(machine *clusterv1.Machine) string {
	return fmt.Sprintf("%s-pip", s.GetVMName(machine))
}

// GetNetworkInterfaceName returns the nic resource name of the machine.
func (s *Service) GetNetworkInterfaceName(machine *clusterv1.Machine) string {
	return fmt.Sprintf("%s-nic", s.GetVMName(machine))
}

// GetOSDiskName returns the OS disk resource name of the machine.
func (s *Service) GetOSDiskName(machine *clusterv1.Machine) string {
	return fmt.Sprintf("%s_OSDisk", s.GetVMName(machine))
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

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
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/ghodss/yaml"
	"github.com/joho/godotenv"
	"github.com/platform9/azure-provider/cloud/azure/actuators/machine/machinesetup"
	"github.com/platform9/azure-provider/cloud/azure/actuators/machine/wrappers"
	"github.com/platform9/azure-provider/cloud/azure/providerconfig/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
)

const (
	machineConfigFile = "testconfigs/machines.yaml"
)

func TestNewMachineActuator(t *testing.T) {
	params := MachineActuatorParams{KubeadmToken: "token"}
	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	if actuator.kubeadmToken != params.KubeadmToken {
		t.Fatalf("actuator.kubeadmToken != params.KubeadmToken: %v != %v", actuator.kubeadmToken, params.KubeadmToken)
	}
}

func TestCreate(t *testing.T) {
	clusterConfigFile := "testconfigs/cluster-ci-create.yaml"
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files :%v", err)
	}
	azure, err := NewMachineActuator(MachineActuatorParams{KubeadmToken: "dummy"})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	var clusterConfig v1alpha1.AzureClusterProviderConfig
	err = azure.decodeClusterProviderConfig(cluster.Spec.ProviderConfig, &clusterConfig)
	if err != nil {
		t.Fatalf("unable to parse cluster provider config: %v", err)
	}
	defer deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
	for _, machine := range machines {
		err = azure.Create(cluster, machine)
		if err != nil {
			t.Fatalf("unable to create machine: %v", err)
		}
	}
}

func TestCreateUnit(t *testing.T) {
	clusterConfigFile := "testconfigs/cluster-ci-create.yaml"
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	azure, err := mockAzureClient(t)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	for _, machine := range machines {
		err = azure.Create(cluster, machine)
		if err != nil {
			t.Fatalf("unable to create machine: %v", err)
		}
	}
}

func TestUpdate(t *testing.T) {
	// TODO: write test
	return
}

func TestUpdateUnit(t *testing.T) {
	clusterConfigFile := "testconfigs/cluster-ci-create.yaml"
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	azure, err := mockAzureClient(t)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	for _, machine := range machines {
		err = azure.Create(cluster, machine)
		if err != nil {
			t.Fatalf("unable to create machine: %v", err)
		}
	}
	// TODO: Finish test when update functionality is completed
	return
}

func TestDelete(t *testing.T) {
	clusterConfigFile := "testconfigs/cluster-ci-delete.yaml"
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files :%v", err)
	}
	azure, err := NewMachineActuator(MachineActuatorParams{KubeadmToken: "dummy"})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	var clusterConfig v1alpha1.AzureClusterProviderConfig
	err = azure.decodeClusterProviderConfig(cluster.Spec.ProviderConfig, &clusterConfig)
	if err != nil {
		t.Fatalf("unable to parse cluster provider config: %v", err)
	}
	var machineConfig v1alpha1.AzureMachineProviderConfig
	err = azure.decodeMachineProviderConfig(machines[0].Spec.ProviderConfig, &machineConfig)
	if err != nil {
		t.Fatalf("unable to parse machine provider config: %v", err)
	}
	_, err = azure.createOrUpdateGroup(cluster)
	if err != nil {
		t.Fatalf("unable to create resource group: %v", err)
	}
	_, err = azure.createOrUpdateDeployment(cluster, machines[0])
	if err != nil {
		deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
		t.Fatalf("unable to create deployment: %v", err)
	}
	err = azure.Delete(cluster, machines[0])
	if err != nil {
		deleteTestResourceGroup(t, azure, clusterConfig.ResourceGroup)
		t.Fatalf("unable to delete cluster: %v", err)
	}
}

func TestDeleteUnit(t *testing.T) {
	clusterConfigFile := "testconfigs/cluster-ci-delete.yaml"
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files :%v", err)
	}
	azure, err := mockAzureClient(t)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	_, err = azure.createOrUpdateGroup(cluster)
	if err != nil {
		t.Fatalf("unable to create resource group: %v", err)
	}
	_, err = azure.createOrUpdateDeployment(cluster, machines[0])
	if err != nil {
		t.Fatalf("unable to create deployment: %v", err)
	}
	err = azure.Delete(cluster, machines[0])
	if err != nil {
		t.Fatalf("unable to delete cluster: %v", err)
	}
	exists, err := azure.checkResourceGroupExists(cluster)
	if err != nil {
		t.Fatalf("unable to check existence of resource group: %v", err)
	}
	if exists {
		t.Fatalf("got resource group that should've been deleted")
	}
}

func TestExists(t *testing.T) {
	// TODO: write test
	return
}

func TestExistsUnit(t *testing.T) {
	// TODO: write test
	return
}

func TestParseProviderConfigs(t *testing.T) {
	clusterConfigFile := "testconfigs/cluster-ci-parse-providers.yaml"
	cluster, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	if err != nil {
		t.Fatalf("unable to parse config files: %v", err)
	}
	azure, err := NewMachineActuator(MachineActuatorParams{KubeadmToken: "dummy"})
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	var clusterConfig v1alpha1.AzureClusterProviderConfig
	err = azure.decodeClusterProviderConfig(cluster.Spec.ProviderConfig, &clusterConfig)
	if err != nil {
		t.Fatalf("unable to parse cluster provider config: %v", err)
	}
	for _, machine := range machines {
		var machineConfig v1alpha1.AzureMachineProviderConfig
		err = azure.decodeMachineProviderConfig(machine.Spec.ProviderConfig, &machineConfig)
		if err != nil {
			t.Fatalf("unable to parse machine provider config: %v", err)
		}
	}
}

func TestBase64Encoding(t *testing.T) {
	baseText := "echo 'Hello world!'"
	expectedEncoded := "ZWNobyAnSGVsbG8gd29ybGQhJw=="
	actualEncoded := *base64EncodeCommand(baseText)

	if expectedEncoded != actualEncoded {
		t.Fatalf("encoded string does not match expected result: %s != %s", actualEncoded, expectedEncoded)
	}
}

func TestGetStartupScript(t *testing.T) {
	expectedStartupScript := `(
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

	azure := AzureClient{
		machineSetupConfigs: machinesetup.MachineSetup{
			Items: []machinesetup.Params{
				{
					MachineParams: machinesetup.MachineParams{},
					Metadata: machinesetup.Metadata{
						StartupScript: expectedStartupScript,
					},
				},
			},
		},
	}
	machineConfig := mockAzureMachineProviderConfig(t)
	machineConfig.Roles = []v1alpha1.MachineRole{"Master"}
	actualStartupScript, err := azure.getStartupScript(*machineConfig)
	if err != nil {
		t.Fatalf("unable to get startup script: %v", err)
	}
	if actualStartupScript != expectedStartupScript {
		t.Fatalf("got wrong startup script: %v != %v", actualStartupScript, expectedStartupScript)
	}
}

func TestConvertMachineToDeploymentParams(t *testing.T) {
	clusterConfigFile := "testconfigs/cluster-ci-parse-providers.yaml"
	_, machines, err := readConfigs(t, clusterConfigFile, machineConfigFile)
	azure, err := mockAzureClient(t)
	if err != nil {
		t.Fatalf("unable to create mock azure client: %v", err)
	}
	params, err := azure.convertMachineToDeploymentParams(nil, machines[0])
	if err != nil {
		t.Fatalf("unable to convert machine to deployment params: %v", err)
	}
	if (*params)["virtualNetworks_ClusterAPIVM_vnet_name"].(map[string]interface{})["value"].(string) != "ClusterAPIVnet" {
		t.Fatalf("params are not populated correctly")
	}
}

func readConfigs(t *testing.T, clusterConfigPath string, machinesConfigPath string) (*clusterv1.Cluster, []*clusterv1.Machine, error) {
	t.Helper()

	data, err := ioutil.ReadFile(clusterConfigPath)
	if err != nil {
		return nil, nil, err
	}
	cluster := &clusterv1.Cluster{}
	err = yaml.Unmarshal(data, cluster)
	if err != nil {
		return nil, nil, err
	}

	data, err = ioutil.ReadFile(machinesConfigPath)
	if err != nil {
		return nil, nil, err
	}
	list := &clusterv1.MachineList{}
	err = yaml.Unmarshal(data, &list)
	if err != nil {
		return nil, nil, err
	}

	var machines []*clusterv1.Machine
	for index, machine := range list.Items {
		if machine.Spec.ProviderConfig.Value == nil {
			return nil, nil, fmt.Errorf("Machine %d's value is nil", index)
		}
		machines = append(machines, machine.DeepCopy())
	}

	return cluster, machines, nil
}

func mockAzureMachineProviderConfig(t *testing.T) *v1alpha1.AzureMachineProviderConfig {
	t.Helper()
	return &v1alpha1.AzureMachineProviderConfig{
		Location: "eastus",
		VMSize:   "Standard_B1s",
		Image: v1alpha1.Image{
			Publisher: "Canonical",
			Offer:     "UbuntuServer",
			SKU:       "16.04-LTS",
			Version:   "latest",
		},
		OSDisk: v1alpha1.OSDisk{
			OSType: "Linux",
			ManagedDisk: v1alpha1.ManagedDisk{
				StorageAccountType: "Premium_LRS",
			},
			DiskSizeGB: 30,
		},
	}
}

func mockAzureClusterProviderConfig(t *testing.T, rg string) *v1alpha1.AzureClusterProviderConfig {
	t.Helper()
	return &v1alpha1.AzureClusterProviderConfig{
		ResourceGroup: rg,
		Location:      "eastus",
	}
}

func mockAzureClient(t *testing.T) (*AzureClient, error) {
	t.Helper()
	scheme, codecFactory, err := v1alpha1.NewSchemeAndCodecs()
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
	return &AzureClient{
		SubscriptionID: wrappers.MockSubscriptionID,
		scheme:         scheme,
		codecFactory:   codecFactory,
		Authorizer:     authorizer,
	}, nil
}

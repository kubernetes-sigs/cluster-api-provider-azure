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
	"net/http"
	"os"
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2018-04-01/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/ghodss/yaml"
	"github.com/joho/godotenv"
	azure "github.com/platform9/azure-provider/cloud/azure/actuators/cluster"
	"github.com/platform9/azure-provider/cloud/azure/actuators/machine/machinesetup"
	"github.com/platform9/azure-provider/cloud/azure/actuators/machine/wrappers"
	azureconfigv1 "github.com/platform9/azure-provider/cloud/azure/providerconfig/v1alpha1"
	"github.com/platform9/azure-provider/cloud/azure/services"

	"sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

func TestDeleteSuccess(t *testing.T) {
	azureServicesClient := mockVMExists()
	params := MachineActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderConfig()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	if err != nil {
		t.Fatalf("unable to create machine actuator: %v", err)
	}
	err = actuator.Delete(cluster, machine)
	if err != nil {
		t.Fatalf("unable to delete machine: %v", err)
	}
}

func TestDeleteFailureVMNotExists(t *testing.T) {
	azureServicesClient := mockVMNotExists()
	params := MachineActuatorParams{Services: &azureServicesClient}
	machineConfig := newMachineProviderConfig()
	machine := newMachine(t, machineConfig)
	cluster := newCluster(t)

	actuator, err := NewMachineActuator(params)
	err = actuator.Delete(cluster, machine)
	if err == nil {
		t.Fatalf("expected error, but got none")
	}
}

func TestExists(t *testing.T) {
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
	_, err = azure.azureProviderConfigCodec.ClusterProviderFromProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		t.Fatalf("unable to parse cluster provider config: %v", err)
	}
	for _, machine := range machines {
		_, err := azure.decodeMachineProviderConfig(machine.Spec.ProviderConfig)
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
	machineConfig.Roles = []azureconfigv1.MachineRole{"Master"}
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

func mockAzureMachineProviderConfig(t *testing.T) *azureconfigv1.AzureMachineProviderConfig {
	t.Helper()
	return &azureconfigv1.AzureMachineProviderConfig{
		Location: "eastus",
		VMSize:   "Standard_B1s",
		Image: azureconfigv1.Image{
			Publisher: "Canonical",
			Offer:     "UbuntuServer",
			SKU:       "16.04-LTS",
			Version:   "latest",
		},
		OSDisk: azureconfigv1.OSDisk{
			OSType: "Linux",
			ManagedDisk: azureconfigv1.ManagedDisk{
				StorageAccountType: "Premium_LRS",
			},
			DiskSizeGB: 30,
		},
	}
}

func mockAzureClusterProviderConfig(t *testing.T, rg string) *azureconfigv1.AzureClusterProviderConfig {
	t.Helper()
	return &azureconfigv1.AzureClusterProviderConfig{
		ResourceGroup: rg,
		Location:      "eastus",
	}
}

func mockAzureClient(t *testing.T) (*AzureClient, error) {
	t.Helper()
	scheme, codec, err := azureconfigv1.NewSchemeAndCodecs()
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
		SubscriptionID:           wrappers.MockSubscriptionID,
		scheme:                   scheme,
		azureProviderConfigCodec: codec,
		Authorizer:               authorizer,
	}, nil
}

func createClusterActuator() (*azure.AzureClusterClient, error) {
	params := azure.ClusterActuatorParams{}
	actuator, err := azure.NewClusterActuator(params)
	if err != nil {
		log.Fatalf("failed to create cluster actuator")
		return nil, err
	}
	return actuator, nil
}

func newMachine(t *testing.T, machineConfig azureconfigv1.AzureMachineProviderConfig) *v1alpha1.Machine {
	providerConfig, err := providerConfigFromMachine(&machineConfig)
	if err != nil {
		t.Fatalf("error encoding provider config: %v", err)
	}
	return &v1alpha1.Machine{
		ObjectMeta: v1.ObjectMeta{
			Name: "machine-test",
		},
		Spec: v1alpha1.MachineSpec{
			ProviderConfig: *providerConfig,
			Versions: v1alpha1.MachineVersionInfo{
				Kubelet:      "1.9.4",
				ControlPlane: "1.9.4",
			},
		},
	}
}

func newCluster(t *testing.T) *v1alpha1.Cluster {
	clusterProviderConfig := newClusterProviderConfig()
	providerConfig, err := providerConfigFromCluster(&clusterProviderConfig)
	if err != nil {
		t.Fatalf("error encoding provider config: %v", err)
	}

	return &v1alpha1.Cluster{
		TypeMeta: v1.TypeMeta{
			Kind: "Cluster",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "cluster-test",
		},
		Spec: v1alpha1.ClusterSpec{
			ClusterNetwork: v1alpha1.ClusterNetworkingConfig{
				Services: v1alpha1.NetworkRanges{
					CIDRBlocks: []string{
						"10.96.0.0/12",
					},
				},
				Pods: v1alpha1.NetworkRanges{
					CIDRBlocks: []string{
						"192.168.0.0/16",
					},
				},
			},
			ProviderConfig: *providerConfig,
		},
	}
}

func providerConfigFromMachine(in *azureconfigv1.AzureMachineProviderConfig) (*clusterv1.ProviderConfig, error) {
	bytes, err := yaml.Marshal(in)
	if err != nil {
		return nil, err
	}
	return &clusterv1.ProviderConfig{
		Value: &runtime.RawExtension{Raw: bytes},
	}, nil
}

func providerConfigFromCluster(in *azureconfigv1.AzureClusterProviderConfig) (*clusterv1.ProviderConfig, error) {
	bytes, err := yaml.Marshal(in)
	if err != nil {
		return nil, err
	}
	return &clusterv1.ProviderConfig{
		Value: &runtime.RawExtension{Raw: bytes},
	}, nil
}

func newClusterProviderConfig() azureconfigv1.AzureClusterProviderConfig {
	return azureconfigv1.AzureClusterProviderConfig{
		ResourceGroup: "resource-group-test",
		Location:      "westus2",
	}
}

func mockVMExists() services.AzureClients {
	computeMock := services.MockAzureComputeClient{
		MockVmIfExists: func(resourceGroup string, name string) (*compute.VirtualMachine, error) {
			networkProfile := compute.NetworkProfile{NetworkInterfaces: &[]compute.NetworkInterfaceReference{compute.NetworkInterfaceReference{ID: to.StringPtr("001")}}}
			OsDiskName := fmt.Sprintf("OS_Disk_%v", name)
			storageProfile := compute.StorageProfile{OsDisk: &compute.OSDisk{Name: &OsDiskName}}
			vmProperties := compute.VirtualMachineProperties{StorageProfile: &storageProfile, NetworkProfile: &networkProfile}
			return &compute.VirtualMachine{Name: &name, VirtualMachineProperties: &vmProperties}, nil
		},
	}
	return services.AzureClients{Compute: &computeMock, Resourcemanagement: &services.MockAzureResourceManagementClient{}, Network: &services.MockAzureNetworkClient{}}
}

func mockVMNotExists() services.AzureClients {
	resourcemanagementMock := services.MockAzureResourceManagementClient{
		MockCheckGroupExistence: func(rgName string) (autorest.Response, error) {
			return autorest.Response{Response: &http.Response{StatusCode: 200}}, nil
		},
	}
	return services.AzureClients{Compute: &services.MockAzureComputeClient{}, Resourcemanagement: &resourcemanagementMock, Network: &services.MockAzureNetworkClient{}}
}

func newMachineProviderConfig() azureconfigv1.AzureMachineProviderConfig {
	return azureconfigv1.AzureMachineProviderConfig{
		Location: "westus2",
		VMSize:   "Standard_B2ms",
		Image: azureconfigv1.Image{
			Publisher: "Canonical",
			Offer:     "UbuntuServer",
			SKU:       "16.04-LTS",
			Version:   "latest",
		},
		OSDisk: azureconfigv1.OSDisk{
			OSType: "Linux",
			ManagedDisk: azureconfigv1.ManagedDisk{
				StorageAccountType: "Premium_LRS",
			},
			DiskSizeGB: 30,
		},
	}
}

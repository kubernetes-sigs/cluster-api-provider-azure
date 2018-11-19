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
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strings"

	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/runtime"

	"time"

	"github.com/golang/glog"
	"github.com/platform9/azure-provider/cloud/azure/actuators/machine/machinesetup"
	azureconfigv1 "github.com/platform9/azure-provider/cloud/azure/providerconfig/v1alpha1"
	"github.com/platform9/azure-provider/cloud/azure/services"
	"github.com/platform9/azure-provider/cloud/azure/services/compute"
	"github.com/platform9/azure-provider/cloud/azure/services/network"
	"github.com/platform9/azure-provider/cloud/azure/services/resourcemanagement"
	clustercommon "sigs.k8s.io/cluster-api/pkg/apis/cluster/common"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	client "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
)

// The Azure Client, also used as a machine actuator
type AzureClient struct {
	services *services.AzureClients
	// interface to cluster-api
	v1Alpha1Client           client.ClusterV1alpha1Interface
	azureProviderConfigCodec *azureconfigv1.AzureProviderConfigCodec
	machineSetupConfigs      machinesetup.MachineSetup
	scheme                   *runtime.Scheme
}

// Parameter object used to create a machine actuator.
// These are not indicative of all requirements for a machine actuator, environment variables are also necessary.
type MachineActuatorParams struct {
	V1Alpha1Client         client.ClusterV1alpha1Interface
	Services               *services.AzureClients
	MachineSetupConfigPath string
}

const (
	ProviderName = "azure"
	SSHUser      = "ClusterAPI"
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
	scheme, err := azureconfigv1.NewScheme()
	if err != nil {
		return nil, err
	}
	azureProviderConfigCodec, err := azureconfigv1.NewCodec()
	if err != nil {
		return nil, fmt.Errorf("error creating codec for provider: %v", err)
	}
	azureServicesClients, err := azureServicesClientOrDefault(params)
	if err != nil {
		return nil, fmt.Errorf("error getting azure services client: %v", err)
	}
	return &AzureClient{
		services:                 azureServicesClients,
		v1Alpha1Client:           params.V1Alpha1Client,
		azureProviderConfigCodec: azureProviderConfigCodec,
		scheme:                   scheme,
	}, nil
}

// Create a machine based on the cluster and machine spec passed
func (azure *AzureClient) Create(cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	clusterConfig, err := azure.azureProviderConfigCodec.ClusterProviderFromProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		return fmt.Errorf("error loading cluster provider config: %v", err)
	}
	machineConfig, err := azure.azureProviderConfigCodec.MachineProviderFromProviderConfig(machine.Spec.ProviderConfig)
	if err != nil {
		return fmt.Errorf("error loading machine provider config: %v", err)
	}

	err = azure.resourcemanagement().ValidateDeployment(machine, clusterConfig, machineConfig)
	if err != nil {
		return fmt.Errorf("error validating deployment: %v", err)
	}

	deploymentsFuture, err := azure.resourcemanagement().CreateOrUpdateDeployment(machine, clusterConfig, machineConfig)
	if err != nil {
		return fmt.Errorf("error creating or updating deployment: %v", err)
	}
	err = azure.resourcemanagement().WaitForDeploymentsCreateOrUpdateFuture(*deploymentsFuture)
	if err != nil {
		return fmt.Errorf("error waiting for deployment creation or update: %v", err)
	}

	deployment, err := azure.resourcemanagement().GetDeploymentResult(*deploymentsFuture)
	// Work around possible bugs or late-stage failures
	if deployment.Name == nil || err != nil {
		return fmt.Errorf("error getting deployment result: %v", err)
	}
	return azure.updateAnnotations(cluster, machine)
}

// Update an existing machine based on the cluster and machine spec passed.
func (azure *AzureClient) Update(cluster *clusterv1.Cluster, goalMachine *clusterv1.Machine) error {
	clusterConfig, err := azure.azureProviderConfigCodec.ClusterProviderFromProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		return fmt.Errorf("error loading cluster provider config: %v", err)
	}
	_, err = azure.azureProviderConfigCodec.MachineProviderFromProviderConfig(goalMachine.Spec.ProviderConfig)
	if err != nil {
		return fmt.Errorf("error loading goal machine provider config: %v", err)
	}

	vm, err := azure.compute().VmIfExists(clusterConfig.ResourceGroup, resourcemanagement.GetVMName(goalMachine))
	if err != nil || vm == nil {
		return fmt.Errorf("error checking if vm exists: %v", err)
	}

	status, err := azure.status(goalMachine)
	if err != nil {
		return err
	}

	currentMachine := (*clusterv1.Machine)(status)
	if currentMachine == nil {
		return fmt.Errorf("current machine %v no longer exists: %v", goalMachine.ObjectMeta.Name, err)
	}
	currentMachineConfig, err := azure.azureProviderConfigCodec.MachineProviderFromProviderConfig(currentMachine.Spec.ProviderConfig)
	if err != nil {
		return fmt.Errorf("error loading current machine provider config: %v", err)
	}

	// no need for update if fields havent changed
	if !azure.shouldUpdate(currentMachine, goalMachine) {
		glog.Infof("no need to update machine: %v", currentMachine.ObjectMeta.Name)
		return nil
	}

	// update master inplace
	if isMasterMachine(currentMachineConfig.Roles) {
		glog.Infof("updating master machine %v in place", currentMachine.ObjectMeta.Name)
		err = azure.updateMaster(cluster, currentMachine, goalMachine)
		if err != nil {
			return fmt.Errorf("error updating master machine %v in place: %v", currentMachine.ObjectMeta.Name, err)
		}
	} else {
		// delete and recreate machine for nodes
		glog.Infof("replacing node machine %v", currentMachine.ObjectMeta.Name)
		err = azure.Delete(cluster, currentMachine)
		if err != nil {
			return fmt.Errorf("error updating node machine %v, deleting node machine failed: %v", currentMachine.ObjectMeta.Name, err)
		}
		err = azure.Create(cluster, goalMachine)
		if err != nil {
			glog.Errorf("error updating node machine %v, creating node machine failed: %v", goalMachine.ObjectMeta.Name, err)
		}
	}
	return azure.updateAnnotations(cluster, goalMachine)
}

func (azure *AzureClient) updateMaster(cluster *clusterv1.Cluster, currentMachine *clusterv1.Machine, goalMachine *clusterv1.Machine) error {
	clusterConfig, err := azure.azureProviderConfigCodec.ClusterProviderFromProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		return fmt.Errorf("error loading cluster provider config: %v", err)
	}

	// update the control plane
	if currentMachine.Spec.Versions.ControlPlane != goalMachine.Spec.Versions.ControlPlane {
		// upgrade kubeadm
		cmd := fmt.Sprintf("curl -sSL https://dl.k8s.io/release/v%s/bin/linux/amd64/kubeadm | "+
			"sudo tee /usr/bin/kubeadm > /dev/null;"+
			"sudo chmod a+rx /usr/bin/kubeadm;", goalMachine.Spec.Versions.ControlPlane)
		cmd += fmt.Sprintf("sudo kubeadm upgrade apply v%s -y;", goalMachine.Spec.Versions.ControlPlane)

		commandRunFuture, err := azure.compute().RunCommand(clusterConfig.ResourceGroup, resourcemanagement.GetVMName(goalMachine), cmd)
		if err != nil {
			return fmt.Errorf("error running command on vm: %v", err)
		}
		err = azure.compute().WaitForVMRunCommandFuture(commandRunFuture)
		if err != nil {
			return fmt.Errorf("error waiting for vm run command future: %v", err)
		}
	}

	// update kubelet and kubectl client version
	if currentMachine.Spec.Versions.Kubelet != goalMachine.Spec.Versions.Kubelet {
		nodeName := strings.ToLower(resourcemanagement.GetVMName(goalMachine))
		cmd := fmt.Sprintf("sudo kubectl drain %s --kubeconfig /etc/kubernetes/admin.conf --ignore-daemonsets;"+
			"sudo apt-get install kubelet=%s;", nodeName, goalMachine.Spec.Versions.Kubelet+"-00")
		cmd += fmt.Sprintf("sudo kubectl uncordon %s --kubeconfig /etc/kubernetes/admin.conf;", nodeName)
		// update kubectl client version
		cmd += fmt.Sprintf("curl -sSL https://dl.k8s.io/release/v%s/bin/linux/amd64/kubectl | "+
			"sudo tee /usr/bin/kubectl > /dev/null;"+
			"sudo chmod a+rx /usr/bin/kubectl;", goalMachine.Spec.Versions.Kubelet)

		commandRunFuture, err := azure.compute().RunCommand(clusterConfig.ResourceGroup, resourcemanagement.GetVMName(goalMachine), cmd)
		if err != nil {
			return fmt.Errorf("error running command on vm: %v", err)
		}
		err = azure.compute().WaitForVMRunCommandFuture(commandRunFuture)
		if err != nil {
			return fmt.Errorf("error waiting for vm run command future: %v", err)
		}
	}
	return nil
}

func (azure *AzureClient) shouldUpdate(m1 *clusterv1.Machine, m2 *clusterv1.Machine) bool {
	return !reflect.DeepEqual(m1.Spec.Versions, m2.Spec.Versions) ||
		!reflect.DeepEqual(m1.Spec.ObjectMeta, m2.Spec.ObjectMeta) ||
		!reflect.DeepEqual(m1.Spec.ProviderConfig, m2.Spec.ProviderConfig) ||
		m1.ObjectMeta.Name != m2.ObjectMeta.Name
}

// Delete an existing machine based on the cluster and machine spec passed.
// Will block until the machine has been successfully deleted, or an error is returned.
func (azure *AzureClient) Delete(cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	clusterConfig, err := azure.azureProviderConfigCodec.ClusterProviderFromProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		return fmt.Errorf("error loading cluster provider config: %v", err)
	}
	// Parse in provider configs
	_, err = azure.azureProviderConfigCodec.MachineProviderFromProviderConfig(machine.Spec.ProviderConfig)
	if err != nil {
		return fmt.Errorf("error loading machine provider config: %v", err)
	}
	// Check if VM exists
	vm, err := azure.compute().VmIfExists(clusterConfig.ResourceGroup, resourcemanagement.GetVMName(machine))
	if err != nil {
		return fmt.Errorf("error checking if vm exists: %v", err)
	}
	if vm == nil {
		return fmt.Errorf("couldn't find vm for machine: %v", machine.Name)
	}
	osDiskName := vm.VirtualMachineProperties.StorageProfile.OsDisk.Name
	nicID := (*vm.VirtualMachineProperties.NetworkProfile.NetworkInterfaces)[0].ID

	// delete the VM instance
	vmDeleteFuture, err := azure.compute().DeleteVM(clusterConfig.ResourceGroup, resourcemanagement.GetVMName(machine))
	if err != nil {
		return fmt.Errorf("error deleting virtual machine: %v", err)
	}
	err = azure.compute().WaitForVMDeletionFuture(vmDeleteFuture)
	if err != nil {
		return fmt.Errorf("error waiting for virtual machine deletion: %v", err)
	}

	// delete OS disk associated with the VM
	diskDeleteFuture, err := azure.compute().DeleteManagedDisk(clusterConfig.ResourceGroup, *osDiskName)
	if err != nil {
		return fmt.Errorf("error deleting managed disk: %v", err)
	}
	err = azure.compute().WaitForDisksDeleteFuture(diskDeleteFuture)
	if err != nil {
		return fmt.Errorf("error waiting for managed disk deletion: %v", err)
	}

	// delete NIC associated with the VM
	nicName, err := resourcemanagement.ResourceName(*nicID)
	if err != nil {
		return fmt.Errorf("error retrieving network interface name: %v", err)
	}
	interfacesDeleteFuture, err := azure.network().DeleteNetworkInterface(clusterConfig.ResourceGroup, nicName)
	if err != nil {
		return fmt.Errorf("error deleting network interface: %v", err)
	}
	err = azure.network().WaitForNetworkInterfacesDeleteFuture(interfacesDeleteFuture)
	if err != nil {
		return fmt.Errorf("error waiting for network interface deletion: %v", err)
	}

	// delete public ip address associated with the VM
	publicIpAddressDeleteFuture, err := azure.network().DeletePublicIpAddress(clusterConfig.ResourceGroup, resourcemanagement.GetPublicIPName(machine))
	if err != nil {
		return fmt.Errorf("error deleting public IP address: %v", err)
	}
	err = azure.network().WaitForPublicIpAddressDeleteFuture(publicIpAddressDeleteFuture)
	if err != nil {
		return fmt.Errorf("error waiting for public ip address deletion: %v", err)
	}
	return nil
}

// Get the kubeconfig of a machine based on the cluster and machine spec passed.
// Has not been fully tested as k8s is not yet bootstrapped on created machines.
func (azure *AzureClient) GetKubeConfig(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	clusterConfig, err := azure.azureProviderConfigCodec.ClusterProviderFromProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		return "", fmt.Errorf("error loading cluster provider config: %v", err)
	}
	machineConfig, err := azure.azureProviderConfigCodec.MachineProviderFromProviderConfig(machine.Spec.ProviderConfig)
	if err != nil {
		return "", fmt.Errorf("error loading machine provider config: %v", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(machineConfig.SSHPrivateKey)
	privateKey := string(decoded)
	if err != nil {
		return "", err
	}

	ip, err := azure.network().GetPublicIpAddress(clusterConfig.ResourceGroup, resourcemanagement.GetPublicIPName(machine))
	if err != nil {
		return "", fmt.Errorf("error getting public ip address: %v ", err)
	}
	sshclient, err := GetSshClient(*ip.IPAddress, privateKey)
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
	clusterConfig, err := azure.azureProviderConfigCodec.ClusterProviderFromProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		return false, err
	}
	resp, err := azure.resourcemanagement().CheckGroupExistence(clusterConfig.ResourceGroup)
	if err != nil {
		return false, err
	}
	if resp.StatusCode == 404 {
		return false, nil
	}
	vm, err := azure.compute().VmIfExists(clusterConfig.ResourceGroup, resourcemanagement.GetVMName(machine))
	if err != nil {
		return false, err
	}
	return vm != nil, nil
}

// Return the ip address of an existing machine based on the cluster and machine spec passed.
func (azure *AzureClient) GetIP(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	clusterConfig, err := azure.azureProviderConfigCodec.ClusterProviderFromProviderConfig(cluster.Spec.ProviderConfig)
	if err != nil {
		return "", fmt.Errorf("error loading cluster provider config: %v", err)
	}
	publicIP, err := azure.network().GetPublicIpAddress(clusterConfig.ResourceGroup, resourcemanagement.GetPublicIPName(machine))
	if err != nil {
		return "", fmt.Errorf("error getting public ip address: %v", err)
	}
	return *publicIP.IPAddress, nil

}

func azureServicesClientOrDefault(params MachineActuatorParams) (*services.AzureClients, error) {
	if params.Services != nil {
		return params.Services, nil
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
	azureComputeClient := compute.NewService(subscriptionID)
	azureComputeClient.SetAuthorizer(authorizer)
	azureNetworkClient := network.NewService(subscriptionID)
	azureNetworkClient.SetAuthorizer(authorizer)
	azureResourceManagementClient := resourcemanagement.NewService(subscriptionID)
	azureResourceManagementClient.SetAuthorizer(authorizer)
	return &services.AzureClients{
		Compute:            azureComputeClient,
		Network:            azureNetworkClient,
		Resourcemanagement: azureResourceManagementClient,
	}, nil
}

func (azure *AzureClient) compute() services.AzureComputeClient {
	return azure.services.Compute
}

func (azure *AzureClient) network() services.AzureNetworkClient {
	return azure.services.Network
}

func (azure *AzureClient) resourcemanagement() services.AzureResourceManagementClient {
	return azure.services.Resourcemanagement
}

func isMasterMachine(roles []azureconfigv1.MachineRole) bool {
	for _, r := range roles {
		if r == azureconfigv1.Master {
			return true
		}
	}
	return false
}

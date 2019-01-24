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
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"k8s.io/klog"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/compute"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/network"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/resources"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/deployer"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	client "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
)

// Actuator is responsible for performing machine reconciliation.
type Actuator struct {
	*deployer.Deployer

	client client.ClusterV1alpha1Interface
}

// ActuatorParams contains the parameters that are used to create a machine actuator.
// These are not indicative of all requirements for a machine actuator, environment variables are also necessary.
type ActuatorParams struct {
	Client client.ClusterV1alpha1Interface
}

// NewActuator returns an actuator.
func NewActuator(params ActuatorParams) *Actuator {
	return &Actuator{
		Deployer: deployer.New(deployer.Params{ScopeGetter: actuators.DefaultScopeGetter}),
		client:   params.Client,
	}
}

const (
	// ProviderName is the default name of the cloud provider used.
	ProviderName = "azure"
	// SSHUser is the default ssh username.
	SSHUser = "ClusterAPI"
)

// Create creates a machine and is invoked by the machine controller.
func (a *Actuator) Create(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	klog.Infof("Creating machine %v for cluster %v", machine.Name, cluster.Name)

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{Machine: machine, Cluster: cluster, Client: a.client})
	if err != nil {
		return errors.Errorf("failed to create scope: %+v", err)
	}

	defer scope.Close()

	resourcesSvc := resources.NewService(scope.Scope)

	err = resourcesSvc.ValidateDeployment(machine, scope.ClusterConfig, scope.MachineConfig)
	if err != nil {
		return fmt.Errorf("error validating deployment: %v", err)
	}

	deploymentsFuture, err := resourcesSvc.CreateOrUpdateDeployment(machine, scope.ClusterConfig, scope.MachineConfig)
	if err != nil {
		return fmt.Errorf("error creating or updating deployment: %v", err)
	}
	err = resourcesSvc.WaitForDeploymentsCreateOrUpdateFuture(*deploymentsFuture)
	if err != nil {
		return fmt.Errorf("error waiting for deployment creation or update: %v", err)
	}

	deployment, err := resourcesSvc.GetDeploymentResult(*deploymentsFuture)
	// Work around possible bugs or late-stage failures
	if deployment.Name == nil || err != nil {
		return fmt.Errorf("error getting deployment result: %v", err)
	}
	// TODO: Is this still required with scope.Close()?
	//return a.updateAnnotations(cluster, machine)
	return nil
}

// Update an existing machine based on the cluster and machine spec parameters.
func (a *Actuator) Update(ctx context.Context, cluster *clusterv1.Cluster, goalMachine *clusterv1.Machine) error {
	klog.Infof("Updating machine %v for cluster %v.", goalMachine.Name, cluster.Name)

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{Machine: goalMachine, Cluster: cluster, Client: a.client})
	if err != nil {
		return errors.Errorf("failed to create scope: %+v", err)
	}

	defer scope.Close()

	computeSvc := compute.NewService(scope.Scope)

	/*
		status, err := a.status(goalMachine)
		if err != nil {
			return err
		}
		currentMachine := (*clusterv1.Machine)(status)
	*/

	currentMachine := scope.Machine

	if currentMachine == nil {
		vm, err := computeSvc.VMIfExists(scope.ClusterConfig.ResourceGroup, resources.GetVMName(goalMachine))
		if err != nil || vm == nil {
			return fmt.Errorf("error checking if vm exists: %v", err)
		}
		// update annotations for bootstrap machine
		if vm != nil {
			// TODO: Is this still required with scope.Close()?
			//return a.updateAnnotations(cluster, goalMachine)
			return nil
		}
		return fmt.Errorf("current machine %v no longer exists: %v", goalMachine.ObjectMeta.Name, err)
	}

	// no need for update if fields havent changed
	if !a.shouldUpdate(currentMachine, goalMachine) {
		glog.Infof("no need to update machine: %v", currentMachine.ObjectMeta.Name)
		return nil
	}

	// update master inplace
	if isMasterMachine(scope.MachineConfig.Roles) {
		glog.Infof("updating master machine %v in place", currentMachine.ObjectMeta.Name)
		err = a.updateMaster(cluster, currentMachine, goalMachine)
		if err != nil {
			return fmt.Errorf("error updating master machine %v in place: %v", currentMachine.ObjectMeta.Name, err)
		}
		// TODO: Is this still required with scope.Close()?
		//return a.updateStatus(goalMachine)
		return nil
	}
	// delete and recreate machine for nodes
	glog.Infof("replacing node machine %v", currentMachine.ObjectMeta.Name)
	err = a.Delete(ctx, cluster, currentMachine)
	if err != nil {
		return fmt.Errorf("error updating node machine %v, deleting node machine failed: %v", currentMachine.ObjectMeta.Name, err)
	}
	err = a.Create(ctx, cluster, goalMachine)
	if err != nil {
		glog.Errorf("error updating node machine %v, creating node machine failed: %v", goalMachine.ObjectMeta.Name, err)
	}
	return nil
}

func (a *Actuator) updateMaster(cluster *clusterv1.Cluster, currentMachine *clusterv1.Machine, goalMachine *clusterv1.Machine) error {
	//klog.Infof("Creating machine %v for cluster %v", machine.Name, cluster.Name)

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{Machine: goalMachine, Cluster: cluster, Client: a.client})
	if err != nil {
		return errors.Errorf("failed to create scope: %+v", err)
	}

	defer scope.Close()

	computeSvc := compute.NewService(scope.Scope)

	// update the control plane
	if currentMachine.Spec.Versions.ControlPlane != goalMachine.Spec.Versions.ControlPlane {
		// upgrade kubeadm
		cmd := fmt.Sprintf("curl -sSL https://dl.k8s.io/release/v%s/bin/linux/amd64/kubeadm | "+
			"sudo tee /usr/bin/kubeadm > /dev/null;"+
			"sudo chmod a+rx /usr/bin/kubeadm;", goalMachine.Spec.Versions.ControlPlane)
		cmd += fmt.Sprintf("sudo kubeadm upgrade apply v%s -y;", goalMachine.Spec.Versions.ControlPlane)

		// update kubectl client version
		cmd += fmt.Sprintf("curl -sSL https://dl.k8s.io/release/v%s/bin/linux/amd64/kubectl | "+
			"sudo tee /usr/bin/kubectl > /dev/null;"+
			"sudo chmod a+rx /usr/bin/kubectl;", goalMachine.Spec.Versions.ControlPlane)
		commandRunFuture, err := computeSvc.RunCommand(scope.ClusterConfig.ResourceGroup, resources.GetVMName(goalMachine), cmd)
		if err != nil {
			return fmt.Errorf("error running command on vm: %v", err)
		}
		err = computeSvc.WaitForVMRunCommandFuture(commandRunFuture)
		if err != nil {
			return fmt.Errorf("error waiting for upgrade control plane future: %v", err)
		}
	}

	// update master and node packages
	if currentMachine.Spec.Versions.Kubelet != goalMachine.Spec.Versions.Kubelet {
		nodeName := strings.ToLower(resources.GetVMName(goalMachine))
		// prepare node for maintenance
		cmd := fmt.Sprintf("sudo kubectl drain %s --kubeconfig /etc/kubernetes/admin.conf --ignore-daemonsets;"+
			"sudo apt-get install kubelet=%s;", nodeName, goalMachine.Spec.Versions.Kubelet+"-00")
		// mark the node as schedulable
		cmd += fmt.Sprintf("sudo kubectl uncordon %s --kubeconfig /etc/kubernetes/admin.conf;", nodeName)

		commandRunFuture, err := computeSvc.RunCommand(scope.ClusterConfig.ResourceGroup, resources.GetVMName(goalMachine), cmd)
		if err != nil {
			return fmt.Errorf("error running command on vm: %v", err)
		}
		err = computeSvc.WaitForVMRunCommandFuture(commandRunFuture)
		if err != nil {
			return fmt.Errorf("error waiting for upgrade kubelet command future: %v", err)
		}
	}
	return nil
}

func (a *Actuator) shouldUpdate(m1 *clusterv1.Machine, m2 *clusterv1.Machine) bool {
	return !reflect.DeepEqual(m1.Spec.Versions, m2.Spec.Versions) ||
		!reflect.DeepEqual(m1.Spec.ObjectMeta, m2.Spec.ObjectMeta) ||
		!reflect.DeepEqual(m1.Spec.ProviderSpec, m2.Spec.ProviderSpec) ||
		m1.ObjectMeta.Name != m2.ObjectMeta.Name
}

// Delete an existing machine based on the cluster and machine spec passed.
// Will block until the machine has been successfully deleted, or an error is returned.
func (a *Actuator) Delete(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	klog.Infof("Creating machine %v for cluster %v", machine.Name, cluster.Name)

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{Machine: machine, Cluster: cluster, Client: a.client})
	if err != nil {
		return errors.Errorf("failed to create scope: %+v", err)
	}

	defer scope.Close()

	computeSvc := compute.NewService(scope.Scope)
	networkSvc := network.NewService(scope.Scope)

	// Check if VM exists
	vm, err := computeSvc.VMIfExists(scope.ClusterConfig.ResourceGroup, resources.GetVMName(machine))
	if err != nil {
		return fmt.Errorf("error checking if vm exists: %v", err)
	}
	if vm == nil {
		return fmt.Errorf("couldn't find vm for machine: %v", machine.Name)
	}
	osDiskName := vm.VirtualMachineProperties.StorageProfile.OsDisk.Name
	nicID := (*vm.VirtualMachineProperties.NetworkProfile.NetworkInterfaces)[0].ID

	// delete the VM instance
	vmDeleteFuture, err := computeSvc.DeleteVM(scope.ClusterConfig.ResourceGroup, resources.GetVMName(machine))
	if err != nil {
		return fmt.Errorf("error deleting virtual machine: %v", err)
	}
	err = computeSvc.WaitForVMDeletionFuture(vmDeleteFuture)
	if err != nil {
		return fmt.Errorf("error waiting for virtual machine deletion: %v", err)
	}

	// delete OS disk associated with the VM
	diskDeleteFuture, err := computeSvc.DeleteManagedDisk(scope.ClusterConfig.ResourceGroup, *osDiskName)
	if err != nil {
		return fmt.Errorf("error deleting managed disk: %v", err)
	}
	err = computeSvc.WaitForDisksDeleteFuture(diskDeleteFuture)
	if err != nil {
		return fmt.Errorf("error waiting for managed disk deletion: %v", err)
	}

	// delete NIC associated with the VM
	nicName, err := resources.ResourceName(*nicID)
	if err != nil {
		return fmt.Errorf("error retrieving network interface name: %v", err)
	}
	interfacesDeleteFuture, err := networkSvc.DeleteNetworkInterface(scope.ClusterConfig.ResourceGroup, nicName)
	if err != nil {
		return fmt.Errorf("error deleting network interface: %v", err)
	}
	err = networkSvc.WaitForNetworkInterfacesDeleteFuture(interfacesDeleteFuture)
	if err != nil {
		return fmt.Errorf("error waiting for network interface deletion: %v", err)
	}

	// delete public ip address associated with the VM
	publicIPAddressDeleteFuture, err := networkSvc.DeletePublicIPAddress(scope.ClusterConfig.ResourceGroup, resources.GetPublicIPName(machine))
	if err != nil {
		return fmt.Errorf("error deleting public IP address: %v", err)
	}
	err = networkSvc.WaitForPublicIPAddressDeleteFuture(publicIPAddressDeleteFuture)
	if err != nil {
		return fmt.Errorf("error waiting for public ip address deletion: %v", err)
	}
	return nil
}

// GetKubeConfig gets the kubeconfig of a machine based on the cluster and machine spec passed.
// Has not been fully tested as k8s is not yet bootstrapped on created machines.
func (a *Actuator) GetKubeConfig(cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	klog.Infof("Creating machine %v for cluster %v", machine.Name, cluster.Name)

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{Machine: machine, Cluster: cluster, Client: a.client})
	if err != nil {
		return "", errors.Errorf("failed to create scope: %+v", err)
	}

	defer scope.Close()

	networkSvc := network.NewService(scope.Scope)

	decoded, err := base64.StdEncoding.DecodeString(scope.MachineConfig.SSHPrivateKey)
	privateKey := string(decoded)
	if err != nil {
		return "", err
	}

	ip, err := networkSvc.GetPublicIPAddress(scope.ClusterConfig.ResourceGroup, resources.GetPublicIPName(machine))
	if err != nil {
		return "", fmt.Errorf("error getting public ip address: %v ", err)
	}
	sshclient, err := GetSSHClient(*ip.IPAddress, privateKey)
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

// GetSSHClient returns an instance of ssh.Client from the host and private key passed.
func GetSSHClient(host string, privatekey string) (*ssh.Client, error) {
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

// Exists determines whether a machine exists based on the cluster and machine spec passed.
func (a *Actuator) Exists(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) (bool, error) {
	klog.Infof("Checking if machine %v for cluster %v exists", machine.Name, cluster.Name)

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{Machine: machine, Cluster: cluster, Client: a.client})
	if err != nil {
		return false, errors.Errorf("failed to create scope: %+v", err)
	}

	defer scope.Close()

	computeSvc := compute.NewService(scope.Scope)
	resourcesSvc := resources.NewService(scope.Scope)

	resp, err := resourcesSvc.CheckGroupExistence(scope.ClusterConfig.ResourceGroup)
	if err != nil {
		return false, err
	}
	if resp.StatusCode == 404 {
		return false, nil
	}
	vm, err := computeSvc.VMIfExists(scope.ClusterConfig.ResourceGroup, resources.GetVMName(machine))
	if err != nil {
		return false, err
	}
	return vm != nil, nil
}

func isMasterMachine(roles []v1alpha1.MachineRole) bool {
	for _, r := range roles {
		if r == v1alpha1.Master {
			return true
		}
	}
	return false
}

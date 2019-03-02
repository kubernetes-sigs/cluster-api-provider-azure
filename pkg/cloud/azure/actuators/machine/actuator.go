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
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/klog"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/compute"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/network"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/resources"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/deployer"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/tokens"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	client "sigs.k8s.io/cluster-api/pkg/client/clientset_generated/clientset/typed/cluster/v1alpha1"
	controllerError "sigs.k8s.io/cluster-api/pkg/controller/error"
)

//+kubebuilder:rbac:groups=azureprovider.k8s.io,resources=azuremachineproviderconfigs;azuremachineproviderstatuses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.k8s.io,resources=machines;machines/status;machinedeployments;machinedeployments/status;machinesets;machinesets/status;machineclasses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=nodes;events,verbs=get;list;watch;create;update;patch;delete

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

func (a *Actuator) getControlPlaneMachines(machineList *clusterv1.MachineList) []*clusterv1.Machine {
	var cpm []*clusterv1.Machine
	for _, m := range machineList.Items {
		if m.Spec.Versions.ControlPlane != "" {
			cpm = append(cpm, m.DeepCopy())
		}
	}
	return cpm
}

// defining equality as name and namespace are equivalent and not checking any other fields.
func machinesEqual(m1 *clusterv1.Machine, m2 *clusterv1.Machine) bool {
	return m1.Name == m2.Name && m1.Namespace == m2.Namespace
}

func (a *Actuator) isNodeJoin(controlPlaneMachines []*clusterv1.Machine, newMachine *clusterv1.Machine, cluster *clusterv1.Cluster) (bool, error) {
	switch newMachine.ObjectMeta.Labels["set"] {
	case "node":
		return true, nil
	case "controlplane":
		controlPlaneExists := false
		for _, cm := range controlPlaneMachines {
			m, err := actuators.NewMachineScope(actuators.MachineScopeParams{
				Machine: cm,
				Cluster: cluster,
				Client:  a.client,
			})
			if err != nil {
				return false, errors.Wrapf(err, "failed to create machine scope for machine %q", cm.Name)
			}

			computeSvc := compute.NewService(m.Scope)
			controlPlaneExists, err = computeSvc.MachineExists(m)
			if err != nil {
				return false, errors.Wrapf(err, "failed to verify existence of machine %q", m.Name())
			}
			if controlPlaneExists {
				break
			}
		}

		klog.V(2).Infof("Machine %q should join the controlplane: %t", newMachine.Name, controlPlaneExists)
		return controlPlaneExists, nil
	default:
		errMsg := fmt.Sprintf("Unknown value %q for label \"set\" on machine %q, skipping machine creation", newMachine.ObjectMeta.Labels["set"], newMachine.Name)
		klog.Errorf(errMsg)
		err := errors.Errorf(errMsg)
		return false, err
	}
}

// Create creates a machine and is invoked by the machine controller.
func (a *Actuator) Create(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	klog.Infof("Creating machine %v for cluster %v", machine.Name, cluster.Name)

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{Machine: machine, Cluster: cluster, Client: a.client})
	if err != nil {
		return errors.Errorf("failed to create scope: %+v", err)
	}

	defer scope.Close()

	computeSvc := compute.NewService(scope.Scope)
	networkSvc := network.NewService(scope.Scope)

	clusterMachines, err := scope.MachineClient.List(v1.ListOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to retrieve machines in cluster %q", cluster.Name)
	}
	controlPlaneMachines := a.getControlPlaneMachines(clusterMachines)
	isNodeJoin, err := a.isNodeJoin(controlPlaneMachines, machine, cluster)
	if err != nil {
		return errors.Wrapf(err, "failed to determine whether machine %q should join cluster %q", machine.Name, cluster.Name)
	}

	var bootstrapToken string
	if isNodeJoin {
		bootstrapToken, err = a.getNodeJoinToken(cluster)
		if err != nil {
			return errors.Wrapf(err, "failed to obtain token for node %q to join cluster %q", machine.Name, cluster.Name)
		}
	}

	kubeConfig := ""

	if len(controlPlaneMachines) > 0 {
		kubeConfig, err = a.GetKubeConfig(cluster, nil)
		if err != nil {
			return errors.Wrapf(err, "failed to retrieve kubeconfig while creating machine %q", machine.Name)
		}
	}

	nic, err := networkSvc.CreateDefaultVMNetworkInterface(scope.ClusterConfig.ResourceGroup, scope.Machine)
	if err != nil {
		klog.Errorf("Unable to create VM network interface: %+v", err)
		return &controllerError.RequeueAfterError{
			RequeueAfter: time.Second * 30,
		}
	}

	if scope.Network().APIServerLB.BackendPool.ID == "" {
		klog.Errorf("Unable to find backend pool ID. Retrying...")
		return &controllerError.RequeueAfterError{
			RequeueAfter: time.Second * 15,
		}
	}

	err = networkSvc.ReconcileNICBackendPool(*nic.Name, scope.Network().APIServerLB.BackendPool.ID)
	if err != nil {
		klog.Errorf("Unable to reconcile backend pool attachment: %+v", err)
		return &controllerError.RequeueAfterError{
			RequeueAfter: time.Second * 30,
		}
	}

	pip, err := networkSvc.CreateOrUpdatePublicIPAddress(scope.ClusterConfig.ResourceGroup, scope.Machine.Name, networkSvc.GetDefaultPublicIPZone())
	if err != nil {
		klog.Errorf("Unable to create public IP: %+v", err)
		return &controllerError.RequeueAfterError{
			RequeueAfter: time.Second * 30,
		}
	}

	err = networkSvc.ReconcileNICPublicIP(*nic.Name, pip)
	if err != nil {
		klog.Errorf("Unable to reconcile public IP attachment: %+v", err)
		return &controllerError.RequeueAfterError{
			RequeueAfter: time.Second * 30,
		}
	}

	i, err := computeSvc.CreateOrGetMachine(scope, bootstrapToken, kubeConfig)
	if err != nil {
		klog.Errorf("network not ready to launch instances yet: %+v", err)
		return &controllerError.RequeueAfterError{
			RequeueAfter: time.Minute,
		}
	}

	scope.MachineStatus.VMID = &i.ID

	// TODO: Is this still required with scope.Close()?
	//return a.updateAnnotations(cluster, machine)

	// TODO: update once machine controllers have a way to indicate a machine has been provisoned. https://github.com/kubernetes-sigs/cluster-api/issues/253
	// Seeing a node cannot be purely relied upon because the provisioned control plane will not be registering with
	// the stack that provisions it.
	if machine.Annotations == nil {
		machine.Annotations = map[string]string{}
	}

	machine.Annotations["cluster-api-provider-azure"] = "true"

	return nil
}

func (a *Actuator) getNodeJoinToken(cluster *clusterv1.Cluster) (string, error) {
	controlPlaneDNSName, err := a.GetIP(cluster, nil)
	if err != nil {
		return "", errors.Errorf("failed to retrieve controlplane (GetIP): %+v", err)
	}

	controlPlaneURL := fmt.Sprintf("https://%s:6443", controlPlaneDNSName)

	kubeConfig, err := a.GetKubeConfig(cluster, nil)
	if err != nil {
		return "", errors.Wrapf(err, "failed to retrieve kubeconfig for cluster %q.", cluster.Name)
	}

	clientConfig, err := clientcmd.BuildConfigFromKubeconfigGetter(controlPlaneURL, func() (*clientcmdapi.Config, error) {
		return clientcmd.Load([]byte(kubeConfig))
	})

	if err != nil {
		return "", errors.Wrapf(err, "failed to get client config for cluster at %q", controlPlaneURL)
	}

	coreClient, err := corev1.NewForConfig(clientConfig)
	if err != nil {
		return "", errors.Wrapf(err, "failed to initialize new corev1 client")
	}

	bootstrapToken, err := tokens.NewBootstrap(coreClient, 10*time.Minute)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create new bootstrap token")
	}

	return bootstrapToken, nil
}

// Delete an existing machine based on the cluster and machine spec passed.
// Will block until the machine has been successfully deleted, or an error is returned.
func (a *Actuator) Delete(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	klog.Infof("Deleting machine %v for cluster %v.", machine.Name, cluster.Name)

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{Machine: machine, Cluster: cluster, Client: a.client})
	if err != nil {
		return errors.Errorf("failed to create scope: %+v", err)
	}

	defer scope.Close()

	computeSvc := compute.NewService(scope.Scope)
	networkSvc := network.NewService(scope.Scope)
	resourcesSvc := resources.NewService(scope.Scope)

	// Check if VM exists
	vm, err := computeSvc.VMIfExists(resourcesSvc.GetVMName(machine))
	if err != nil {
		return fmt.Errorf("error checking if vm exists: %v", err)
	}
	if vm == nil {
		return fmt.Errorf("couldn't find vm for machine: %v", machine.Name)
	}

	avm, err := scope.VM.Get(scope.Context, scope.ClusterConfig.ResourceGroup, vm.Name, "")
	if err != nil {
		return errors.Errorf("could not set vm: %v", err)
	}

	osDiskName := avm.VirtualMachineProperties.StorageProfile.OsDisk.Name
	nicID := (*avm.VirtualMachineProperties.NetworkProfile.NetworkInterfaces)[0].ID

	// delete the VM instance
	vmDeleteFuture, err := computeSvc.DeleteVM(scope.ClusterConfig.ResourceGroup, resourcesSvc.GetVMName(machine))
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
	err = networkSvc.DeletePublicIPAddress(scope.ClusterConfig.ResourceGroup, networkSvc.GetPublicIPName(machine))
	if err != nil {
		return fmt.Errorf("error deleting public IP address: %v", err)
	}

	return nil
}

// TODO: Implement method
// isMachineOudated checks that no immutable fields have been updated in an
// Update request.
// Returns a bool indicating if an attempt to change immutable state occurred.
//  - true:  An attempt to change immutable state occurred.
//  - false: Immutable state was untouched.
/*
func (a *Actuator) isMachineOutdated(machineSpec *v1alpha1.AzureMachineProviderSpec, instance *v1alpha1.VM) bool {

	// No immutable state changes found.
	return false
}
*/

// Update updates a machine and is invoked by the Machine Controller.
// If the Update attempts to mutate any immutable state, the method will error
// and no updates will be performed.
func (a *Actuator) Update(ctx context.Context, cluster *clusterv1.Cluster, goalMachine *clusterv1.Machine) error {
	klog.Infof("Updating machine %v for cluster %v.", goalMachine.Name, cluster.Name)

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{Machine: goalMachine, Cluster: cluster, Client: a.client})
	if err != nil {
		return errors.Errorf("failed to create scope: %+v", err)
	}

	defer scope.Close()

	computeSvc := compute.NewService(scope.Scope)
	resourcesSvc := resources.NewService(scope.Scope)

	/*
		status, err := a.status(goalMachine)
		if err != nil {
			return err
		}
		currentMachine := (*clusterv1.Machine)(status)
	*/

	currentMachine := scope.Machine

	if currentMachine == nil {
		vm, err := computeSvc.VMIfExists(resourcesSvc.GetVMName(goalMachine))
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
		klog.Infof("no need to update machine: %v", currentMachine.ObjectMeta.Name)
		return nil
	}

	// update master inplace
	if isMasterMachine(scope.MachineConfig.Roles) {
		klog.Infof("updating master machine %v in place", currentMachine.ObjectMeta.Name)
		err = a.updateMaster(cluster, currentMachine, goalMachine)
		if err != nil {
			return fmt.Errorf("error updating master machine %v in place: %v", currentMachine.ObjectMeta.Name, err)
		}
		// TODO: Is this still required with scope.Close()?
		//return a.updateStatus(goalMachine)
		return nil
	}
	// delete and recreate machine for nodes
	klog.Infof("replacing node machine %v", currentMachine.ObjectMeta.Name)
	err = a.Delete(ctx, cluster, currentMachine)
	if err != nil {
		return fmt.Errorf("error updating node machine %v, deleting node machine failed: %v", currentMachine.ObjectMeta.Name, err)
	}
	err = a.Create(ctx, cluster, goalMachine)
	if err != nil {
		klog.Errorf("error updating node machine %v, creating node machine failed: %v", goalMachine.ObjectMeta.Name, err)
	}
	return nil
}

// Exists test for the existence of a machine and is invoked by the Machine Controller
func (a *Actuator) Exists(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) (bool, error) {
	klog.Infof("Checking if machine %v for cluster %v exists", machine.Name, cluster.Name)

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{Machine: machine, Cluster: cluster, Client: a.client})
	if err != nil {
		return false, errors.Errorf("failed to create scope: %+v", err)
	}

	defer scope.Close()

	computeSvc := compute.NewService(scope.Scope)

	// TODO worry about pointers. instance if exists returns *any* instance
	if scope.MachineStatus.VMID == nil {
		return false, nil
	}

	instance, err := computeSvc.VMIfExists(*scope.MachineStatus.VMID)
	if err != nil {
		return false, errors.Errorf("failed to retrieve instance: %+v", err)
	}

	if instance == nil {
		return false, nil
	}

	klog.Infof("Found instance for machine %q: %v", machine.Name, instance)

	switch instance.State {
	case v1alpha1.VMStateSucceeded:
		klog.Infof("Machine %v is running", *scope.MachineStatus.VMID)
	case v1alpha1.VMStateUpdating:
		klog.Infof("Machine %v is updating", *scope.MachineStatus.VMID)
	default:
		return false, nil
	}

	/*
		if err := a.reconcileLBAttachment(scope, machine, instance); err != nil {
			return true, err
		}
	*/

	return true, nil
}

// Old methods

// TODO: Remove old methods
func (a *Actuator) updateMaster(cluster *clusterv1.Cluster, currentMachine *clusterv1.Machine, goalMachine *clusterv1.Machine) error {
	//klog.Infof("Creating machine %v for cluster %v", machine.Name, cluster.Name)

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{Machine: goalMachine, Cluster: cluster, Client: a.client})
	if err != nil {
		return errors.Errorf("failed to create scope: %+v", err)
	}

	defer scope.Close()

	computeSvc := compute.NewService(scope.Scope)
	resourcesSvc := resources.NewService(scope.Scope)

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
		commandRunFuture, err := computeSvc.RunCommand(scope.ClusterConfig.ResourceGroup, resourcesSvc.GetVMName(goalMachine), cmd)
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
		nodeName := strings.ToLower(resourcesSvc.GetVMName(goalMachine))
		// prepare node for maintenance
		cmd := fmt.Sprintf("sudo kubectl drain %s --kubeconfig /etc/kubernetes/admin.conf --ignore-daemonsets;"+
			"sudo apt-get install kubelet=%s;", nodeName, goalMachine.Spec.Versions.Kubelet+"-00")
		// mark the node as schedulable
		cmd += fmt.Sprintf("sudo kubectl uncordon %s --kubeconfig /etc/kubernetes/admin.conf;", nodeName)

		commandRunFuture, err := computeSvc.RunCommand(scope.ClusterConfig.ResourceGroup, resourcesSvc.GetVMName(goalMachine), cmd)
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

func isMasterMachine(roles []v1alpha1.MachineRole) bool {
	for _, r := range roles {
		if r == v1alpha1.Master {
			return true
		}
	}
	return false
}

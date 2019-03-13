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
	"strings"
	"time"

	"github.com/pkg/errors"
	apicorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

const (
	defaultTokenTTL = 10 * time.Minute
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

// ActuatorParams holds parameter information for Actuator.
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

// GetControlPlaneMachines retrieves all control plane nodes from a MachineList
func GetControlPlaneMachines(machineList *clusterv1.MachineList) []*clusterv1.Machine {
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

func (a *Actuator) isNodeJoin(scope *actuators.MachineScope, controlPlaneMachines []*clusterv1.Machine) (bool, error) {
	switch set := scope.Machine.ObjectMeta.Labels["set"]; set {
	case "node":
		return true, nil
	case "controlplane":
		for _, cm := range controlPlaneMachines {
			m, err := actuators.NewMachineScope(actuators.MachineScopeParams{
				Machine: cm,
				Cluster: scope.Cluster,
				Client:  a.client,
			})

			if err != nil {
				return false, errors.Wrapf(err, "failed to create machine scope for machine %q", cm.Name)
			}

			computeSvc := compute.NewService(m.Scope)

			ok, err := computeSvc.MachineExists(m)
			if err != nil {
				return false, errors.Wrapf(err, "failed to verify existence of machine %q", m.Name())
			}

			klog.V(2).Infof("Machine %q should join the controlplane: %t", scope.Machine.Name, ok)
			return ok, nil
		}

		return false, nil
	default:
		return false, errors.Errorf("Unknown value %q for label `set` on machine %q, skipping machine creation", set, scope.Machine.Name)
	}
}

// Create creates a machine and is invoked by the machine controller.
func (a *Actuator) Create(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	klog.Infof("Creating machine %v for cluster %v", machine.Name, cluster.Name)

	coreClient, bootstrapErr := a.coreV1Client(cluster)
	if bootstrapErr != nil {
		return errors.Wrapf(bootstrapErr, "failed to retrieve corev1 client for cluster %q", cluster.Name)
	}

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{Machine: machine, Cluster: cluster, Client: a.client, CoreClient: coreClient})
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

	controlPlaneMachines := GetControlPlaneMachines(clusterMachines)

	isNodeJoin, err := a.isNodeJoin(scope, controlPlaneMachines)
	if err != nil {
		return errors.Wrapf(err, "failed to determine whether machine %q should join cluster %q", machine.Name, cluster.Name)
	}

	var bootstrapToken string
	if isNodeJoin {

		bootstrapToken, bootstrapErr = tokens.NewBootstrap(scope.CoreClient, defaultTokenTTL)
		if bootstrapErr != nil {
			return errors.Wrapf(bootstrapErr, "failed to create new bootstrap token")
		}
	}

	kubeConfig, err := a.GetKubeConfig(cluster, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to retrieve kubeconfig while creating machine %q", machine.Name)
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

	pip, err := networkSvc.CreateOrUpdatePublicIPAddress(scope.ClusterConfig.ResourceGroup, networkSvc.GetPublicIPName(machine), networkSvc.GetDefaultPublicIPZone())
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

	vm, err := computeSvc.CreateOrGetMachine(scope, bootstrapToken, kubeConfig)
	if err != nil {
		klog.Errorf("failed to create or get machine: %+v", err)
		return &controllerError.RequeueAfterError{
			RequeueAfter: time.Minute,
		}
	}

	scope.MachineStatus.VMID = &vm.ID
	scope.MachineStatus.VMState = &vm.State

	// TODO: update once machine controllers have a way to indicate a machine has been provisoned. https://github.com/kubernetes-sigs/cluster-api/issues/253
	// Seeing a node cannot be purely relied upon because the provisioned control plane will not be registering with
	// the stack that provisions it.
	if machine.Annotations == nil {
		machine.Annotations = map[string]string{}
	}

	machine.Annotations["cluster-api-provider-azure"] = "true"

	return nil
}

func (a *Actuator) coreV1Client(cluster *clusterv1.Cluster) (corev1.CoreV1Interface, error) {
	controlPlaneDNSName, err := a.GetIP(cluster, nil)
	if err != nil {
		return nil, errors.Errorf("failed to retrieve controlplane (GetIP): %+v", err)
	}

	controlPlaneURL := fmt.Sprintf("https://%s:6443", controlPlaneDNSName)

	kubeConfig, err := a.GetKubeConfig(cluster, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve kubeconfig for cluster %q.", cluster.Name)
	}

	clientConfig, err := clientcmd.BuildConfigFromKubeconfigGetter(controlPlaneURL, func() (*clientcmdapi.Config, error) {
		return clientcmd.Load([]byte(kubeConfig))
	})

	if err != nil {
		return nil, errors.Wrapf(err, "failed to get client config for cluster at %q", controlPlaneURL)
	}

	return corev1.NewForConfig(clientConfig)
}

// Delete deletes a machine and is invoked by the Machine Controller
// TODO: Rewrite method
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

	// delete the VM
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

// isMachineOutdated checks that no immutable fields have been updated in an
// Update request.
// Returns a bool indicating if an attempt to change immutable state occurred.
//  - true:  An attempt to change immutable state occurred.
//  - false: Immutable state was untouched.
func (a *Actuator) isMachineOutdated(machineSpec *v1alpha1.AzureMachineProviderSpec, vm *v1alpha1.VM) bool {
	// VM Size
	if machineSpec.VMSize != vm.VMSize {
		return true
	}

	// TODO: Add additional checks for immutable fields

	// No immutable state changes found.
	return false
}

// Update updates a machine and is invoked by the Machine Controller.
// If the Update attempts to mutate any immutable state, the method will error
// and no updates will be performed.
func (a *Actuator) Update(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	klog.Infof("Updating machine %v for cluster %v.", machine.Name, cluster.Name)

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{Machine: machine, Cluster: cluster, Client: a.client})
	if err != nil {
		return errors.Errorf("failed to create scope: %+v", err)
	}

	defer scope.Close()

	computeSvc := compute.NewService(scope.Scope)

	// Get the current vm description from Azure.
	vmDescription, err := computeSvc.VMIfExists(*scope.MachineStatus.VMID)
	if err != nil {
		return errors.Errorf("failed to get vm: %+v", err)
	}

	// We can now compare the various Azure state to the state we were passed.
	// We will check immutable state first, in order to fail quickly before
	// moving on to state that we can mutate.
	if a.isMachineOutdated(scope.MachineConfig, vmDescription) {
		return errors.Errorf("found attempt to change immutable state")
	}

	// TODO: Uncomment after implementing tagging.
	// Ensure that the tags are correct.
	/*
		_, err = a.ensureTags(computeSvc, machine, scope.MachineStatus.VMID, scope.MachineConfig.AdditionalTags)
		if err != nil {
			return errors.Errorf("failed to ensure tags: %+v", err)
		}
	*/

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

	// TODO worry about pointers. vm if exists returns *any* vm
	if scope.MachineStatus.VMID == nil {
		return false, nil
	}

	vm, err := computeSvc.VMIfExists(*scope.MachineStatus.VMID)
	if err != nil {
		return false, errors.Errorf("failed to retrieve vm: %+v", err)
	}

	if vm == nil {
		return false, nil
	}

	klog.Infof("Found vm for machine %q: %v", machine.Name, vm)

	switch vm.State {
	case v1alpha1.VMStateSucceeded:
		klog.Infof("Machine %v is running", *scope.MachineStatus.VMID)
	case v1alpha1.VMStateUpdating:
		klog.Infof("Machine %v is updating", *scope.MachineStatus.VMID)
	default:
		return false, nil
	}

	scope.MachineStatus.VMState = &vm.State

	if machine.Spec.ProviderID == nil || *machine.Spec.ProviderID == "" {
		// TODO: This should be unified with the logic for getting the nodeRef, and
		// should potentially leverage the code that already exists in
		// kubernetes/cloud-provider-azure
		providerID := fmt.Sprintf("azure:////%s", *scope.MachineStatus.VMID)
		scope.Machine.Spec.ProviderID = &providerID
	}

	// Set the Machine NodeRef.
	if machine.Status.NodeRef == nil {
		nodeRef, err := a.getNodeReference(scope)
		if err != nil {
			klog.Warningf("Failed to set nodeRef: %v", err)
			return true, nil
		}

		scope.Machine.Status.NodeRef = nodeRef
		klog.Infof("Setting machine %q nodeRef to %q", scope.Name(), nodeRef.Name)
	}

	return true, nil
}

func (a *Actuator) getNodeReference(scope *actuators.MachineScope) (*apicorev1.ObjectReference, error) {
	instanceID := *scope.MachineStatus.VMID

	coreClient, err := a.coreV1Client(scope.Cluster)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve corev1 client for cluster %q", scope.Cluster.Name)
	}

	listOpt := metav1.ListOptions{}

	for {
		nodeList, err := coreClient.Nodes().List(listOpt)
		if err != nil {
			return nil, errors.Wrap(err, "failed to query cluster nodes")
		}

		for _, node := range nodeList.Items {
			// TODO(vincepri): Improve this comparison without relying on substrings.
			if strings.Contains(node.Spec.ProviderID, instanceID) {
				return &apicorev1.ObjectReference{
					Kind:       node.Kind,
					APIVersion: node.APIVersion,
					Name:       node.Name,
				}, nil
			}
		}

		listOpt.Continue = nodeList.Continue
		if listOpt.Continue == "" {
			break
		}
	}

	return nil, errors.Errorf("no node found for machine %q", scope.Name())
}

// Old methods

// TODO: Remove old methods
/*
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

// TODO: Remove usage of MachineRole
func isMasterMachine(roles []v1alpha1.MachineRole) bool {
	for _, r := range roles {
		if r == v1alpha1.Master {
			return true
		}
	}
	return false
}
*/

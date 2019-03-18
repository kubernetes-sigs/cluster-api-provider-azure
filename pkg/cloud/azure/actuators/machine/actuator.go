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
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2018-10-01/compute"
	"github.com/pkg/errors"
	apicorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/certificates"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/config"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/virtualmachineextensions"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/virtualmachines"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/deployer"
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

			vmInterface, err := virtualmachines.NewService(m.Scope).Get(context.Background(), &virtualmachines.Spec{Name: m.Machine.Name})
			if err != nil && vmInterface == nil {
				klog.V(2).Infof("Machine %q should join the controlplane: false", m.Machine.Name)
				return false, nil
			}

			if err != nil {
				return false, errors.Wrapf(err, "failed to verify existence of machine %q", m.Name())
			}

			vmExtSpec := &virtualmachineextensions.Spec{
				Name:   "startupScript",
				VMName: m.Machine.Name,
			}

			vmExt, err := virtualmachineextensions.NewService(scope.Scope).Get(context.Background(), vmExtSpec)
			if err != nil && vmExt == nil {
				klog.V(2).Infof("Machine %q should join the controlplane: false", m.Machine.Name)
				return false, nil
			}

			klog.V(2).Infof("Machine %q should join the controlplane: true", m.Machine.Name)
			return true, nil
		}

		return false, nil
	default:
		return false, errors.Errorf("Unknown value %q for label `set` on machine %q, skipping machine creation", set, scope.Machine.Name)
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

	bootstrapToken, err := a.checkControlPlaneMachines(scope, cluster, machine)
	if err != nil {
		return errors.Wrapf(err, "failed to check control plane machines in cluster %s", cluster.Name)
	}

	// kubeConfig, err := a.GetKubeConfig(cluster, nil)
	// if err != nil {
	// 	return errors.Wrapf(err, "failed to retrieve kubeconfig while creating machine %q", machine.Name)
	// }

	networkInterfaceSpec := &networkinterfaces.Spec{
		Name:     fmt.Sprintf("%s-nic", machine.Name),
		VNETName: azure.DefaultVnetName,
	}
	switch set := scope.Machine.ObjectMeta.Labels["set"]; set {
	case "node":
		networkInterfaceSpec.SubnetName = azure.DefaultNodeSubnetName
	case "controlplane":
		networkInterfaceSpec.SubnetName = azure.DefaultControlPlaneSubnetName
		networkInterfaceSpec.PublicLoadBalancerName = azure.DefaultPublicLBName
		networkInterfaceSpec.InternalLoadBalancerName = azure.DefaultInternalLBName
		networkInterfaceSpec.NatRule = 0
	default:
		return errors.Errorf("Unknown value %q for label `set` on machine %q, skipping machine creation", set, scope.Machine.Name)
	}

	err = networkinterfaces.NewService(scope.Scope).CreateOrUpdate(ctx, networkInterfaceSpec)
	if err != nil {
		klog.Errorf("Unable to create VM network interface: %+v", err)
		return &controllerError.RequeueAfterError{
			RequeueAfter: time.Second * 30,
		}
	}

	decoded, err := base64.StdEncoding.DecodeString(scope.MachineConfig.SSHPublicKey)
	if err != nil {
		errors.Wrapf(err, "failed to decode ssh public key")
	}

	vmSpec := &virtualmachines.Spec{
		Name:       scope.Machine.Name,
		NICName:    networkInterfaceSpec.Name,
		SSHKeyData: string(decoded),
	}
	err = virtualmachines.NewService(scope.Scope).CreateOrUpdate(context.Background(), vmSpec)
	if err != nil {
		klog.Errorf("failed to create or get machine: %+v", err)
		return &controllerError.RequeueAfterError{
			RequeueAfter: time.Minute,
		}
	}

	scriptData, err := config.GetVMStartupScript(scope, bootstrapToken)
	if err != nil {
		return errors.Wrapf(err, "failed to get vm startup script")
	}

	vmExtSpec := &virtualmachineextensions.Spec{
		Name:       "startupScript",
		VMName:     scope.Machine.Name,
		ScriptData: base64.StdEncoding.EncodeToString([]byte(scriptData)),
	}
	err = virtualmachineextensions.NewService(scope.Scope).CreateOrUpdate(context.Background(), vmExtSpec)
	if err != nil {
		klog.Errorf("failed to create or get machine: %+v", err)
		return &controllerError.RequeueAfterError{
			RequeueAfter: time.Minute,
		}
	}

	// TODO: update once machine controllers have a way to indicate a machine has been provisoned. https://github.com/kubernetes-sigs/cluster-api/issues/253
	// Seeing a node cannot be purely relied upon because the provisioned control plane will not be registering with
	// the stack that provisions it.
	if machine.Annotations == nil {
		machine.Annotations = map[string]string{}
	}

	machine.Annotations["cluster-api-provider-azure"] = "true"

	return nil
}

func (a *Actuator) checkControlPlaneMachines(scope *actuators.MachineScope, cluster *clusterv1.Cluster, machine *clusterv1.Machine) (string, error) {
	clusterMachines, err := scope.MachineClient.List(v1.ListOptions{})
	if err != nil {
		return "", errors.Wrapf(err, "failed to retrieve machines in cluster %s", cluster.Name)
	}

	controlPlaneMachines := GetControlPlaneMachines(clusterMachines)

	isNodeJoin, err := a.isNodeJoin(scope, controlPlaneMachines)
	if err != nil {
		return "", errors.Wrapf(err, "failed to determine whether machine %s should join cluster %s", machine.Name, cluster.Name)
	}

	var bootstrapToken string
	if isNodeJoin {
		if scope.ClusterConfig == nil {
			return "", errors.Errorf("failed to retrieve corev1 client for empty kubeconfig %s", cluster.Name)
		}
		bootstrapToken, err = certificates.CreateNewBootstrapToken(scope.ClusterConfig.AdminKubeconfig, defaultTokenTTL)
		if err != nil {
			return "", errors.Wrapf(err, "failed to create new bootstrap token")
		}
	}
	return bootstrapToken, nil
}

func (a *Actuator) coreV1Client(kubeconfig string) (corev1.CoreV1Interface, error) {
	clientConfig, err := clientcmd.NewClientConfigFromBytes([]byte(kubeconfig))

	if err != nil {
		return nil, errors.Wrapf(err, "failed to get client config for cluster")
	}

	cfg, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get client config for cluster")
	}

	return corev1.NewForConfig(cfg)
}

// Delete deletes a machine and is invoked by the Machine Controller.
// TODO: Rewrite method
func (a *Actuator) Delete(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	klog.Infof("Deleting machine %v for cluster %v.", machine.Name, cluster.Name)

	scope, err := actuators.NewMachineScope(actuators.MachineScopeParams{Machine: machine, Cluster: cluster, Client: a.client})
	if err != nil {
		return errors.Wrapf(err, "failed to create scope")
	}

	defer scope.Close()

	vmSpec := &virtualmachines.Spec{
		Name: scope.Machine.Name,
	}

	err = virtualmachines.NewService(scope.Scope).Delete(context.Background(), vmSpec)
	if err != nil {
		klog.Errorf("failed to delete machine: %+v", err)
		return &controllerError.RequeueAfterError{
			RequeueAfter: time.Minute,
		}
	}

	networkInterfaceSpec := &networkinterfaces.Spec{
		Name:     fmt.Sprintf("%s-nic", machine.Name),
		VNETName: azure.DefaultVnetName,
	}

	err = networkinterfaces.NewService(scope.Scope).Delete(ctx, networkInterfaceSpec)
	if err != nil {
		klog.Errorf("Unable to delete network interface: %+v", err)
		return &controllerError.RequeueAfterError{
			RequeueAfter: time.Second * 30,
		}
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

	vmSpec := &virtualmachines.Spec{
		Name: scope.Machine.Name,
	}
	vmInterface, err := virtualmachines.NewService(scope.Scope).Get(ctx, vmSpec)
	if err != nil {
		return errors.Errorf("failed to get vm: %+v", err)
	}

	vm, ok := vmInterface.(compute.VirtualMachine)
	if !ok {
		return errors.New("returned incorrect vm interface")
	}

	// We can now compare the various Azure state to the state we were passed.
	// We will check immutable state first, in order to fail quickly before
	// moving on to state that we can mutate.
	if a.isMachineOutdated(scope.MachineConfig, converters.SDKToVM(vm)) {
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

	exists, err := isVMExists(ctx, scope)
	if err != nil {
		return false, err
	} else if !exists {
		return false, nil
	}

	switch *scope.MachineStatus.VMState {
	case v1alpha1.VMStateSucceeded:
		klog.Infof("Machine %v is running", *scope.MachineStatus.VMID)
	case v1alpha1.VMStateUpdating:
		klog.Infof("Machine %v is updating", *scope.MachineStatus.VMID)
	default:
		return false, nil
	}

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

func isVMExists(ctx context.Context, scope *actuators.MachineScope) (bool, error) {
	vmSpec := &virtualmachines.Spec{
		Name: scope.Machine.Name,
	}
	vmInterface, err := virtualmachines.NewService(scope.Scope).Get(ctx, vmSpec)

	if err != nil && vmInterface == nil {
		return false, nil
	}

	if err != nil {
		return false, errors.Wrap(err, "Failed to get vm")
	}

	vm, ok := vmInterface.(compute.VirtualMachine)
	if !ok {
		return false, errors.New("returned incorrect vm interface")
	}

	klog.Infof("Found vm for machine %s", scope.Machine.Name)

	vmExtSpec := &virtualmachineextensions.Spec{
		Name:   "startupScript",
		VMName: scope.Machine.Name,
	}

	vmExt, err := virtualmachineextensions.NewService(scope.Scope).Get(context.Background(), vmExtSpec)
	if err != nil && vmExt == nil {
		return false, nil
	}

	if err != nil {
		return false, errors.Wrapf(err, "failed to get vm extension")
	}

	vmState := v1alpha1.VMState(*vm.ProvisioningState)

	scope.MachineStatus.VMID = vm.ID
	scope.MachineStatus.VMState = &vmState
	return true, nil
}

func (a *Actuator) getNodeReference(scope *actuators.MachineScope) (*apicorev1.ObjectReference, error) {
	if scope.MachineStatus.VMID == nil {
		return nil, errors.Errorf("instance id is empty for machine %s", scope.Machine.Name)
	}

	instanceID := *scope.MachineStatus.VMID

	if scope.ClusterConfig == nil {
		return nil, errors.Errorf("failed to retrieve corev1 client for empty kubeconfig %s", scope.Cluster.Name)
	}

	coreClient, err := a.coreV1Client(scope.ClusterConfig.AdminKubeconfig)
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

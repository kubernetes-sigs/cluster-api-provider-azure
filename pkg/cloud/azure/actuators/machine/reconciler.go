/*
Copyright 2019 The Kubernetes Authors.

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
	"math/rand"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2018-10-01/compute"
	"github.com/pkg/errors"
	apicorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/availabilityzones"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/certificates"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/config"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/virtualmachineextensions"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/virtualmachines"

	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	// DefaultBootstrapTokenTTL default ttl for bootstrap token
	DefaultBootstrapTokenTTL = 10 * time.Minute
)

// Reconciler are list of services required by cluster actuator, easy to create a fake
type Reconciler struct {
	scope                 *actuators.MachineScope
	availabilityZonesSvc  azure.Service
	networkInterfacesSvc  azure.Service
	virtualMachinesSvc    azure.Service
	virtualMachinesExtSvc azure.Service
}

// NewReconciler populates all the services based on input scope
func NewReconciler(scope *actuators.MachineScope) *Reconciler {
	return &Reconciler{
		scope:                 scope,
		availabilityZonesSvc:  availabilityzones.NewService(scope.Scope),
		networkInterfacesSvc:  networkinterfaces.NewService(scope.Scope),
		virtualMachinesSvc:    virtualmachines.NewService(scope.Scope),
		virtualMachinesExtSvc: virtualmachineextensions.NewService(scope.Scope),
	}
}

// Create creates machine if and only if machine exists, handled by cluster-api
func (s *Reconciler) Create(ctx context.Context) error {
	// TODO: update once machine controllers have a way to indicate a machine has been provisoned. https://github.com/kubernetes-sigs/cluster-api/issues/253
	// Seeing a node cannot be purely relied upon because the provisioned control plane will not be registering with
	// the stack that provisions it.
	if s.scope.Machine.Annotations == nil {
		s.scope.Machine.Annotations = map[string]string{}
	}

	nicName := fmt.Sprintf("%s-nic", s.scope.Machine.Name)
	if err := s.createNetworkInterface(ctx, nicName); err != nil {
		return errors.Wrapf(err, "failed to create nic %s for machine %s", nicName, s.scope.Machine.Name)
	}

	if err := s.createVirtualMachine(ctx, nicName); err != nil {
		return errors.Wrapf(err, "failed to create vm %s ", s.scope.Machine.Name)
	}

	if s.scope.MachineConfig.UserDataSecret == nil {
		bootstrapToken, err := s.checkControlPlaneMachines()
		if err != nil {
			return errors.Wrap(err, "failed to check control plane machines in cluster")
		}

		scriptData, err := config.GetVMStartupScript(s.scope, bootstrapToken)
		if err != nil {
			return errors.Wrapf(err, "failed to get vm startup script")
		}

		vmExtSpec := &virtualmachineextensions.Spec{
			Name:       "startupScript",
			VMName:     s.scope.Machine.Name,
			ScriptData: base64.StdEncoding.EncodeToString([]byte(scriptData)),
		}
		err = s.virtualMachinesExtSvc.CreateOrUpdate(ctx, vmExtSpec)
		if err != nil {
			return errors.Wrap(err, "failed to create vm extension")
		}
	}

	s.scope.Machine.Annotations["cluster-api-provider-azure"] = "true"

	return nil
}

// Update updates machine if and only if machine exists, handled by cluster-api
func (s *Reconciler) Update(ctx context.Context) error {
	vmSpec := &virtualmachines.Spec{
		Name: s.scope.Machine.Name,
	}
	vmInterface, err := s.virtualMachinesSvc.Get(ctx, vmSpec)
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
	if isMachineOutdated(s.scope.MachineConfig, converters.SDKToVM(vm)) {
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

// Exists checks if machine exists
func (s *Reconciler) Exists(ctx context.Context) (bool, error) {
	exists, err := s.isVMExists(ctx)
	if err != nil {
		return false, err
	} else if !exists {
		return false, nil
	}

	switch *s.scope.MachineStatus.VMState {
	case v1alpha1.VMStateSucceeded:
		klog.Infof("Machine %v is running", *s.scope.MachineStatus.VMID)
	case v1alpha1.VMStateUpdating:
		klog.Infof("Machine %v is updating", *s.scope.MachineStatus.VMID)
	default:
		return false, nil
	}

	if s.scope.Machine.Spec.ProviderID == nil || *s.scope.Machine.Spec.ProviderID == "" {
		// TODO: This should be unified with the logic for getting the nodeRef, and
		// should potentially leverage the code that already exists in
		// kubernetes/cloud-provider-azure
		providerID := fmt.Sprintf("azure:////%s", *s.scope.MachineStatus.VMID)
		s.scope.Machine.Spec.ProviderID = &providerID
	}

	// Set the Machine NodeRef.
	if s.scope.Machine.Status.NodeRef == nil {
		nodeRef, err := getNodeReference(s.scope)
		if err != nil {
			klog.Warningf("Failed to set nodeRef: %v", err)
			return true, nil
		}

		s.scope.Machine.Status.NodeRef = nodeRef
		klog.Infof("Setting machine %s nodeRef to %s", s.scope.Name(), nodeRef.Name)
	}

	return true, nil
}

// Delete reconciles all the services in pre determined order
func (s *Reconciler) Delete(ctx context.Context) error {
	vmSpec := &virtualmachines.Spec{
		Name: s.scope.Machine.Name,
	}

	err := s.virtualMachinesSvc.Delete(ctx, vmSpec)
	if err != nil {
		return errors.Wrapf(err, "failed to delete machine")
	}

	networkInterfaceSpec := &networkinterfaces.Spec{
		Name:     fmt.Sprintf("%s-nic", s.scope.Machine.Name),
		VnetName: azure.GenerateVnetName(s.scope.Cluster.Name),
	}

	err = s.networkInterfacesSvc.Delete(ctx, networkInterfaceSpec)
	if err != nil {
		return errors.Wrapf(err, "Unable to delete network interface")
	}

	return nil
}

// isMachineOutdated checks that no immutable fields have been updated in an
// Update request.
// Returns a bool indicating if an attempt to change immutable state occurred.
//  - true:  An attempt to change immutable state occurred.
//  - false: Immutable state was untouched.
func isMachineOutdated(machineSpec *v1alpha1.AzureMachineProviderSpec, vm *v1alpha1.VM) bool {
	// VM Size
	if !strings.EqualFold(machineSpec.VMSize, vm.VMSize) {
		return true
	}

	// TODO: Add additional checks for immutable fields

	// No immutable state changes found.
	return false
}

func (s *Reconciler) isNodeJoin() (bool, error) {
	clusterMachines, err := s.scope.MachineClient.List(metav1.ListOptions{})
	if err != nil {
		return false, errors.Wrapf(err, "failed to retrieve machines in cluster")
	}

	switch set := s.scope.Machine.ObjectMeta.Labels[v1alpha1.MachineRoleLabel]; set {
	case v1alpha1.Node:
		return true, nil
	case v1alpha1.ControlPlane:
		for _, cm := range clusterMachines.Items {
			if cm.ObjectMeta.Labels[v1alpha1.MachineRoleLabel] == v1alpha1.ControlPlane {
				continue
			}
			vmInterface, err := s.virtualMachinesSvc.Get(context.Background(), &virtualmachines.Spec{Name: cm.Name})
			if err != nil && vmInterface == nil {
				klog.V(2).Infof("Machine %s should join the controlplane: false", s.scope.Name())
				return false, nil
			}

			if err != nil {
				return false, errors.Wrapf(err, "failed to verify existence of machine %s", cm.Name)
			}

			vmExtSpec := &virtualmachineextensions.Spec{
				Name:   "startupScript",
				VMName: cm.Name,
			}

			vmExt, err := s.virtualMachinesExtSvc.Get(context.Background(), vmExtSpec)
			if err != nil && vmExt == nil {
				klog.V(2).Infof("Machine %s should join the controlplane: false", cm.Name)
				return false, nil
			}

			klog.V(2).Infof("Machine %s should join the controlplane: true", s.scope.Name())
			return true, nil
		}

		return len(clusterMachines.Items) > 0, nil
	default:
		return false, errors.Errorf("Unknown value %s for label `set` on machine %s, skipping machine creation", set, s.scope.Name())
	}
}

func (s *Reconciler) checkControlPlaneMachines() (string, error) {
	isJoin, err := s.isNodeJoin()
	if err != nil {
		return "", errors.Wrapf(err, "failed to determine whether machine should join cluster")
	}

	var bootstrapToken string
	if isJoin {
		if s.scope.ClusterConfig == nil {
			return "", errors.Errorf("failed to retrieve corev1 client for empty kubeconfig")
		}
		bootstrapToken, err = certificates.CreateNewBootstrapToken(s.scope.ClusterConfig.AdminKubeconfig, DefaultBootstrapTokenTTL)
		if err != nil {
			return "", errors.Wrapf(err, "failed to create new bootstrap token")
		}
	}
	return bootstrapToken, nil
}

func coreV1Client(kubeconfig string) (corev1.CoreV1Interface, error) {
	if kubeconfig == "" {
		cfg, err := controllerconfig.GetConfig()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get config")
		}
		return corev1.NewForConfig(cfg)
	}
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

func (s *Reconciler) isVMExists(ctx context.Context) (bool, error) {
	vmSpec := &virtualmachines.Spec{
		Name: s.scope.Name(),
	}
	vmInterface, err := s.virtualMachinesSvc.Get(ctx, vmSpec)

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

	klog.Infof("Found vm for machine %s", s.scope.Name())

	if s.scope.MachineConfig.UserDataSecret == nil {
		vmExtSpec := &virtualmachineextensions.Spec{
			Name:   "startupScript",
			VMName: s.scope.Name(),
		}

		vmExt, err := s.virtualMachinesExtSvc.Get(ctx, vmExtSpec)
		if err != nil && vmExt == nil {
			return false, nil
		}

		if err != nil {
			return false, errors.Wrapf(err, "failed to get vm extension")
		}
	}

	vmState := v1alpha1.VMState(*vm.ProvisioningState)

	s.scope.MachineStatus.VMID = vm.ID
	s.scope.MachineStatus.VMState = &vmState
	return true, nil
}

func getNodeReference(scope *actuators.MachineScope) (*apicorev1.ObjectReference, error) {
	if scope.MachineStatus.VMID == nil {
		return nil, errors.Errorf("instance id is empty for machine %s", scope.Machine.Name)
	}

	instanceID := *scope.MachineStatus.VMID

	if scope.ClusterConfig == nil {
		return nil, errors.Errorf("failed to retrieve corev1 client for empty kubeconfig %s", scope.Cluster.Name)
	}

	coreClient, err := coreV1Client(scope.ClusterConfig.AdminKubeconfig)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve corev1 client for cluster %s", scope.Cluster.Name)
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

	return nil, errors.Errorf("no node found for machine %s", scope.Name())
}

// getVirtualMachineZone gets a random availability zones from available set,
// this will hopefully be an input from upstream machinesets so all the vms are balanced
func (s *Reconciler) getVirtualMachineZone(ctx context.Context) (string, error) {
	zonesSpec := &availabilityzones.Spec{
		VMSize: s.scope.MachineConfig.VMSize,
	}
	zonesInterface, err := s.availabilityZonesSvc.Get(ctx, zonesSpec)
	if err != nil {
		return "", errors.Wrapf(err, "failed to check availability zones for %s in region %s", s.scope.MachineConfig.VMSize, s.scope.ClusterConfig.Location)
	}
	if zonesInterface == nil {
		// if its nil, probably means no zones found
		return "", nil
	}
	zones, ok := zonesInterface.([]string)
	if !ok {
		return "", errors.New("availability zones Get returned invalid interface")
	}

	if len(zones) <= 0 {
		return "", nil
	}

	rand.Seed(time.Now().Unix()) // initialize global pseudo random generator
	return zones[rand.Intn(len(zones))], nil
}

func (s *Reconciler) createNetworkInterface(ctx context.Context, nicName string) error {
	networkInterfaceSpec := &networkinterfaces.Spec{
		Name:     nicName,
		VnetName: azure.GenerateVnetName(s.scope.Cluster.Name),
	}
	switch set := s.scope.Machine.ObjectMeta.Labels[v1alpha1.MachineRoleLabel]; set {
	case v1alpha1.Node:
		networkInterfaceSpec.SubnetName = azure.GenerateNodeSubnetName(s.scope.Cluster.Name)
	case v1alpha1.ControlPlane:
		networkInterfaceSpec.SubnetName = azure.GenerateControlPlaneSubnetName(s.scope.Cluster.Name)
		networkInterfaceSpec.PublicLoadBalancerName = azure.GeneratePublicLBName(s.scope.Cluster.Name)
		networkInterfaceSpec.InternalLoadBalancerName = azure.GenerateInternalLBName(s.scope.Cluster.Name)
		networkInterfaceSpec.NatRule = 0
	default:
		return errors.Errorf("unknown value %s for label `set` on machine %s, skipping machine creation", set, s.scope.Machine.Name)
	}

	err := s.networkInterfacesSvc.CreateOrUpdate(ctx, networkInterfaceSpec)
	if err != nil {
		return errors.Wrap(err, "unable to create VM network interface")
	}

	return err
}

func (s *Reconciler) createVirtualMachine(ctx context.Context, nicName string) error {
	decoded, err := base64.StdEncoding.DecodeString(s.scope.MachineConfig.SSHPublicKey)
	if err != nil {
		errors.Wrapf(err, "failed to decode ssh public key")
	}

	vmSpec := &virtualmachines.Spec{
		Name: s.scope.Machine.Name,
	}

	vmInterface, err := s.virtualMachinesSvc.Get(ctx, vmSpec)
	if err != nil && vmInterface == nil {
		vmZone, zoneErr := s.getVirtualMachineZone(ctx)
		if zoneErr != nil {
			return errors.Wrap(zoneErr, "failed to get availability zone")
		}

		managedIdentity := azure.GenerateManagedIdentityName(s.scope.SubscriptionID, s.scope.ClusterConfig.ResourceGroup, s.scope.Cluster.Name)

		vmSpec = &virtualmachines.Spec{
			Name:            s.scope.Machine.Name,
			NICName:         nicName,
			SSHKeyData:      string(decoded),
			Size:            s.scope.MachineConfig.VMSize,
			OSDisk:          s.scope.MachineConfig.OSDisk,
			Image:           s.scope.MachineConfig.Image,
			Zone:            vmZone,
			ManagedIdentity: managedIdentity,
		}

		userData, userDataErr := s.getCustomUserData()
		if userDataErr != nil {
			return errors.Wrapf(userDataErr, "failed to get custom script data")
		}

		if userData != "" {
			vmSpec.CustomData = userData
		}

		err = s.virtualMachinesSvc.CreateOrUpdate(ctx, vmSpec)
		if err != nil {
			return errors.Wrapf(err, "failed to create or get machine")
		}
		s.scope.Machine.Annotations["availability-zone"] = vmZone
	} else if err != nil {
		return errors.Wrap(err, "failed to get vm")
	} else {
		vm, ok := vmInterface.(compute.VirtualMachine)
		if !ok {
			return errors.New("returned incorrect vm interface")
		}
		if vm.ProvisioningState == nil {
			return errors.Errorf("vm %s is nil provisioning state, reconcile", s.scope.Machine.Name)
		}

		if *vm.ProvisioningState == "Failed" {
			// If VM failed provisioning, delete it so it can be recreated
			err = s.virtualMachinesSvc.Delete(ctx, vmSpec)
			if err != nil {
				return errors.Wrapf(err, "failed to delete machine")
			}
			return errors.Errorf("vm %s is deleted, retry creating in next reconcile", s.scope.Machine.Name)
		} else if *vm.ProvisioningState != "Succeeded" {
			return errors.Errorf("vm %s is still in provisioningstate %s, reconcile", s.scope.Machine.Name, *vm.ProvisioningState)
		}
	}

	return nil
}

func (s *Reconciler) getCustomUserData() (string, error) {
	if s.scope.MachineConfig.UserDataSecret == nil {
		return "", nil
	}
	var userDataSecret apicorev1.Secret

	if err := s.scope.CoreClient.Get(context.Background(), client.ObjectKey{Namespace: s.scope.Namespace(), Name: s.scope.MachineConfig.UserDataSecret.Name}, &userDataSecret); err != nil {
		return "", errors.Wrapf(err, "error getting user data secret %s in namespace %s", s.scope.MachineConfig.UserDataSecret.Name, s.scope.Namespace())
	}
	data, exists := userDataSecret.Data["userData"]
	if !exists {
		return "", errors.Errorf("Secret %v/%v does not have userData field set. Thus, no user data applied when creating an instance.", s.scope.Namespace(), s.scope.MachineConfig.UserDataSecret.Name)
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

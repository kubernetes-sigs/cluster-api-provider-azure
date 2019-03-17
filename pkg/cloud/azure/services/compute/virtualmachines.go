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

package compute

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2018-10-01/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/actuators"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/config"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/resources"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/record"
)

// CreateOrGetMachine will either return an existing instance or create and return an instance.
func (s *Service) CreateOrGetMachine(machine *actuators.MachineScope, bootstrapToken, kubeConfig string) (*v1alpha1.VM, error) {
	klog.V(2).Infof("Attempting to create or get machine %q", machine.Name())

	// instance id exists, try to get it
	if machine.MachineStatus.VMID != nil {
		klog.V(2).Infof("Looking up machine %q (id: %q)", machine.Name(), *machine.MachineStatus.VMID)

		instance, err := s.VMIfExists(machine.Name())
		if err != nil {
			return nil, errors.Wrapf(err, "failed to look up machine %q (id: %q)", machine.Name(), *machine.MachineStatus.VMID)
		} else if err == nil && instance != nil {
			return instance, nil
		}
	}

	instance, err := s.createVM(machine, bootstrapToken, kubeConfig)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create virtual machine %q", machine.Name())
	}

	return instance, nil
}

// VMIfExists returns the existing instance or nothing if it doesn't exist.
func (s *Service) VMIfExists(name string) (*v1alpha1.VM, error) {
	klog.V(2).Infof("Looking for instance %q", name)

	vm, err := s.scope.VM.Get(s.scope.Context, s.scope.ClusterConfig.ResourceGroup, name, "")
	if err != nil {
		if aerr, ok := err.(autorest.DetailedError); ok {
			if aerr.StatusCode.(int) == 404 {
				return nil, nil
			}
		}
		return nil, errors.Wrapf(err, "failed to describe instance: %q", name)
	}

	return converters.SDKToVM(vm), nil
}

// createVM creates a new Azure VM instance.
func (s *Service) createVM(machine *actuators.MachineScope, bootstrapToken, kubeConfig string) (*v1alpha1.VM, error) {
	klog.V(2).Infof("Creating a new instance for machine %q", machine.Name())

	input := &v1alpha1.VM{}

	startupScript, err := s.getVMStartupScript(machine, bootstrapToken)
	if err != nil {
		return nil, errors.Wrapf(err, "error while trying to get VM startupscript for machine name %s in cluster %s", machine.Name(), s.scope.Name())
	}
	input.StartupScript = startupScript
	// TODO: Set ssh key

	resourcesSvc := resources.NewService(s.scope)
	err = resourcesSvc.ValidateDeployment(machine.Machine, s.scope.ClusterConfig, machine.MachineConfig, input.StartupScript)
	if err != nil {
		return nil, fmt.Errorf("error validating deployment: %v", err)
	}

	deploymentsFuture, err := resourcesSvc.CreateOrUpdateDeployment(machine.Machine, s.scope.ClusterConfig, machine.MachineConfig, input.StartupScript)
	if err != nil {
		return nil, fmt.Errorf("error creating or updating deployment: %v", err)
	}

	err = resourcesSvc.WaitForDeploymentsCreateOrUpdateFuture(*deploymentsFuture)
	if err != nil {
		return nil, fmt.Errorf("error waiting for deployment creation or update: %v", err)
	}

	deployment, err := resourcesSvc.GetDeploymentResult(*deploymentsFuture)
	// Work around possible bugs or late-stage failures
	if deployment.Name == nil || err != nil {
		return nil, fmt.Errorf("error getting deployment result: %v", err)
	}

	vm, err := s.scope.VM.Get(s.scope.Context, machine.ClusterConfig.ResourceGroup, machine.Name(), "")
	if err != nil {
		return nil, err
	}

	record.Eventf(machine.Machine, "CreatedVM", "Created new %s vm with id %q", machine.Role(), vm.ID)
	return converters.SDKToVM(vm), nil
}

// DeleteVM deletes the virtual machine.
func (s *Service) DeleteVM(resourceGroup string, name string) (compute.VirtualMachinesDeleteFuture, error) {
	return s.scope.VM.Delete(s.scope.Context, resourceGroup, name)
}

// MachineExists will return whether or not a machine exists.
func (s *Service) MachineExists(machine *actuators.MachineScope) (bool, error) {
	var err error
	var vm *v1alpha1.VM

	if machine.MachineStatus.VMID != nil {
		vm, err = s.VMIfExists(machine.Name())
	}

	if err != nil {
		return false, errors.Wrapf(err, "failed to lookup machine %q", machine.Name())
	}
	return vm != nil, nil
}

func (s *Service) getVMStartupScript(machine *actuators.MachineScope, bootstrapToken string) (string, error) {
	var startupScript string

	if !s.scope.ClusterConfig.CAKeyPair.HasCertAndKey() {
		return "", errors.New("failed to run controlplane, missing CACertificate")
	}

	if s.scope.Network().APIServerIP.DNSName == "" {
		return "", errors.New("failed to run controlplane, APIServer DNS name not available")
	}

	caCertHash := ""

	if len(s.scope.ClusterStatus.CertificateStatus.DiscoveryHashes) > 0 {
		caCertHash = s.scope.ClusterStatus.CertificateStatus.DiscoveryHashes[0]
	}

	if caCertHash == "" {
		return "", errors.New("failed to run controlplane, missing discovery hashes")
	}

	// apply values based on the role of the machine
	switch machine.Role() {
	case "controlplane":
		// TODO: Check for existence of control plane subnet & ensure NSG is attached to subnet

		var err error

		if bootstrapToken != "" {
			klog.V(2).Infof("Allowing machine %s to join control plane for cluster %s", machine.Name(), s.scope.Name())

			startupScript, err = config.JoinControlPlane(&config.ContolPlaneJoinInput{
				CACert:            string(s.scope.ClusterConfig.CAKeyPair.Cert),
				CAKey:             string(s.scope.ClusterConfig.CAKeyPair.Key),
				CACertHash:        caCertHash,
				EtcdCACert:        string(s.scope.ClusterConfig.EtcdCAKeyPair.Cert),
				EtcdCAKey:         string(s.scope.ClusterConfig.EtcdCAKeyPair.Key),
				FrontProxyCACert:  string(s.scope.ClusterConfig.FrontProxyCAKeyPair.Cert),
				FrontProxyCAKey:   string(s.scope.ClusterConfig.FrontProxyCAKeyPair.Key),
				SaCert:            string(s.scope.ClusterConfig.SAKeyPair.Cert),
				SaKey:             string(s.scope.ClusterConfig.SAKeyPair.Key),
				BootstrapToken:    bootstrapToken,
				LBAddress:         s.scope.Network().APIServerIP.DNSName,
				KubernetesVersion: machine.Machine.Spec.Versions.ControlPlane,
			})
			if err != nil {
				return "", err
			}
		} else {
			klog.V(2).Infof("Machine %s is the first controlplane machine for cluster %s", machine.Name(), s.scope.Name())
			if !s.scope.ClusterConfig.CAKeyPair.HasCertAndKey() {
				return "", errors.New("failed to run controlplane, missing CAPrivateKey")
			}

			startupScript, err = config.NewControlPlane(&config.ControlPlaneInput{
				CACert:            string(s.scope.ClusterConfig.CAKeyPair.Cert),
				CAKey:             string(s.scope.ClusterConfig.CAKeyPair.Key),
				EtcdCACert:        string(s.scope.ClusterConfig.EtcdCAKeyPair.Cert),
				EtcdCAKey:         string(s.scope.ClusterConfig.EtcdCAKeyPair.Key),
				FrontProxyCACert:  string(s.scope.ClusterConfig.FrontProxyCAKeyPair.Cert),
				FrontProxyCAKey:   string(s.scope.ClusterConfig.FrontProxyCAKeyPair.Key),
				SaCert:            string(s.scope.ClusterConfig.SAKeyPair.Cert),
				SaKey:             string(s.scope.ClusterConfig.SAKeyPair.Key),
				LBAddress:         s.scope.Network().APIServerIP.DNSName,
				ClusterName:       s.scope.Name(),
				PodSubnet:         s.scope.Cluster.Spec.ClusterNetwork.Pods.CIDRBlocks[0],
				ServiceSubnet:     s.scope.Cluster.Spec.ClusterNetwork.Services.CIDRBlocks[0],
				ServiceDomain:     s.scope.Cluster.Spec.ClusterNetwork.ServiceDomain,
				KubernetesVersion: machine.Machine.Spec.Versions.ControlPlane,
			})

			if err != nil {
				return "", err
			}
		}

	case "node":
		// TODO: Check for existence of node subnet & ensure NSG is attached to subnet
		var err error
		startupScript, err = config.NewNode(&config.NodeInput{
			CACertHash:        caCertHash,
			BootstrapToken:    bootstrapToken,
			LBAddress:         s.scope.Network().APIServerIP.DNSName,
			KubernetesVersion: machine.Machine.Spec.Versions.ControlPlane,
		})

		if err != nil {
			return "", err
		}

	default:
		return "", errors.Errorf("Unknown node role %s", machine.Role())
	}
	return startupScript, nil
}

// Old methods

// RunCommand executes a command on the VM.
func (s *Service) RunCommand(resoureGroup string, name string, cmd string) (compute.VirtualMachinesRunCommandFuture, error) {
	cmdInput := compute.RunCommandInput{
		CommandID: to.StringPtr("RunShellScript"),
		Script:    to.StringSlicePtr([]string{cmd}),
	}
	return s.scope.VM.RunCommand(s.scope.Context, resoureGroup, name, cmdInput)
}

// WaitForVMRunCommandFuture returns when the RunCommand operation completes.
func (s *Service) WaitForVMRunCommandFuture(future compute.VirtualMachinesRunCommandFuture) error {
	return future.Future.WaitForCompletionRef(s.scope.Context, s.scope.VM.Client)
}

// WaitForVMDeletionFuture returns when the DeleteVM operation completes.
func (s *Service) WaitForVMDeletionFuture(future compute.VirtualMachinesDeleteFuture) error {
	return future.Future.WaitForCompletionRef(s.scope.Context, s.scope.VM.Client)
}

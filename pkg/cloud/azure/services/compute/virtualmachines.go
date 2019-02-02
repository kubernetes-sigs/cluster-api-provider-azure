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
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/certificates"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/config"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/record"
)

// CreateOrGetMachine will either return an existing instance or create and return an instance.
func (s *Service) CreateOrGetMachine(machine *actuators.MachineScope, bootstrapToken, kubeConfig string) (*v1alpha1.VM, error) {
	klog.V(2).Infof("Attempting to create or get machine %q", machine.Name())

	// instance id exists, try to get it
	if machine.MachineStatus.VMID != nil {
		klog.V(2).Infof("Looking up machine %q by id %q", machine.Name(), *machine.MachineStatus.VMID)

		instance, err := s.VMIfExists(*machine.MachineStatus.VMID)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to look up machine %q by id %q", machine.Name(), *machine.MachineStatus.VMID)
		} else if err == nil && instance != nil {
			return instance, nil
		}
	}

	return s.createVM(machine, bootstrapToken, kubeConfig)
}

// VMIfExists returns the existing instance or nothing if it doesn't exist.
func (s *Service) VMIfExists(name string) (*v1alpha1.VM, error) {
	vm, err := s.scope.AzureClients.VM.Get(s.scope.Context, s.scope.ClusterConfig.ResourceGroup, name, "")
	if err != nil {
		if aerr, ok := err.(autorest.DetailedError); ok {
			if aerr.StatusCode.(int) == 404 {
				return nil, nil
			}
		}
		return nil, err
	}

	return converters.SDKToVM(vm), nil
}

// createVM runs an ec2 instance.
func (s *Service) createVM(machine *actuators.MachineScope, bootstrapToken, kubeConfig string) (*v1alpha1.VM, error) {
	klog.V(2).Infof("Creating a new instance for machine %q", machine.Name())

	input := &v1alpha1.VM{}

	if !s.scope.ClusterConfig.CAKeyPair.HasCertAndKey() {
		return nil, errors.New("failed to run controlplane, missing CACertificate")
	}

	// TODO: Renable this once load balancer is implemented
	/*
		if s.scope.Network().APIServerIP.IPAddress == "" {
			return nil, errors.New("failed to run controlplane, APIServer IP not available")
		}
	*/

	caCertHash, err := certificates.GenerateCertificateHash(s.scope.ClusterConfig.CAKeyPair.Cert)
	if err != nil {
		return nil, err
	}

	// apply values based on the role of the machine
	switch machine.Role() {
	case "controlplane":
		// TODO: Check for existence of control plane subnet & ensure NSG is attached to subnet

		var cfg string
		var err error

		if bootstrapToken != "" {
			klog.V(2).Infof("Allowing machine %q to join control plane for cluster %q", machine.Name(), s.scope.Name())

			cfg, err = config.JoinControlPlane(&config.ContolPlaneJoinInput{
				CACert:           string(s.scope.ClusterConfig.CAKeyPair.Cert),
				CAKey:            string(s.scope.ClusterConfig.CAKeyPair.Key),
				CACertHash:       caCertHash,
				EtcdCACert:       string(s.scope.ClusterConfig.EtcdCAKeyPair.Cert),
				EtcdCAKey:        string(s.scope.ClusterConfig.EtcdCAKeyPair.Key),
				FrontProxyCACert: string(s.scope.ClusterConfig.FrontProxyCAKeyPair.Cert),
				FrontProxyCAKey:  string(s.scope.ClusterConfig.FrontProxyCAKeyPair.Key),
				SaCert:           string(s.scope.ClusterConfig.SAKeyPair.Cert),
				SaKey:            string(s.scope.ClusterConfig.SAKeyPair.Key),
				BootstrapToken:   bootstrapToken,
				LBAddress:        s.scope.Network().APIServerIP.IPAddress,
			})
			if err != nil {
				return input, err
			}
		} else {
			klog.V(2).Infof("Machine %q is the first controlplane machine for cluster %q", machine.Name(), s.scope.Name())
			if !s.scope.ClusterConfig.CAKeyPair.HasCertAndKey() {
				return nil, errors.New("failed to run controlplane, missing CAPrivateKey")
			}

			cfg, err = config.NewControlPlane(&config.ControlPlaneInput{
				CACert:            string(s.scope.ClusterConfig.CAKeyPair.Cert),
				CAKey:             string(s.scope.ClusterConfig.CAKeyPair.Key),
				EtcdCACert:        string(s.scope.ClusterConfig.EtcdCAKeyPair.Cert),
				EtcdCAKey:         string(s.scope.ClusterConfig.EtcdCAKeyPair.Key),
				FrontProxyCACert:  string(s.scope.ClusterConfig.FrontProxyCAKeyPair.Cert),
				FrontProxyCAKey:   string(s.scope.ClusterConfig.FrontProxyCAKeyPair.Key),
				SaCert:            string(s.scope.ClusterConfig.SAKeyPair.Cert),
				SaKey:             string(s.scope.ClusterConfig.SAKeyPair.Key),
				LBAddress:         s.scope.Network().APIServerIP.IPAddress,
				ClusterName:       s.scope.Name(),
				PodSubnet:         s.scope.Cluster.Spec.ClusterNetwork.Pods.CIDRBlocks[0],
				ServiceSubnet:     s.scope.Cluster.Spec.ClusterNetwork.Services.CIDRBlocks[0],
				ServiceDomain:     s.scope.Cluster.Spec.ClusterNetwork.ServiceDomain,
				KubernetesVersion: machine.Machine.Spec.Versions.ControlPlane,
			})

			if err != nil {
				return input, err
			}
		}

		input.StartupScript = cfg

	case "node":
		// TODO: Check for existence of node subnet & ensure NSG is attached to subnet

		cfg, err := config.NewNode(&config.NodeInput{
			CACertHash:     caCertHash,
			BootstrapToken: bootstrapToken,
			LBAddress:      s.scope.Network().APIServerIP.IPAddress,
		})

		if err != nil {
			return input, err
		}

		input.StartupScript = cfg

	default:
		return nil, errors.Errorf("Unknown node role %q", machine.Role())
	}

	// TODO: Set ssh key

	err = s.scope.Resources.ValidateDeployment(machine.Machine, s.scope.ClusterConfig, machine.MachineConfig, input.StartupScript)
	if err != nil {
		return nil, fmt.Errorf("error validating deployment: %v", err)
	}

	deploymentsFuture, err := s.scope.Resources.CreateOrUpdateDeployment(machine.Machine, s.scope.ClusterConfig, machine.MachineConfig, input.StartupScript)
	if err != nil {
		return nil, fmt.Errorf("error creating or updating deployment: %v", err)
	}

	err = s.scope.Resources.WaitForDeploymentsCreateOrUpdateFuture(*deploymentsFuture)
	if err != nil {
		return nil, fmt.Errorf("error waiting for deployment creation or update: %v", err)
	}

	deployment, err := s.scope.Resources.GetDeploymentResult(*deploymentsFuture)
	// Work around possible bugs or late-stage failures
	if deployment.Name == nil || err != nil {
		return nil, fmt.Errorf("error getting deployment result: %v", err)
	}

	vm, err := s.scope.VM.Get(s.scope.Context, machine.ClusterConfig.ResourceGroup, machine.Machine.Name, "")
	if err != nil {
		return nil, err
	}

	record.Eventf(machine.Machine, "CreatedVM", "Created new %s vm with id %q", machine.Role(), vm.ID)
	return converters.SDKToVM(vm), nil
}

// DeleteVM deletes the virtual machine.
func (s *Service) DeleteVM(resourceGroup string, name string) (compute.VirtualMachinesDeleteFuture, error) {
	return s.scope.AzureClients.VM.Delete(s.scope.Context, resourceGroup, name)
}

// MachineExists will return whether or not a machine exists.
func (s *Service) MachineExists(machine *actuators.MachineScope) (bool, error) {
	var err error
	var instance *v1alpha1.VM
	if machine.MachineStatus.VMID != nil {
		instance, err = s.VMIfExists(*machine.MachineStatus.VMID)
	}

	if err != nil {
		return false, errors.Wrapf(err, "failed to lookup machine %q", machine.Name())
	}
	return instance != nil, nil
}

// Old methods

// RunCommand executes a command on the VM.
func (s *Service) RunCommand(resoureGroup string, name string, cmd string) (compute.VirtualMachinesRunCommandFuture, error) {
	cmdInput := compute.RunCommandInput{
		CommandID: to.StringPtr("RunShellScript"),
		Script:    to.StringSlicePtr([]string{cmd}),
	}
	return s.scope.AzureClients.VM.RunCommand(s.scope.Context, resoureGroup, name, cmdInput)
}

// WaitForVMRunCommandFuture returns when the RunCommand operation completes.
func (s *Service) WaitForVMRunCommandFuture(future compute.VirtualMachinesRunCommandFuture) error {
	return future.Future.WaitForCompletionRef(s.scope.Context, s.scope.AzureClients.VM.Client)
}

// WaitForVMDeletionFuture returns when the DeleteVM operation completes.
func (s *Service) WaitForVMDeletionFuture(future compute.VirtualMachinesDeleteFuture) error {
	return future.Future.WaitForCompletionRef(s.scope.Context, s.scope.AzureClients.VM.Client)
}

/*
Copyright 2020 The Kubernetes Authors.

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

package v1alpha3

import (
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/ssh"

	utilSSH "sigs.k8s.io/cluster-api-provider-azure/util/ssh"
)

const (
	// defaultAKSVnetCIDR is the default Vnet CIDR
	defaultAKSVnetCIDR = "10.0.0.0/8"
	// defaultAKSNodeSubnetCIDR is the default Node Subnet CIDR
	defaultAKSNodeSubnetCIDR = "10.240.0.0/16"
)

// setDefaultSSHPublicKey sets the default SSHPublicKey for an AzureManagedControlPlane
func (r *AzureManagedControlPlane) setDefaultSSHPublicKey() error {
	sshKeyData := r.Spec.SSHPublicKey
	if sshKeyData == "" {
		_, publicRsaKey, err := utilSSH.GenerateSSHKey()
		if err != nil {
			return err
		}

		r.Spec.SSHPublicKey = base64.StdEncoding.EncodeToString(ssh.MarshalAuthorizedKey(publicRsaKey))
	}

	return nil
}

// setDefaultNodeResourceGroupName sets the default NodeResourceGroup for an AzureManagedControlPlane
func (r *AzureManagedControlPlane) setDefaultNodeResourceGroupName() {
	if r.Spec.NodeResourceGroupName == "" {
		r.Spec.NodeResourceGroupName = fmt.Sprintf("MC_%s_%s_%s", r.Spec.ResourceGroupName, r.Name, r.Spec.Location)
	}
}

// setDefaultVirtualNetwork sets the default VirtualNetwork for an AzureManagedControlPlane
func (r *AzureManagedControlPlane) setDefaultVirtualNetwork() {
	if r.Spec.VirtualNetwork.Name == "" {
		r.Spec.VirtualNetwork.Name = r.Name
	}
	if r.Spec.VirtualNetwork.CIDRBlock == "" {
		r.Spec.VirtualNetwork.CIDRBlock = defaultAKSVnetCIDR
	}
}

// setDefaultSubnet sets the default Subnet for an AzureManagedControlPlane
func (r *AzureManagedControlPlane) setDefaultSubnet() {
	if r.Spec.VirtualNetwork.Subnet.Name == "" {
		r.Spec.VirtualNetwork.Subnet.Name = r.Name
	}
	if r.Spec.VirtualNetwork.Subnet.CIDRBlock == "" {
		r.Spec.VirtualNetwork.Subnet.CIDRBlock = defaultAKSNodeSubnetCIDR
	}
}

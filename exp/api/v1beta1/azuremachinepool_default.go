/*
Copyright 2021 The Kubernetes Authors.

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

package v1beta1

import (
	"encoding/base64"

	"golang.org/x/crypto/ssh"
	"k8s.io/apimachinery/pkg/util/uuid"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	utilSSH "sigs.k8s.io/cluster-api-provider-azure/util/ssh"
)

// SetDefaultSSHPublicKey sets the default SSHPublicKey for an AzureMachinePool.
func (amp *AzureMachinePool) SetDefaultSSHPublicKey() error {
	if sshKeyData := amp.Spec.Template.SSHPublicKey; sshKeyData == "" {
		_, publicRsaKey, err := utilSSH.GenerateSSHKey()
		if err != nil {
			return err
		}

		amp.Spec.Template.SSHPublicKey = base64.StdEncoding.EncodeToString(ssh.MarshalAuthorizedKey(publicRsaKey))
	}
	return nil
}

// SetIdentityDefaults sets the defaults for VMSS Identity.
func (amp *AzureMachinePool) SetIdentityDefaults() {
	if amp.Spec.Identity == infrav1.VMIdentitySystemAssigned {
		if amp.Spec.RoleAssignmentName == "" {
			amp.Spec.RoleAssignmentName = string(uuid.NewUUID())
		}
	}
}

// SetSpotEvictionPolicyDefaults sets the defaults for the spot VM eviction policy.
func (amp *AzureMachinePool) SetSpotEvictionPolicyDefaults() {
	if amp.Spec.Template.SpotVMOptions != nil && amp.Spec.Template.SpotVMOptions.EvictionPolicy == nil {
		defaultPolicy := infrav1.SpotEvictionPolicyDeallocate
		if amp.Spec.Template.OSDisk.DiffDiskSettings != nil && amp.Spec.Template.OSDisk.DiffDiskSettings.Option == "Local" {
			defaultPolicy = infrav1.SpotEvictionPolicyDelete
		}
		amp.Spec.Template.SpotVMOptions.EvictionPolicy = &defaultPolicy
	}
}

// SetDiagnosticsDefaults sets the defaults for Diagnostic settings for an AzureMachinePool.
func (amp *AzureMachinePool) SetDiagnosticsDefaults() {
	bootDefault := &infrav1.BootDiagnostics{
		StorageAccountType: infrav1.ManagedDiagnosticsStorage,
	}

	if amp.Spec.Template.Diagnostics == nil {
		amp.Spec.Template.Diagnostics = &infrav1.Diagnostics{
			Boot: bootDefault,
		}
	}

	if amp.Spec.Template.Diagnostics.Boot == nil {
		amp.Spec.Template.Diagnostics.Boot = bootDefault
	}
}

// SetNetworkInterfacesDefaults sets the defaults for the network interfaces.
func (amp *AzureMachinePool) SetNetworkInterfacesDefaults() {
	// Ensure the deprecated fields and new fields are not populated simultaneously
	if (amp.Spec.Template.SubnetName != "" || amp.Spec.Template.AcceleratedNetworking != nil) && len(amp.Spec.Template.NetworkInterfaces) > 0 {
		// Both the deprecated and the new fields are both set, return without changes
		// and reject the request in the validating webhook which runs later.
		return
	}

	if len(amp.Spec.Template.NetworkInterfaces) == 0 {
		amp.Spec.Template.NetworkInterfaces = []infrav1.NetworkInterface{
			{
				SubnetName:            amp.Spec.Template.SubnetName,
				AcceleratedNetworking: amp.Spec.Template.AcceleratedNetworking,
			},
		}
		amp.Spec.Template.SubnetName = ""
		amp.Spec.Template.AcceleratedNetworking = nil
	}

	// Ensure that PrivateIPConfigs defaults to 1 if not specified.
	for i := 0; i < len(amp.Spec.Template.NetworkInterfaces); i++ {
		if amp.Spec.Template.NetworkInterfaces[i].PrivateIPConfigs == 0 {
			amp.Spec.Template.NetworkInterfaces[i].PrivateIPConfigs = 1
		}
	}
}

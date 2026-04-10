/*
Copyright The Kubernetes Authors.

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

package apifixtures

import (
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta2"
)

// CreateMachineWithSSHPublicKey returns an AzureMachine with the given SSH public key.
func CreateMachineWithSSHPublicKey(sshPublicKey string) *infrav1.AzureMachine {
	machine := HardcodedAzureMachineWithSSHKey(sshPublicKey)
	return machine
}

// CreateMachineWithUserAssignedIdentities returns an AzureMachine with user-assigned identities.
func CreateMachineWithUserAssignedIdentities(identitiesList []infrav1.UserAssignedIdentity) *infrav1.AzureMachine {
	machine := HardcodedAzureMachineWithSSHKey(GenerateSSHPublicKey(true))
	machine.Spec.Identity = infrav1.VMIdentityUserAssigned
	machine.Spec.UserAssignedIdentities = identitiesList
	return machine
}

// CreateMachineWithUserAssignedIdentitiesWithBadIdentity returns an AzureMachine with invalid identity configuration.
func CreateMachineWithUserAssignedIdentitiesWithBadIdentity(identitiesList []infrav1.UserAssignedIdentity) *infrav1.AzureMachine {
	machine := HardcodedAzureMachineWithSSHKey(GenerateSSHPublicKey(true))
	machine.Spec.Identity = infrav1.VMIdentitySystemAssigned
	machine.Spec.UserAssignedIdentities = identitiesList
	return machine
}

// HardcodedAzureMachineWithSSHKey returns an AzureMachine with hardcoded fields and the given SSH public key.
func HardcodedAzureMachineWithSSHKey(sshPublicKey string) *infrav1.AzureMachine {
	return &infrav1.AzureMachine{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: "test-cluster",
			},
		},
		Spec: infrav1.AzureMachineSpec{
			SSHPublicKey: sshPublicKey,
			OSDisk:       GenerateValidOSDisk(),
			Image: &infrav1.Image{
				SharedGallery: &infrav1.AzureSharedGalleryImage{
					SubscriptionID: "SUB123",
					ResourceGroup:  "RG123",
					Name:           "NAME123",
					Gallery:        "GALLERY1",
					Version:        "1.0.0",
				},
			},
		},
	}
}

// GenerateValidOSDisk returns a valid OSDisk configuration.
func GenerateValidOSDisk() infrav1.OSDisk {
	return infrav1.OSDisk{
		DiskSizeGB: ptr.To[int32](30),
		OSType:     infrav1.LinuxOS,
		ManagedDisk: &infrav1.ManagedDiskParameters{
			StorageAccountType: "Premium_LRS",
		},
		CachingType: string(armcompute.PossibleCachingTypesValues()[0]),
	}
}

// CreateOSDiskWithCacheType returns a valid OSDisk with the given cache type.
func CreateOSDiskWithCacheType(cacheType string) infrav1.OSDisk {
	osDisk := GenerateValidOSDisk()
	osDisk.CachingType = cacheType
	return osDisk
}

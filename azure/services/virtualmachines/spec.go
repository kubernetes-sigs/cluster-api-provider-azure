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

package virtualmachines

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/pkg/errors"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/util/generators"
)

// VMSpec defines the specification for a Virtual Machine.
type VMSpec struct {
	Name                   string
	ResourceGroup          string
	Location               string
	ClusterName            string
	Role                   string
	NICIDs                 []string
	SSHKeyData             string
	Size                   string
	AvailabilitySetID      string
	Zone                   string
	Identity               infrav1.VMIdentity
	OSDisk                 infrav1.OSDisk
	DataDisks              []infrav1.DataDisk
	UserAssignedIdentities []infrav1.UserAssignedIdentity
	SpotVMOptions          *infrav1.SpotVMOptions
	SecurityProfile        *infrav1.SecurityProfile
	AdditionalTags         infrav1.Tags
	AdditionalCapabilities *infrav1.AdditionalCapabilities
	DiagnosticsProfile     *infrav1.Diagnostics
	SKU                    resourceskus.SKU
	Image                  *infrav1.Image
	BootstrapData          string
	ProviderID             string
}

// ResourceName returns the name of the virtual machine.
func (s *VMSpec) ResourceName() string {
	return s.Name
}

// ResourceGroupName returns the name of the virtual machine.
func (s *VMSpec) ResourceGroupName() string {
	return s.ResourceGroup
}

// OwnerResourceName is a no-op for virtual machines.
func (s *VMSpec) OwnerResourceName() string {
	return ""
}

// Parameters returns the parameters for the virtual machine.
func (s *VMSpec) Parameters(ctx context.Context, existing interface{}) (params interface{}, err error) {
	if existing != nil {
		if _, ok := existing.(compute.VirtualMachine); !ok {
			return nil, errors.Errorf("%T is not a compute.VirtualMachine", existing)
		}
		// vm already exists
		return nil, nil
	}

	// VM got deleted outside of capz, do not recreate it as Machines are immutable.
	if s.ProviderID != "" {
		return nil, azure.VMDeletedError{ProviderID: s.ProviderID}
	}

	storageProfile, err := s.generateStorageProfile()
	if err != nil {
		return nil, err
	}

	securityProfile, err := s.generateSecurityProfile()
	if err != nil {
		return nil, err
	}

	osProfile, err := s.generateOSProfile()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate OS Profile")
	}

	priority, evictionPolicy, billingProfile, err := converters.GetSpotVMOptions(s.SpotVMOptions, s.OSDisk.DiffDiskSettings)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Spot VM options")
	}

	identity, err := converters.VMIdentityToVMSDK(s.Identity, s.UserAssignedIdentities)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate VM identity")
	}

	return compute.VirtualMachine{
		Plan:     converters.ImageToPlan(s.Image),
		Location: pointer.String(s.Location),
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: s.ClusterName,
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Name:        pointer.String(s.Name),
			Role:        pointer.String(s.Role),
			Additional:  s.AdditionalTags,
		})),
		VirtualMachineProperties: &compute.VirtualMachineProperties{
			AdditionalCapabilities: s.generateAdditionalCapabilities(),
			AvailabilitySet:        s.getAvailabilitySet(),
			HardwareProfile: &compute.HardwareProfile{
				VMSize: compute.VirtualMachineSizeTypes(s.Size),
			},
			StorageProfile:  storageProfile,
			SecurityProfile: securityProfile,
			OsProfile:       osProfile,
			NetworkProfile: &compute.NetworkProfile{
				NetworkInterfaces: s.generateNICRefs(),
			},
			Priority:           priority,
			EvictionPolicy:     evictionPolicy,
			BillingProfile:     billingProfile,
			DiagnosticsProfile: converters.GetDiagnosticsProfile(s.DiagnosticsProfile),
		},
		Identity: identity,
		Zones:    s.getZones(),
	}, nil
}

// generateStorageProfile generates a pointer to a compute.StorageProfile which can utilized for VM creation.
func (s *VMSpec) generateStorageProfile() (*compute.StorageProfile, error) {
	storageProfile := &compute.StorageProfile{
		OsDisk: &compute.OSDisk{
			Name:         pointer.String(azure.GenerateOSDiskName(s.Name)),
			OsType:       compute.OperatingSystemTypes(s.OSDisk.OSType),
			CreateOption: compute.DiskCreateOptionTypesFromImage,
			DiskSizeGB:   s.OSDisk.DiskSizeGB,
			Caching:      compute.CachingTypes(s.OSDisk.CachingType),
		},
	}

	// Checking if the requested VM size has at least 2 vCPUS
	vCPUCapability, err := s.SKU.HasCapabilityWithCapacity(resourceskus.VCPUs, resourceskus.MinimumVCPUS)
	if err != nil {
		return nil, azure.WithTerminalError(errors.Wrap(err, "failed to validate the vCPU capability"))
	}
	if !vCPUCapability {
		return nil, azure.WithTerminalError(errors.New("vm size should be bigger or equal to at least 2 vCPUs"))
	}

	// Checking if the requested VM size has at least 2 Gi of memory
	MemoryCapability, err := s.SKU.HasCapabilityWithCapacity(resourceskus.MemoryGB, resourceskus.MinimumMemory)
	if err != nil {
		return nil, azure.WithTerminalError(errors.Wrap(err, "failed to validate the memory capability"))
	}

	if !MemoryCapability {
		return nil, azure.WithTerminalError(errors.New("vm memory should be bigger or equal to at least 2Gi"))
	}
	// enable ephemeral OS
	if s.OSDisk.DiffDiskSettings != nil {
		if !s.SKU.HasCapability(resourceskus.EphemeralOSDisk) {
			return nil, azure.WithTerminalError(fmt.Errorf("vm size %s does not support ephemeral os. select a different vm size or disable ephemeral os", s.Size))
		}

		storageProfile.OsDisk.DiffDiskSettings = &compute.DiffDiskSettings{
			Option: compute.DiffDiskOptions(s.OSDisk.DiffDiskSettings.Option),
		}
	}

	if s.OSDisk.ManagedDisk != nil {
		storageProfile.OsDisk.ManagedDisk = &compute.ManagedDiskParameters{}
		if s.OSDisk.ManagedDisk.StorageAccountType != "" {
			storageProfile.OsDisk.ManagedDisk.StorageAccountType = compute.StorageAccountTypes(s.OSDisk.ManagedDisk.StorageAccountType)
		}
		if s.OSDisk.ManagedDisk.DiskEncryptionSet != nil {
			storageProfile.OsDisk.ManagedDisk.DiskEncryptionSet = &compute.DiskEncryptionSetParameters{ID: pointer.String(s.OSDisk.ManagedDisk.DiskEncryptionSet.ID)}
		}
	}

	dataDisks := make([]compute.DataDisk, len(s.DataDisks))
	for i, disk := range s.DataDisks {
		dataDisks[i] = compute.DataDisk{
			CreateOption: compute.DiskCreateOptionTypesEmpty,
			DiskSizeGB:   pointer.Int32(disk.DiskSizeGB),
			Lun:          disk.Lun,
			Name:         pointer.String(azure.GenerateDataDiskName(s.Name, disk.NameSuffix)),
			Caching:      compute.CachingTypes(disk.CachingType),
		}

		if disk.ManagedDisk != nil {
			dataDisks[i].ManagedDisk = &compute.ManagedDiskParameters{
				StorageAccountType: compute.StorageAccountTypes(disk.ManagedDisk.StorageAccountType),
			}

			if disk.ManagedDisk.DiskEncryptionSet != nil {
				dataDisks[i].ManagedDisk.DiskEncryptionSet = &compute.DiskEncryptionSetParameters{ID: pointer.String(disk.ManagedDisk.DiskEncryptionSet.ID)}
			}

			// check the support for ultra disks based on location and vm size
			if disk.ManagedDisk.StorageAccountType == string(compute.StorageAccountTypesUltraSSDLRS) && !s.SKU.HasLocationCapability(resourceskus.UltraSSDAvailable, s.Location, s.Zone) {
				return nil, azure.WithTerminalError(fmt.Errorf("vm size %s does not support ultra disks in location %s. select a different vm size or disable ultra disks", s.Size, s.Location))
			}
		}
	}
	storageProfile.DataDisks = &dataDisks

	imageRef, err := converters.ImageToSDK(s.Image)
	if err != nil {
		return nil, err
	}

	storageProfile.ImageReference = imageRef

	return storageProfile, nil
}

func (s *VMSpec) generateOSProfile() (*compute.OSProfile, error) {
	sshKey, err := base64.StdEncoding.DecodeString(s.SSHKeyData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode ssh public key")
	}

	osProfile := &compute.OSProfile{
		ComputerName:  pointer.String(s.Name),
		AdminUsername: pointer.String(azure.DefaultUserName),
		CustomData:    pointer.String(s.BootstrapData),
	}

	switch s.OSDisk.OSType {
	case string(compute.OperatingSystemTypesWindows):
		// Cloudbase-init is used to generate a password.
		// https://cloudbase-init.readthedocs.io/en/latest/plugins.html#setting-password-main
		//
		// We generate a random password here in case of failure
		// but the password on the VM will NOT be the same as created here.
		// Access is provided via SSH public key that is set during deployment
		// Azure also provides a way to reset user passwords in the case of need.
		osProfile.AdminPassword = pointer.String(generators.SudoRandomPassword(123))
		osProfile.WindowsConfiguration = &compute.WindowsConfiguration{
			EnableAutomaticUpdates: pointer.Bool(false),
		}
	default:
		osProfile.LinuxConfiguration = &compute.LinuxConfiguration{
			DisablePasswordAuthentication: pointer.Bool(true),
			SSH: &compute.SSHConfiguration{
				PublicKeys: &[]compute.SSHPublicKey{
					{
						Path:    pointer.String(fmt.Sprintf("/home/%s/.ssh/authorized_keys", azure.DefaultUserName)),
						KeyData: pointer.String(string(sshKey)),
					},
				},
			},
		}
	}

	return osProfile, nil
}

func (s *VMSpec) generateSecurityProfile() (*compute.SecurityProfile, error) {
	if s.SecurityProfile == nil {
		return nil, nil
	}

	if !s.SKU.HasCapability(resourceskus.EncryptionAtHost) {
		return nil, azure.WithTerminalError(errors.Errorf("encryption at host is not supported for VM type %s", s.Size))
	}

	return &compute.SecurityProfile{
		EncryptionAtHost: s.SecurityProfile.EncryptionAtHost,
	}, nil
}

func (s *VMSpec) generateNICRefs() *[]compute.NetworkInterfaceReference {
	nicRefs := make([]compute.NetworkInterfaceReference, len(s.NICIDs))
	for i, id := range s.NICIDs {
		primary := i == 0
		nicRefs[i] = compute.NetworkInterfaceReference{
			ID: pointer.String(id),
			NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{
				Primary: pointer.Bool(primary),
			},
		}
	}
	return &nicRefs
}

func (s *VMSpec) generateAdditionalCapabilities() *compute.AdditionalCapabilities {
	var capabilities *compute.AdditionalCapabilities

	// Provisionally detect whether there is any Data Disk defined which uses UltraSSDs.
	// If that's the case, enable the UltraSSD capability.
	for _, dataDisk := range s.DataDisks {
		if dataDisk.ManagedDisk != nil && dataDisk.ManagedDisk.StorageAccountType == string(compute.StorageAccountTypesUltraSSDLRS) {
			capabilities = &compute.AdditionalCapabilities{
				UltraSSDEnabled: pointer.Bool(true),
			}
			break
		}
	}

	// Set Additional Capabilities if any is present on the spec.
	if s.AdditionalCapabilities != nil {
		if capabilities == nil {
			capabilities = &compute.AdditionalCapabilities{}
		}
		// Set UltraSSDEnabled if a specific value is set on the spec for it.
		if s.AdditionalCapabilities.UltraSSDEnabled != nil {
			capabilities.UltraSSDEnabled = s.AdditionalCapabilities.UltraSSDEnabled
		}
	}

	return capabilities
}

func (s *VMSpec) getAvailabilitySet() *compute.SubResource {
	var as *compute.SubResource
	if s.AvailabilitySetID != "" {
		as = &compute.SubResource{ID: &s.AvailabilitySetID}
	}
	return as
}

func (s *VMSpec) getZones() *[]string {
	var zones *[]string
	if s.Zone != "" {
		zones = &[]string{s.Zone}
	}
	return zones
}

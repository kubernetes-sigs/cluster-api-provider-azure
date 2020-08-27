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

package scalesets

import (
	"context"
	"encoding/base64"
	"fmt"

	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"

	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-01/compute"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
)

// getExisting provides information about a scale set.
func (s *Service) getExisting(ctx context.Context, name string) (*infrav1exp.VMSS, error) {
	vmss, err := s.Client.Get(ctx, s.Scope.ResourceGroup(), name)
	if err != nil {
		return nil, err
	}

	vmssInstances, err := s.Client.ListInstances(ctx, s.Scope.ResourceGroup(), name)
	if err != nil {
		return nil, err
	}

	return converters.SDKToVMSS(vmss, vmssInstances), nil
}

func (s *Service) Reconcile(ctx context.Context) error {
	vmssSpec := s.Scope.ScaleSetSpec()

	sku, err := s.ResourceSKUCache.Get(ctx, vmssSpec.Size, resourceskus.VirtualMachines)
	if err != nil {
		return errors.Wrapf(err, "failed to get find SKU %s in compute api", vmssSpec.Size)
	}

	// Checking if the requested VM size has at least 2 vCPUS
	vCPUCapability, err := sku.HasCapabilityWithCapacity(resourceskus.VCPUs, resourceskus.MinimumVCPUS)
	if err != nil {
		return errors.Wrap(err, "failed to validate the vCPU cabability")
	}
	if !vCPUCapability {
		return errors.New("vm size should be bigger or equal to at least 2 vCPUs")
	}

	// Checking if the requested VM size has at least 2 Gi of memory
	MemoryCapability, err := sku.HasCapabilityWithCapacity(resourceskus.MemoryGB, resourceskus.MinimumMemory)
	if err != nil {
		return errors.Wrap(err, "failed to validate the memory cabability")
	}
	if !MemoryCapability {
		return errors.New("vm memory should be bigger or equal to at least 2Gi")
	}

	if vmssSpec.AcceleratedNetworking == nil {
		// set accelerated networking to the capability of the VMSize
		accelNet := sku.HasCapability(resourceskus.AcceleratedNetworking)
		vmssSpec.AcceleratedNetworking = &accelNet
	}

	storageProfile, err := s.generateStorageProfile(vmssSpec, sku)
	if err != nil {
		return err
	}

	// Get the node outbound LB backend pool ID
	backendAddressPools := []compute.SubResource{}
	if vmssSpec.PublicLBName != "" {
		if vmssSpec.PublicLBAddressPoolName != "" {
			backendAddressPools = append(backendAddressPools,
				compute.SubResource{
					ID: to.StringPtr(azure.AddressPoolID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), vmssSpec.PublicLBName, vmssSpec.PublicLBAddressPoolName)),
				})
		}
	}

	sshKey, err := base64.StdEncoding.DecodeString(vmssSpec.SSHKeyData)
	if err != nil {
		return errors.Wrapf(err, "failed to decode ssh public key")
	}
	bootstrapData, err := s.Scope.GetBootstrapData(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to retrieve bootstrap data")
	}

	vmss := compute.VirtualMachineScaleSet{
		Location: to.StringPtr(s.Scope.Location()),
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: s.Scope.ClusterName(),
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Name:        to.StringPtr(vmssSpec.Name),
			Role:        to.StringPtr(infrav1.Node),
			Additional:  s.Scope.AdditionalTags(),
		})),
		Sku: &compute.Sku{
			Name:     to.StringPtr(vmssSpec.Size),
			Tier:     to.StringPtr("Standard"),
			Capacity: to.Int64Ptr(vmssSpec.Capacity),
		},
		VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
			UpgradePolicy: &compute.UpgradePolicy{
				Mode: compute.UpgradeModeManual,
			},
			VirtualMachineProfile: &compute.VirtualMachineScaleSetVMProfile{
				OsProfile: &compute.VirtualMachineScaleSetOSProfile{
					ComputerNamePrefix: to.StringPtr(vmssSpec.Name),
					AdminUsername:      to.StringPtr(azure.DefaultUserName),
					CustomData:         to.StringPtr(bootstrapData),
					LinuxConfiguration: &compute.LinuxConfiguration{
						SSH: &compute.SSHConfiguration{
							PublicKeys: &[]compute.SSHPublicKey{
								{
									Path:    to.StringPtr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", azure.DefaultUserName)),
									KeyData: to.StringPtr(string(sshKey)),
								},
							},
						},
						DisablePasswordAuthentication: to.BoolPtr(true),
					},
				},
				StorageProfile: storageProfile,
				DiagnosticsProfile: &compute.DiagnosticsProfile{
					BootDiagnostics: &compute.BootDiagnostics{
						Enabled: to.BoolPtr(true),
					},
				},
				NetworkProfile: &compute.VirtualMachineScaleSetNetworkProfile{
					NetworkInterfaceConfigurations: &[]compute.VirtualMachineScaleSetNetworkConfiguration{
						{
							Name: to.StringPtr(vmssSpec.Name + "-netconfig"),
							VirtualMachineScaleSetNetworkConfigurationProperties: &compute.VirtualMachineScaleSetNetworkConfigurationProperties{
								Primary:            to.BoolPtr(true),
								EnableIPForwarding: to.BoolPtr(true),
								IPConfigurations: &[]compute.VirtualMachineScaleSetIPConfiguration{
									{
										Name: to.StringPtr(vmssSpec.Name + "-ipconfig"),
										VirtualMachineScaleSetIPConfigurationProperties: &compute.VirtualMachineScaleSetIPConfigurationProperties{
											Subnet: &compute.APIEntityReference{
												ID: to.StringPtr(azure.SubnetID(s.Scope.SubscriptionID(), vmssSpec.VNetResourceGroup, vmssSpec.VNetName, vmssSpec.SubnetName)),
											},
											Primary:                         to.BoolPtr(true),
											PrivateIPAddressVersion:         compute.IPv4,
											LoadBalancerBackendAddressPools: &backendAddressPools,
										},
									},
								},
								EnableAcceleratedNetworking: vmssSpec.AcceleratedNetworking,
							},
						},
					},
				},
			},
		},
	}

	if vmssSpec.TerminateNotificationTimeout != nil {
		vmss.VirtualMachineProfile.ScheduledEventsProfile = &compute.ScheduledEventsProfile{
			TerminateNotificationProfile: &compute.TerminateNotificationProfile{
				Enable:           to.BoolPtr(true),
				NotBeforeTimeout: to.StringPtr(fmt.Sprintf("PT%dM", *vmssSpec.TerminateNotificationTimeout)),
			},
		}
		// Once we have scheduled events termination notification we can switch upgrade policy to be rolling
		vmss.VirtualMachineScaleSetProperties.UpgradePolicy = &compute.UpgradePolicy{
			// Prefer rolling upgrade compared to Automatic (which updates all instances at same time)
			Mode: compute.UpgradeModeRolling,
			// We need to set the rolling upgrade policy based on user defined values
			// for now lets stick to defaults, future PR will include the configurability
			// RollingUpgradePolicy: &compute.RollingUpgradePolicy{},
		}
	}

	existingVMSS, err := s.getExisting(ctx, vmssSpec.Name)
	switch {
	case err != nil && !azure.ResourceNotFound(err):
		return errors.Wrapf(err, "failed to get VMSS %s", vmssSpec.Name)
	case err == nil:
		// VMSS already exists
		// update the status.
		s.Scope.SetProviderID(fmt.Sprintf("azure:///%s", existingVMSS.ID))
		s.Scope.SetAnnotation("cluster-api-provider-azure", "true")
		s.Scope.SetProvisioningState(existingVMSS.State)
		// update it
		// we do this to avoid overwriting fields in networkProfile modified by cloud-provider
		update, err := getVMSSUpdateFromVMSS(vmss)
		if err != nil {
			return errors.Wrapf(err, "failed to generate scale set update parameters for %s", vmssSpec.Name)
		}
		update.VirtualMachineProfile.NetworkProfile = nil
		return s.Client.Update(ctx, s.Scope.ResourceGroup(), vmssSpec.Name, update)
	default:
		s.Scope.V(2).Info("creating VMSS", "scale set", vmssSpec.Name)

		err = s.Client.CreateOrUpdate(
			ctx,
			s.Scope.ResourceGroup(),
			vmssSpec.Name,
			vmss)
		if err != nil {
			return errors.Wrapf(err, "cannot create VMSS")
		}
		s.Scope.V(2).Info("successfully created VMSS", "scale set", vmssSpec.Name)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context) error {
	vmssSpec := s.Scope.ScaleSetSpec()
	s.Scope.V(2).Info("deleting VMSS", "scale set", vmssSpec.Name)
	err := s.Client.Delete(ctx, s.Scope.ResourceGroup(), vmssSpec.Name)
	if err != nil {
		if azure.ResourceNotFound(err) {
			// already deleted
			return nil
		}
		return errors.Wrapf(err, "failed to delete VMSS %s in resource group %s", vmssSpec.Name, s.Scope.ResourceGroup())
	}

	s.Scope.V(2).Info("successfully deleted VMSS", "scale set", vmssSpec.Name)
	return nil
}

// generateStorageProfile generates a pointer to a compute.VirtualMachineScaleSetStorageProfile which can utilized for VM creation.
func (s *Service) generateStorageProfile(vmssSpec azure.ScaleSetSpec, sku resourceskus.SKU) (*compute.VirtualMachineScaleSetStorageProfile, error) {
	storageProfile := &compute.VirtualMachineScaleSetStorageProfile{
		OsDisk: &compute.VirtualMachineScaleSetOSDisk{
			OsType:       compute.OperatingSystemTypes(vmssSpec.OSDisk.OSType),
			CreateOption: compute.DiskCreateOptionTypesFromImage,
			DiskSizeGB:   to.Int32Ptr(vmssSpec.OSDisk.DiskSizeGB),
			ManagedDisk: &compute.VirtualMachineScaleSetManagedDiskParameters{
				StorageAccountType: compute.StorageAccountTypes(vmssSpec.OSDisk.ManagedDisk.StorageAccountType),
			},
		},
	}

	// enable ephemeral OS
	if vmssSpec.OSDisk.DiffDiskSettings != nil {
		if !sku.HasCapability(resourceskus.EphemeralOSDisk) {
			return nil, fmt.Errorf("vm size %s does not support ephemeral os. select a different vm size or disable ephemeral os", vmssSpec.Size)
		}

		storageProfile.OsDisk.DiffDiskSettings = &compute.DiffDiskSettings{
			Option: compute.DiffDiskOptions(vmssSpec.OSDisk.DiffDiskSettings.Option),
		}
	}

	dataDisks := []compute.VirtualMachineScaleSetDataDisk{}
	for _, disk := range vmssSpec.DataDisks {
		dataDisks = append(dataDisks, compute.VirtualMachineScaleSetDataDisk{
			CreateOption: compute.DiskCreateOptionTypesEmpty,
			DiskSizeGB:   to.Int32Ptr(disk.DiskSizeGB),
			Lun:          disk.Lun,
			Name:         to.StringPtr(azure.GenerateDataDiskName(vmssSpec.Name, disk.NameSuffix)),
		})
	}
	storageProfile.DataDisks = &dataDisks

	image, err := s.Scope.GetVMImage()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get VM image")
	}

	imageRef, err := converters.ImageToSDK(image)
	if err != nil {
		return nil, err
	}

	storageProfile.ImageReference = imageRef

	return storageProfile, nil
}

func getVMSSUpdateFromVMSS(vmss compute.VirtualMachineScaleSet) (compute.VirtualMachineScaleSetUpdate, error) {
	json, err := vmss.MarshalJSON()
	if err != nil {
		return compute.VirtualMachineScaleSetUpdate{}, err
	}
	var update compute.VirtualMachineScaleSetUpdate
	err = update.UnmarshalJSON(json)
	return update, err
}

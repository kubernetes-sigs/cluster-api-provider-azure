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

package virtualmachines

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-01/compute"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
)

// getExisting provides information about a virtual machine.
func (s *Service) getExisting(ctx context.Context, name string) (*infrav1.VM, error) {
	vm, err := s.Client.Get(ctx, s.Scope.ResourceGroup(), name)
	if err != nil {
		return nil, err
	}

	convertedVM, err := converters.SDKToVM(vm)
	if err != nil {
		return convertedVM, err
	}

	// Discover addresses for NICs associated with the VM
	// and add them to our converted vm struct
	addresses, err := s.getAddresses(ctx, vm)
	if err != nil {
		return convertedVM, err
	}
	convertedVM.Addresses = addresses
	return convertedVM, nil
}

// Reconcile gets/creates/updates a virtual machine.
func (s *Service) Reconcile(ctx context.Context) error {
	for _, vmSpec := range s.Scope.VMSpecs() {
		existingVM, err := s.getExisting(ctx, vmSpec.Name)
		switch {
		case err != nil && !azure.ResourceNotFound(err):
			return errors.Wrapf(err, "failed to get VM %s", vmSpec.Name)
		case err == nil:
			// VM already exists, update the spec and skip creation.
			s.Scope.SetProviderID(fmt.Sprintf("azure:///%s", existingVM.ID))
			s.Scope.SetAnnotation("cluster-api-provider-azure", "true")
			s.Scope.SetAddresses(existingVM.Addresses)
			s.Scope.SetVMState(existingVM.State)
		default:
			s.Scope.V(2).Info("creating VM", "vm", vmSpec.Name)

			storageProfile, err := s.generateStorageProfile(ctx, vmSpec)
			if err != nil {
				return err
			}

			nicRefs := make([]compute.NetworkInterfaceReference, len(vmSpec.NICNames))
			for i, nicName := range vmSpec.NICNames {
				primary := i == 0
				nicRefs[i] = compute.NetworkInterfaceReference{
					ID: to.StringPtr(azure.NetworkInterfaceID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), nicName)),
					NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{
						Primary: to.BoolPtr(primary),
					},
				}
			}

			priority, evictionPolicy, billingProfile, err := getSpotVMOptions(vmSpec.SpotVMOptions)
			if err != nil {
				return errors.Wrapf(err, "failed to get Spot VM options")
			}

			sshKey, err := base64.StdEncoding.DecodeString(vmSpec.SSHKeyData)
			if err != nil {
				return errors.Wrapf(err, "failed to decode ssh public key")
			}
			bootstrapData, err := s.Scope.GetBootstrapData(ctx)
			if err != nil {
				return errors.Wrap(err, "failed to retrieve bootstrap data")
			}

			virtualMachine := compute.VirtualMachine{
				Plan:     s.generateImagePlan(),
				Location: to.StringPtr(s.Scope.Location()),
				Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
					ClusterName: s.Scope.ClusterName(),
					Lifecycle:   infrav1.ResourceLifecycleOwned,
					Name:        to.StringPtr(vmSpec.Name),
					Role:        to.StringPtr(vmSpec.Role),
					Additional:  s.Scope.AdditionalTags(),
				})),
				VirtualMachineProperties: &compute.VirtualMachineProperties{
					HardwareProfile: &compute.HardwareProfile{
						VMSize: compute.VirtualMachineSizeTypes(vmSpec.Size),
					},
					StorageProfile: storageProfile,
					OsProfile: &compute.OSProfile{
						ComputerName:  to.StringPtr(vmSpec.Name),
						AdminUsername: to.StringPtr(azure.DefaultUserName),
						CustomData:    to.StringPtr(bootstrapData),
						LinuxConfiguration: &compute.LinuxConfiguration{
							DisablePasswordAuthentication: to.BoolPtr(true),
							SSH: &compute.SSHConfiguration{
								PublicKeys: &[]compute.SSHPublicKey{
									{
										Path:    to.StringPtr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", azure.DefaultUserName)),
										KeyData: to.StringPtr(string(sshKey)),
									},
								},
							},
						},
					},
					NetworkProfile: &compute.NetworkProfile{
						NetworkInterfaces: &nicRefs,
					},
					Priority:       priority,
					EvictionPolicy: evictionPolicy,
					BillingProfile: billingProfile,
					DiagnosticsProfile: &compute.DiagnosticsProfile{
						BootDiagnostics: &compute.BootDiagnostics{
							Enabled: to.BoolPtr(true),
						},
					},
				},
			}

			if vmSpec.Zone != "" {
				zones := []string{vmSpec.Zone}
				virtualMachine.Zones = &zones
			}

			if vmSpec.Identity == infrav1.VMIdentitySystemAssigned {
				virtualMachine.Identity = &compute.VirtualMachineIdentity{
					Type: compute.ResourceIdentityTypeSystemAssigned,
				}
			} else if vmSpec.Identity == infrav1.VMIdentityUserAssigned {
				if len(vmSpec.UserAssignedIdentities) == 0 {
					return errors.Wrapf(err, "cannot create VM: The user-assigned identity provider ids must not be null or empty for 'UserAssigned' identity type.")
				}
				// UserAssignedIdentities - The list of user identities associated with the Virtual Machine.
				// The user identity dictionary key references will be ARM resource ids in the form:
				// '/subscriptions/{subscriptionId}/resourceGroups/{resourceGroupName}/providers/Microsoft.ManagedIdentity/userAssignedIdentities/{identityName}'.
				userIdentitiesMap := make(map[string]*compute.VirtualMachineIdentityUserAssignedIdentitiesValue, len(vmSpec.UserAssignedIdentities))
				for _, id := range vmSpec.UserAssignedIdentities {
					key := id.ProviderID
					if strings.HasPrefix(id.ProviderID, "azure:///") {
						key = strings.TrimPrefix(key, "azure:///")
					}
					userIdentitiesMap[key] = &compute.VirtualMachineIdentityUserAssignedIdentitiesValue{}
				}
				virtualMachine.Identity = &compute.VirtualMachineIdentity{
					Type:                   compute.ResourceIdentityTypeUserAssigned,
					UserAssignedIdentities: userIdentitiesMap,
				}
			}

			if err := s.Client.CreateOrUpdate(ctx, s.Scope.ResourceGroup(), vmSpec.Name, virtualMachine); err != nil {
				return errors.Wrapf(err, "failed to create VM %s in resource group %s", vmSpec.Name, s.Scope.ResourceGroup())
			}

			s.Scope.V(2).Info("successfully created VM", "vm", vmSpec.Name)
		}
	}

	return nil
}

func (s *Service) generateImagePlan() *compute.Plan {
	image, err := s.Scope.GetVMImage()
	if err != nil {
		return nil
	}
	if image.Marketplace == nil || image.Marketplace.ThirdPartyImage == false {
		return nil
	}
	if image.Marketplace.Publisher == "" || image.Marketplace.SKU == "" || image.Marketplace.Offer == "" {
		return nil
	}

	return &compute.Plan{
		Publisher: to.StringPtr(image.Marketplace.Publisher),
		Name:      to.StringPtr(image.Marketplace.SKU),
		Product:   to.StringPtr(image.Marketplace.Offer),
	}
}

// Delete deletes the virtual machine with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	for _, vmSpec := range s.Scope.VMSpecs() {
		s.Scope.V(2).Info("deleting VM", "vm", vmSpec.Name)
		err := s.Client.Delete(ctx, s.Scope.ResourceGroup(), vmSpec.Name)
		if err != nil && azure.ResourceNotFound(err) {
			// already deleted
			continue
		}
		if err != nil {
			return errors.Wrapf(err, "failed to delete VM %s in resource group %s", vmSpec.Name, s.Scope.ResourceGroup())
		}

		s.Scope.V(2).Info("successfully deleted VM", "vm", vmSpec.Name)
	}
	return nil
}

func (s *Service) getAddresses(ctx context.Context, vm compute.VirtualMachine) ([]corev1.NodeAddress, error) {

	addresses := []corev1.NodeAddress{}

	if vm.NetworkProfile.NetworkInterfaces == nil {
		return addresses, nil
	}
	for _, nicRef := range *vm.NetworkProfile.NetworkInterfaces {

		// The full ID includes the name at the very end. Split the string and pull the last element
		// Ex: /subscriptions/$SUB/resourceGroups/$RG/providers/Microsoft.Network/networkInterfaces/$NICNAME
		// We'll check to see if ID is nil and bail early if we don't have it
		if nicRef.ID == nil {
			continue
		}
		nicName := getResourceNameByID(to.String(nicRef.ID))

		// Fetch nic and append its addresses
		nic, err := s.InterfacesClient.Get(ctx, s.Scope.ResourceGroup(), nicName)
		if err != nil {
			return addresses, err
		}

		if nic.IPConfigurations == nil {
			continue
		}
		for _, ipConfig := range *nic.IPConfigurations {
			if ipConfig.PrivateIPAddress != nil {
				addresses = append(addresses,
					corev1.NodeAddress{
						Type:    corev1.NodeInternalIP,
						Address: to.String(ipConfig.PrivateIPAddress),
					},
				)
			}

			if ipConfig.PublicIPAddress == nil {
				continue
			}
			// ID is the only field populated in PublicIPAddress sub-resource.
			// Thus, we have to go fetch the publicIP with the name.
			publicIPName := getResourceNameByID(to.String(ipConfig.PublicIPAddress.ID))
			publicNodeAddress, err := s.getPublicIPAddress(ctx, publicIPName)
			if err != nil {
				return addresses, err
			}
			addresses = append(addresses, publicNodeAddress)
		}
	}

	return addresses, nil
}

// getPublicIPAddress will fetch a public ip address resource by name and return a nodeaddresss representation
func (s *Service) getPublicIPAddress(ctx context.Context, publicIPAddressName string) (corev1.NodeAddress, error) {
	retAddress := corev1.NodeAddress{}
	publicIP, err := s.PublicIPsClient.Get(ctx, s.Scope.ResourceGroup(), publicIPAddressName)
	if err != nil {
		return retAddress, err
	}
	retAddress.Type = corev1.NodeExternalIP
	retAddress.Address = to.String(publicIP.IPAddress)

	return retAddress, nil
}

// getResourceNameById takes a resource ID like
// `/subscriptions/$SUB/resourceGroups/$RG/providers/Microsoft.Network/networkInterfaces/$NICNAME`
// and parses out the string after the last slash.
func getResourceNameByID(resourceID string) string {
	explodedResourceID := strings.Split(resourceID, "/")
	resourceName := explodedResourceID[len(explodedResourceID)-1]
	return resourceName
}

// generateStorageProfile generates a pointer to a compute.StorageProfile which can utilized for VM creation.
func (s *Service) generateStorageProfile(ctx context.Context, vmSpec azure.VMSpec) (*compute.StorageProfile, error) {
	storageProfile := &compute.StorageProfile{
		OsDisk: &compute.OSDisk{
			Name:         to.StringPtr(azure.GenerateOSDiskName(vmSpec.Name)),
			OsType:       compute.OperatingSystemTypes(vmSpec.OSDisk.OSType),
			CreateOption: compute.DiskCreateOptionTypesFromImage,
			DiskSizeGB:   to.Int32Ptr(vmSpec.OSDisk.DiskSizeGB),
			ManagedDisk: &compute.ManagedDiskParameters{
				StorageAccountType: compute.StorageAccountTypes(vmSpec.OSDisk.ManagedDisk.StorageAccountType),
			},
			Caching: compute.CachingTypes(vmSpec.OSDisk.CachingType),
		},
	}

	sku, err := s.ResourceSKUCache.Get(ctx, vmSpec.Size, resourceskus.VirtualMachines)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get find vm sku %s in compute api", vmSpec.Size)
	}

	// Checking if the requested VM size has at least 2 vCPUS
	vCPUCapability, err := sku.HasCapabilityWithCapacity(resourceskus.VCPUs, resourceskus.MinimumVCPUS)
	if err != nil {
		return nil, errors.Wrap(err, "failed to validate the vCPU cabability")
	}
	if !vCPUCapability {
		return nil, errors.New("vm size should be bigger or equal to at least 2 vCPUs")
	}

	// Checking if the requested VM size has at least 2 Gi of memory
	MemoryCapability, err := sku.HasCapabilityWithCapacity(resourceskus.MemoryGB, resourceskus.MinimumMemory)
	if err != nil {
		return nil, errors.Wrap(err, "failed to validate the memory cabability")
	}
	if !MemoryCapability {
		return nil, errors.New("vm memory should be bigger or equal to at least 2Gi")
	}

	// enable ephemeral OS
	if vmSpec.OSDisk.DiffDiskSettings != nil {
		if !sku.HasCapability(resourceskus.EphemeralOSDisk) {
			return nil, fmt.Errorf("vm size %s does not support ephemeral os. select a different vm size or disable ephemeral os", vmSpec.Size)
		}

		storageProfile.OsDisk.DiffDiskSettings = &compute.DiffDiskSettings{
			Option: compute.DiffDiskOptions(vmSpec.OSDisk.DiffDiskSettings.Option),
		}
	}

	dataDisks := []compute.DataDisk{}
	for _, disk := range vmSpec.DataDisks {
		dataDisks = append(dataDisks, compute.DataDisk{
			CreateOption: compute.DiskCreateOptionTypesEmpty,
			DiskSizeGB:   to.Int32Ptr(disk.DiskSizeGB),
			Lun:          disk.Lun,
			Name:         to.StringPtr(azure.GenerateDataDiskName(vmSpec.Name, disk.NameSuffix)),
			Caching:      compute.CachingTypes(disk.CachingType),
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

func getSpotVMOptions(spotVMOptions *infrav1.SpotVMOptions) (compute.VirtualMachinePriorityTypes, compute.VirtualMachineEvictionPolicyTypes, *compute.BillingProfile, error) {
	// Spot VM not requested, return zero values to apply defaults
	if spotVMOptions == nil {
		return "", "", nil, nil
	}
	var billingProfile *compute.BillingProfile
	if spotVMOptions.MaxPrice != nil {
		maxPrice, err := strconv.ParseFloat(*spotVMOptions.MaxPrice, 64)
		if err != nil {
			return "", "", nil, err
		}
		billingProfile = &compute.BillingProfile{
			MaxPrice: &maxPrice,
		}
	}
	return compute.Spot, compute.Deallocate, billingProfile, nil
}

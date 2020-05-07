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
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-12-01/compute"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
)

// Spec contains properties to create a managed cluster.
// Spec input specification for Get/CreateOrUpdate/Delete calls
type (
	Spec struct {
		Name            string
		ResourceGroup   string
		Location        string
		ClusterName     string
		MachinePoolName string
		Sku             string
		Capacity        int64
		SSHKeyData      string
		Image           *infrav1.Image
		OSDisk          infrav1.OSDisk
		CustomData      string
		SubnetID        string
		AdditionalTags  infrav1.Tags
	}
)

func (s *Service) Get(ctx context.Context, spec interface{}) (interface{}, error) {
	vmssSpec, ok := spec.(*Spec)
	if !ok {
		return compute.VirtualMachineScaleSet{}, errors.New("invalid VMSS specification")
	}

	vmss, err := s.Client.Get(ctx, vmssSpec.ResourceGroup, vmssSpec.Name)
	if err != nil {
		return vmss, err
	}

	vmssInstances, err := s.Client.ListInstances(ctx, vmssSpec.ResourceGroup, vmssSpec.Name)
	if err != nil {
		return vmss, err
	}

	return converters.SDKToVMSS(vmss, vmssInstances), nil
}

func (s *Service) Reconcile(ctx context.Context, spec interface{}) error {
	vmssSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid VMSS specification")
	}

	storageProfile, err := generateStorageProfile(*vmssSpec)
	if err != nil {
		return err
	}

	// Make sure to use the MachineScope here to get the merger of AzureCluster and AzureMachine tags
	// Set the cloud provider tag
	if vmssSpec.AdditionalTags == nil {
		vmssSpec.AdditionalTags = make(infrav1.Tags)
	}
	vmssSpec.AdditionalTags[infrav1.ClusterAzureCloudProviderTagKey(vmssSpec.MachinePoolName)] = string(infrav1.ResourceLifecycleOwned)

	vmss := compute.VirtualMachineScaleSet{
		Location: to.StringPtr(vmssSpec.Location),
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: vmssSpec.ClusterName,
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Name:        to.StringPtr(vmssSpec.MachinePoolName),
			Role:        to.StringPtr(infrav1.Node),
			Additional:  vmssSpec.AdditionalTags,
		})),
		Sku: &compute.Sku{
			Name:     to.StringPtr(vmssSpec.Sku),
			Tier:     to.StringPtr("Standard"),
			Capacity: to.Int64Ptr(vmssSpec.Capacity),
		},
		VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
			UpgradePolicy: &compute.UpgradePolicy{
				Mode: compute.Manual,
			},
			VirtualMachineProfile: &compute.VirtualMachineScaleSetVMProfile{
				OsProfile: &compute.VirtualMachineScaleSetOSProfile{
					ComputerNamePrefix: to.StringPtr(vmssSpec.Name),
					AdminUsername:      to.StringPtr(azure.DefaultUserName),
					CustomData:         to.StringPtr(vmssSpec.CustomData),
					LinuxConfiguration: &compute.LinuxConfiguration{
						SSH: &compute.SSHConfiguration{
							PublicKeys: &[]compute.SSHPublicKey{
								{
									Path:    to.StringPtr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", azure.DefaultUserName)),
									KeyData: to.StringPtr(vmssSpec.SSHKeyData),
								},
							},
						},
						DisablePasswordAuthentication: to.BoolPtr(true),
					},
				},
				StorageProfile: storageProfile,
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
												ID: to.StringPtr(vmssSpec.SubnetID),
											},
											Primary:                 to.BoolPtr(true),
											PrivateIPAddressVersion: compute.IPv4,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	err = s.Client.CreateOrUpdate(
		ctx,
		vmssSpec.ResourceGroup,
		vmssSpec.Name,
		vmss)
	if err != nil {
		return errors.Wrapf(err, "cannot create VMSS")
	}

	klog.V(2).Infof("successfully created VMSS %s ", vmssSpec.Name)
	return nil
}

func (s *Service) Delete(ctx context.Context, spec interface{}) error {
	vmssSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid VMSS specification")
	}
	klog.V(2).Infof("deleting VMSS %s ", vmssSpec.Name)
	err := s.Client.Delete(ctx, vmssSpec.ResourceGroup, vmssSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "failed to delete VMSS %s in resource group %s", vmssSpec.Name, vmssSpec.ResourceGroup)
	}

	klog.V(2).Infof("successfully deleted VMSS %s ", vmssSpec.Name)
	return nil
}

// generateStorageProfile generates a pointer to a compute.VirtualMachineScaleSetStorageProfile which can utilized for VM creation.
func generateStorageProfile(vmssSpec Spec) (*compute.VirtualMachineScaleSetStorageProfile, error) {
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

	imageRef, err := converters.ImageToSDK(vmssSpec.Image)
	if err != nil {
		return nil, err
	}

	storageProfile.ImageReference = imageRef

	return storageProfile, nil
}

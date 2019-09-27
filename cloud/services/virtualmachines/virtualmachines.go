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
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	"k8s.io/klog"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha2"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/networkinterfaces"
)

// Spec input specification for Get/CreateOrUpdate/Delete calls
type Spec struct {
	Name       string
	NICName    string
	SSHKeyData string
	Size       string
	Zone       string
	Image      infrav1.Image
	OSDisk     infrav1.OSDisk
	CustomData string
}

// Get provides information about a virtual machine.
func (s *Service) Get(ctx context.Context, spec interface{}) (interface{}, error) {
	vmSpec, ok := spec.(*Spec)
	if !ok {
		return compute.VirtualMachine{}, errors.New("invalid vm specification")
	}
	vm, err := s.Client.Get(ctx, s.Scope.AzureCluster.Spec.ResourceGroup, vmSpec.Name, "")
	if err != nil && azure.ResourceNotFound(err) {
		return nil, errors.Wrapf(err, "vm %s not found", vmSpec.Name)
	} else if err != nil {
		return vm, err
	}

	return converters.SDKToVM(vm)
}

// Reconcile gets/creates/updates a virtual machine.
func (s *Service) Reconcile(ctx context.Context, spec interface{}) error {
	vmSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid vm specification")
	}

	storageProfile, err := generateStorageProfile(*vmSpec)
	if err != nil {
		return err
	}

	klog.V(2).Infof("getting nic %s", vmSpec.NICName)
	nicInterface, err := networkinterfaces.NewService(s.Scope).Get(ctx, &networkinterfaces.Spec{Name: vmSpec.NICName})
	if err != nil {
		return err
	}
	nic, ok := nicInterface.(network.Interface)
	if !ok {
		return errors.New("error getting network security group")
	}
	klog.V(2).Infof("got nic %s", vmSpec.NICName)

	klog.V(2).Infof("creating vm %s ", vmSpec.Name)

	sshKeyData := vmSpec.SSHKeyData
	if sshKeyData == "" {
		privateKey, perr := rsa.GenerateKey(rand.Reader, 2048)
		if perr != nil {
			return errors.Wrap(perr, "Failed to generate private key")
		}

		publicRsaKey, perr := ssh.NewPublicKey(&privateKey.PublicKey)
		if perr != nil {
			return errors.Wrap(perr, "Failed to generate public key")
		}
		sshKeyData = string(ssh.MarshalAuthorizedKey(publicRsaKey))
	}

	randomPassword, err := GenerateRandomString(32)
	if err != nil {
		return errors.Wrapf(err, "failed to generate random string")
	}

	virtualMachine := compute.VirtualMachine{
		Location: to.StringPtr(s.Scope.Location()),
		VirtualMachineProperties: &compute.VirtualMachineProperties{
			HardwareProfile: &compute.HardwareProfile{
				VMSize: compute.VirtualMachineSizeTypes(vmSpec.Size),
			},
			StorageProfile: storageProfile,
			OsProfile: &compute.OSProfile{
				ComputerName:  to.StringPtr(vmSpec.Name),
				AdminUsername: to.StringPtr(azure.DefaultUserName),
				AdminPassword: to.StringPtr(randomPassword),
				CustomData:    to.StringPtr(vmSpec.CustomData),
				LinuxConfiguration: &compute.LinuxConfiguration{
					SSH: &compute.SSHConfiguration{
						PublicKeys: &[]compute.SSHPublicKey{
							{
								Path:    to.StringPtr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", azure.DefaultUserName)),
								KeyData: to.StringPtr(sshKeyData),
							},
						},
					},
				},
			},
			NetworkProfile: &compute.NetworkProfile{
				NetworkInterfaces: &[]compute.NetworkInterfaceReference{
					{
						ID: nic.ID,
						NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{
							Primary: to.BoolPtr(true),
						},
					},
				},
			},
		},
	}

	klog.V(2).Infof("Setting zone %s ", vmSpec.Zone)

	if vmSpec.Zone != "" {
		zones := []string{vmSpec.Zone}
		virtualMachine.Zones = &zones
	}

	future, err := s.Client.CreateOrUpdate(
		ctx,
		s.Scope.AzureCluster.Spec.ResourceGroup,
		vmSpec.Name,
		virtualMachine)
	if err != nil {
		return errors.Wrapf(err, "cannot create vm")
	}

	err = future.WaitForCompletionRef(ctx, s.Client.Client)
	if err != nil {
		return errors.Wrapf(err, "cannot get the vm create or update future response")
	}

	_, err = future.Result(s.Client)
	if err != nil {
		return err
	}

	klog.V(2).Infof("successfully created vm %s ", vmSpec.Name)
	return err
}

// Delete deletes the virtual machine with the provided name.
func (s *Service) Delete(ctx context.Context, spec interface{}) error {
	vmSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid vm Specification")
	}
	klog.V(2).Infof("deleting vm %s ", vmSpec.Name)
	future, err := s.Client.Delete(ctx, s.Scope.AzureCluster.Spec.ResourceGroup, vmSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "failed to delete vm %s in resource group %s", vmSpec.Name, s.Scope.AzureCluster.Spec.ResourceGroup)
	}

	err = future.WaitForCompletionRef(ctx, s.Client.Client)
	if err != nil {
		return errors.Wrap(err, "cannot delete, future response")
	}

	_, err = future.Result(s.Client)

	klog.V(2).Infof("successfully deleted vm %s ", vmSpec.Name)
	return err
}

// generateStorageProfile generates a pointer to a compute.StorageProfile which can utilized for VM creation.
func generateStorageProfile(vmSpec Spec) (*compute.StorageProfile, error) {
	// TODO: Validate parameters before building storage profile
	storageProfile := &compute.StorageProfile{
		OsDisk: &compute.OSDisk{
			Name:         to.StringPtr(azure.GenerateOSDiskName(vmSpec.Name)),
			OsType:       compute.OperatingSystemTypes(vmSpec.OSDisk.OSType),
			CreateOption: compute.DiskCreateOptionTypesFromImage,
			DiskSizeGB:   to.Int32Ptr(vmSpec.OSDisk.DiskSizeGB),
			ManagedDisk: &compute.ManagedDiskParameters{
				StorageAccountType: compute.StorageAccountTypes(vmSpec.OSDisk.ManagedDisk.StorageAccountType),
			},
		},
	}

	imageRef, err := generateImageReference(vmSpec.Image)
	if err != nil {
		return nil, err
	}

	storageProfile.ImageReference = imageRef

	return storageProfile, nil
}

// generateImageReference generates a pointer to a compute.ImageReference which can utilized for VM creation.
func generateImageReference(image infrav1.Image) (*compute.ImageReference, error) {
	imageRef := &compute.ImageReference{}

	if image.ID != nil {
		imageRef.ID = to.StringPtr(*image.ID)

		// return early since we should only need the image ID
		return imageRef, nil
	} else if image.SubscriptionID != nil && image.ResourceGroup != nil && image.Gallery != nil && image.Name != nil && image.Version != nil {
		imageID, err := generateImageID(image)
		if err != nil {
			return nil, err
		}

		imageRef.ID = to.StringPtr(imageID)

		// return early since we're referencing an image that may not be published
		return imageRef, nil
	}

	if image.Publisher != nil {
		imageRef.Publisher = image.Publisher
	}
	if image.Offer != nil {
		imageRef.Offer = image.Offer
	}
	if image.SKU != nil {
		imageRef.Sku = image.SKU
	}
	if image.Version != nil {
		imageRef.Version = image.Version

		return imageRef, nil
	}

	return nil, errors.Errorf("Image reference cannot be generated, as fields are missing: %+v", *imageRef)
}

// generateImageID generates the resource ID for an image stored in an Azure Shared Image Gallery.
func generateImageID(image infrav1.Image) (string, error) {
	if image.SubscriptionID == nil {
		return "", errors.New("Image subscription ID cannot be nil when specifying an image from an Azure Shared Image Gallery")
	}
	if image.ResourceGroup == nil {
		return "", errors.New("Image resource group cannot be nil when specifying an image from an Azure Shared Image Gallery")
	}
	if image.Gallery == nil {
		return "", errors.New("Image gallery cannot be nil when specifying an image from an Azure Shared Image Gallery")
	}
	if image.Name == nil {
		return "", errors.New("Image name cannot be nil when specifying an image from an Azure Shared Image Gallery")
	}
	if image.Version == nil {
		return "", errors.New("Image version cannot be nil when specifying an image from an Azure Shared Image Gallery")
	}

	return fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/galleries/%s/images/%s/versions/%s", *image.SubscriptionID, *image.ResourceGroup, *image.Gallery, *image.Name, *image.Version), nil
}

// GenerateRandomString returns a URL-safe, base64 encoded
// securely generated random string.
// It will return an error if the system's secure random
// number generator fails to function correctly, in which
// case the caller should not continue.
func GenerateRandomString(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), err
}

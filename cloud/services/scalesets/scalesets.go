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
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2020-06-30/compute"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/util/generators"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// ScaleSetScope defines the scope interface for a scale sets service.
type ScaleSetScope interface {
	logr.Logger
	azure.ClusterDescriber
	ScaleSetSpec() azure.ScaleSetSpec
	GetBootstrapData(ctx context.Context) (string, error)
	GetVMImage() (*infrav1.Image, error)
	SetAnnotation(string, string)
	SetProviderID(string)
	UpdateInstanceStatuses(context.Context, []infrav1exp.VMSSVM) error
	NeedsK8sVersionUpdate() bool
	SaveK8sVersion()
	SetProvisioningState(infrav1.VMState)
	SetLongRunningOperationState(*infrav1.Future)
	GetLongRunningOperationState() *infrav1.Future
}

type vmssBuildResult struct {
	VMSSWithoutHash compute.VirtualMachineScaleSet
	Tags            infrav1.Tags
	Hash            string
}

// Service provides operations on azure resources
type Service struct {
	Scope ScaleSetScope
	Client
	resourceSKUCache *resourceskus.Cache
}

// NewService creates a new service.
func NewService(scope ScaleSetScope, skuCache *resourceskus.Cache) *Service {
	return &Service{
		Client:           NewClient(scope),
		Scope:            scope,
		resourceSKUCache: skuCache,
	}
}

// Reconcile idempotently gets, creates, and updates a scale set.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "scalesets.Service.Reconcile")
	defer span.End()

	if err := s.validateSpec(ctx); err != nil {
		// do as much early validation as possible to limit calls to Azure
		return err
	}

	// check if there is an ongoing long running operation
	future := s.Scope.GetLongRunningOperationState()
	var fetchedVMSS *infrav1exp.VMSS
	var err error
	if future == nil {
		fetchedVMSS, err = s.getVirtualMachineScaleSet(ctx)
	} else {
		fetchedVMSS, err = s.getVirtualMachineScaleSetIfDone(ctx, future)
	}

	switch {
	case err != nil && !azure.ResourceNotFound(err):
		// There was an error and it was not an HTTP 404 not found. This is either a transient error in Azure or a bug.
		return errors.Wrapf(err, "failed to get VMSS %s", s.Scope.ScaleSetSpec().Name)
	case err != nil && azure.ResourceNotFound(err):
		// HTTP(404) resource was not found, so we need to create it with a PUT
		future, err = s.createVMSS(ctx)
		if err != nil {
			return errors.Wrapf(err, "failed to start creating VMSS")
		}
	case err == nil:
		// HTTP(200)
		// VMSS already exists and may have changes; update it with a PATCH
		// we do this to avoid overwriting fields in networkProfile modified by cloud-provider
		future, err = s.patchVMSSIfNeeded(ctx, fetchedVMSS)
		if err != nil {
			return errors.Wrapf(err, "failed to start updating VMSS")
		}
	default:
		// just in case, set the provider ID if the instance exists
		s.Scope.SetProviderID(fmt.Sprintf("azure://%s", fetchedVMSS.ID))
	}

	// Try to get the VMSS to update status if we have created a long running operation. If the VMSS is still in a long
	// running operation, getVirtualMachineScaleSetIfDone will return an azure.WithTransientError and requeue.
	if future != nil {
		fetchedVMSS, err = s.getVirtualMachineScaleSetIfDone(ctx, future)
		if err != nil {
			return errors.Wrapf(err, "failed to get VMSS %s after create or update", s.Scope.ScaleSetSpec().Name)
		}
	}

	// if we get to hear, we have completed any long running VMSS operations (creates / updates)
	s.Scope.SetLongRunningOperationState(nil)

	defer func() {
		// make sure we always set the provisioning state at the end of reconcile
		s.Scope.SetProvisioningState(fetchedVMSS.State)
	}()

	if err := s.reconcileInstances(ctx, fetchedVMSS); err != nil {
		return errors.Wrap(err, "failed to reconcile instances")
	}

	return nil
}

// Delete deletes a scale set asynchronously. Delete sends a DELETE request to Azure and if accepted without error,
// the VMSS will be considered deleted. The actual delete in Azure may take longer, but should eventually complete.
func (s *Service) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "scalesets.Service.Delete")
	defer span.End()

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

func (s *Service) createVMSS(ctx context.Context) (*infrav1.Future, error) {
	ctx, span := tele.Tracer().Start(ctx, "scalesets.Service.createVMSS")
	defer span.End()

	spec := s.Scope.ScaleSetSpec()
	result, err := s.buildVMSSFromSpec(ctx, spec)
	if err != nil {
		return nil, errors.Wrap(err, "failed building VMSS from spec")
	}

	vmss := result.VMSSWithoutHash
	vmss.Tags = converters.TagsToMap(result.Tags.AddSpecVersionHashTag(result.Hash))
	s.Scope.SetProvisioningState(infrav1.VMStateCreating)
	future, err := s.Client.CreateOrUpdateAsync(ctx, s.Scope.ResourceGroup(), spec.Name, vmss)
	if err != nil {
		return future, errors.Wrapf(err, "cannot create VMSS")
	}

	s.Scope.V(2).Info("starting to create VMSS", "scale set", spec.Name)
	s.Scope.SetLongRunningOperationState(future)
	s.Scope.SaveK8sVersion()
	return future, err
}

func (s *Service) patchVMSSIfNeeded(ctx context.Context, infraVMSS *infrav1exp.VMSS) (*infrav1.Future, error) {
	ctx, span := tele.Tracer().Start(ctx, "scalesets.Service.patchVMSSIfNeeded")
	defer span.End()

	s.Scope.SetProviderID(fmt.Sprintf("azure://%s", infraVMSS.ID))

	spec := s.Scope.ScaleSetSpec()
	result, err := s.buildVMSSFromSpec(ctx, spec)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate scale set update parameters for %s", spec.Name)
	}

	if infraVMSS.Tags.HasMatchingSpecVersionHash(result.Hash) {
		// The VMSS built from the AzureMachinePool spec matches the hash in the tag of the existing VMSS. This means
		// the VMSS does not need to be patched since it has not changed.
		//
		// hash(AzureMachinePool.Spec)
		//
		// Note: if a user were to mutate the VMSS in Azure rather than through CAPZ, this hash match may match, but not
		// reflect the state of the specification in K8s.
		return nil, nil
	}

	vmss := result.VMSSWithoutHash
	vmss.Tags = converters.TagsToMap(result.Tags.AddSpecVersionHashTag(result.Hash))
	patch, err := getVMSSUpdateFromVMSS(vmss)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate vmss patch for %s", spec.Name)
	}

	// wipe out network profile, so updates won't conflict with Cloud Provider updates
	patch.VirtualMachineProfile.NetworkProfile = nil
	future, err := s.UpdateAsync(ctx, s.Scope.ResourceGroup(), spec.Name, patch)
	if err != nil {
		if azure.ResourceConflict(err) {
			return future, azure.WithTransientError(err, 30*time.Second)
		}
		return future, errors.Wrapf(err, "failed updating VMSS")
	}

	s.Scope.SetProvisioningState(infrav1.VMStateUpdating)
	s.Scope.SetLongRunningOperationState(future)
	s.Scope.V(2).Info("successfully started to update vmss", "scale set", spec.Name)
	return future, err
}

func (s *Service) reconcileInstances(ctx context.Context, vmss *infrav1exp.VMSS) error {
	ctx, span := tele.Tracer().Start(ctx, "scalesets.Service.reconcileInstances")
	defer span.End()

	// check to see if we are running the most K8s version specified in the MachinePool spec
	// if not, then update the instances that are not running that model
	if s.Scope.NeedsK8sVersionUpdate() {
		instanceIDs := make([]string, len(vmss.Instances))
		for i, vm := range vmss.Instances {
			instanceIDs[i] = vm.InstanceID
		}

		if err := s.Client.UpdateInstances(ctx, s.Scope.ResourceGroup(), vmss.Name, instanceIDs); err != nil {
			return errors.Wrapf(err, "failed to update VMSS %s instances", vmss.Name)
		}

		s.Scope.SaveK8sVersion()
		// get the VMSS to update status
		var err error
		vmss, err = s.getVirtualMachineScaleSet(ctx)
		if err != nil {
			return errors.Wrap(err, "failed to get VMSS after updating an instance")
		}
	}

	// update the status.
	if err := s.Scope.UpdateInstanceStatuses(ctx, vmss.Instances); err != nil {
		return errors.Wrap(err, "unable to update instance status")
	}

	return nil
}

func (s *Service) validateSpec(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "scalesets.Service.validateSpec")
	defer span.End()

	spec := s.Scope.ScaleSetSpec()
	sku, err := s.resourceSKUCache.Get(ctx, spec.Size, resourceskus.VirtualMachines)
	if err != nil {
		return azure.WithTerminalError(errors.Wrapf(err, "failed to get find SKU %s in compute api", spec.Size))
	}

	// Checking if the requested VM size has at least 2 vCPUS
	vCPUCapability, err := sku.HasCapabilityWithCapacity(resourceskus.VCPUs, resourceskus.MinimumVCPUS)
	if err != nil {
		return azure.WithTerminalError(errors.Wrap(err, "failed to validate the vCPU cabability"))
	}

	if !vCPUCapability {
		return azure.WithTerminalError(errors.New("vm size should be bigger or equal to at least 2 vCPUs"))
	}

	// Checking if the requested VM size has at least 2 Gi of memory
	MemoryCapability, err := sku.HasCapabilityWithCapacity(resourceskus.MemoryGB, resourceskus.MinimumMemory)
	if err != nil {
		return azure.WithTerminalError(errors.Wrap(err, "failed to validate the memory cabability"))
	}

	if !MemoryCapability {
		return azure.WithTerminalError(errors.New("vm memory should be bigger or equal to at least 2Gi"))
	}

	// enable ephemeral OS
	if spec.OSDisk.DiffDiskSettings != nil && !sku.HasCapability(resourceskus.EphemeralOSDisk) {
		return azure.WithTerminalError(fmt.Errorf("vm size %s does not support ephemeral os. select a different vm size or disable ephemeral os", spec.Size))
	}

	if spec.SecurityProfile != nil && !sku.HasCapability(resourceskus.EncryptionAtHost) {
		return azure.WithTerminalError(errors.Errorf("encryption at host is not supported for VM type %s", spec.Size))
	}

	return nil
}

func (s *Service) buildVMSSFromSpec(ctx context.Context, vmssSpec azure.ScaleSetSpec) (vmssBuildResult, error) {
	ctx, span := tele.Tracer().Start(ctx, "scalesets.Service.buildVMSSFromSpec")
	defer span.End()

	var result vmssBuildResult

	sku, err := s.resourceSKUCache.Get(ctx, vmssSpec.Size, resourceskus.VirtualMachines)
	if err != nil {
		return result, errors.Wrapf(err, "failed to get find SKU %s in compute api", vmssSpec.Size)
	}

	if vmssSpec.AcceleratedNetworking == nil {
		// set accelerated networking to the capability of the VMSize
		accelNet := sku.HasCapability(resourceskus.AcceleratedNetworking)
		vmssSpec.AcceleratedNetworking = &accelNet
	}

	storageProfile, err := s.generateStorageProfile(vmssSpec, sku)
	if err != nil {
		return result, err
	}

	securityProfile, err := getSecurityProfile(vmssSpec, sku)
	if err != nil {
		return result, err
	}

	priority, evictionPolicy, billingProfile, err := converters.GetSpotVMOptions(vmssSpec.SpotVMOptions)
	if err != nil {
		return result, errors.Wrapf(err, "failed to get Spot VM options")
	}

	// Get the node outbound LB backend pool ID
	var backendAddressPools []compute.SubResource
	if vmssSpec.PublicLBName != "" {
		if vmssSpec.PublicLBAddressPoolName != "" {
			backendAddressPools = append(backendAddressPools,
				compute.SubResource{
					ID: to.StringPtr(azure.AddressPoolID(s.Scope.SubscriptionID(), s.Scope.ResourceGroup(), vmssSpec.PublicLBName, vmssSpec.PublicLBAddressPoolName)),
				})
		}
	}

	osProfile, err := s.generateOSProfile(ctx, vmssSpec)
	if err != nil {
		return result, err
	}

	vmss := compute.VirtualMachineScaleSet{
		Location: to.StringPtr(s.Scope.Location()),
		Sku: &compute.Sku{
			Name:     to.StringPtr(vmssSpec.Size),
			Tier:     to.StringPtr("Standard"),
			Capacity: to.Int64Ptr(vmssSpec.Capacity),
		},
		VirtualMachineScaleSetProperties: &compute.VirtualMachineScaleSetProperties{
			UpgradePolicy: &compute.UpgradePolicy{
				Mode: compute.UpgradeModeManual,
			},
			DoNotRunExtensionsOnOverprovisionedVMs: to.BoolPtr(true),
			VirtualMachineProfile: &compute.VirtualMachineScaleSetVMProfile{
				OsProfile:       osProfile,
				StorageProfile:  storageProfile,
				SecurityProfile: securityProfile,
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
				Priority:       priority,
				EvictionPolicy: evictionPolicy,
				BillingProfile: billingProfile,
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

	// Assign Identity to VMSS
	if vmssSpec.Identity == infrav1.VMIdentitySystemAssigned {
		vmss.Identity = &compute.VirtualMachineScaleSetIdentity{
			Type: compute.ResourceIdentityTypeSystemAssigned,
		}
	} else if vmssSpec.Identity == infrav1.VMIdentityUserAssigned {
		userIdentitiesMap, err := converters.UserAssignedIdentitiesToVMSSSDK(vmssSpec.UserAssignedIdentities)
		if err != nil {
			return result, errors.Wrapf(err, "failed to assign identity %q", vmssSpec.Name)
		}
		vmss.Identity = &compute.VirtualMachineScaleSetIdentity{
			Type:                   compute.ResourceIdentityTypeUserAssigned,
			UserAssignedIdentities: userIdentitiesMap,
		}
	}

	tagsWithoutHash := infrav1.Build(infrav1.BuildParams{
		ClusterName: s.Scope.ClusterName(),
		Lifecycle:   infrav1.ResourceLifecycleOwned,
		Name:        to.StringPtr(vmssSpec.Name),
		Role:        to.StringPtr(infrav1.Node),
		Additional:  s.Scope.AdditionalTags(),
	})

	vmss.Tags = converters.TagsToMap(tagsWithoutHash)
	hash, err := base64EncodedHash(vmss)
	if err != nil {
		return result, errors.Wrap(err, "failed to generate hash in vmss create")
	}

	return vmssBuildResult{
		VMSSWithoutHash: vmss,
		Tags:            tagsWithoutHash,
		Hash:            hash,
	}, nil
}

// getVirtualMachineScaleSet provides information about a Virtual Machine Scale Set and its instances
func (s *Service) getVirtualMachineScaleSet(ctx context.Context) (*infrav1exp.VMSS, error) {
	ctx, span := tele.Tracer().Start(ctx, "scalesets.Service.getVirtualMachineScaleSet")
	defer span.End()

	name := s.Scope.ScaleSetSpec().Name
	vmss, err := s.Client.Get(ctx, s.Scope.ResourceGroup(), name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get existing vmss")
	}

	vmssInstances, err := s.Client.ListInstances(ctx, s.Scope.ResourceGroup(), name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list instances")
	}

	return converters.SDKToVMSS(vmss, vmssInstances), nil
}

// getVirtualMachineScaleSetIfDone gets a Virtual Machine Scale Set and its instances from Azure if the future is completed
func (s *Service) getVirtualMachineScaleSetIfDone(ctx context.Context, future *infrav1.Future) (*infrav1exp.VMSS, error) {
	ctx, span := tele.Tracer().Start(ctx, "scalesets.Service.getVirtualMachineScaleSetIfDone")
	defer span.End()

	vmss, err := s.GetResultIfDone(ctx, future)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get result from future")
	}

	vmssInstances, err := s.Client.ListInstances(ctx, future.ResourceGroup, future.Name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list instances")
	}

	return converters.SDKToVMSS(vmss, vmssInstances), nil
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

	if vmssSpec.OSDisk.ManagedDisk.DiskEncryptionSet != nil {
		storageProfile.OsDisk.ManagedDisk.DiskEncryptionSet = &compute.DiskEncryptionSetParameters{ID: to.StringPtr(vmssSpec.OSDisk.ManagedDisk.DiskEncryptionSet.ID)}
	}

	dataDisks := make([]compute.VirtualMachineScaleSetDataDisk, len(vmssSpec.DataDisks))
	for i, disk := range vmssSpec.DataDisks {
		dataDisks[i] = compute.VirtualMachineScaleSetDataDisk{
			CreateOption: compute.DiskCreateOptionTypesEmpty,
			DiskSizeGB:   to.Int32Ptr(disk.DiskSizeGB),
			Lun:          disk.Lun,
			Name:         to.StringPtr(azure.GenerateDataDiskName(vmssSpec.Name, disk.NameSuffix)),
		}
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

func (s *Service) generateOSProfile(ctx context.Context, vmssSpec azure.ScaleSetSpec) (*compute.VirtualMachineScaleSetOSProfile, error) {
	sshKey, err := base64.StdEncoding.DecodeString(vmssSpec.SSHKeyData)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode ssh public key")
	}
	bootstrapData, err := s.Scope.GetBootstrapData(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve bootstrap data")
	}

	osProfile := &compute.VirtualMachineScaleSetOSProfile{
		ComputerNamePrefix: to.StringPtr(vmssSpec.Name),
		AdminUsername:      to.StringPtr(azure.DefaultUserName),
		CustomData:         to.StringPtr(bootstrapData),
	}

	switch vmssSpec.OSDisk.OSType {
	case string(compute.Windows):
		// Cloudbase-init is used to generate a password.
		// https://cloudbase-init.readthedocs.io/en/latest/plugins.html#setting-password-main
		//
		// We generate a random password here in case of failure
		// but the password on the VM will NOT be the same as created here.
		// Access is provided via SSH public key that is set during deployment
		// Azure also provides a way to reset user passwords in the case of need.
		osProfile.AdminPassword = to.StringPtr(generators.SudoRandomPassword(123))
		osProfile.WindowsConfiguration = &compute.WindowsConfiguration{
			EnableAutomaticUpdates: to.BoolPtr(false),
		}
	default:
		osProfile.LinuxConfiguration = &compute.LinuxConfiguration{
			DisablePasswordAuthentication: to.BoolPtr(true),
			SSH: &compute.SSHConfiguration{
				PublicKeys: &[]compute.SSHPublicKey{
					{
						Path:    to.StringPtr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", azure.DefaultUserName)),
						KeyData: to.StringPtr(string(sshKey)),
					},
				},
			},
		}
	}

	return osProfile, nil
}

func getVMSSUpdateFromVMSS(vmss compute.VirtualMachineScaleSet) (compute.VirtualMachineScaleSetUpdate, error) {
	jsonData, err := vmss.MarshalJSON()
	if err != nil {
		return compute.VirtualMachineScaleSetUpdate{}, err
	}
	var update compute.VirtualMachineScaleSetUpdate
	err = update.UnmarshalJSON(jsonData)
	return update, err
}

func getSecurityProfile(vmssSpec azure.ScaleSetSpec, sku resourceskus.SKU) (*compute.SecurityProfile, error) {
	if vmssSpec.SecurityProfile == nil {
		return nil, nil
	}

	if !sku.HasCapability(resourceskus.EncryptionAtHost) {
		return nil, azure.WithTerminalError(errors.Errorf("encryption at host is not supported for VM type %s", vmssSpec.Size))
	}

	return &compute.SecurityProfile{
		EncryptionAtHost: to.BoolPtr(*vmssSpec.SecurityProfile.EncryptionAtHost),
	}, nil
}

// base64EncodedHash transforms a VMSS into json and then creates a sha256 hash of the data encoded as a base64 encoded string
func base64EncodedHash(vmss compute.VirtualMachineScaleSet) (string, error) {
	jsonData, err := vmss.MarshalJSON()
	if err != nil {
		return "", errors.Wrapf(err, "failed marshaling vmss")
	}

	hasher := sha256.New()
	_, _ = hasher.Write(jsonData)
	return base64.URLEncoding.EncodeToString(hasher.Sum(nil)), nil
}

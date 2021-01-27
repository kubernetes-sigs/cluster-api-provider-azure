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

package scope

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// MachineScopeParams defines the input parameters used to create a new MachineScope.
type MachineScopeParams struct {
	Client       client.Client
	Logger       logr.Logger
	ClusterScope azure.ClusterScoper
	Machine      *clusterv1.Machine
	AzureMachine *infrav1.AzureMachine
}

// NewMachineScope creates a new MachineScope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewMachineScope(params MachineScopeParams) (*MachineScope, error) {
	if params.Client == nil {
		return nil, errors.New("client is required when creating a MachineScope")
	}
	if params.Machine == nil {
		return nil, errors.New("machine is required when creating a MachineScope")
	}
	if params.AzureMachine == nil {
		return nil, errors.New("azure machine is required when creating a MachineScope")
	}
	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	helper, err := patch.NewHelper(params.AzureMachine, params.Client)
	if err != nil {
		return nil, errors.Errorf("failed to init patch helper: %v ", err)
	}
	return &MachineScope{
		client:        params.Client,
		Machine:       params.Machine,
		AzureMachine:  params.AzureMachine,
		Logger:        params.Logger,
		patchHelper:   helper,
		ClusterScoper: params.ClusterScope,
	}, nil
}

// MachineScope defines a scope defined around a machine and its cluster.
type MachineScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	azure.ClusterScoper
	Machine      *clusterv1.Machine
	AzureMachine *infrav1.AzureMachine
}

// VMSpec returns the VM spec.
func (m *MachineScope) VMSpec() azure.VMSpec {
	return azure.VMSpec{
		Name:                   m.Name(),
		Role:                   m.Role(),
		NICNames:               m.NICNames(),
		SSHKeyData:             m.AzureMachine.Spec.SSHPublicKey,
		Size:                   m.AzureMachine.Spec.VMSize,
		OSDisk:                 m.AzureMachine.Spec.OSDisk,
		DataDisks:              m.AzureMachine.Spec.DataDisks,
		Zone:                   m.AvailabilityZone(),
		Identity:               m.AzureMachine.Spec.Identity,
		UserAssignedIdentities: m.AzureMachine.Spec.UserAssignedIdentities,
		SpotVMOptions:          m.AzureMachine.Spec.SpotVMOptions,
		SecurityProfile:        m.AzureMachine.Spec.SecurityProfile,
	}
}

// TagsSpecs returns the tags for the AzureMachine.
func (m *MachineScope) TagsSpecs() []azure.TagsSpec {
	return []azure.TagsSpec{
		{
			Scope:      azure.VMID(m.SubscriptionID(), m.ResourceGroup(), m.Name()),
			Tags:       m.AdditionalTags(),
			Annotation: infrav1.VMTagsLastAppliedAnnotation,
		},
	}
}

// PublicIPSpecs returns the public IP specs.
func (m *MachineScope) PublicIPSpecs() []azure.PublicIPSpec {
	var spec []azure.PublicIPSpec
	if m.AzureMachine.Spec.AllocatePublicIP {
		spec = append(spec, azure.PublicIPSpec{
			Name: azure.GenerateNodePublicIPName(m.Name()),
		})
	}
	return spec
}

// InboundNatSpecs returns the inbound NAT specs.
func (m *MachineScope) InboundNatSpecs() []azure.InboundNatSpec {
	if m.Role() == infrav1.ControlPlane {
		return []azure.InboundNatSpec{
			{
				Name:             m.Name(),
				LoadBalancerName: m.APIServerLBName(),
			},
		}
	}
	return []azure.InboundNatSpec{}
}

// NICSpecs returns the network interface specs.
func (m *MachineScope) NICSpecs() []azure.NICSpec {
	spec := azure.NICSpec{
		Name:                    azure.GenerateNICName(m.Name()),
		MachineName:             m.Name(),
		VNetName:                m.Vnet().Name,
		VNetResourceGroup:       m.Vnet().ResourceGroup,
		SubnetName:              m.Subnet().Name,
		VMSize:                  m.AzureMachine.Spec.VMSize,
		AcceleratedNetworking:   m.AzureMachine.Spec.AcceleratedNetworking,
		IPv6Enabled:             m.IsIPv6Enabled(),
		EnableIPForwarding:      m.AzureMachine.Spec.EnableIPForwarding,
		PublicLBName:            m.OutboundLBName(m.Role()),
		PublicLBAddressPoolName: m.OutboundPoolName(m.OutboundLBName(m.Role())),
	}
	if m.Role() == infrav1.ControlPlane {
		if m.IsAPIServerPrivate() {
			spec.InternalLBName = m.APIServerLBName()
			spec.InternalLBAddressPoolName = m.APIServerLBPoolName(m.APIServerLBName())
		} else {
			spec.PublicLBNATRuleName = m.Name()
			spec.PublicLBAddressPoolName = m.APIServerLBPoolName(m.APIServerLBName())
		}
	}
	specs := []azure.NICSpec{spec}
	if m.AzureMachine.Spec.AllocatePublicIP {
		specs = append(specs, azure.NICSpec{
			Name:                  azure.GeneratePublicNICName(m.Name()),
			MachineName:           m.Name(),
			VNetName:              m.Vnet().Name,
			VNetResourceGroup:     m.Vnet().ResourceGroup,
			SubnetName:            m.Subnet().Name,
			PublicIPName:          azure.GenerateNodePublicIPName(m.Name()),
			VMSize:                m.AzureMachine.Spec.VMSize,
			AcceleratedNetworking: m.AzureMachine.Spec.AcceleratedNetworking,
		})
	}

	return specs
}

// NICNames returns the NIC names
func (m *MachineScope) NICNames() []string {
	nicNames := make([]string, len(m.NICSpecs()))
	for i, nic := range m.NICSpecs() {
		nicNames[i] = nic.Name
	}
	return nicNames
}

// DiskSpecs returns the disk specs.
func (m *MachineScope) DiskSpecs() []azure.DiskSpec {
	disks := make([]azure.DiskSpec, 1+len(m.AzureMachine.Spec.DataDisks))
	disks[0] = azure.DiskSpec{
		Name: azure.GenerateOSDiskName(m.Name()),
	}

	for i, dd := range m.AzureMachine.Spec.DataDisks {
		disks[i+1] = azure.DiskSpec{Name: azure.GenerateDataDiskName(m.Name(), dd.NameSuffix)}
	}
	return disks
}

// BastionSpecs returns the bastion specs.
func (m *MachineScope) BastionSpecs() []azure.BastionSpec {
	spec := azure.BastionSpec{
		Name:         azure.GenerateOSDiskName(m.Name()),
		SubnetName:   m.Subnet().Name,
		PublicIPName: azure.GenerateNodePublicIPName(azure.GenerateNICName(m.Name())),
		VNetName:     m.Vnet().Name,
	}
	return []azure.BastionSpec{spec}
}

// RoleAssignmentSpecs returns the role assignment specs.
func (m *MachineScope) RoleAssignmentSpecs() []azure.RoleAssignmentSpec {
	if m.AzureMachine.Spec.Identity == infrav1.VMIdentitySystemAssigned {
		return []azure.RoleAssignmentSpec{
			{
				MachineName:  m.Name(),
				Name:         m.AzureMachine.Spec.RoleAssignmentName,
				ResourceType: azure.VirtualMachine,
			},
		}
	}
	return []azure.RoleAssignmentSpec{}
}

// VMExtensionSpecs returns the vm extension specs.
func (m *MachineScope) VMExtensionSpecs() []azure.VMExtensionSpec {
	name, publisher, version := azure.GetBootstrappingVMExtension(m.AzureMachine.Spec.OSDisk.OSType, m.CloudEnvironment())
	if name != "" {
		return []azure.VMExtensionSpec{
			{
				Name:      name,
				VMName:    m.Name(),
				Publisher: publisher,
				Version:   version,
			},
		}
	}
	return []azure.VMExtensionSpec{}
}

// Subnet returns the machine's subnet based on its role
func (m *MachineScope) Subnet() *infrav1.SubnetSpec {
	if m.IsControlPlane() {
		return m.ControlPlaneSubnet()
	}
	return m.NodeSubnet()
}

// AvailabilityZone returns the AzureMachine Availability Zone.
// Priority for selecting the AZ is
//   1) Machine.Spec.FailureDomain
//   2) AzureMachine.Spec.FailureDomain (This is to support deprecated AZ)
//   3) AzureMachine.Spec.AvailabilityZone.ID (This is DEPRECATED)
//   4) No AZ
func (m *MachineScope) AvailabilityZone() string {
	if m.Machine.Spec.FailureDomain != nil {
		return *m.Machine.Spec.FailureDomain
	}
	// DEPRECATED: to support old clients
	if m.AzureMachine.Spec.FailureDomain != nil {
		return *m.AzureMachine.Spec.FailureDomain
	}
	if m.AzureMachine.Spec.AvailabilityZone.ID != nil {
		return *m.AzureMachine.Spec.AvailabilityZone.ID
	}

	return ""
}

// Name returns the AzureMachine name.
func (m *MachineScope) Name() string {
	// Windows Machine names cannot be longer than 15 chars
	if m.AzureMachine.Spec.OSDisk.OSType == azure.WindowsOS && len(m.AzureMachine.Name) > 15 {
		clustername := m.ClusterName()
		if len(m.ClusterName()) > 9 {
			clustername = strings.TrimSuffix(clustername[0:9], "-")
		}

		return clustername + "-" + m.AzureMachine.Name[len(m.AzureMachine.Name)-5:]
	}
	return m.AzureMachine.Name
}

// Namespace returns the namespace name.
func (m *MachineScope) Namespace() string {
	return m.AzureMachine.Namespace
}

// IsControlPlane returns true if the machine is a control plane.
func (m *MachineScope) IsControlPlane() bool {
	return util.IsControlPlaneMachine(m.Machine)
}

// Role returns the machine role from the labels.
func (m *MachineScope) Role() string {
	if util.IsControlPlaneMachine(m.Machine) {
		return infrav1.ControlPlane
	}
	return infrav1.Node
}

// GetVMID returns the AzureMachine instance id by parsing Spec.ProviderID.
func (m *MachineScope) GetVMID() string {
	parsed, err := noderefutil.NewProviderID(m.ProviderID())
	if err != nil {
		return ""
	}
	return parsed.ID()
}

// ProviderID returns the AzureMachine providerID from the spec.
func (m *MachineScope) ProviderID() string {
	parsed, err := noderefutil.NewProviderID(to.String(m.AzureMachine.Spec.ProviderID))
	if err != nil {
		return ""
	}
	return parsed.ID()
}

// AvailabilitySet returns the availability set for this machine if available
func (m *MachineScope) AvailabilitySet() (string, bool) {
	if !m.AvailabilitySetEnabled() {
		return "", false
	}

	if m.IsControlPlane() {
		return azure.GenerateAvailabilitySetName(m.ClusterName(), azure.ControlPlaneNodeGroup), true
	}

	// get machine deployment name from labels for machines that maybe part of a machine deployment.
	if mdName, ok := m.Machine.Labels[clusterv1.MachineDeploymentLabelName]; ok {
		return azure.GenerateAvailabilitySetName(m.ClusterName(), mdName), true
	}

	return "", false
}

// SetProviderID sets the AzureMachine providerID in spec.
func (m *MachineScope) SetProviderID(v string) {
	m.AzureMachine.Spec.ProviderID = to.StringPtr(v)
}

// VMState returns the AzureMachine VM state.
func (m *MachineScope) VMState() infrav1.VMState {
	if m.AzureMachine.Status.VMState != nil {
		return *m.AzureMachine.Status.VMState
	}
	return ""
}

// SetVMState sets the AzureMachine VM state.
func (m *MachineScope) SetVMState(v infrav1.VMState) {
	m.AzureMachine.Status.VMState = &v
}

// SetReady sets the AzureMachine Ready Status to true.
func (m *MachineScope) SetReady() {
	m.AzureMachine.Status.Ready = true
}

// SetNotReady sets the AzureMachine Ready Status to false.
func (m *MachineScope) SetNotReady() {
	m.AzureMachine.Status.Ready = false
}

// SetFailureMessage sets the AzureMachine status failure message.
func (m *MachineScope) SetFailureMessage(v error) {
	m.AzureMachine.Status.FailureMessage = to.StringPtr(v.Error())
}

// SetFailureReason sets the AzureMachine status failure reason.
func (m *MachineScope) SetFailureReason(v capierrors.MachineStatusError) {
	m.AzureMachine.Status.FailureReason = &v
}

// SetAnnotation sets a key value annotation on the AzureMachine.
func (m *MachineScope) SetAnnotation(key, value string) {
	if m.AzureMachine.Annotations == nil {
		m.AzureMachine.Annotations = map[string]string{}
	}
	m.AzureMachine.Annotations[key] = value
}

// AnnotationJSON returns a map[string]interface from a JSON annotation.
func (m *MachineScope) AnnotationJSON(annotation string) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	jsonAnnotation := m.AzureMachine.GetAnnotations()[annotation]
	if len(jsonAnnotation) == 0 {
		return out, nil
	}
	err := json.Unmarshal([]byte(jsonAnnotation), &out)
	if err != nil {
		return out, err
	}
	return out, nil
}

// UpdateAnnotationJSON updates the `annotation` with
// `content`. `content` in this case should be a `map[string]interface{}`
// suitable for turning into JSON. This `content` map will be marshalled into a
// JSON string before being set as the given `annotation`.
func (m *MachineScope) UpdateAnnotationJSON(annotation string, content map[string]interface{}) error {
	b, err := json.Marshal(content)
	if err != nil {
		return err
	}
	m.SetAnnotation(annotation, string(b))
	return nil
}

// SetAddresses sets the Azure address status.
func (m *MachineScope) SetAddresses(addrs []corev1.NodeAddress) {
	m.AzureMachine.Status.Addresses = addrs
}

// PatchObject persists the machine spec and status.
func (m *MachineScope) PatchObject(ctx context.Context) error {
	conditions.SetSummary(m.AzureMachine,
		conditions.WithConditions(
			infrav1.VMRunningCondition,
		),
		conditions.WithStepCounterIfOnly(
			infrav1.VMRunningCondition,
		),
	)

	return m.patchHelper.Patch(
		ctx,
		m.AzureMachine,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			infrav1.VMRunningCondition,
		}})
}

// Close the MachineScope by updating the machine spec, machine status.
func (m *MachineScope) Close(ctx context.Context) error {
	return m.PatchObject(ctx)
}

// AdditionalTags merges AdditionalTags from the scope's AzureCluster and AzureMachine. If the same key is present in both,
// the value from AzureMachine takes precedence.
func (m *MachineScope) AdditionalTags() infrav1.Tags {
	tags := make(infrav1.Tags)
	// Start with the cluster-wide tags...
	tags.Merge(m.ClusterScoper.AdditionalTags())
	// ... and merge in the Machine's
	tags.Merge(m.AzureMachine.Spec.AdditionalTags)
	// Set the cloud provider tag
	tags[infrav1.ClusterAzureCloudProviderTagKey(m.ClusterName())] = string(infrav1.ResourceLifecycleOwned)

	return tags
}

// GetBootstrapData returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func (m *MachineScope) GetBootstrapData(ctx context.Context) (string, error) {
	if m.Machine.Spec.Bootstrap.DataSecretName == nil {
		return "", errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")
	}
	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: m.Namespace(), Name: *m.Machine.Spec.Bootstrap.DataSecretName}
	if err := m.client.Get(ctx, key, secret); err != nil {
		return "", errors.Wrapf(err, "failed to retrieve bootstrap data secret for AzureMachine %s/%s", m.Namespace(), m.Name())
	}

	value, ok := secret.Data["value"]
	if !ok {
		return "", errors.New("error retrieving bootstrap data: secret value key is missing")
	}
	return base64.StdEncoding.EncodeToString(value), nil
}

// GetVMImage returns the image from the machine configuration, or a default one.
func (m *MachineScope) GetVMImage() (*infrav1.Image, error) {
	// Use custom Marketplace image, Image ID or a Shared Image Gallery image if provided
	if m.AzureMachine.Spec.Image != nil {
		return m.AzureMachine.Spec.Image, nil
	}

	if m.AzureMachine.Spec.OSDisk.OSType == azure.WindowsOS {
		m.Info("No image specified for machine, using default Windows Image", "machine", m.AzureMachine.GetName())
		return azure.GetDefaultWindowsImage(to.String(m.Machine.Spec.Version))
	}

	m.Info("No image specified for machine, using default Linux Image", "machine", m.AzureMachine.GetName())
	return azure.GetDefaultUbuntuImage(to.String(m.Machine.Spec.Version))
}

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
	"github.com/Azure/go-autorest/autorest"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/klogr"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// MachineScopeParams defines the input parameters used to create a new MachineScope.
type MachineScopeParams struct {
	AzureClients
	Client       client.Client
	Logger       logr.Logger
	ClusterScope *ClusterScope
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
		return nil, errors.Wrap(err, "failed to init patch helper")
	}
	return &MachineScope{
		client:       params.Client,
		Machine:      params.Machine,
		AzureMachine: params.AzureMachine,
		Logger:       params.Logger,
		patchHelper:  helper,
		ClusterScope: params.ClusterScope,
	}, nil
}

// MachineScope defines a scope defined around a machine and its cluster.
type MachineScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	ClusterScope azure.ClusterDescriber
	Machine      *clusterv1.Machine
	AzureMachine *infrav1.AzureMachine
}

// PublicIPSpec returns the public IP specs.
func (m *MachineScope) PublicIPSpecs() []azure.PublicIPSpec {
	var spec []azure.PublicIPSpec
	if m.AzureMachine.Spec.AllocatePublicIP == true {
		nicName := azure.GenerateNICName(m.Name())
		spec = append(spec, azure.PublicIPSpec{
			Name: azure.GenerateNodePublicIPName(nicName),
		})
	}
	return spec
}

// NICSpecs returns the network interface specs.
func (m *MachineScope) NICSpecs() []azure.NICSpec {
	spec := azure.NICSpec{
		Name:                  azure.GenerateNICName(m.Name()),
		MachineName:           m.Name(),
		MachineRole:           m.Role(),
		VNetName:              m.ClusterScope.Vnet().Name,
		VNetResourceGroup:     m.ClusterScope.Vnet().ResourceGroup,
		SubnetName:            m.Subnet().Name,
		VMSize:                m.AzureMachine.Spec.VMSize,
		AcceleratedNetworking: m.AzureMachine.Spec.AcceleratedNetworking,
	}
	if m.Role() == infrav1.ControlPlane {
		spec.PublicLoadBalancerName = azure.GeneratePublicLBName(m.ClusterName())
		spec.InternalLoadBalancerName = azure.GenerateInternalLBName(m.ClusterName())
	} else if m.Role() == infrav1.Node {
		spec.PublicLoadBalancerName = m.ClusterName()
	}
	if m.AzureMachine.Spec.AllocatePublicIP == true {
		spec.PublicIPName = azure.GenerateNodePublicIPName(azure.GenerateNICName(m.Name()))
	}

	return []azure.NICSpec{spec}
}

// Location returns the AzureCluster location.
func (m *MachineScope) Location() string {
	return m.ClusterScope.Location()
}

// ResourceGroup returns the AzureCluster resource group.
func (m *MachineScope) ResourceGroup() string {
	return m.ClusterScope.ResourceGroup()
}

// ClusterName returns the AzureCluster name.
func (m *MachineScope) ClusterName() string {
	return m.ClusterScope.ClusterName()
}

// SubscriptionID returns the Azure client Subscription ID.
func (m *MachineScope) SubscriptionID() string {
	return m.ClusterScope.SubscriptionID()
}

// BaseURI returns the Azure ResourceManagerEndpoint.
func (m *MachineScope) BaseURI() string {
	return m.ClusterScope.BaseURI()
}

// Authorizer returns the Azure client Authorizer.
func (m *MachineScope) Authorizer() autorest.Authorizer {
	return m.ClusterScope.Authorizer()
}

// Vnet returns the cluster VNet.
func (m *MachineScope) Vnet() *infrav1.VnetSpec {
	return m.ClusterScope.Vnet()
}

// NodeSubnet returns the cluster node subnet.
func (m *MachineScope) NodeSubnet() *infrav1.SubnetSpec {
	return m.ClusterScope.NodeSubnet()
}

// ControlPlaneSubnet returns the cluster control plane subnet.
func (m *MachineScope) ControlPlaneSubnet() *infrav1.SubnetSpec {
	return m.ClusterScope.ControlPlaneSubnet()
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
//   2) AzureMachine.Spec.FailureDomain
//   3) AzureMachine.Spec.AvailabilityZone.ID (This is DEPRECATED)
//   4) No AZ
func (m *MachineScope) AvailabilityZone() string {
	if m.Machine.Spec.FailureDomain != nil {
		return *m.Machine.Spec.FailureDomain
	}
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
func (m *MachineScope) GetVMID() *string {
	parsed, err := noderefutil.NewProviderID(m.GetProviderID())
	if err != nil {
		return nil
	}
	return pointer.StringPtr(parsed.ID())
}

// GetProviderID returns the AzureMachine providerID from the spec.
func (m *MachineScope) GetProviderID() string {
	if m.AzureMachine.Spec.ProviderID != nil {
		return *m.AzureMachine.Spec.ProviderID
	}
	return ""
}

// SetProviderID sets the AzureMachine providerID in spec.
func (m *MachineScope) SetProviderID(v string) {
	m.AzureMachine.Spec.ProviderID = pointer.StringPtr(v)
}

// GetVMState returns the AzureMachine VM state.
func (m *MachineScope) GetVMState() *infrav1.VMState {
	return m.AzureMachine.Status.VMState
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
	m.AzureMachine.Status.FailureMessage = pointer.StringPtr(v.Error())
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

// SetAddresses sets the Azure address status.
func (m *MachineScope) SetAddresses(addrs []corev1.NodeAddress) {
	m.AzureMachine.Status.Addresses = addrs
}

// PatchObject persists the machine spec and status.
func (m *MachineScope) PatchObject(ctx context.Context) error {
	return m.patchHelper.Patch(ctx, m.AzureMachine)
}

// Close the MachineScope by updating the machine spec, machine status.
func (m *MachineScope) Close(ctx context.Context) error {
	return m.patchHelper.Patch(ctx, m.AzureMachine)
}

// AdditionalTags merges AdditionalTags from the scope's AzureCluster and AzureMachine. If the same key is present in both,
// the value from AzureMachine takes precedence.
func (m *MachineScope) AdditionalTags() infrav1.Tags {
	tags := make(infrav1.Tags)

	// Start with the cluster-wide tags...
	tags.Merge(m.ClusterScope.AdditionalTags())
	// ... and merge in the Machine's
	tags.Merge(m.AzureMachine.Spec.AdditionalTags)

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

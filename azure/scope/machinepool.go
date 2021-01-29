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

package scope

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/klogr"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
	capierrors "sigs.k8s.io/cluster-api/errors"
	capiv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

type (
	// MachinePoolScopeParams defines the input parameters used to create a new MachinePoolScope.
	MachinePoolScopeParams struct {
		Client           client.Client
		Logger           logr.Logger
		MachinePool      *capiv1exp.MachinePool
		AzureMachinePool *infrav1exp.AzureMachinePool
		ClusterScope     azure.ClusterScoper
	}

	// MachinePoolScope defines a scope defined around a machine pool and its cluster.
	MachinePoolScope struct {
		azure.ClusterScoper
		logr.Logger
		AzureMachinePool *infrav1exp.AzureMachinePool
		MachinePool      *capiv1exp.MachinePool
		client           client.Client
		patchHelper      *patch.Helper
		vmssState        *infrav1exp.VMSS
	}

	// NodeStatus represents the status of a Kubernetes node
	NodeStatus struct {
		Ready   bool
		Version string
	}
)

// NewMachinePoolScope creates a new MachinePoolScope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewMachinePoolScope(params MachinePoolScopeParams) (*MachinePoolScope, error) {
	if params.Client == nil {
		return nil, errors.New("client is required when creating a MachinePoolScope")
	}
	if params.MachinePool == nil {
		return nil, errors.New("machine pool is required when creating a MachinePoolScope")
	}
	if params.AzureMachinePool == nil {
		return nil, errors.New("azure machine pool is required when creating a MachinePoolScope")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	helper, err := patch.NewHelper(params.AzureMachinePool, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}
	return &MachinePoolScope{
		client:           params.Client,
		MachinePool:      params.MachinePool,
		AzureMachinePool: params.AzureMachinePool,
		Logger:           params.Logger,
		patchHelper:      helper,
		ClusterScoper:    params.ClusterScope,
	}, nil
}

// ScaleSetSpec returns the scale set spec.
func (m *MachinePoolScope) ScaleSetSpec() azure.ScaleSetSpec {
	return azure.ScaleSetSpec{
		Name:                    m.Name(),
		Size:                    m.AzureMachinePool.Spec.Template.VMSize,
		Capacity:                int64(to.Int32(m.MachinePool.Spec.Replicas)),
		SSHKeyData:              m.AzureMachinePool.Spec.Template.SSHPublicKey,
		OSDisk:                  m.AzureMachinePool.Spec.Template.OSDisk,
		DataDisks:               m.AzureMachinePool.Spec.Template.DataDisks,
		SubnetName:              m.NodeSubnet().Name,
		VNetName:                m.Vnet().Name,
		VNetResourceGroup:       m.Vnet().ResourceGroup,
		PublicLBName:            m.OutboundLBName(infrav1.Node),
		PublicLBAddressPoolName: azure.GenerateOutboundBackendAddressPoolName(m.OutboundLBName(infrav1.Node)),
		AcceleratedNetworking:   m.AzureMachinePool.Spec.Template.AcceleratedNetworking,
		Identity:                m.AzureMachinePool.Spec.Identity,
		UserAssignedIdentities:  m.AzureMachinePool.Spec.UserAssignedIdentities,
		SecurityProfile:         m.AzureMachinePool.Spec.Template.SecurityProfile,
		SpotVMOptions:           m.AzureMachinePool.Spec.Template.SpotVMOptions,
	}
}

// Name returns the Azure Machine Pool Name.
func (m *MachinePoolScope) Name() string {
	// Windows Machine pools names cannot be longer than 9 chars
	if m.AzureMachinePool.Spec.Template.OSDisk.OSType == azure.WindowsOS && len(m.AzureMachinePool.Name) > 9 {
		return "win-" + m.AzureMachinePool.Name[len(m.AzureMachinePool.Name)-5:]
	}
	return m.AzureMachinePool.Name
}

// ProviderID returns the AzureMachinePool ID by parsing Spec.ProviderID.
func (m *MachinePoolScope) ProviderID() string {
	parsed, err := noderefutil.NewProviderID(m.AzureMachinePool.Spec.ProviderID)
	if err != nil {
		return ""
	}
	return parsed.ID()
}

// SetProviderID sets the AzureMachinePool providerID in spec.
func (m *MachinePoolScope) SetProviderID(v string) {
	m.AzureMachinePool.Spec.ProviderID = v
}

// ProvisioningState returns the AzureMachinePool provisioning state.
func (m *MachinePoolScope) ProvisioningState() infrav1.VMState {
	if m.AzureMachinePool.Status.ProvisioningState != nil {
		return *m.AzureMachinePool.Status.ProvisioningState
	}
	return ""
}

// NeedsK8sVersionUpdate compares the MachinePool spec and the AzureMachinePool status to determine if the
// VMSS model needs to be updated
func (m *MachinePoolScope) NeedsK8sVersionUpdate() bool {
	return m.AzureMachinePool.Status.Version != *m.MachinePool.Spec.Template.Spec.Version
}

// SetVMSSState updates the machine pool scope with the current state of the VMSS
func (m *MachinePoolScope) SetVMSSState(vmssState *infrav1exp.VMSS) {
	m.vmssState = vmssState
}

// updateReplicasAndProviderIDs ties the Azure VMSS instance data and the Node status data together to build and update
// the AzureMachinePool replica count and providerIDList.
func (m *MachinePoolScope) updateReplicasAndProviderIDs(ctx context.Context, instances []infrav1exp.VMSSVM) error {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolScope.UpdateInstanceStatuses")
	defer span.End()

	machines, err := m.getMachinePoolMachines(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get machine pool machines")
	}

	var readyReplicas int32
	providerIDs := make([]string, len(machines))
	for i, machine := range machines {
		if machine.Status.Ready {
			readyReplicas++
		}
		providerIDs[i] = machine.Spec.ProviderID
	}

	m.AzureMachinePool.Status.Replicas = readyReplicas
	m.AzureMachinePool.Spec.ProviderIDList = providerIDs
	return nil
}

func (m *MachinePoolScope) getMachinePoolMachines(ctx context.Context) ([]infrav1exp.AzureMachinePoolMachine, error) {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolScope.getMachinePoolMachines")
	defer span.End()

	labels := map[string]string{
		clusterv1.ClusterLabelName:      m.ClusterName(),
		infrav1exp.MachinePoolNameLabel: m.AzureMachinePool.Name,
	}
	ampml := &infrav1exp.AzureMachinePoolMachineList{}
	if err := m.client.List(ctx, ampml, client.InNamespace(m.AzureMachinePool.Namespace), client.MatchingLabels(labels)); err != nil {
		return nil, errors.Wrap(err, "failed to list AzureMachinePoolMachines")
	}

	return ampml.Items, nil
}

func (m *MachinePoolScope) applyAzureMachinePoolMachines(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolScope.Close")
	defer span.End()

	if m.vmssState == nil {
		return nil
	}

	labels := map[string]string{
		clusterv1.ClusterLabelName:      m.ClusterName(),
		infrav1exp.MachinePoolNameLabel: m.AzureMachinePool.Name,
	}
	ampml := &infrav1exp.AzureMachinePoolMachineList{}
	if err := m.client.List(ctx, ampml, client.InNamespace(m.AzureMachinePool.Namespace), client.MatchingLabels(labels)); err != nil {
		return errors.Wrap(err, "failed to list AzureMachinePoolMachines")
	}

	existingMachinesByProviderID := make(map[string]infrav1exp.AzureMachinePoolMachine, len(ampml.Items))
	for _, machine := range ampml.Items {
		existingMachinesByProviderID[machine.Spec.ProviderID] = machine
	}

	// determine which machines need to be created to reflect the current state in Azure
	latestMachinesByProviderID := m.vmssState.InstancesByProviderID()
	for key, val := range latestMachinesByProviderID {
		if _, ok := existingMachinesByProviderID[key]; !ok {
			if err := m.createMachine(ctx, val); err != nil {
				return errors.Wrap(err, "failed creating machine")
			}
		}
	}

	// determine which machines need to be deleted since they are not in Azure
	for key, val := range existingMachinesByProviderID {
		val := val
		if _, ok := latestMachinesByProviderID[key]; !ok {
			if err := m.client.Delete(ctx, &val); err != nil && !apierrors.IsNotFound(err) {
				return errors.Wrap(err, "failed deleting machine")
			}
		}
	}

	return nil
}

func (m *MachinePoolScope) createMachine(ctx context.Context, machine infrav1exp.VMSSVM) error {
	if machine.InstanceID == "" {
		return errors.New("machine.InstanceID must not be empty")
	}

	if machine.Name == "" {
		return errors.New("machine.Name must not be empty")
	}

	ampm := infrav1exp.AzureMachinePoolMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      strings.Join([]string{m.AzureMachinePool.Name, machine.InstanceID}, "-"),
			Namespace: m.AzureMachinePool.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         infrav1exp.GroupVersion.String(),
					Kind:               "AzureMachinePool",
					Name:               m.AzureMachinePool.Name,
					BlockOwnerDeletion: to.BoolPtr(true),
					UID:                m.AzureMachinePool.UID,
				},
			},
			Labels: map[string]string{
				m.ClusterName():                 string(infrav1.ResourceLifecycleOwned),
				clusterv1.ClusterLabelName:      m.ClusterName(),
				infrav1exp.MachinePoolNameLabel: m.AzureMachinePool.Name,
			},
		},
		Spec: infrav1exp.AzureMachinePoolMachineSpec{
			ProviderID: machine.ProviderID(),
		},
		Status: infrav1exp.AzureMachinePoolMachineStatus{
			InstanceID:         machine.InstanceID,
			InstanceName:       machine.Name,
			LatestModelApplied: machine.LatestModelApplied,
			ProvisioningState:  &machine.State,
		},
	}

	controllerutil.AddFinalizer(&ampm, infrav1exp.AzureMachinePoolMachineFinalizer)
	conditions.MarkFalse(&ampm, infrav1.VMRunningCondition, infrav1.VMNCreatingReason, clusterv1.ConditionSeverityInfo, "")
	if err := m.client.Create(ctx, &ampm); err != nil {
		return errors.Wrapf(err, "failed creating AzureMachinePoolMachine %s in AzureMachinePool %s", machine.ID, m.AzureMachinePool.Name)
	}

	if err := m.client.Status().Update(ctx, &ampm); err != nil {
		return errors.Wrapf(err, "failed updating status of AzureMachinePoolMachine %s in AzureMachinePool %s", machine.ID, m.AzureMachinePool.Name)
	}

	return nil
}

// SaveK8sVersion stores the MachinePool spec K8s version to the AzureMachinePool status
func (m *MachinePoolScope) SaveK8sVersion() {
	m.AzureMachinePool.Status.Version = *m.MachinePool.Spec.Template.Spec.Version
}

// SetLongRunningOperationState will set the future on the AzureMachinePool status to allow the resource to continue
// in the next reconciliation.
func (m *MachinePoolScope) SetLongRunningOperationState(future *infrav1.Future) {
	m.AzureMachinePool.Status.LongRunningOperationState = future
}

// GetLongRunningOperationState will get the future on the AzureMachinePool status to allow the resource to continue
// in the next reconciliation.
func (m *MachinePoolScope) GetLongRunningOperationState() *infrav1.Future {
	return m.AzureMachinePool.Status.LongRunningOperationState
}

// MaxUnavailable is the max number of unavailable machine pool machines
func (m *MachinePoolScope) MaxUnavailable() int32 {
	return m.AzureMachinePool.Spec.MaxUnavailable
}

// MaxSurge is the max replica count the machine pool can grow to during an upgrade
func (m *MachinePoolScope) MaxSurge() int32 {
	if m.AzureMachinePool.Spec.MaxSurge != nil {
		return *m.AzureMachinePool.Spec.MaxSurge
	}

	return 0
}

// setProvisioningStateAndConditions sets the AzureMachinePool provisioning state and conditions.
func (m *MachinePoolScope) setProvisioningStateAndConditions(v infrav1.VMState) {
	switch {
	case v == infrav1.VMStateSucceeded && *m.MachinePool.Spec.Replicas == m.AzureMachinePool.Status.Replicas:
		// vmss is provisioned with enough ready replicas
		m.AzureMachinePool.Status.ProvisioningState = &v
		conditions.MarkTrue(m.AzureMachinePool, infrav1.PoolRunningCondition)
		conditions.MarkTrue(m.AzureMachinePool, infrav1.PoolModelUpdatedCondition)
		conditions.MarkTrue(m.AzureMachinePool, infrav1.PoolDesiredReplicasCondition)
	case v == infrav1.VMStateSucceeded && *m.MachinePool.Spec.Replicas != m.AzureMachinePool.Status.Replicas:
		// not enough ready or too many ready replicas we must still be scaling up or down
		updatingState := infrav1.VMStateUpdating
		m.AzureMachinePool.Status.ProvisioningState = &updatingState
		if *m.MachinePool.Spec.Replicas > m.AzureMachinePool.Status.Replicas {
			conditions.MarkFalse(m.AzureMachinePool, infrav1.PoolDesiredReplicasCondition, infrav1.PoolScaleUpReason, clusterv1.ConditionSeverityInfo, "")
		} else {
			conditions.MarkFalse(m.AzureMachinePool, infrav1.PoolDesiredReplicasCondition, infrav1.PoolScaleDownReason, clusterv1.ConditionSeverityInfo, "")
		}
	case v == infrav1.VMStateUpdating:
		conditions.MarkFalse(m.AzureMachinePool, infrav1.PoolModelUpdatedCondition, infrav1.PoolModelOutOfDateReason, clusterv1.ConditionSeverityInfo, "")
	case v == infrav1.VMStateCreating:
		conditions.MarkFalse(m.AzureMachinePool, infrav1.PoolRunningCondition, infrav1.PoolCreatingReason, clusterv1.ConditionSeverityInfo, "")
	case v == infrav1.VMStateDeleting:
		conditions.MarkFalse(m.AzureMachinePool, infrav1.PoolRunningCondition, infrav1.PoolDeletingReason, clusterv1.ConditionSeverityInfo, "")
	default:
		m.AzureMachinePool.Status.ProvisioningState = &v
		conditions.MarkFalse(m.AzureMachinePool, infrav1.PoolRunningCondition, string(v), clusterv1.ConditionSeverityInfo, "")
	}
}

// SetReady sets the AzureMachinePool Ready Status to true.
func (m *MachinePoolScope) SetReady() {
	m.AzureMachinePool.Status.Ready = true
}

// SetNotReady sets the AzureMachinePool Ready Status to false.
func (m *MachinePoolScope) SetNotReady() {
	m.AzureMachinePool.Status.Ready = false
}

// SetFailureMessage sets the AzureMachinePool status failure message.
func (m *MachinePoolScope) SetFailureMessage(v error) {
	m.AzureMachinePool.Status.FailureMessage = pointer.StringPtr(v.Error())
}

// SetFailureReason sets the AzureMachinePool status failure reason.
func (m *MachinePoolScope) SetFailureReason(v capierrors.MachineStatusError) {
	m.AzureMachinePool.Status.FailureReason = &v
}

// AdditionalTags merges AdditionalTags from the scope's AzureCluster and AzureMachinePool. If the same key is present in both,
// the value from AzureMachinePool takes precedence.
func (m *MachinePoolScope) AdditionalTags() infrav1.Tags {
	tags := make(infrav1.Tags)
	// Start with the cluster-wide tags...
	tags.Merge(m.ClusterScoper.AdditionalTags())
	// ... and merge in the Machine Pool's
	tags.Merge(m.AzureMachinePool.Spec.AdditionalTags)
	// Set the cloud provider tag
	tags[infrav1.ClusterAzureCloudProviderTagKey(m.ClusterName())] = string(infrav1.ResourceLifecycleOwned)

	return tags
}

// SetAnnotation sets a key value annotation on the AzureMachinePool.
func (m *MachinePoolScope) SetAnnotation(key, value string) {
	if m.AzureMachinePool.Annotations == nil {
		m.AzureMachinePool.Annotations = map[string]string{}
	}
	m.AzureMachinePool.Annotations[key] = value
}

// PatchObject persists the machine spec and status.
func (m *MachinePoolScope) PatchObject(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolScope.PatchObject")
	defer span.End()

	return m.patchHelper.Patch(ctx, m.AzureMachinePool)
}

// AzureMachineTemplate gets the Azure machine template in this scope.
func (m *MachinePoolScope) AzureMachineTemplate(ctx context.Context) (*infrav1.AzureMachineTemplate, error) {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolScope.AzureMachineTemplate")
	defer span.End()

	ref := m.MachinePool.Spec.Template.Spec.InfrastructureRef
	return getAzureMachineTemplate(ctx, m.client, ref.Name, ref.Namespace)
}

// Close the MachineScope by updating the machine spec, machine status.
func (m *MachinePoolScope) Close(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolScope.Close")
	defer span.End()

	if m.vmssState != nil {
		if err := m.applyAzureMachinePoolMachines(ctx); err != nil {
			return errors.Wrap(err, "failed to apply changes to AzureMachinePoolMachines")
		}

		m.setProvisioningStateAndConditions(m.vmssState.State)
		if err := m.updateReplicasAndProviderIDs(ctx, m.vmssState.Instances); err != nil {
			return errors.Wrap(err, "failed to update replicas and providerIDs")
		}
	}

	return m.patchHelper.Patch(ctx, m.AzureMachinePool)
}

// GetBootstrapData returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func (m *MachinePoolScope) GetBootstrapData(ctx context.Context) (string, error) {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolScope.GetBootstrapData")
	defer span.End()

	dataSecretName := m.MachinePool.Spec.Template.Spec.Bootstrap.DataSecretName
	if dataSecretName == nil {
		return "", errors.New("error retrieving bootstrap data: linked Machine Spec's bootstrap.dataSecretName is nil")
	}
	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: m.AzureMachinePool.Namespace, Name: *dataSecretName}
	if err := m.client.Get(ctx, key, secret); err != nil {
		return "", errors.Wrapf(err, "failed to retrieve bootstrap data secret for AzureMachinePool %s/%s", m.AzureMachinePool.Namespace, m.Name())
	}

	value, ok := secret.Data["value"]
	if !ok {
		return "", errors.New("error retrieving bootstrap data: secret value key is missing")
	}
	return base64.StdEncoding.EncodeToString(value), nil
}

// GetVMImage picks an image from the machine configuration, or uses a default one.
func (m *MachinePoolScope) GetVMImage() (*infrav1.Image, error) {
	// Use custom Marketplace image, Image ID or a Shared Image Gallery image if provided
	if m.AzureMachinePool.Spec.Template.Image != nil {
		return m.AzureMachinePool.Spec.Template.Image, nil
	}

	if m.AzureMachinePool.Spec.Template.OSDisk.OSType == azure.WindowsOS {
		m.Info("No image specified for machine, using default Windows Image", "machine", m.MachinePool.GetName())
		return azure.GetDefaultWindowsImage(to.String(m.MachinePool.Spec.Template.Spec.Version))
	}

	m.Info("No image specified for machine, using default", "machine", m.MachinePool.GetName())
	return azure.GetDefaultUbuntuImage(to.String(m.MachinePool.Spec.Template.Spec.Version))
}

// RoleAssignmentSpecs returns the role assignment specs.
func (m *MachinePoolScope) RoleAssignmentSpecs() []azure.RoleAssignmentSpec {
	if m.AzureMachinePool.Spec.Identity == infrav1.VMIdentitySystemAssigned {
		return []azure.RoleAssignmentSpec{
			{
				MachineName:  m.Name(),
				Name:         m.AzureMachinePool.Spec.RoleAssignmentName,
				ResourceType: azure.VirtualMachineScaleSet,
			},
		}
	}
	return []azure.RoleAssignmentSpec{}
}

// VMSSExtensionSpecs returns the vmss extension specs.
func (m *MachinePoolScope) VMSSExtensionSpecs() []azure.VMSSExtensionSpec {
	name, publisher, version := azure.GetBootstrappingVMExtension(m.AzureMachinePool.Spec.Template.OSDisk.OSType, m.CloudEnvironment())
	if name != "" {
		return []azure.VMSSExtensionSpec{
			{
				Name:         name,
				ScaleSetName: m.Name(),
				Publisher:    publisher,
				Version:      version,
			},
		}
	}
	return []azure.VMSSExtensionSpec{}
}

func getAzureMachineTemplate(ctx context.Context, c client.Client, name, namespace string) (*infrav1.AzureMachineTemplate, error) {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolScope.getAzureMachineTemplate")
	defer span.End()

	m := &infrav1.AzureMachineTemplate{}
	key := client.ObjectKey{Name: name, Namespace: namespace}
	if err := c.Get(ctx, key, m); err != nil {
		return nil, err
	}
	return m, nil
}

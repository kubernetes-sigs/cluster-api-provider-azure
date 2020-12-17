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
	"fmt"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/klogr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
	capierrors "sigs.k8s.io/cluster-api/errors"
	capiv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	utilkubeconfig "sigs.k8s.io/cluster-api/util/kubeconfig"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
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
		logr.Logger
		client           client.Client
		patchHelper      *patch.Helper
		MachinePool      *capiv1exp.MachinePool
		AzureMachinePool *infrav1exp.AzureMachinePool
		azure.ClusterScoper
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

// UpdateInstanceStatuses ties the Azure VMSS instance data and the Node status data together to build and update
// the AzureMachinePool. This calculates the number of ready replicas, the current version the kubelet
// is running on the node, the provider IDs for the instances and the providerIDList for the AzureMachinePool spec.
func (m *MachinePoolScope) UpdateInstanceStatuses(ctx context.Context, instances []infrav1exp.VMSSVM) error {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolScope.UpdateInstanceStatuses")
	defer span.End()

	providerIDs := make([]string, len(instances))
	for i, instance := range instances {
		providerIDs[i] = fmt.Sprintf("azure://%s", instance.ID)
	}

	nodeStatusByProviderID, err := m.getNodeStatusByProviderID(ctx, providerIDs)
	if err != nil {
		return errors.Wrap(err, "failed to get node status by provider id")
	}

	var readyReplicas int32
	instanceStatuses := make([]*infrav1exp.AzureMachinePoolInstanceStatus, len(instances))
	for i, instance := range instances {
		instanceStatuses[i] = &infrav1exp.AzureMachinePoolInstanceStatus{
			ProviderID:        fmt.Sprintf("azure://%s", instance.ID),
			InstanceID:        instance.InstanceID,
			InstanceName:      instance.Name,
			ProvisioningState: &instance.State,
		}

		instanceStatus := instanceStatuses[i]
		if nodeStatus, ok := nodeStatusByProviderID[instanceStatus.ProviderID]; ok {
			instanceStatus.Version = nodeStatus.Version
			if m.MachinePool.Spec.Template.Spec.Version != nil {
				instanceStatus.LatestModelApplied = instanceStatus.Version == *m.MachinePool.Spec.Template.Spec.Version
			}

			if nodeStatus.Ready {
				readyReplicas++
			}
		}
	}

	m.AzureMachinePool.Status.Replicas = readyReplicas
	m.AzureMachinePool.Spec.ProviderIDList = providerIDs
	m.AzureMachinePool.Status.Instances = instanceStatuses
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

// SetProvisioningState sets the AzureMachinePool provisioning state.
func (m *MachinePoolScope) SetProvisioningState(v infrav1.VMState) {
	switch {
	case v == infrav1.VMStateSucceeded && *m.MachinePool.Spec.Replicas == m.AzureMachinePool.Status.Replicas:
		// vmss is provisioned with enough ready replicas
		m.AzureMachinePool.Status.ProvisioningState = &v
	case v == infrav1.VMStateSucceeded && *m.MachinePool.Spec.Replicas != m.AzureMachinePool.Status.Replicas:
		// not enough ready or too many ready replicas we must still be scaling up or down
		updatingState := infrav1.VMStateUpdating
		m.AzureMachinePool.Status.ProvisioningState = &updatingState
	default:
		m.AzureMachinePool.Status.ProvisioningState = &v
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

func (m *MachinePoolScope) getNodeStatusByProviderID(ctx context.Context, providerIDList []string) (map[string]*NodeStatus, error) {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolScope.getNodeStatusByProviderID")
	defer span.End()

	nodeStatusMap := map[string]*NodeStatus{}
	for _, id := range providerIDList {
		nodeStatusMap[id] = &NodeStatus{}
	}

	workloadClient, err := m.getWorkloadClient(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the workload cluster client")
	}

	nodeList := corev1.NodeList{}
	for {
		if err := workloadClient.List(ctx, &nodeList, client.Continue(nodeList.Continue)); err != nil {
			return nil, errors.Wrapf(err, "failed to List nodes")
		}

		for _, node := range nodeList.Items {
			if status, ok := nodeStatusMap[node.Spec.ProviderID]; ok {
				status.Ready = nodeIsReady(node)
				status.Version = node.Status.NodeInfo.KubeletVersion
			}
		}

		if nodeList.Continue == "" {
			break
		}
	}

	return nodeStatusMap, nil
}

func (m *MachinePoolScope) getWorkloadClient(ctx context.Context) (client.Client, error) {
	ctx, span := tele.Tracer().Start(ctx, "scope.MachinePoolScope.getWorkloadClient")
	defer span.End()

	obj := client.ObjectKey{
		Namespace: m.MachinePool.Namespace,
		Name:      m.ClusterName(),
	}
	dataBytes, err := utilkubeconfig.FromSecret(ctx, m.client, obj)
	if err != nil {
		return nil, errors.Wrapf(err, "\"%s-kubeconfig\" not found in namespace %q", obj.Name, obj.Namespace)
	}

	config, err := clientcmd.Load(dataBytes)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load \"%s-kubeconfig\" in namespace %q", obj.Name, obj.Namespace)
	}

	restConfig, err := clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, errors.Wrapf(err, "failed transform config \"%s-kubeconfig\" in namespace %q", obj.Name, obj.Namespace)
	}

	return client.New(restConfig, client.Options{})
}

func nodeIsReady(node corev1.Node) bool {
	for _, n := range node.Status.Conditions {
		if n.Type == corev1.NodeReady {
			return n.Status == corev1.ConditionTrue
		}
	}
	return false
}

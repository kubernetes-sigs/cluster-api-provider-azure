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
	"fmt"

	asoredhatopenshiftv1 "github.com/Azure/azure-service-operator/v2/api/redhatopenshift/v1api20240610preview"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	cplane "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
	v1beta2 "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-azure/util/futures"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// AROMachinePoolScopeParams defines the input parameters used to create a new Scope.
type AROMachinePoolScopeParams struct {
	AzureClients
	Client               client.Client
	Cluster              *clusterv1.Cluster
	MachinePool          *clusterv1.MachinePool
	ControlPlane         *cplane.AROControlPlane
	AROMachinePool       *v1beta2.AROMachinePool
	Cache                *AROMachinePoolCache
	Timeouts             azure.AsyncReconciler
	CredentialCache      azure.CredentialCache
	AROControlPlaneScope *AROControlPlaneScope
}

// NewAROMachinePoolScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewAROMachinePoolScope(ctx context.Context, params AROMachinePoolScopeParams) (*AROMachinePoolScope, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "azure.aroMachinePoolScope.NewAROMachinePoolScope")
	defer done()

	if params.AROMachinePool == nil {
		return nil, errors.New("failed to generate new scope from nil AROMachinePool")
	}

	credentialsProvider, err := NewAzureCredentialsProvider(ctx, params.CredentialCache, params.Client, params.ControlPlane.Spec.IdentityRef, params.AROMachinePool.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init credentials provider")
	}
	err = params.AzureClients.setCredentialsWithProvider(ctx, params.ControlPlane.Spec.SubscriptionID, params.ControlPlane.Spec.AzureEnvironment, credentialsProvider)
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure azure settings and credentials for Identity")
	}

	if params.Cache == nil {
		params.Cache = &AROMachinePoolCache{}
	}

	helper, err := patch.NewHelper(params.AROMachinePool, params.Client)
	if err != nil {
		return nil, errors.Errorf("failed to init patch helper: %v", err)
	}

	capiMachinePoolPatchHelper, err := patch.NewHelper(params.MachinePool, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}
	return &AROMachinePoolScope{
		Client:                     params.Client,
		patchHelper:                helper,
		cache:                      params.Cache,
		AzureClients:               params.AzureClients,
		Cluster:                    params.Cluster,
		MachinePool:                params.MachinePool,
		ControlPlane:               params.ControlPlane,
		InfraMachinePool:           params.AROMachinePool,
		capiMachinePoolPatchHelper: capiMachinePoolPatchHelper,
		AsyncReconciler:            params.Timeouts,
	}, nil
}

// AROMachinePoolScope defines the basic context for an actuator to operate upon.
type AROMachinePoolScope struct {
	Client                     client.Client
	patchHelper                *patch.Helper
	capiMachinePoolPatchHelper *patch.Helper
	cache                      *AROMachinePoolCache

	AzureClients
	Cluster          *clusterv1.Cluster
	ControlPlane     *cplane.AROControlPlane
	MachinePool      *clusterv1.MachinePool
	InfraMachinePool *v1beta2.AROMachinePool

	azure.AsyncReconciler
}

// SetLongRunningOperationState will set the future on the AROMachinePool status to allow the resource to continue
// in the next reconciliation.
func (s *AROMachinePoolScope) SetLongRunningOperationState(future *infrav1.Future) {
	futures.Set(s.InfraMachinePool, future)
}

// GetLongRunningOperationState will get the future on the AROMachinePool status.
func (s *AROMachinePoolScope) GetLongRunningOperationState(name, service, futureType string) *infrav1.Future {
	return futures.Get(s.InfraMachinePool, name, service, futureType)
}

// DeleteLongRunningOperationState will delete the future from the AROMachinePool status.
func (s *AROMachinePoolScope) DeleteLongRunningOperationState(name, service, futureType string) {
	futures.Delete(s.InfraMachinePool, name, service, futureType)
}

// UpdateDeleteStatus updates a condition on the AROMachinePool status after a DELETE operation.
func (s *AROMachinePoolScope) UpdateDeleteStatus(condition clusterv1.ConditionType, service string, err error) {
	switch {
	case err == nil:
		conditions.Set(s.InfraMachinePool, metav1.Condition{
			Type:    string(condition),
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.DeletedReason,
			Message: fmt.Sprintf("%s successfully deleted", service),
		})
	case azure.IsOperationNotDoneError(err):
		conditions.Set(s.InfraMachinePool, metav1.Condition{
			Type:    string(condition),
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.DeletingReason,
			Message: fmt.Sprintf("%s deleting", service),
		})
	default:
		conditions.Set(s.InfraMachinePool, metav1.Condition{
			Type:    string(condition),
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.DeletionFailedReason,
			Message: fmt.Sprintf("%s failed to delete. err: %s", service, err.Error()),
		})
	}
}

// UpdatePutStatus updates a condition on the AROMachinePool status after a PUT operation.
func (s *AROMachinePoolScope) UpdatePutStatus(condition clusterv1.ConditionType, service string, err error) {
	switch {
	case err == nil:
		conditions.Set(s.InfraMachinePool, metav1.Condition{
			Type:   string(condition),
			Status: metav1.ConditionTrue,
			Reason: "Succeeded",
		})
	case azure.IsOperationNotDoneError(err):
		reason := infrav1.CreatingReason
		if s.InfraMachinePool.Status.ProvisioningState == ProvisioningStateUpdating {
			reason = infrav1.UpdatingReason
		}
		conditions.Set(s.InfraMachinePool, metav1.Condition{
			Type:    string(condition),
			Status:  metav1.ConditionFalse,
			Reason:  reason,
			Message: fmt.Sprintf("%s creating or updating", service),
		})
	default:
		conditions.Set(s.InfraMachinePool, metav1.Condition{
			Type:    string(condition),
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.FailedReason,
			Message: fmt.Sprintf("%s failed to create or update. err: %s", service, err.Error()),
		})
	}
}

// UpdatePatchStatus updates a condition on the AROMachinePool status after a PATCH operation.
func (s *AROMachinePoolScope) UpdatePatchStatus(condition clusterv1.ConditionType, service string, err error) {
	switch {
	case err == nil:
		conditions.Set(s.InfraMachinePool, metav1.Condition{
			Type:   string(condition),
			Status: metav1.ConditionTrue,
			Reason: "Succeeded",
		})
	case azure.IsOperationNotDoneError(err):
		conditions.Set(s.InfraMachinePool, metav1.Condition{
			Type:    string(condition),
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.UpdatingReason,
			Message: fmt.Sprintf("%s updating", service),
		})
	default:
		conditions.Set(s.InfraMachinePool, metav1.Condition{
			Type:    string(condition),
			Status:  metav1.ConditionFalse,
			Reason:  infrav1.FailedReason,
			Message: fmt.Sprintf("%s failed to update. err: %s", service, err.Error()),
		})
	}
}

// AROMachinePoolCache stores AROMachinePoolCache data locally so we don't have to hit the API multiple times within the same reconcile loop.
type AROMachinePoolCache struct {
}

// BaseURI returns the Azure ResourceManagerEndpoint.
func (s *AROMachinePoolScope) BaseURI() string {
	return s.ResourceManagerEndpoint
}

// GetClient returns the controller-runtime client.
func (s *AROMachinePoolScope) GetClient() client.Client {
	return s.Client
}

// GetDeletionTimestamp returns the deletion timestamp of the Cluster.
func (s *AROMachinePoolScope) GetDeletionTimestamp() *metav1.Time {
	return s.Cluster.DeletionTimestamp
}

// PatchObject persists the control plane configuration and status.
func (s *AROMachinePoolScope) PatchObject(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scope.ManagedMachinePoolScope.PatchObject")
	defer done()

	return s.patchHelper.Patch(
		ctx,
		s.InfraMachinePool,
		patch.WithOwnedConditions{Conditions: []string{
			string(clusterv1.ReadyCondition),
			string(v1beta2.AROMachinePoolReadyCondition),
			// string(v1beta2.AROMachinePoolValidCondition),
			// string(v1beta2.AROMachinePoolUpgradingCondition),
		}})
}

// PatchCAPIMachinePoolObject persists the capi machinepool configuration and status.
func (s *AROMachinePoolScope) PatchCAPIMachinePoolObject(ctx context.Context) error {
	return s.capiMachinePoolPatchHelper.Patch(
		ctx,
		s.MachinePool,
	)
}

// SetAgentPoolProviderIDList sets a list of agent pool's Azure VM IDs.
func (s *AROMachinePoolScope) SetAgentPoolProviderIDList(providerIDs []string) {
	s.InfraMachinePool.Spec.ProviderIDList = providerIDs
}

// SetAgentPoolReplicas sets the number of agent pool replicas.
func (s *AROMachinePoolScope) SetAgentPoolReplicas(replicas int32) {
	s.InfraMachinePool.Status.Replicas = replicas
}

// SetAgentPoolProvisioningState sets the provisioning state for the agent pool.
func (s *AROMachinePoolScope) SetAgentPoolProvisioningState(state string) {
	s.InfraMachinePool.Status.ProvisioningState = state
}

// SetAgentPoolReady sets the flag that indicates if the agent pool is ready or not.
func (s *AROMachinePoolScope) SetAgentPoolReady(ready bool) {
	if s.InfraMachinePool.Status.ProvisioningState != ProvisioningStateSucceeded &&
		s.InfraMachinePool.Status.ProvisioningState != ProvisioningStateUpdating {
		ready = false
	}
	s.InfraMachinePool.Status.Ready = ready
	if s.InfraMachinePool.Status.Initialization == nil || !s.InfraMachinePool.Status.Initialization.Provisioned {
		s.InfraMachinePool.Status.Initialization = &v1beta2.AROMachinePoolInitializationStatus{Provisioned: ready}
	}
}

// Close closes the current scope persisting the control plane configuration and status.
func (s *AROMachinePoolScope) Close(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "scope.AROMachinePoolScope.Close")
	defer done()

	return s.PatchObject(ctx)
}

// Name returns the machine pool name.
func (s *AROMachinePoolScope) Name() string {
	return s.InfraMachinePool.Name
}

// Location returns location.
func (s *AROMachinePoolScope) Location() string {
	return s.ControlPlane.Spec.Platform.Location
}

// ResourceGroup returns the cluster resource group.
func (s *AROMachinePoolScope) ResourceGroup() string {
	return s.ControlPlane.Spec.Platform.ResourceGroup
}

// NodeResourceGroup returns the node resource group name.
func (s *AROMachinePoolScope) NodeResourceGroup() string {
	return s.ControlPlane.NodeResourceGroup()
}

// ClusterName returns the cluster name.
func (s *AROMachinePoolScope) ClusterName() string {
	return s.Cluster.Name
}

// Namespace returns the cluster namespace.
func (s *AROMachinePoolScope) Namespace() string {
	return s.Cluster.Namespace
}

// HcpOpenShiftNodePoolProperties returns the properties for the ASO HcpOpenShiftClusterNodePool resource.
func (s *AROMachinePoolScope) HcpOpenShiftNodePoolProperties() *asoredhatopenshiftv1.NodePoolProperties {
	props := &asoredhatopenshiftv1.NodePoolProperties{}

	// Set auto-repair
	props.AutoRepair = ptr.To(s.InfraMachinePool.Spec.AutoRepair)

	// Set autoscaling or replicas
	if s.InfraMachinePool.Spec.Autoscaling != nil {
		props.AutoScaling = &asoredhatopenshiftv1.NodePoolAutoScaling{
			Min: ptr.To(s.InfraMachinePool.Spec.Autoscaling.MinReplicas),
			Max: ptr.To(s.InfraMachinePool.Spec.Autoscaling.MaxReplicas),
		}
	} else if s.MachinePool.Spec.Replicas != nil {
		props.Replicas = ptr.To(int(*s.MachinePool.Spec.Replicas))
	}

	// Set labels
	if len(s.InfraMachinePool.Spec.Labels) > 0 {
		labels := make([]asoredhatopenshiftv1.Label, 0, len(s.InfraMachinePool.Spec.Labels))
		for key, value := range s.InfraMachinePool.Spec.Labels {
			labels = append(labels, asoredhatopenshiftv1.Label{
				Key:   ptr.To(key),
				Value: ptr.To(value),
			})
		}
		props.Labels = labels
	}

	// Set taints
	if len(s.InfraMachinePool.Spec.Taints) > 0 {
		taints := make([]asoredhatopenshiftv1.Taint, 0, len(s.InfraMachinePool.Spec.Taints))
		for _, taint := range s.InfraMachinePool.Spec.Taints {
			asoTaint := asoredhatopenshiftv1.Taint{
				Key: ptr.To(taint.Key),
			}
			if taint.Value != "" {
				asoTaint.Value = ptr.To(taint.Value)
			}
			// Convert effect
			switch taint.Effect {
			case corev1.TaintEffectNoSchedule:
				asoTaint.Effect = ptr.To(asoredhatopenshiftv1.Effect_NoSchedule)
			case corev1.TaintEffectPreferNoSchedule:
				asoTaint.Effect = ptr.To(asoredhatopenshiftv1.Effect_PreferNoSchedule)
			case corev1.TaintEffectNoExecute:
				asoTaint.Effect = ptr.To(asoredhatopenshiftv1.Effect_NoExecute)
			}
			taints = append(taints, asoTaint)
		}
		props.Taints = taints
	}

	// Set version
	if s.InfraMachinePool.Spec.Version != "" {
		props.Version = &asoredhatopenshiftv1.NodePoolVersionProfile{
			Id:           ptr.To(s.InfraMachinePool.Spec.Version),
			ChannelGroup: ptr.To(string(s.ControlPlane.Spec.ChannelGroup)),
		}
	}

	// Set platform configuration
	platform := &asoredhatopenshiftv1.NodePoolPlatformProfile{}

	// VM size
	if s.InfraMachinePool.Spec.Platform.VMSize != "" {
		platform.VmSize = ptr.To(s.InfraMachinePool.Spec.Platform.VMSize)
	}

	// Availability zone
	if s.InfraMachinePool.Spec.Platform.AvailabilityZone != "" {
		platform.AvailabilityZone = ptr.To(s.InfraMachinePool.Spec.Platform.AvailabilityZone)
	}

	// OSDisk configuration
	osDisk := &asoredhatopenshiftv1.OsDiskProfile{}
	if s.InfraMachinePool.Spec.Platform.DiskSizeGiB > 0 {
		osDisk.SizeGiB = ptr.To(int(s.InfraMachinePool.Spec.Platform.DiskSizeGiB))
	}

	// Disk storage account type
	if s.InfraMachinePool.Spec.Platform.DiskStorageAccountType != "" {
		switch s.InfraMachinePool.Spec.Platform.DiskStorageAccountType {
		case "Premium_LRS":
			osDisk.DiskStorageAccountType = ptr.To(asoredhatopenshiftv1.OsDiskProfile_DiskStorageAccountType_Premium_LRS)
		case "StandardSSD_LRS":
			osDisk.DiskStorageAccountType = ptr.To(asoredhatopenshiftv1.OsDiskProfile_DiskStorageAccountType_StandardSSD_LRS)
		case "Standard_LRS":
			osDisk.DiskStorageAccountType = ptr.To(asoredhatopenshiftv1.OsDiskProfile_DiskStorageAccountType_Standard_LRS)
		}
	}

	platform.OsDisk = osDisk

	// Subnet - use reference if SubnetRef is set, otherwise use direct ID
	if s.InfraMachinePool.Spec.Platform.SubnetRef != "" {
		platform.SubnetReference = &genruntime.ResourceReference{
			ARMID: s.InfraMachinePool.Spec.Platform.Subnet,
		}
	} else if s.InfraMachinePool.Spec.Platform.Subnet != "" {
		platform.SubnetReference = &genruntime.ResourceReference{
			ARMID: s.InfraMachinePool.Spec.Platform.Subnet,
		}
	}

	props.Platform = platform

	return props
}

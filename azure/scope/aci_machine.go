/*
Copyright 2021 The Kubernetes Authors.

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
	"k8s.io/klog/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

type (
	// ContainerInstanceMachineScope defines a scope defined around an container instance machine and its cluster.
	ContainerInstanceMachineScope struct {
		logr.Logger
		client      client.Client
		patchHelper *patch.Helper

		azure.ClusterScoper
		Machine    *clusterv1.Machine
		ACIMachine *infrav1.AzureContainerInstanceMachine
	}

	// ContainerInstanceMachineScopeParams defines the input parameters used to create a new ContainerInstanceMachineScope.
	ContainerInstanceMachineScopeParams struct {
		Client       client.Client
		Logger       logr.Logger
		ClusterScope azure.ClusterScoper
		Machine      *clusterv1.Machine
		ACIMachine   *infrav1.AzureContainerInstanceMachine
	}
)

// NewContainerInstanceMachineScope creates a new ContainerInstanceMachineScope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewContainerInstanceMachineScope(params ContainerInstanceMachineScopeParams) (*ContainerInstanceMachineScope, error) {
	if params.Client == nil {
		return nil, errors.New("client is required when creating a MachineScope")
	}
	if params.Machine == nil {
		return nil, errors.New("machine is required when creating a MachineScope")
	}
	if params.ACIMachine == nil {
		return nil, errors.New("azure machine is required when creating a MachineScope")
	}
	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	helper, err := patch.NewHelper(params.ACIMachine, params.Client)
	if err != nil {
		return nil, errors.Errorf("failed to init patch helper: %v ", err)
	}
	return &ContainerInstanceMachineScope{
		client:        params.Client,
		Machine:       params.Machine,
		ACIMachine:    params.ACIMachine,
		Logger:        params.Logger,
		patchHelper:   helper,
		ClusterScoper: params.ClusterScope,
	}, nil
}

// PatchObject persists the machine spec and status.
func (m *ContainerInstanceMachineScope) PatchObject(ctx context.Context) error {
	switch m.VMState() {
	case infrav1.VMStateSucceeded:
		conditions.MarkTrue(m.ACIMachine, infrav1.VMRunningCondition)
		m.SetReady()
	case infrav1.VMStateCreating:
		conditions.MarkFalse(m.ACIMachine, infrav1.VMRunningCondition, infrav1.VMNCreatingReason, clusterv1.ConditionSeverityInfo, "")
		m.SetNotReady()
	case infrav1.VMStateUpdating:
		conditions.MarkFalse(m.ACIMachine, infrav1.VMRunningCondition, infrav1.VMNUpdatingReason, clusterv1.ConditionSeverityInfo, "")
		m.SetNotReady()
	case infrav1.VMStateDeleting:
		conditions.MarkFalse(m.ACIMachine, infrav1.VMRunningCondition, infrav1.VMDDeletingReason, clusterv1.ConditionSeverityWarning, "")
		m.SetNotReady()
	case infrav1.VMStateFailed:
		m.Error(errors.New("Failed to create or update ACIMachine"), "ACIMachine is in failed state")
		m.SetFailureReason(capierrors.UpdateMachineError)
		m.SetFailureMessage(errors.Errorf("Azure VM state is %s", m.VMState()))
		conditions.MarkFalse(m.ACIMachine, infrav1.VMRunningCondition, infrav1.VMProvisionFailedReason, clusterv1.ConditionSeverityWarning, "")
		m.SetNotReady()
	default:
		conditions.MarkUnknown(m.ACIMachine, infrav1.VMRunningCondition, "", "")
		m.SetNotReady()
	}

	m.V(2).Info(fmt.Sprintf("ACIMachine is in state %s", m.VMState()))

	conditions.SetSummary(m.ACIMachine,
		conditions.WithConditions(
			infrav1.VMRunningCondition,
		),
		conditions.WithStepCounterIfOnly(
			infrav1.VMRunningCondition,
		),
	)

	return m.patchHelper.Patch(
		ctx,
		m.ACIMachine,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			infrav1.VMRunningCondition,
		}})
}

// Close the ContainerInstanceMachineScope by updating the ACI machine spec, ACI machine status.
func (m *ContainerInstanceMachineScope) Close(ctx context.Context) error {
	return m.PatchObject(ctx)
}

// ContainerGroupSpec converts the ACIMachine resource into the service spec
func (m *ContainerInstanceMachineScope) ContainerGroupSpec(ctx context.Context) (azure.ContainerGroupSpec, error) {
	bootstrapData, err := m.getBootstrapData(ctx)
	if err != nil {
		return azure.ContainerGroupSpec{}, errors.Wrap(err, "failed to get boostrap data when building container group spec")
	}

	return azure.ContainerGroupSpec{
		Name:                   generateContainerGroupName(m.ClusterName(), m.ACIMachine.Name),
		Identity:               m.ACIMachine.Spec.Identity,
		UserAssignedIdentities: m.ACIMachine.Spec.UserAssignedIdentities,
		Containers: []azure.ContainerSpec{
			{
				Name:    "virtual-kubelet",
				Command: []string{"tail", "-f", "/dev/null"},
				EnvVars: []azure.ContainerEnvironmentVariableSpec{
					{
						Name:  "foo",
						Value: "not_secret",
					},
				},
				Image: "devigned/vktest",
				VolumeMounts: []azure.ContainerVolumeMountSpec{
					{
						Name: "bootstrapVolume",
						MountPath: "/var/lib/capi",
					},
				},
			},
		},
		Volumes: []azure.ContainerVolumeSpec{
			{
				Name:    "bootstrapVolume",
				Secrets: []azure.ContainerSecretSpec{
					{
						Name:  "bootstrap.yml",
						Value: bootstrapData,
					},
				},
			},
		},
	}, nil
}

// Name for the ACIMachine in scope
func (m *ContainerInstanceMachineScope) Name() string {
	return m.ACIMachine.Name
}

// Namespace for the ACIMachine in scope
func (m *ContainerInstanceMachineScope) Namespace() string {
	return m.ACIMachine.Namespace
}

// VMState will get the provisioning state of the ACI machine
func (m *ContainerInstanceMachineScope) VMState() infrav1.VMState {
	if m.ACIMachine.Status.VMState != nil {
		return *m.ACIMachine.Status.VMState
	}
	return ""
}

// SetVMState will set the provisioning state of the ACI machine
func (m *ContainerInstanceMachineScope) SetVMState(v infrav1.VMState) {
	m.ACIMachine.Status.VMState = &v
}

// SetReady marks the ACIMachine status to ready
func (m *ContainerInstanceMachineScope) SetReady() {
	m.ACIMachine.Status.Ready = true
}

// SetNotReady marks the ACIMachine status to not ready
func (m *ContainerInstanceMachineScope) SetNotReady() {
	m.ACIMachine.Status.Ready = false
}

// SetProviderID will set the ProviderID on the ACIMachine
func (m *ContainerInstanceMachineScope) SetProviderID(id string) {
	m.ACIMachine.Spec.ProviderID = &id
}

// SetFailureReason sets the failure reason the ACIMachine
func (m *ContainerInstanceMachineScope) SetFailureReason(reason capierrors.MachineStatusError) {
	m.ACIMachine.Status.FailureReason = &reason
}

// SetFailureMessage sets the failure message on the ACIMachine
func (m *ContainerInstanceMachineScope) SetFailureMessage(v error) {
	m.ACIMachine.Status.FailureMessage = to.StringPtr(v.Error())
}

// getBootstrapData returns the bootstrap data from the secret in the Machine's bootstrap.dataSecretName.
func (m *ContainerInstanceMachineScope) getBootstrapData(ctx context.Context) (string, error) {
	if m.Machine.Spec.Bootstrap.DataSecretName == nil {
		return "", errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")
	}
	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: m.ACIMachine.Namespace, Name: *m.Machine.Spec.Bootstrap.DataSecretName }
	if err := m.client.Get(ctx, key, secret); err != nil {
		return "", errors.Wrapf(err, "failed to retrieve bootstrap data secret for AzureMachine %s/%s", m.ACIMachine.Namespace, m.ACIMachine.Name)
	}

	value, ok := secret.Data["value"]
	if !ok {
		return "", errors.New("error retrieving bootstrap data: secret value key is missing")
	}
	return base64.StdEncoding.EncodeToString(value), nil
}

// generateContainerGroupName generates a container group name
func generateContainerGroupName(clusterName, containerGroupName string) string {
	return fmt.Sprintf("%s-%s", clusterName, containerGroupName)
}

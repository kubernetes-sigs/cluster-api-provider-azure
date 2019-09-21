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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/klog/klogr"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
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
	Cluster      *clusterv1.Cluster
	Machine      *clusterv1.Machine
	AzureCluster *infrav1.AzureCluster
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
	if params.Cluster == nil {
		return nil, errors.New("cluster is required when creating a MachineScope")
	}
	if params.AzureCluster == nil {
		return nil, errors.New("azure cluster is required when creating a MachineScope")
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
		Cluster:      params.Cluster,
		Machine:      params.Machine,
		AzureCluster: params.AzureCluster,
		AzureMachine: params.AzureMachine,
		Logger:       params.Logger,
		patchHelper:  helper,
	}, nil
}

// MachineScope defines a scope defined around a machine and its cluster.
type MachineScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	Cluster      *clusterv1.Cluster
	Machine      *clusterv1.Machine
	AzureCluster *infrav1.AzureCluster
	AzureMachine *infrav1.AzureMachine
}

// Region returns the AzureMachine region.
func (m *MachineScope) Region() string {
	return m.AzureCluster.Spec.Region
}

// Zone returns the AzureMachine zone.
func (m *MachineScope) Zone() string {
	return m.AzureMachine.Spec.Zone
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
		return "control-plane"
	}
	return "node"
}

// GetInstanceID returns the AzureMachine instance id by parsing Spec.ProviderID.
func (m *MachineScope) GetInstanceID() *string {
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

// GetInstanceStatus returns the AzureMachine instance status.
func (m *MachineScope) GetInstanceStatus() *infrav1.InstanceStatus {
	return m.AzureMachine.Status.InstanceStatus
}

// SetInstanceStatus sets the AzureMachine instance status.
func (m *MachineScope) SetInstanceStatus(v infrav1.InstanceStatus) {
	m.AzureMachine.Status.InstanceStatus = &v
}

// SetReady sets the AzureMachine Ready Status
func (m *MachineScope) SetReady() {
	m.AzureMachine.Status.Ready = true
}

// SetErrorMessage sets the AzureMachine status error message.
func (m *MachineScope) SetErrorMessage(v error) {
	m.AzureMachine.Status.ErrorMessage = pointer.StringPtr(v.Error())
}

// SetErrorReason sets the AzureMachine status error reason.
func (m *MachineScope) SetErrorReason(v capierrors.MachineStatusError) {
	m.AzureMachine.Status.ErrorReason = &v
}

// SetAnnotation sets a key value annotation on the AzureMachine.
func (m *MachineScope) SetAnnotation(key, value string) {
	if m.AzureMachine.Annotations == nil {
		m.AzureMachine.Annotations = map[string]string{}
	}
	m.AzureMachine.Annotations[key] = value
}

// Close the MachineScope by updating the machine spec, machine status.
func (m *MachineScope) Close() error {
	return m.patchHelper.Patch(context.TODO(), m.AzureMachine)
}

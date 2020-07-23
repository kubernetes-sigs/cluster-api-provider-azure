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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/klogr"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1alpha3"

	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagedControlPlaneScopeParams defines the input parameters used to create a new
type ManagedControlPlaneScopeParams struct {
	*AzureClients
	Client           client.Client
	Logger           logr.Logger
	Cluster          *clusterv1.Cluster
	ControlPlane     *infrav1exp.AzureManagedControlPlane
	InfraMachinePool *infrav1exp.AzureManagedMachinePool
	MachinePool      *expv1.MachinePool
	PatchTarget      runtime.Object
	Scheme           *runtime.Scheme
}

// NewManagedControlPlaneScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewManagedControlPlaneScope(params ManagedControlPlaneScopeParams) (*ManagedControlPlaneScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}

	if params.PatchTarget == nil {
		return nil, errors.New("failed to generate new scope with nil patch target")
	}

	if params.ControlPlane == nil {
		return nil, errors.New("failed to generate new scope from nil control plane")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	helper, err := patch.NewHelper(params.PatchTarget, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &ManagedControlPlaneScope{
		Logger:           params.Logger,
		Client:           params.Client,
		AzureClients:     params.AzureClients,
		Cluster:          params.Cluster,
		MachinePool:      params.MachinePool,
		InfraMachinePool: params.InfraMachinePool,
		patchHelper:      helper,
		PatchTarget:      params.PatchTarget,
		ClusterDescriber: params.ControlPlane,
		ControlPlane:     params.ControlPlane,
		Scheme:           params.Scheme,
	}, nil
}

// ManagedControlPlaneScope defines the basic context for an actuator to operate upon.
type ManagedControlPlaneScope struct {
	logr.Logger
	Client      client.Client
	patchHelper *patch.Helper

	*AzureClients
	Cluster     *clusterv1.Cluster
	MachinePool *expv1.MachinePool
	azure.ClusterDescriber
	ControlPlane     *infrav1exp.AzureManagedControlPlane
	InfraMachinePool *infrav1exp.AzureManagedMachinePool
	PatchTarget      runtime.Object
	Scheme           *runtime.Scheme
}

// PatchObject persists the cluster configuration and status.
func (s *ManagedControlPlaneScope) PatchObject(ctx context.Context) error {
	return s.patchHelper.Patch(ctx, s.PatchTarget)
}

// VNetSpecs returns the virtual network specs.
func (s *ManagedControlPlaneScope) VNetSpecs() []azure.VNetSpec {
	return []azure.VNetSpec{
		{
			ResourceGroup: s.Vnet().ResourceGroup,
			Name:          s.Vnet().Name,
		},
	}
}

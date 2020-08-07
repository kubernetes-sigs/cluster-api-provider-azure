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

	"github.com/Azure/go-autorest/autorest"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1alpha3"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"

	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ManagedControlPlaneScopeParams defines the input parameters used to create a new
type ManagedControlPlaneScopeParams struct {
	AzureClients
	Client           client.Client
	Logger           logr.Logger
	Cluster          *clusterv1.Cluster
	ControlPlane     *infrav1exp.AzureManagedControlPlane
	InfraMachinePool *infrav1exp.AzureManagedMachinePool
	MachinePool      *expv1.MachinePool
	PatchTarget      runtime.Object
}

// NewManagedControlPlaneScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewManagedControlPlaneScope(params ManagedControlPlaneScopeParams) (*ManagedControlPlaneScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}

	if params.ControlPlane == nil {
		return nil, errors.New("failed to generate new scope from nil ControlPlane")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	if err := params.AzureClients.setCredentials(params.ControlPlane.Spec.SubscriptionID); err != nil {
		return nil, errors.Wrap(err, "failed to create Azure session")
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
		ControlPlane:     params.ControlPlane,
		MachinePool:      params.MachinePool,
		InfraMachinePool: params.InfraMachinePool,
		PatchTarget:      params.PatchTarget,
		patchHelper:      helper,
	}, nil
}

// ManagedControlPlaneScope defines the basic context for an actuator to operate upon.
type ManagedControlPlaneScope struct {
	logr.Logger
	Client      client.Client
	patchHelper *patch.Helper

	AzureClients
	Cluster          *clusterv1.Cluster
	MachinePool      *expv1.MachinePool
	ControlPlane     *infrav1exp.AzureManagedControlPlane
	InfraMachinePool *infrav1exp.AzureManagedMachinePool
	PatchTarget      runtime.Object
}

func (s *ManagedControlPlaneScope) ResourceGroup() string {
	if s.ControlPlane == nil {
		return ""
	}
	return s.ControlPlane.Spec.ResourceGroup
}

func (s *ManagedControlPlaneScope) ClusterName() string {
	return s.Cluster.Name
}

func (s *ManagedControlPlaneScope) Location() string {
	if s.ControlPlane == nil {
		return ""
	}
	return s.ControlPlane.Spec.Location
}

// AdditionalTags returns AdditionalTags from the ControlPlane spec.
func (s *ManagedControlPlaneScope) AdditionalTags() infrav1.Tags {
	tags := make(infrav1.Tags)
	if s.ControlPlane.Spec.AdditionalTags != nil {
		tags = s.ControlPlane.Spec.AdditionalTags.DeepCopy()
	}
	return tags
}

// SubscriptionID returns the Azure client Subscription ID.
func (s *ManagedControlPlaneScope) SubscriptionID() string {
	return s.ControlPlane.Spec.SubscriptionID
}

// BaseURI returns the Azure ResourceManagerEndpoint.
func (s *ManagedControlPlaneScope) BaseURI() string {
	return s.AzureClients.ResourceManagerEndpoint
}

// Authorizer returns the Azure client Authorizer.
func (s *ManagedControlPlaneScope) Authorizer() autorest.Authorizer {
	return s.AzureClients.Authorizer
}

// PatchObject persists the cluster configuration and status.
func (s *ManagedControlPlaneScope) PatchObject(ctx context.Context) error {
	return s.patchHelper.Patch(ctx, s.PatchTarget)
}

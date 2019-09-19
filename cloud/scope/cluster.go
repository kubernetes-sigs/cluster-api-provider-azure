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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"google.golang.org/api/compute/v1"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha2"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterScopeParams defines the input parameters used to create a new Scope.
type ClusterScopeParams struct {
	AzureClients
	Client     client.Client
	Logger     logr.Logger
	Cluster    *clusterv1.Cluster
	AzureCluster *infrav1.AzureCluster
}

// NewClusterScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewClusterScope(params ClusterScopeParams) (*ClusterScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}
	if params.AzureCluster == nil {
		return nil, errors.New("failed to generate new scope from nil AzureCluster")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	computeSvc, err := compute.NewService(context.TODO())
	if err != nil {
		return nil, errors.Errorf("failed to create azure compute client: %v", err)
	}

	if params.AzureClients.Compute == nil {
		params.AzureClients.Compute = computeSvc
	}

	helper, err := patch.NewHelper(params.AzureCluster, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}
	return &ClusterScope{
		Logger:      params.Logger,
		client:      params.Client,
		AzureClients:  params.AzureClients,
		Cluster:     params.Cluster,
		AzureCluster:  params.AzureCluster,
		patchHelper: helper,
	}, nil
}

// ClusterScope defines the basic context for an actuator to operate upon.
type ClusterScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	AzureClients
	Cluster    *clusterv1.Cluster
	AzureCluster *infrav1.AzureCluster
}

// Project returns the current project name.
func (s *ClusterScope) Project() string {
	return s.AzureCluster.Spec.Project
}

// Network returns the cluster network unique identifier.
func (s *ClusterScope) NetworkID() string {
	return s.AzureCluster.Spec.NetworkSpec.Name
}

// Network returns the cluster network object.
func (s *ClusterScope) Network() *infrav1.Network {
	return &s.AzureCluster.Status.Network
}

// Subnets returns the cluster subnets.
func (s *ClusterScope) Subnets() infrav1.Subnets {
	return s.AzureCluster.Spec.NetworkSpec.Subnets
}

// Name returns the cluster name.
func (s *ClusterScope) Name() string {
	return s.Cluster.Name
}

// Namespace returns the cluster namespace.
func (s *ClusterScope) Namespace() string {
	return s.Cluster.Namespace
}

// Region returns the cluster region.
func (s *ClusterScope) Region() string {
	return s.AzureCluster.Spec.Region
}

// ControlPlaneConfigMapName returns the name of the ConfigMap used to
// coordinate the bootstrapping of control plane nodes.
func (s *ClusterScope) ControlPlaneConfigMapName() string {
	return fmt.Sprintf("%s-controlplane", s.Cluster.UID)
}

// ListOptionsLabelSelector returns a ListOptions with a label selector for clusterName.
func (s *ClusterScope) ListOptionsLabelSelector() client.ListOption {
	return client.MatchingLabels(map[string]string{
		clusterv1.MachineClusterLabelName: s.Cluster.Name,
	})
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ClusterScope) Close() error {
	return s.patchHelper.Patch(context.TODO(), s.AzureCluster)
}

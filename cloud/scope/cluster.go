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
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterScopeParams defines the input parameters used to create a new Scope.
type ClusterScopeParams struct {
	*AzureClients
	Client           client.Client
	Logger           logr.Logger
	Cluster          *clusterv1.Cluster
	ClusterDescriber azure.ClusterDescriber
}

// NewClusterScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewClusterScope(params ClusterScopeParams) (*ClusterScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}

	if params.ClusterDescriber == nil {
		return nil, errors.New("failed to generate new scope from nil ClusterDescriber")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	helper, err := patch.NewHelper(params.ClusterDescriber, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &ClusterScope{
		Logger:           params.Logger,
		client:           params.Client,
		AzureClients:     params.AzureClients,
		Cluster:          params.Cluster,
		ClusterDescriber: params.ClusterDescriber,
		patchHelper:      helper,
	}, nil
}

// ClusterScope defines the basic context for an actuator to operate upon.
type ClusterScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	*AzureClients
	Cluster *clusterv1.Cluster
	azure.ClusterDescriber
}

// PublicIPSpecs returns the public IP specs.
func (s *ClusterScope) PublicIPSpecs() []azure.PublicIPSpec {
	return []azure.PublicIPSpec{
		{
			Name: azure.GenerateNodeOutboundIPName(s.ClusterName()),
		},
		{
			Name:    s.Network().APIServerIP.Name,
			DNSName: s.Network().APIServerIP.DNSName,
		},
	}
}

// LBSpecs returns the load balancer specs.
func (s *ClusterScope) LBSpecs() []azure.LBSpec {
	return []azure.LBSpec{
		{
			// Internal control plane LB
			Name:             azure.GenerateInternalLBName(s.ClusterName()),
			SubnetName:       s.ControlPlaneSubnet().Name,
			SubnetCidr:       s.ControlPlaneSubnet().CidrBlock,
			PrivateIPAddress: s.ControlPlaneSubnet().InternalLBIPAddress,
			APIServerPort:    s.APIServerPort(),
			Role:             infrav1.InternalRole,
		},
		{
			// Public API Server LB
			Name:          azure.GeneratePublicLBName(s.ClusterName()),
			PublicIPName:  s.Network().APIServerIP.Name,
			APIServerPort: s.APIServerPort(),
			Role:          infrav1.APIServerRole,
		},
		{
			// Public Node outbound LB
			Name:         s.ClusterName(),
			PublicIPName: azure.GenerateNodeOutboundIPName(s.ClusterName()),
			Role:         infrav1.NodeOutboundRole,
		},
	}
}

// RouteTableSpecs returns the node route table(s)
func (s *ClusterScope) RouteTableSpecs() []azure.RouteTableSpec {
	return []azure.RouteTableSpec{{
		Name: s.RouteTable().Name,
	}}
}

// SubnetSpecs returns the subnets specs.
func (s *ClusterScope) SubnetSpecs() []azure.SubnetSpec {
	return []azure.SubnetSpec{
		{
			Name:                s.ControlPlaneSubnet().Name,
			CIDR:                s.ControlPlaneSubnet().CidrBlock,
			VNetName:            s.Vnet().Name,
			SecurityGroupName:   s.ControlPlaneSubnet().SecurityGroup.Name,
			Role:                s.ControlPlaneSubnet().Role,
			RouteTableName:      s.ControlPlaneSubnet().RouteTable.Name,
			InternalLBIPAddress: s.ControlPlaneSubnet().InternalLBIPAddress,
		},
		{
			Name:              s.NodeSubnet().Name,
			CIDR:              s.NodeSubnet().CidrBlock,
			VNetName:          s.Vnet().Name,
			SecurityGroupName: s.NodeSubnet().SecurityGroup.Name,
			RouteTableName:    s.NodeSubnet().RouteTable.Name,
			Role:              s.NodeSubnet().Role,
		},
	}
}

// VNetSpecs returns the virtual network specs.
func (s *ClusterScope) VNetSpecs() []azure.VNetSpec {
	return []azure.VNetSpec{
		{
			ResourceGroup: s.Vnet().ResourceGroup,
			Name:          s.Vnet().Name,
			CIDR:          s.Vnet().CidrBlock,
		},
	}
}

// Namespace returns the cluster namespace.
func (s *ClusterScope) Namespace() string {
	return s.Cluster.Namespace
}

// GenerateFQDN generates a fully qualified domain name, based on the public IP name and cluster location.
func (s *ClusterScope) GenerateFQDN() string {
	return fmt.Sprintf("%s.%s.%s", s.Network().APIServerIP.Name, s.Location(), s.AzureClients.ResourceManagerVMDNSSuffix)
}

// ListOptionsLabelSelector returns a ListOptions with a label selector for clusterName.
func (s *ClusterScope) ListOptionsLabelSelector() client.ListOption {
	return client.MatchingLabels(map[string]string{
		clusterv1.ClusterLabelName: s.Cluster.Name,
	})
}

// PatchObject persists the cluster configuration and status.
func (s *ClusterScope) PatchObject(ctx context.Context) error {
	return s.patchHelper.Patch(ctx, s.ClusterDescriber)
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ClusterScope) Close(ctx context.Context) error {
	return s.patchHelper.Patch(ctx, s.ClusterDescriber)
}

// APIServerPort returns the APIServerPort to use when creating the load balancer.
func (s *ClusterScope) APIServerPort() int32 {
	if s.Cluster.Spec.ClusterNetwork != nil && s.Cluster.Spec.ClusterNetwork.APIServerPort != nil {
		return *s.Cluster.Spec.ClusterNetwork.APIServerPort
	}
	return 6443
}

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
	"strconv"

	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/klog/klogr"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/defaults"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterScopeParams defines the input parameters used to create a new Scope.
type ClusterScopeParams struct {
	AzureClients
	Client  client.Client
	Logger  logr.Logger
	Cluster *clusterv1.Cluster
	azure.ClusterScoper
}

// NewClusterScope creates a new Scope from the supplied parameters.
// This is meant to be called for each reconcile iteration.
func NewClusterScope(params ClusterScopeParams) (*ClusterScope, error) {
	if params.Cluster == nil {
		return nil, errors.New("failed to generate new scope from nil Cluster")
	}

	if params.ClusterScoper == nil {
		return nil, errors.New("failed to generate new scope from nil ClusterScoper")
	}

	if params.Logger == nil {
		params.Logger = klogr.New()
	}

	err := params.AzureClients.setCredentials(params.ClusterScoper.SubscriptionID())
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure azure settings and credentials from environment")
	}

	helper, err := patch.NewHelper(params.ClusterScoper, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &ClusterScope{
		Logger:        params.Logger,
		Client:        params.Client,
		AzureClients:  params.AzureClients,
		Cluster:       params.Cluster,
		ClusterScoper: params.ClusterScoper,
		patchHelper:   helper,
	}, nil
}

// ClusterScope defines the basic context for an actuator to operate upon.
type ClusterScope struct {
	logr.Logger
	Client      client.Client
	patchHelper *patch.Helper

	AzureClients
	Cluster *clusterv1.Cluster
	azure.ClusterScoper
}

// SubscriptionID returns the Azure client Subscription ID.
func (s *ClusterScope) SubscriptionID() string {
	return s.AzureClients.SubscriptionID()
}

// BaseURI returns the Azure ResourceManagerEndpoint.
func (s *ClusterScope) BaseURI() string {
	return s.AzureClients.ResourceManagerEndpoint
}

// Authorizer returns the Azure client Authorizer.
func (s *ClusterScope) Authorizer() autorest.Authorizer {
	return s.AzureClients.Authorizer
}

// PublicIPSpecs returns the public IP specs.
func (s *ClusterScope) PublicIPSpecs() []azure.PublicIPSpec {
	var controlPlaneOutboundIP azure.PublicIPSpec
	if s.IsAPIServerPrivate() {
		controlPlaneOutboundIP = azure.PublicIPSpec{
			Name: defaults.GenerateControlPlaneOutboundIPName(s.ClusterName()),
		}
	} else {
		controlPlaneOutboundIP = azure.PublicIPSpec{
			Name:    s.APIServerPublicIP().Name,
			DNSName: s.APIServerPublicIP().DNSName,
			IsIPv6:  false, // currently azure requires a ipv4 lb rule to enable ipv6
		}
	}

	return []azure.PublicIPSpec{
		controlPlaneOutboundIP,
		{
			Name: defaults.GenerateNodeOutboundIPName(s.ClusterName()),
		},
	}
}

// LBSpecs returns the load balancer specs.
func (s *ClusterScope) LBSpecs() []azure.LBSpec {
	specs := []azure.LBSpec{
		{
			// Control Plane LB
			Name:              s.APIServerLB().Name,
			SubnetName:        s.ControlPlaneSubnet().Name,
			FrontendIPConfigs: s.APIServerLB().FrontendIPs,
			APIServerPort:     s.APIServerPort(),
			Type:              s.APIServerLB().Type,
			SKU:               infrav1.SKUStandard,
			Role:              infrav1.APIServerRole,
			BackendPoolName:   s.APIServerLBPoolName(s.APIServerLB().Name),
		},
		{
			// Public Node outbound LB
			Name: s.NodeOutboundLBName(),
			FrontendIPConfigs: []infrav1.FrontendIP{
				{
					Name: defaults.GenerateFrontendIPConfigName(s.NodeOutboundLBName()),
					PublicIP: &infrav1.PublicIPSpec{
						Name: defaults.GenerateNodeOutboundIPName(s.ClusterName()),
					},
				},
			},
			Type:            infrav1.Public,
			SKU:             infrav1.SKUStandard,
			BackendPoolName: s.OutboundPoolName(s.NodeOutboundLBName()),
			Role:            infrav1.NodeOutboundRole,
		},
	}

	if !s.IsAPIServerPrivate() {
		return specs
	}

	specs = append(specs, azure.LBSpec{
		// Public Control Plane outbound LB
		Name: defaults.GenerateControlPlaneOutboundLBName(s.ClusterName()),
		FrontendIPConfigs: []infrav1.FrontendIP{
			{
				Name: defaults.GenerateFrontendIPConfigName(defaults.GenerateControlPlaneOutboundLBName(s.ClusterName())),
				PublicIP: &infrav1.PublicIPSpec{
					Name: defaults.GenerateControlPlaneOutboundIPName(s.ClusterName()),
				},
			},
		},
		Type:            infrav1.Public,
		SKU:             infrav1.SKUStandard,
		BackendPoolName: s.OutboundPoolName(defaults.GenerateControlPlaneOutboundLBName(s.ClusterName())),
		Role:            infrav1.ControlPlaneOutboundRole,
	})

	return specs
}

// RouteTableSpecs returns the node route table
func (s *ClusterScope) RouteTableSpecs() []azure.RouteTableSpec {
	routetables := []azure.RouteTableSpec{}
	if s.ControlPlaneRouteTable().Name != "" {
		routetables = append(routetables, azure.RouteTableSpec{Name: s.ControlPlaneRouteTable().Name, Subnet: s.ControlPlaneSubnet()})
	}
	if s.NodeRouteTable().Name != "" {
		routetables = append(routetables, azure.RouteTableSpec{Name: s.NodeRouteTable().Name, Subnet: s.NodeSubnet()})
	}
	return routetables
}

// NSGSpecs returns the security group specs.
func (s *ClusterScope) NSGSpecs() []azure.NSGSpec {
	return []azure.NSGSpec{
		{
			Name:         s.ControlPlaneSubnet().SecurityGroup.Name,
			IngressRules: s.ControlPlaneSubnet().SecurityGroup.IngressRules,
		},
		{
			Name:         s.NodeSubnet().SecurityGroup.Name,
			IngressRules: s.NodeSubnet().SecurityGroup.IngressRules,
		},
	}
}

// SubnetSpecs returns the subnets specs.
func (s *ClusterScope) SubnetSpecs() []azure.SubnetSpec {
	return []azure.SubnetSpec{
		{
			Name:              s.ControlPlaneSubnet().Name,
			CIDRs:             s.ControlPlaneSubnet().CIDRBlocks,
			VNetName:          s.Vnet().Name,
			SecurityGroupName: s.ControlPlaneSubnet().SecurityGroup.Name,
			Role:              s.ControlPlaneSubnet().Role,
			RouteTableName:    s.ControlPlaneSubnet().RouteTable.Name,
		},
		{
			Name:              s.NodeSubnet().Name,
			CIDRs:             s.NodeSubnet().CIDRBlocks,
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
			CIDRs:         s.Vnet().CIDRBlocks,
		},
	}
}

// ClusterName returns the cluster name.
func (s *ClusterScope) ClusterName() string {
	return s.Cluster.Name
}

// Namespace returns the cluster namespace.
func (s *ClusterScope) Namespace() string {
	return s.Cluster.Namespace
}

// ListOptionsLabelSelector returns a ListOptions with a label selector for clusterName.
func (s *ClusterScope) ListOptionsLabelSelector() client.ListOption {
	return client.MatchingLabels(map[string]string{
		clusterv1.ClusterLabelName: s.Cluster.Name,
	})
}

// PatchObject persists the cluster configuration and status.
func (s *ClusterScope) PatchObject(ctx context.Context) error {
	conditions.SetSummary(s.ClusterScoper,
		conditions.WithConditions(
			infrav1.NetworkInfrastructureReadyCondition,
		),
		conditions.WithStepCounterIfOnly(
			infrav1.NetworkInfrastructureReadyCondition,
		),
	)

	return s.patchHelper.Patch(
		ctx,
		s.ClusterScoper,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			infrav1.NetworkInfrastructureReadyCondition,
		}})
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ClusterScope) Close(ctx context.Context) error {
	return s.PatchObject(ctx)
}

// APIServerPort returns the APIServerPort to use when creating the load balancer.
func (s *ClusterScope) APIServerPort() int32 {
	if s.Cluster.Spec.ClusterNetwork != nil && s.Cluster.Spec.ClusterNetwork.APIServerPort != nil {
		return *s.Cluster.Spec.ClusterNetwork.APIServerPort
	}
	return 6443
}

// APIServerHost returns the APIServerHost used to reach the API server.
func (s *ClusterScope) APIServerHost() string {
	if s.IsAPIServerPrivate() {
		return s.APIServerPrivateIP()
	}
	return s.APIServerPublicIP().DNSName
}

// SetControlPlaneIngressRules will set the ingress rules or the control plane subnet
func (s *ClusterScope) SetControlPlaneIngressRules() {
	if s.ControlPlaneSubnet().SecurityGroup.IngressRules == nil {
		s.ControlPlaneSubnet().SecurityGroup.IngressRules = infrav1.IngressRules{
			&infrav1.IngressRule{
				Name:             "allow_ssh",
				Description:      "Allow SSH",
				Priority:         2200,
				Protocol:         infrav1.SecurityGroupProtocolTCP,
				Source:           to.StringPtr("*"),
				SourcePorts:      to.StringPtr("*"),
				Destination:      to.StringPtr("*"),
				DestinationPorts: to.StringPtr("22"),
			},
			&infrav1.IngressRule{
				Name:             "allow_apiserver",
				Description:      "Allow K8s API Server",
				Priority:         2201,
				Protocol:         infrav1.SecurityGroupProtocolTCP,
				Source:           to.StringPtr("*"),
				SourcePorts:      to.StringPtr("*"),
				Destination:      to.StringPtr("*"),
				DestinationPorts: to.StringPtr(strconv.Itoa(int(s.APIServerPort()))),
			},
		}
	}
}

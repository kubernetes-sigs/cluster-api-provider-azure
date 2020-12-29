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

	"github.com/Azure/go-autorest/autorest"
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
	AzureClients
	Client       client.Client
	Logger       logr.Logger
	Cluster      *clusterv1.Cluster
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

	/*err := params.AzureClients.setCredentials(params.AzureCluster.Spec.SubscriptionID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Azure session")
	}*/

	log := klogr.New()
	err := params.AzureClients.setDBECredentials(params.AzureCluster.Spec.SubscriptionID)
	if err != nil {
		log.Info(err.Error())
		return nil, errors.Wrap(err, "failed to create Azure session")
	}

	helper, err := patch.NewHelper(params.AzureCluster, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	return &ClusterScope{
		Logger:       params.Logger,
		Client:       params.Client,
		AzureClients: params.AzureClients,
		Cluster:      params.Cluster,
		AzureCluster: params.AzureCluster,
		patchHelper:  helper,
	}, nil
}

// ClusterScope defines the basic context for an actuator to operate upon.
type ClusterScope struct {
	logr.Logger
	Client      client.Client
	patchHelper *patch.Helper

	AzureClients
	Cluster      *clusterv1.Cluster
	AzureCluster *infrav1.AzureCluster
}

// SubscriptionID returns the Azure client Subscription ID.
func (s *ClusterScope) SubscriptionID() string {
	return s.AzureClients.SubscriptionID()
}

// BaseURI returns the Azure ResourceManagerEndpoint.
func (s *ClusterScope) BaseURI() string {
	return s.ResourceManagerEndpoint
}

// Authorizer returns the Azure client Authorizer.
func (s *ClusterScope) Authorizer() autorest.Authorizer {
	return s.AzureClients.Authorizer
}

// Network returns the cluster network object.
func (s *ClusterScope) Network() *infrav1.Network {
	return &s.AzureCluster.Status.Network
}

// NetworkSpec returns the cluster networkSpec object.
func (s *ClusterScope) NetworkSpec() *infrav1.NetworkSpec {
	return &s.AzureCluster.Spec.NetworkSpec
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
/*func (s *ClusterScope) RouteTableSpecs() []azure.RouteTableSpec {
	return []azure.RouteTableSpec{{
		Name: s.RouteTable().Name,
	}}
}*/

// NSGSpecs returns the security group specs.
/*func (s *ClusterScope) NSGSpecs() []azure.NSGSpec {
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
}*/

// SubnetSpecs returns the subnets specs.
func (s *ClusterScope) SubnetSpecs() []azure.SubnetSpec {
	return []azure.SubnetSpec{
		{
			Name:     s.ControlPlaneSubnet().Name,
			CIDR:     s.ControlPlaneSubnet().CidrBlock,
			VNetName: s.Vnet().Name,
			//SecurityGroupName:   s.ControlPlaneSubnet().SecurityGroup.Name,
			Role: s.ControlPlaneSubnet().Role,
			//RouteTableName:      s.ControlPlaneSubnet().RouteTable.Name,
			InternalLBIPAddress: s.ControlPlaneSubnet().InternalLBIPAddress,
		},
		{
			Name:     s.NodeSubnet().Name,
			CIDR:     s.NodeSubnet().CidrBlock,
			VNetName: s.Vnet().Name,
			//SecurityGroupName: s.NodeSubnet().SecurityGroup.Name,
			//RouteTableName:    s.NodeSubnet().RouteTable.Name,
			Role: s.NodeSubnet().Role,
		}}
}

/// VNetSpecs returns the virtual network specs.
func (s *ClusterScope) VNetSpecs() []azure.VNetSpec {
	return []azure.VNetSpec{
		{
			ResourceGroup: s.Vnet().ResourceGroup,
			Name:          s.Vnet().Name,
			CIDR:          s.Vnet().CidrBlock,
		},
	}
}

// Vnet returns the cluster Vnet.
func (s *ClusterScope) Vnet() *infrav1.VnetSpec {
	return &s.AzureCluster.Spec.NetworkSpec.Vnet
}

// IsVnetManaged returns true if the vnet is managed.
func (s *ClusterScope) IsVnetManaged() bool {
	return s.Vnet().ID == "" || s.Vnet().Tags.HasOwned(s.ClusterName())
}

// Subnets returns the cluster subnets.
func (s *ClusterScope) Subnets() infrav1.Subnets {
	return s.AzureCluster.Spec.NetworkSpec.Subnets
}

// ControlPlaneSubnet returns the cluster control plane subnet.
func (s *ClusterScope) ControlPlaneSubnet() *infrav1.SubnetSpec {
	return s.AzureCluster.Spec.NetworkSpec.GetControlPlaneSubnet()
}

// NodeSubnet returns the cluster node subnet.
func (s *ClusterScope) NodeSubnet() *infrav1.SubnetSpec {
	return s.AzureCluster.Spec.NetworkSpec.GetNodeSubnet()
}

// RouteTable returns the cluster node routetable.
func (s *ClusterScope) RouteTable() *infrav1.RouteTable {
	return &s.AzureCluster.Spec.NetworkSpec.GetNodeSubnet().RouteTable
}

// ResourceGroup returns the cluster resource group.
func (s *ClusterScope) ResourceGroup() string {
	return s.AzureCluster.Spec.ResourceGroup
}

// ClusterName returns the cluster name.
func (s *ClusterScope) ClusterName() string {
	return s.Cluster.Name
}

// Namespace returns the cluster namespace.
func (s *ClusterScope) Namespace() string {
	return s.Cluster.Namespace
}

// Location returns the cluster location.
func (s *ClusterScope) Location() string {
	return s.AzureCluster.Spec.Location
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
	return s.patchHelper.Patch(
		ctx,
		s.AzureCluster,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			infrav1.NetworkInfrastructureReadyCondition,
		}})
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ClusterScope) Close(ctx context.Context) error {
	return s.patchHelper.Patch(ctx, s.AzureCluster)
}

// AdditionalTags returns AdditionalTags from the scope's AzureCluster.
func (s *ClusterScope) AdditionalTags() infrav1.Tags {
	tags := make(infrav1.Tags)
	if s.AzureCluster.Spec.AdditionalTags != nil {
		tags = s.AzureCluster.Spec.AdditionalTags.DeepCopy()
	}
	return tags
}

// APIServerPort returns the APIServerPort to use when creating the load balancer.
func (s *ClusterScope) APIServerPort() int32 {
	if s.Cluster.Spec.ClusterNetwork != nil && s.Cluster.Spec.ClusterNetwork.APIServerPort != nil {
		return *s.Cluster.Spec.ClusterNetwork.APIServerPort
	}
	return 6443
}

// SetFailureDomain will set the spec for a for a given key
func (s *ClusterScope) SetFailureDomain(id string, spec clusterv1.FailureDomainSpec) {
	if s.AzureCluster.Status.FailureDomains == nil {
		s.AzureCluster.Status.FailureDomains = make(clusterv1.FailureDomains, 0)
	}
	s.AzureCluster.Status.FailureDomains[id] = spec
}

/*func (s *ClusterScope) SetControlPlaneIngressRules() {
	if s.ControlPlaneSubnet().SecurityGroup.IngressRules == nil {
		s.ControlPlaneSubnet().SecurityGroup.IngressRules = infrav1.IngressRules{
			&infrav1.IngressRule{
				Name:             "allow_ssh",
				Description:      "Allow SSH",
				Priority:         100,
				Protocol:         infrav1.SecurityGroupProtocolTCP,
				Source:           to.StringPtr("*"),
				SourcePorts:      to.StringPtr("*"),
				Destination:      to.StringPtr("*"),
				DestinationPorts: to.StringPtr("22"),
			},
			&infrav1.IngressRule{
				Name:             "allow_apiserver",
				Description:      "Allow K8s API Server",
				Priority:         101,
				Protocol:         infrav1.SecurityGroupProtocolTCP,
				Source:           to.StringPtr("*"),
				SourcePorts:      to.StringPtr("*"),
				Destination:      to.StringPtr("*"),
				DestinationPorts: to.StringPtr(strconv.Itoa(int(s.APIServerPort()))),
			},
		}
	}
}*/

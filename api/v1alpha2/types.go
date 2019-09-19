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

package v1alpha2

import (
	"fmt"
)

// AzureMachineTemplateResource describes the data needed to create am AzureMachine from a template
type AzureMachineTemplateResource struct {
	// Spec is the specification of the desired behavior of the machine.
	Spec AzureMachineSpec `json:"spec"`
}

// Filter is a filter used to identify an Azure resource
type Filter struct {
	// Name of the filter. Filter names are case-sensitive.
	Name string `json:"name"`

	// Values includes one or more filter values. Filter values are case-sensitive.
	Values []string `json:"values"`
}

// Network encapsulates Azure networking resources.
type Network struct {
	// FirewallRules is a map from the name of the rule to its full reference.
	// +optional
	FirewallRules map[string]string `json:"firewallRules,omitempty"`

	// APIServerAddress is the IPV4 global address assigned to the load balancer
	// created for the API Server.
	// +optional
	APIServerAddress *string `json:"apiServerIpAddress,omitempty"`

	// APIServerHealthCheck is the full reference to the health check
	// created for the API Server.
	// +optional
	APIServerHealthCheck *string `json:"apiServerHealthCheck,omitempty"`

	// APIServerInstanceGroups is a map from zone to the full reference
	// to the instance groups created for the control plane nodes created in the same zone.
	// +optional
	APIServerInstanceGroups map[string]string `json:"apiServerInstanceGroups,omitempty"`

	// APIServerBackendService is the full reference to the backend service
	// created for the API Server.
	// +optional
	APIServerBackendService *string `json:"apiServerBackendService,omitempty"`

	// APIServerTargetProxy is the full reference to the target proxy
	// created for the API Server.
	// +optional
	APIServerTargetProxy *string `json:"apiServerTargetProxy,omitempty"`

	// APIServerForwardingRule is the full reference to the forwarding rule
	// created for the API Server.
	// +optional
	APIServerForwardingRule *string `json:"apiServerForwardingRule,omitempty"`
}

// NetworkSpec encapsulates all things related to a Azure network.
type NetworkSpec struct {
	// Name is the name of the network to be used.
	// +optional
	Name string `json:"id,omitempty"`

	// Subnets configuration.
	// +optional
	Subnets Subnets `json:"subnets,omitempty"`
}

// APIEndpoint represents a reachable Kubernetes API endpoint.
type APIEndpoint struct {
	// The hostname on which the API server is serving.
	Host string `json:"host"`

	// The port on which the API server is serving.
	Port int `json:"port"`
}

// SubnetSpec configures an Azure Subnet.
type SubnetSpec struct {
	// Name defines a unique identifier to reference this resource.
	Name string `json:"name,omitempty"`

	// CidrBlock is the CIDR block assigned to this subnet.
	CidrBlock string `json:"cidrBlock,omitempty"`

	// Description is an optional description associated with the resource.
	// +optional
	Description *string `json:"description,omitempty"`

	// SecondaryCidrBlocks defines secondary CIDR ranges,
	// from which secondary IP ranges of a VM may be allocated
	// +optional
	SecondaryCidrBlocks map[string]string `json:"secondaryCidrBlocks,omitempty"`

	// Region defines the region to use for this subnet in the cluster's region.
	Region string `json:"region,omitempty"`

	// PrivateGoogleAccess defines whether VMs in this subnet can access
	// Google services without assigning external IP addresses
	// +optional
	PrivateGoogleAccess *bool `json:"privateGoogleAccess,omitempty"`

	// FlowLogs turns on the VPC flow logging.
	// +optional
	FlowLogs *bool `json:"routeTableId"`
}

// String returns a string representation of the subnet.
func (s *SubnetSpec) String() string {
	return fmt.Sprintf("name=%s/region=%s", s.Name, s.Region)
}

// Subnets is a slice of Subnet.
type Subnets []*SubnetSpec

// ToMap returns a map from name to subnet.
func (s Subnets) ToMap() map[string]*SubnetSpec {
	res := make(map[string]*SubnetSpec)
	for _, x := range s {
		res[x.Name] = x
	}
	return res
}

// FindByName returns a single subnet matching the given name or nil.
func (s Subnets) FindByName(name string) *SubnetSpec {
	for _, x := range s {
		if x.Name == name {
			return x
		}
	}

	return nil
}

// FilterByZone returns a slice containing all subnets that live in the specified region.
func (s Subnets) FilterByRegion(region string) (res Subnets) {
	for _, x := range s {
		if x.Region == region {
			res = append(res, x)
		}
	}
	return
}

// InstanceStatus describes the state of an Azure instance.
type InstanceStatus string

var (
	// InstanceStatusProvisioning is the string representing an instance in a provisioning state.
	InstanceStatusProvisioning = InstanceStatus("PROVISIONING")

	// InstanceStatusRepairing is the string representing an instance in a repairing state.
	InstanceStatusRepairing = InstanceStatus("REPAIRING")

	// InstanceStatusRunning is the string representing an instance in a pending state.
	InstanceStatusRunning = InstanceStatus("RUNNING")

	// InstanceStatusStaging is the string representing an instance in a staging state.
	InstanceStatusStaging = InstanceStatus("STAGING")

	// InstanceStatusStopped is the string representing an instance
	// that has been stopped and can be restarted.
	InstanceStatusStopped = InstanceStatus("STOPPED")

	// InstanceStatusStopping is the string representing an instance
	// that is in the process of being stopped and can be restarted.
	InstanceStatusStopping = InstanceStatus("STOPPING")

	// InstanceStatusSuspended is the string representing an instance
	// that is suspended.
	InstanceStatusSuspended = InstanceStatus("SUSPENDED")

	// InstanceStatusSuspending is the string representing an instance
	// that is in the process of being suspended
	InstanceStatusSuspending = InstanceStatus("SUSPENDING")

	// InstanceStatusTerminated is the string representing an instance that has been terminated.
	InstanceStatusTerminated = InstanceStatus("TERMINATED")
)

// ServiceAccount describes compute.serviceAccount
type ServiceAccount struct {
	// Email: Email address of the service account.
	Email string `json:"email,omitempty"`

	// Scopes: The list of scopes to be made available for this service
	// account.
	Scopes []string `json:"scopes,omitempty"`
}

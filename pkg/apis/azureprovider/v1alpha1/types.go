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

package v1alpha1

import (
	"fmt"
	"reflect"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AzureResourceReference is a reference to a specific Azure resource by ID
type AzureResourceReference struct {
	// ID of resource
	// +optional
	ID *string `json:"id,omitempty"`
	// TODO: Investigate if we should reference resources in other ways
}

// TODO: Investigate resource filters

// AzureMachineProviderConditionType is a valid value for AzureMachineProviderCondition.Type
type AzureMachineProviderConditionType string

// Valid conditions for an Azure machine instance
const (
	// MachineCreated indicates whether the machine has been created or not. If not,
	// it should include a reason and message for the failure.
	MachineCreated AzureMachineProviderConditionType = "MachineCreated"
)

// AzureMachineProviderCondition is a condition in a AzureMachineProviderStatus
type AzureMachineProviderCondition struct {
	// Type is the type of the condition.
	Type AzureMachineProviderConditionType `json:"type"`
	// Status is the status of the condition.
	Status corev1.ConditionStatus `json:"status"`
	// LastProbeTime is the last time we probed the condition.
	// +optional
	LastProbeTime metav1.Time `json:"lastProbeTime"`
	// LastTransitionTime is the last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`
	// Reason is a unique, one-word, CamelCase reason for the condition's last transition.
	// +optional
	Reason string `json:"reason"`
	// Message is a human-readable message indicating details about last transition.
	// +optional
	Message string `json:"message"`
}

// TODO
// Network encapsulates Azure networking resources.
type Network struct {
	// Vnet defines the cluster vnet.
	Vnet Vnet `json:"vnet,omitempty"`

	// InternetGatewayID is the id of the internet gateway associated with the Vnet.
	InternetGatewayID *string `json:"internetGatewayId,omitempty"`

	// SecurityGroups is a map from the role/kind of the security group to its unique name, if any.
	SecurityGroups map[SecurityGroupRole]*SecurityGroup `json:"securityGroups,omitempty"`

	// Subnets includes all the subnets defined inside the Vnet.
	Subnets Subnets `json:"subnets,omitempty"`

	// APIServerLB is the Kubernetes api server load balancer.
	APIServerLB LoadBalancer `json:"apiServerLb,omitempty"`
}

// Vnet defines an Azure Virtual Network.
type Vnet struct {
	ID        string             `json:"id"`
	CidrBlock string             `json:"cidrBlock"`
	Name      *string            `json:"name,omitempty"`
	Tags      map[string]*string `json:"tags"`
}

// TODO: Do we need this?
// String returns a string representation of the Vnet.
func (v *Vnet) String() string {
	return fmt.Sprintf("id=%s", v.ID)
}

// Subnet defines an Azure subnet attached to a Vnet.
type Subnet struct {
	ID string `json:"id"`

	VnetID       string  `json:"vnetId"`
	CidrBlock    string  `json:"cidrBlock"`
	NSGID        string  `json:"nsgId"`
	RouteTableID *string `json:"routeTableId"`
}

/*
// TODO
// String returns a string representation of the subnet.
func (s *Subnet) String() string {
	return fmt.Sprintf("id=%s/az=%s/public=%v", s.ID, s.AvailabilityZone, s.IsPublic)
}
*/

// TODO
// LoadBalancerScheme defines the scheme of a load balancer.
type LoadBalancerScheme string

// TODO
var (
	// LoadBalancerSchemeInternetFacing defines an internet-facing, publicly
	// accessible Azure LB scheme
	LoadBalancerSchemeInternetFacing = LoadBalancerScheme("Internet-facing")

	// LoadBalancerSchemeInternal defines an internal-only facing
	// load balancer internal to a LB.
	LoadBalancerSchemeInternal = LoadBalancerScheme("internal")
)

// TODO
// LoadBalancerProtocol defines listener protocols for a load balancer.
type LoadBalancerProtocol string

// TODO
var (
	// LoadBalancerProtocolTCP defines the LB API string representing the TCP protocol
	LoadBalancerProtocolTCP = LoadBalancerProtocol("TCP")

	// LoadBalancerProtocolSSL defines the LB API string representing the TLS protocol
	LoadBalancerProtocolSSL = LoadBalancerProtocol("SSL")

	// LoadBalancerProtocolHTTP defines the LB API string representing the HTTP protocol at L7
	LoadBalancerProtocolHTTP = LoadBalancerProtocol("HTTP")

	// LoadBalancerProtocolHTTPS defines the LB API string representing the HTTP protocol at L7
	LoadBalancerProtocolHTTPS = LoadBalancerProtocol("HTTPS")
)

// TODO
// LoadBalancer defines an Azure load balancer.
type LoadBalancer struct {
	// The name of the load balancer. It must be unique within the set of load balancers
	// defined in the location. It also serves as identifier.
	Name string `json:"name,omitempty"`

	// DNSName is the dns name of the load balancer.
	DNSName string `json:"dnsName,omitempty"`

	// IPAddress is the IP address of the load balancer.
	IPAddress string `json:"ipAddress,omitempty"`

	// Scheme is the load balancer scheme, either internet-facing or private.
	Scheme LoadBalancerScheme `json:"scheme,omitempty"`

	// SubnetIDs is an array of subnets in the Vnet attached to the load balancer.
	SubnetIDs []string `json:"subnetIds,omitempty"`

	// SecurityGroupIDs is an array of security groups assigned to the load balancer.
	SecurityGroupIDs []string `json:"securityGroupIds,omitempty"`

	// Listeners is an array of elb listeners associated with the load balancer. There must be at least one.
	Listeners []*LoadBalancerListener `json:"listeners,omitempty"`

	// HealthCheck is the elb health check associated with the load balancer.
	HealthCheck *LoadBalancerHealthCheck `json:"healthChecks,omitempty"`

	// Tags is a map of tags associated with the load balancer.
	Tags map[string]string `json:"tags,omitempty"`
}

// TODO
// LoadBalancerListener defines an Azure load balancer listener.
type LoadBalancerListener struct {
	Protocol         LoadBalancerProtocol `json:"protocol"`
	Port             int64                `json:"port"`
	InstanceProtocol LoadBalancerProtocol `json:"instanceProtocol"`
	InstancePort     int64                `json:"instancePort"`
}

// TODO
// LoadBalancerHealthCheck defines an Azure load balancer health check.
type LoadBalancerHealthCheck struct {
	Target             string        `json:"target"`
	Interval           time.Duration `json:"interval"`
	Timeout            time.Duration `json:"timeout"`
	HealthyThreshold   int64         `json:"healthyThreshold"`
	UnhealthyThreshold int64         `json:"unhealthyThreshold"`
}

// TODO
// Subnets is a slice of Subnet.
type Subnets []*Subnet

// TODO
// ToMap returns a map from id to subnet.
func (s Subnets) ToMap() map[string]*Subnet {
	res := make(map[string]*Subnet)
	for _, x := range s {
		res[x.ID] = x
	}
	return res
}

// RouteTable defines an Azure routing table.
type RouteTable struct {
	ID string `json:"id"`
}

// SecurityGroupRole defines the unique role of a security group.
type SecurityGroupRole string

var (
	// SecurityGroupBastion defines an SSH bastion role
	SecurityGroupBastion = SecurityGroupRole("bastion")

	// SecurityGroupNode defines a Kubernetes workload node role
	SecurityGroupNode = SecurityGroupRole("node")

	// SecurityGroupControlPlane defines a Kubernetes control plane node role
	SecurityGroupControlPlane = SecurityGroupRole("controlplane")
)

// TODO
// SecurityGroup defines an Azure security group.
type SecurityGroup struct {
	ID   string `json:"id"`
	Name string `json:"name"`

	IngressRules IngressRules `json:"ingressRule"`
}

/*
// TODO
// String returns a string representation of the security group.
func (s *SecurityGroup) String() string {
	return fmt.Sprintf("id=%s/name=%s", s.ID, s.Name)
}
*/

// TODO
// SecurityGroupProtocol defines the protocol type for a security group rule.
type SecurityGroupProtocol string

// TODO
var (
	// SecurityGroupProtocolAll is a wildcard for all IP protocols
	SecurityGroupProtocolAll = SecurityGroupProtocol("*")

	// SecurityGroupProtocolTCP represents the TCP protocol in ingress rules
	SecurityGroupProtocolTCP = SecurityGroupProtocol("tcp")

	// SecurityGroupProtocolUDP represents the UDP protocol in ingress rules
	SecurityGroupProtocolUDP = SecurityGroupProtocol("udp")
)

// TODO
// IngressRule defines an Azure ingress rule for security groups.
type IngressRule struct {
	Description string                `json:"description"`
	Protocol    SecurityGroupProtocol `json:"protocol"`
	FromPort    int64                 `json:"fromPort"`
	ToPort      int64                 `json:"toPort"`

	// List of CIDR blocks to allow access from. Cannot be specified with SourceSecurityGroupID.
	CidrBlocks []string `json:"cidrBlocks"`

	// The security group id to allow access from. Cannot be specified with CidrBlocks.
	SourceSecurityGroupIDs []string `json:"sourceSecurityGroupIds"`
}

// TODO
// String returns a string representation of the ingress rule.
func (i *IngressRule) String() string {
	return fmt.Sprintf("protocol=%s/range=[%d-%d]/description=%s", i.Protocol, i.FromPort, i.ToPort, i.Description)
}

// TODO
// IngressRules is a slice of Azure ingress rules for security groups.
type IngressRules []*IngressRule

// TODO
// Difference returns the difference between this slice and the other slice.
func (i IngressRules) Difference(o IngressRules) (out IngressRules) {
	for _, x := range i {
		found := false
		for _, y := range o {
			sort.Strings(x.CidrBlocks)
			sort.Strings(y.CidrBlocks)
			sort.Strings(x.SourceSecurityGroupIDs)
			sort.Strings(y.SourceSecurityGroupIDs)
			if reflect.DeepEqual(x, y) {
				found = true
				break
			}
		}

		if !found {
			out = append(out, x)
		}
	}

	return
}

// TODO
// InstanceState describes the state of an Azure instance.
type InstanceState string

// TODO
var (
	// InstanceStatePending is the string representing an instance in a pending state
	InstanceStatePending = InstanceState("pending")

	// InstanceStateRunning is the string representing an instance in a pending state
	InstanceStateRunning = InstanceState("running")

	// InstanceStateShuttingDown is the string representing an instance shutting down
	InstanceStateShuttingDown = InstanceState("shutting-down")

	// InstanceStateTerminated is the string representing an instance that has been terminated
	InstanceStateTerminated = InstanceState("terminated")

	// InstanceStateStopping is the string representing an instance
	// that is in the process of being stopped and can be restarted
	InstanceStateStopping = InstanceState("stopping")

	// InstanceStateStopped is the string representing an instance
	// that has been stopped and can be restarted
	InstanceStateStopped = InstanceState("stopped")
)

// TODO
// Instance describes an Azure instance.
type Instance struct {
	ID string `json:"id"`

	// The current state of the instance.
	State InstanceState `json:"instanceState,omitempty"`

	// The instance type.
	Type string `json:"type,omitempty"`

	// The ID of the subnet of the instance.
	SubnetID string `json:"subnetId,omitempty"`

	// The ID of the AMI used to launch the instance.
	ImageID string `json:"imageId,omitempty"`

	// The name of the SSH key pair.
	KeyName *string `json:"keyName,omitempty"`

	// SecurityGroupIDs are one or more security group IDs this instance belongs to.
	SecurityGroupIDs []string `json:"securityGroupIds,omitempty"`

	// UserData is the raw data script passed to the instance which is run upon bootstrap.
	// This field must not be base64 encoded and should only be used when running a new instance.
	UserData *string `json:"userData,omitempty"`

	// The name of the IAM instance profile associated with the instance, if applicable.
	IAMProfile string `json:"iamProfile,omitempty"`

	// The private IPv4 address assigned to the instance.
	PrivateIP *string `json:"privateIp,omitempty"`

	// The public IPv4 address assigned to the instance, if applicable.
	PublicIP *string `json:"publicIp,omitempty"`

	// Specifies whether enhanced networking with ENA is enabled.
	ENASupport *bool `json:"enaSupport,omitempty"`

	// Indicates whether the instance is optimized for Amazon EBS I/O.
	EBSOptimized *bool `json:"ebsOptimized,omitempty"`

	// The tags associated with the instance.
	Tags map[string]string `json:"tags,omitempty"`
}

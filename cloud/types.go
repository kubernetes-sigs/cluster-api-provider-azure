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

package azure

import (
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
)

// PublicIPSpec defines the specification for a Public IP.
type PublicIPSpec struct {
	Name    string
	DNSName string
}

// NICSpec defines the specification for a Network Interface.
type NICSpec struct {
	Name                     string
	MachineName              string
	MachineRole              string
	SubnetName               string
	VNetName                 string
	VNetResourceGroup        string
	StaticIPAddress          string
	PublicLoadBalancerName   string
	InternalLoadBalancerName string
	PublicIPName             string
	VMSize                   string
	AcceleratedNetworking    *bool
}

// DiskSpec defines the specification for a Disk.
type DiskSpec struct {
	Name string
}

// LBSpec defines the specification for a Load Balancer.
type LBSpec struct {
	Name             string
	PublicIPName     string
	Role             string
	SubnetName       string
	SubnetCidr       string
	PrivateIPAddress string
	APIServerPort    int32
}

// RouteTableSpec defines the specification for a route table
type RouteTableSpec struct {
	Name string
}

// InboundNatSpec defines the specification for an inbound NAT rule.
type InboundNatSpec struct {
	Name             string
	LoadBalancerName string
}

// SubnetSpec defines the specification for a Subnet.
type SubnetSpec struct {
	Name                string
	CIDR                string
	VNetName            string
	RouteTableName      string
	SecurityGroupName   string
	Role                infrav1.SubnetRole
	InternalLBIPAddress string
}

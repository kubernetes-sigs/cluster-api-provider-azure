/*
Copyright 2019 The Kubernetes Authors.

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

package internalloadbalancers

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-12-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/subnets"
)

// Spec specification for internal load balancer
type Spec struct {
	Name       string
	SubnetName string
	VnetName   string
	IPAddress  string
}

// Get provides information about a route table.
func (s *Service) Get(ctx context.Context, spec azure.Spec) (interface{}, error) {
	internalLBSpec, ok := spec.(*Spec)
	if !ok {
		return network.LoadBalancer{}, errors.New("invalid internal load balancer specification")
	}
	//lbName := fmt.Sprintf("%s-api-internallb", s.Scope.Cluster.Name)
	lb, err := s.Client.Get(ctx, s.Scope.ClusterConfig.ResourceGroup, internalLBSpec.Name, "")
	if err != nil && azure.ResourceNotFound(err) {
		return nil, errors.Wrapf(err, "load balancer %s not found", internalLBSpec.Name)
	} else if err != nil {
		return lb, err
	}
	return lb, nil
}

// CreateOrUpdate creates or updates a route table.
func (s *Service) CreateOrUpdate(ctx context.Context, spec azure.Spec) error {
	internalLBSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid internal load balancer specification")
	}
	klog.V(2).Infof("creating internal load balancer %s", internalLBSpec.Name)
	probeName := "tcpHTTPSProbe"
	frontEndIPConfigName := "controlplane-internal-lbFrontEnd"
	backEndAddressPoolName := "controlplane-internal-backEndPool"
	idPrefix := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers", s.Scope.SubscriptionID, s.Scope.ClusterConfig.ResourceGroup)
	lbName := internalLBSpec.Name

	klog.V(2).Infof("getting subnet %s", internalLBSpec.SubnetName)
	subnetInterface, err := subnets.NewService(s.Scope).Get(ctx, &subnets.Spec{Name: internalLBSpec.SubnetName, VnetName: internalLBSpec.VnetName})
	if err != nil {
		return err
	}

	subnet, ok := subnetInterface.(network.Subnet)
	if !ok {
		return errors.New("subnet Get returned invalid interface")
	}
	klog.V(2).Infof("successfully got subnet %s", internalLBSpec.SubnetName)

	// https://docs.microsoft.com/en-us/azure/load-balancer/load-balancer-standard-availability-zones#zone-redundant-by-default
	future, err := s.Client.CreateOrUpdate(ctx,
		s.Scope.ClusterConfig.ResourceGroup,
		lbName,
		network.LoadBalancer{
			Sku:      &network.LoadBalancerSku{Name: network.LoadBalancerSkuNameStandard},
			Location: to.StringPtr(s.Scope.ClusterConfig.Location),
			LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
				FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
					{
						Name: &frontEndIPConfigName,
						FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
							PrivateIPAllocationMethod: network.Static,
							Subnet:                    &subnet,
							PrivateIPAddress:          to.StringPtr(internalLBSpec.IPAddress),
						},
					},
				},
				BackendAddressPools: &[]network.BackendAddressPool{
					{
						Name: &backEndAddressPoolName,
					},
				},
				Probes: &[]network.Probe{
					{
						Name: &probeName,
						ProbePropertiesFormat: &network.ProbePropertiesFormat{
							Protocol:          network.ProbeProtocolTCP,
							Port:              to.Int32Ptr(6443),
							IntervalInSeconds: to.Int32Ptr(15),
							NumberOfProbes:    to.Int32Ptr(4),
						},
					},
				},
				LoadBalancingRules: &[]network.LoadBalancingRule{
					{
						Name: to.StringPtr("LBRuleHTTPS"),
						LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
							Protocol:             network.TransportProtocolTCP,
							FrontendPort:         to.Int32Ptr(6443),
							BackendPort:          to.Int32Ptr(6443),
							IdleTimeoutInMinutes: to.Int32Ptr(4),
							EnableFloatingIP:     to.BoolPtr(false),
							LoadDistribution:     network.LoadDistributionDefault,
							FrontendIPConfiguration: &network.SubResource{
								ID: to.StringPtr(fmt.Sprintf("/%s/%s/frontendIPConfigurations/%s", idPrefix, lbName, frontEndIPConfigName)),
							},
							BackendAddressPool: &network.SubResource{
								ID: to.StringPtr(fmt.Sprintf("/%s/%s/backendAddressPools/%s", idPrefix, lbName, backEndAddressPoolName)),
							},
							Probe: &network.SubResource{
								ID: to.StringPtr(fmt.Sprintf("/%s/%s/probes/%s", idPrefix, lbName, probeName)),
							},
						},
					},
				},
			},
		})

	if err != nil {
		return errors.Wrap(err, "cannot create load balancer")
	}

	err = future.WaitForCompletionRef(ctx, s.Client.Client)
	if err != nil {
		return errors.Wrap(err, "cannot get internal load balancer create or update future response")
	}

	_, err = future.Result(s.Client)
	klog.V(2).Infof("successfully created internal load balancer %s", internalLBSpec.Name)
	return err
}

// Delete deletes the route table with the provided name.
func (s *Service) Delete(ctx context.Context, spec azure.Spec) error {
	internalLBSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid internal load balancer specification")
	}
	klog.V(2).Infof("deleting internal load balancer %s", internalLBSpec.Name)
	f, err := s.Client.Delete(ctx, s.Scope.ClusterConfig.ResourceGroup, internalLBSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "failed to delete internal load balancer %s in resource group %s", internalLBSpec.Name, s.Scope.ClusterConfig.ResourceGroup)
	}

	err = f.WaitForCompletionRef(ctx, s.Client.Client)
	if err != nil {
		return errors.Wrap(err, "cannot create, future response")
	}

	_, err = f.Result(s.Client)
	if err != nil {
		return errors.Wrap(err, "result error")
	}
	klog.V(2).Infof("successfully deleted internal load balancer %s", internalLBSpec.Name)
	return err
}

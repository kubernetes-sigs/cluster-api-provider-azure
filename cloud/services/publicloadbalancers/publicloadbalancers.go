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

package publicloadbalancers

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha2"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
)

// Spec specification for public load balancer
type Spec struct {
	Name         string
	PublicIPName string
}

// Get provides information about a public load balancer.
func (s *Service) Get(ctx context.Context, spec interface{}) (interface{}, error) {
	publicLBSpec, ok := spec.(*Spec)
	if !ok {
		return network.LoadBalancer{}, errors.New("invalid public loadbalancer specification")
	}
	lb, err := s.Client.Get(ctx, s.Scope.ResourceGroup(), publicLBSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		return nil, errors.Wrapf(err, "load balancer %s not found", publicLBSpec.Name)
	} else if err != nil {
		return lb, err
	}
	return lb, nil
}

// Reconcile gets/creates/updates a public load balancer.
func (s *Service) Reconcile(ctx context.Context, spec interface{}) error {
	publicLBSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid public loadbalancer specification")
	}
	probeName := "tcpHTTPSProbe"
	frontEndIPConfigName := "controlplane-lbFrontEnd"
	backEndAddressPoolName := "controlplane-backEndPool"
	idPrefix := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers", s.Scope.SubscriptionID, s.Scope.ResourceGroup())
	lbName := publicLBSpec.Name
	klog.V(2).Infof("creating public load balancer %s", lbName)

	klog.V(2).Infof("getting public ip %s", publicLBSpec.PublicIPName)
	publicIP, err := s.PublicIPsClient.Get(ctx, s.Scope.ResourceGroup(), publicLBSpec.PublicIPName)
	if err != nil {
		return err
	}

	klog.V(2).Infof("successfully got public ip %s", publicLBSpec.PublicIPName)

	// https://docs.microsoft.com/en-us/azure/load-balancer/load-balancer-standard-availability-zones#zone-redundant-by-default
	err = s.Client.CreateOrUpdate(ctx,
		s.Scope.ResourceGroup(),
		lbName,
		network.LoadBalancer{
			Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
				ClusterName: s.Scope.Name(),
				Lifecycle:   infrav1.ResourceLifecycleOwned,
				Role:        to.StringPtr(infrav1.APIServerRoleTagValue),
				Additional:  s.Scope.AdditionalTags(),
			})),
			Sku:      &network.LoadBalancerSku{Name: network.LoadBalancerSkuNameStandard},
			Location: to.StringPtr(s.Scope.Location()),
			LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
				FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
					{
						Name: &frontEndIPConfigName,
						FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
							PrivateIPAllocationMethod: network.Dynamic,
							PublicIPAddress:           &publicIP,
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
							Port:              to.Int32Ptr(s.Scope.APIServerPort()),
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
							FrontendPort:         to.Int32Ptr(s.Scope.APIServerPort()),
							BackendPort:          to.Int32Ptr(s.Scope.APIServerPort()),
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
				InboundNatRules: &[]network.InboundNatRule{
					{
						Name: to.StringPtr("natRule1"),
						InboundNatRulePropertiesFormat: &network.InboundNatRulePropertiesFormat{
							Protocol:             network.TransportProtocolTCP,
							FrontendPort:         to.Int32Ptr(22),
							BackendPort:          to.Int32Ptr(22),
							EnableFloatingIP:     to.BoolPtr(false),
							IdleTimeoutInMinutes: to.Int32Ptr(4),
							FrontendIPConfiguration: &network.SubResource{
								ID: to.StringPtr(fmt.Sprintf("/%s/%s/frontendIPConfigurations/%s", idPrefix, lbName, frontEndIPConfigName)),
							},
						},
					},
					{
						Name: to.StringPtr("natRule2"),
						InboundNatRulePropertiesFormat: &network.InboundNatRulePropertiesFormat{
							Protocol:             network.TransportProtocolTCP,
							FrontendPort:         to.Int32Ptr(2201),
							BackendPort:          to.Int32Ptr(22),
							EnableFloatingIP:     to.BoolPtr(false),
							IdleTimeoutInMinutes: to.Int32Ptr(4),
							FrontendIPConfiguration: &network.SubResource{
								ID: to.StringPtr(fmt.Sprintf("/%s/%s/frontendIPConfigurations/%s", idPrefix, lbName, frontEndIPConfigName)),
							},
						},
					},
					{
						Name: to.StringPtr("natRule3"),
						InboundNatRulePropertiesFormat: &network.InboundNatRulePropertiesFormat{
							Protocol:             network.TransportProtocolTCP,
							FrontendPort:         to.Int32Ptr(2202),
							BackendPort:          to.Int32Ptr(22),
							EnableFloatingIP:     to.BoolPtr(false),
							IdleTimeoutInMinutes: to.Int32Ptr(4),
							FrontendIPConfiguration: &network.SubResource{
								ID: to.StringPtr(fmt.Sprintf("/%s/%s/frontendIPConfigurations/%s", idPrefix, lbName, frontEndIPConfigName)),
							},
						},
					},
				},
			},
		})

	if err != nil {
		return errors.Wrap(err, "cannot create public load balancer")
	}

	klog.V(2).Infof("successfully created public load balancer %s", lbName)
	return nil
}

// Delete deletes the public load balancer with the provided name.
func (s *Service) Delete(ctx context.Context, spec interface{}) error {
	publicLBSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid public loadbalancer specification")
	}
	klog.V(2).Infof("deleting public load balancer %s", publicLBSpec.Name)
	err := s.Client.Delete(ctx, s.Scope.ResourceGroup(), publicLBSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "failed to delete public load balancer %s in resource group %s", publicLBSpec.Name, s.Scope.ResourceGroup())
	}

	klog.V(2).Infof("deleted public load balancer %s", publicLBSpec.Name)
	return nil
}

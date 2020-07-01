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
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
)

// Spec specification for public load balancer
type Spec struct {
	Name         string
	PublicIPName string
	Role         string
}

// Reconcile gets/creates/updates a public load balancer.
func (s *Service) Reconcile(ctx context.Context, spec interface{}) error {
	publicLBSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid public loadbalancer specification")
	}
	lbName := publicLBSpec.Name
	frontEndIPConfigName := fmt.Sprintf("%s-%s", publicLBSpec.Name, "frontEnd")
	backEndAddressPoolName := fmt.Sprintf("%s-%s", publicLBSpec.Name, "backendPool")
	if publicLBSpec.Role == infrav1.NodeOutboundRole {
		backEndAddressPoolName = fmt.Sprintf("%s-%s", publicLBSpec.Name, "outboundBackendPool")
	}
	idPrefix := fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Network/loadBalancers", s.Scope.SubscriptionID(), s.Scope.ResourceGroup())

	s.Scope.Logger.V(2).Info("creating public load balancer", "load balancer", lbName)

	s.Scope.Logger.V(2).Info("getting public ip", "public ip", publicLBSpec.PublicIPName)
	publicIP, err := s.PublicIPsClient.Get(ctx, s.Scope.ResourceGroup(), publicLBSpec.PublicIPName)
	if err != nil && azure.ResourceNotFound(err) {
		return errors.Wrap(err, fmt.Sprintf("public ip %s not found in RG %s", publicLBSpec.PublicIPName, s.Scope.ResourceGroup()))
	} else if err != nil {
		return errors.Wrap(err, "failed to look for existing public IP")
	}
	s.Scope.Logger.V(2).Info("successfully got public ip", "public ip", publicLBSpec.PublicIPName)

	lb := network.LoadBalancer{
		Sku:      &network.LoadBalancerSku{Name: network.LoadBalancerSkuNameStandard},
		Location: to.StringPtr(s.Scope.Location()),
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: s.Scope.ClusterName(),
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Role:        to.StringPtr(publicLBSpec.Role),
			Additional:  s.Scope.AdditionalTags(),
		})),
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
			OutboundRules: &[]network.OutboundRule{
				{
					Name: to.StringPtr("OutboundNATAllProtocols"),
					OutboundRulePropertiesFormat: &network.OutboundRulePropertiesFormat{
						Protocol:             network.LoadBalancerOutboundRuleProtocolAll,
						IdleTimeoutInMinutes: to.Int32Ptr(4),
						FrontendIPConfigurations: &[]network.SubResource{
							{
								ID: to.StringPtr(fmt.Sprintf("/%s/%s/frontendIPConfigurations/%s", idPrefix, lbName, frontEndIPConfigName)),
							},
						},
						BackendAddressPool: &network.SubResource{
							ID: to.StringPtr(fmt.Sprintf("/%s/%s/backendAddressPools/%s", idPrefix, lbName, backEndAddressPoolName)),
						},
					},
				},
			},
		},
	}

	if publicLBSpec.Role == infrav1.APIServerRole {
		probeName := "tcpHTTPSProbe"
		lb.LoadBalancerPropertiesFormat.Probes = &[]network.Probe{
			{
				Name: to.StringPtr(probeName),
				ProbePropertiesFormat: &network.ProbePropertiesFormat{
					Protocol:          network.ProbeProtocolTCP,
					Port:              to.Int32Ptr(s.Scope.APIServerPort()),
					IntervalInSeconds: to.Int32Ptr(15),
					NumberOfProbes:    to.Int32Ptr(4),
				},
			},
		}
		// We disable outbound SNAT explicitly in the HTTPS LB rule and enable TCP and UDP outbound NAT with an outbound rule.
		// For more information on Standard LB outbound connections see https://docs.microsoft.com/en-us/azure/load-balancer/load-balancer-outbound-connections.
		lb.LoadBalancerPropertiesFormat.LoadBalancingRules = &[]network.LoadBalancingRule{
			{
				Name: to.StringPtr("LBRuleHTTPS"),
				LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
					DisableOutboundSnat:  to.BoolPtr(true),
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
		}
	}

	err = s.Client.CreateOrUpdate(ctx, s.Scope.ResourceGroup(), lbName, lb)

	if err != nil {
		return errors.Wrap(err, "cannot create public load balancer")
	}

	s.Scope.Logger.V(2).Info("successfully created public load balancer", "load balancer", lbName)
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

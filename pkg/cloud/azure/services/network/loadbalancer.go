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

package network

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-11-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
)

func (s *Service) ReconcileLoadBalancer(role string) error {
	klog.V(2).Info("Reconciling load balancer")

	// Get default LB spec.
	klog.V(2).Info("Getting load balancer spec")
	spec, err := s.getLBSpec(role)
	if err != nil {
		klog.V(2).Info("Unable to get load balancer spec")
		return err
	}

	// Create or get a public IP
	klog.V(2).Info("Getting or creating a public IP for load balancer")
	pip, err := s.CreateOrUpdatePublicIPAddress(s.scope.ClusterConfig.ResourceGroup, to.String(s.setLBName(role)))
	if err != nil {
		return errors.Errorf("Public IP get/create was unsuccessful: %s", err)
	}
	klog.V(2).Info("Successfully retrieved a public IP for load balancer")

	// TODO: Use deepcopy function to copy public IP attributes to Network object
	s.scope.Network().APIServerIP.ID = *pip.ID
	s.scope.Network().APIServerIP.Name = *pip.Name
	s.scope.Network().APIServerIP.IPAddress = *pip.IPAddress
	s.scope.Network().APIServerIP.DNSName = *pip.DNSSettings.Fqdn
	klog.V(2).Infof("APIServerIP.DNSName stored as %s", s.scope.Network().APIServerIP.DNSName)

	klog.V(2).Info("Building frontend IP for load balancer")
	frontendIPConfigs := []network.FrontendIPConfiguration{
		{
			Name: spec.Name,
			FrontendIPConfigurationPropertiesFormat: &network.FrontendIPConfigurationPropertiesFormat{
				PrivateIPAllocationMethod: network.Dynamic,
				PublicIPAddress:           &pip,
			},
		},
	}

	// Describe or create a load balancer.
	apiLB, err := s.describeLB(to.String(spec.Name))
	if err != nil {
		klog.V(2).Info("Unable to find existing load balancer.")
		spec.FrontendIPConfigurations = &frontendIPConfigs

		apiLB, err := s.createLB(*spec, role)
		if err != nil {
			return err
		}

		klog.V(2).Infof("Created new load balancer for %s: %v", role, apiLB)
	}

	apiLB, err = s.reconcileLBRules(&apiLB)
	if err != nil {
		return err
	}

	// TODO: Use deepcopy function to copy LB attributes to Network object
	s.scope.Network().APIServerLB.ID = *apiLB.ID
	s.scope.Network().APIServerLB.Name = *apiLB.Name
	s.scope.Network().APIServerLB.SKU = v1alpha1.SKU(apiLB.Sku.Name)

	backendPools := *apiLB.BackendAddressPools
	s.scope.Network().APIServerLB.BackendPool.ID = *backendPools[0].ID
	s.scope.Network().APIServerLB.BackendPool.Name = *backendPools[0].Name

	klog.V(2).Infof("Control plane load balancer: %v", apiLB)
	klog.V(2).Info("Reconcile load balancers completed successfully")

	return nil
}

/*
func (s *Service) DeleteLoadBalancer() error {

}
*/

func (s *Service) getLBSpec(role string) (*network.LoadBalancer, error) {
	switch role {
	case "api":
		return s.getAPILBSpec(), nil
	// TODO: Uncomment case once getServiceLBSpec exists
	/*
		case "service":
			return s.getServiceLBSpec(), nil
	*/
	default:
		return nil, errors.Errorf("No load balancer spec exists for %s", role)
	}
}

func (s *Service) getAPILBSpec() *network.LoadBalancer {
	lbName := s.setLBName("api")

	res := &network.LoadBalancer{
		Name:     lbName,
		Location: &s.scope.ClusterConfig.Location,
		Sku: &network.LoadBalancerSku{
			Name: network.LoadBalancerSkuNameStandard,
		},
		LoadBalancerPropertiesFormat: &network.LoadBalancerPropertiesFormat{
			// TODO: Remove debug
			/*
				FrontendIPConfigurations: &[]network.FrontendIPConfiguration{
					{
						Name: lbName,
					},
				},
			*/
			BackendAddressPools: &[]network.BackendAddressPool{
				{
					Name: lbName,
				},
			},
			Probes: &[]network.Probe{
				{
					Name: lbName,
					ProbePropertiesFormat: &network.ProbePropertiesFormat{
						Protocol:          network.ProbeProtocolTCP,
						Port:              to.Int32Ptr(6443),
						IntervalInSeconds: to.Int32Ptr(5),
						NumberOfProbes:    to.Int32Ptr(2),
					},
				},
			},
		},
		//Tags:
	}

	// TODO: Needs converter method
	return res
}

// TODO: Add getServiceLBSpec
/*
func (s *Service) getServiceLBSpec() *v1alpha1.LoadBalancer {
}
*/

func (s *Service) describeLB(lbName string) (network.LoadBalancer, error) {
	klog.V(2).Info("Attempting to find existing load balancer")
	return s.scope.LB.Get(s.scope.Context, s.scope.ClusterConfig.ResourceGroup, lbName, "")
}

func (s *Service) createLB(lbSpec network.LoadBalancer, role string) (lb network.LoadBalancer, err error) {
	klog.V(2).Info("Creating a new load balancer.")
	future, err := s.scope.LB.CreateOrUpdate(s.scope.Context, s.scope.ClusterConfig.ResourceGroup, *lbSpec.Name, lbSpec)

	if err != nil {
		return lb, err
	}

	err = future.WaitForCompletionRef(s.scope.Context, s.scope.LB.Client)
	if err != nil {
		return lb, fmt.Errorf("cannot get load balancer create or update future response: %v", err)
	}

	return future.Result(s.scope.LB)
}

func (s *Service) reconcileLBRules(lbSpec *network.LoadBalancer) (lb network.LoadBalancer, err error) {
	klog.V(2).Infof("Reconciling load balancer rules")
	lbRules := []network.LoadBalancingRule{
		{
			Name: to.StringPtr("api"),
			LoadBalancingRulePropertiesFormat: &network.LoadBalancingRulePropertiesFormat{
				Protocol:     network.TransportProtocolTCP,
				FrontendPort: to.Int32Ptr(6443),
				BackendPort:  to.Int32Ptr(6443),
				FrontendIPConfiguration: &network.SubResource{
					ID: to.StringPtr(
						s.getResourceID(
							*lbSpec.ID,
							"frontendIPConfigurations",
							*lbSpec.Name,
						),
					),
				},
				BackendAddressPool: &network.SubResource{
					ID: to.StringPtr(
						s.getResourceID(
							*lbSpec.ID,
							"backendAddressPools",
							*lbSpec.Name,
						),
					),
				},
				Probe: &network.SubResource{
					ID: to.StringPtr(
						s.getResourceID(
							*lbSpec.ID,
							"probes",
							*lbSpec.Name,
						),
					),
				},
			},
		},
	}

	lbSpec.LoadBalancingRules = &lbRules

	future, err := s.scope.LB.CreateOrUpdate(
		s.scope.Context,
		s.scope.ClusterConfig.ResourceGroup,
		to.String(lbSpec.Name),
		*lbSpec,
	)

	if err != nil {
		return lb, err
	}

	err = future.WaitForCompletionRef(s.scope.Context, s.scope.LB.Client)
	if err != nil {
		return lb, fmt.Errorf("cannot get load balancer create or update future response: %v", err)
	}

	klog.V(2).Info("Successfully reconciled load balancer rules")
	return future.Result(s.scope.LB)
}

func (s *Service) getResourceID(lbID, resource, resourceName string) string {
	return fmt.Sprintf("%s/%s/%s", lbID, resource, resourceName)
}

func (s *Service) setLBName(role string) *string {
	str := fmt.Sprintf("%s-%s", s.scope.Cluster.Name, role)
	return &str
}

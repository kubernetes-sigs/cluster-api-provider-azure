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

package compute

import (
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"google.golang.org/api/compute/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/azureerrors"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/wait"
)

func (s *Service) ReconcileFirewalls() error {
	for _, firewallSpec := range s.getFirewallSpecs() {
		// Get or create the firewall rules.
		firewall, err := s.firewalls.Get(s.scope.Project(), firewallSpec.Name).Do()
		if azureerrors.IsNotFound(err) {
			op, err := s.firewalls.Insert(s.scope.Project(), firewallSpec).Do()
			if err != nil {
				return errors.Wrapf(err, "failed to create firewall rule")
			}
			if err := wait.ForComputeOperation(s.scope.Compute, s.scope.Project(), op); err != nil {
				return errors.Wrapf(err, "failed to create firewall rule")
			}
			firewall, err = s.firewalls.Get(s.scope.Project(), firewallSpec.Name).Do()
			if err != nil {
				return errors.Wrapf(err, "failed to describe firewall rule")
			}
		} else if err != nil {
			return errors.Wrapf(err, "failed to describe firewall rule")
		}

		// Store in the Cluster Status.
		if s.scope.Network().FirewallRules == nil {
			s.scope.Network().FirewallRules = make(map[string]string)
		}
		s.scope.Network().FirewallRules[firewall.Name] = firewall.SelfLink
	}

	return nil
}

func (s *Service) DeleteFirewalls() error {
	for name := range s.scope.Network().FirewallRules {
		op, err := s.firewalls.Delete(s.scope.Project(), name).Do()
		if err != nil {
			return errors.Wrapf(err, "failed to delete forwarding rules")
		}
		if err := wait.ForComputeOperation(s.scope.Compute, s.scope.Project(), op); err != nil {
			return errors.Wrapf(err, "failed to delete forwarding rules")
		}
		delete(s.scope.Network().FirewallRules, name)
	}

	return nil
}

func (s *Service) getFirewallSpecs() []*compute.Firewall {
	return []*compute.Firewall{
		{
			Name:    fmt.Sprintf("allow-%s-%s-healthchecks", s.scope.Name(), infrav1.APIServerRoleTagValue),
			Network: s.scope.NetworkID(),
			Allowed: []*compute.FirewallAllowed{
				{
					IPProtocol: "TCP",
					Ports: []string{
						strconv.Itoa(APIServerLoadBalancerBackendPort),
					},
				},
			},
			Direction: "INGRESS",
			SourceRanges: []string{
				// Allow Google's internal IP ranges to perform health checks against our registered API servers.
				// For more information, https://cloud.google.com/load-balancing/docs/health-checks#fw-rule.
				"35.191.0.0/16",
				"130.211.0.0/22",
			},
			TargetTags: []string{
				fmt.Sprintf("%s-control-plane", s.scope.Name()),
			},
		},
		{
			Name:    fmt.Sprintf("allow-%s-%s-cluster", s.scope.Name(), infrav1.APIServerRoleTagValue),
			Network: s.scope.NetworkID(),
			Allowed: []*compute.FirewallAllowed{
				{
					IPProtocol: "all",
				},
			},
			Direction: "INGRESS",
			SourceTags: []string{
				fmt.Sprintf("%s-control-plane", s.scope.Name()),
				fmt.Sprintf("%s-node", s.scope.Name()),
			},
			TargetTags: []string{
				fmt.Sprintf("%s-control-plane", s.scope.Name()),
				fmt.Sprintf("%s-node", s.scope.Name()),
			},
		},
	}
}

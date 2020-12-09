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

package securitygroups

import (
	"context"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/converters"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// NSGScope defines the scope interface for a security groups service.
type NSGScope interface {
	logr.Logger
	azure.ClusterDescriber
	azure.NetworkDescriber
	NSGSpecs() []azure.NSGSpec
}

// Service provides operations on azure resources
type Service struct {
	Scope NSGScope
	client
}

// New creates a new service.
func New(scope NSGScope) *Service {
	return &Service{
		Scope:  scope,
		client: newClient(scope),
	}
}

// Reconcile gets/creates/updates a network security group.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "securitygroups.Service.Reconcile")
	defer span.End()

	if !s.Scope.IsVnetManaged() {
		s.Scope.V(4).Info("Skipping network security group reconcile in custom VNet mode")
		return nil
	}

	for _, nsgSpec := range s.Scope.NSGSpecs() {
		securityRules := make([]network.SecurityRule, 0)
		var etag *string

		existingNSG, err := s.client.Get(ctx, s.Scope.ResourceGroup(), nsgSpec.Name)
		switch {
		case err != nil && !azure.ResourceNotFound(err):
			return errors.Wrapf(err, "failed to get NSG %s in %s", nsgSpec.Name, s.Scope.ResourceGroup())
		case err == nil:
			// security group already exists
			// We append the existing NSG etag to the header to ensure we only apply the updates if the NSG has not been modified.
			etag = existingNSG.Etag
			// Check if the expected rules are present
			update := false
			securityRules = *existingNSG.SecurityRules
			for _, rule := range nsgSpec.IngressRules {
				if !ruleExists(securityRules, converters.IngresstoSecurityRule(*rule)) {
					update = true
					securityRules = append(securityRules, converters.IngresstoSecurityRule(*rule))
				}
			}
			if !update {
				// Skip update for NSG as the required default rules are present
				s.Scope.V(2).Info("security group exists and no default rules are missing, skipping update", "security group", nsgSpec.Name)
				continue
			}
		default:
			s.Scope.V(2).Info("creating security group", "security group", nsgSpec.Name)
			for _, rule := range nsgSpec.IngressRules {
				securityRules = append(securityRules, converters.IngresstoSecurityRule(*rule))
			}

		}
		sg := network.SecurityGroup{
			Location: to.StringPtr(s.Scope.Location()),
			SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
				SecurityRules: &securityRules,
			},
			Etag: etag,
		}
		err = s.client.CreateOrUpdate(ctx, s.Scope.ResourceGroup(), nsgSpec.Name, sg)
		if err != nil {
			return errors.Wrapf(err, "failed to create or update security group %s in resource group %s", nsgSpec.Name, s.Scope.ResourceGroup())
		}

		s.Scope.V(2).Info("successfully created or updated security group", "security group", nsgSpec.Name)
	}
	return nil
}

func ruleExists(rules []network.SecurityRule, rule network.SecurityRule) bool {
	for _, existingRule := range rules {
		if !strings.EqualFold(to.String(existingRule.Name), to.String(rule.Name)) {
			continue
		}
		if !strings.EqualFold(to.String(existingRule.DestinationPortRange), to.String(rule.DestinationPortRange)) {
			continue
		}
		if existingRule.Protocol != network.SecurityRuleProtocolTCP &&
			existingRule.Access != network.SecurityRuleAccessAllow &&
			existingRule.Direction != network.SecurityRuleDirectionInbound {
			continue
		}
		if !strings.EqualFold(to.String(existingRule.SourcePortRange), "*") &&
			!strings.EqualFold(to.String(existingRule.SourceAddressPrefix), "*") &&
			!strings.EqualFold(to.String(existingRule.DestinationAddressPrefix), "*") {
			continue
		}
		return true
	}
	return false
}

// Delete deletes the network security group with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "securitygroups.Service.Delete")
	defer span.End()

	for _, nsgSpec := range s.Scope.NSGSpecs() {
		s.Scope.V(2).Info("deleting security group", "security group", nsgSpec.Name)
		err := s.client.Delete(ctx, s.Scope.ResourceGroup(), nsgSpec.Name)
		if err != nil && azure.ResourceNotFound(err) {
			// already deleted
			continue
		}
		if err != nil {
			return errors.Wrapf(err, "failed to delete security group %s in resource group %s", nsgSpec.Name, s.Scope.ResourceGroup())
		}

		s.Scope.V(2).Info("successfully deleted security group", "security group", nsgSpec.Name)
	}
	return nil
}

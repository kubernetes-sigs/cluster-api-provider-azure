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
	"github.com/pkg/errors"
	"k8s.io/klog"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Spec specification for network security groups
type Spec struct {
	Name           string
	IsControlPlane bool
}

// Reconcile gets/creates/updates a network security group.
func (s *Service) Reconcile(ctx context.Context, spec interface{}) error {
	if !s.Scope.Vnet().IsManaged(s.Scope.ClusterName()) {
		s.Scope.V(4).Info("Skipping network security group reconcile in custom vnet mode")
		return nil
	}
	nsgSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid security groups specification")
	}

	securityGroup, err := s.Client.Get(ctx, s.Scope.ResourceGroup(), nsgSpec.Name)
	if err != nil && !azure.ResourceNotFound(err) {
		return errors.Wrapf(err, "failed to get NSG %s in %s", nsgSpec.Name, s.Scope.ResourceGroup())
	}

	nsgExists := false
	securityRules := make([]network.SecurityRule, 0)
	if securityGroup.Name != nil {
		nsgExists = true
		securityRules = *securityGroup.SecurityRules
	}

	ingressRules := make(map[string]network.SecurityRule, 0)

	if nsgSpec.IsControlPlane {
		// Add any specified ingress rules from controlplane security group spec
		cpSubnet := s.Scope.ControlPlaneSubnet()
		if cpSubnet != nil && len(cpSubnet.SecurityGroup.IngressRules) > 0 {
			for _, ingressRule := range cpSubnet.SecurityGroup.IngressRules {
				ingressRules[ingressRule.Name] = newIngressSecurityRule(*ingressRule)
			}

		}
	} else {
		// Add any specified ingress rules from node security group spec
		nodeSubnet := s.Scope.NodeSubnet()
		if nodeSubnet != nil && len(nodeSubnet.SecurityGroup.IngressRules) > 0 {
			for _, ingressRule := range nodeSubnet.SecurityGroup.IngressRules {
				ingressRules[ingressRule.Name] = newIngressSecurityRule(*ingressRule)
			}
		}
	}

	if nsgExists {
		// Check if the expected rules are present
		update := false
		for _, rule := range ingressRules {
			if !ruleExists(securityRules, rule) {
				update = true
				securityRules = append(securityRules, rule)
			}
		}
		if !update {
			// Skip update for control-plane NSG as the required default rules are present
			klog.V(2).Infof("security group %s exists and no default rules are missing, skipping update", nsgSpec.Name)
			return nil
		}
	} else {
		klog.V(2).Infof("applying missing default rules for control plane NSG %s", nsgSpec.Name)
		for _, rule := range ingressRules {
			securityRules = append(securityRules, rule)
		}
	}

	sg := network.SecurityGroup{
		Location: to.StringPtr(s.Scope.Location()),
		SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
			SecurityRules: &securityRules,
		},
	}
	if nsgExists {
		// We append the existing NSG etag to the header to ensure we only apply the updates if the NSG has not been modified.
		sg.Etag = securityGroup.Etag
	}
	klog.V(2).Infof("creating security group %s", nsgSpec.Name)
	err = s.Client.CreateOrUpdate(ctx, s.Scope.ResourceGroup(), nsgSpec.Name, sg)
	if err != nil {
		return errors.Wrapf(err, "failed to create security group %s in resource group %s", nsgSpec.Name, s.Scope.ResourceGroup())
	}

	klog.V(2).Infof("created security group %s", nsgSpec.Name)
	return err
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

func newIngressSecurityRule(ingress infrav1.IngressRule) network.SecurityRule {
	secRule := network.SecurityRule{
		Name: to.StringPtr(ingress.Name),
		SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
			Description:              to.StringPtr(ingress.Description),
			SourceAddressPrefix:      ingress.Source,
			SourcePortRange:          ingress.SourcePorts,
			DestinationAddressPrefix: ingress.Destination,
			DestinationPortRange:     ingress.DestinationPorts,
			Access:                   network.SecurityRuleAccessAllow,
			Direction:                network.SecurityRuleDirectionInbound,
			Priority:                 to.Int32Ptr(ingress.Priority),
		},
	}

	switch ingress.Protocol {
	case infrav1.SecurityGroupProtocolAll:
		secRule.SecurityRulePropertiesFormat.Protocol = network.SecurityRuleProtocolAsterisk
	case infrav1.SecurityGroupProtocolTCP:
		secRule.SecurityRulePropertiesFormat.Protocol = network.SecurityRuleProtocolTCP
	case infrav1.SecurityGroupProtocolUDP:
		secRule.SecurityRulePropertiesFormat.Protocol = network.SecurityRuleProtocolUDP
	}

	return secRule
}

// Delete deletes the network security group with the provided name.
func (s *Service) Delete(ctx context.Context, spec interface{}) error {
	nsgSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid security groups specification")
	}
	klog.V(2).Infof("deleting security group %s", nsgSpec.Name)
	err := s.Client.Delete(ctx, s.Scope.ResourceGroup(), nsgSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "failed to delete security group %s in resource group %s", nsgSpec.Name, s.Scope.ResourceGroup())
	}

	klog.V(2).Infof("deleted security group %s", nsgSpec.Name)
	return nil
}

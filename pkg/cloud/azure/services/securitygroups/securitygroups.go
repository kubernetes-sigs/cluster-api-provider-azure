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

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-12-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure"
)

// Spec specification for network security groups
type Spec struct {
	Name           string
	IsControlPlane bool
}

// Get provides information about a network security group.
func (s *Service) Get(ctx context.Context, spec v1alpha1.ResourceSpec) (interface{}, error) {
	nsgSpec, ok := spec.(*Spec)
	if !ok {
		return network.SecurityGroup{}, errors.New("invalid security groups specification")
	}
	securityGroup, err := s.Client.Get(ctx, s.Scope.ClusterConfig.ResourceGroup, nsgSpec.Name, "")
	if err != nil && azure.ResourceNotFound(err) {
		return nil, errors.Wrapf(err, "security group %s not found", nsgSpec.Name)
	} else if err != nil {
		return securityGroup, err
	}
	return securityGroup, nil
}

// Reconcile gets/creates/updates a network security group.
func (s *Service) Reconcile(ctx context.Context, spec v1alpha1.ResourceSpec) error {
	nsgSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid security groups specification")
	}

	securityRules := &[]network.SecurityRule{}

	if nsgSpec.IsControlPlane {
		klog.V(2).Infof("using additional rules for control plane %s", nsgSpec.Name)
		securityRules = &[]network.SecurityRule{
			{
				Name: to.StringPtr("allow_ssh"),
				SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
					Protocol:                 network.SecurityRuleProtocolTCP,
					SourceAddressPrefix:      to.StringPtr("*"),
					SourcePortRange:          to.StringPtr("*"),
					DestinationAddressPrefix: to.StringPtr("*"),
					DestinationPortRange:     to.StringPtr("22"),
					Access:                   network.SecurityRuleAccessAllow,
					Direction:                network.SecurityRuleDirectionInbound,
					Priority:                 to.Int32Ptr(100),
				},
			},
			{
				Name: to.StringPtr("allow_6443"),
				SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
					Protocol:                 network.SecurityRuleProtocolTCP,
					SourceAddressPrefix:      to.StringPtr("*"),
					SourcePortRange:          to.StringPtr("*"),
					DestinationAddressPrefix: to.StringPtr("*"),
					DestinationPortRange:     to.StringPtr("6443"),
					Access:                   network.SecurityRuleAccessAllow,
					Direction:                network.SecurityRuleDirectionInbound,
					Priority:                 to.Int32Ptr(101),
				},
			},
		}
	}

	klog.V(2).Infof("creating security group %s", nsgSpec.Name)
	future, err := s.Client.CreateOrUpdate(
		ctx,
		s.Scope.ClusterConfig.ResourceGroup,
		nsgSpec.Name,
		network.SecurityGroup{
			Location: to.StringPtr(s.Scope.ClusterConfig.Location),
			SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
				SecurityRules: securityRules,
			},
		},
	)
	if err != nil {
		return errors.Wrapf(err, "failed to create security group %s in resource group %s", nsgSpec.Name, s.Scope.ClusterConfig.ResourceGroup)
	}

	err = future.WaitForCompletionRef(ctx, s.Client.Client)
	if err != nil {
		return errors.Wrap(err, "cannot create, future response")
	}

	_, err = future.Result(s.Client)
	if err != nil {
		return errors.Wrap(err, "result error")
	}
	klog.V(2).Infof("created security group %s", nsgSpec.Name)
	return err
}

// Delete deletes the network security group with the provided name.
func (s *Service) Delete(ctx context.Context, spec v1alpha1.ResourceSpec) error {
	nsgSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid security groups specification")
	}
	klog.V(2).Infof("deleting security group %s", nsgSpec.Name)
	future, err := s.Client.Delete(ctx, s.Scope.ClusterConfig.ResourceGroup, nsgSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "failed to delete security group %s in resource group %s", nsgSpec.Name, s.Scope.ClusterConfig.ResourceGroup)
	}

	err = future.WaitForCompletionRef(ctx, s.Client.Client)
	if err != nil {
		return errors.Wrap(err, "cannot create, future response")
	}

	_, err = future.Result(s.Client)
	if err != nil {
		return err
	}

	klog.V(2).Infof("deleted security group %s", nsgSpec.Name)
	return err
}

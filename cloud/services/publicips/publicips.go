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

package publicips

import (
	"context"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-06-01/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Spec specification for public ip
type Spec struct {
	Name    string
	DNSName string
}

// Reconcile gets/creates/updates a public ip.
func (s *Service) Reconcile(ctx context.Context, spec interface{}) error {
	publicIPSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid PublicIP Specification")
	}
	ipName := publicIPSpec.Name
	klog.V(2).Infof("creating public ip %s", ipName)

	// https://docs.microsoft.com/en-us/azure/load-balancer/load-balancer-standard-availability-zones#zone-redundant-by-default
	err := s.Client.CreateOrUpdate(
		ctx,
		s.Scope.ResourceGroup(),
		ipName,
		network.PublicIPAddress{
			Sku:      &network.PublicIPAddressSku{Name: network.PublicIPAddressSkuNameStandard},
			Name:     to.StringPtr(ipName),
			Location: to.StringPtr(s.Scope.Location()),
			PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
				PublicIPAddressVersion:   network.IPv4,
				PublicIPAllocationMethod: network.Static,
				DNSSettings: &network.PublicIPAddressDNSSettings{
					DomainNameLabel: to.StringPtr(strings.ToLower(ipName)),
					Fqdn:            &publicIPSpec.DNSName,
				},
			},
		},
	)

	if err != nil {
		return errors.Wrap(err, "cannot create public ip")
	}

	klog.V(2).Infof("successfully created public ip %s", ipName)
	return nil
}

// Delete deletes the public ip with the provided scope.
func (s *Service) Delete(ctx context.Context, spec interface{}) error {
	publicIPSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("invalid PublicIP Specification")
	}
	klog.V(2).Infof("deleting public ip %s", publicIPSpec.Name)
	err := s.Client.Delete(ctx, s.Scope.ResourceGroup(), publicIPSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "failed to delete public ip %s in resource group %s", publicIPSpec.Name, s.Scope.ResourceGroup())
	}

	klog.V(2).Infof("deleted public ip %s", publicIPSpec.Name)
	return err
}

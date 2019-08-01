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

package availabilityzones

import (
	"context"
	"sort"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-07-01/compute"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
)

// Spec input specification for Get/CreateOrUpdate/Delete calls
type Spec struct {
	VMSize string
}

// Get provides information about a availability zones.
func (s *Service) Get(ctx context.Context, spec v1alpha1.ResourceSpec) (interface{}, error) {
	var zones []string
	skusSpec, ok := spec.(*Spec)
	if !ok {
		return zones, errors.New("invalid availability zones specification")
	}
	// Prefer ListComplete() over List() to automatically traverse pages via iterator.
	res, err := s.Client.ListComplete(ctx)
	if err != nil {
		return zones, err
	}

	for res.NotDone() {
		resSku := res.Value()
		if strings.EqualFold(*resSku.Name, skusSpec.VMSize) {
			// Use map for easy deletion and iteration
			availableZones := make(map[string]bool)
			for _, locationInfo := range *resSku.LocationInfo {
				for _, zone := range *locationInfo.Zones {
					availableZones[zone] = true
				}
				if strings.EqualFold(*locationInfo.Location, s.Scope.ClusterConfig.Location) {
					for _, restriction := range *resSku.Restrictions {
						// Can't deploy anything in this subscription in this location. Bail out.
						if restriction.Type == compute.Location {
							return []string{}, errors.Errorf("rejecting sku: %s in location: %s due to susbcription restriction", skusSpec.VMSize, s.Scope.ClusterConfig.Location)
						}
						// May be able to deploy one or more zones to this location.
						for _, restrictedZone := range *restriction.RestrictionInfo.Zones {
							delete(availableZones, restrictedZone)
						}
					}
					// Back to slice. Empty is fine, and will deploy the VM to some FD/UD (no point in configuring this until supported at higher levels)
					result := make([]string, 0)
					for availableZone := range availableZones {
						result = append(result, availableZone)
					}
					// Lexical sort so comparisons work in tests
					sort.Strings(result)
					zones = result
				}
			}
		}
		err = res.NextWithContext(ctx)
		if err != nil {
			return zones, errors.Wrap(err, "could not iterate availability zones")
		}
	}

	return zones, nil
}

// Reconcile no-op.
func (s *Service) Reconcile(ctx context.Context, spec v1alpha1.ResourceSpec) error {
	// Not implemented since there is nothing to reconcile
	return nil
}

// Delete no-op.
func (s *Service) Delete(ctx context.Context, spec v1alpha1.ResourceSpec) error {
	// Not implemented since there is nothing to delete
	return nil
}

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
	"strings"

	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure"
)

// Spec input specification for Get/CreateOrUpdate/Delete calls
type Spec struct {
	VMSize string
}

// Get provides information about a availability zones.
func (s *Service) Get(ctx context.Context, spec azure.Spec) (interface{}, error) {
	var zones []string
	skusSpec, ok := spec.(*Spec)
	if !ok {
		return zones, errors.New("invalid availability zones specification")
	}
	res, err := s.Client.List(ctx)
	if err != nil {
		return zones, err
	}

	for _, resSku := range res.Values() {
		if strings.EqualFold(*resSku.Name, skusSpec.VMSize) {
			for _, locationInfo := range *resSku.LocationInfo {
				if strings.EqualFold(*locationInfo.Location, s.Scope.ClusterConfig.Location) {
					zones = *locationInfo.Zones
				}
			}
		}
	}

	return zones, nil
}

// Reconcile no-op.
func (s *Service) Reconcile(ctx context.Context, spec azure.Spec) error {
	// Not implemented since there is nothing to reconcile
	return nil
}

// Delete no-op.
func (s *Service) Delete(ctx context.Context, spec azure.Spec) error {
	// Not implemented since there is nothing to delete
	return nil
}

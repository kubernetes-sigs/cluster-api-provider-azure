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

	"github.com/pkg/errors"
)

func (s *Service) getZones() ([]string, error) {
	region, err := s.scope.Compute.Regions.Get(s.scope.Project(), s.scope.Region()).Do()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to describe region %q", s.scope.Region())
	}

	zones, err := s.scope.Compute.Zones.
		List(s.scope.Project()).
		Filter(fmt.Sprintf("region = %q", region.SelfLink)).
		Do()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to describe zones in region %q", s.scope.Region())
	}

	res := make([]string, 0, len(zones.Items))
	for _, x := range zones.Items {
		res = append(res, x.Name)
	}
	return res, nil
}

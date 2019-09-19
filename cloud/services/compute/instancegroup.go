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
	"path"

	"github.com/pkg/errors"
	"google.golang.org/api/compute/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/azureerrors"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/wait"
)

func (s *Service) ReconcileInstanceGroups() error {
	// Get each available zone.
	zones, err := s.getZones()
	if err != nil {
		return err
	}

	// Reconcile API Server instance groups and record them.
	for _, zone := range zones {
		name := fmt.Sprintf("%s-%s-%s", s.scope.Name(), infrav1.APIServerRoleTagValue, zone)
		group, err := s.instancegroups.Get(s.scope.Project(), zone, name).Do()
		switch {
		case azureerrors.IsNotFound(err):
			continue
		case err != nil:
			return errors.Wrapf(err, "failed to describe instance group %q", name)
		default:
			if s.scope.Network().APIServerInstanceGroups == nil {
				s.scope.Network().APIServerInstanceGroups = make(map[string]string)
			}
			s.scope.Network().APIServerInstanceGroups[zone] = group.SelfLink
		}
	}

	return nil
}

func (s *Service) DeleteInstanceGroups() error {
	for zone, groupSelfLink := range s.scope.Network().APIServerInstanceGroups {
		name := path.Base(groupSelfLink)
		op, err := s.instancegroups.Delete(s.scope.Project(), zone, name).Do()
		if err != nil {
			return errors.Wrapf(err, "failed to create backend service")
		}
		if err := wait.ForComputeOperation(s.scope.Compute, s.scope.Project(), op); err != nil {
			return errors.Wrapf(err, "failed to create backend service")
		}
	}
	return nil
}

func (s *Service) GetOrCreateInstanceGroup(zone, name string) (*compute.InstanceGroup, error) {
	group, err := s.instancegroups.Get(s.scope.Project(), zone, name).Do()
	if azureerrors.IsNotFound(err) {
		spec := &compute.InstanceGroup{
			Name:    name,
			Network: s.scope.NetworkID(),
			NamedPorts: []*compute.NamedPort{
				{
					Name: "apiserver",
					Port: 6443,
				},
			},
		}
		op, err := s.instancegroups.Insert(s.scope.Project(), zone, spec).Do()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create instance group")
		}
		if err := wait.ForComputeOperation(s.scope.Compute, s.scope.Project(), op); err != nil {
			return nil, errors.Wrapf(err, "failed to create instance group")
		}
		group, err = s.instancegroups.Get(s.scope.Project(), zone, name).Do()
		if err != nil {
			return nil, errors.Wrapf(err, "failed to describe instance group")
		}
	} else if err != nil {
		return nil, errors.Wrapf(err, "failed to describe instance group")
	}

	return group, nil
}

func (s *Service) GetInstanceGroupMembers(zone, name string) ([]*compute.InstanceWithNamedPorts, error) {
	members, err := s.instancegroups.
		ListInstances(s.scope.Project(), zone, name, &compute.InstanceGroupsListInstancesRequest{}).
		Do()
	if err != nil {
		return nil, errors.Wrapf(err, "could not list instances in group %q", name)
	}
	return members.Items, nil
}

func (s *Service) EnsureInstanceGroupMember(zone, name string, i *compute.Instance) error {
	members, err := s.GetInstanceGroupMembers(zone, name)
	if err != nil {
		return err
	}

	// If the instance is already registered, return early.
	for _, registered := range members {
		if registered.Instance == i.SelfLink {
			return nil
		}
	}

	// Register the instance with the group
	req := &compute.InstanceGroupsAddInstancesRequest{
		Instances: []*compute.InstanceReference{
			{
				Instance: i.SelfLink,
			},
		},
	}
	op, err := s.instancegroups.AddInstances(s.scope.Project(), zone, name, req).Do()
	if err != nil {
		return errors.Wrapf(err, "failed to add instance to group")
	}
	if err := wait.ForComputeOperation(s.scope.Compute, s.scope.Project(), op); err != nil {
		return errors.Wrapf(err, "failed to add instance to group")
	}

	return nil
}

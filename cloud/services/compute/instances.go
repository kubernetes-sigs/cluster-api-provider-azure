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
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/blang/semver"
	"github.com/pkg/errors"
	"google.golang.org/api/compute/v1"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha2"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/azureerrors"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/wait"
	"sigs.k8s.io/cluster-api/util/record"
)

// InstanceIfExists returns the existing instance or nothing if it doesn't exist.
func (s *Service) InstanceIfExists(scope *scope.MachineScope) (*compute.Instance, error) {
	s.scope.V(2).Info("Looking for instance by name", "instance-name", scope.Name())

	res, err := s.instances.Get(s.scope.Project(), scope.Zone(), scope.Name()).Do()
	switch {
	case azureerrors.IsNotFound(err):
		return nil, nil
	case err != nil:
		return nil, errors.Wrapf(err, "failed to describe instance: %q", scope.Name())
	}

	return res, nil
}

// CreateInstance runs an ec2 instance.
func (s *Service) CreateInstance(scope *scope.MachineScope) (*compute.Instance, error) {
	s.scope.V(2).Info("Creating an instance")

	decoded, err := base64.StdEncoding.DecodeString(*scope.Machine.Spec.Bootstrap.Data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode bootstrapData")
	}

	if scope.Machine.Spec.Version == nil {
		return nil, errors.Errorf("missing required Spec.Version on Machine %q in namespace %q",
			scope.Name(), scope.Namespace())
	}

	version, err := semver.ParseTolerant(*scope.Machine.Spec.Version)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing Spec.Version on Machine %q in namespace %q, expected valid SemVer string",
			scope.Name(), scope.Namespace())
	}

	input := &compute.Instance{
		Name:         scope.Name(),
		Zone:         scope.Zone(),
		MachineType:  fmt.Sprintf("zones/%s/machineTypes/%s", scope.Zone(), scope.AzureMachine.Spec.InstanceType),
		CanIpForward: true,
		NetworkInterfaces: []*compute.NetworkInterface{{
			Network: s.scope.NetworkID(),
		}},
		Tags: &compute.Tags{
			Items: append(
				scope.AzureMachine.Spec.AdditionalNetworkTags,
				fmt.Sprintf("%s-%s", scope.Cluster.Name, scope.Role()),
			),
		},
		Disks: []*compute.AttachedDisk{
			{
				AutoDelete: true,
				Boot:       true,
				InitializeParams: &compute.AttachedDiskInitializeParams{
					DiskSizeGb: 30,
					DiskType:   fmt.Sprintf("zones/%s/diskTypes/%s", scope.Zone(), "pd-standard"),
					SourceImage: fmt.Sprintf(
						"projects/%s/global/images/family/capi-ubuntu-1804-k8s-v%d-%d",
						s.scope.Project(), version.Major, version.Minor),
				},
			},
		},
		Metadata: &compute.Metadata{
			Items: []*compute.MetadataItems{
				{
					Key:   "user-data",
					Value: pointer.StringPtr(string(decoded)),
				},
			},
		},
		ServiceAccounts: []*compute.ServiceAccount{
			{
				Email: "default",
				Scopes: []string{
					compute.CloudPlatformScope,
				},
			},
		},
	}

	if scope.AzureMachine.Spec.ServiceAccount != nil {
		serviceAccount := scope.AzureMachine.Spec.ServiceAccount
		input.ServiceAccounts = []*compute.ServiceAccount{
			{
				Email:  serviceAccount.Email,
				Scopes: serviceAccount.Scopes,
			},
		}
	}

	input.Labels = infrav1.Build(infrav1.BuildParams{
		ClusterName: s.scope.Name(),
		Lifecycle:   infrav1.ResourceLifecycleOwned,
		Role:        aws.String(scope.Role()),
		// TODO(vincepri): Check what needs to be added for the cloud provider label.
		Additional: s.scope.
			AzureCluster.Spec.
			AdditionalLabels.
			AddLabels(scope.AzureMachine.Spec.AdditionalLabels),
	})

	if scope.AzureMachine.Spec.Image != nil {
		input.Disks[0].InitializeParams.SourceImage = *scope.AzureMachine.Spec.Image
	} else if scope.AzureMachine.Spec.ImageFamily != nil {
		input.Disks[0].InitializeParams.SourceImage = *scope.AzureMachine.Spec.ImageFamily
	}

	if scope.AzureMachine.Spec.PublicIP != nil && *scope.AzureMachine.Spec.PublicIP {
		input.NetworkInterfaces[0].AccessConfigs = []*compute.AccessConfig{
			{
				Type: "ONE_TO_ONE_NAT",
				Name: "External NAT",
			},
		}
	}

	if scope.AzureMachine.Spec.RootDeviceSize > 0 {
		input.Disks[0].InitializeParams.DiskSizeGb = scope.AzureMachine.Spec.RootDeviceSize
	}

	if scope.AzureMachine.Spec.Subnet != nil {
		input.NetworkInterfaces[0].Subnetwork = fmt.Sprintf("regions/%s/subnetwork/%s",
			scope.Region(), *scope.AzureMachine.Spec.Subnet)
	}

	if s.scope.Network().APIServerAddress == nil {
		return nil, errors.New("failed to run controlplane, APIServer address not available")
	}

	s.scope.V(2).Info("Running instance", "machine-role", scope.Role())
	out, err := s.runInstance(input)
	if err != nil {
		record.Warnf(scope.Machine, "FailedCreate", "Failed to create instance: %v", err)
		return nil, err
	}

	record.Eventf(scope.Machine, "SuccessfulCreate", "Created new %s instance with name %q", scope.Role(), out.Name)
	return out, nil
}

func (s *Service) runInstance(input *compute.Instance) (*compute.Instance, error) {
	op, err := s.instances.Insert(s.scope.Project(), input.Zone, input).Do()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create azure instance")
	}

	if err := wait.ForComputeOperation(s.scope.Compute, s.scope.Project(), op); err != nil {
		return nil, errors.Wrap(err, "failed to create azure instance")
	}

	return s.instances.Get(s.scope.Project(), input.Zone, input.Name).Do()
}

func (s *Service) TerminateInstanceAndWait(scope *scope.MachineScope) error {
	op, err := s.instances.Delete(s.scope.Project(), scope.Zone(), scope.Name()).Do()
	if err != nil {
		return errors.Wrap(err, "failed to terminate azure instance")
	}

	if err := wait.ForComputeOperation(s.scope.Compute, s.scope.Project(), op); err != nil {
		return errors.Wrap(err, "failed to terminate azure instance")
	}

	return nil
}

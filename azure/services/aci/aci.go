/*
Copyright 2021 The Kubernetes Authors.

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

package aci

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/containerinstance/mgmt/2019-12-01/containerinstance"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

type (
	// ContainerGroupScope defines the scope interface for a container group service
	ContainerGroupScope interface {
		logr.Logger
		azure.ClusterDescriber
		ContainerGroupSpec(ctx context.Context) (azure.ContainerGroupSpec, error)
		SetVMState(state infrav1.VMState)
		Name() string
	}

	// Service provides operations on azure resources
	Service struct {
		Scope ContainerGroupScope
		Client
	}
)

// New creates a new service.
func New(scope ContainerGroupScope) *Service {
	return &Service{
		Scope:  scope,
		Client: newClient(scope),
	}
}

// Reconcile idempotently creates or updates a container group, if possible.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "aci.Service.Reconcile")
	defer span.End()

	spec, err := s.Scope.ContainerGroupSpec(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to build container group spec")
	}

	containerGroup := s.containerGroupSpecToContainerGroup(spec)
	s.Scope.Info("!!!!!!!bootstrapData!!!!!!!", "data", spec.BootstrapData)
	cg, err := s.Client.CreateOrUpdate(ctx, s.Scope.ResourceGroup(), s.Scope.Name(), containerGroup)
	if err != nil {
		return errors.Wrap(err, "failed to create or update the container group")
	}

	if cg.ProvisioningState != nil {
		s.Scope.SetVMState(infrav1.VMState(*cg.ProvisioningState))
	}

	return nil
}

// Delete deletes the container group with the provided name.
func (s *Service) Delete(ctx context.Context) error {
	ctx, span := tele.Tracer().Start(ctx, "aci.Service.Delete")
	defer span.End()

	return s.Client.Delete(ctx, s.Scope.ResourceGroup(), s.Scope.Name())
}

func (s Service) containerGroupSpecToContainerGroup(spec azure.ContainerGroupSpec) containerinstance.ContainerGroup {
	containers := make([]containerinstance.Container, len(spec.Containers))
	for i, c := range spec.Containers {
		var (
			c       = c
			envVars = make([]containerinstance.EnvironmentVariable, len(c.EnvVars))
		)

		for i, envVar := range c.EnvVars {
			envVars[i] = containerinstance.EnvironmentVariable{
				Name:        to.StringPtr(envVar.Name),
				Value:       to.StringPtr(envVar.Value),
				SecureValue: to.StringPtr(envVar.SecureValue),
			}
		}

		containers[i] = containerinstance.Container{
			Name: to.StringPtr(c.Name),
			ContainerProperties: &containerinstance.ContainerProperties{
				Image:                to.StringPtr(c.Image),
				Command:              &c.Command,
				EnvironmentVariables: &envVars,
				Resources: &containerinstance.ResourceRequirements{
					Requests: &containerinstance.ResourceRequests{
						MemoryInGB: to.Float64Ptr(4),
						CPU:        to.Float64Ptr(2),
					},
					Limits:   &containerinstance.ResourceLimits{
						MemoryInGB: to.Float64Ptr(8),
						CPU:        to.Float64Ptr(4),
					},
				},
			},
		}
	}

	return containerinstance.ContainerGroup{
		ContainerGroupProperties: &containerinstance.ContainerGroupProperties{
			Containers:     &containers,
			OsType:         containerinstance.Linux,
		},
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: s.Scope.ClusterName(),
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Name:        to.StringPtr(spec.Name),
			Role:        to.StringPtr(infrav1.Node),
			Additional:  s.Scope.AdditionalTags(),
		})),
		Name:     to.StringPtr(s.Scope.Name()),
		Location: to.StringPtr(s.Scope.Location()),
	}
}

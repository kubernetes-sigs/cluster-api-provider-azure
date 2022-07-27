/*
Copyright 2020 The Kubernetes Authors.

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

package managedclusters

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/containerservice/mgmt/2021-05-01/containerservice"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const serviceName = "managedcluster"

// ManagedClusterScope defines the scope interface for a managed cluster.
type ManagedClusterScope interface {
	azure.Authorizer
	azure.AsyncStatusUpdater
	ManagedClusterSpec(context.Context) azure.ResourceSpecGetter
	SetControlPlaneEndpoint(clusterv1.APIEndpoint)
	MakeEmptyKubeConfigSecret() corev1.Secret
	GetKubeConfigData() []byte
	SetKubeConfigData([]byte)
}

// Service provides operations on azure resources.
type Service struct {
	Scope ManagedClusterScope
	async.Reconciler
	CredentialGetter
}

// New creates a new service.
func New(scope ManagedClusterScope) *Service {
	client := newClient(scope)
	return &Service{
		Scope:            scope,
		Reconciler:       async.New(scope, client, client),
		CredentialGetter: client,
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return serviceName
}

// Reconcile idempotently creates or updates a managed cluster, if possible.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "managedclusters.Service.Reconcile")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	managedClusterSpec := s.Scope.ManagedClusterSpec(ctx)
	if managedClusterSpec == nil {
		return nil
	}

	result, resultErr := s.CreateResource(ctx, managedClusterSpec, serviceName)
	if resultErr == nil {
		managedCluster, ok := result.(containerservice.ManagedCluster)
		if !ok {
			return errors.Errorf("%T is not a containerservice.ManagedCluster", result)
		}
		// Update control plane endpoint.
		endpoint := clusterv1.APIEndpoint{
			Host: to.String(managedCluster.ManagedClusterProperties.Fqdn),
			Port: 443,
		}
		s.Scope.SetControlPlaneEndpoint(endpoint)

		// Update kubeconfig data
		// Always fetch credentials in case of rotation
		typedSpec := managedClusterSpec.(*ManagedClusterSpec)
		kubeConfigData, err := s.GetCredentials(ctx, typedSpec)
		if err != nil {
			return errors.Wrap(err, "failed to get credentials for managed cluster")
		}
		s.Scope.SetKubeConfigData(kubeConfigData)
	}
	s.Scope.UpdatePutStatus(infrav1.ManagedClusterRunningCondition, serviceName, resultErr)
	return resultErr
}

// Delete deletes the managed cluster.
func (s *Service) Delete(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "managedclusters.Service.Delete")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	managedClusterSpec := s.Scope.ManagedClusterSpec(ctx)
	if managedClusterSpec == nil {
		return nil
	}

	err := s.DeleteResource(ctx, managedClusterSpec, serviceName)
	s.Scope.UpdateDeleteStatus(infrav1.ManagedClusterRunningCondition, serviceName, err)
	return err
}

// IsManaged returns always returns true as CAPZ does not support BYO managed cluster.
func (s *Service) IsManaged(ctx context.Context) (bool, error) {
	return true, nil
}

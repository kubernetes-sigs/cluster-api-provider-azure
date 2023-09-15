/*
Copyright 2023 The Kubernetes Authors.

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
package aso

import (
	"context"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Service provides operations on Azure resources.
type Service[T deepCopier[T], S Scope] struct {
	Reconciler[T]

	Scope             S
	Spec              azure.ASOResourceSpecGetter[T]
	PostReconcileHook func(scope S, result T, err error) error
	PostDeleteHook    func(scope S, err error) error

	name string
}

// NewService creates a new Service.
func NewService[T deepCopier[T], S Scope](name string, scope S) *Service[T, S] {
	return &Service[T, S]{
		name:       name,
		Scope:      scope,
		Reconciler: New[T](scope.GetClient(), scope.ClusterName()),
	}
}

// Name returns the service name.
func (s *Service[T, S]) Name() string {
	return s.name
}

// Reconcile idempotently creates or updates a resource group.
func (s *Service[T, S]) Reconcile(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "aso.Service.Reconcile")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	result, err := s.CreateOrUpdateResource(ctx, s.Spec, s.Name())
	if s.PostReconcileHook != nil {
		return s.PostReconcileHook(s.Scope, result, err)
	}
	return err
}

// Delete deletes the resource group if it is managed by capz.
func (s *Service[T, S]) Delete(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "aso.Service.Delete")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureServiceReconcileTimeout)
	defer cancel()

	err := s.DeleteResource(ctx, s.Spec, s.Name())
	if s.PostDeleteHook != nil {
		return s.PostDeleteHook(s.Scope, err)
	}
	return err
}

// Pause implements azure.Pauser.
func (s *Service[T, S]) Pause(ctx context.Context) error {
	var _ azure.Pauser = (*Service[T, S])(nil)
	return PauseResource(ctx, s.Scope.GetClient(), s.Spec, s.Scope.ClusterName(), s.Name())
}

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

package conditionalaso

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/aso"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// Service provides operations on Azure resources.
type Service[T genruntime.MetaObject, S Scope] struct {
	aso.Reconciler[T]

	Scope S
	Specs []azure.ASOResourceSpecGetter[T]
	// ListFunc is used to enumerate ASO existent resources. Currently this interface is designed only to aid
	// discovery of ASO resources that no longer have a CAPZ reference, and can thus be deleted. This behavior
	// may be skipped for a service by leaving this field nil.
	ListFunc func(ctx context.Context, client client.Client, opts ...client.ListOption) (resources []T, err error)

	ConditionType                  clusterv1.ConditionType
	PostCreateOrUpdateResourceHook func(ctx context.Context, scope S, result T, err error) error
	PostReconcileHook              func(ctx context.Context, scope S, err error) error
	PostDeleteHook                 func(ctx context.Context, scope S, err error) error

	name string
}

// NewService creates a new Service.
func NewService[T genruntime.MetaObject, S Scope](name string, scope S) *Service[T, S] {
	return &Service[T, S]{
		Reconciler: New[T](scope.GetClient(), scope.ClusterName(), scope.ASOOwner()),
		Scope:      scope,
		name:       name,
	}
}

// Name returns the service name.
func (s *Service[T, S]) Name() string {
	return s.name
}

// Reconcile idempotently creates or updates the resources.
func (s *Service[T, S]) Reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "aso.Service.Reconcile")
	defer done()

	// TODO: mveber - Use a longer timeout for conditionalaso service to handle rate limiting
	timeout := s.Scope.DefaultedAzureServiceReconcileTimeout()
	if timeout < 120*time.Second {
		timeout = 120 * time.Second // Minimum 1 minute for rate-limited operations
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// We go through the list of Specs to reconcile each one, independently of the result of the previous one.
	// If multiple errors occur, we return the most pressing one.
	// Order of precedence (highest -> lowest) is:
	//   - error that is not an operationNotDoneError (i.e. error creating)
	//   - operationNotDoneError (i.e. creating in progress)
	//   - no error (i.e. created)
	var resultErr error

	inList := map[string]T{}
	if s.ListFunc != nil {
		toReconcile := map[string]struct{}{}
		for _, spec := range s.Specs {
			toReconcile[spec.ResourceRef().GetName()] = struct{}{}
		}
		list, err := s.ListFunc(ctx, s.Scope.GetClient(), client.InNamespace(s.Scope.ASOOwner().GetNamespace()))
		if err != nil {
			resultErr = err
		} else {
			for _, existing := range list {
				inList[existing.GetName()] = existing
				if _, exists := toReconcile[existing.GetName()]; exists {
					continue
				}
				err := s.Reconciler.DeleteResource(ctx, existing, s.Name())
				if err != nil && (!azure.IsOperationNotDoneError(err) || resultErr == nil) {
					resultErr = err
				}
			}
		}
	}

	for _, spec := range s.Specs {
		if obj, exists := inList[spec.ResourceRef().GetName()]; exists {
			// Check if the existing resource is reconciled/ready
			if err := s.checkResourceReconciled(obj); err != nil {
				resultErr = errors.Wrapf(err, "existing resource %s/%s is not yet reconciled (service: %s)", obj.GetName(), obj.GetNamespace(), s.name)
				break
			}
			continue
		}
		if !s.Scope.CreateIfNotExists() {
			obj := spec.ResourceRef()
			return errors.Errorf("Waiting for resource %s/%s to be crated (service: %s)", obj.GetName(), obj.GetNamespace(), s.name)
		}
		result, err := s.CreateOrUpdateResource(ctx, spec, s.Name())
		if s.PostCreateOrUpdateResourceHook != nil {
			err = s.PostCreateOrUpdateResourceHook(ctx, s.Scope, result, err)
		}
		if err != nil && (!azure.IsOperationNotDoneError(err) || resultErr == nil) {
			resultErr = err
		}
	}

	if s.PostReconcileHook != nil {
		resultErr = s.PostReconcileHook(ctx, s.Scope, resultErr)
	}
	s.Scope.UpdatePutStatus(s.ConditionType, s.Name(), resultErr)
	if resultErr != nil {
		log.Error(resultErr, "update condition", "condition", s.ConditionType, "service", s.Name())
	} else {
		log.Info("update condition", "condition", s.ConditionType, "service", s.Name())
	}
	return resultErr
}

// Delete deletes the resources.
func (s *Service[T, S]) Delete(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "aso.Service.Delete")
	defer done()

	ctx, cancel := context.WithTimeout(ctx, s.Scope.DefaultedAzureServiceReconcileTimeout())
	defer cancel()

	if len(s.Specs) == 0 {
		return nil
	}

	// We go through the list of Specs to delete each one, independently of the resultErr of the previous one.
	// If multiple errors occur, we return the most pressing one.
	// Order of precedence (highest -> lowest) is:
	//   - error that is not an operationNotDoneError (i.e. error deleting)
	//   - operationNotDoneError (i.e. deleting in progress)
	//   - no error (i.e. deleted)
	var resultErr error
	for _, spec := range s.Specs {
		err := s.DeleteResource(ctx, spec.ResourceRef(), s.Name())
		if err != nil && (!azure.IsOperationNotDoneError(err) || resultErr == nil) {
			log.V(4).Info(fmt.Sprintf("Delete-not done: %s", err.Error()))
			resultErr = err
		}
	}

	if s.PostDeleteHook != nil {
		resultErr = s.PostDeleteHook(ctx, s.Scope, resultErr)
	}
	s.Scope.UpdateDeleteStatus(s.ConditionType, s.Name(), resultErr)
	return resultErr
}

// checkResourceReconciled checks if the ASO resource is reconciled/ready.
func (s *Service[T, S]) checkResourceReconciled(resource T) error {
	conds := resource.GetConditions()
	i, readyExists := conds.FindIndexByType(conditions.ConditionTypeReady)
	if !readyExists {
		return errors.New("ready status unknown")
	}
	if cond := conds[i]; cond.Status != metav1.ConditionTrue {
		return errors.Errorf("resource is not Ready: %s", cond.Message)
	}
	return nil
}

// Pause implements azure.Pauser.
func (s *Service[T, S]) Pause(ctx context.Context) error {
	var _ azure.Pauser = (*Service[T, S])(nil)

	ctx, _, done := tele.StartSpanWithLogger(ctx, "aso.Service.Pause")
	defer done()

	for _, spec := range s.Specs {
		ref := spec.ResourceRef()
		if err := s.PauseResource(ctx, ref, s.Name()); err != nil {
			return errors.Wrapf(err, "failed to pause ASO resource %s %s/%s", ref.GetObjectKind().GroupVersionKind(), s.Scope.ASOOwner().GetNamespace(), ref.GetName())
		}
	}

	return nil
}

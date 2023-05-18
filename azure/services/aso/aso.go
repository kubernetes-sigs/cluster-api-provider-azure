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
	"fmt"
	"time"

	"github.com/Azure/azure-service-operator/v2/pkg/genruntime"
	"github.com/Azure/azure-service-operator/v2/pkg/genruntime/conditions"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	requeueInterval = 20 * time.Second

	createOrUpdateFutureType = "ASOCreateOrUpdate"
	deleteFutureType         = "ASODelete"
)

// Service is an implementation of the Reconciler interface. It handles creation
// and deletion of resources using ASO.
type Service struct {
	client.Client
}

// New creates a new ASO service.
func New(ctrlClient client.Client) *Service {
	return &Service{
		Client: ctrlClient,
	}
}

// CreateOrUpdateResource implements the logic for creating a new or updating an
// existing resource with ASO.
func (s *Service) CreateOrUpdateResource(ctx context.Context, spec azure.ASOResourceSpecGetter, serviceName string) (result genruntime.MetaObject, err error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "aso.Service.CreateOrUpdateResource")
	defer done()

	resource := spec.ResourceRef()
	resourceName := resource.GetName()
	resourceNamespace := resource.GetNamespace()

	log = log.WithValues("service", serviceName, "resource", resourceName, "namespace", resourceNamespace)

	var existing genruntime.MetaObject
	if err := s.Client.Get(ctx, client.ObjectKeyFromObject(resource), resource); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, errors.Wrapf(err, "failed to get existing resource %s/%s (service: %s)", resourceNamespace, resourceName, serviceName)
		}
	} else {
		existing = resource
		log.V(2).Info("successfully got existing resource")

		// Check if there is an ongoing long running operation.
		conds := existing.GetConditions()
		i, readyExists := conds.FindIndexByType(conditions.ConditionTypeReady)
		if !readyExists {
			return nil, azure.WithTransientError(errors.New("ready status unknown"), requeueInterval)
		}
		var readyErr error
		if cond := conds[i]; cond.Status != metav1.ConditionTrue {
			switch {
			case cond.Reason == conditions.ReasonReconciling.Name:
				readyErr = azure.NewOperationNotDoneError(&infrav1.Future{
					Type:          createOrUpdateFutureType,
					ResourceGroup: existing.GetNamespace(),
					Name:          existing.GetName(),
				})
			default:
				readyErr = fmt.Errorf("resource is not Ready: %s", conds[i].Message)
			}

			if readyErr != nil {
				if conds[i].Severity == conditions.ConditionSeverityError {
					return nil, azure.WithTerminalError(readyErr)
				}
				return nil, azure.WithTransientError(readyErr, requeueInterval)
			}
		}
	}

	// Construct parameters using the resource spec and information from the existing resource, if there is one.
	parameters, err := spec.Parameters(ctx, deepCopyOrNil(existing))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get desired parameters for resource %s/%s (service: %s)", resourceNamespace, resourceName, serviceName)
	}
	// a nil result here is a special case for compatibility with the old
	// SDK-driven service implementations.
	if parameters == nil {
		if existing == nil {
			return nil, errors.New("parameters cannot be nil if no object already exists")
		}
		parameters = existing.DeepCopyObject().(genruntime.MetaObject)
	}

	parameters.SetName(resourceName)
	parameters.SetNamespace(resourceNamespace)

	diff := cmp.Diff(existing, parameters)
	if diff == "" {
		log.V(2).Info("resource up to date")
		return existing, nil
	}

	// Create or update the resource with the desired parameters.
	logMessageVerbPrefix := "creat"
	if existing != nil {
		logMessageVerbPrefix = "updat"
	}
	log.V(2).Info(logMessageVerbPrefix+"ing resource", "diff", diff)
	if existing != nil {
		var helper *patch.Helper
		helper, err = patch.NewHelper(existing, s.Client)
		if err != nil {
			return nil, errors.Errorf("failed to init patch helper: %v", err)
		}
		err = helper.Patch(ctx, parameters)
	} else {
		err = s.Client.Create(ctx, parameters)
	}
	if err == nil {
		// Resources need to be requeued to wait for the create or update to finish.
		return nil, azure.WithTransientError(azure.NewOperationNotDoneError(&infrav1.Future{
			Type:          createOrUpdateFutureType,
			ResourceGroup: resourceNamespace,
			Name:          resourceName,
		}), requeueInterval)
	}
	return nil, errors.Wrapf(err, fmt.Sprintf("failed to %se resource %s/%s (service: %s)", logMessageVerbPrefix, resourceNamespace, resourceName, serviceName))
}

// DeleteResource implements the logic for deleting a resource Asynchronously.
func (s *Service) DeleteResource(ctx context.Context, spec azure.ASOResourceSpecGetter, serviceName string) (err error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "aso.Service.DeleteResource")
	defer done()

	resource := spec.ResourceRef()
	resourceName := resource.GetName()
	resourceNamespace := resource.GetNamespace()

	log = log.WithValues("service", serviceName, "resource", resourceName, "namespace", resourceNamespace)

	log.V(2).Info("deleting resource")
	err = s.Client.Delete(ctx, resource)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// already deleted
			log.V(2).Info("successfully deleted resource")
			return nil
		}
		return errors.Wrapf(err, "failed to delete resource %s/%s (service: %s)", resourceNamespace, resourceName, serviceName)
	}

	return azure.WithTransientError(azure.NewOperationNotDoneError(&infrav1.Future{
		Type:          deleteFutureType,
		ResourceGroup: resourceNamespace,
		Name:          resourceName,
	}), requeueInterval)
}

func deepCopyOrNil(obj genruntime.MetaObject) genruntime.MetaObject {
	if obj == nil {
		return nil
	}
	return obj.DeepCopyObject().(genruntime.MetaObject)
}

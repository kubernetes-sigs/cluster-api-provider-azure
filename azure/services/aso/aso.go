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
	"sigs.k8s.io/cluster-api-provider-azure/util/maps"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ReconcilePolicyAnnotation is the annotation key for ASO's reconcile policy mechanism.
	ReconcilePolicyAnnotation = "serviceoperator.azure.com/reconcile-policy"
	// ReconcilePolicySkip indicates changes will not be PUT or DELETEd in Azure from ASO.
	ReconcilePolicySkip = "skip"
	// ReconcilePolicyManage indicates changes will be PUT and DELETEd in Azure from ASO.
	ReconcilePolicyManage = "manage"

	requeueInterval = 20 * time.Second

	createOrUpdateFutureType = "ASOCreateOrUpdate"
	deleteFutureType         = "ASODelete"
)

// Service is an implementation of the Reconciler interface. It handles creation
// and deletion of resources using ASO.
type Service struct {
	client.Client

	clusterName string
}

// New creates a new ASO service.
func New(ctrlClient client.Client, clusterName string) *Service {
	return &Service{
		Client:      ctrlClient,
		clusterName: clusterName,
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

		if !ownedByCluster(existing.GetLabels(), s.clusterName) {
			log.V(4).Info("skipping reconcile for unmanaged resource")
			return existing, nil
		}

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

	labels := make(map[string]string)
	annotations := make(map[string]string)

	if existing == nil {
		labels[infrav1.OwnedByClusterLabelKey] = s.clusterName
		annotations[ReconcilePolicyAnnotation] = ReconcilePolicyManage
	}

	if len(labels) > 0 {
		parameters.SetLabels(maps.Merge(parameters.GetLabels(), labels))
	}
	if len(annotations) > 0 {
		parameters.SetAnnotations(maps.Merge(parameters.GetAnnotations(), annotations))
	}

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

	managed, err := IsManaged(ctx, s.Client, spec, s.clusterName)
	if apierrors.IsNotFound(err) {
		// already deleted
		log.V(2).Info("successfully deleted resource")
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "failed to determine if resource is managed")
	}
	if !managed {
		log.V(4).Info("skipping delete for unmanaged resource")
		return nil
	}

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

// IsManaged returns whether the ASO resource referred to by spec was created by
// CAPZ and therefore whether CAPZ should manage its lifecycle.
func IsManaged(ctx context.Context, ctrlClient client.Client, spec azure.ASOResourceSpecGetter, clusterName string) (bool, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "aso.Service.IsManaged")
	defer done()

	resource := spec.ResourceRef()
	err := ctrlClient.Get(ctx, client.ObjectKeyFromObject(resource), resource)
	if err != nil {
		return false, errors.Wrap(err, "error getting resource")
	}

	return ownedByCluster(resource.GetLabels(), clusterName), nil
}

func ownedByCluster(labels map[string]string, clusterName string) bool {
	return labels[infrav1.OwnedByClusterLabelKey] == clusterName
}

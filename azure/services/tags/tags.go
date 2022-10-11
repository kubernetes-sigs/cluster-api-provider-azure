/*
Copyright 2022 The Kubernetes Authors.

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

package tags

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-10-01/resources"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

const serviceName = "tags"

// TagScope defines the scope interface for a tags service.
type TagScope interface {
	azure.Authorizer
	ClusterName() string
	TagsSpecs() ([]azure.TagsSpecGetter, error)
	AnnotationJSON(string) (map[string]interface{}, error)
	UpdateAnnotationJSON(string, map[string]interface{}) error
}

// Service provides operations on Azure resources.
type Service struct {
	Scope TagScope
	Client
}

// New creates a new service.
func New(scope TagScope) *Service {
	return &Service{
		Scope:  scope,
		Client: NewClient(scope),
	}
}

// Name returns the service name.
func (s *Service) Name() string {
	return serviceName
}

// Reconcile ensures tags are correct.
func (s *Service) Reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "tags.Service.Reconcile")
	defer done()

	specs, err := s.Scope.TagsSpecs()
	if err != nil {
		return errors.Wrap(err, "failed to get tags specs")
	}

	updateTagsPatchResource := func(spec azure.TagsSpecGetter, params *resources.TagsPatchResource) error {
		if params == nil {
			return nil
		}
		if _, err := s.Client.UpdateAtScope(ctx, spec, *params); err != nil {
			return errors.Wrapf(err, "cannot apply operation `%s` on tags", params.Operation)
		}

		return nil
	}

	for _, tagsSpec := range specs {
		existingTags, err := s.Client.GetAtScope(ctx, tagsSpec)
		if err != nil {
			return errors.Wrap(err, "failed to get existing tags")
		}
		if existingTags.Properties != nil && existingTags.Properties.Tags != nil && !s.isResourceManaged(existingTags.Properties.Tags) {
			log.V(4).Info("skipping tags reconciliation for unmanaged resource")
			continue
		}

		createdOrUpdatedParams, err := tagsSpec.MergeParameters(&existingTags)
		if err != nil {
			return errors.Wrap(err, "failed to get merge operation parameters")
		}
		if err := updateTagsPatchResource(tagsSpec, createdOrUpdatedParams); err != nil {
			return err
		}

		deleteParams, err := tagsSpec.DeleteParameters(&existingTags)
		if err != nil {
			return errors.Wrap(err, "failed to get delete operation parameters")
		}
		if err := updateTagsPatchResource(tagsSpec, deleteParams); err != nil {
			return err
		}

		annotations, err := tagsSpec.NewAnnotation(&existingTags)
		if err != nil {
			return errors.Wrap(err, "failed to get annotation")
		}
		if err := s.Scope.UpdateAnnotationJSON(tagsSpec.TagsScope(), annotations); err != nil {
			return errors.Wrap(err, "failed to update annotation")
		}
	}

	return nil
}

func (s *Service) isResourceManaged(tags map[string]*string) bool {
	return converters.MapToTags(tags).HasOwned(s.Scope.ClusterName())
}

// Delete is a no-op as the tags get deleted as part of VM deletion.
func (s *Service) Delete(ctx context.Context) error {
	_, _, done := tele.StartSpanWithLogger(ctx, "tags.Service.Delete")
	defer done()

	return nil
}

// IsManaged returns always returns true as CAPZ does not support BYO tags.
func (s *Service) IsManaged(ctx context.Context) (bool, error) {
	return true, nil
}

/*
Copyright 2019 The Kubernetes Authors.

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

package controllers

import (
	"context"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualmachines"
)

const (
	// TagsLastAppliedAnnotation is the key for the machine object annotation
	// which tracks the SecurityGroups that the machine actuator is responsible
	// for. These are the SecurityGroups that have been handled by the
	// AdditionalTags in the Machine Provider Config.
	// See https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/
	// for annotation formatting rules.
	TagsLastAppliedAnnotation = "sigs.k8s.io/cluster-api-provider-azure-last-applied-tags"
)

// Ensure that the tags of the machine are correct
func (r *AzureMachineReconciler) reconcileTags(ctx context.Context, scope *scope.MachineScope, skuCache *resourceskus.Cache) error {
	annotation, err := r.machineAnnotationJSON(scope.AzureMachine, TagsLastAppliedAnnotation)
	if err != nil {
		return err
	}
	changed, created, deleted, newAnnotation := TagsChanged(annotation, scope.AdditionalTags())
	if changed {
		scope.Info("Updating tags on AzureMachine")
		svc := virtualmachines.NewService(scope, skuCache)
		vm, err := svc.Client.Get(ctx, scope.ResourceGroup(), scope.Name())
		if err != nil {
			return errors.Wrapf(err, "failed to query AzureMachine VM")
		}
		tags := vm.Tags
		for k, v := range created {
			tags[k] = to.StringPtr(v)
		}

		for k := range deleted {
			delete(tags, k)
		}

		vm.Tags = tags
		if err := svc.Client.CreateOrUpdate(ctx, scope.ResourceGroup(), scope.Name(), vm); err != nil {
			return errors.Wrapf(err, "cannot update VM tags")
		}

		// We also need to update the annotation if anything changed.
		if err = r.updateMachineAnnotationJSON(scope.AzureMachine, TagsLastAppliedAnnotation, newAnnotation); err != nil {
			return err
		}
	}

	return nil
}

// TagsChanged determines which tags to delete and which to add.
func TagsChanged(annotation map[string]interface{}, src map[string]string) (bool, map[string]string, map[string]string, map[string]interface{}) {
	// Bool tracking if we found any changed state.
	changed := false

	// Tracking for created/updated
	created := map[string]string{}

	// Tracking for tags that were deleted.
	deleted := map[string]string{}

	// The new annotation that we need to set if anything is created/updated.
	newAnnotation := map[string]interface{}{}

	// Loop over annotation, checking if entries are in src.
	// If an entry is present in annotation but not src, it has been deleted
	// since last time. We flag this in the deleted map.
	for t, v := range annotation {
		_, ok := src[t]

		// Entry isn't in src, it has been deleted.
		if !ok {
			// Cast v to a string here. This should be fine, tags are always
			// strings.
			deleted[t] = v.(string)
			changed = true
		}
	}

	// Loop over src, checking for entries in annotation.
	//
	// If an entry is in src, but not annotation, it has been created since
	// last time.
	//
	// If an entry is in both src and annotation, we compare their values, if
	// the value in src differs from that in annotation, the tag has been
	// updated since last time.
	for t, v := range src {
		av, ok := annotation[t]

		// Entries in the src always need to be noted in the newAnnotation. We
		// know they're going to be created or updated.
		newAnnotation[t] = v

		// Entry isn't in annotation, it's new.
		if !ok {
			created[t] = v
			newAnnotation[t] = v
			changed = true
			continue
		}

		// Entry is in annotation, has the value changed?
		if v != av {
			created[t] = v
			changed = true
		}

		// Entry existed in both src and annotation, and their values were
		// equal. Nothing to do.
	}

	// We made it through the loop, and everything that was in src, was also
	// in dst. Nothing changed.
	return changed, created, deleted, newAnnotation
}

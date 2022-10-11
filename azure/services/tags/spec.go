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
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-10-01/resources"
	"github.com/Azure/go-autorest/autorest/to"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

// TagsSpec defines the specification for a set of tags.
type TagsSpec struct {
	Scope string
	Tags  infrav1.Tags
	// The last applied tags are used to find out which tags are being managed by CAPZ
	// and if any has to be deleted by comparing it with the new desired tags. They can
	// be found using an annotation as a key.
	LastAppliedTags map[string]interface{}
}

// TagsScope returns the scope of a set of tags.
func (s *TagsSpec) TagsScope() string {
	return s.Scope
}

// MergeParameters returns the merge parameters for a set of tags.
func (s *TagsSpec) MergeParameters(existing *resources.TagsResource) (*resources.TagsPatchResource, error) {
	tags := make(map[string]*string)
	if existing != nil && existing.Properties != nil && existing.Properties.Tags != nil {
		tags = existing.Properties.Tags
	}

	changed, createdOrUpdated, _ := getCreatedOrUpdatedTags(s.LastAppliedTags, s.Tags, tags)
	if !changed {
		// Nothing to create or update.
		return nil, nil
	}

	if len(createdOrUpdated) > 0 {
		createdOrUpdatedTags := make(map[string]*string)
		for k, v := range createdOrUpdated {
			createdOrUpdatedTags[k] = to.StringPtr(v)
		}

		return &resources.TagsPatchResource{Operation: "Merge", Properties: &resources.Tags{Tags: createdOrUpdatedTags}}, nil
	}

	return nil, nil
}

// NewAnnotation returns the new annotation for a set of tags.
func (s *TagsSpec) NewAnnotation(existing *resources.TagsResource) (map[string]interface{}, error) {
	tags := make(map[string]*string)
	if existing != nil && existing.Properties != nil && existing.Properties.Tags != nil {
		tags = existing.Properties.Tags
	}

	changed, _, newAnnotation := getCreatedOrUpdatedTags(s.LastAppliedTags, s.Tags, tags)
	if !changed {
		// Nothing created or updated.
		return nil, nil
	}

	return newAnnotation, nil
}

// DeleteParameters returns the delete parameters for a set of tags.
func (s *TagsSpec) DeleteParameters(existing *resources.TagsResource) (*resources.TagsPatchResource, error) {
	changed, deleted := getDeletedTags(s.LastAppliedTags, s.Tags)
	if !changed {
		// Nothing to delete, return nil
		return nil, nil
	}

	if len(deleted) > 0 {
		deletedTags := make(map[string]*string)
		for k, v := range deleted {
			deletedTags[k] = to.StringPtr(v)
		}

		return &resources.TagsPatchResource{Operation: "Delete", Properties: &resources.Tags{Tags: deletedTags}}, nil
	}

	return nil, nil
}

// getCreatedOrUpdatedTags determines which tags to which to add.
func getCreatedOrUpdatedTags(lastAppliedTags map[string]interface{}, desiredTags map[string]string, currentTags map[string]*string) (bool, map[string]string, map[string]interface{}) {
	// Bool tracking if we found any changed state.
	changed := false

	// Tracking for created/updated
	createdOrUpdated := map[string]string{}

	// The new annotation that we need to set if anything is created/updated.
	newAnnotation := map[string]interface{}{}

	// Loop over desiredTags, checking for entries in currentTags.
	//
	// If an entry is in desiredTags, but not currentTags, it has been created since
	// last time, or some external entity deleted it.
	//
	// If an entry is in both desiredTags and currentTags, we compare their values, if
	// the value in desiredTags differs from that in currentTags, the tag has been
	// updated since last time or some external entity modified it.
	for t, v := range desiredTags {
		av, ok := currentTags[t]

		// Entries in the desiredTags always need to be noted in the newAnnotation. We
		// know they're going to be created or updated.
		newAnnotation[t] = v

		// Entry isn't in desiredTags, it's new.
		if !ok {
			createdOrUpdated[t] = v
			newAnnotation[t] = v
			changed = true
			continue
		}
		// newAnnotations = union(desiredTags, updatedValues in currentTags)

		// Entry is in desiredTags, has the value changed?
		if v != *av {
			createdOrUpdated[t] = v
			changed = true
		}

		// Entry existed in both desiredTags and desiredTags, and their values were
		// equal. Nothing to do.
	}

	// We made it through the loop, and everything that was in desiredTags, was also
	// in dst. Nothing changed.
	return changed, createdOrUpdated, newAnnotation
}

// getDeletedTags determines which tags to delete and which to add.
func getDeletedTags(lastAppliedTags map[string]interface{}, desiredTags map[string]string) (bool, map[string]string) {
	// Bool tracking if we found any changed state.
	changed := false

	// Tracking for tags that were deleted.
	deleted := map[string]string{}

	// Loop over lastAppliedTags, checking if entries are in desiredTags.
	// If an entry is present in lastAppliedTags but not in desiredTags, it has been deleted
	// since last time. We flag this in the deleted map.
	for t, v := range lastAppliedTags {
		_, ok := desiredTags[t]

		// Entry isn't in desiredTags, it has been deleted.
		if !ok {
			// Cast v to a string here. This should be fine, tags are always
			// strings.
			deleted[t] = v.(string)
			changed = true
		}
	}

	// We made it through the loop, and everything that was in desiredTags, was also
	// in dst. Nothing changed.
	return changed, deleted
}

// // tagsChanged determines which tags to delete and which to add.
// func tagsChanged(lastAppliedTags map[string]interface{}, desiredTags map[string]string, currentTags map[string]*string) (bool, map[string]string, map[string]string, map[string]interface{}) {
// 	// Bool tracking if we found any changed state.
// 	changed := false

// 	// Tracking for created/updated
// 	createdOrUpdated := map[string]string{}

// 	// Tracking for tags that were deleted.
// 	deleted := map[string]string{}

// 	// The new annotation that we need to set if anything is created/updated.
// 	newAnnotation := map[string]interface{}{}

// 	// Loop over lastAppliedTags, checking if entries are in desiredTags.
// 	// If an entry is present in lastAppliedTags but not in desiredTags, it has been deleted
// 	// since last time. We flag this in the deleted map.
// 	for t, v := range lastAppliedTags {
// 		_, ok := desiredTags[t]

// 		// Entry isn't in desiredTags, it has been deleted.
// 		if !ok {
// 			// Cast v to a string here. This should be fine, tags are always
// 			// strings.
// 			deleted[t] = v.(string)
// 			changed = true
// 		}
// 	}

// 	// Loop over desiredTags, checking for entries in currentTags.
// 	//
// 	// If an entry is in desiredTags, but not currentTags, it has been created since
// 	// last time, or some external entity deleted it.
// 	//
// 	// If an entry is in both desiredTags and currentTags, we compare their values, if
// 	// the value in desiredTags differs from that in currentTags, the tag has been
// 	// updated since last time or some external entity modified it.
// 	for t, v := range desiredTags {
// 		av, ok := currentTags[t]

// 		// Entries in the desiredTags always need to be noted in the newAnnotation. We
// 		// know they're going to be created or updated.
// 		newAnnotation[t] = v

// 		// Entry isn't in desiredTags, it's new.
// 		if !ok {
// 			createdOrUpdated[t] = v
// 			newAnnotation[t] = v
// 			changed = true
// 			continue
// 		}

// 		// Entry is in desiredTags, has the value changed?
// 		if v != *av {
// 			createdOrUpdated[t] = v
// 			changed = true
// 		}

// 		// Entry existed in both desiredTags and desiredTags, and their values were
// 		// equal. Nothing to do.
// 	}

// 	// We made it through the loop, and everything that was in desiredTags, was also
// 	// in dst. Nothing changed.
// 	return changed, createdOrUpdated, deleted, newAnnotation
// }

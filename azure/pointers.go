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

package azure

import "k8s.io/utils/pointer"

// StringSlice returns a string slice value for the passed string slice pointer. It returns a nil
// slice if the pointer is nil.
func StringSlice(s *[]string) []string {
	if s != nil {
		return *s
	}
	return nil
}

// StringMapPtr returns a pointer to a given string map, or nil if the map is nil.
func StringMapPtr(m map[string]string) *map[string]*string { //nolint:gocritic
	msp := make(map[string]*string, len(m))
	for k, v := range m {
		msp[k] = pointer.String(v)
	}
	return &msp
}

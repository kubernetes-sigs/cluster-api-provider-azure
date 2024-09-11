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

// Package versions provides utilities for working with package versions.
package versions

import (
	"strings"

	"golang.org/x/mod/semver"
)

// GetHigherK8sVersion returns the higher k8s version out of a and b.
func GetHigherK8sVersion(a, b string) string {
	v1 := a
	if !strings.HasPrefix(a, "v") {
		v1 = "v" + a
	}
	v2 := b
	if !strings.HasPrefix(b, "v") {
		v2 = "v" + b
	}

	if comp := semver.Compare(v1, v2); comp < 0 {
		return b
	}
	return a
}

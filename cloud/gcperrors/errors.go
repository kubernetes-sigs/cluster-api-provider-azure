/*
Copyright 2018 The Kubernetes Authors.

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

package azureerrors

import (
	"net/http"

	"google.golang.org/api/googleapi"
)

// IsNotModified reports whether err is the result of the
// server replying with http.StatusNotModified.
// Such error values are sometimes returned by "Do" methods
// on calls when If-None-Match is used.
func IsNotModified(err error) bool {
	if err == nil {
		return false
	}
	ae, ok := err.(*googleapi.Error)
	return ok && ae.Code == http.StatusNotModified
}

// IsNotFound reports whether err is a Google API error
// with http.StatusNotModified.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	ae, ok := err.(*googleapi.Error)
	return ok && ae.Code == http.StatusNotFound
}

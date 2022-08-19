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

package services

import (
	"strconv"
	"time"

	"github.com/pkg/errors"
)

// Standard defaults for Azure service exponential backoff responses,
// in case we don't get Retry-After data in the HTTP header response.
const (
	// DefaultBackoffWaitTimeRead is the default backoff time for a read request following a HTTP 429 response.
	DefaultBackoffWaitTimeRead time.Duration = 1 * time.Minute
	// DefaultBackoffWaitTimeWrite is the default backoff time for a write request following a HTTP 429 response.
	DefaultBackoffWaitTimeWrite time.Duration = 3 * time.Minute
	// DefaultBackoffWaitTimeDelete is the default backoff time for a delete request following a HTTP 429 response.
	DefaultBackoffWaitTimeDelete time.Duration = 3 * time.Minute
)

var retryAfterError error = errors.Errorf("Retry-After has not yet expired")

// GetRetryAfterTime parses the Retry-After data from an HTTP response header
// and returns the absolute time after adding that data to the current time.
// If we can't interpret Retry-After data we return a passed in default.
func GetRetryAfterTime(ra string, defaultWait time.Duration) time.Time {
	// Retry-After can be expressed in either units of seconds or absolute time
	// Here we handle the case when Retry-After is expressed in units of seconds
	if retryAfter, _ := strconv.Atoi(ra); retryAfter > 0 {
		dur := time.Duration(retryAfter) * time.Second
		return time.Now().Add(dur)
		// Here we handle the case when Retry-After is expressed in absolute time
	} else if t, err := time.Parse(time.RFC1123, ra); err == nil {
		return t
	}
	// If we can't figure out how to interpret Retry-After simply return the default
	return time.Now().Add(defaultWait)
}

// +build e2e

/*
Copyright 2021 The Kubernetes Authors.

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

package e2e

import (
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	// Parameters for retrying with exponential backoff.
	retryBackoffInitialDuration = 100 * time.Millisecond
	retryBackoffFactor          = 3
	retryBackoffJitter          = 0.1
	retryBackoffSteps           = 3
)

// retryWithExponentialBackOff retries the function until it returns nil,
// or until the number of attempts (steps) has reached the maximum value.
func retryWithExponentialBackOff(fn func() error) error {
	backoff := wait.Backoff{
		Duration: retryBackoffInitialDuration,
		Factor:   retryBackoffFactor,
		Jitter:   retryBackoffJitter,
		Steps:    retryBackoffSteps,
	}
	retryFn := func(fn func() error) func() (bool, error) {
		return func() (bool, error) {
			err := fn()
			if err == nil {
				return true, nil
			}
			return false, err
		}
	}
	return wait.ExponentialBackoff(backoff, retryFn(fn))
}

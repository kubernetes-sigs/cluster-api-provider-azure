//go:build e2e
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
	"context"
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

// retryWithTimeout retries a function that returns an error until a timeout is reached
func retryWithTimeout(interval, timeout time.Duration, fn func() error) error {
	var pollError error
	err := wait.PollUntilContextTimeout(context.TODO(), interval, timeout, true, func(context.Context) (bool, error) {
		pollError = nil
		err := fn()
		if err != nil {
			pollError = err
			return false, nil //nolint:nilerr // We don't want to return err here
		}
		return true, nil
	})
	if pollError != nil {
		return pollError
	}
	return err
}

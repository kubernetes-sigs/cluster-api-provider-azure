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
	"errors"
	"fmt"
	"reflect"
	"runtime"
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
	var pollError, returnError error
	result := make(chan error)

	ctx, cancelFunc := context.WithTimeout(context.Background(), timeout)
	defer cancelFunc()

	go func(ctx context.Context) {
		result <- wait.PollImmediateWithContext(ctx, interval, timeout, func(ctx context.Context) (bool, error) {
			if ctx.Err() != nil {
				return false, ctx.Err()
			}
			err := fn() // TODO: mitigate function leaking; fn() can run in the background
			if err != nil {
				pollError = err
				return false, nil //nolint:nilerr // do not error yet to retry
			}
			return true, nil
		})
	}(ctx)

	select {
	case <-ctx.Done(): // as soon as context Timesout
		funcName := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
		errStr := fmt.Sprintf("timed out waiting for %s to complete", funcName)
		returnError = errors.New(errStr)
	case myResult := <-result: // result gets filled when fn() returns nil error or context times out. But context time out is caught in above case.
		returnError = myResult
	}

	if returnError != nil {
		if pollError != nil {
			returnError = errors.New(pollError.Error() + " : " + returnError.Error())
		}
	}

	return returnError
}

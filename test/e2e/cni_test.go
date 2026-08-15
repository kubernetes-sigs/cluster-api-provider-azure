//go:build e2e
// +build e2e

/*
Copyright 2024 The Kubernetes Authors.

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
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// conflictErr returns a Conflict StatusError using the proper schema.GroupResource type.
func conflictErr() error {
	return apierrors.NewConflict(schema.GroupResource{Group: "apps", Resource: "daemonsets"}, "azure-cni", errors.New("the object has been modified"))
}

func TestIsConflictError_Nil(t *testing.T) {
	if isConflictError(nil) {
		t.Error("expected nil to not be a conflict")
	}
}

func TestIsConflictError_DirectConflict(t *testing.T) {
	if !isConflictError(conflictErr()) {
		t.Error("expected a direct conflict error to be retryable")
	}
}

func TestIsConflictError_DirectNonConflict(t *testing.T) {
	if isConflictError(errors.New("some other error")) {
		t.Error("expected a non-conflict error to not be retryable")
	}
}

func TestIsConflictError_AggregateOnlyConflicts(t *testing.T) {
	agg := utilerrors.NewAggregate([]error{conflictErr(), conflictErr()})
	if !isConflictError(agg) {
		t.Error("expected aggregate of only conflict errors to be retryable")
	}
}

func TestIsConflictError_AggregateOnlyNonConflicts(t *testing.T) {
	agg := utilerrors.NewAggregate([]error{errors.New("err1"), errors.New("err2")})
	if isConflictError(agg) {
		t.Error("expected aggregate of only non-conflict errors to not be retryable")
	}
}

func TestIsConflictError_AggregateMixed(t *testing.T) {
	agg := utilerrors.NewAggregate([]error{conflictErr(), errors.New("non-conflict")})
	if isConflictError(agg) {
		t.Error("expected mixed aggregate to not be retryable")
	}
}

func TestIsConflictError_EmptyAggregate(t *testing.T) {
	agg := utilerrors.NewAggregate([]error{})
	// NewAggregate returns nil for an empty slice.
	if isConflictError(agg) {
		t.Error("expected empty/nil aggregate to not be a conflict")
	}
}

// fakeCreateOrUpdate simulates a workload cluster CreateOrUpdate call for retry tests.
type fakeCreateOrUpdate struct {
	calls  int
	errors []error
}

func (f *fakeCreateOrUpdate) do(_ []byte) error {
	idx := f.calls
	f.calls++
	if idx < len(f.errors) {
		return f.errors[idx]
	}
	return nil
}

func TestRetryOnConflict_SuccessAfterOneConflict(t *testing.T) {
	fake := &fakeCreateOrUpdate{
		errors: []error{conflictErr()}, // first call conflicts, second succeeds
	}
	ctx := context.Background()
	var lastErr error
	retryErr := retryCreateOrUpdate(ctx, func() error {
		return fake.do(nil)
	}, &lastErr)
	if retryErr != nil || lastErr != nil {
		t.Errorf("expected success, got retryErr=%v lastErr=%v", retryErr, lastErr)
	}
	if fake.calls != 2 {
		t.Errorf("expected 2 calls, got %d", fake.calls)
	}
}

func TestRetryOnConflict_ImmediateNonConflict(t *testing.T) {
	nonConflict := errors.New("internal error")
	fake := &fakeCreateOrUpdate{
		errors: []error{nonConflict},
	}
	ctx := context.Background()
	var lastErr error
	retryErr := retryCreateOrUpdate(ctx, func() error {
		return fake.do(nil)
	}, &lastErr)
	if retryErr != nonConflict {
		t.Errorf("expected non-conflict to be returned immediately, got %v", retryErr)
	}
	if fake.calls != 1 {
		t.Errorf("expected 1 call, got %d", fake.calls)
	}
}

func TestRetryOnConflict_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	fake := &fakeCreateOrUpdate{
		errors: []error{conflictErr(), conflictErr(), conflictErr()},
	}
	var lastErr error
	retryErr := retryCreateOrUpdate(ctx, func() error {
		return fake.do(nil)
	}, &lastErr)
	if retryErr == nil {
		t.Error("expected error due to context cancellation")
	}
}

func TestRetryOnConflict_Success(t *testing.T) {
	fake := &fakeCreateOrUpdate{} // no errors
	ctx := context.Background()
	var lastErr error
	retryErr := retryCreateOrUpdate(ctx, func() error {
		return fake.do(nil)
	}, &lastErr)
	if retryErr != nil || lastErr != nil {
		t.Errorf("expected clean success, got retryErr=%v lastErr=%v", retryErr, lastErr)
	}
}

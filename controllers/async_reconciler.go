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

package controllers

import (
	"context"
	"reflect"
	"sync"

	"github.com/pkg/errors"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

// asyncReconciler executes multiple reconcilers simultaneously and stores the output in results.
type asyncReconciler struct {
	wg      sync.WaitGroup
	results []result
}

// result represents the outcome of a reconilitation operation along with some metadata.
type result struct {
	err        error
	reconciler azure.Reconciler
}

// newAsyncReconciler returns a new instance of asyncReconciler.
func newAsyncReconciler() asyncReconciler {
	return asyncReconciler{}
}

// submit initiates a go routine for the reconciler and appends the result to results.
func (ar *asyncReconciler) submit(ctx context.Context, reconciler azure.Reconciler) {
	ar.wg.Add(1)
	go func() {
		err := reconciler.Reconcile(ctx)
		ar.results = append(ar.results, result{err, reconciler})
		ar.wg.Done()
	}()
}

// wait waits for all pending reconcilers to complete.
func (ar *asyncReconciler) wait() error {
	ar.wg.Wait()

	defer func() {
		ar.results = []result{}
	}()

	var errs []error
	for _, r := range ar.results {
		if r.err != nil {
			reconcilerImpl := reflect.TypeOf(r.reconciler)
			errs = append(errs, errors.Wrapf(r.err, "failed to reconcile %s", reconcilerImpl.Name()))
		}
	}

	if len(errs) > 0 {
		return kerrors.NewAggregate(errs)
	}

	return nil
}
